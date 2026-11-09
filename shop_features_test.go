package main

import (
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRepairedFrontendRoutes(t *testing.T) {
	s := testShop(t)
	// Use the full handler chain (reference site + feature + classic + SOAP)
	// so that /oss/serv/* routes are served by the reference site.
	// The reference site requires ./reference-site to exist; skip if missing.
	if _, err := os.Stat("./reference-site"); err != nil {
		t.Skip("reference-site directory not found")
	}
	// Match the production chain: reference site -> feature routes -> classic
	// shop pages. s.handler alone only covers the homebrew storefront.
	h := referenceSiteHandler("./reference-site", s.featureHandler(s.classicSiteHandler(s.handler(http.NotFoundHandler()))))
	// Use Wii UA so "/" serves the shop front page, not the community redirect.
	wiiUA := "Wii Shop Channel/21.0(N)"
	for _, path := range []string{"/oss/serv/W_03.html", "/oss/serv/B_04.html", "/oss/serv/B_05.html"} {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", path, nil)
		req.Header.Set("User-Agent", wiiUA)
		h.ServeHTTP(w, req)
		body := w.Body.String()
		if w.Code != 200 {
			t.Fatalf("%s: status %d, body=%q", path, w.Code, body)
		}
		// Each archived page gets at least one of the Shroombrew adapters.
		if !strings.Contains(body, `shroombrewShopHandle`) && !strings.Contains(body, `shroombrew-welcome.js`) &&
			!strings.Contains(body, `shroombrew-navigation.js`) && !strings.Contains(body, `shroombrew-catalog.js`) &&
			!strings.Contains(body, `shroombrew-detail.js`) {
			t.Fatalf("%s: reference site scripts missing, body[:500]=%q", path, body[:500])
		}
		if path == "/oss/serv/B_04.html" && (strings.Contains(w.Body.String(), "games.filter(e=>e)") || strings.Contains(w.Body.String(), "const urlParams = new URLSearchParams")) {
			t.Fatal("incompatible catalog renderer still served")
		}
	}
	// A Wii starts at the connecting screen, not directly on the shop front page.
	start := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("User-Agent", wiiUA)
	h.ServeHTTP(start, req)
	if start.Code != 200 || !strings.Contains(start.Body.String(), "Connecting. Please wait...") {
		t.Fatalf("startup: status %d, body=%q", start.Code, start.Body.String())
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/download/demo", nil))
	if w.Code != 303 || !strings.HasPrefix(w.Header().Get("Location"), "/wii-install/title/") {
		t.Fatal("download bypasses checkout")
	}
	w = httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("POST", "/api/install/native/demo", nil))
	if w.Code != 409 {
		t.Fatalf("experimental native route exposed: %d", w.Code)
	}
	// The real-Wii WAD reaches the HTML entry point on its patched host.
	t.Setenv("SHROOMBREW_NATIVE_EC", "1")
	ecChain := s.nativeECHandler(h)
	for _, tc := range []struct {
		host, path string
		code       int
		text       string
	}{
		{"oss-auth.wiirw.bot.nu", "/startup", 302, ""},
		{"shop.shroombrew.org", "/startup", 303, ""},
		{"oss-auth.wiirw.bot.nu", "/oss/getLog", 200, ""},
	} {
		w = httptest.NewRecorder()
		r := httptest.NewRequest("GET", "https://"+tc.host+tc.path, nil)
		r.Header.Set("User-Agent", wiiUA)
		ecChain.ServeHTTP(w, r)
		if w.Code != tc.code {
			t.Fatalf("%s%s: status %d, want %d", tc.host, tc.path, w.Code, tc.code)
		}
		if tc.text != "" && !strings.Contains(w.Body.String(), tc.text) {
			t.Fatalf("%s%s: missing %q", tc.host, tc.path, tc.text)
		}
	}
	// Manual launches from the channel go to the archived operations guide.
	w = httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/startup?initpage=showManual&titleId=0001000158483245", nil)
	r.Header.Set("User-Agent", wiiUA)
	h.ServeHTTP(w, r)
	if w.Code != http.StatusFound || !strings.HasPrefix(w.Header().Get("Location"), "/manual?titleId=") {
		t.Fatalf("manual launch: %d %q", w.Code, w.Header().Get("Location"))
	}
	// The native registration chain must be complete for ECS to issue a token.
	for _, path := range []string{"/oss/serv/CheckRegistered.html", "/oss/serv/L_03.html", "/oss/serv/Register.html", "/oss/serv/CheckBalance.html"} {
		w = httptest.NewRecorder()
		r := httptest.NewRequest("GET", path, nil)
		r.Header.Set("User-Agent", wiiUA)
		h.ServeHTTP(w, r)
		if w.Code != 200 {
			t.Fatalf("%s: status %d", path, w.Code)
		}
	}
	w = httptest.NewRecorder()
	r = httptest.NewRequest("GET", "/oss/serv/CheckRegistered.html", nil)
	r.Header.Set("User-Agent", wiiUA)
	h.ServeHTTP(w, r)
	if !strings.Contains(w.Body.String(), "Connecting") {
		t.Fatal("CheckRegistered page lost its connecting screen")
	}
	// The old /wad page is gone; the WAD is a direct download route.
	w = httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/wad", nil))
	if w.Code != 404 {
		t.Fatalf("old wad page still served: %d", w.Code)
	}
	w = httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("POST", "/wii-install/start", nil))
	if w.Code != 409 {
		t.Fatalf("legacy pairing route exposed: %d", w.Code)
	}
}

func TestPreparedPackageIntegrity(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "extracted", "apps", "demo", "boot.dol")
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	data := []byte("verified app")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	job := &installJob{Workspace: root, Manifest: installManifest{Slug: "demo", Files: []installFile{{Path: "apps/demo/boot.dol", Size: int64(len(data)), SHA256: fmt.Sprintf("%x", sha256.Sum256(data))}}}}
	if err := verifyPreparedJob(job); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("tampered app"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := verifyPreparedJob(job); err == nil {
		t.Fatal("accepted a tampered app")
	}
	job.Manifest.Files[0].Path = "../outside"
	if err := verifyPreparedJob(job); err == nil {
		t.Fatal("accepted path traversal")
	}
}

func TestShopPostProtection(t *testing.T) {
	for _, tc := range []struct {
		origin, token string
		valid         bool
	}{
		{"https://shop.example", csrfToken("wallet"), true},
		{"https://other.example", csrfToken("wallet"), false},
		{"https://shop.example", "bad", false},
	} {
		r := httptest.NewRequest("POST", "https://shop.example/points", strings.NewReader("csrf="+tc.token))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		r.Header.Set("Origin", tc.origin)
		if validShopPost(r, "wallet") != tc.valid {
			t.Fatal("invalid POST validation")
		}
	}
}
