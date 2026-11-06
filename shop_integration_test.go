package main

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"os"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v4/pgxpool"
)

func TestWalletAndVerifiedDownloadIntegration(t *testing.T) {
	configPath := os.Getenv("SHOP_INTEGRATION_CONFIG")
	if configPath == "" {
		t.Skip("set SHOP_INTEGRATION_CONFIG for a disposable-wallet database check")
	}
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	var cfg Config
	if err = xml.Unmarshal(data, &cfg); err != nil {
		t.Fatal(err)
	}
	u := &url.URL{Scheme: "postgres", User: url.UserPassword(cfg.SQLUser, cfg.SQLPass), Host: cfg.SQLAddress, Path: "/" + cfg.SQLDB}
	testPool, err := pgxpool.Connect(ctx, u.String())
	if err != nil {
		t.Fatal("database unavailable")
	}
	beforePool := pool
	pool = testPool
	t.Cleanup(func() { pool = beforePool; testPool.Close() })
	t.Setenv("SHROOMBREW_SECRET_FILE", t.TempDir()+"/secrets.key")
	if err = initSecrets(); err != nil {
		t.Fatal(err)
	}
	if err = initShopWallets(); err != nil {
		t.Fatal(err)
	}
	if err = initShopPurchases(); err != nil {
		t.Fatal(err)
	}
	if err = initCommunity(); err != nil {
		t.Fatal(err)
	}
	s := testShop(t)
	s.channelsRoot = t.TempDir()
	s.iconsRoot = t.TempDir()
	s.installs.root = t.TempDir()
	archive := testPackage(t, map[string]string{"apps/demo/boot.dol": "verified-homebrew", "apps/demo/meta.xml": "<app/>"})
	s.installs.client = &http.Client{Transport: installRoundTrip(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewReader(archive)), ContentLength: int64(len(archive)), Header: make(http.Header)}, nil
	})}
	server := httptest.NewServer(s.featureHandler(s.classicSiteHandler(http.NotFoundHandler())))
	defer server.Close()
	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar, Timeout: 5 * time.Second}
	getBody := func(path string) string {
		r, err := client.Get(server.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		defer r.Body.Close()
		b, _ := io.ReadAll(r.Body)
		if r.StatusCode != 200 {
			t.Fatalf("GET %s: %d", path, r.StatusCode)
		}
		return string(b)
	}
	page := getBody("/points")
	re := regexp.MustCompile(`name="csrf" value="([a-f0-9]+)"`)
	token := re.FindStringSubmatch(page)[1]
	parsed, _ := url.Parse(server.URL)
	wallet := ""
	for _, c := range jar.Cookies(parsed) {
		if c.Name == "shroombrew_wallet" {
			wallet = c.Value
		}
	}
	if wallet == "" {
		t.Fatal("wallet cookie missing")
	}
	t.Cleanup(func() {
		testPool.Exec(ctx, `DELETE FROM shroombrew_download_requests WHERE wallet=$1`, wallet)
		testPool.Exec(ctx, `DELETE FROM shroombrew_purchases WHERE wallet=$1`, wallet)
		testPool.Exec(ctx, `DELETE FROM shroombrew_wallets WHERE id=$1`, wallet)
	})
	// A card works exactly once, through the page the Wii uses.
	cardNum, _ := newCardNumber()
	if _, err = testPool.Exec(ctx, `INSERT INTO shroombrew_cards(hash, points, creator) VALUES($1,$2,'test')`, cardHash(cardNum), cardPoints); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { testPool.Exec(ctx, `DELETE FROM shroombrew_cards WHERE hash=$1`, cardHash(cardNum)) })
	for i := 0; i < 2; i++ {
		r, err := client.PostForm(server.URL+"/points", url.Values{"csrf": {token}, "card": {formatCard(cardNum)}})
		if err != nil {
			t.Fatal(err)
		}
		r.Body.Close()
		if r.StatusCode != 200 {
			t.Fatal("points POST failed")
		}
	}
	if points, _ := walletBalance(wallet); points != cardPoints {
		t.Fatalf("points=%d want %d (card must count once)", points, cardPoints)
	}
	if _, err = testPool.Exec(ctx, `UPDATE shroombrew_wallets SET points=800 WHERE id=$1`, wallet); err != nil {
		t.Fatal(err)
	}
	points, _ := walletBalance(wallet)
	if points != 800 {
		t.Fatalf("points=%d want800", points)
	}
	bad, err := client.PostForm(server.URL+"/points", url.Values{"csrf": {"bad"}})
	if err != nil {
		t.Fatal(err)
	}
	bad.Body.Close()
	if bad.StatusCode != 403 {
		t.Fatal("invalid CSRF accepted")
	}
	price := classicPrice("demo")
	for attempt := 0; attempt < 2; attempt++ {
		r, err := client.PostForm(server.URL+"/wii-install/title/"+classicTitleIDForApp(s.bySlug["demo"]), url.Values{"csrf": {token}})
		if err != nil {
			t.Fatal(err)
		}
		preparePath := r.Request.URL.Path
		r.Body.Close()
		if !strings.HasPrefix(preparePath, "/shop-download/prepare/") {
			t.Fatal("checkout failed to prepare package")
		}
		jobID := strings.TrimPrefix(preparePath, "/shop-download/prepare/")
		job, ok := s.installs.find(jobID)
		if !ok {
			t.Fatal("job missing")
		}
		deadline := time.Now().Add(3 * time.Second)
		for {
			job.mu.RLock()
			state := job.State
			job.mu.RUnlock()
			if state == "ready" {
				break
			}
			if state == "failed" || time.Now().After(deadline) {
				t.Fatal("package preparation failed")
			}
			time.Sleep(10 * time.Millisecond)
		}
		before, _ := walletBalance(wallet)
		r, err = client.PostForm(server.URL+"/shop-download/zip/"+jobID, url.Values{"csrf": {token}})
		if err != nil {
			t.Fatal(err)
		}
		b, err := io.ReadAll(r.Body)
		r.Body.Close()
		if err != nil {
			t.Fatal(err)
		}
		if r.StatusCode != 200 || r.Header.Get("Content-Type") != "application/zip" {
			t.Fatalf("download status=%d", r.StatusCode)
		}
		z, err := zip.NewReader(bytes.NewReader(b), int64(len(b)))
		if err != nil {
			t.Fatal(err)
		}
		if len(z.File) != 3 {
			t.Fatalf("archive files=%d", len(z.File))
		}
		after, _ := walletBalance(wallet)
		if attempt == 0 && before-after != price {
			t.Fatal("wrong first-purchase debit")
		}
		if attempt == 1 && before != after {
			t.Fatal("repeat download charged twice")
		}
		// Content-Length lets a client observe EOF before the handler finishes
		// recording delivery and cleaning up. Wait for that bounded cleanup.
		cleanupDeadline := time.Now().Add(time.Second)
		for {
			_, err = os.Stat(job.Workspace)
			if os.IsNotExist(err) || time.Now().After(cleanupDeadline) {
				break
			}
			time.Sleep(10 * time.Millisecond)
		}
		if !os.IsNotExist(err) {
			t.Fatal("temporary package was not removed")
		}
	}
	if !strings.Contains(getBody("/downloaded"), "Demo &lt;script&gt;") {
		t.Fatal("download history missing")
	}
	var group sync.WaitGroup
	for i := 0; i < 8; i++ {
		group.Add(1)
		go func() {
			defer group.Done()
			if err := purchaseOnce(ctx, wallet, "concurrent-test", 100); err != nil {
				t.Error("concurrent purchase failed")
			}
		}()
	}
	group.Wait()
	final, _ := walletBalance(wallet)
	if final != 800-price-100 {
		t.Fatal("concurrent purchase debited more than once")
	}
}
