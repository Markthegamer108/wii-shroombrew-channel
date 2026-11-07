package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
)

const nativeV2Agent = "Wii Shop Channel/21.0(S)"

func (s *storefront) nativeV2Games(page, region string) []byte {
	titles := s.classicTitles()
	if page == "W_01" && len(titles) > 21 {
		titles = titles[:21]
	}
	for i := 1; i < len(titles); i++ {
		g := &titles[i]
		g.Region = region
		g.Thumbnail = "FFFD0001"
		switch strings.ToLower(g.Genre) {
		case "games":
			g.Console = "WiiWare"
		case "emulators":
			g.Console = "Homebrew"
		default:
			g.Console = "Wii Channels"
		}
		if i <= 20 {
			g.Attributes = "popular"
		}
		if g.ReleaseUnix > 0 {
			g.Date = time.Unix(g.ReleaseUnix, 0).UTC().Format("01/02/2006")
			g.Added = g.Date
		}
	}
	b, _ := json.Marshal(titles)
	return append([]byte("var gameslist="), append(b, ';')...)
}

// Only the new WAD's explicit user-agent marker selects this frontend.
// Unmodified and earlier clients continue to use their existing route.
func (s *storefront) nativeV2Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("SOAPAction") != "" || (!strings.Contains(r.UserAgent(), nativeV2Agent) && !strings.HasPrefix(r.URL.Path, "/native-v2/")) {
			next.ServeHTTP(w, r)
			return
		}
		w.Header().Set("X-Shroombrew-Frontend", "native-v2-20261017")
		w.Header().Set("Cache-Control", "no-store, max-age=0")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Add("Vary", "User-Agent")
		switch r.URL.Path {
		case "/native-v2/state.js":
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
			w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
			token, _ := json.Marshal(csrfToken(id))
			fmt.Fprintf(w, "points=%d;var nativeV2CSRF=%s;", balance, token)
			return
		case "/native-v2/points":
			id, err := walletID(w, r)
			if err != nil {
				http.Error(w, "Wallet unavailable", 500)
				return
			}
			if !validShopPost(r, id) {
				http.Error(w, "Reload the page and try again", 403)
				return
			}
			balance, err := walletAdd(id)
			if err != nil {
				http.Error(w, "Wallet unavailable", 500)
				return
			}
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			fmt.Fprint(w, balance)
			return
		case "/native-v2/games.js", "/oss/serv/games.js":
			region := "US"
			if c, e := r.Cookie("nativev2_region"); e == nil {
				if v, e := url.QueryUnescape(c.Value); e == nil && (v == "US" || v == "EU" || v == "JP" || v == "KR") {
					region = v
				}
			}
			w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
			w.Write(s.nativeV2Games(r.URL.Query().Get("page"), region))
			return
		case "/native-v2/home.js", "/oss/serv/home.js":
			w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
			w.Write(s.classicHomeJS(r))
			return
		case "/native-v2/sound.js", "/oss/oss/common/js/sound.js":
			w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
			w.Write(classicSoundJS())
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			next.ServeHTTP(w, r)
			return
		}
		name := path.Clean("/" + r.URL.Path)
		switch name {
		case "/font2.ttf":
			name = "/oss/serv/font2.ttf"
		case "/", "/startup":
			name = "/oss/serv/W_01.html"
		case "/main":
			name = "/oss/serv/W_03.html"
		case "/settings":
			name = "/oss/serv/S_01.html"
		case "/news":
			name = "/oss/serv/W_02.html"
		case "/browse", "/catalog", "/search":
			name = "/oss/serv/B_04.html"
		case "/categories":
			name = "/oss/serv/B_23.html"
		case "/native-v2/bridge.js":
			name = "/bridge.js"
		}
		if strings.HasPrefix(name, "/oss/ccs/") {
			next.ServeHTTP(w, r)
			return
		}
		if strings.HasPrefix(name, "/oss/serv/") {
			if strings.HasSuffix(name, ".jsp") {
				name = strings.TrimSuffix(name, ".jsp") + ".html"
			}
			if filepath.Ext(name) == "" {
				name += ".html"
			}
		}
		local := filepath.Join("./wii-site", filepath.FromSlash(strings.TrimPrefix(name, "/")))
		info, err := os.Stat(local)
		if err != nil || info.IsDir() {
			// Art and fonts retain the downloaded reference bytes. The native
			// homebrew title-icon route continues to resolve through the catalog.
			local = filepath.Join("./reference-site", filepath.FromSlash(strings.TrimPrefix(name, "/")))
			info, err = os.Stat(local)
		}
		if err != nil || info.IsDir() {
			next.ServeHTTP(w, r)
			return
		}
		ext := strings.ToLower(filepath.Ext(local))
		if ext == ".html" || ext == ".js" || ext == ".css" {
			b, err := os.ReadFile(local)
			if err != nil {
				http.NotFound(w, r)
				return
			}
			text := string(b)
			for _, origin := range []string{"https://wiishopchannel.net", "http://wiishopchannel.net", "https://wiishopchannel.neocities.org", "http://wiishopchannel.neocities.org"} {
				text = strings.ReplaceAll(text, origin, "")
			}
			text = shopServiceHosts(text, shopBaseDomain(r.Host))
			w.Header().Set("Content-Type", mime.TypeByExtension(ext))
			http.ServeContent(w, r, filepath.Base(local), info.ModTime(), bytes.NewReader([]byte(text)))
			return
		}
		http.ServeFile(w, r, local)
	})
}
