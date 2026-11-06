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
	h := s.handler(http.NotFoundHandler())
	// Use Wii UA so "/" serves the shop front page, not the community redirect.
	wiiUA := "Wii Shop Channel/21.0(N)"
	for _, path := range []string{"/", "/oss/serv/W_03.html", "/oss/serv/B_04.html", "/oss/serv/B_05.html"} {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", path, nil)
		req.Header.Set("User-Agent", wiiUA)
		h.ServeHTTP(w, req)
		body := w.Body.String()
		if w.Code != 200 {
			t.Fatalf("%s: status %d, body=%q", path, w.Code, body)
		}
		// The reference site's W_01/W_03 pages include these markers.
		if !strings.Contains(body, `shroombrewShopHandle`) && !strings.Contains(body, `shroombrew-welcome.js`) && !strings.Contains(body, `shroombrew-navigation.js`) {
			t.Fatalf("%s: reference site scripts missing, body[:500]=%q", path, body[:500])
		}
		if path == "/oss/serv/B_04.html" && (strings.Contains(w.Body.String(), "games.filter(e=>e)") || strings.Contains(w.Body.String(), "const urlParams = new URLSearchParams")) {
			t.Fatal("incompatible catalog renderer still served")
		}
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
