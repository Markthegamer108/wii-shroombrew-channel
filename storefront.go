package main

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"html"
	"html/template"
	"io/fs"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

//go:embed shop/*
var shopFiles embed.FS

type catalogApp struct {
	Slug        string `json:"slug"`
	Name        string `json:"name"`
	Author      string `json:"author"`
	Category    string `json:"category"`
	Version     string `json:"version"`
	Flash       bool   `json:"flash"`
	// Licensed marks a real retail/Wii Shop WAD (Virtual Console, WiiWare,
	// DLC, channels, IOS) that Shroombrew does not host: the console's native
	// download imports the WAD directly from its external URL (mariocube) on
	// first use, caching it locally in ./channels/. Price is the authentic
	// Wii Points price (0 when the catalog falls back to derived pricing).
	Licensed bool `json:"licensed"`
	Price    int  `json:"price"`
	Description struct {
		Short string `json:"short"`
		Long  string `json:"long"`
	} `json:"description"`
	URL struct {
		Icon string `json:"icon"`
		Zip  string `json:"zip"`
	} `json:"url"`
	FileSize struct {
		ZipCompressed   int64 `json:"zip_compressed"`
		ZipUncompressed int64 `json:"zip_uncompressed"`
	} `json:"file_size"`
	Shop        Shop     `json:"shop"`
	Peripherals []string `json:"peripherals"`
	ReleaseDate int64    `json:"release_date"`
	// Real Wii Shop metadata (from the WiiMart revival of the real catalog):
	// the canonical platform (NES, Super NES, Nintendo 64, Sega Genesis,
	// WiiWare, Wii Channel, ...), genre and ESRB rating exactly as the real
	// shop displayed them.
	Platform string `json:"platform,omitempty"`
	Genre    string `json:"genre,omitempty"`
	Rating   string `json:"rating,omitempty"`
	// Variants lists every region-locked release of a collapsed licensed title
	// (Europe/USA/Japan/Korea/...). The console's region selects which one the
	// native download imports, so one catalog row serves the right WAD for any
	// region. Empty for homebrew and single-release titles.
	Variants []catalogVariant `json:"variants,omitempty"`
}

// catalogVariant is one region-locked release of a collapsed licensed title:
// its region marker and coarse console-region group, the release's real title
// ID and mariocube WAD URL, and the authentic Wii Points price.
type catalogVariant struct {
	Region      string `json:"region,omitempty"`
	RegionGroup string `json:"region_group,omitempty"`
	TitleID     string `json:"title_id,omitempty"`
	Zip         string `json:"zip,omitempty"`
	Price       int    `json:"price,omitempty"`
}
type storefront struct {
	apps         []catalogApp
	bySlug       map[string]catalogApp
	view         *template.Template
	installs     *installManager
	channels          *channelBuilder
	channelsRoot      string
	iconsRoot         string
	flashTemplatePath string
}
type shopPage struct {
	Screen       string
	Section      string
	SectionTitle string
	Title        string
	Apps         []catalogApp
	Featured     []catalogApp
	App          *catalogApp
	DownloadSize string
	InstallCode  string
	Categories   []string
	Category     string
	Query        string
	Order        string
	Total        int
	Page         int
	Pages        int
	Prev         string
	Next         string
	Wii          bool
	Platform     string
}

var safeSlug = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)
var safeAsset = regexp.MustCompile(`^/shop/assets/[A-Za-z0-9_-]+\.(png|gif)$`)

// displayName repairs catalog entries whose name field holds the short
// description (OceanPop is listed as "A relaxing match 3 puzzle game"); the
// slug is the app's real name in those cases. Real names are written in title
// case ("SEEPROM Viewer"); a description has lower-case words.
func displayName(a catalogApp) string {
	words := strings.Fields(a.Name)
	if a.Name != a.Description.Short || len(words) < 2 {
		return a.Name
	}
	for _, word := range words[1:] {
		if c := word[0]; c >= 'a' && c <= 'z' {
			return a.Slug
		}
	}
	return a.Name
}

func newStorefront(path string) (*storefront, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var apps []catalogApp
	if err = json.Unmarshal(data, &apps); err != nil {
		return nil, err
	}
	s := &storefront{bySlug: make(map[string]catalogApp)}
	for _, a := range apps {
		if !safeSlug.MatchString(a.Slug) || a.Name == "" {
			continue
		}
		a.Name = displayName(a)
		s.apps = append(s.apps, a)
		s.bySlug[a.Slug] = a
	}
	// Optional Flash-game catalog: a separate flash.json beside catalog.json
	// lists SWF titles that are injected into an official template channel.
	// They are free (0 points) and tagged Flash=true for the browser shop.
	if flashData, err := os.ReadFile(filepath.Join(filepath.Dir(path), "flash.json")); err == nil {
		var flashApps []catalogApp
		if err = json.Unmarshal(flashData, &flashApps); err == nil {
			for _, a := range flashApps {
				if !safeSlug.MatchString(a.Slug) || a.Name == "" {
					continue
				}
				a.Flash = true
				if a.Category == "" {
					a.Category = "Flash"
				}
				a.Name = displayName(a)
				s.apps = append(s.apps, a)
				s.bySlug[a.Slug] = a
			}
		}
	}
	s.view, err = template.ParseFS(shopFiles, "shop/page.html")
	if err != nil {
		return nil, err
	}
	s.installs = newInstallManager()
	s.channels = newChannelBuilder()
	s.channelsRoot = "./channels"
	s.iconsRoot = "./icons"
	// Operator-supplied official Flash runtime WAD used to build Flash game
	// channels. Configured with an env var so it can point anywhere.
	s.flashTemplatePath = os.Getenv("SHROOMBREW_FLASH_TEMPLATE")
	if s.flashTemplatePath == "" {
		s.flashTemplatePath = "./flash-template.wad"
	}
	return s, err
}
func (s *storefront) handler(soap http.Handler) http.Handler {
	assets, _ := fs.Sub(shopFiles, "shop")
	static := http.StripPrefix("/shop/", http.FileServer(http.FS(assets)))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		if r.Header.Get("SOAPAction") != "" {
			soap.ServeHTTP(w, r)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/api/install/") {
			s.installs.serveHTTP(w, r, s.bySlug)
			return
		}
		if r.URL.Path == "/wii-install/start" {
			s.startWiiInstall(w, r)
			return
		}
		if r.Method != "GET" && r.Method != "HEAD" {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "Method not allowed", 405)
			return
		}
		switch {
		case r.URL.Path == "/healthz":
			w.Header().Set("Content-Type", "text/plain")
			w.Write([]byte("ok\n"))
			return
		case r.URL.Path == "/source/shroombrew-source.tar.gz":
			http.ServeFile(w, r, "./shroombrew-source.tar.gz")
			return
		case r.URL.Path == "/tools/install-sd-windows.exe":
			w.Header().Set("Content-Type", "application/octet-stream")
			w.Header().Set("Content-Disposition", `attachment; filename="Shroombrew-install-sd.exe"`)
			http.ServeFile(w, r, "./downloads/Shroombrew-install-sd.exe")
			return
		case r.URL.Path == "/tools/wii-installer.zip":
			w.Header().Set("Content-Type", "application/zip")
			w.Header().Set("Content-Disposition", `attachment; filename="Shroombrew-Installer.zip"`)
			http.ServeFile(w, r, "./downloads/Shroombrew-Installer.zip")
			return
		case r.URL.Path == "/shop/shop.css" || r.URL.Path == "/shop/shop.js" || safeAsset.MatchString(r.URL.Path):
			static.ServeHTTP(w, r)
			return
		case strings.HasPrefix(r.URL.Path, "/icons/"):
			slug := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/icons/"), ".png")
			if !s.serveShopIcon(w, r, slug) {
				http.NotFound(w, r)
			}
			return
		case strings.HasPrefix(r.URL.Path, "/download/"):
			a, ok := s.bySlug[strings.TrimPrefix(r.URL.Path, "/download/")]
			if !ok {
				http.NotFound(w, r)
				return
			}
			target, ok := s.downloadTarget(a)
			if !ok {
				http.Error(w, "Download unavailable", 503)
				return
			}
			http.Redirect(w, r, target, http.StatusFound)
			return
		}
		p := shopPage{Title: "Wii Shop Channel", Screen: "catalog", Wii: strings.Contains(r.UserAgent(), "Wii Shop"), Order: r.URL.Query().Get("sort"), Section: r.URL.Query().Get("section")}
		switch p.Section {
		case "vc":
			p.SectionTitle = "Virtual Console"
		case "ww":
			p.SectionTitle = "WiiWare"
		case "channels":
			p.SectionTitle = "Wii Channels"
		default:
			p.Section = ""
			p.SectionTitle = "Homebrew"
		}
		if strings.HasPrefix(r.URL.Path, "/title/") || strings.HasPrefix(r.URL.Path, "/install/") {
			slug := strings.TrimPrefix(strings.TrimPrefix(r.URL.Path, "/title/"), "/install/")
			a, ok := s.bySlug[slug]
			if !ok {
				http.NotFound(w, r)
				return
			}
			p.App = &a
			p.Platform = a.Platform
			if p.Platform == "" {
				p.Platform = consoleLabel(a.Category)
			}
			if a.FileSize.ZipCompressed > 0 {
				p.DownloadSize = fmt.Sprintf("%.1f MB", float64(a.FileSize.ZipCompressed)/1000000)
			}
			p.Title = a.Name + " - Wii Shop Channel"
			if strings.HasPrefix(r.URL.Path, "/install/") {
				p.Screen = "install"
			} else {
				p.Screen = "detail"
			}
		} else {
			switch r.URL.Path {
			case "/", "/startup", "/oss/serv/index.jsp", "/oss/serv/W_01.jsp":
				p.Screen = "welcome"
			case "/main", "/oss/serv/W_03.jsp":
				p.Screen = "main"
			case "/browse":
				p.Screen = "browse"
			case "/search":
				p.Screen = "search"
			case "/categories":
				p.Screen = "categories"
			case "/help", "/points", "/settings", "/activity", "/news":
				p.Screen = strings.TrimPrefix(r.URL.Path, "/")
			case "/catalog":
			default:
				http.NotFound(w, r)
				return
			}
			p.Query = strings.TrimSpace(r.URL.Query().Get("q"))
			p.Category = r.URL.Query().Get("category")
			categories := map[string]bool{}
			for _, a := range s.apps {
				categories[a.Category] = true
				licensedCat := a.Category == "Virtual Console" || a.Category == "WiiWare" || a.Category == "DLC"
				if p.Section == "vc" && a.Category != "Virtual Console" {
					continue
				}
				// the WiiWare section shows licensed WiiWare/DLC plus the
				// homebrew "games" category (existing behavior)
				if p.Section == "ww" && a.Category != "WiiWare" && a.Category != "DLC" && a.Category != "games" {
					continue
				}
				// the Channels section shows the real downloadable Wii channels
				// (BBC iPlayer, Netflix, Forecast, Mii, ...), and nothing else.
				if p.Section == "channels" && a.Category != "Channels" {
					continue
				}
				// the Homebrew default view shows homebrew apps; licensed
				// VC/WiiWare/DLC titles and real Wii channels live in their own
				// shop sections
				if p.Section == "" && (licensedCat || a.Category == "Channels") {
					continue
				}
				if p.Category != "" && a.Category != p.Category {
					continue
				}
				if p.Query != "" && !strings.Contains(strings.ToLower(a.Name+" "+a.Author+" "+a.Description.Short), strings.ToLower(p.Query)) {
					continue
				}
				p.Apps = append(p.Apps, a)
			}
			for c := range categories {
				p.Categories = append(p.Categories, c)
			}
			sort.Strings(p.Categories)
			sort.SliceStable(p.Apps, func(i, j int) bool {
				if p.Order == "new" && p.Apps[i].ReleaseDate != p.Apps[j].ReleaseDate {
					return p.Apps[i].ReleaseDate > p.Apps[j].ReleaseDate
				}
				return strings.ToLower(p.Apps[i].Name) < strings.ToLower(p.Apps[j].Name)
			})
			p.Total = len(p.Apps)
			if p.Screen == "welcome" {
				featured := 4
				if p.Total < featured {
					featured = p.Total
				}
				p.Featured = append(p.Featured, p.Apps[:featured]...)
			}
			pageSize := 3
			if p.Screen == "welcome" {
				pageSize = 4
			}
			p.Pages = (p.Total + pageSize - 1) / pageSize
			if p.Pages == 0 {
				p.Pages = 1
			}
			p.Page, _ = strconv.Atoi(r.URL.Query().Get("page"))
			if p.Page < 1 {
				p.Page = 1
			}
			if p.Page > p.Pages {
				p.Page = p.Pages
			}
			start := (p.Page - 1) * pageSize
			end := start + pageSize
			if end > p.Total {
				end = p.Total
			}
			p.Apps = p.Apps[start:end]
			link := func(n int) string {
				v := url.Values{"q": {p.Query}, "category": {p.Category}, "sort": {p.Order}, "section": {p.Section}, "page": {strconv.Itoa(n)}}
				return "/catalog?" + v.Encode()
			}
			if p.Page > 1 {
				p.Prev = link(p.Page - 1)
			}
			if p.Page < p.Pages {
				p.Next = link(p.Page + 1)
			}
		}
		var b bytes.Buffer
		if err := s.view.Execute(&b, p); err != nil {
			log.Printf("shop template: %v", err)
			http.Error(w, "Shop temporarily unavailable", 500)
			return
		}
		log.Printf("shop %s path=%s wii=%t", r.Method, r.URL.Path, p.Wii)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		if r.Method != "HEAD" {
			w.Write(b.Bytes())
		}
	})
}

func (s *storefront) startWiiInstall(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 8192)
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}
	slug := r.Form.Get("slug")
	app, ok := s.bySlug[slug]
	if !ok || !safeSlug.MatchString(slug) {
		http.NotFound(w, r)
		return
	}
	s.renderWiiInstallPair(w, r, s.regionApp(app, r))
}

func (s *storefront) startWiiInstallByTitle(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		w.Header().Set("Allow", "GET, POST")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	titleID := strings.ToUpper(strings.TrimPrefix(r.URL.Path, "/wii-install/title/"))
	if len(titleID) != 16 {
		http.NotFound(w, r)
		return
	}
	app, ok := s.appByTitleID(titleID)
	if !ok {
		http.NotFound(w, r)
		return
	}
	app = s.regionApp(app, r)
	if r.Method == http.MethodGet {
		s.confirmWiiPurchase(w, r, app)
		return
	}
	s.renderWiiInstallPair(w, r, app)
}

// appByTitleID finds the catalog entry whose primary (canonical) title ID or
// any region-locked variant title ID matches. A collapsed licensed title is
// addressed by its primary title ID from the catalog page, but once the
// console's region has been applied the install/confirm pages carry that
// region-specific variant ID, so both must resolve to the same entry.
func (s *storefront) appByTitleID(titleID string) (catalogApp, bool) {
	for _, app := range s.apps {
		if classicTitleIDForApp(app) == titleID {
			return app, true
		}
		for _, v := range app.Variants {
			if v.TitleID == titleID {
				return app, true
			}
		}
	}
	return catalogApp{}, false
}

func (s *storefront) confirmWiiPurchase(w http.ResponseWriter, r *http.Request, app catalogApp) {
	id, err := walletID(w, r)
	if err != nil {
		http.Error(w, "Wallet unavailable", 500)
		return
	}
	balance, err := walletBalance(id)
	if err != nil {
		http.Error(w, "Wallet unavailable", 500)
		return
	}
	price := classicPriceFor(app)
	controllers := "See title details"
	if len(app.Peripherals) > 0 {
		controllers = strings.Join(app.Peripherals, ", ")
	}
	button := fmt.Sprintf(`<form method="post" action="/wii-install/title/%s"><p class="center">%s</p></form>`, classicTitleIDForApp(app), uiSubmit("", "", "Download"))
	if balance < price {
		button = `<p class="center err">You need more Wii Points for this title.</p><p class="center">` + uiButton("/points", "Add Wii Points") + `</p>`
	}
	content := fmt.Sprintf(`<div class="panel plain"><p class="center">%s<br><span class="blue" style="font-size:18px">%s</span></p><table class="aa" style="width:auto;margin:0 auto 10px"><tr><td style="border:0;text-align:right">Compatible controllers:</td><td style="border:0">%s</td></tr><tr><td style="border:0;text-align:right">Wii Points to Download:</td><td style="border:0">%d</td></tr><tr><td style="border:0;text-align:right">Current Wii Points:</td><td style="border:0">%d</td></tr></table>%s</div><div class="foot">%s</div>`, html.EscapeString(app.Author), html.EscapeString(app.Name), html.EscapeString(controllers), price, balance, button, uiButton("/oss/serv/B_05.html?titleId="+classicTitleIDForApp(app), "Back"))
	writeShopPage(w, "Download to SD Card", content)
}

func (s *storefront) renderWiiInstallPair(w http.ResponseWriter, r *http.Request, app catalogApp) {
	price := classicPriceFor(app)
	id := ""
	if pool != nil {
		var err error
		id, err = walletID(w, r)
		if err != nil {
			http.Error(w, "Wallet unavailable", 500)
			return
		}
		paid, err := walletSpend(id, price)
		if err != nil {
			http.Error(w, "Wallet unavailable", 500)
			return
		}
		if !paid {
			http.Redirect(w, r, "/points", http.StatusSeeOther)
			return
		}
	}
	job, err := s.installs.start(app)
	if err != nil {
		if id != "" {
			walletRefund(id, price)
		}
		http.Error(w, "This download is unavailable right now", http.StatusServiceUnavailable)
		return
	}
	page := shopPage{Screen: "pair", Title: "Download to SD Card", App: &app, InstallCode: job.Code, Wii: strings.Contains(r.UserAgent(), "Wii Shop")}
	var body bytes.Buffer
	if err := s.view.Execute(&body, page); err != nil {
		http.Error(w, "Shop temporarily unavailable", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(body.Bytes())
}
