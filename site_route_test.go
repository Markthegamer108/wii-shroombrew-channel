package main

import (
	"net/http/httptest"
	"testing"
)

func TestRewriteSiteRoute(t *testing.T) {
	cases := []struct {
		in, out  string
		redirect string
	}{
		{"/", "/", ""},
		{"/shroombrew", "", "/shroombrew/"},
		{"/shroombrew/", "/community/index.html", ""},
		{"/shroombrew/cards.html", "/community/cards.html", ""},
		{"/shroombrew/lang/en.json", "/community/lang/en.json", ""},
		{"/community/", "/community/", ""},
		{"/wiiwave/", "/wiiwave/", ""},
		{"/ravage/", "/ravage/", ""},
		{"/healthz", "/healthz", ""},
		{"/oss/serv/W_01.html", "/oss/serv/W_01.html", ""},
	}
	for _, c := range cases {
		w := httptest.NewRecorder()
		r := httptest.NewRequest("GET", c.in, nil)
		done := rewriteSiteRoute(w, r)
		if c.redirect != "" {
			if !done || w.Header().Get("Location") != c.redirect {
				t.Errorf("%s: want redirect to %s, got done=%v location=%q", c.in, c.redirect, done, w.Header().Get("Location"))
			}
			continue
		}
		if done || r.URL.Path != c.out {
			t.Errorf("%s: want path %s, got %s (done=%v)", c.in, c.out, r.URL.Path, done)
		}
	}
}

func TestRewriteSiteRouteBlocksTraversal(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/shroombrew/a", nil)
	r.URL.Path = "/shroombrew/../secrets.key"
	if !rewriteSiteRoute(w, r) || w.Code != 404 {
		t.Fatalf("traversal not blocked: code=%d", w.Code)
	}
}
