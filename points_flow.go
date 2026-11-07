package main

// The Add Wii Points screens after P_01, as the shop (and the WiiMart revival)
// had them:
//
//	P_02 card number -> P_04 "Wii Points on this card" -> P_15 points added
//	P_06 amount -> P_07 card type -> P_08 card details -> P_12 confirmation -> P_15
//
// Wii Points are free, so the credit-card screens take no card: P_08 shows a
// made-up number the person never needs to change, every amount costs $0.00,
// and Yes on P_12 just adds the points.

import (
	"context"
	"fmt"
	"math/rand"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// pointsOption is one P_06 button.
type pointsOption struct {
	Points  int
	Price   string
	Enabled bool
	Left    int // the button's x position, as P_06 placed its four buttons
}

// buyAmounts are the shop's four Wii Points amounts and their old US prices.
var buyAmounts = []struct {
	points int
	price  string
}{{1000, "$10.00"}, {2000, "$20.00"}, {3000, "$30.00"}, {5000, "$50.00"}}

func validBuyAmount(n int) bool {
	for _, a := range buyAmounts {
		if a.points == n {
			return true
		}
	}
	return false
}

// buyPoints adds a free "credit card" purchase to a wallet.
func buyPoints(ctx context.Context, wallet string, n int) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var balance int
	if err = tx.QueryRow(ctx, `SELECT points FROM shroombrew_wallets WHERE id=$1 FOR UPDATE`, wallet).Scan(&balance); err != nil {
		return err
	}
	if balance+n > walletCap {
		return errPointsCap
	}
	if err = addPoints(ctx, tx, wallet, n, "purchase", "free"); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// fakeCardNumber makes up a card number of the chosen type (MasterCard 5x,
// Visa 4) for P_08 to show, like the revival does; it is never charged.
func fakeCardNumber(kind string) string {
	prefix := "4"
	if kind == "M" {
		prefix = "5" + strconv.Itoa(1+rand.Intn(5))
	}
	n := prefix
	for len(n) < 15 {
		n += strconv.Itoa(rand.Intn(10))
	}
	return n + string(luhnCheckDigit(n))
}

// pointsFlow serves every /points/... screen after P_01.
func (s *storefront) pointsFlow(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/points/card":
		s.cardEntryPage(w, r)
	case "/points/card/redeem":
		s.cardRedeem(w, r)
	case "/points/buy":
		s.buyAmountPage(w, r)
	case "/points/buy/card":
		s.buyCardTypePage(w, r)
	case "/points/buy/info":
		s.buyCardInfoPage(w, r)
	case "/points/buy/confirm":
		s.buyConfirmPage(w, r)
	case "/points/added":
		s.pointsAddedPage(w, r)
	default:
		http.NotFound(w, r)
	}
}

// returnQuery carries where the person goes once points are added through
// every screen of the flow.
func returnQuery(back string) string { return "return=" + url.QueryEscape(back) }

// cardRedeem is P_04's Redeem button.
func (s *storefront) cardRedeem(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/points/card", http.StatusSeeOther)
		return
	}
	id, err := walletID(w, r)
	if err != nil {
		http.Error(w, "Wallet unavailable", 500)
		return
	}
	if !validShopPost(r, id) {
		http.Error(w, "Reload the page and try again", 403)
		return
	}
	back := archivedPointsReturn(r)
	got, rerr := redeemCard(r.Context(), id, clientIP(r), r.Form.Get("card"))
	if rerr != nil {
		http.Redirect(w, r, "/points/card?"+returnQuery(back)+redeemErrorQuery(rerr), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/points/added?n="+strconv.Itoa(got)+"&"+returnQuery(back), http.StatusSeeOther)
}

// redeemErrorQuery turns a redeemCard error into P_02's ?m= message code.
func redeemErrorQuery(err error) string {
	if we, ok := err.(*waitError); ok {
		return "&m=locked&n=" + strconv.Itoa(we.seconds())
	}
	switch err {
	case errPointsCap:
		return "&m=cap"
	case errCardBad:
		return "&m=err"
	}
	return "&m=fail"
}

func (s *storefront) buyAmountPage(w http.ResponseWriter, r *http.Request) {
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
	back := archivedPointsReturn(r)
	options := make([]pointsOption, 0, len(buyAmounts))
	for i, a := range buyAmounts {
		options = append(options, pointsOption{Points: a.points, Price: a.price, Enabled: balance+a.points <= walletCap, Left: 34 + 140*i})
	}
	writeArchivedCheckout(w, r, archivedCheckoutData{
		Page: "P_06", Balance: balance, Back: "/points?" + returnQuery(back),
		Next: "/points/buy/card?" + returnQuery(back), Options: options, Cap: walletCap,
	})
}

// buyStep reads the chosen amount, or sends the person back to choose one.
func buyStep(w http.ResponseWriter, r *http.Request) (wallet string, balance, points int, back string, ok bool) {
	back = archivedPointsReturn(r)
	points, _ = strconv.Atoi(r.URL.Query().Get("points"))
	if r.Method == http.MethodPost {
		points, _ = strconv.Atoi(r.FormValue("points"))
	}
	if !validBuyAmount(points) {
		http.Redirect(w, r, "/points/buy?"+returnQuery(back), http.StatusSeeOther)
		return
	}
	wallet, err := walletID(w, r)
	if err != nil {
		http.Error(w, "Wallet unavailable", 500)
		return
	}
	if balance, err = walletBalance(wallet); err != nil {
		http.Error(w, "Wallet unavailable", 500)
		return
	}
	return wallet, balance, points, back, true
}

func (s *storefront) buyCardTypePage(w http.ResponseWriter, r *http.Request) {
	_, balance, points, back, ok := buyStep(w, r)
	if !ok {
		return
	}
	writeArchivedCheckout(w, r, archivedCheckoutData{
		Page: "P_07", Balance: balance, Points: points, Back: "/points/buy?" + returnQuery(back),
		Next: fmt.Sprintf("/points/buy/info?points=%d&%s", points, returnQuery(back)),
	})
}

func (s *storefront) buyCardInfoPage(w http.ResponseWriter, r *http.Request) {
	_, balance, points, back, ok := buyStep(w, r)
	if !ok {
		return
	}
	kind := r.URL.Query().Get("card")
	if kind != "M" {
		kind = "V"
	}
	now := time.Now()
	writeArchivedCheckout(w, r, archivedCheckoutData{
		Page: "P_08", Balance: balance, Points: points, CardType: kind,
		CardNumber: fakeCardNumber(kind), CardExpMonth: strconv.Itoa(int(now.Month())), CardExpYear: strconv.Itoa(now.Year() + 1 + rand.Intn(4)),
		CardCode: fmt.Sprintf("%03d", rand.Intn(1000)),
		Back:     fmt.Sprintf("/points/buy/card?points=%d&%s", points, returnQuery(back)),
		Next:     fmt.Sprintf("/points/buy/confirm?points=%d&%s", points, returnQuery(back)),
	})
}

func (s *storefront) buyConfirmPage(w http.ResponseWriter, r *http.Request) {
	wallet, balance, points, back, ok := buyStep(w, r)
	if !ok {
		return
	}
	if r.Method == http.MethodPost {
		if !validShopPost(r, wallet) {
			http.Error(w, "Reload the page and try again", 403)
			return
		}
		if err := buyPoints(r.Context(), wallet, points); err != nil {
			// Over the balance limit: the amount page greys out what no longer fits.
			http.Redirect(w, r, "/points/buy?"+returnQuery(back), http.StatusSeeOther)
			return
		}
		http.Redirect(w, r, fmt.Sprintf("/points/added?n=%d&%s", points, returnQuery(back)), http.StatusSeeOther)
		return
	}
	writeArchivedCheckout(w, r, archivedCheckoutData{
		Page: "P_12", Balance: balance, Points: points, CSRF: csrfToken(wallet),
		Action: fmt.Sprintf("/points/buy/confirm?%s", returnQuery(back)), Back: "/points?" + returnQuery(back),
	})
}

func (s *storefront) pointsAddedPage(w http.ResponseWriter, r *http.Request) {
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
	n, _ := strconv.Atoi(r.URL.Query().Get("n"))
	if n < 0 || n > walletCap {
		n = 0
	}
	writeArchivedCheckout(w, r, archivedCheckoutData{Page: "P_15", Balance: balance, Points: n, NewBalance: balance, Back: archivedPointsReturn(r)})
}
