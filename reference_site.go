package main

import (
	"bytes"
	"crypto/sha1"
	"fmt"
	"mime"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
)

// referenceSiteHandler serves the permissioned, unmodified browser recreation
// before the older Shroombrew page generators. The original files and their
// hashes live separately so catalog injection and Wii compatibility transforms
// cannot change the browser reference's layout or behavior.
//
// The reference emulates Wii APIs using modern browser JavaScript. Native Shop
// clients still need the existing native frontend and its real server APIs.
func referenceSiteHandler(root string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ua := strings.ToLower(r.UserAgent())
		if r.Header.Get("SOAPAction") != "" ||
			(r.Method != http.MethodGet && r.Method != http.MethodHead) ||
			strings.Contains(ua, "wii shop channel/") || strings.Contains(ua, "nintendo wii") {
			next.ServeHTTP(w, r)
			return
		}

		requestPath := path.Clean("/" + r.URL.Path)
		// Reserve server routes even if a future reference download happens to
		// contain a matching filename.
		for _, prefix := range []string{"/api/", "/download/", "/shop-download/", "/wii-install/", "/source/", "/tools/", "/icons/", "/shop/", "/shroombrew-"} {
			if strings.HasPrefix(requestPath, prefix) {
				next.ServeHTTP(w, r)
				return
			}
		}

		filePath := filepath.Join(root, filepath.FromSlash(strings.TrimPrefix(requestPath, "/")))
		info, err := os.Stat(filePath)
		if err == nil && info.IsDir() {
			filePath = filepath.Join(filePath, "index.html")
			info, err = os.Stat(filePath)
		} else if os.IsNotExist(err) && filepath.Ext(requestPath) == "" {
			filePath += ".html"
			info, err = os.Stat(filePath)
		}
		if err != nil || info.IsDir() || !info.Mode().IsRegular() {
			next.ServeHTTP(w, r)
			return
		}
		// Match the reference host's clean HTML URLs. Query strings remain
		// intact because they carry catalog filters and selected title IDs.
		if strings.HasSuffix(requestPath, ".html") {
			destination := strings.TrimSuffix(requestPath, ".html")
			if path.Base(requestPath) == "index.html" {
				destination = strings.TrimSuffix(requestPath, "index.html")
			}
			if r.URL.RawQuery != "" {
				destination += "?" + r.URL.RawQuery
			}
			http.Redirect(w, r, destination, http.StatusMovedPermanently)
			return
		}

		w.Header().Set("X-Content-Type-Options", "nosniff")
		ext := strings.ToLower(filepath.Ext(filePath))
		switch ext {
		case ".html", ".js", ".css", ".json", ".xml":
			body, err := os.ReadFile(filePath)
			if err != nil {
				http.Error(w, "Unable to read frontend", http.StatusInternalServerError)
				return
			}
			// The reference hard-codes its own origin in iframe navigation and
			// frame URL checks. Rebase only those origins/checks, retaining all
			// markup, styles, scripts, catalog data and media as downloaded.
			scheme := "http"
			if r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https") {
				scheme = "https"
			}
			origin := scheme + "://" + r.Host
			text := string(body)
			for _, source := range []string{"https://wiishopchannel.net", "http://wiishopchannel.net", "https://wiishopchannel.neocities.org", "http://wiishopchannel.neocities.org"} {
				text = strings.ReplaceAll(text, source, origin)
			}
			text = strings.ReplaceAll(text, "wiishopchannel.net/oss/serv/", r.Host+"/oss/serv/")
			text = strings.ReplaceAll(text, `page.includes("wiishopchannel.net")`, `page.includes("`+r.Host+`")`)
			// Seasonal tracks are scheduled by the shell's main.js. Snow and the
			// reversed-text day come from the shared effects script.
			if requestPath == "/" && ext == ".html" {
				text = strings.Replace(text, "</body>", `<script src="/shroombrew-fx.js"></script></body>`, 1)
			}
			w.Header().Set("Content-Type", mime.TypeByExtension(ext))
			// Keep deployments fresh, including the shell's runtime state.
			w.Header().Set("Cache-Control", "no-cache")
			// Validate on the bytes actually sent: the file's date would let a
			// browser keep a copy from before a rewrite changed.
			sum := sha1.Sum([]byte(text))
			w.Header().Set("ETag", fmt.Sprintf("\"%x\"", sum[:8]))
			http.ServeContent(w, r, filepath.Base(filePath), time.Time{}, bytes.NewReader([]byte(text)))
		default:
			w.Header().Set("Cache-Control", "public, max-age=3600")
			// ServeContent handles range requests for the original audio.
			f, err := os.Open(filePath)
			if err != nil {
				http.NotFound(w, r)
				return
			}
			defer f.Close()
			http.ServeContent(w, r, filepath.Base(filePath), info.ModTime(), f)
		}
	})
}
