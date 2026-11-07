package main

// Pages shown inside the Wii Shop Channel, translated to the console's
// System Menu language: points, friends, website link, startup notice, help,
// news, settings and account activity.

import (
	"context"
	"fmt"
	"html"
	"html/template"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

func shopFootT(lang, back string) string {
	return `<div class="foot">` + shopLink(back, tl(lang, "w_back")) + ` &nbsp; ` + shopLink("/oss/serv/W_03.html", tl(lang, "w_home")) + `</div>`
}

func isWiiShopUA(r *http.Request) bool {
	return strings.Contains(r.UserAgent(), "Wii Shop Channel/")
}

// pointCardImage is the retail Wii Points Card the Add Wii Points page shows:
// the console region's card, as the shop picked it (P_01_getRegionPic).
func pointCardImage(r *http.Request) string {
	switch regionFor(r) {
	case "EU":
		return "pointcard_pic_EU.gif"
	case "JP", "KR":
		return "pointcard_pic_JP.gif"
	}
	return "pointcard_pic_US.gif"
}

// ---- points: add points, then redeem a card number ---------------------------

// pointsResult turns the ?m= code the card form redirects with into the one
// line message shown on the card page.
func pointsResult(lang string, q url.Values) (template.HTML, bool) {
	switch q.Get("m") {
	case "ok":
		return template.HTML(tl(lang, "c_ok", "n", strconv.Itoa(cardPoints))), false
	case "err":
		return template.HTML(tl(lang, "er_card_bad")), true
	case "cap":
		return template.HTML(tl(lang, "er_card_cap", "n", strconv.Itoa(walletCap))), true
	case "locked":
		n, _ := strconv.Atoi(q.Get("n"))
		if n < 1 || n > 3600 {
			n = 60
		}
		return template.HTML(tl(lang, "er_locked", "m", strconv.Itoa((n+59)/60))), true
	case "fail":
		return template.HTML(tl(lang, "er_generic")), true
	}
	return "", false
}

// freePointsPage is the original Add Wii Points landing screen: the two ways to
// add points, exactly as the shop laid them out.
func (s *storefront) freePointsPage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		// The card form used to post here; send old copies to the card page.
		http.Redirect(w, r, "/points/card?"+r.URL.RawQuery, http.StatusSeeOther)
		return
	}
	id, err := walletID(w, r)
	if err != nil {
		http.Error(w, "Wallet unavailable", 500)
		return
	}
	points, err := walletBalance(id)
	if err != nil {
		http.Error(w, "Wallet unavailable", 500)
		return
	}
	lang := langFor(r)
	writeArchivedCheckout(w, r, archivedCheckoutData{
		Page: "P_01", Balance: points, Back: archivedPointsReturn(r),
		T:    tmap(lang, "c_title", "c_sub", "c_text", "c_text2", "c_redeem_a", "c_redeem_b", "c_continue_a", "c_continue_b"),
		Site: publicSite(), Card: pointCardImage(r),
	})
}

// cardEntryPage is the prepaid card screen (P_02): one activation number
// field, where the shop asked for it. OK shows what the card is worth (P_04)
// before anything is redeemed.
func (s *storefront) cardEntryPage(w http.ResponseWriter, r *http.Request) {
	id, err := walletID(w, r)
	if err != nil {
		http.Error(w, "Wallet unavailable", 500)
		return
	}
	lang := langFor(r)
	back := archivedPointsReturn(r)
	self := "/points/card?return=" + url.QueryEscape(back)
	if r.Method == http.MethodPost {
		if !validShopPost(r, id) {
			http.Error(w, "Reload the page and try again", 403)
			return
		}
		num := normalizeCard(r.Form.Get("card"))
		if num == "" {
			http.Redirect(w, r, self+"&m=err", http.StatusSeeOther)
			return
		}
		points, err := walletBalance(id)
		if err != nil {
			http.Error(w, "Wallet unavailable", 500)
			return
		}
		writeArchivedCheckout(w, r, archivedCheckoutData{
			Page: "P_04", Balance: points, Points: cardValue(r.Context(), num), CardNumber: formatCard(num),
			CSRF: csrfToken(id), Action: "/points/card/redeem?" + returnQuery(back), Back: self,
		})
		return
	}
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET, POST")
		http.Error(w, "Method not allowed", 405)
		return
	}
	points, err := walletBalance(id)
	if err != nil {
		http.Error(w, "Wallet unavailable", 500)
		return
	}
	flash, bad := pointsResult(lang, r.URL.Query())
	writeArchivedCheckout(w, r, archivedCheckoutData{
		Page: "P_02", Balance: points, CSRF: csrfToken(id), Action: self, Back: back,
		T:    tmap(lang, "c_r_title", "c_r_text", "c_r_label", "c_go"),
		Site: publicSite(), Flash: flash, FlashBad: bad, Card: pointCardImage(r),
	})
}

// ---- friends ----------------------------------------------------------------

func (s *storefront) friendsPage(w http.ResponseWriter, r *http.Request) {
	lang := langFor(r)
	wallet, err := walletID(w, r)
	if err != nil {
		http.Error(w, "Wallet unavailable", 500)
		return
	}
	ctx := r.Context()
	flash := ""
	if r.Method == http.MethodPost {
		if !validShopPost(r, wallet) {
			http.Error(w, "Reload the page and try again", 403)
			return
		}
		var err error
		switch r.Form.Get("action") {
		case "redeem":
			var got int
			if got, err = redeemReferral(ctx, wallet, r.Form.Get("code")); err == nil {
				flash = tl(lang, "f_done", "n", strconv.Itoa(got))
			}
		case "accept", "decline":
			err = s.resolveGift(ctx, wallet, r.Form.Get("id"), r.Form.Get("action"))
		}
		if err != nil {
			flash = errText(lang, err)
		}
		http.Redirect(w, r, "/friends?msg="+url.QueryEscape(flash), http.StatusSeeOther)
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", 405)
		return
	}
	flash = r.URL.Query().Get("msg")
	if len(flash) > 160 {
		flash = ""
	}
	code, err := ensureFriend(ctx, wallet)
	if err != nil {
		http.Error(w, "Friends unavailable", 500)
		return
	}
	n, _ := referralCount(ctx, wallet)
	points, _ := walletBalance(wallet)
	incoming, _, _ := s.listGifts(ctx, wallet)
	ups, _ := s.ownedUpdates(ctx, wallet)
	esc := html.EscapeString
	tier := tierFor(n)
	if tier == "" {
		tier = tl(lang, "f_none")
	}
	csrf := csrfToken(wallet)
	_ = points // the balance is already shown under the page, as on every shop screen
	var b strings.Builder
	b.WriteString(`<div class="panel">`)
	if flash != "" {
		b.WriteString(`<p class="blue"><b>` + esc(flash) + `</b></p>`)
	}
	fmt.Fprintf(&b, `<h2>%s</h2><p class="big">%s</p><p class="small">%s</p>`, esc(tl(lang, "f_code")), esc(code), esc(tl(lang, "f_share")))
	fmt.Fprintf(&b, `<p>%s: <b>%s</b> (%d %s)</p>`, esc(tl(lang, "f_level")), esc(tier), n, esc(tl(lang, "f_refs")))
	fmt.Fprintf(&b, `<h2>%s</h2><form method="post" action="/friends"><input type="hidden" name="csrf" value="%s"><input type="hidden" name="action" value="redeem"><p><input name="code" maxlength="14" size="14">%s</p></form>`, esc(tl(lang, "f_enter")), csrf, uiSubmit("", "", tl(lang, "f_redeem")))
	b.WriteString(`<h2>` + esc(tl(lang, "f_in")) + `</h2>`)
	if len(incoming) == 0 {
		b.WriteString(`<p class="small">` + esc(tl(lang, "f_nogifts")) + `</p>`)
	}
	for _, g := range incoming {
		fmt.Fprintf(&b, `<form method="post" action="/friends"><input type="hidden" name="csrf" value="%s"><input type="hidden" name="id" value="%s"><p><b>%s</b> %s %s</p><p>%s%s</p></form>`,
			csrf, esc(g.ID), esc(g.Name), esc(tl(lang, "f_from")), esc(g.Other), uiSubmit("action", "accept", tl(lang, "f_accept")), uiSubmit("action", "decline", tl(lang, "f_decline")))
	}
	b.WriteString(`<h2>` + esc(tl(lang, "f_updates")) + `</h2>`)
	if len(ups) == 0 {
		b.WriteString(`<p class="small">` + esc(tl(lang, "f_noupdates")) + `</p>`)
	}
	for _, u := range ups {
		b.WriteString(uiRow("/download/"+u.Slug, "/icons/"+u.Slug+".png", u.Name, tl(lang, "f_update_btn")+" (v"+fmt.Sprint(u.Latest)+")"))
	}
	b.WriteString(`<p></p>` + uiMenu("/link", "keys", tl(lang, "l_title")) + `</div>` + shopFootT(lang, "/settings"))
	writeShopPage(w, tl(lang, "f_title"), b.String())
}

// ---- link the website to this Wii ------------------------------------------

func (s *storefront) linkPage(w http.ResponseWriter, r *http.Request) {
	lang := langFor(r)
	wallet, err := walletID(w, r)
	if err != nil {
		http.Error(w, "Wallet unavailable", 500)
		return
	}
	ctx := r.Context()
	esc := html.EscapeString
	csrf := csrfToken(wallet)
	var b strings.Builder
	b.WriteString(`<div class="panel plain">`)
	switch r.Method {
	case http.MethodPost:
		if !validShopPost(r, wallet) {
			http.Error(w, "Reload the page and try again", 403)
			return
		}
		if r.Form.Get("action") == "revoke" {
			_, _ = revokeRemote(ctx, wallet)
			b.WriteString(`<p class="blue center"><b>` + esc(tl(lang, "l_signed_out")) + `</b></p>`)
			break
		}
		code, _, err := createLinkCode(ctx, wallet)
		if err != nil {
			b.WriteString(`<p class="err center">` + esc(errText(lang, err)) + `</p>`)
			break
		}
		fmt.Fprintf(&b, `<p class="center">%s</p><p class="center big" style="font-size:30px;line-height:40px;letter-spacing:4px">%s</p><p class="center small">%s</p>`,
			esc(tl(lang, "l_go", "site", publicSite())), esc(formatLinkCode(code)), esc(tl(lang, "l_once")))
	case http.MethodGet:
		fmt.Fprintf(&b, `<p class="center">%s</p>`, esc(tl(lang, "l_intro", "site", publicSite())))
	default:
		http.Error(w, "Method not allowed", 405)
		return
	}
	fmt.Fprintf(&b, `<form method="post" action="/link"><input type="hidden" name="csrf" value="%s"><p class="center">%s</p>`, csrf, uiSubmit("action", "new", tl(lang, "l_new")))
	if n := countRemote(ctx, wallet); n > 0 {
		fmt.Fprintf(&b, `<p class="center small">%s</p><p class="center">%s</p>`, esc(tl(lang, "l_sessions", "n", strconv.Itoa(n))), uiSubmit("action", "revoke", tl(lang, "l_signout")))
	}
	b.WriteString(`</form></div>` + shopFootT(lang, "/friends"))
	writeShopPage(w, tl(lang, "l_title"), b.String())
}

// ---- startup notice ---------------------------------------------------------

// noticeWanted decides whether the "free service" notice shows at launch.
func noticeWanted(r *http.Request) bool {
	if os.Getenv("SHROOMBREW_NOTICE") == "0" || !isWiiShopUA(r) {
		return false
	}
	if c, err := r.Cookie("shroombrew_notice"); err == nil && noticeHiddenRecently(c.Value) {
		return false
	}
	if c, err := r.Cookie("shroombrew_wallet"); err == nil && validWalletID(c.Value) && pool != nil {
		if noticeHiddenRecently(getPref(r.Context(), c.Value, "notice_off")) {
			return false
		}
	}
	return true
}

// The notice may be hidden, but it comes back after 30 days so the "this is
// free" message keeps reaching people who might be sold a copy later.
const noticeReminder = 30 * 24 * time.Hour

// noticeHiddenRecently reads the unix time a person pressed "Don't show again".
// Older values that are not a timestamp count as expired.
func noticeHiddenRecently(v string) bool {
	ts, err := strconv.ParseInt(v, 10, 64)
	if err != nil || ts <= 0 {
		return false
	}
	return time.Since(time.Unix(ts, 0)) < noticeReminder
}

func (s *storefront) noticePage(w http.ResponseWriter, r *http.Request) {
	lang := langFor(r)
	if r.URL.Query().Get("hide") == "1" {
		now := strconv.FormatInt(time.Now().Unix(), 10)
		http.SetCookie(w, &http.Cookie{Name: "shroombrew_notice", Value: now, Path: "/", MaxAge: 60 * 60 * 24 * 365 * 5, SameSite: http.SameSiteLaxMode})
		if c, err := r.Cookie("shroombrew_wallet"); err == nil && validWalletID(c.Value) {
			_ = setPref(r.Context(), c.Value, "notice_off", now)
		}
		http.Redirect(w, r, "/oss/serv/W_01.html", http.StatusSeeOther)
		return
	}
	esc := html.EscapeString
	// One button: continuing counts as having seen it, so it stays away for 30 days.
	// Like the original welcome notice (L_03): the text sits straight on the page.
	content := `<div class="panel plain"><p style="margin-top:14px">` + esc(tl(lang, "n_body")) + `</p></div>` +
		`<div class="foot">` + shopLink("/notice?hide=1", tl(lang, "n_continue")) + `</div>`
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	_, _ = w.Write([]byte(shopShellOpts(tl(lang, "n_title"), content, false)))
}

// connectingPage is the console's first screen, like the original F_01 page:
// "Connecting. Please wait..." in the console's own words while the shop shows
// its waiting icon and the console registers, then on to the notice or the
// welcome page.
func (s *storefront) connectingPage(w http.ResponseWriter, r *http.Request) {
	next := "/oss/serv/W_01.html"
	if noticeWanted(r) {
		next = "/notice"
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w, `<!DOCTYPE html><html><head><meta http-equiv="Content-Type" content="text/html;charset=UTF-8" />
<script src="%s"></script>
<link href="/oss/oss/common/css/oss.css" rel="stylesheet" type="text/css" />
<title>Wii Shop Channel</title>
<style type="text/css">
@font-face{font-family:'Wii NTLG PGothic JPN Regular';src:url(/oss/serv/font2.ttf)}
#text01-01{position:absolute;left:29px;top:28px;width:546px;height:28px;z-index:27}
#text02-01{position:absolute;left:25px;top:100px;width:560px;height:180px;z-index:31}
</style>
<script type="text/JavaScript">
var shroomNext = %s, shroomGone = false;
function shroomGo() { if (shroomGone) return; shroomGone = true; location.replace(shroomNext); }
window.shroomAfterRegister = function() { setTimeout(shroomGo, 300); };
function initPage() {
  try {
    if (typeof wiiShop != 'undefined') {
      var shop = new wiiShop();
      var m = document.getElementById('wiiMessage');
      if (shop.connecting && m) m.innerHTML = shop.connecting;
      // No waiting ring and no HOME/Reset/Power mid-registration: the shop
      // simply says "Connecting. Please wait..." and moves on by itself.
      if ('disableHRP' in shop) shop.disableHRP();
    }
  } catch (e) {}
  // Registration happens once per launch; when it has already run, move on.
  setTimeout(shroomGo, document.cookie.indexOf('shroombrew_reg=1') != -1 ? 800 : 5000);
}
</script>
</head>
<body onload="initPage()" style="overflow:hidden">
<div class="dot" id="line01">%s</div>
<div class="dot" id="line02">%s</div>
<div class="titleBlueL" id="text01-01"><span id="wiiTitle">Wii Shop Channel</span></div>
<div align="center" class="titleBlueL" id="text02-01"><span id="wiiMessage">Connecting. Please wait...</span></div>
</body></html>`, compatURL, shellJS(next), shellDots, shellDots)
}

// ---- help, news, settings, activity ----------------------------------------

func (s *storefront) featureInfo(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", 405)
		return
	}
	lang := langFor(r)
	esc := html.EscapeString
	title := tl(lang, "w_help_title")
	content := `<h2>` + esc(tl(lang, "w_help_h1")) + `</h2><p>` + esc(tl(lang, "w_help_1", "site", publicSite())) + `</p><h2>` + esc(tl(lang, "w_help_h2")) + `</h2><p>` + esc(tl(lang, "w_help_2")) + `</p><p>` + esc(tl(lang, "w_help_3")) + `</p>`
	back := "/oss/serv/W_03.html"
	panel := `<div class="panel">`
	switch r.URL.Path {
	case "/news", "/oss/serv/W_02.html":
		// The original News page: a white box with a blue header bar (box.gif).
		title = tl(lang, "w_news_title")
		panel = `<div class="panel news" data-head="` + esc(tl(lang, "w_news_head")) + `">`
		content = `<p class="blue"><b>` + esc(tl(lang, "w_news_1")) + `</b></p><p>` + esc(tl(lang, "w_news_2")) + `</p><p>` + esc(tl(lang, "n_body")) + `</p>`
		back = "/oss/serv/W_01.html"
	case "/controllers":
		title = tl(lang, "w_ctrl_title")
		id := strings.ToUpper(r.URL.Query().Get("titleId"))
		app, ok := s.appByTitleID(id)
		if !ok {
			http.NotFound(w, r)
			return
		}
		controls := controllerText(app)
		if controls == "" {
			controls = tl(lang, "w_ctrl_none")
		}
		// The controller images the real B_08 page showed, one per controller.
		icons := ""
		for _, key := range controllerIcons(app) {
			if name, known := controllerNames[key]; known {
				icons += `<img src="/oss/oss/common/images/banner/B_08_` + key + `.gif" width="68" height="51" alt="` + esc(name) + `" style="margin:0 4px">`
			}
		}
		panel = `<div class="panel plain">`
		content = `<p class="blue" style="font-size:18px">` + esc(app.Name) + `</p>`
		if icons != "" {
			content += `<p class="center">` + icons + `</p>`
		}
		content += `<p>` + esc(controls) + `</p>`
		if extra := controllerExtra(app); extra != "" {
			content += `<p class="small">` + esc(extra) + `</p>`
		}
		if app.Players != "" {
			content += `<p>` + esc(app.Players) + `</p>`
		}
		back = "/oss/serv/B_05.html?titleId=" + id
	case "/settings":
		title = tl(lang, "w_settings_title")
		// Like the original S_01: a column of icon banners, then the shop's
		// own small pills for the two settings that are a simple choice.
		music := func(on bool, key, status string) string {
			return `<a class="opt" href="#"` + shellSnd + ` onclick="try{wiiSelectSound()}catch(e){}shroomSetMusic(` + strconv.FormatBool(on) + `);document.getElementById('settingStatus').innerHTML=` + esc(shellJS(tl(lang, status))) + `;return false">` + esc(tl(lang, key)) + `</a>`
		}
		content = uiMenu("/friends", "connect", tl(lang, "f_title")) + uiMenu("/link", "keys", tl(lang, "l_title")) +
			`<h2>` + esc(tl(lang, "w_music")) + `</h2><p>` + music(true, "w_music_on", "w_music_enabled") + music(false, "w_music_off", "w_music_muted") + `</p>` +
			`<p id="settingStatus" class="small">` + esc(tl(lang, "w_settings_note")) + `</p>` +
			`<h2>` + esc(tl(lang, "w_lang_label")) + `</h2><p>` + uiButton("/language", wiiLangLabel(lang)) + `</p>`
	case "/activity", "/downloaded":
		title = tl(lang, "w_activity_title")
		if r.URL.Path == "/downloaded" {
			title = tl(lang, "w_downloaded_title")
		}
		id, err := walletID(w, r)
		if err != nil {
			http.Error(w, "Wallet unavailable", 500)
			return
		}
		points, err := walletBalance(id)
		if err != nil {
			http.Error(w, "Wallet unavailable", 500)
			return
		}
		_ = points // shown under the page like on every shop screen
		content = s.activityRows(r.Context(), lang, id, r.URL.Path == "/downloaded")
	}
	if r.URL.Path == "/help" {
		content += `<p class="small">` + esc(tl(lang, "ft_credit")) + ` <a href="/source/shroombrew-source.tar.gz">Server source</a>.</p>`
	}
	writeShopPage(w, title, panel+content+`</div>`+shopFootT(lang, back))
}

// activityRows lists purchases. Account Activity is the original H_01 table
// (date, title, points); Titles You've Downloaded uses the software list rows
// of B_04 and opens each title's details.
func (s *storefront) activityRows(ctx context.Context, lang, id string, onlyDelivered bool) string {
	esc := html.EscapeString
	rows, err := pool.Query(ctx, `SELECT slug, price, delivered_at IS NOT NULL, created_at FROM shroombrew_purchases WHERE wallet=$1 ORDER BY created_at DESC LIMIT 30`, id)
	if err != nil {
		return `<p class="err">` + esc(tl(lang, "er_generic")) + "</p>"
	}
	defer rows.Close()
	var out strings.Builder
	count := 0
	if !onlyDelivered {
		out.WriteString(`<table class="aa"><tr><th>` + esc(tl(lang, "aa_date")) + `</th><th>` + esc(tl(lang, "aa_title")) + `</th><th class="n">` + esc(tl(lang, "f_points")) + `</th></tr>`)
	}
	for rows.Next() {
		var slug string
		var price int
		var delivered bool
		var at time.Time
		if rows.Scan(&slug, &price, &delivered, &at) != nil {
			continue
		}
		app, ok := s.bySlug[slug]
		if !ok || (onlyDelivered && !delivered) {
			continue
		}
		tid := classicTitleIDForApp(app)
		if onlyDelivered {
			out.WriteString(uiRow("/oss/serv/B_05.html?titleId="+tid, "/icons/"+app.Slug+".png", app.Name, tl(lang, "w_again", "n", strconv.Itoa(price))))
		} else {
			fmt.Fprintf(&out, `<tr><td>%s</td><td><a href="/oss/serv/B_05.html?titleId=%s" onclick="shroomNavigate(this.href);return false">%s</a></td><td class="n">%d</td></tr>`, at.UTC().Format("2006/01/02"), tid, esc(app.Name), price)
		}
		count++
	}
	if !onlyDelivered {
		out.WriteString(`</table>`)
	}
	if count == 0 {
		out.WriteString(`<p class="small" style="margin-top:12px">` + esc(tl(lang, "w_none")) + "</p>")
	}
	return out.String()
}

// wiiLangLabel names a language for the console's screen. The shop's font is
// the Japanese Wii font: it has no Hangul and lacks some simplified Chinese
// characters, so those names are written in Latin letters instead of boxes.
func wiiLangLabel(code string) string {
	switch code {
	case "ko":
		return "Korean"
	case "zh-cn":
		return "Chinese (Simplified)"
	case "zh-tw":
		return "Chinese (Traditional)"
	}
	return supportedLangs[code]
}

// languagePage is the language picker, laid out like the original search
// grid: a light-blue frame of 235x53 tiles in two columns.
func (s *storefront) languagePage(w http.ResponseWriter, r *http.Request) {
	lang := langFor(r)
	esc := html.EscapeString
	tile := func(code, label string, on bool) string {
		cls := "tile"
		if on {
			cls += " on"
		}
		return `<a class="` + cls + `" onclick="try{wiiSelectSound()}catch(e){}shroomNavigate(this.href);return false" href="/language?set=` + code + `&amp;return=/settings"` + shellSnd + `><span>` + esc(label) + `</span></a>`
	}
	var b strings.Builder
	b.WriteString(`<div class="panel tiles">` + tile("auto", tl(lang, "w_lang_auto"), false))
	for _, code := range langOrder {
		b.WriteString(tile(code, wiiLangLabel(code), code == lang))
	}
	b.WriteString(`</div>` + shopFootT(lang, "/settings"))
	writeShopPage(w, tl(lang, "w_lang_title"), b.String())
}
