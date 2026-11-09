package main

import (
	"context"
	"errors"
	"fmt"
	"html"
	"net/http"
	"os"
	"strings"
)

// Wii Points here are free, local play credits. No Nintendo account or payment is used.
func initShopWallets() error {
	_, err := pool.Exec(context.Background(), `CREATE TABLE IF NOT EXISTS shroombrew_wallets (
		id text PRIMARY KEY, points integer NOT NULL DEFAULT 0 CHECK (points >= 0),
		updated_at timestamptz NOT NULL DEFAULT now())`)
	return err
}

func walletID(w http.ResponseWriter, r *http.Request) (string, error) {
	if c, err := r.Cookie("shroombrew_wallet"); err == nil && validWalletID(c.Value) {
		setWalletCookie(w, r, c.Value) // sliding expiry: every visit renews it
		return c.Value, nil
	}
	value, err := randomHex(16)
	if err != nil {
		return "", err
	}
	setWalletCookie(w, r, value)
	return value, nil
}

func walletBalance(id string) (int, error) {
	var points int
	err := pool.QueryRow(context.Background(), `INSERT INTO shroombrew_wallets (id) VALUES ($1)
		ON CONFLICT (id) DO UPDATE SET id = EXCLUDED.id RETURNING points`, id).Scan(&points)
	return points, err
}

var errFreePointsOff = errors.New("free points are disabled; redeem a card number instead")

// Free points predate Wii Points cards. They stay off unless an operator sets
// SHROOMBREW_FREE_POINTS=1 (used by tests and local development).
func walletAdd(id string) (int, error) {
	if os.Getenv("SHROOMBREW_FREE_POINTS") != "1" {
		return 0, errFreePointsOff
	}
	var points int
	err := pool.QueryRow(context.Background(), `INSERT INTO shroombrew_wallets (id, points) VALUES ($1, 100)
		ON CONFLICT (id) DO UPDATE SET points = shroombrew_wallets.points + 100,
		updated_at = now() RETURNING points`, id).Scan(&points)
	if err == nil {
		_ = ledgerAdd(context.Background(), pool, id, 100, "free", "")
	}
	return points, err
}

func walletSpend(id string, price int) (bool, error) {
	result, err := pool.Exec(context.Background(), `UPDATE shroombrew_wallets SET points = points - $2,
		updated_at = now() WHERE id = $1 AND points >= $2`, id, price)
	ok := err == nil && result.RowsAffected() == 1
	if ok && price > 0 {
		_ = ledgerAdd(context.Background(), pool, id, -price, "purchase", "")
	}
	return ok, err
}

func walletRefund(id string, price int) {
	_, _ = pool.Exec(context.Background(), `UPDATE shroombrew_wallets SET points = points + $2,
		updated_at = now() WHERE id = $1`, id, price)
	if price > 0 {
		_ = ledgerAdd(context.Background(), pool, id, price, "refund", "")
	}
}

func writeShopPage(w http.ResponseWriter, title, content string) {
	writeShopPageOpts(w, title, content, true)
}

func writeShopPageOpts(w http.ResponseWriter, title, content string, topHelp bool) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	// These pages show live balances and change with each deploy; the
	// console's browser would otherwise reuse an old copy.
	w.Header().Set("Cache-Control", "no-cache")
	_, _ = w.Write([]byte(shopShellOpts(title, content, topHelp)))
}

func (s *storefront) pointsPage(w http.ResponseWriter, r *http.Request) {
	id, err := walletID(w, r)
	if err != nil {
		http.Error(w, "Wallet unavailable", 500)
		return
	}
	if r.Method == http.MethodPost {
		if _, err := walletAdd(id); err != nil {
			http.Error(w, "Wallet unavailable", 500)
			return
		}
		http.Redirect(w, r, "/points", http.StatusSeeOther)
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", 405)
		return
	}
	balance, err := walletBalance(id)
	if err != nil {
		http.Error(w, "Wallet unavailable", 500)
		return
	}
	content := fmt.Sprintf(`<div class="panel plain"><p class="center">Wii Points: <span class="big">%d</span></p><p class="center">Add 100 free Shroombrew points each time you press the button. No real money is involved.</p><form method="post" action="/points"><p class="center">%s</p></form></div><div class="foot">%s</div>`, balance, uiSubmit("", "", "Add 100 Wii Points"), uiButton("/oss/serv/W_03.html", "Back"))
	writeShopPage(w, "Add Wii Points", content)
}

func (s *storefront) balanceJS(w http.ResponseWriter, r *http.Request) {
	id, err := walletID(w, r)
	if err != nil {
		http.Error(w, "Wallet unavailable", 500)
		return
	}
	balance, err := walletBalance(id)
	if err != nil {
		http.Error(w, "Wallet unavailable", 500)
		return
	}
	w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
	fmt.Fprintf(w, `var shroombrewPoints=%d;try{if(parent!==window)parent.points=shroombrewPoints;}catch(e){} (function(){var n=document.getElementById('currentBalance');if(n)n.innerHTML=shroombrewPoints;})();`, balance)
}

func (s *storefront) simpleInfo(w http.ResponseWriter, r *http.Request) {
	title := "Shroombrew Help"
	text := "Browse homebrew titles, add free Wii Points, and use the Download to SD button to get a pairing code for the Shroombrew Installer in the Homebrew Channel."
	switch r.URL.Path {
	case "/oss/serv/W_02.html", "/news":
		title = "Important Info"
		text = "Welcome to Shroombrew! Browse homebrew and use free Wii Points. Downloads use the Shroombrew Installer on your SD card."
	case "/activity":
		title = "Account Activity"
		text = "Your free Wii Points balance and downloads are part of this Shroombrew wallet."
	case "/downloaded":
		title = "Titles You've Downloaded"
		text = "Completed downloads appear on the SD card in the apps folder. Open them from the Homebrew Channel."
	case "/settings":
		title = "Settings and Features"
		text = "Shroombrew is a community homebrew catalog. Use Add Wii Points to earn free play credits."
	case "/controllers":
		title = "Compatible Controllers"
		id := strings.ToUpper(r.URL.Query().Get("titleId"))
		for _, app := range s.apps {
			if classicTitleIDForApp(app) == id {
				text = app.Name + ": " + strings.Join(app.Peripherals, ", ")
				break
			}
		}
	}
	content := `<div class="panel plain"><p>` + html.EscapeString(text) + `</p></div><div class="foot"><a class="opt" href="javascript:history.back()">Back</a>` + uiButton("/oss/serv/W_03.html", "Shop Home") + `</div>`
	writeShopPage(w, title, content)
}
