package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPagesCORSOnlyForThePagesSite(t *testing.T) {
	for _, origin := range []string{"https://shroombrew.org", "https://www.shroombrew.org", "https://markthegamer108.github.io"} {
		pre := httptest.NewRequest(http.MethodOptions, "/community/api/link", nil)
		pre.Header.Set("Origin", origin)
		w := httptest.NewRecorder()
		if !pagesCORS(w, pre) || w.Code != http.StatusNoContent {
			t.Fatalf("preflight from %s not answered: %d", origin, w.Code)
		}
		if w.Header().Get("Access-Control-Allow-Origin") != origin ||
			!strings.Contains(w.Header().Get("Access-Control-Allow-Headers"), "X-Shroombrew") {
			t.Errorf("bad preflight headers for %s: %v", origin, w.Header())
		}
	}

	for _, origin := range []string{"https://evil.example", "https://markthegamer108.github.io.evil.example", "https://shroombrew.org.evil.example", "http://shroombrew.org", ""} {
		r := httptest.NewRequest(http.MethodOptions, "/community/api/link", nil)
		if origin != "" {
			r.Header.Set("Origin", origin)
		}
		w := httptest.NewRecorder()
		if pagesCORS(w, r) || w.Header().Get("Access-Control-Allow-Origin") != "" {
			t.Errorf("origin %q must not be allowed", origin)
		}
	}
}

func TestRemoteTokenFromCookieOrBearer(t *testing.T) {
	tok := strings.Repeat("ab", 32)
	r := httptest.NewRequest(http.MethodGet, "/community/api/me", nil)
	r.Header.Set("Authorization", "Bearer "+tok)
	if remoteToken(r) != tok {
		t.Error("bearer token not read")
	}
	r = httptest.NewRequest(http.MethodGet, "/community/api/me", nil)
	r.AddCookie(&http.Cookie{Name: remoteCookie, Value: tok})
	if remoteToken(r) != tok {
		t.Error("cookie token not read")
	}
	r = httptest.NewRequest(http.MethodGet, "/community/api/me", nil)
	r.Header.Set("Authorization", "Bearer short")
	if remoteToken(r) != "" {
		t.Error("malformed token accepted")
	}
}
