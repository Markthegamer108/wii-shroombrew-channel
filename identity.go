package main

// Shared helpers for per-console identity, hashing secrets and rate limiting.

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

var secretKey []byte

// initSecrets loads (or creates) the server secret used to key every hash we
// store about consoles, cards and addresses. Losing the file orphans them, so
// it is never silently replaced.
func initSecrets() error {
	path := os.Getenv("SHROOMBREW_SECRET_FILE")
	if path == "" {
		path = "./secrets.key"
	}
	b, err := os.ReadFile(path)
	if err == nil {
		if len(b) < 32 {
			return errors.New("secret file is too short; refusing to replace it")
		}
		secretKey = b[:32]
		return nil
	}
	if !os.IsNotExist(err) {
		return err
	}
	key := make([]byte, 32)
	if _, err = rand.Read(key); err != nil {
		return err
	}
	if err = os.WriteFile(path, key, 0600); err != nil {
		return err
	}
	secretKey = key
	return nil
}

// hmacHex returns a keyed hash. The purpose string keeps hashes made for one
// use from being valid for another.
func hmacHex(purpose string, parts ...string) string {
	m := hmac.New(sha256.New, secretKey)
	m.Write([]byte(purpose))
	for _, p := range parts {
		m.Write([]byte{0})
		m.Write([]byte(p))
	}
	return hex.EncodeToString(m.Sum(nil))
}

func randomHex(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func validWalletID(v string) bool {
	if len(v) != 32 {
		return false
	}
	_, err := hex.DecodeString(v)
	return err == nil
}

// siteHost is the public website named on Wii screens. Override with
// SHROOMBREW_SITE_URL when the site moves to its own domain.
func siteHost() string {
	h := strings.TrimSpace(os.Getenv("SHROOMBREW_SITE_URL"))
	h = strings.TrimPrefix(strings.TrimPrefix(h, "https://"), "http://")
	h = strings.TrimRight(h, "/")
	if h == "" {
		return "shop.shroombrew.org"
	}
	return h
}

// publicSite is the community website as written on Wii screens and in API
// replies. It can live elsewhere (GitHub Pages) while siteHost keeps routing.
func publicSite() string {
	h := strings.TrimSpace(os.Getenv("SHROOMBREW_PUBLIC_SITE"))
	h = strings.TrimRight(strings.TrimPrefix(strings.TrimPrefix(h, "https://"), "http://"), "/")
	if h == "" {
		return siteHost()
	}
	return h
}

// pagesOrigins are the other websites allowed to call the community API from
// the browser: the community site on shroombrew.org (served by GitHub Pages)
// and its old github.io address. SHROOMBREW_PAGES_ORIGIN overrides the list
// with comma-separated origins.
func pagesOrigins() []string {
	list := []string{"https://shroombrew.org", "https://www.shroombrew.org", "https://markthegamer108.github.io"}
	if env := strings.TrimSpace(os.Getenv("SHROOMBREW_PAGES_ORIGIN")); env != "" {
		list = nil
		for _, o := range strings.Split(env, ",") {
			if o = strings.TrimRight(strings.TrimSpace(o), "/"); o != "" {
				list = append(list, o)
			}
		}
	}
	return list
}

// siteHostMatches reports whether a request's Host header is the public website.
func siteHostMatches(host string) bool {
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	return strings.EqualFold(host, siteHost())
}

// clientIP trusts X-Real-IP only when the connection comes from the local
// reverse proxy, so a client cannot choose its own address.
func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	ip := net.ParseIP(host)
	if ip != nil && ip.IsLoopback() {
		if xr := net.ParseIP(strings.TrimSpace(r.Header.Get("X-Real-IP"))); xr != nil {
			ip = xr
		}
	}
	if ip == nil {
		return host
	}
	return ip.String()
}

// ipBucket groups IPv6 addresses by /64 so one household cannot dodge a limit
// by rotating addresses inside its own range.
func ipBucket(ip string) string {
	p := net.ParseIP(ip)
	if p == nil {
		return ip
	}
	if p4 := p.To4(); p4 != nil {
		return p4.String()
	}
	return p.Mask(net.CIDRMask(64, 128)).String() + "/64"
}

type rateLimiter struct {
	mu   sync.Mutex
	hits map[string][]time.Time
}

func newRateLimiter() *rateLimiter { return &rateLimiter{hits: map[string][]time.Time{}} }

// allow records a hit and reports whether the key is still within limit hits
// per window.
func (l *rateLimiter) allow(key string, limit int, window time.Duration) bool {
	now := time.Now()
	l.mu.Lock()
	defer l.mu.Unlock()
	if len(l.hits) > 20000 { // keep memory bounded under a flood of distinct keys
		for k, v := range l.hits {
			if len(v) == 0 || now.Sub(v[len(v)-1]) > window {
				delete(l.hits, k)
			}
		}
	}
	keep := l.hits[key][:0]
	for _, t := range l.hits[key] {
		if now.Sub(t) < window {
			keep = append(keep, t)
		}
	}
	if len(keep) >= limit {
		l.hits[key] = keep
		return false
	}
	l.hits[key] = append(keep, now)
	return true
}

var apiLimiter = newRateLimiter()

// setWalletCookie issues (or renews) the console's wallet cookie.
func setWalletCookie(w http.ResponseWriter, r *http.Request, id string) {
	secure := r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https") || strings.HasSuffix(r.Host, ".wiirw.bot.nu")
	http.SetCookie(w, &http.Cookie{Name: "shroombrew_wallet", Value: id, Path: "/", MaxAge: 60 * 60 * 24 * 365 * 5, HttpOnly: true, Secure: secure, SameSite: http.SameSiteLaxMode})
}
