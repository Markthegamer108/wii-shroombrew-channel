package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"html"
	"regexp"
	"strings"
)

// The pages the server builds itself (points, settings, help, friends, ...) use the
// same skeleton as the shop's own pages: its stylesheet and scripts, the Wii font,
// the title class, the two dotted rules (one under the title, one above the buttons
// at the bottom of the screen) and the under-banner buttons. Callers still pass a
// body plus an optional trailing <div class="foot"> of links; those links become
// the real Back / Next banner buttons.

var (
	shellFootRe = regexp.MustCompile(`(?s)<div class="foot">(.*)</div>\s*$`)
	shellLinkRe = regexp.MustCompile(`(?s)<a\b[^>]*?href="([^"]*)"[^>]*>(.*?)</a>`)
	shellTagRe  = regexp.MustCompile(`<[^>]*>`)
	newsHeadRe  = regexp.MustCompile(`^<div class="panel news" data-head="([^"]*)">`)
)

type shellButton struct{ label, link string }

// compatURL names the bridge script by its content, because the console's
// browser keeps scripts it has seen and would otherwise run an old copy.
var compatURL = func() string {
	data, _ := shopFiles.ReadFile("shop/compat.js")
	sum := sha256.Sum256(data)
	return "/shroombrew-compat.js?v=" + hex.EncodeToString(sum[:4])
}()

func shellJS(s string) string {
	b, _ := json.Marshal(s) // escapes <, > and & so it is safe inside <script>
	return string(b)
}

func shellLink(href string) string {
	href = html.UnescapeString(href)
	switch {
	case href == "javascript:history.back()":
		return "javascript:showBack()"
	case strings.HasPrefix(href, "javascript:"):
		return href
	}
	return "javascript:shroomNavigate(" + shellJS(href) + ")"
}

// splitFoot separates the trailing button row from the page body.
func splitFoot(content string) (string, []shellButton) {
	m := shellFootRe.FindStringSubmatchIndex(content)
	if m == nil {
		return content, nil
	}
	var buttons []shellButton
	for _, l := range shellLinkRe.FindAllStringSubmatch(content[m[2]:m[3]], -1) {
		label := strings.TrimSpace(html.UnescapeString(shellTagRe.ReplaceAllString(l[2], "")))
		if label != "" {
			buttons = append(buttons, shellButton{label, shellLink(l[1])})
		}
	}
	return content[:m[0]], buttons
}

const shellDots = "････････････････････････････････････････････････････････････････････････････"

// The original shop drew nothing in CSS: every button, row and frame is one of
// its own images with text laid over it, and hovering swaps *_a for *_b. The
// classes below reproduce that with the shop's images. Scrolling lists sit in
// a square frame with a 1px #35beed border (the list frame of B_04/S_01); plain
// pages put their text straight on the white page, as L_03 and B_10 do.
const shellImg = "/oss/oss/common/images//banner/"

const shellStyle = `
@font-face{font-family:'Wii NTLG PGothic JPN Regular';src:url(/oss/serv/font2.ttf)}
@font-face{font-family:'PW-RodinNTLG TT M';src:url(/oss/serv/font2.ttf)}
#text01-01{position:absolute;left:29px;top:28px;width:442px;height:28px;z-index:38}
#shopPanel{position:absolute;left:35px;top:88px;width:540px;height:268px;z-index:30;overflow:auto;overflow-x:hidden;border:1px solid #35beed;background:#fff}
#shopPanel.plain{left:34px;top:80px;width:540px;height:276px;border:0;overflow:auto;overflow-x:hidden}
#shopPanel .inner{padding:8px 0;font-family:'Wii NTLG PGothic JPN Regular',sans-serif;font-size:16px;line-height:21px;color:#323232}
#shopPanel p{margin:0 12px 10px}
#shopPanel.plain p{margin:0 0 10px}
#shopPanel h2{font-size:16px;line-height:20px;margin:12px 12px 6px;color:#34beed;font-weight:bold}
#shopPanel .inner h2:first-child{margin-top:0}
#shopPanel .small{font-size:14px;line-height:18px;color:#8c8c8c}
#shopPanel .blue{color:#34beed}
#shopPanel .big{font-size:24px;line-height:30px;color:#34beed;letter-spacing:1px}
#shopPanel .center{text-align:center}
#shopPanel .err{color:#ff0000}
#shopPanel a{color:#34beed}
a.opt,button.opt{display:inline-block;position:relative;width:184px;height:59px;line-height:59px;padding:0;margin:0 8px 8px 0;border:0;vertical-align:middle;text-align:center;text-decoration:none;white-space:nowrap;overflow:hidden;cursor:pointer;
 font-family:'Wii NTLG PGothic JPN Regular',sans-serif;font-size:17px;color:#323232;background:url(` + shellImg + `option2_a.png) no-repeat}
a.opt:hover,button.opt:hover{background-image:url(` + shellImg + `option2_b.png)}
#shopPanel a.opt,#shopPanel button.opt{color:#323232}
a.menu{display:block;position:relative;width:477px;height:94px;margin:0 0 1px 5px;text-decoration:none;background-repeat:no-repeat}
a.menu span{position:absolute;left:146px;top:5px;width:300px;height:80px;line-height:80px;font-family:'Wii NTLG PGothic JPN Regular',sans-serif;font-size:16px;font-weight:bold;color:#34beed;white-space:nowrap;overflow:hidden}
a.m-connect{background-image:url(` + shellImg + `connect_a.gif)}a.m-connect:hover{background-image:url(` + shellImg + `connect_b.gif)}
a.m-keys{background-image:url(` + shellImg + `accessible_a.gif)}a.m-keys:hover{background-image:url(` + shellImg + `accessible_b.gif)}
a.row{display:block;position:relative;width:488px;height:91px;margin:0 0 21px 0;text-decoration:none;background:url(` + shellImg + `B_04_softbanner_a.gif) no-repeat}
a.row:hover{background-image:url(` + shellImg + `B_04_softbanner_b.gif)}
a.row .thumb{position:absolute;left:18px;top:9px;width:96px;height:72px;background:#fff;text-align:center}
a.row .thumb img{margin-top:18px}
a.row .name{position:absolute;left:130px;top:12px;width:340px;height:26px;line-height:26px;overflow:hidden;white-space:nowrap;font-family:'Wii NTLG PGothic JPN Regular',sans-serif;font-size:18px;color:#323232}
a.row .sub{position:absolute;left:130px;top:52px;width:340px;height:22px;line-height:22px;overflow:hidden;white-space:nowrap;font-family:'Wii NTLG PGothic JPN Regular',sans-serif;font-size:14px;color:#34beed}
a.dest{display:inline-block;position:relative;width:236px;height:214px;margin:0 10px;text-decoration:none;text-align:center;vertical-align:top}
a.dest img{display:block;border:0}
a.dest span{display:block;height:30px;line-height:30px;font-family:'Wii NTLG PGothic JPN Regular',sans-serif;font-size:18px;color:#323232}
#shopPanel.tiles{background:#c4fbff}
a.tile{display:inline-block;position:relative;width:235px;height:53px;margin:0 0 1px 2px;text-decoration:none;background:url(` + shellImg + `B_17_publisher_a_00.png) no-repeat;vertical-align:top}
a.tile:hover{background-image:url(` + shellImg + `B_17_publisher_a_01.png)}
a.tile span{position:absolute;left:12px;top:3px;width:211px;height:24px;line-height:24px;font-family:'Wii NTLG PGothic JPN Regular',sans-serif;font-size:16px;color:#323232;overflow:hidden;white-space:nowrap}
a.tile.on span{color:#34beed;font-weight:bold}
table.aa{width:460px;margin:0 12px;border-collapse:collapse}
#shopPanel.tiles .inner{padding:4px 0 4px 4px}
table.aa th{font-size:14px;font-weight:bold;color:#34beed;text-align:left;padding:0 6px 6px 0}
table.aa td{font-size:14px;line-height:18px;padding:9px 6px 9px 0;border-top:1px solid #35beed;vertical-align:top}
table.aa .n{text-align:right;white-space:nowrap}
#shopPanel input{font-family:'Wii NTLG PGothic JPN Regular',sans-serif;font-size:18px;color:#323232;border:2px solid #cccccc;background:#fff;padding:6px 8px;margin:0 8px 8px 0;vertical-align:middle}
#shopPanel input:hover{background:#e6e6e6}
#newsBox{position:absolute;left:35px;top:80px;width:540px;height:272px;z-index:30}
#newsBox .head{position:absolute;left:9px;top:2px;font-family:'Wii NTLG PGothic JPN Regular',sans-serif;font-size:16px;font-weight:bold;color:#fff}
#newsBox .body{position:absolute;left:4px;top:27px;width:522px;height:236px;overflow:auto;padding:4px;font-family:'Wii NTLG PGothic JPN Regular',sans-serif;font-size:16px;line-height:21px;color:#323232}
#newsBox .body p{margin:0 0 10px}
#newsBox .blue{color:#34beed}
#upperLineLong{position:absolute;left:0;top:59px;width:608px;height:10px;z-index:100}
.dot{white-space:nowrap;font-family:'Wii NTLG PGothic JPN Regular' !important;font-size:16px;color:#8C8C8C}
#line01{position:absolute;left:0;top:59px;width:608px;height:10px;z-index:98}
#line02{position:absolute;left:0;top:363px;width:608px;height:10px;z-index:100}
`

// Sounds the shop plays on its own banners; guarded because the desktop copy
// of the page may load before shop.js defines them.
const shellSnd = ` onmouseover="try{wiiFocusSound()}catch(e){}"`

// uiButton is the shop's small pill (option2, used for Account Activity and
// Settings on the main menu) as a link.
func uiButton(href, label string) string {
	return `<a class="opt" onclick="try{wiiSelectSound()}catch(e){}shroomNavigate(this.href);return false" href="` + html.EscapeString(href) + `"` + shellSnd + `>` + html.EscapeString(label) + `</a>`
}

// uiSubmit is the same pill as a form button.
func uiSubmit(name, value, label string) string {
	attrs := ""
	if name != "" {
		attrs = ` name="` + html.EscapeString(name) + `" value="` + html.EscapeString(value) + `"`
	}
	return `<button class="opt" type="submit"` + attrs + shellSnd + `>` + html.EscapeString(label) + `</button>`
}

// uiMenu is a 477x94 Settings and Features banner; kind picks its icon
// ("connect" people, "keys" keypad).
func uiMenu(href, kind, label string) string {
	return `<a class="menu m-` + kind + `" onclick="try{wiiSelectSound()}catch(e){}shroomNavigate(this.href);return false" href="` + html.EscapeString(href) + `"` + shellSnd + `><span>` + html.EscapeString(label) + `</span></a>`
}

// uiRow is a software list row (B_04_softbanner) with the app's icon.
func uiRow(href, icon, name, sub string) string {
	return `<a class="row" onclick="try{wiiSelectSound()}catch(e){}shroomNavigate(this.href);return false" href="` + html.EscapeString(href) + `"` + shellSnd + `><span class="thumb"><img src="` + html.EscapeString(icon) + `" width="96" height="36" alt=""></span><span class="name">` + html.EscapeString(name) + `</span><span class="sub">` + html.EscapeString(sub) + `</span></a>`
}

const shellUnder = `<div id="underButtonL" style="display:none"><div id="underbannershadowL" class="buttonBannerShadow"><img src="/oss/oss/common/images//banner/under_banner_shadow.gif" width="211" height="75" /></div><div id="underbannerL" class="buttonBanner"><img src="/oss/oss/common/images//banner/under_banner_a.gif" width="187" height="55" id="underImgL" /></div><div id="underspacerL" class="buttonSpacer"><a href="" id="underlinkL"><img id="underimageL" src="/oss/oss/common/images//spacer.gif" width="187" height="55" border="0" onmouseover="MM_swapImage('underImgL','','/oss/oss/common/images//banner/under_banner_b.gif',1); if(snd) snd.playSE( cSE_Forcus );" onmouseout="MM_swapImgRestore()"/></a></div><div id="underwordL" align="center" class="buttonTextBlackM buttonWord"></div></div>
<div id="underButtonR" style="display:none"><div id="underbannershadowR" class="buttonBannerShadow"><img src="/oss/oss/common/images//banner/under_banner_shadow.gif" width="211" height="75" /></div><div id="underbannerR" class="buttonBanner"><img src="/oss/oss/common/images//banner/under_banner_a.gif" width="187" height="55" id="underImgR" /></div><div id="underspacerR" class="buttonSpacer"><a href="" id="underlinkR"><img id="underimageR" src="/oss/oss/common/images//spacer.gif" width="187" height="55" border="0" onmouseover="MM_swapImage('underImgR','','/oss/oss/common/images//banner/under_banner_b.gif',1); if(snd) snd.playSE( cSE_Forcus );" onmouseout="MM_swapImgRestore()"/></a></div><div id="underwordR" align="center" class="buttonTextBlackM buttonWord"></div></div>
<div id="underButton" style="display:none"><div id="underbannershadow" class="buttonBannerShadow"><img src="/oss/oss/common/images//banner/under_banner_shadow.gif" width="211" height="75" /></div><div id="underbanner" class="buttonBanner"><img src="/oss/oss/common/images//banner/under_banner_a.gif" width="187" height="55" id="underImg" /></div><div id="underspacer" class="buttonSpacer"><a href="" id="underlink"><img id="underimage" src="/oss/oss/common/images//spacer.gif" width="187" height="55" border="0" onmouseover="MM_swapImage('underImg','','/oss/oss/common/images//banner/under_banner_b.gif',1); if(snd) snd.playSE( cSE_Forcus );" onmouseout="MM_swapImgRestore()"/></a></div><div id="underword" align="center" class="buttonTextBlackM buttonWord"></div></div>`

const shellConst = `<div id="constElements">
  <div id="tophelpshadow"><img src="/oss/oss/common/images//banner/top_help_shadow01.gif" width="132" height="75" /></div>
  <div id="help"><img src="/oss/oss/common/images//banner/help_a.gif" name="ManualImage" width="52" height="55" border="0" id="ManualImageID" onmouseout="MM_swapImgRestore()" onmouseover="MM_swapImage('ManualImage','','/oss/oss/common/images//banner/help_b.gif',1); wiiFocusSound();" onclick="showHelp(); wiiSelectSound();"/></div>
  <div id="top"><img src="/oss/oss/common/images//banner/top_a.gif" name="TopImage" width="52" height="55" border="0" id="TopImageID" onmouseout="MM_swapImgRestore()" onmouseover="MM_swapImage('TopImage','','/oss/oss/common/images//banner/top_b.gif',1); wiiFocusSound();" onclick="showHome(); wiiCancelSound();"/></div>
  <div class="dot" id="line01">` + shellDots + `</div>
  <div class="dot" id="line02">` + shellDots + `</div>
  <div id='balanceInfo' onclick="showPoints(); wiiSelectSound();" onmouseover="MM_swapImage('PointSpacer','','/oss/oss/common/images//banner/Addpoints_everywhere.gif',1);wiiFocusSound();" onmouseout="MM_swapImgRestore();">
    <img src="/oss/oss/common/images//spacer.gif" name="PointSpacer" width="130" height="55" border="0" id='PointSpacer' style="position:absolute; top:376px; left:239px; z-index:20;"/>
    <div id="Wiipoint"><div align="center" class="buttonTextBlackM">Wii Points</div></div>
    <div id="point" class="wiiPoint"><span id="currentBalance"><script>document.write((window.parent && parent.points != null) ? parent.points : '0');</script></span></div>
  </div>
</div>`

func shopShell(title, content string) string { return shopShellOpts(title, content, true) }

// shopShellOpts can leave out the Top and Help buttons (the startup notice has
// neither; they appear once the person continues into the shop).
func shopShellOpts(title, content string, topHelp bool) string {
	body, buttons := splitFoot(content)
	consts := shellConst
	if !topHelp {
		consts = strings.Replace(consts, `<div id="help">`, `<div id="help" style="display:none">`, 1)
		consts = strings.Replace(consts, `<div id="top">`, `<div id="top" style="display:none">`, 1)
		consts = strings.Replace(consts, `<div id="tophelpshadow">`, `<div id="tophelpshadow" style="display:none">`, 1)
		// Without Top and Help the upper rule runs the full width (upperLineLong).
		consts = strings.Replace(consts, `<div class="dot" id="line01">`, `<div class="dot" id="upperLineLong">`, 1)
	}
	if len(buttons) == 0 {
		buttons = []shellButton{{"Back", "javascript:showBack()"}}
	}
	// A screen whose only button moves forward (the startup notice) shows it on
	// the right, where the shop puts OK and Continue.
	forwardOnly := len(buttons) == 1 && !topHelp
	init := "var st='start';try{" +
		// The shop keeps showing its waiting icon, and ignores the A button, until a
		// page ends the wait; the real pages do this through setConnectingToServer(false).
		"st='wait';if(typeof wiiShop!='undefined'){var sh=new wiiShop();if('endWaiting' in sh)sh.endWaiting();if('enableHRP' in sh)sh.enableHRP();}" +
		"document.onmousedown=function(){};" +
		// Load check: a script this browser cannot parse leaves its functions undefined
		// and every button dead, so say which one is missing.
		"var miss='';if(typeof shroomNavigate!='function')miss+=' compat';if(typeof setUnderButtonL!='function')miss+=' buttons';if(typeof MM_preloadImages!='function')miss+=' images';" +
		// Hover images load up front so a highlighted banner never flashes blank.
		"st='preload';MM_preloadImages('" + shellImg + "under_banner_b.gif','" + shellImg + "option2_b.png','" + shellImg + "B_04_softbanner_b.gif','" + shellImg + "connect_b.gif','" + shellImg + "accessible_b.gif','" + shellImg + "B_17_publisher_a_01.png');" +
		// setCursor only exists in the desktop copy of oss.js; the console's has none.
		"st='cursor';if(typeof setCursor=='function'){setCursor(document);}"
	if forwardOnly {
		init += "st='underR';setUnderButtonR(true," + shellJS(buttons[0].label) + "," + shellJS(buttons[0].link) + ",\"if(snd) snd.playSE(cSE_Decide)\");"
	} else {
		init += "st='underL';setUnderButtonL(true," + shellJS(buttons[0].label) + "," + shellJS(buttons[0].link) + ",null);setButtonOnClick('underimageL',\"if(snd) snd.playSE(cSE_Cancel)\");"
	}
	if len(buttons) > 1 {
		init += "st='underR';if(document.getElementById('underButtonR')){setUnderButtonR(true," + shellJS(buttons[1].label) + "," + shellJS(buttons[1].link) + ",\"if(snd) snd.playSE(cSE_Decide)\");}"
	}
	init += "}catch(e){}"
	t := html.EscapeString(title)
	// The body's outer class picks how its content sits on the page: in the
	// bordered list frame (default), straight on the page ("plain"), on the
	// light-blue search grid ("tiles"), or in the News box ("news").
	panel := `<div id="shopPanel"><div class="inner">` + body + `</div></div>`
	switch {
	case strings.HasPrefix(body, `<div class="panel plain">`):
		panel = `<div id="shopPanel" class="plain"><div class="inner">` + body + `</div></div>`
	case strings.HasPrefix(body, `<div class="panel tiles">`):
		panel = `<div id="shopPanel" class="tiles"><div class="inner">` + body + `</div></div>`
	case strings.HasPrefix(body, `<div class="panel news"`):
		head := ""
		if m := newsHeadRe.FindStringSubmatch(body); m != nil {
			head = m[1]
		}
		panel = `<div id="newsBox"><img src="` + shellImg + `box.gif" width="540" height="272" alt=""><div class="head">` + head + `</div><div class="body">` + body + `</div></div>`
	}
	return `<!DOCTYPE html><html><head><meta http-equiv="Content-Type" content="text/html;charset=UTF-8" />
<script src="` + compatURL + `"></script>
<link href="/oss/oss/common/css/oss.css" rel="stylesheet" type="text/css" />
<link href="/oss/oss/common/css/error.css" rel="stylesheet" type="text/css" />
<script type="text/JavaScript">var ecCheck=false,errorCheck=false,buttonsCheck=false,imagesCheck=false,soundCheck=false,shopCheck=false,ossCheck=false;</script>
<script language="JavaScript" src="/oss/oss/common/js//ec.js"></script>
<script language="JavaScript" src="/oss/oss/common/js//error.js"></script>
<script language="JavaScript" src="/oss/oss/common/js//buttons.js"></script>
<script language="JavaScript" src="/oss/oss/common/js//images.js"></script>
<script language="JavaScript" src="/oss/oss/common/js//sound.js"></script>
<script language="JavaScript" src="/oss/oss/common/js//shop.js"></script>
<script language="JavaScript" src="/oss/oss/common/js//oss.js"></script>
<title>` + t + `</title>
<script>try { if (typeof wiiShop != 'undefined') { var shroombrewShopHandle = new wiiShop(); var shroombrewConnecting = shroombrewShopHandle.connecting; } } catch(e) {}</script>
<style type="text/css">` + shellStyle + `</style>
<script type="text/JavaScript">function initPage(){` + init + `}</script>
</head>
<body onload="initPage();" style="overflow: hidden;">
` + consts + `
` + shellUnder + `
<div style="position:absolute;left:0;top:0;width:1px;height:1px;z-index:6"><img src="/oss/oss/common/images//spacer.gif" name="ktouch" width="1" height="1" border="0" id="ktouch"/></div>
<div id="text01-01"><div align="left" class="titleBlackL">` + t + `</div></div>
` + panel + `
<script src="/shroombrew-balance.js"></script>
<script>shroomInstallBridge();</script>
</body></html>`
}
