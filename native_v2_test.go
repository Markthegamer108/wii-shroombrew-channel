package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestNativeV2BuildSelection(t *testing.T) {
	if _, err := os.Stat("wii-site/oss/serv/W_01.html"); err != nil {
		t.Skip("generated native frontend not in this checkout")
	}
	s := &storefront{}
	h := s.nativeV2Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(418) }))
	for _, tc := range []struct {
		agent, soap string
		want        int
	}{{nativeV2Agent, "", 200}, {"Wii Shop Channel/21.0(A)", "", 418}, {"Mozilla/5.0", "", 418}, {nativeV2Agent, "GetECConfig", 418}} {
		r := httptest.NewRequest("GET", "https://oss-auth.wiirw.bot.nu/", nil)
		r.Header.Set("User-Agent", tc.agent)
		r.Header.Set("SOAPAction", tc.soap)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != tc.want {
			t.Fatalf("agent=%s SOAP=%s: got %d", tc.agent, tc.soap, w.Code)
		}
		if tc.want == 200 {
			if !strings.Contains(w.Body.String(), "/native-v2/bridge.js") || w.Header().Get("Location") != "" {
				t.Fatal("new client must receive the welcome HTML directly")
			}
		}
	}
}

func TestNativeV2CatalogSizeAndFields(t *testing.T) {
	// catalog.json only exists on the server; skip if missing.
	if _, err := os.Stat("catalog.json"); err != nil {
		t.Skip("catalog.json not found")
	}
	s, err := newStorefront("catalog.json")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		page, region string
		max          int
	}{{"W_01", "US", 21}, {"B_04", "EU", len(s.apps) + 1}} {
		body := s.nativeV2Games(tc.page, tc.region)
		var titles []classicTitle
		if err := json.Unmarshal(body[len("var gameslist="):len(body)-1], &titles); err != nil {
			t.Fatal(err)
		}
		if len(titles) < 2 || len(titles) > tc.max {
			t.Fatalf("unexpected %d titles", len(titles))
		}
		for _, g := range titles[1:] {
			if g.Region != tc.region || g.Thumbnail != "FFFD0001" {
				t.Fatal("invalid native title fields")
			}
		}
	}
}
