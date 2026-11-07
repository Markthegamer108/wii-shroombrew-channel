package main

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v4/pgxpool"
)

func TestLuhnAndCardFormat(t *testing.T) {
	secretKey = []byte("0123456789abcdef0123456789abcdef")
	for i := 0; i < 200; i++ {
		n, err := newCardNumber()
		if err != nil || len(n) != 16 || !luhnValid(n) {
			t.Fatalf("bad card %q %v", n, err)
		}
		if normalizeCard(formatCard(n)) != n {
			t.Fatalf("format/normalize round trip failed for %q", n)
		}
		// a single mistyped digit must be caught by the check digit
		b := []byte(n)
		b[3] = '0' + (b[3]-'0'+1)%10
		if luhnValid(string(b)) {
			t.Fatalf("typo not detected in %q", n)
		}
	}
	for _, bad := range []string{"", "1234", "abcd efgh ijkl mnop", "1234 5678 9012 345", "1234 5678 9012 34567", "1234.5678.9012.3456"} {
		if normalizeCard(bad) != "" {
			t.Errorf("normalizeCard(%q) should be rejected", bad)
		}
	}
	if cardHash("1") == cardHash("2") || cardHash("1") != cardHash("1") {
		t.Fatal("card hash must be deterministic and distinct")
	}
}

func TestWiiLangCode(t *testing.T) {
	for _, c := range []struct{ lang, country, want string }{
		{"ja", "JP", "ja"}, {"en", "US", "en"}, {"de", "DE", "de"}, {"fr", "FR", "fr"}, {"es", "ES", "es"},
		{"it", "IT", "it"}, {"nl", "NL", "nl"}, {"ko", "KR", "ko"}, {"zh_CN", "CN", "zh-cn"}, {"zh-TW", "TW", "zh-tw"},
		{"zh", "TW", "zh-tw"}, {"zh", "", "zh-cn"}, {"JA", "", "ja"}, {"", "", "en"}, {"xx", "", "en"}, {"pt-BR", "BR", "pt"},
	} {
		if got := wiiLangCode(c.lang, c.country); got != c.want {
			t.Errorf("wiiLangCode(%q,%q)=%q want %q", c.lang, c.country, got, c.want)
		}
	}
}

func TestDeviceKey(t *testing.T) {
	secretKey = []byte("0123456789abcdef0123456789abcdef")
	a := deviceInfo{ID: "123456", Serial: "LU12345678"}.key()
	if a == "" || a != (deviceInfo{ID: "123456", Wifi: "ignored", Serial: "LU12345678"}.key()) {
		t.Fatal("key must use the first available secondary identifier")
	}
	if a == (deviceInfo{ID: "123457", Serial: "LU12345678"}).key() {
		t.Fatal("different consoles must differ")
	}
	for _, bad := range []deviceInfo{{}, {ID: "0", Serial: "x"}, {ID: "123"}, {ID: "123", Serial: "\x01bad"}} {
		if bad.key() != "" {
			t.Errorf("%+v should not produce a key", bad)
		}
	}
	if strings.Contains(a, "123456") || strings.Contains(a, "LU12345678") {
		t.Fatal("raw identifiers must not leak into the key")
	}
}

func TestRateLimiter(t *testing.T) {
	l := newRateLimiter()
	for i := 0; i < 3; i++ {
		if !l.allow("k", 3, time.Minute) {
			t.Fatal("under the limit")
		}
	}
	if l.allow("k", 3, time.Minute) || !l.allow("other", 3, time.Minute) {
		t.Fatal("limit is per key")
	}
}

// Every language must have all Wii-facing strings, with the same {slots} as
// English, so no console ever shows a blank or broken line.
func TestWiiStringsComplete(t *testing.T) {
	dir := os.Getenv("SHROOMBREW_LANG_DIR")
	if dir == "" {
		dir = filepath.Join("..", "community", "lang")
	}
	read := func(code string) map[string]string {
		b, err := os.ReadFile(filepath.Join(dir, code+".json"))
		if err != nil {
			t.Skip("language files not found: " + dir)
		}
		m := map[string]string{}
		if err = json.Unmarshal(b, &m); err != nil {
			t.Fatalf("%s.json: %v", code, err)
		}
		return m
	}
	en := read("en")
	slots := regexp.MustCompile(`\{\w+\}`)
	wii := regexp.MustCompile(`^(w_|c_|er_|l_|f_(title|code|share|level|none|points|enter|redeem|done|in|nogifts|from|accept|decline|updates|noupdates)$|n_(title|body|toggle|continue|hide)$)`)
	for _, code := range langOrder {
		m := read(code)
		for k, e := range en {
			if !wii.MatchString(k) {
				continue
			}
			v := m[k]
			if strings.TrimSpace(v) == "" {
				t.Errorf("%s: missing Wii string %q", code, k)
				continue
			}
			set := func(s string) string {
				f := slots.FindAllString(s, -1)
				sort.Strings(f)
				return strings.Join(f, ",")
			}
			if set(v) != set(e) {
				t.Errorf("%s: %q has different {slots} than English", code, k)
			}
		}
	}
}

// Every sentence on the purchase and download screens must be translated in
// every Wii language, otherwise part of the screen stays English.
func TestArchiveDictionaryComplete(t *testing.T) {
	dir := os.Getenv("SHROOMBREW_LANG_DIR")
	if dir == "" {
		dir = filepath.Join("..", "community", "lang")
	}
	if _, err := os.Stat(dir); err != nil {
		t.Skip("language files not found: " + dir)
	}
	before := langDir
	langDir = dir
	t.Cleanup(func() { langDir = before })
	for _, code := range langOrder {
		if code == "en" {
			if archiveDictionary("en") != nil {
				t.Fatal("English needs no dictionary")
			}
			continue
		}
		d := archiveDictionary(code)
		for en, key := range archiveLiterals {
			if _, ok := d[en]; !ok {
				t.Errorf("%s: no translation for %q (%s)", code, en, key)
			}
		}
	}
}

func openTestDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
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
	return testPool
}

func uniqueIP(t *testing.T) string {
	h, _ := randomHex(3)
	var a, b, c int
	fmt.Sscanf(h[0:2], "%x", &a)
	fmt.Sscanf(h[2:4], "%x", &b)
	fmt.Sscanf(h[4:6], "%x", &c)
	return fmt.Sprintf("198.%d.%d.%d", 18+a%2, b, c)
}

func TestCardsIntegration(t *testing.T) {
	db := openTestDB(t)
	bg := context.Background()
	wallet := randomWalletForTest(t)
	if _, err := walletBalance(wallet); err != nil {
		t.Fatal(err)
	}

	// --- generation: one card, then the 15 second cooldown ---
	ip := uniqueIP(t)
	num, err := generateCard(bg, ip, 1000)
	if err != nil || !luhnValid(num) {
		t.Fatalf("generate: %q %v", num, err)
	}
	var stored string
	if err = db.QueryRow(bg, `SELECT hash FROM shroombrew_cards WHERE hash=$1`, cardHash(num)).Scan(&stored); err != nil || strings.Contains(stored, num) {
		t.Fatalf("card must be stored as a hash only: %v", err)
	}
	_, err = generateCard(bg, ip, 1000)
	we, ok := err.(*waitError)
	if !ok || we.seconds() < 10 || we.seconds() > 15 {
		t.Fatalf("expected a ~15 s cooldown, got %v", err)
	}
	// the same /64 shares one cooldown
	if _, err = generateCard(bg, "2001:db8:1:2:aaaa::1", 1000); err != nil {
		t.Fatalf("first v6 card: %v", err)
	}
	if _, err = generateCard(bg, "2001:db8:1:2:bbbb::9", 1000); err == nil {
		t.Fatal("an address in the same /64 must share the cooldown")
	}

	// --- parallel requests cannot beat the cooldown ---
	pip := uniqueIP(t)
	var wg sync.WaitGroup
	var mu sync.Mutex
	okCount := 0
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, e := generateCard(bg, pip, 1000); e == nil {
				mu.Lock()
				okCount++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	if okCount != 1 {
		t.Fatalf("12 parallel requests produced %d cards, want exactly 1", okCount)
	}

	// --- redeem once, and only once ---
	before, _ := walletBalance(wallet)
	rip := uniqueIP(t)
	got, err := redeemCard(bg, wallet, rip, formatCard(num))
	if err != nil || got != 1000 {
		t.Fatalf("redeem: %d %v", got, err)
	}
	if after, _ := walletBalance(wallet); after != before+1000 {
		t.Fatalf("balance %d want %d", after, before+1000)
	}
	h, _ := ledgerHistory(bg, wallet, 5)
	if len(h) == 0 || h[0].Delta != 1000 || h[0].Reason != "card" || h[0].Balance != before+1000 || h[0].Ref != cardHash(num)[:12] {
		t.Fatalf("ledger entry missing or wrong: %+v", h)
	}

	// --- points persist: a fresh read still has them ---
	if p, _ := walletBalance(wallet); p != before+1000 {
		t.Fatal("points not persisted")
	}

	// --- a used card, or any other number, is a standard card ---
	other := randomWalletForTest(t)
	walletBalance(other)
	for _, n := range []string{num, "1111 1111 1111 1111"} {
		got, err = redeemCard(bg, other, uniqueIP(t), n)
		if err != nil || got != cardPoints {
			t.Fatalf("standard card %s: %d %v", n, got, err)
		}
		if h, _ = ledgerHistory(bg, other, 1); len(h) == 0 || h[0].Ref != "standard" {
			t.Fatalf("standard card ledger: %+v", h)
		}
	}
	var by string
	if db.QueryRow(bg, `SELECT redeemed_by FROM shroombrew_cards WHERE hash=$1`, cardHash(num)).Scan(&by); by != wallet {
		t.Fatalf("the minted card must stay redeemed by its first wallet, got %q", by)
	}

	// --- brute force lockout: entries that are not card numbers ---
	lw := randomWalletForTest(t)
	walletBalance(lw)
	lip := uniqueIP(t)
	for i := 0; i < redeemFailLimit; i++ {
		if _, err = redeemCard(bg, lw, lip, "0000 0000"); err != errCardBad {
			t.Fatalf("wrong card %d: %v", i, err)
		}
	}
	good, err := generateCard(bg, uniqueIP(t), 1000)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = redeemCard(bg, lw, lip, good); err == nil {
		t.Fatal("locked wallet must not redeem even a valid card")
	} else if _, ok := err.(*waitError); !ok {
		t.Fatalf("expected lockout, got %v", err)
	}
	if p, _ := walletBalance(lw); p != 0 {
		t.Fatal("locked out redeem must not add points")
	}
	// a different, clean wallet and address can still use the card
	if _, err = redeemCard(bg, other, uniqueIP(t), good); err != nil {
		t.Fatalf("valid card unusable after someone else's lockout: %v", err)
	}

	// --- wallet cap keeps the card unused ---
	cw := randomWalletForTest(t)
	walletBalance(cw)
	db.Exec(bg, `UPDATE shroombrew_wallets SET points=$2 WHERE id=$1`, cw, walletCap-500)
	capCard, _ := generateCard(bg, uniqueIP(t), 1000)
	if _, err = redeemCard(bg, cw, uniqueIP(t), capCard); err != errPointsCap {
		t.Fatalf("cap: %v", err)
	}
	db.Exec(bg, `UPDATE shroombrew_wallets SET points=0 WHERE id=$1`, cw)
	if _, err = redeemCard(bg, cw, uniqueIP(t), capCard); err != nil {
		t.Fatalf("card must stay valid after a cap refusal: %v", err)
	}

	// --- an expired card is no longer minted: it redeems as a standard card ---
	old, _ := generateCard(bg, uniqueIP(t), 1000)
	db.Exec(bg, `UPDATE shroombrew_cards SET created_at=now()-interval '31 days' WHERE hash=$1`, cardHash(old))
	ew := randomWalletForTest(t)
	walletBalance(ew)
	if _, err = redeemCard(bg, ew, uniqueIP(t), old); err != nil {
		t.Fatalf("expired card: %v", err)
	}
	if h, _ = ledgerHistory(bg, ew, 1); len(h) == 0 || h[0].Ref != "standard" {
		t.Fatalf("expired card redeemed as minted: %+v", h)
	}
}

func TestLinkCodesIntegration(t *testing.T) {
	db := openTestDB(t)
	bg := context.Background()
	wallet := randomWalletForTest(t)
	walletBalance(wallet)
	code, _, err := createLinkCode(bg, wallet)
	if err != nil || normalizeLinkCode(formatLinkCode(code)) != code {
		t.Fatalf("create: %q %v", code, err)
	}
	if _, _, err = createLinkCode(bg, wallet); err == nil {
		t.Fatal("new link codes must be rate limited")
	}
	ip := uniqueIP(t)
	token, err := redeemLinkCode(bg, strings.ToLower(formatLinkCode(code)), ip)
	if err != nil || len(token) != 64 {
		t.Fatalf("redeem: %v", err)
	}
	if _, err = redeemLinkCode(bg, code, ip); err == nil {
		t.Fatal("a link code must work once")
	}
	var w string
	if err = db.QueryRow(bg, `SELECT wallet FROM shroombrew_remote WHERE token_hash=$1`, tokenHash(token)).Scan(&w); err != nil || w != wallet {
		t.Fatalf("session not stored for the wallet: %v", err)
	}
	if strings.Contains(w, token) {
		t.Fatal("token must be stored hashed")
	}
	// expiry
	db.Exec(bg, `UPDATE shroombrew_links SET created_at=now()-interval '1 hour'`)
	w2 := randomWalletForTest(t)
	walletBalance(w2)
	c2, _, _ := createLinkCode(bg, w2)
	db.Exec(bg, `UPDATE shroombrew_links SET expires_at=now()-interval '1 second' WHERE wallet=$1`, w2)
	if _, err = redeemLinkCode(bg, c2, uniqueIP(t)); err == nil {
		t.Fatal("expired link code accepted")
	}
	// revoking signs the website out
	if n, _ := revokeRemote(bg, wallet); n != 1 {
		t.Fatalf("revoked %d sessions", n)
	}
	// brute force
	bip := uniqueIP(t)
	for i := 0; i < linkFailLimit; i++ {
		redeemLinkCode(bg, "AAAAAAAA", bip)
	}
	c3, _, _ := createLinkCode(bg, randomWalletForTest(t))
	if _, err = redeemLinkCode(bg, c3, bip); err == nil {
		t.Fatal("link guessing must lock out")
	}
}

func TestDeviceRegistrationIntegration(t *testing.T) {
	db := openTestDB(t)
	bg := context.Background()
	id, _ := randomHex(6)
	info := deviceInfo{ID: id, Serial: "LU" + id, Language: "ko", Country: "KR"}
	key := info.key()

	w1, lang, err := registerDeviceWallet(bg, key, "", info)
	if err != nil || !validWalletID(w1) || lang != "ko" {
		t.Fatalf("register: %q %q %v", w1, lang, err)
	}
	w2, _, err := registerDeviceWallet(bg, key, "", info)
	if err != nil || w2 != w1 {
		t.Fatalf("same console must get the same wallet: %q vs %q (%v)", w2, w1, err)
	}
	// a different browser cookie cannot take over a registered console's wallet
	w3, _, _ := registerDeviceWallet(bg, key, randomWalletForTest(t), info)
	if w3 != w1 {
		t.Fatal("console wallet must not change")
	}
	// points earned before registration move onto the console
	id2, _ := randomHex(6)
	info2 := deviceInfo{ID: id2, Wifi: "00:11:22:33:44:55", Language: "ja"}
	pre := randomWalletForTest(t)
	walletBalance(pre)
	db.Exec(bg, `UPDATE shroombrew_wallets SET points=700 WHERE id=$1`, pre)
	w4, lang4, err := registerDeviceWallet(bg, info2.key(), pre, info2)
	if err != nil || w4 != pre || lang4 != "ja" {
		t.Fatalf("migrate: %q %q %v", w4, lang4, err)
	}
	if p, _ := walletBalance(w4); p != 700 {
		t.Fatal("points lost during registration")
	}
	// a wallet already tied to a console cannot be claimed by a second console
	id3, _ := randomHex(6)
	info3 := deviceInfo{ID: id3, Serial: "S" + id3, Language: "en"}
	w5, _, _ := registerDeviceWallet(bg, info3.key(), pre, info3)
	if w5 == pre {
		t.Fatal("a claimed wallet must not be shared by a second console")
	}
	// a language picked in Settings beats the System Menu language
	if err = setPref(bg, w1, "lang", "de"); err != nil {
		t.Fatal(err)
	}
	if _, lang, _ = registerDeviceWallet(bg, key, "", info); lang != "de" {
		t.Fatalf("saved language ignored: %q", lang)
	}
	var raw int
	db.QueryRow(bg, `SELECT count(*) FROM shroombrew_devices WHERE device_key LIKE '%'||$1||'%'`, id).Scan(&raw)
	if raw != 0 {
		t.Fatal("raw device id leaked into storage")
	}
}
