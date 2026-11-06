package main

// Linking the website to a Wii. The friend code is public (people share it to
// receive gifts), so it can never double as a password. Instead the console
// shows a one-time link code that expires in 10 minutes; entering it on the
// website starts a revocable session for that console's wallet.

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v4"
)

const (
	linkTTL         = 10 * time.Minute
	linkMinInterval = 10 * time.Second
	linkFailLimit   = 8
	linkGlobalLimit = 300
	remoteCookie    = "shroombrew_remote"
)

func formatLinkCode(c string) string { return c[:4] + "-" + c[4:] }

func normalizeLinkCode(in string) string {
	var b []byte
	for _, c := range strings.ToUpper(in) {
		switch {
		case c == '-' || c == ' ':
		case strings.ContainsRune(friendAlphabet, c):
			b = append(b, byte(c))
		default:
			return ""
		}
	}
	if len(b) != 8 {
		return ""
	}
	return string(b)
}

// createLinkCode replaces any earlier unused code for the wallet.
func createLinkCode(ctx context.Context, wallet string) (string, time.Time, error) {
	var sinceLast float64
	err := pool.QueryRow(ctx, `SELECT COALESCE(EXTRACT(EPOCH FROM (now() - max(created_at))), 1e9) FROM shroombrew_links WHERE wallet=$1`, wallet).Scan(&sinceLast)
	if err != nil {
		return "", time.Time{}, err
	}
	if wait := linkMinInterval - time.Duration(sinceLast*float64(time.Second)); wait > 0 {
		return "", time.Time{}, &waitError{Key: "er_wait", Wait: wait, Msg: "Please wait a moment."}
	}
	code := make([]byte, 8)
	for i := range code {
		var b [1]byte
		for { // rejection sampling keeps every character equally likely
			if _, err = rand.Read(b[:]); err != nil {
				return "", time.Time{}, err
			}
			if int(b[0]) < 256-256%len(friendAlphabet) {
				break
			}
		}
		code[i] = friendAlphabet[int(b[0])%len(friendAlphabet)]
	}
	exp := time.Now().Add(linkTTL)
	tx, err := pool.Begin(ctx)
	if err != nil {
		return "", time.Time{}, err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `DELETE FROM shroombrew_links WHERE wallet=$1`, wallet); err != nil {
		return "", time.Time{}, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO shroombrew_links(code_hash, wallet, expires_at) VALUES($1,$2,$3)`, hmacHex("link", string(code)), wallet, exp); err != nil {
		return "", time.Time{}, err
	}
	return string(code), exp, tx.Commit(ctx)
}

// redeemLinkCode trades a valid code for a new remote session token.
func redeemLinkCode(ctx context.Context, raw, ip string) (string, error) {
	if isBanned(ctx, ip) {
		return "", errBanned
	}
	bi, bg := hmacHex("link-ip", ipBucket(ip)), "link-global"
	for _, c := range []struct {
		b string
		n int
	}{{bi, linkFailLimit}, {bg, linkGlobalLimit}} {
		wait, err := lockoutWait(ctx, c.b, c.n)
		if err != nil {
			return "", err
		}
		if wait > 0 {
			return "", &waitError{Key: "er_locked", Wait: wait, Msg: "Too many wrong tries. Try again later."}
		}
	}
	code := normalizeLinkCode(raw)
	var wallet string
	if code != "" {
		err := pool.QueryRow(ctx, `DELETE FROM shroombrew_links WHERE code_hash=$1 AND expires_at > now() RETURNING wallet`, hmacHex("link", code)).Scan(&wallet)
		if err != nil && err != pgx.ErrNoRows {
			return "", err
		}
	}
	if wallet == "" {
		recordFail(ctx, bi, bg)
		return "", &userError{"er_link_bad", "That link code is wrong or expired."}
	}
	token, err := randomHex(32)
	if err != nil {
		return "", err
	}
	if _, err = pool.Exec(ctx, `INSERT INTO shroombrew_remote(token_hash, wallet) VALUES($1,$2)`, tokenHash(token), wallet); err != nil {
		return "", err
	}
	return token, nil
}

func tokenHash(token string) string {
	h := sha256.Sum256([]byte("shroombrew-remote:" + token))
	return hex.EncodeToString(h[:])
}

// remoteToken reads the website session: the cookie on this host, or a bearer
// header from the GitHub Pages copy, which cannot rely on third-party cookies.
func remoteToken(r *http.Request) string {
	t := ""
	if c, err := r.Cookie(remoteCookie); err == nil {
		t = c.Value
	} else if a := r.Header.Get("Authorization"); strings.HasPrefix(a, "Bearer ") {
		t = strings.TrimSpace(strings.TrimPrefix(a, "Bearer "))
	}
	if len(t) != 64 {
		return ""
	}
	return t
}

// remoteWallet resolves the website session to a wallet.
func remoteWallet(ctx context.Context, r *http.Request) (string, bool) {
	t := remoteToken(r)
	if t == "" {
		return "", false
	}
	var wallet string
	err := pool.QueryRow(ctx, `UPDATE shroombrew_remote SET last_seen=now() WHERE token_hash=$1 RETURNING wallet`, tokenHash(t)).Scan(&wallet)
	if err != nil {
		return "", false
	}
	return wallet, true
}

func dropRemote(ctx context.Context, r *http.Request) {
	if t := remoteToken(r); t != "" {
		_, _ = pool.Exec(ctx, `DELETE FROM shroombrew_remote WHERE token_hash=$1`, tokenHash(t))
	}
}

// fromPages reports a browser request sent by the community site.
func fromPages(r *http.Request) bool {
	o := r.Header.Get("Origin")
	if o == "" {
		return false
	}
	for _, allowed := range pagesOrigins() {
		if o == allowed {
			return true
		}
	}
	return false
}

// pagesCORS lets the Pages site read the API, the status check and the shop
// font. It answers preflights itself and reports whether the request is done.
func pagesCORS(w http.ResponseWriter, r *http.Request) bool {
	if !fromPages(r) {
		return false
	}
	h := w.Header()
	// fromPages matched it exactly, so echoing it never widens access.
	h.Set("Access-Control-Allow-Origin", r.Header.Get("Origin"))
	h.Add("Vary", "Origin")
	if r.Method == http.MethodOptions {
		h.Set("Access-Control-Allow-Methods", "GET, POST")
		h.Set("Access-Control-Allow-Headers", "Content-Type, X-Shroombrew, Authorization")
		h.Set("Access-Control-Max-Age", "600")
		w.WriteHeader(http.StatusNoContent)
		return true
	}
	return false
}

// revokeRemote signs every website session out of a wallet.
func revokeRemote(ctx context.Context, wallet string) (int64, error) {
	tag, err := pool.Exec(ctx, `DELETE FROM shroombrew_remote WHERE wallet=$1`, wallet)
	return tag.RowsAffected(), err
}

func countRemote(ctx context.Context, wallet string) int {
	var n int
	_ = pool.QueryRow(ctx, `SELECT count(*) FROM shroombrew_remote WHERE wallet=$1`, wallet).Scan(&n)
	return n
}

func secureRequest(r *http.Request) bool {
	return r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https")
}

func setRemoteCookie(w http.ResponseWriter, r *http.Request, token string) {
	http.SetCookie(w, &http.Cookie{Name: remoteCookie, Value: token, Path: "/", MaxAge: 60 * 60 * 24 * 30, HttpOnly: true, Secure: secureRequest(r), SameSite: http.SameSiteStrictMode})
}

func clearRemoteCookie(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{Name: remoteCookie, Value: "", Path: "/", MaxAge: -1, HttpOnly: true, Secure: secureRequest(r), SameSite: http.SameSiteStrictMode})
}

var errNoWallet = errors.New("no wallet")
