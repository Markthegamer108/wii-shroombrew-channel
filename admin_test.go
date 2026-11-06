package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"testing"
	"time"
)

func TestNoticeReminderWindow(t *testing.T) {
	ago := func(d time.Duration) string { return strconv.FormatInt(time.Now().Add(-d).Unix(), 10) }
	for v, want := range map[string]bool{
		"": false, "0": false, "1": false, "junk": false, // old or unusable values show the notice again
		ago(time.Hour): true, ago(29 * 24 * time.Hour): true, ago(31 * 24 * time.Hour): false,
	} {
		if got := noticeHiddenRecently(v); got != want {
			t.Errorf("noticeHiddenRecently(%q)=%v want %v", v, got, want)
		}
	}
}

// Every data-i18n key used by a website page must exist in the English file,
// otherwise a visitor would see the raw key.
func TestWebsiteKeysExistInEnglish(t *testing.T) {
	dir := filepath.Join("..", "community")
	b, err := os.ReadFile(filepath.Join(dir, "lang", "en.json"))
	if err != nil {
		t.Skip("website files not found")
	}
	en := map[string]string{}
	if err = json.Unmarshal(b, &en); err != nil {
		t.Fatal(err)
	}
	pages, _ := filepath.Glob(filepath.Join(dir, "*.html"))
	re := regexp.MustCompile(`data-i18n(?:-ph)?="([a-z0-9_]+)"`)
	js := regexp.MustCompile(`I18N\.t\("([a-z0-9_]*[a-z0-9])"`) // "h_"+x style prefixes are built dynamically
	for _, p := range pages {
		html, _ := os.ReadFile(p)
		for _, m := range append(re.FindAllStringSubmatch(string(html), -1), js.FindAllStringSubmatch(string(html), -1)...) {
			if _, ok := en[m[1]]; !ok {
				t.Errorf("%s uses missing key %q", filepath.Base(p), m[1])
			}
		}
	}
}

func TestBansAndAdminIntegration(t *testing.T) {
	db := openTestDB(t)
	bg := context.Background()
	ip := uniqueIP(t)

	// ban blocks every public action, lifting it restores them
	if err := adminRun(bg, []string{"ban", ip, "2", "test", "abuse"}); err != nil {
		t.Fatal(err)
	}
	if _, err := generateCard(bg, ip); err != errBanned {
		t.Fatalf("banned generate: %v", err)
	}
	w := randomWalletForTest(t)
	walletBalance(w)
	if _, err := redeemCard(bg, w, ip, "0000 0000 0000 0000"); err != errBanned {
		t.Fatalf("banned redeem: %v", err)
	}
	if _, err := redeemLinkCode(bg, "AAAA-AAAA", ip); err != errBanned {
		t.Fatalf("banned link: %v", err)
	}
	other := uniqueIP(t)
	if _, err := generateCard(bg, other); err != nil {
		t.Fatalf("an unrelated address must be unaffected: %v", err)
	}
	if err := adminRun(bg, []string{"unban", ip}); err != nil {
		t.Fatal(err)
	}
	if _, err := generateCard(bg, ip); err != nil {
		t.Fatalf("after unban: %v", err)
	}

	// revoking an unused card makes it unusable; a used card cannot be revoked
	rip := uniqueIP(t)
	num, _ := generateCard(bg, rip)
	if err := adminRun(bg, []string{"card-revoke", formatCard(num)}); err != nil {
		t.Fatal(err)
	}
	if _, err := redeemCard(bg, w, uniqueIP(t), num); err != errCardBad {
		t.Fatalf("revoked card still works: %v", err)
	}
	used, _ := generateCard(bg, uniqueIP(t))
	if _, err := redeemCard(bg, w, uniqueIP(t), used); err != nil {
		t.Fatal(err)
	}
	adminRun(bg, []string{"card-revoke", used})
	var n int
	db.QueryRow(bg, `SELECT count(*) FROM shroombrew_cards WHERE hash=$1 AND redeemed_at IS NOT NULL`, cardHash(used)).Scan(&n)
	if n != 1 {
		t.Fatal("a redeemed card record must survive revoke")
	}
	// bulk revoke by address
	bip := uniqueIP(t)
	generateCard(bg, bip)
	if err := adminRun(bg, []string{"cards-revoke-ip", bip}); err != nil {
		t.Fatal(err)
	}
	db.QueryRow(bg, `SELECT count(*) FROM shroombrew_cards WHERE creator=$1`, hmacHex("ip", ipBucket(bip))).Scan(&n)
	if n != 0 {
		t.Fatal("unused cards from that address should be gone")
	}

	// adjust: logged, never below zero
	code, _ := ensureFriend(bg, w)
	before, _ := walletBalance(w)
	if err := adminRun(bg, []string{"adjust", code, "250", "goodwill", "refund"}); err != nil {
		t.Fatal(err)
	}
	if p, _ := walletBalance(w); p != before+250 {
		t.Fatalf("balance %d want %d", p, before+250)
	}
	h, _ := ledgerHistory(bg, w, 1)
	if len(h) == 0 || h[0].Reason != "admin" || h[0].Delta != 250 || h[0].Ref != "goodwill refund" {
		t.Fatalf("adjust must be in the ledger: %+v", h)
	}
	if err := adminRun(bg, []string{"adjust", code, "-999999", "x"}); err == nil {
		t.Fatal("absurd adjustment accepted")
	}
	if err := adminRun(bg, []string{"adjust", code, "-" + strconv.Itoa(before+251), "x"}); err == nil {
		t.Fatal("adjustment below zero accepted")
	}
	for _, cmd := range [][]string{{"stats"}, {"bans"}, {"wallet", code}, {"sessions-revoke", code}} {
		if err := adminRun(bg, cmd); err != nil {
			t.Fatalf("%v: %v", cmd, err)
		}
	}
	if err := adminRun(bg, []string{"wallet", "ZZZZ-ZZZZ-ZZZZ"}); err == nil {
		t.Fatal("unknown friend code accepted")
	}
}
