package main

import (
	"context"
	"encoding/xml"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v4/pgxpool"
)

func randomWalletForTest(t *testing.T) string {
	t.Helper()
	id, err := randomHex(16) // the 32-hex wallet format
	if err != nil {
		t.Fatal(err)
	}
	return id
}

// initTestSecrets gives the test a throwaway server secret.
func initTestSecrets(t *testing.T) {
	t.Helper()
	t.Setenv("SHROOMBREW_SECRET_FILE", t.TempDir()+"/secrets.key")
	if err := initSecrets(); err != nil {
		t.Fatal(err)
	}
}

func TestFriendCodeFormat(t *testing.T) {
	c := friendCodeFor("abc", 0)
	if len(c) != 14 || c[4] != '-' || c[9] != '-' {
		t.Fatalf("bad code %q", c)
	}
	if c != friendCodeFor("abc", 0) || c == friendCodeFor("abc", 1) || c == friendCodeFor("abd", 0) {
		t.Fatal("code must be stable per wallet+salt and differ otherwise")
	}
	for _, in := range []string{c, strings.ToLower(c), strings.ReplaceAll(c, "-", " ")} {
		if normalizeFriendCode(in) != c {
			t.Fatalf("normalize(%q) != %q", in, c)
		}
	}
	for _, bad := range []string{"", "ABCD", "0000-0000-0000", "ABCD-EFGH-JKLM-N"} {
		if normalizeFriendCode(bad) != "" {
			t.Fatalf("%q should be rejected", bad)
		}
	}
}

func TestTiers(t *testing.T) {
	cases := map[int]string{0: "", 1: "Bronze", 4: "Bronze", 5: "Silver", 10: "Gold", 25: "Platinum"}
	for n, want := range cases {
		if got := tierFor(n); got != want {
			t.Errorf("tierFor(%d)=%q want %q", n, got, want)
		}
	}
	if tierBonus(4, 5) != 500 || tierBonus(9, 10) != 1000 || tierBonus(24, 25) != 3000 || tierBonus(1, 2) != 0 || tierBonus(0, 1) != 0 {
		t.Fatal("tier bonus boundaries wrong")
	}
	if tierBonus(0, 25) != 4500 {
		t.Fatal("crossing several tiers should pay all of them")
	}
}

func TestSeasonWindow(t *testing.T) {
	d := func(m time.Month, day int) time.Time { return time.Date(2026, m, day, 12, 0, 0, 0, time.UTC) }
	for _, c := range []struct {
		t    time.Time
		want bool
	}{
		{d(time.November, 30), false}, {d(time.December, 1), true}, {d(time.December, 25), true},
		{d(time.December, 26), true}, {d(time.December, 31), true}, {d(time.January, 1), false}, {d(time.January, 5), false}, {d(time.July, 4), false},
	} {
		if got, _ := seasonActive(c.t); got != c.want {
			t.Errorf("seasonActive(%v)=%v want %v", c.t.Format("Jan 2"), got, c.want)
		}
	}
}

func TestSafeLocalPathAndLanguage(t *testing.T) {
	for in, want := range map[string]string{"/settings": "/settings", "/a/b?x=1": "/a/b?x=1", "//evil.com": "/settings", "https://evil.com": "/settings", "/a\\b": "/settings", "": "/settings"} {
		if got := safeLocalPath(in); got != want {
			t.Errorf("safeLocalPath(%q)=%q want %q", in, got, want)
		}
	}
	r := httptest.NewRequest("GET", "/", nil)
	if langFor(r) != "en" {
		t.Fatal("default language must be English")
	}
	r.AddCookie(&http.Cookie{Name: "shroombrew_lang", Value: "fr"})
	if langFor(r) != "fr" {
		t.Fatal("cookie language ignored")
	}
	r = httptest.NewRequest("GET", "/", nil)
	r.AddCookie(&http.Cookie{Name: "shroombrew_lang", Value: "xx"})
	if langFor(r) != "en" {
		t.Fatal("unknown language must fall back to English")
	}
}

// The database flow runs only against a disposable database.
func TestReferralGiftAndUpdatesIntegration(t *testing.T) {
	configPath := os.Getenv("SHOP_INTEGRATION_CONFIG")
	if configPath == "" {
		t.Skip("set SHOP_INTEGRATION_CONFIG for the database check")
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
	before := pool
	pool = testPool
	t.Cleanup(func() { pool = before; testPool.Close() })
	initTestSecrets(t)
	for _, f := range []func() error{initShopWallets, initShopPurchases, initCommunity} {
		if err = f(); err != nil {
			t.Fatal(err)
		}
	}
	bg := context.Background()
	s := testShop(t)
	if len(s.apps) == 0 {
		t.Skip("test catalog is empty")
	}
	slug := s.apps[0].Slug
	price := classicPrice(slug)

	newWallet := func(points int) string {
		id := randomWalletForTest(t)
		if _, err := walletBalance(id); err != nil {
			t.Fatal(err)
		}
		if _, err := testPool.Exec(bg, `UPDATE shroombrew_wallets SET points=$2 WHERE id=$1`, id, points); err != nil {
			t.Fatal(err)
		}
		return id
	}
	bal := func(id string) int { p, _ := walletBalance(id); return p }

	alice, bob, carol := newWallet(0), newWallet(0), newWallet(price*3)
	ca, _ := ensureFriend(bg, alice)
	cb, _ := ensureFriend(bg, bob)
	cc, _ := ensureFriend(bg, carol)
	if ca == cb || ca == "" {
		t.Fatal("friend codes must be unique")
	}
	if again, _ := ensureFriend(bg, alice); again != ca {
		t.Fatal("friend code must be stable")
	}

	// --- referral ---
	if _, err = redeemReferral(bg, alice, ca); err != errSelfCode {
		t.Fatalf("own code: %v", err)
	}
	if _, err = redeemReferral(bg, bob, "ZZZZ-ZZZZ-ZZZZ"); err != errBadCode {
		t.Fatalf("unknown code: %v", err)
	}
	if got, err := redeemReferral(bg, bob, strings.ToLower(ca)); err != nil || got != referralJoinerReward {
		t.Fatalf("redeem: %d %v", got, err)
	}
	if bal(alice) != referralInviterReward+tierBonus(0, 1) || bal(bob) != referralJoinerReward {
		t.Fatalf("rewards wrong: alice=%d bob=%d", bal(alice), bal(bob))
	}
	if _, err = redeemReferral(bg, bob, cc); err != errAlreadyReferred {
		t.Fatalf("second code: %v", err)
	}
	if n, _ := referralCount(bg, alice); n != 1 || tierFor(n) != "Bronze" {
		t.Fatalf("count %d", n)
	}

	// --- gifts ---
	if _, err = s.sendGift(bg, alice, cb, slug, ""); err != errNoPoints {
		t.Fatalf("broke sender: %v", err)
	}
	if _, err = s.sendGift(bg, carol, cc, slug, ""); err != errSelfCode {
		t.Fatalf("self gift: %v", err)
	}
	if _, err = s.sendGift(bg, carol, cb, "no-such-app", ""); err != errUnknownApp {
		t.Fatalf("unknown app: %v", err)
	}
	id, err := s.sendGift(bg, carol, cb, slug, "enjoy")
	if err != nil {
		t.Fatal(err)
	}
	if bal(carol) != price*2 {
		t.Fatalf("points should be held: %d", bal(carol))
	}
	if _, err = s.sendGift(bg, carol, cb, slug, ""); err != errDupGift {
		t.Fatalf("duplicate pending gift: %v", err)
	}
	if in, _, _ := s.listGifts(bg, bob); len(in) != 1 || in[0].ID != id {
		t.Fatal("bob should see the gift")
	}
	if err = s.resolveGift(bg, alice, id, "accept"); err != errGiftGone {
		t.Fatalf("stranger accepted: %v", err)
	}
	if err = s.resolveGift(bg, bob, id, "cancel"); err != errGiftGone {
		t.Fatalf("recipient cancelled: %v", err)
	}
	if err = s.resolveGift(bg, bob, id, "accept"); err != nil {
		t.Fatal(err)
	}
	if owned, _ := ownsApp(bg, testPool, bob, slug); !owned {
		t.Fatal("bob should own the app")
	}
	if err = s.resolveGift(bg, bob, id, "accept"); err != errGiftGone {
		t.Fatalf("double accept: %v", err)
	}
	if _, err = s.sendGift(bg, carol, cb, slug, ""); err != errAlreadyOwned {
		t.Fatalf("gift of owned app: %v", err)
	}

	// decline and cancel refund the sender
	if len(s.apps) > 1 {
		slug2 := s.apps[1].Slug
		p2 := classicPrice(slug2)
		start := bal(carol)
		id2, err := s.sendGift(bg, carol, cb, slug2, "")
		if err != nil {
			t.Fatal(err)
		}
		if err = s.resolveGift(bg, bob, id2, "decline"); err != nil || bal(carol) != start {
			t.Fatalf("decline refund: %v bal=%d want %d", err, bal(carol), start)
		}
		id3, _ := s.sendGift(bg, carol, cb, slug2, "")
		if err = s.resolveGift(bg, carol, id3, "cancel"); err != nil || bal(carol) != start {
			t.Fatalf("cancel refund: %v bal=%d want %d (price %d)", err, bal(carol), start, p2)
		}
	}

	// --- updates ---
	recordPurchaseVersion(bg, bob, slug, 1)
	app := s.bySlug[slug]
	ups, err := s.ownedUpdates(bg, bob)
	if err != nil {
		t.Fatal(err)
	}
	if app.Shop.Version > 1 && (len(ups) != 1 || ups[0].Slug != slug) {
		t.Fatalf("expected an update for %s, got %v", slug, ups)
	}
	if app.Shop.Version <= 1 && len(ups) != 0 {
		t.Fatalf("unexpected updates %v", ups)
	}

	// --- HTTP: API needs the custom header, season endpoint is public ---
	srv := httptest.NewServer(s.featureHandler(http.NotFoundHandler()))
	defer srv.Close()
	resp, _ := http.Post(srv.URL+"/community/api/referral", "application/json", strings.NewReader(`{"code":"x"}`))
	if resp.StatusCode != 401 {
		t.Fatalf("API without a linked Wii should be refused, got %d", resp.StatusCode)
	}
	resp, _ = http.Get(srv.URL + "/community/api/history")
	if resp.StatusCode != 401 {
		t.Fatalf("history without a link should be refused, got %d", resp.StatusCode)
	}
	resp, _ = http.Get(srv.URL + "/community/api/season")
	if resp.StatusCode != 200 {
		t.Fatalf("season endpoint %d", resp.StatusCode)
	}
}
