package main

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestWiiInstallPageCreatesPairCode(t *testing.T) {
	s := testShop(t)
	s.installs.root = t.TempDir()
	archive := testPackage(t, map[string]string{"apps/demo/boot.dol": "test"})
	s.installs.client = &http.Client{Transport: installRoundTrip(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewReader(archive)), ContentLength: int64(len(archive)), Header: make(http.Header)}, nil
	})}
	r := httptest.NewRequest(http.MethodPost, "/wii-install/start", strings.NewReader("slug=demo"))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.Header.Set("User-Agent", "Wii Shop Channel/21.0(A)")
	w := httptest.NewRecorder()
	s.handler(http.NotFoundHandler()).ServeHTTP(w, r)
	if w.Code != 200 || !strings.Contains(w.Body.String(), `id="install-code"`) || !strings.Contains(w.Body.String(), "Continue on Your Wii") {
		t.Fatalf("pair page failed: %d %s", w.Code, w.Body.String())
	}
	// The pair response is immediate; wait for its background extraction before
	// TempDir cleanup so the test does not race the worker on disk.
	deadline := time.Now().Add(time.Second)
	for {
		s.installs.mu.RLock()
		var job *installJob
		for _, candidate := range s.installs.jobs {
			job = candidate
			break
		}
		s.installs.mu.RUnlock()
		if job != nil {
			job.mu.RLock()
			state := job.State
			job.mu.RUnlock()
			if state == "ready" || state == "failed" {
				break
			}
		}
		if time.Now().After(deadline) {
			t.Fatal("download preparation did not finish")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func testShop(t *testing.T) *storefront {
	t.Helper()
	p := filepath.Join(t.TempDir(), "catalog.json")
	err := os.WriteFile(p, []byte(`[{"slug":"demo","name":"Demo <script>","author":"Author","category":"games","description":{"short":"Fun","long":"<script>alert(1)</script>"},"url":{"zip":"https://hbb1.oscwii.org/api/contents/demo/demo.zip"},"shop":{"title_id":"0001000144454d4f","title_version":1}},{"slug":"badurl","name":"Bad","category":"utilities","url":{"zip":"https://example.com/private"}}]`), 0600)
	if err != nil {
		t.Fatal(err)
	}
	s, err := newStorefront(p)
	if err != nil {
		t.Fatal(err)
	}
	return s
}
func TestStorefrontRoutes(t *testing.T) {
	s := testShop(t)
	called := false
	h := s.handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { called = true; w.WriteHeader(204) }))
	for _, tc := range []struct {
		path string
		code int
		text string
	}{{"/", 200, "Start Shopping"}, {"/startup", 200, "Start Shopping"}, {"/oss/serv/index.jsp", 200, "Start Shopping"}, {"/catalog?q=demo", 200, "1 apps"}, {"/catalog?category=games&page=999999", 200, "1/1"}, {"/catalog?q=nothing", 200, "No apps match"}, {"/title/demo", 200, "&lt;script&gt;alert(1)&lt;/script&gt;"}, {"/install/demo", 200, "Get SD Installer"}, {"/title/missing", 404, ""}, {"/not-a-shop-page", 404, ""}, {"/download/demo", 302, ""}, {"/download/badurl", 503, ""}, {"/icons/../../config.xml", 404, ""}, {"/shop/page.html", 404, ""}} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", tc.path, nil))
		if w.Code != tc.code || !strings.Contains(w.Body.String(), tc.text) {
			t.Errorf("%s: status=%d body=%s", tc.path, w.Code, w.Body.String())
		}
	}
	welcome := httptest.NewRecorder()
	h.ServeHTTP(welcome, httptest.NewRequest("GET", "/", nil))
	if !strings.Contains(welcome.Body.String(), "welcome-recommendations") || !strings.Contains(welcome.Body.String(), "/title/demo") {
		t.Fatal("welcome page did not render catalog-backed recommendations")
	}
	req := httptest.NewRequest("POST", "/ecs/ECommerceSOAP", strings.NewReader("soap"))
	req.Header.Set("SOAPAction", `"urn:ecs.wsapi.broadon.com/GetECConfig"`)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if !called || w.Code != 204 {
		t.Fatal("SOAP was not delegated")
	}
	w = httptest.NewRecorder()
	req = httptest.NewRequest("GET", "/title/demo", nil)
	req.Header.Set("User-Agent", "Opera Wii Shop Channel")
	h.ServeHTTP(w, req)
	if strings.Contains(w.Body.String(), ">Download ZIP</a>") {
		t.Fatal("Wii was offered unsupported ZIP install")
	}
}
func TestSOAPActionAndGetECConfig(t *testing.T) {
	for _, a := range []string{"urn:ecs.wsapi.broadon.com/GetECConfig", `"urn:ecs.wsapi.broadon.com/GetECConfig"`, `  "urn:ecs.wsapi.broadon.com/GetECConfig"  `} {
		s, n := parseAction(a)
		if s != "ecs" || n != "GetECConfig" {
			t.Fatalf("rejected valid action %q", a)
		}
	}
	for _, a := range []string{`"urn:ecs.wsapi.broadon.com/GetECConfig`, "urn:ecs.wsapi.broadon.com/GetECConfig/extra"} {
		s, _ := parseAction(a)
		if s != "" {
			t.Fatalf("accepted malformed action %q", a)
		}
	}
	r := NewRoute()
	group := r.HandleGroup("ecs")
	group.Unauthenticated("GetECConfig", getECConfig)
	body := `<soap:Envelope xmlns:soap="http://schemas.xmlsoap.org/soap/envelope/" xmlns:ecs="urn:ecs.wsapi.broadon.com"><soap:Body><ecs:GetECConfig><ecs:Version>2.0</ecs:Version><ecs:DeviceId>12345</ecs:DeviceId><ecs:MessageId>test</ecs:MessageId><ecs:Region>EUR</ecs:Region><ecs:Country>DE</ecs:Country><ecs:Language>en</ecs:Language></ecs:GetECConfig></soap:Body></soap:Envelope>`
	req := httptest.NewRequest("POST", "/ecs/services/ECommerceSOAP", strings.NewReader(body))
	req.Header.Set("SOAPAction", `"urn:ecs.wsapi.broadon.com/GetECConfig"`)
	w := httptest.NewRecorder()
	r.Handle().ServeHTTP(w, req)
	if w.Code != 200 || !strings.Contains(w.Body.String(), "GetECConfigResponse") {
		t.Fatalf("SOAP request failed: %d %s", w.Code, w.Body.String())
	}
}

func TestSOAPErrorIsHTTP200(t *testing.T) {
	r := NewRoute()
	group := r.HandleGroup("ecs")
	group.Unauthenticated("FailTest", func(e *Envelope) { e.Error(2, "test failure", nil) })
	body := `<soap:Envelope xmlns:soap="http://schemas.xmlsoap.org/soap/envelope/" xmlns:ecs="urn:ecs.wsapi.broadon.com"><soap:Body><ecs:FailTest><ecs:Version>2.0</ecs:Version><ecs:DeviceId>12345</ecs:DeviceId><ecs:MessageId>test</ecs:MessageId><ecs:Region>EUR</ecs:Region><ecs:Country>DE</ecs:Country><ecs:Language>en</ecs:Language></ecs:FailTest></soap:Body></soap:Envelope>`
	req := httptest.NewRequest("POST", "/ecs/services/ECommerceSOAP", strings.NewReader(body))
	req.Header.Set("SOAPAction", `"urn:ecs.wsapi.broadon.com/FailTest"`)
	w := httptest.NewRecorder()
	r.Handle().ServeHTTP(w, req)
	// The console must be able to read ErrorCode from the body; a bare HTTP
	// 500 or 401 is shown as 204800/204701 instead of the shop's 205002.
	if w.Code != 200 || !strings.Contains(w.Body.String(), "<ErrorCode>2</ErrorCode>") {
		t.Fatalf("SOAP error reply: %d %s", w.Code, w.Body.String())
	}
}

func TestDisplayNameRepairsDescriptions(t *testing.T) {
	for _, tc := range []struct{ slug, name, short, want string }{
		{"OceanPop", "A relaxing match 3 puzzle game", "A relaxing match 3 puzzle game", "OceanPop"},
		{"seeprom-viewer", "SEEPROM Viewer", "SEEPROM Viewer", "SEEPROM Viewer"},
		{"wiimc", "WiiMC", "WiiMC", "WiiMC"},
		{"demo", "Demo app", "Something else", "Demo app"},
	} {
		a := catalogApp{Slug: tc.slug, Name: tc.name}
		a.Description.Short = tc.short
		if got := displayName(a); got != tc.want {
			t.Errorf("%s: got %q want %q", tc.slug, got, tc.want)
		}
	}
}

func TestShellPanelStaysAboveBottomRule(t *testing.T) {
	// The list frame starts at 88px and plain pages at 80px; the bottom
	// dotted rule is at 363px, so both must end by 356px.
	if !strings.Contains(shellStyle, "top:88px;width:540px;height:268px") || !strings.Contains(shellStyle, "top:80px;width:540px;height:276px") {
		t.Fatal("shop panel must end above the bottom dotted rule")
	}
}

func TestClassicShopRoutes(t *testing.T) {
	s := testShop(t)
	h := s.handler(http.NotFoundHandler())
	for _, tc := range []struct{ path, text string }{{"/main", "Virtual Console"}, {"/browse?section=ww", "WiiWare"}, {"/catalog?section=vc", "No apps match"}, {"/catalog?section=ww", "Demo &lt;script&gt;"}, {"/search", "search-form"}, {"/help", "Shopping Guide"}, {"/points", "All homebrew titles are free"}, {"/settings", "sound-setting"}, {"/shop/shop.js", "connectingMessage = shroomShop.connecting"}} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", tc.path, nil))
		if w.Code != 200 || !strings.Contains(w.Body.String(), tc.text) {
			t.Errorf("%s: %d %s", tc.path, w.Code, w.Body.String())
		}
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/shop/assets/top_buy_vc_a.png", nil))
	if w.Code != 200 || w.Header().Get("Content-Type") != "image/png" {
		t.Fatal("original image asset unavailable")
	}
}
