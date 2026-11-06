package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReferenceFrontendRouting(t *testing.T) {
	root := t.TempDir()
	for name, content := range map[string]string{
		"index.html":         "<title>Wii Shop Channel</title>",
		"main.js":            `goto("https://wiishopchannel.net/oss/serv/W_01.html");page.includes("wiishopchannel.net");frame.includes("wiishopchannel.net/oss/serv/W_03");`,
		"oss/serv/W_01.html": "welcome",
		"api/catalog":        "must not replace the API",
		"snd/select.mp3":     "audio-bytes",
	} {
		p := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	fallback := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(418) })
	h := referenceSiteHandler(root, fallback)
	for _, tc := range []struct {
		path, method, ua, soap string
		status                 int
	}{
		{"/", "GET", "Mozilla", "", 200},
		{"/oss/serv/W_01", "GET", "Mozilla", "", 200},
		{"/oss//serv/W_01.html", "GET", "Mozilla", "", 301},
		{"/api/catalog", "GET", "Mozilla", "", 418},
		{"/", "GET", "Wii Shop Channel/21", "", 418},
		{"/main.js", "GET", "Nintendo Wii", "", 418},
		{"/", "POST", "Mozilla", "", 418},
		{"/", "GET", "Mozilla", "GetECConfig", 418},
		{"/missing.html", "GET", "Mozilla", "", 418},
	} {
		t.Run(tc.path+tc.method+tc.ua+tc.soap, func(t *testing.T) {
			r := httptest.NewRequest(tc.method, "https://shop.example"+tc.path, nil)
			r.Header.Set("User-Agent", tc.ua)
			r.Header.Set("SOAPAction", tc.soap)
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if w.Code != tc.status {
				t.Fatalf("got %d, want %d", w.Code, tc.status)
			}
		})
	}
	routeRequest := httptest.NewRequest("GET", "https://shop.example/oss/serv/W_01.html?titleId=123&p=2", nil)
	routeResponse := httptest.NewRecorder()
	h.ServeHTTP(routeResponse, routeRequest)
	if routeResponse.Header().Get("Location") != "/oss/serv/W_01?titleId=123&p=2" {
		t.Fatal("clean URL redirect discarded catalog state")
	}
	r := httptest.NewRequest("GET", "http://shop.example/main.js", nil)
	r.Header.Set("X-Forwarded-Proto", "https")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if strings.Contains(w.Body.String(), "wiishopchannel.net") || !strings.Contains(w.Body.String(), "https://shop.example/oss/serv/") {
		t.Fatalf("frame navigation did not remain on the deployed origin: %s", w.Body.String())
	}
	r = httptest.NewRequest("GET", "https://shop.example/snd/select.mp3", nil)
	r.Header.Set("Range", "bytes=0-4")
	w = httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 206 || w.Body.String() != "audio" {
		t.Fatalf("audio range response: %d %q", w.Code, w.Body.String())
	}
}
