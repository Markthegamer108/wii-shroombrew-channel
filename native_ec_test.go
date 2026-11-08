package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNativeECDevelopmentGates(t *testing.T) {
	s := &storefront{}
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(418) })
	h := s.nativeECHandler(next)
	for _, tc := range []struct {
		name, enabled, host, path, agent, soap, method string
		want                                           int
	}{
		{"disabled", "", "shop.shroombrew.org", "/native-ec/", "", "", "GET", 418},
		{"native startup", "1", "shop.shroombrew.org", "/", "Wii Shop Channel/21.0(N)", "", "GET", 303},
		{"wsc startup stays on html", "1", "oss-auth.wiirw.bot.nu", "/startup", "Wii Shop Channel/21.0(N)", "", "GET", 418},
		{"previous build", "1", "shop.shroombrew.org", "/", "Wii Shop Channel/21.0(S)", "", "GET", 418},
		{"SOAP passthrough", "1", "shop.shroombrew.org", "/native-ec/", "", "PurchaseTitle", "POST", 418},
		{"empty catalog", "1", "shop.shroombrew.org", "/native-ec/", "", "", "GET", 200},
		{"reject writes", "1", "shop.shroombrew.org", "/native-ec/", "", "", "POST", 405},
		{"reject system title", "1", "shop.shroombrew.org", "/native-ec/title/0001000248414241", "", "", "GET", 404},
		{"reject missing package", "1", "shop.shroombrew.org", "/native-ec/title/0001000154455354?destination=nand", "", "", "GET", 409},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("SHROOMBREW_NATIVE_EC", tc.enabled)
			r := httptest.NewRequest(tc.method, "https://"+tc.host+tc.path, nil)
			r.Header.Set("User-Agent", tc.agent)
			r.Header.Set("SOAPAction", tc.soap)
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if w.Code != tc.want {
				t.Fatalf("got %d, want %d", w.Code, tc.want)
			}
			if tc.name == "native startup" && w.Header().Get("Location") != "/native-ec/" {
				t.Fatal("incorrect startup destination")
			}
			if tc.name == "empty catalog" && !strings.Contains(w.Body.String(), "No matching native channel packages") {
				t.Fatal("empty catalog must not offer a simulated download")
			}
		})
	}
}
