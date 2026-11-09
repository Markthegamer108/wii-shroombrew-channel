package main

import (
	"net/http/httptest"
	"os"
	"regexp"
	"strings"
	"testing"
)

func TestSplitFootBecomesUnderButtons(t *testing.T) {
	body, btns := splitFoot(`<div class="panel"><p>x</p></div><div class="foot"><a class="btn" href="javascript:history.back()">Back</a> &nbsp; <a class="btn" onclick="shroomNavigate(this.href);return false" href="/oss/serv/W_03.html">Shop &amp; Home</a></div>`)
	if strings.Contains(body, "foot") || !strings.Contains(body, "<p>x</p>") {
		t.Fatalf("body was not separated from the button row: %q", body)
	}
	if len(btns) != 2 || btns[0].label != "Back" || btns[0].link != "javascript:showBack()" {
		t.Fatalf("back button wrong: %+v", btns)
	}
	if btns[1].label != "Shop & Home" || btns[1].link != `javascript:shroomNavigate("/oss/serv/W_03.html")` {
		t.Fatalf("second button wrong: %+v", btns[1])
	}
}

func TestShopShellUsesTheRealSkeleton(t *testing.T) {
	out := shopShell("Settings <and> Features", `<div class="panel"><p>hi</p></div>`)
	for _, want := range []string{
		`/oss/oss/common/css/oss.css`, `id="line01"`, `id="line02"`, `class="titleBlackL"`,
		`Settings &lt;and&gt; Features`, `setUnderButtonL(true,"Back","javascript:showBack()",null)`, `<p>hi</p>`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q", want)
		}
	}
	if strings.Contains(out, "Arial") || strings.Contains(out, `class="rule"`) {
		t.Error("old hand-drawn shell leaked into the page")
	}
}

func TestNoticeHasOnlyContinueAndNoTopHelp(t *testing.T) {
	out := shopShellOpts("Notice", `<div class="panel"><p>x</p></div><div class="foot">`+shopLink("/notice?hide=1", "Continue")+`</div>`, false)
	for _, want := range []string{`setUnderButtonR(true,"Continue"`, `sh.endWaiting()`, `id="upperLineLong"`} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %s", want)
		}
	}
	for _, gone := range []string{`<div id="help"`, `<div id="top"`, `<div id="tophelpshadow">`} {
		if strings.Contains(out, gone) {
			t.Errorf("notice still contains %s", gone)
		}
	}
	if strings.Contains(out, "setUnderButtonL(") {
		t.Error("notice must have a single button")
	}
	if !strings.Contains(shopShell("Help", "<p>x</p>"), `<div id="top">`) {
		t.Error("ordinary pages keep the Top button")
	}
}

// The original shop drew its buttons and rows as images; CSS pills, gradients
// and shadows are what made the server's own pages look wrong.
func TestShellUsesShopImagesNotCSSPills(t *testing.T) {
	out := shopShell("Help", `<div class="panel"><p>x</p></div><div class="foot">`+shopLink("/a", "A")+shopLink("/b", "B")+`</div>`)
	for _, bad := range []string{"linear-gradient", "box-shadow", "border-radius", `class="btn"`} {
		if strings.Contains(out, bad) {
			t.Errorf("found %q", bad)
		}
	}
	if !strings.Contains(out, "option2_a.png") || !strings.Contains(uiButton("/x", "X"), `class="opt"`) {
		t.Error("buttons must use the shop's option2 pill image")
	}
}

// The shop's rules are rows of the Wii font's middle dot (U+FF65), whose own
// spacing leaves gaps. CSS "dotted" borders pack the dots into a near-solid line.
func TestDottedRulesUseTheShopGlyph(t *testing.T) {
	// W_01 itself has no rules (wiishopchannel.net and WiiMart draw none).
	welcome := string(classicWelcomeFallbackJS())
	if strings.Contains(welcome, "dotted") || strings.Contains(welcome, "upperLineLong") {
		t.Error("welcome page must not add rules the original W_01 does not have")
	}
	if !strings.Contains(shellConst, `<div class="dot" id="line01">`+shellDots) || strings.Trim(shellDots, "･") != "" || len([]rune(shellDots)) < 75 {
		t.Error("shell rules must be at least 75 middle dots, as on the original pages")
	}
	if css, err := os.ReadFile("../community/style.css"); err == nil && regexp.MustCompile(`(?i)(border|outline)[a-z-]*\s*:[^;}]*\bdotted\b`).Match(css) {
		t.Error("website rules must use the spaced dot pattern, not a CSS dotted border")
	}
}

func TestShellPanelModes(t *testing.T) {
	for body, want := range map[string]string{
		`<div class="panel"><p>x</p></div>`:                          `<div id="shopPanel"><div class="inner">`,
		`<div class="panel plain"><p>x</p></div>`:                    `<div id="shopPanel" class="plain">`,
		`<div class="panel tiles"><p>x</p></div>`:                    `<div id="shopPanel" class="tiles">`,
		`<div class="panel news" data-head="Important Info:"></div>`: `box.gif" width="540" height="272"`,
	} {
		if out := shopShell("T", body); !strings.Contains(out, want) {
			t.Errorf("%s: missing %s", body, want)
		}
	}
	if !strings.Contains(shopShell("T", `<div class="panel news" data-head="Important Info:"></div>`), `<div class="head">Important Info:</div>`) {
		t.Error("news header text missing")
	}
}

func TestSettingsUsesBannersAndDestinationChoice(t *testing.T) {
	s := &storefront{}
	w := httptest.NewRecorder()
	s.featureInfo(w, httptest.NewRequest("GET", "/settings", nil))
	body := w.Body.String()
	for _, want := range []string{`class="menu m-connect"`, `href="/friends"`, `class="menu m-keys"`, `href="/language"`} {
		if !strings.Contains(body, want) {
			t.Errorf("settings missing %s", want)
		}
	}
	if strings.Contains(body, "<select") {
		t.Error("the language dropdown was replaced by the tile page")
	}
	w = httptest.NewRecorder()
	writeDestinationChoice(w, httptest.NewRequest("GET", "/native-ec/title/0001000154455354", nil), catalogApp{Name: "Demo"}, "0001000154455354")
	body = w.Body.String()
	for _, want := range []string{"sdToNAND_a.png", "sdToSD_a.png", "?destination=nand", "?destination=sd", "Demo"} {
		if !strings.Contains(body, want) {
			t.Errorf("destination choice missing %s", want)
		}
	}
	w = httptest.NewRecorder()
	s.setLanguage(w, httptest.NewRequest("GET", "/language", nil))
	if n := strings.Count(w.Body.String(), `<a class="tile`); w.Code != 200 || n != len(langOrder)+1 {
		t.Errorf("language page: %d tiles", n)
	}
}

// The console's browser is ES3: a reserved word used as a dotted property name
// (window.goto) makes it reject the whole compat script, and with it every button.
func TestCompatScriptAvoidsES3ReservedProperties(t *testing.T) {
	src, err := shopFiles.ReadFile("shop/compat.js")
	if err != nil {
		t.Fatal(err)
	}
	bad := regexp.MustCompile(`\.(goto|class|int|byte|char|float|double|long|short|final|native|package|private|public|static|throws|abstract|boolean|synchronized|transient|volatile|enum|export|import|extends|super|const|debugger|implements|interface|protected)\b`)
	if m := bad.Find(src); m != nil {
		t.Errorf("ES3 reserved word as property: %s", m)
	}
}
