package main

import (
	"bytes"
	"strings"
	"testing"
)

// Every Add Wii Points screen renders with the data its handler passes.
func TestPointsFlowTemplates(t *testing.T) {
	options := []pointsOption{{Points: 1000, Price: "$10.00", Enabled: true, Left: 34}, {Points: 5000, Price: "$50.00", Enabled: false, Left: 454}}
	cases := []struct {
		data archivedCheckoutData
		want []string
	}{
		{archivedCheckoutData{Page: "P_04", Points: 1000, CardNumber: "1111 1111 1111 1111", CSRF: "x", Action: "/points/card/redeem"}, []string{"Wii Points on this card", "1000 Wii Points", `name="card" value="1111 1111 1111 1111"`}},
		{archivedCheckoutData{Page: "P_06", Options: options, Cap: walletCap, Next: "/points/buy/card"}, []string{"$10.00", `data-points="1000"`, "point_gray.png", "more than 50000 Wii Points"}},
		{archivedCheckoutData{Page: "P_07", Points: 1000}, []string{`data-card="M"`, `data-card="V"`, "card02_a.gif"}},
		{archivedCheckoutData{Page: "P_08", Points: 1000, CardType: "M", CardNumber: fakeCardNumber("M"), CardExpMonth: "1", CardExpYear: "2027", CardCode: "123"}, []string{"card02_icon.gif", "Expiration Date", `value="2027"`}},
		{archivedCheckoutData{Page: "P_12", Points: 2000, CSRF: "x"}, []string{"Wii Points Purchase Confirmation", "$0.00", `name="points" value="2000"`}},
		{archivedCheckoutData{Page: "P_15", Points: 1000, NewBalance: 1700}, []string{"Wii Points Added:", "1700", "1000 Wii Points were added"}},
	}
	for _, c := range cases {
		var b bytes.Buffer
		if err := archivedShopTemplates.ExecuteTemplate(&b, c.data.Page+".html", c.data); err != nil {
			t.Errorf("%s: %v", c.data.Page, err)
			continue
		}
		for _, w := range c.want {
			if !strings.Contains(b.String(), w) {
				t.Errorf("%s: missing %q", c.data.Page, w)
			}
		}
	}
	if n := fakeCardNumber("V"); len(n) != 16 || n[0] != '4' || !luhnValid(n) {
		t.Errorf("fake Visa number %q", n)
	}
	if n := fakeCardNumber("M"); n[0] != '5' || !luhnValid(n) {
		t.Errorf("fake MasterCard number %q", n)
	}
}
