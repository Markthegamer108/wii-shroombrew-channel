package main

import (
	"html"
	"net/http"
	"os"
	"strings"

	"github.com/Markthegamer108/wii-shroombrew-channel/internal/channelpkg"
)

// writeDestinationChoice asks where to save a channel with the shop's own
// Wii-console and SD-card pictures (sdToNAND/sdToSD), as the original did.
func writeDestinationChoice(w http.ResponseWriter, r *http.Request, app catalogApp, id string) {
	lang := langFor(r)
	esc := html.EscapeString
	choice := func(dest, img, label string) string {
		return `<a class="dest" onclick="try{wiiSelectSound()}catch(e){}shroomNavigate(this.href);return false" href="/native-ec/title/` + id + `?destination=` + dest + `"` +
			` onmouseout="this.firstChild.src='` + shellImg + img + `_a.png'" onmouseover="this.firstChild.src='` + shellImg + img + `_b.png';try{wiiFocusSound()}catch(e){}"><img src="` + shellImg + img + `_a.png" width="236" height="184" alt=""><span>` + esc(label) + `</span></a>`
	}
	content := `<div class="panel plain"><p class="center">` + esc(tl(lang, "w_dest_q")) + `<br><span class="blue" style="font-size:18px">` + esc(app.Name) + `</span></p><p class="center">` +
		choice("nand", "sdToNAND", tl(lang, "w_nand")) + choice("sd", "sdToSD", tl(lang, "w_sd")) + `</p></div>` + shopFootT(lang, "/native-ec/")
	writeShopPage(w, tl(lang, "w_dest_title"), content)
}

// Native EC is an explicit development mode. It uses the console's EC APIs,
// rather than the browser wallet or server-side ZIP preparation APIs.
func (s *storefront) nativeECHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if os.Getenv("SHROOMBREW_NATIVE_EC") != "1" || r.Header.Get("SOAPAction") != "" {
			next.ServeHTTP(w, r)
			return
		}
		if (r.URL.Path == "/" || r.URL.Path == "/startup") && strings.Contains(r.UserAgent(), "Wii Shop Channel/21.0(N)") {
			http.Redirect(w, r, "/native-ec/", http.StatusSeeOther)
			return
		}
		if !strings.HasPrefix(r.URL.Path, "/native-ec/") {
			next.ServeHTTP(w, r)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("X-Shroombrew-Frontend", "native-ec-development")
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", "GET")
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if r.URL.Path == "/native-ec/" {
			// A ZIP catalog record is not enough to offer native installation.
			// Only list imported, validated channel packages matching its metadata.
			// Rows are the shop's software list rows (B_04); each opens the
			// destination choice for that title.
			lang := langFor(r)
			content := `<div class="panel">`
			count := 0
			for _, app := range s.apps {
				id := classicTitleIDForApp(app)
				manifest, err := channelpkg.Load(s.channelsRoot, strings.ToLower(id))
				if err != nil || !channelPackageMatches(app, manifest) {
					continue
				}
				href, sub := "/native-ec/title/"+id, tl(lang, "w_dl_choose")
				if manifest.Kind == "installer" {
					if manifest.SDApp != app.Slug {
						continue
					}
					href, sub = href+"?destination=apps", tl(lang, "n_q_apps")
				}
				content += uiRow(href, "/icons/"+app.Slug+".png", app.Name, sub)
				count++
			}
			if count == 0 {
				content += `<p>No matching native channel packages are ready yet. ` + html.EscapeString(tl(lang, "w_dl_empty")) + `</p>`
			}
			writeShopPage(w, tl(lang, "w_dl_title"), content+"</div>"+shopFootT(lang, "/oss/serv/W_03.html"))
			return
		}
		id := strings.ToUpper(strings.TrimPrefix(r.URL.Path, "/native-ec/title/"))
		if !strings.HasPrefix(r.URL.Path, "/native-ec/title/") || !channelpkg.TitlePattern.MatchString(id) {
			http.NotFound(w, r)
			return
		}
		var app *catalogApp
		for i := range s.apps {
			if classicTitleIDForApp(s.apps[i]) == id {
				app = &s.apps[i]
				break
			}
		}
		if app == nil {
			// A system title (00010002...) is not a catalogue page at all; a
			// catalogue-prefix title with no matching package is a conflict.
			if strings.HasPrefix(id, "00010002") {
				http.NotFound(w, r)
				return
			}
			http.Error(w, "A matching native channel package is required", http.StatusConflict)
			return
		}
		manifest, err := channelpkg.Load(s.channelsRoot, strings.ToLower(id))
		if err != nil || !channelPackageMatches(*app, manifest) {
			http.Error(w, "A matching native channel package is required", http.StatusConflict)
			return
		}
		destination := r.URL.Query().Get("destination")
		if manifest.Kind == "installer" {
			if destination != "apps" {
				http.Error(w, "This package does not support the selected installation mode", http.StatusConflict)
				return
			}
		} else if destination == "" {
			writeDestinationChoice(w, r, *app, id)
			return
		} else if destination != "sd" && destination != "nand" {
			http.Error(w, "Choose nand or sd", http.StatusBadRequest)
			return
		}
		writeArchivedCheckout(w, r, archivedCheckoutData{Page: "B_10", Mode: "native", Name: app.Name, Back: "/oss/serv/B_05.html?titleId=" + classicTitleIDForApp(*app), Health: healthSafetyURL(*app), TitleID: id, Destination: destination, ServiceDomain: shopBaseDomain(r.Host)})
	})
}
