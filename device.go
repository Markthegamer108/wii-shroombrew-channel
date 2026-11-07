package main

// Console registration. The shop client's JavaScript can read the console's
// device info (device id, serial, MAC addresses, system language). The page
// posts it here once per launch. We derive a keyed hash from it, look up the
// wallet that belongs to that console and hand it back as the wallet cookie,
// so points and purchases follow the console even if the browser cookie is lost.
// Raw identifiers are never stored or logged.

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v4"
)

type deviceInfo struct {
	ID, Serial, Wifi, BT, Language, Country, Region string
}

func cleanField(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > 64 {
		s = s[:64]
	}
	for _, c := range s {
		if c < 0x20 || c > 0x7e {
			return ""
		}
	}
	return s
}

// key identifies the console: its device id plus the first stable secondary
// identifier the console reports. Returns "" when there is not enough to go on.
func (d deviceInfo) key() string {
	id := cleanField(d.ID)
	if id == "" || id == "0" {
		return ""
	}
	second := ""
	for _, v := range []string{d.Serial, strings.ToLower(d.Wifi), strings.ToLower(d.BT)} {
		if v = cleanField(v); v != "" && v != "0" {
			second = v
			break
		}
	}
	if second == "" {
		return ""
	}
	return hmacHex("device", id, second)
}

// registerDeviceWallet finds or creates the wallet for a console. If the
// browser already carries an unclaimed wallet (points earned before consoles
// were recognised) it becomes the console's wallet, so nothing is lost.
func registerDeviceWallet(ctx context.Context, key, cookieWallet string, info deviceInfo) (wallet, lang string, err error) {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return "", "", err
	}
	defer tx.Rollback(ctx)
	err = tx.QueryRow(ctx, `SELECT wallet FROM shroombrew_devices WHERE device_key=$1 FOR UPDATE`, key).Scan(&wallet)
	switch {
	case err == nil:
		// known console
	case err == pgx.ErrNoRows:
		wallet = ""
		if validWalletID(cookieWallet) {
			var one int
			e := tx.QueryRow(ctx, `SELECT 1 FROM shroombrew_wallets w WHERE w.id=$1 AND NOT EXISTS (SELECT 1 FROM shroombrew_devices d WHERE d.wallet=w.id)`, cookieWallet).Scan(&one)
			if e == nil {
				wallet = cookieWallet
			}
		}
		if wallet == "" {
			if wallet, err = randomHex(16); err != nil {
				return "", "", err
			}
			if _, err = tx.Exec(ctx, `INSERT INTO shroombrew_wallets(id) VALUES($1)`, wallet); err != nil {
				return "", "", err
			}
		}
		if _, err = tx.Exec(ctx, `INSERT INTO shroombrew_devices(device_key, wallet) VALUES($1,$2)`, key, wallet); err != nil {
			return "", "", err
		}
	default:
		return "", "", err
	}
	lang = wiiLangCode(info.Language, info.Country)
	var pref string
	if e := tx.QueryRow(ctx, `SELECT value FROM shroombrew_prefs WHERE wallet=$1 AND key='lang'`, wallet).Scan(&pref); e == nil {
		if _, ok := supportedLangs[pref]; ok {
			lang = pref // the person chose a language in Settings
		}
	}
	if _, err = tx.Exec(ctx, `UPDATE shroombrew_devices SET language=$2, region=$3, country=$4, last_seen=now() WHERE device_key=$1`,
		key, lang, cleanField(info.Region), cleanField(info.Country)); err != nil {
		return "", "", err
	}
	return wallet, lang, tx.Commit(ctx)
}

func setLangCookie(w http.ResponseWriter, lang string) {
	http.SetCookie(w, &http.Cookie{Name: "shroombrew_lang", Value: lang, Path: "/", MaxAge: 60 * 60 * 24 * 365 * 5, SameSite: http.SameSiteLaxMode})
}

// regionGroupForDevice maps the console's reported region (a coarse group or an
// ISO country code) to the catalog region groups (US/EU/JP/KR). The coarse
// group selects which region-locked WAD a title serves, and the country cookie
// lets regionApp prefer a country-specific release (Germany, France, ...) when
// one exists. Missing or unknown region falls back to "" (regionFor defaults US).
func regionGroupForDevice(info deviceInfo) string {
	switch strings.ToUpper(strings.TrimSpace(info.Region)) {
	case "US", "EU", "JP", "KR":
		return strings.ToUpper(strings.TrimSpace(info.Region))
	}
	switch strings.ToUpper(strings.TrimSpace(info.Country)) {
	case "US", "CA", "MX":
		return "US"
	case "JP":
		return "JP"
	case "KR":
		return "KR"
	case "GB", "DE", "FR", "ES", "IT", "NL", "AU", "AT", "BE", "PT", "SE", "NO", "DK", "FI", "IE", "CH", "PL", "CZ", "GR", "RU":
		return "EU"
	}
	return ""
}

func setRegionCookies(w http.ResponseWriter, info deviceInfo) {
	country := cleanField(info.Country)
	if country != "" {
		http.SetCookie(w, &http.Cookie{Name: "shroombrew_country", Value: country, Path: "/", MaxAge: 60 * 60 * 24 * 365 * 5, SameSite: http.SameSiteLaxMode})
	}
	if rg := regionGroupForDevice(info); rg != "" {
		http.SetCookie(w, &http.Cookie{Name: "shroombrew_region", Value: rg, Path: "/", MaxAge: 60 * 60 * 24 * 365 * 5, SameSite: http.SameSiteLaxMode})
	}
}

// deviceRegister is POSTed by the Wii client script (see compat.js).
func (s *storefront) deviceRegister(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !apiLimiter.allow("register:"+ipBucket(clientIP(r)), 30, time.Minute) {
		jsonOut(w, http.StatusTooManyRequests, map[string]interface{}{"ok": 0})
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	if r.ParseForm() != nil {
		jsonOut(w, 400, map[string]interface{}{"ok": 0})
		return
	}
	info := deviceInfo{
		ID: r.Form.Get("d"), Serial: r.Form.Get("s"), Wifi: r.Form.Get("w"), BT: r.Form.Get("b"),
		Language: r.Form.Get("l"), Country: r.Form.Get("c"), Region: r.Form.Get("r"),
	}
	key := info.key()
	if key == "" {
		jsonOut(w, 400, map[string]interface{}{"ok": 0})
		return
	}
	cookieWallet := ""
	if c, err := r.Cookie("shroombrew_wallet"); err == nil {
		cookieWallet = c.Value
	}
	wallet, lang, err := registerDeviceWallet(r.Context(), key, cookieWallet, info)
	if err != nil {
		jsonOut(w, 500, map[string]interface{}{"ok": 0})
		return
	}
	changed := langFor(r) != lang || cookieWallet != wallet
	setWalletCookie(w, r, wallet)
	setLangCookie(w, lang)
	setRegionCookies(w, info)
	jsonOut(w, 200, map[string]interface{}{"ok": 1, "lang": lang, "changed": changed})
}

// setLanguage lets a person pick a language in Settings. The choice is saved
// to their wallet so it survives the next launch; "auto" goes back to following
// the Wii System Menu language.
func (s *storefront) setLanguage(w http.ResponseWriter, r *http.Request) {
	set := r.URL.Query().Get("set")
	if set == "" && r.Method == http.MethodGet {
		s.languagePage(w, r)
		return
	}
	wallet := ""
	if c, err := r.Cookie("shroombrew_wallet"); err == nil && validWalletID(c.Value) {
		wallet = c.Value
	}
	if set == "auto" {
		if wallet != "" {
			_, _ = pool.Exec(r.Context(), `DELETE FROM shroombrew_prefs WHERE wallet=$1 AND key='lang'`, wallet)
			var lang string
			if pool.QueryRow(r.Context(), `SELECT language FROM shroombrew_devices WHERE wallet=$1`, wallet).Scan(&lang) == nil {
				setLangCookie(w, lang)
			}
		}
		http.Redirect(w, r, safeLocalPath(r.URL.Query().Get("return")), http.StatusSeeOther)
		return
	}
	if _, ok := supportedLangs[set]; !ok {
		http.Error(w, "Unknown language", http.StatusBadRequest)
		return
	}
	setLangCookie(w, set)
	if wallet != "" {
		_ = setPref(r.Context(), wallet, "lang", set)
	}
	http.Redirect(w, r, safeLocalPath(r.URL.Query().Get("return")), http.StatusSeeOther)
}
