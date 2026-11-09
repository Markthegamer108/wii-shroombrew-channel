package main

import (
	"crypto/md5"
	"fmt"
	"testing"
)

func TestSyncTokenMatches(t *testing.T) {
	const token = "AbCdEfGhIjKlMnOpQrStU"
	md5Token := fmt.Sprintf("%x", md5.Sum([]byte(token)))

	for _, request := range []string{token, "ST-" + token, "WT-" + md5Token} {
		if !syncTokenMatches(request, token) {
			t.Errorf("syncTokenMatches(%q) = false, want true", request)
		}
	}
	for _, request := range []string{"", "ST-", "WT-", token + "x", "WT-" + token} {
		if syncTokenMatches(request, token) {
			t.Errorf("syncTokenMatches(%q) = true, want false", request)
		}
	}
}

func TestNewAccountIDIsNineDigits(t *testing.T) {
	for i := 0; i < 100; i++ {
		id, err := newAccountID()
		if err != nil {
			t.Fatal(err)
		}
		if id < 100000000 || id > 999999999 {
			t.Fatalf("newAccountID() = %d, want a nine-digit value", id)
		}
	}
}
