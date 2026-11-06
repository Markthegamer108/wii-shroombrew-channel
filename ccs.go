package main

import (
	"github.com/Markthegamer108/wii-shroombrew-channel/internal/channelpkg"
	"net/http"
	"path/filepath"
	"strings"
)

// nativeContentHandler exposes only validated package contents, never tickets or arbitrary files.
func nativeContentHandler(root string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/ccs/download/") {
			next.ServeHTTP(w, r)
			return
		}
		w.Header().Set("X-Content-Type-Options", "nosniff")
		if r.Method != "GET" && r.Method != "HEAD" {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "Method not allowed", 405)
			return
		}
		parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/ccs/download/"), "/")
		if len(parts) != 2 {
			http.NotFound(w, r)
			return
		}
		m, e := channelpkg.Load(root, strings.ToLower(parts[0]))
		if e != nil || !m.Allows(parts[1]) {
			http.NotFound(w, r)
			return
		}
		// Packages are stored with lower-case names. The console may ask for
		// "TMD" or an upper-case content ID, so fold the name after validating
		// it against the manifest.
		parts[1] = strings.ToLower(parts[1])
		w.Header().Set("Content-Type", "application/octet-stream")
		if parts[1] == "tmd" {
			w.Header().Set("Cache-Control", "no-cache")
		} else {
			w.Header().Set("Cache-Control", "public, max-age=3600")
		}
		http.ServeFile(w, r, filepath.Join(root, m.TitleID, parts[1]))
	})
}
