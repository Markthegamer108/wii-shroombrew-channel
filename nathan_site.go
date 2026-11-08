package main

import (
	"bytes"
	"crypto/sha1"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io/fs"
	"log"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// nathanSiteRoot is ./site in production. Go tests run from server-src, where
// the site tree is one directory up; fall back to it so the same handlers can
// be exercised without a temporary copy.
var nathanSiteRoot = func() string {
	if info, err := os.Stat("./site"); err == nil && info.IsDir() {
		return "./site"
	}
	if info, err := os.Stat("../site"); err == nil && info.IsDir() {
		return "../site"
	}
	return "./site"
}()

// Imported frontend sources used by Shroombrew. Nathan's site provides the
// browser shell directly reused and adapted in site/, while the Internet Archive backup provides
// preserved Wii Shop shared assets used under site/oss/oss/common/.  The live
// catalog is still generated exclusively from Shroombrew's homebrew catalog.
const (
	nathanSiteSourceURL    = "https://wiishopchannel.net/"
	wiiShopBackupSourceURL = "https://archive.org/details/WiiShopChannelBackup"
)

var wiiModernDeclaration = regexp.MustCompile(`\b(?:const|let)\b`)
var shopInlineScripts = regexp.MustCompile(`(?is)<script\b[^>]*>.*?</script\s*>`)

func legacyWiiFrontend(text string) string {
	return wiiModernDeclaration.ReplaceAllString(text, "var")
}

// Nintendo retired the commerce hosts used by the imported Shop pages. Every
// one of those pages pushes the URLs below into the console's EC library
// through ec.setWebSvcUrls and ec.setContentUrls. A page that keeps them sends
// the console at a service that no longer answers, and the channel aborts with
// 209592 before Shroombrew's own endpoints are ever used. See ERRORS.md.
var nintendoShopHosts = []struct {
	host    string
	service string
}{
	{"ccs.cdn.shop.wii.com", "ccs"},
	{"oss-auth.shop.wii.com", "oss-auth"},
	{"cas.shop.wii.com", "oss-auth"},
	{"oss.shop.wii.com", "oss-auth"},
	{"ccs.shop.wii.com", "ccs"},
	{"ecs.shop.wii.com", "ecs"},
	{"ias.shop.wii.com", "ias"},
}

var nintendoShopHostSuffix = regexp.MustCompile(`([a-z0-9-]+)\.shop\.wii\.com`)

// shopBaseDomain returns the domain this deployment answers on. The configured
// BaseURL wins; otherwise the requested host is used with its service prefix
// removed, so a stale configuration still keeps the console on this server
// instead of falling back to a host that no longer exists.
func shopBaseDomain(requestHost string) string {
	if domain := strings.Trim(strings.ToLower(baseUrl), "."); domain != "" {
		return domain
	}
	host := strings.ToLower(strings.TrimSuffix(requestHost, "."))
	if i := strings.Index(host, ":"); i >= 0 {
		host = host[:i]
	}
	for _, entry := range nintendoShopHosts {
		if strings.HasPrefix(host, entry.service+".") {
			return strings.TrimPrefix(host, entry.service+".")
		}
	}
	return host
}

// shopServiceHosts points every Nintendo commerce host in a served page at this
// deployment. The patched Shop only enables EC on pages beneath the base domain
// its application was patched to expect, and only this server answers there.
func shopServiceHosts(text, domain string) string {
	if domain == "" {
		return text
	}
	for _, entry := range nintendoShopHosts {
		text = strings.ReplaceAll(text, entry.host, entry.service+"."+domain)
	}
	text = nintendoShopHostSuffix.ReplaceAllString(text, "${1}."+domain)
	text = strings.ReplaceAll(text, "shop.wii.com", domain)
	// The archived registration pages were captured from the reference site
	// and keep its a.taur.cloud URLs. Point them at this deployment and drop
	// the reference server's /oss prefix from the SOAP/content paths.
	text = strings.ReplaceAll(text, "a.taur.cloud", domain)
	text = strings.ReplaceAll(text, "ccs.blinklab.com", "ccs."+domain)
	for _, service := range []string{"ecs", "ias", "ccs", "cas", "nus"} {
		text = strings.ReplaceAll(text, "/oss/"+service+"/", "/"+service+"/")
	}
	return text
}

type classicTitle struct {
	ID          string `json:"id"`
	Title1      string `json:"title1"`
	Title2      string `json:"title2"`
	Console     string `json:"console"`
	Region      string `json:"region"`
	Language    string `json:"language"`
	Publisher   string `json:"publisher"`
	Genre       string `json:"genre"`
	Date        string `json:"date"`
	ReleaseUnix int64  `json:"releaseUnix"`
	Added       string `json:"added"`
	Points      string `json:"points"`
	Players     string `json:"players"`
	Attributes  string `json:"attributes"`
	Controllers string `json:"controllers"`
	// ControllerIcons and ControllerExtra feed the detail page's controller
	// images; DisplayGenre is the real genre (Genre stays the shop section
	// the catalog filters and groups by).
	ControllerIcons []string `json:"controllerIcons"`
	ControllerExtra string   `json:"controllerExtra,omitempty"`
	DisplayGenre    string   `json:"displayGenre,omitempty"`
	Rating          string   `json:"rating"`
	RatingDetails   string   `json:"ratingdetails"`
	Size            int64    `json:"size"`
	Link            string   `json:"link"`
	Thumbnail       string   `json:"thumbnail"`
	Description     string   `json:"description"`
	// IsNew puts the shop's golden NEW badge on the title's welcome tile.
	IsNew bool `json:"isNew,omitempty"`
	// Demo marks a WiiWare demo build. The real shop labelled it Demo and gave
	// it away free.
	Demo bool `json:"demo,omitempty"`
	// NotAvailable marks a catalog title whose WAD is no longer mirrored.
	NotAvailable bool `json:"notAvailable,omitempty"`
	// TitleVersion is the catalog's title version, used to detect updates.
	TitleVersion int `json:"titleVersion,omitempty"`
}

func classicTitleID(slug string) string {
	sum := sha256.Sum256([]byte("shroombrew:" + slug))
	return "00010001" + strings.ToUpper(hex.EncodeToString(sum[:4]))
}

func classicTitleIDForApp(app catalogApp) string {
	id := strings.ToUpper(app.Shop.TitleId)
	if len(id) == 16 {
		if _, err := hex.DecodeString(id); err == nil {
			return id
		}
	}
	return classicTitleID(app.Slug)
}

// Prices are stable for each title and use only play points, never money.
func classicPrice(slug string) int {
	sum := sha256.Sum256([]byte("shroombrew-price:" + slug))
	return (int(sum[0])%5 + 1) * 100
}

// classicPriceFor returns the authentic licensed price (0 for the free
// channels, as the real shop sold them), 0 for Flash titles (free), and the
// derived price otherwise.
// isDemoApp reports a licensed WiiWare demo build. The archived WAD names and
// the More Details pages both say "demo"; the primary variant is checked so a
// full release with a separate demo variant is not mislabelled.
func isDemoApp(app catalogApp) bool {
	if !app.Licensed {
		return false
	}
	isDemoURL := func(u string) bool {
		u = strings.ToLower(u)
		return strings.Contains(u, "(demo)") || strings.Contains(u, "%28demo%29")
	}
	if isDemoURL(app.URL.Zip) {
		return true
	}
	for _, v := range app.Variants {
		if v.TitleID == app.Shop.TitleId && isDemoURL(v.Zip) {
			return true
		}
	}
	// The archived More Details page is the strongest signal: the shop itself
	// printed "This is a demo version" for these releases.
	if strings.Contains(strings.ToLower(moreDetailsDesc[strings.ToUpper(app.Shop.TitleId)]), "demo version") {
		return true
	}
	return false
}

func classicPriceFor(app catalogApp) int {
	if isDemoApp(app) {
		return 0
	}
	if app.Price > 0 || app.Licensed {
		return app.Price
	}
	if app.Flash {
		return 0
	}
	return classicPrice(app.Slug)
}

// consoleLabel returns the platform a title is marked with in the shop. Real
// licensed titles are labelled by their actual Wii Shop platform (Virtual
// Console, WiiWare, DLC, Wii Channel) instead of the old "Homebrew" catch-all.
func consoleLabel(category string) string {
	switch category {
	case "Virtual Console":
		return "Virtual Console"
	case "WiiWare":
		return "WiiWare"
	case "DLC":
		return "DLC"
	case "Channels":
		return "Wii Channel"
	}
	return "Homebrew"
}

// controllerNames names each shop controller icon key (banner/B_08_<key>.gif).
var controllerNames = map[string]string{
	"RvlCtrl": "Wii Remote", "NunchukCtrl": "Nunchuk", "ClassicCtrl": "Classic Controller",
	"GcCtrl": "Nintendo GameCube Controller", "BalanceCtrl": "Wii Balance Board", "ZapperCtrl": "Wii Zapper",
	"WheelCtrl": "Wii Wheel", "MotionCtrl": "Wii MotionPlus", "KeyboardCtrl": "USB Keyboard",
	"SpeakCtrl": "Wii Speak", "MicrophoneCtrl": "Microphone", "DSCtrl": "Nintendo DS", "TWLCtrl": "Nintendo DSi",
}

// peripheralIcons maps Open Shop Channel peripheral names to the same icons.
var peripheralIcons = map[string]string{
	"Wii Remote": "RvlCtrl", "Nunchuk": "NunchukCtrl", "Classic Controller": "ClassicCtrl",
	"GameCube Controller": "GcCtrl", "USB Keyboard": "KeyboardCtrl", "Wii Zapper": "ZapperCtrl",
}

// controllerIcons returns the controller icon keys shown for a title: the
// real shop's list for licensed titles, mapped peripherals for homebrew.
func controllerIcons(app catalogApp) []string {
	icons := []string{}
	if len(app.Controllers) > 0 {
		for _, key := range app.Controllers {
			if _, ok := controllerNames[key]; ok && !containsStr(icons, key) {
				icons = append(icons, key)
			}
		}
		return icons
	}
	for _, p := range app.Peripherals {
		if key, ok := peripheralIcons[p]; ok && !containsStr(icons, key) {
			icons = append(icons, key)
		}
	}
	return icons
}

// controllerText is the controller sentence shown under the icons: the real
// shop's wording when known, else one built from the icons. Empty when the
// title lists no controllers.
func controllerText(app catalogApp) string {
	if app.ControllerNote != "" {
		return app.ControllerNote
	}
	names := []string{}
	for _, key := range controllerIcons(app) {
		if name, ok := controllerNames[key]; ok {
			names = append(names, name)
		}
	}
	switch len(names) {
	case 0:
		return ""
	case 1:
		return "This software can be used with the " + names[0] + "."
	}
	return "This software can be used with the " + strings.Join(names[:len(names)-1], ", ") + " or " + names[len(names)-1] + "."
}

// controllerExtra is the second note under the controllers (60Hz, Rumble
// Pak, ...); for homebrew it notes SDHC card support.
func controllerExtra(app catalogApp) string {
	if app.ControllerExtra != "" {
		return app.ControllerExtra
	}
	if containsStr(app.Peripherals, "SDHC") {
		return "Supports SDHC cards."
	}
	return ""
}

// regionFor returns the console's coarse region from the cookie set when the
// console registered (see deviceRegister). Unknown or missing falls back to US.
func regionFor(r *http.Request) string {
	if c, err := r.Cookie("shroombrew_region"); err == nil {
		switch strings.ToUpper(strings.TrimSpace(c.Value)) {
		case "US", "EU", "JP", "KR":
			return strings.ToUpper(strings.TrimSpace(c.Value))
		}
	}
	return "US"
}

func countryFor(r *http.Request) string {
	if c, err := r.Cookie("shroombrew_country"); err == nil {
		return strings.ToUpper(strings.TrimSpace(c.Value))
	}
	return ""
}

func containsStr(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

// markerCountries maps a catalog region marker to the ISO country codes whose
// consoles should prefer that region-locked release (a German console gets the
// Germany WAD, a US console the USA WAD, ...).
var markerCountries = map[string][]string{
	"germany": {"DE"}, "france": {"FR"}, "spain": {"ES"}, "italy": {"IT"},
	"netherlands": {"NL"}, "uk": {"GB", "UK"}, "australia": {"AU"},
	"usa": {"US"}, "canada": {"CA"}, "mexico": {"MX"},
	"japan": {"JP"}, "korea": {"KR"},
}

// regionApp returns the catalog entry with its Shop.TitleId, URL.Zip and Price
// swapped to the release matching the console's region, so the native download
// imports the correct region-locked WAD (keyed by that release's title ID).
// Titles with no region variants (homebrew, single-release) come back unchanged.
func (s *storefront) regionApp(app catalogApp, r *http.Request) catalogApp {
	if len(app.Variants) == 0 {
		return app
	}
	region := regionFor(r)
	country := countryFor(r)
	best := catalogVariant{}
	bestScore := 99
	for _, v := range app.Variants {
		if v.Zip == "" || v.TitleID == "" {
			continue
		}
		score := 99
		switch {
		case region == "EU" && country != "" && v.RegionGroup == "EU" && containsStr(markerCountries[v.Region], country):
			score = 1
		case v.RegionGroup == region:
			score = 2
		case v.RegionGroup == "US":
			score = 3
		}
		if score < bestScore {
			bestScore = score
			best = v
		}
	}
	if best.TitleID == "" {
		return app
	}
	out := app
	out.Shop.TitleId = best.TitleID
	out.URL.Zip = best.Zip
	out.Price = best.Price
	return out
}

// classicTitleFor is one catalog entry in the shape the shop pages' gameslist
// uses.
func classicTitleFor(app catalogApp) classicTitle {
	category := app.Category
	if category == "" {
		category = "Homebrew"
	}
	releaseDate := ""
	if app.ReleaseDate > 0 {
		releaseDate = time.Unix(app.ReleaseDate, 0).UTC().Format("Jan 2, 2006")
	}
	size := app.FileSize.ZipUncompressed
	if app.Size > 0 {
		size = app.Size
	}
	if size == 0 {
		size = app.Shop.ContentsSize
	}
	// The catalog carries the real Wii Shop platform/rating when known;
	// fall back to the broad shop section otherwise. Genre stays the
	// section (Virtual Console/WiiWare/...) so the catalog groups by the
	// real shop's sections, sorted alphabetically within each.
	console := consoleLabel(category)
	if app.Platform != "" {
		console = app.Platform
	}
	displayGenre := app.Genre
	if isDemoApp(app) {
		if strings.TrimSpace(displayGenre) == "" {
			displayGenre = "Demo"
		} else if !strings.Contains(strings.ToLower(displayGenre), "demo") {
			displayGenre += ", Demo"
		}
	}
	return classicTitle{
		ID: classicTitleIDForApp(app), Title1: app.Name, Title2: app.Subtitle, Console: console,
		Region: "US", Language: "EN", Publisher: app.Author, Genre: category,
		Date: releaseDate, ReleaseUnix: app.ReleaseDate, Added: releaseDate, Points: strconv.Itoa(classicPriceFor(app)), Players: app.Players, Attributes: "",
		Controllers: controllerText(app), ControllerIcons: controllerIcons(app), ControllerExtra: controllerExtra(app), DisplayGenre: displayGenre,
		Rating: app.Rating, RatingDetails: app.RatingDetails, Size: size,
		Link:      "/download/" + app.Slug,
		Thumbnail: "/icons/" + app.Slug + ".png", Description: app.Description.Short,
		IsNew:        app.Added > 0 && time.Since(time.Unix(app.Added, 0)) < 30*24*time.Hour,
		Demo:         isDemoApp(app),
		NotAvailable: app.Licensed && catalogDownloadURL(app) == "",
		TitleVersion: app.Shop.Version,
	}
}

// shopChannels is the Wii Channels section: the channels the Wii Shop
// actually offered (WiiMart's list). Other channel WADs in the catalog, such
// as system menus and disc-game channels, stay out of the shop.
var shopChannels = map[string]bool{
	"netflix-for-wii": true, "bbc-iplayer": true, "kirby-tv-channel": true,
	"legend-of-zelda-the-skyward-sword-save-d": true, "wii-u-transfer-tool": true,
	"youtube": true, "homebrew-channel-open-source-edition": true, "lovefilm": true,
	"photo-channel-v1-1": true, "zelda-save-data-update-channel-2": true,
	"internet-channel": true, "everybody-votes-channel": true, "mii-contest-channel": true,
	"nintendo-channel": true, "today-and-tomorrow-channel": true,
	"metroid-prime-3-preview-channel": true, "jam-with-the-band-live": true,
	"wii-speak-channel-2-0": true,
}

func (s *storefront) classicTitles() []classicTitle {
	titles := make([]classicTitle, 0, len(s.apps)+1)
	titles = append(titles, classicTitle{}) // The imported catalog UI begins at index one.
	for _, app := range s.apps {
		if app.Category == "Channels" && !shopChannels[app.Slug] {
			continue
		}
		titles = append(titles, classicTitleFor(app))
	}
	// The shop catalog is grouped by section (category) and alphabetical
	// within each section, matching the real Wii Shop ordering.
	sort.SliceStable(titles[1:], func(i, j int) bool {
		a, b := titles[i+1], titles[j+1]
		if a.Genre != b.Genre {
			return a.Genre < b.Genre
		}
		return strings.ToLower(a.Title1) < strings.ToLower(b.Title1)
	})
	return titles
}

func (s *storefront) classicGamesJS() []byte {
	b, _ := json.Marshal(s.classicTitles())
	return append([]byte("// Generated from Shroombrew's homebrew catalog.\nvar gameslist = "), append(b, ';')...)
}

func classicWelcomeFallbackJS() []byte {
	return []byte(`(function(){
var page=0;var fillRetries=0;var timer=null;var animating=false;
// The shop steps to the next list on its own every few seconds
// (SCROLL_COUNT 36 x 160ms); an arrow press starts the wait again.
var STEP=6000;
var lists=5;
// W_01's own slide tables: recroot sits at 18px, the outgoing list sweeps
// out in 16ms steps, the list is swapped while it is off screen, and the new
// list slides back in. The heading trails 11px behind recroot.
var OUT_LEFT=[18,17,13,-2,-37,-98,-185,-288,-393,-482,-541,-873,-873];
var IN_RIGHT=[221,135,74,38,23,19,18];
var OUT_RIGHT=[18,19,23,38,74,135,221,325,430,519,578,604,612];
var IN_LEFT=[-185,-98,-37,-2,13,17,18];
function isNew(id){if(typeof homeNew=='undefined')return false;for(var i=0;i<homeNew.length;i++)if(homeNew[i]==id)return true;return false;}
function display(id,on){var n=document.getElementById(id);if(n)n.style.display=on?'':'none';}
function setPos(left){
 var root=document.getElementById('recroot');
 if(root){root.style.left=left+'px';root.style.top='70px';}
 var heading=document.getElementById('txtSoftList');
 if(heading){heading.style.left=(left+11)+'px';heading.style.top='25px';}
 var spacer=document.getElementById('SoftListSpacer');
 if(spacer){spacer.style.left=(left+11)+'px';spacer.style.top='25px';}
}
function ensurePageChangeAudio(){
 // Native Shop clients have no HTML audio; they use their own sound engine.
 if(typeof shroomNativeSound!='undefined'&&shroomNativeSound)return null;
 // The browser shell is the untouched reference index.html, so the sample
 // element is created on first use and registered with the shell's sound list
 // so the sound slider controls it like every other effect.
 var doc=null;try{doc=parent.document;}catch(e){}
 if(!doc)return null;
 var audio=null;
 try{audio=doc.getElementById('cSE_PageChange');}catch(e){}
 if(audio)return audio;
 try{
  audio=doc.createElement('audio');
  audio.id='cSE_PageChange';
  audio.preload='auto';
  audio.src='/snd/page_change.mp3';
  (doc.body||doc.documentElement).appendChild(audio);
  var view=doc.defaultView;
  if(view&&view.soundlist&&view.soundlist.push)view.soundlist.push(audio);
 }catch(e){audio=null;}
 return audio;
}
function pageChangeSound(){
 var audio=ensurePageChangeAudio();
 if(audio){
  try{var view=parent;if(view&&typeof view.soundvol=='number')audio.volume=view.soundvol/100;}catch(e){}
  try{audio.currentTime=0;var p=audio.play();if(p&&p['catch'])p['catch'](function(){});return;}catch(e){}
 }
 // Native Shop clients have no HTML audio; use the shop's decide tone.
 try{if(typeof snd!='undefined')snd.playSE(cSE_Decide);}catch(e){}
}
function tile(title,position){
 if(!title){display('rec0'+position,false);display('new0'+position,false);return;}
 var imageBox=document.getElementById('soft0'+position);
 if(imageBox&&title.thumbnail){
  imageBox.innerHTML='';
  var image=document.createElement('img'); image.src=title.thumbnail;
  image.width=72; image.height=54; image.border=0; image.hspace=9; image.vspace=10;
  imageBox.appendChild(image);
 }
 var detail=document.getElementById('details0'+position);
 if(detail){
  detail.innerHTML='';
  // Same markup as the shop's own showRecommendedTitle: name, second line,
  // then the price, all in style13.
  var text=document.createElement('span'); text.className='style13';
  text.appendChild(document.createTextNode(title.title1||''));
  text.appendChild(document.createElement('br'));
  text.appendChild(document.createTextNode(title.title2||''));
  text.appendChild(document.createElement('br'));
  text.appendChild(document.createTextNode(title.points+' Wii Points'));
  detail.appendChild(text);
 }
 var link=document.getElementById('titleUrl0'+position);
 if(link&&title.id){link.href="javascript:showPage('B_05.html?p=4&titleId="+title.id+"')";}
 display('rec0'+position,true);
 // The golden NEW badge, as the shop's showRecommendedTitle draws it.
 display('new0'+position,isNew(title.id));
}
function fill(){
 if(typeof games=='undefined'||!games||!games.length){if(fillRetries++<20)setTimeout(fill,250);return;}
 for(var i=0;i<4;i++)tile(games[page*4+i],i+1);
 var heading=document.getElementById('txtSoftList');
 if(heading){
  if(page==0)heading.innerHTML='<span class="style1">Wii Shop Channel</span><span class="style2">Recommended Titles</span>';
  else{var label=(typeof homedata!='undefined'&&homedata[1]&&homedata[1][page-1])||'';heading.innerHTML='<nobr><span class="style1">'+label+'</span></nobr>';}
 }
 preload();
}
function preload(){
 // Cache the next list while this one is on screen so a slide never shows
 // an image that is still loading.
 if(typeof games=='undefined'||!games)return;
 var start=((page+1)%lists)*4;
 for(var i=0;i<4;i++){
  var title=games[start+i];
  if(title&&title.thumbnail){var image=new Image();image.src=title.thumbnail;}
 }
}
function go(delta){
 if(animating)return;
 if(typeof games=='undefined'||!games||!games.length){fill();return;}
 animating=true;
 if(timer){clearTimeout(timer);timer=null;}
 pageChangeSound();
 var outgoing=(delta>0)?OUT_LEFT:OUT_RIGHT;
 var incoming=(delta>0)?IN_RIGHT:IN_LEFT;
 var frame=0;
 function slideOut(){
  setPos(outgoing[frame]);frame++;
  if(frame<outgoing.length){setTimeout(slideOut,16);return;}
  page=(page+delta+lists)%lists;
  fill();
  frame=0;
  setTimeout(slideIn,16);
 }
 function slideIn(){
  setPos(incoming[frame]);frame++;
  if(frame<incoming.length){setTimeout(slideIn,16);return;}
  setPos(18);animating=false;schedule();
 }
 slideOut();
}
function schedule(){if(timer)clearTimeout(timer);timer=setTimeout(function(){go(1);},STEP);}
window.add_scroll_counter=function(){};
// start_animation_arrow is left to the page: the arrows bob as on the shop.
window.change_up_RecommendedTitles=function(){go(1);};
window.change_down_RecommendedTitles=function(){go(-1);};
function arrows(){
 // Keep the page's own arrow positions (top 118px, stopped frames at 108px);
 // only wire the clicks to the lists. go() plays the page-change sound and
 // restarts the six second wait, exactly like an arrow on the real shop.
 var ids=[['posLeftArrowID',-1],['posStopLeftArrowID',-1],['posRightArrowID',1],['posStopRightArrowID',1]];
 for(var i=0;i<ids.length;i++){
  var node=document.getElementById(ids[i][0]);if(!node)continue;
  node.style.cursor='pointer';node.onclick=(function(delta){return function(){go(delta);return false;};})(ids[i][1]);
 }
}
function directShopHome(){
 var area=document.getElementById('bannerspacer');
 if(!area)return;
 var links=area.getElementsByTagName('a');
 if(!links.length)return;
 links[0].href='/oss/serv/W_03.html';
 links[0].onclick=function(){
  try{if(typeof snd!='undefined')snd.playSE(cSE_Decide);}catch(e){}
  shroomNavigate('/oss/serv/W_03.html'); return false;
 };
}
// W_01 keeps the connecting screen's two rules (injected on the page).
if(window.setTimeout)setTimeout(function(){fill();arrows();ensurePageChangeAudio();schedule();},900);else{fill();arrows();ensurePageChangeAudio();}
if(window.setTimeout)setTimeout(directShopHome,100);else directShopHome();
})();`)
}

// The archived frontend includes the real per-title, per-language More Details
// pages (9,142 of them). Each page names the two CCS screenshot contents the
// shop showed for that title and language, so B_06 can use the same images
// instead of guessing from the folder layout.
//
//go:embed moredetails_shots.json
var moreDetailsShotsJSON []byte

var moreDetailsShots = func() map[string][]string {
	var m map[string][]string
	if err := json.Unmarshal(moreDetailsShotsJSON, &m); err != nil {
		log.Printf("more details screenshot map: %v", err)
		return map[string][]string{}
	}
	return m
}()

// The same archive keeps the original English long description for each
// title. B_06 shows that text where available, so the page reads like the
// shop's own copy instead of the catalogue's one-line summary.
//
//go:embed moredetails_desc.json
var moreDetailsDescJSON []byte

var moreDetailsDesc = func() map[string]string {
	var m map[string]string
	if err := json.Unmarshal(moreDetailsDescJSON, &m); err != nil {
		log.Printf("more details description map: %v", err)
		return map[string]string{}
	}
	return m
}()

// moreDetailsShotIDs returns the two CCS contents the archived page used for
// this title and language. Japanese and Korean use the archive's JP/KR keys;
// languages without their own page fall back to English.
func moreDetailsShotIDs(titleID, lang string) []string {
	code := lang
	switch code {
	case "ja":
		code = "jp"
	case "ko":
		code = "kr"
	case "zh-cn", "zh-tw", "pt":
		code = "en"
	}
	if shots := moreDetailsShots[titleID+"|"+code]; len(shots) >= 2 {
		return shots
	}
	if code != "en" {
		if shots := moreDetailsShots[titleID+"|en"]; len(shots) >= 2 {
			return shots
		}
	}
	return nil
}

// ccsFilesExist reports whether every named CCS content is present locally, so
// a page archived for a title whose assets were not mirrored falls back to the
// local folder instead of showing broken images.
func ccsFilesExist(titleID string, names []string) bool {
	for _, name := range names {
		if info, err := os.Stat(filepath.Join(nathanSiteRoot, "oss", "ccs", titleID, name)); err != nil || info.IsDir() {
			return false
		}
	}
	return true
}

// ccsShotCache remembers the two chosen screenshots per title.
var ccsShotCache = struct {
	sync.Mutex
	m map[string][]string
}{m: map[string][]string{}}

// ccsScreenshots picks the two capturable 160x120 images in a title's CCS
// folder. Each language has a block of [logo, screenshot, screenshot] (or
// [logo, logo, screenshot, screenshot]) 160x120 files; the last two of the
// first block are the screenshots, which is right for both the older Urbanix
// layout and the newer channel layout.
func ccsScreenshots(titleID string) []string {
	ccsShotCache.Lock()
	if shots, ok := ccsShotCache.m[titleID]; ok {
		ccsShotCache.Unlock()
		return shots
	}
	ccsShotCache.Unlock()
	dir := filepath.Join(nathanSiteRoot, "oss", "ccs", titleID)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var run []string
	pick := func() []string {
		if len(run) < 2 {
			return nil
		}
		return run[len(run)-2:]
	}
	var out []string
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasPrefix(entry.Name(), "FFFD") {
			continue
		}
		file, err := os.Open(filepath.Join(dir, entry.Name()))
		if err != nil {
			continue
		}
		cfg, _, err := image.DecodeConfig(file)
		file.Close()
		if err != nil || cfg.Width != 160 || cfg.Height != 120 {
			if picked := pick(); picked != nil {
				out = picked
				break
			}
			run = nil
			continue
		}
		run = append(run, "/oss/ccs/"+titleID+"/"+entry.Name())
	}
	if out == nil {
		out = pick()
	}
	ccsShotCache.Lock()
	ccsShotCache.m[titleID] = out
	ccsShotCache.Unlock()
	return out
}

// classicMoreDetailsPage fills B_06 with the selected title. The captured page
// ships with a demo game's paragraph and screenshots (Urbanix); the real More
// Details page shows the title the player came from, so the long description,
// title lines and the title's own CCS screenshots replace the placeholder.
func classicMoreDetailsPage(text string, app catalogApp, lang string) string {
	titleID := classicTitleIDForApp(app)
	desc := strings.TrimSpace(moreDetailsDesc[titleID])
	if desc == "" {
		desc = strings.TrimSpace(app.Description.Long)
		if desc == "" {
			desc = strings.TrimSpace(app.Description.Short)
		}
		desc = strings.ReplaceAll(html.EscapeString(desc), "\r\n", "\n")
		desc = strings.ReplaceAll(desc, "\n\n", "<p>")
		desc = strings.ReplaceAll(desc, "\n", "<br>")
	}
	if _, ok := findManual(titleID, lang); ok {
		desc += `<p><a href="/manual?titleId=` + titleID + `">Operations Guide</a></p>`
	}
	if start := strings.Index(text, `<div id="text" class="buttonTextBlackM">`); start >= 0 {
		if end := strings.Index(text[start:], `<div id="image02">`); end > 0 {
			text = text[:start] + `<div id="text" class="buttonTextBlackM">` + desc + "</div>\n  " + text[start+end:]
		}
	}
	console := consoleLabel(app.Category)
	if app.Platform != "" {
		console = app.Platform
	}
	text = strings.Replace(text, `id="Title1"></span>`, `id="Title1">`+html.EscapeString(app.Name)+`</span>`, 1)
	text = strings.Replace(text, `id="Title2"></span>`, `id="Title2">`+html.EscapeString(app.Subtitle)+`</span>`, 1)
	text = strings.Replace(text, `id="Console"></div>`, `id="Console">`+html.EscapeString(console)+`</div>`, 1)
	// The page's two screenshot slots point at the demo title; swap in the
	// exact contents the archived page used for this title and language, or the
	// local folder's best pair when that title was not archived.
	names := moreDetailsShotIDs(titleID, lang)
	var shots []string
	if len(names) >= 2 && ccsFilesExist(titleID, names) {
		shots = []string{"/oss/ccs/" + titleID + "/" + names[0], "/oss/ccs/" + titleID + "/" + names[1]}
	} else {
		shots = ccsScreenshots(titleID)
	}
	if len(shots) > 0 {
		text = strings.Replace(text, `src="/oss/ccs/0001000158494745/FFFD0005"`, `src="`+shots[0]+`"`, 1)
	} else {
		text = strings.Replace(text, `<div id="image02">`, `<div id="image02" style="display:none">`, 1)
	}
	if len(shots) > 1 {
		text = strings.Replace(text, `src="/oss/ccs/0001000158494745/FFFD0006"`, `src="`+shots[1]+`"`, 1)
	} else {
		text = strings.Replace(text, `<div id="image01">`, `<div id="image01" style="display:none">`, 1)
	}
	// The captured initPage is ES6 and cannot parse on the console; this one
	// only wires Back (the details are already in the markup).
	text = strings.Replace(text, "</body>", `<script type="text/javascript">function initPage(){if(typeof setUnderButtonL=='function')setUnderButtonL(true,'Back','javascript:showBack()','wiiCancelSound()');if(typeof setCursor=='function'){try{setCursor(document);}catch(e){}}}function kdown(id){var e=window.event,n=document.getElementById(id);if(!e||!n)return;if(e.keyCode==38)n.scrollTop-=30;else if(e.keyCode==40)n.scrollTop+=30;}</script></body>`, 1)
	return text
}

func classicDetailFallbackJS() []byte {
	return []byte(`(function(){
var retries=0;
function query(name){var parts=location.search.substring(1).split('&');for(var i=0;i<parts.length;i++){var p=parts[i].split('=');if(p[0]==name)return decodeURIComponent(p[1]||'');}return '';}
function put(id,value){var node=document.getElementById(id);if(node){node.innerHTML='';node.appendChild(document.createTextNode(value||''));}}
function makeDownloadButton(){
 var selected=query('titleId');if(!selected)return;
 if(typeof gameslist=='undefined'){if(retries++<20)setTimeout(makeDownloadButton,250);return;}
 var game=null;
 if(typeof gameslist!='undefined')for(var i=0;i<gameslist.length;i++){if(gameslist[i]&&gameslist[i].id==selected){game=gameslist[i];break;}}
 if(!game)return;
 var gver=game.titleVersion||0;
 put('Title1',game.title1);put('Title2',game.title2);put('DisplayCategory',game.displayGenre||game.genre);
 put('PublisherName',game.publisher);put('Description',(game.demo?'This is a demo version. ':'')+game.description);put('ReleaseDate',game.date?'Released '+game.date:'');
 put('Console',game.console||'Homebrew');put('NumPlayers',game.players||'');
 var thumb=document.getElementById('Thumbnail');if(thumb)thumb.src=game.thumbnail;
 // Parental Controls: hide the purchase/download button above the set level.
 var pcA=(function(){var m=/(?:^|;\s*)shroomPC=([a-z0-9]+)/.exec(document.cookie);var l=m?m[1]:'';return l=='e'?6:l=='e10'?10:l=='t'?13:l=='m'?18:0;})();
 var pcR=(function(r){r=String(r||'').toUpperCase();if(r=='E'||r=='EC')return 6;if(r=='E10')return 10;if(r=='T')return 13;if(r=='M')return 17;if(r=='AO')return 18;return 0;})(game.rating);
 if(pcA&&pcR&&pcR>pcA){
  var pcButton=document.getElementById('buyButton');if(pcButton)pcButton.style.display='none';
  var pcMsg=document.getElementById('pcMsg');
  if(!pcMsg){pcMsg=document.createElement('div');pcMsg.id='pcMsg';pcMsg.style.cssText='position:absolute;left:36px;top:230px;width:540px;height:200px;z-index:37;text-align:center;color:#323232;font-size:17px';document.body.appendChild(pcMsg);}
  pcMsg.innerHTML='Parental Controls prevent this software from being downloaded.<br><br>Change the setting in Settings and Features.';
  pcMsg.style.display='';
  return;
 }
 var unavailable=document.getElementById('notAvailableMsg');
 if(game.notAvailable){
  var hiddenButton=document.getElementById('buyButton');if(hiddenButton)hiddenButton.style.display='none';
  if(unavailable)unavailable.style.display='';
  return;
 }
 var isOnDevice=false;var isUpdate=false;
 try{ if(typeof titleManager!='undefined'){ isOnDevice=!!titleManager.getIsOnDevice(); isUpdate=!!titleManager.isUpdate(); } }catch(e){}
 try{ if(!isOnDevice){ var st=sessionStorage.getItem('shroomInstalled_'+selected); if(st){ isOnDevice=true; if(parseInt(st,10)!==gver) isUpdate=true; } } }catch(e){}
 var button=document.getElementById('buyButton');
 var downloaded=document.getElementById('downloadedMsg');
 var caption=document.getElementById('text03-01');
 var price=document.getElementById('price');
 var link=document.getElementById('BuyAnchor');
 if(isOnDevice && !isUpdate){
  if(button)button.style.display='none';
  if(downloaded)downloaded.style.display='';
  if(price)price.innerHTML='';
 } else {
  if(button)button.style.display='';
  if(downloaded)downloaded.style.display='none';
  if(caption)caption.innerHTML=isUpdate?'Update':(parseInt(game.points,10)===0?'Free':'Download');
  if(price)price.innerHTML=game.points+' Wii Points';
  if(link){ link.href='/wii-install/title/'+selected; link.target='_self'; link.onclick=function(){try{wiiSelectSound();}catch(e){} try{sessionStorage.setItem('shroomInstalled_'+selected,''+game.titleVersion);}catch(e){} shroomNavigate('/wii-install/title/'+selected);return false;}; }
 }
 if(isUpdate){ var ub=document.getElementById('updatedBadge'); if(!ub){ ub=document.createElement('div'); ub.id='updatedBadge'; ub.style.cssText='position:absolute;left:18px;top:9px;width:60px;height:18px;z-index:13;background:#c00;color:#fff;font-size:11px;font-weight:bold;text-align:center;line-height:18px;border-radius:2px;display:none;'; ub.innerHTML='Updated!'; var t=document.getElementById('Thumbnail'); if(t && t.parentNode) t.parentNode.insertBefore(ub,t); } if(ub)ub.style.display=''; }
 var control=document.getElementById('ControlAnchor');if(control){control.href='/controllers?titleId='+selected;control.onclick=function(){shroomNavigate(control.href);return false;};}
 if(typeof setUnderButtonR==='function') setUnderButtonR(true,'More Details','javascript:showPage(\'B_06.html?titleId='+selected+'\')','wiiSelectSound()');
 var buyImage=document.getElementById('Image2');if(buyImage)buyImage.onclick=function(){try{wiiSelectSound();}catch(e){}};
 var ratingIds=['Rating','ESRB_Rating','RatingPic'];
 var esrb={'E':1,'E10':1,'T':1,'M':1,'EC':1,'AO':1,'RP':1};
 if(game.rating&&esrb[game.rating]){
  for(var r=0;r<ratingIds.length;r++){var shown=document.getElementById(ratingIds[r]);if(shown)shown.style.display='';}
  var pic=document.getElementById('RatingPic');if(pic){pic.src='/oss/oss/common/images/Pic_Rating/ESRB/ESRB_'+game.rating+'.gif';pic.alt='ESRB '+game.rating;pic.style.cursor='pointer';pic.onclick=function(){try{wiiSelectSound();}catch(e){}showPage('B_07.html?titleId='+selected);};}
  put('Descriptors',game.ratingdetails||'');
 }else{
  for(var q=0;q<ratingIds.length;q++){var rating=document.getElementById(ratingIds[q]);if(rating)rating.style.display='none';}
 }
 var unavailable=document.getElementById('notAvailableMsg');if(unavailable)unavailable.style.display='none';
 // Gift this title to another Wii. The real shop keeps this as a separate
 // purchase path; our community layer owns the points and pending gift.
 var giftLink=document.getElementById('shroomGiftLink');
 if(!giftLink){giftLink=document.createElement('a');giftLink.id='shroomGiftLink';giftLink.style.cssText='position:absolute;left:438px;top:338px;width:110px;text-align:center;color:#34beed;font-size:15px;text-decoration:underline;z-index:40';document.body.appendChild(giftLink);}
 giftLink.href='/gift?titleId='+selected;
 giftLink.onclick=function(){try{wiiSelectSound();}catch(e){}shroomNavigate('/gift?titleId='+selected);return false;};
 giftLink.innerHTML='Gift';
 giftLink.style.display=game.notAvailable?'none':'';
}

window.initPage=function(){shroomInstallBridge();if(typeof setUnderButtonL==='function')setUnderButtonL(true,'Back','javascript:showBack()','wiiCancelSound()');makeDownloadButton();};

if(window.setTimeout){setTimeout(makeDownloadButton,700);if(window.addEventListener)window.addEventListener('load',function(){setTimeout(makeDownloadButton,100);},false);}else makeDownloadButton();
})();`)
}

// The imported website version of sound.js defines wiiSound with an ES6 class
// and tries to play HTML audio from its parent document. Wii Opera rejects
// that syntax and the Shop channel already supplies a native wiiSound object.
func classicSoundJS() []byte {
	return []byte(`var cSE_Slide=1,cSE_Forcus=2,cSE_Decide=3,cSE_Cancel=4,cSE_Choise=5,cSE_Error=6,cSE_AddPoint=7,cSE_Finish=7,cSE_PageChange=8;
var snd;var shroomNativeSound=navigator.userAgent.indexOf('Wii Shop Channel/')!=-1||navigator.userAgent.indexOf('Nintendo Wii')!=-1;
if(shroomNativeSound){try{if(typeof wiiSound!='undefined')snd=new wiiSound();}catch(e){}}
if(!snd)snd={playSE:function(code){try{var names={2:'cSE_Forcus',3:'cSE_Decide',4:'cSE_Cancel',7:'cSE_Finish',8:'cSE_PageChange'};var a=parent.document.getElementById(names[code]);if(a){a.currentTime=0;var p=a.play();if(p&&p['catch'])p['catch'](function(){});}}catch(e){}},playBGM:function(){}};
function wiiFocusSound(){snd.playSE(cSE_Forcus);}function wiiSelectSound(){snd.playSE(cSE_Decide);}function wiiCancelSound(){snd.playSE(cSE_Cancel);}function wiiFinishSound(){snd.playSE(cSE_Finish);}var soundCheck=true;`)
}

func classicNavigationJS() []byte {
	return []byte(`shroomInstallBridge();`)
}

// B_04.html in the imported frontend uses ES6 template literals, arrow
// functions, URLSearchParams, and Array.includes. Wii Opera parses none of
// those. This ES3 renderer keeps the archived page shell and paints the
// Shroombrew catalog after the incompatible inline script fails.
func classicCatalogFallbackJS() []byte {
	return []byte(`(function(){
var retries=0;
function esc(s){return String(s||'').replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/>/g,'&gt;').replace(/"/g,'&quot;');}
function value(name){var q=location.search.substring(1).split('&');for(var i=0;i<q.length;i++){var p=q[i].split('=');if(decodeURIComponent(p[0])==name)return decodeURIComponent((p[1]||'').replace(/\+/g,' '));}return '';}
// Parental Controls: the level is a cookie set in Settings and Features. The
// same values drive ec.getDeviceInfo in the mock above, so the archived page
// code agrees with the catalogue filter. A title rated above the level is not
// shown, as on the real shop ("you may not be able to view all software").
function pcLevel(){var m=/(?:^|;\s*)shroomPC=([a-z0-9]+)/.exec(document.cookie);return m?m[1]:'';}
function pcAge(){var l=pcLevel();return l=='e'?6:l=='e10'?10:l=='t'?13:l=='m'?18:0;}
function ratingAge(r){r=String(r||'').toUpperCase();if(r=='E'||r=='EC')return 6;if(r=='E10')return 10;if(r=='T')return 13;if(r=='M')return 17;if(r=='AO')return 18;return 0;}
function pcAllowed(g){var a=pcAge();if(!a)return true;var r=ratingAge(g.rating);return !r||r<=a;}
// The main menu's tiles link with the archived parameters (vc=true,
// platform=WiiWare, platform=Wii Channels in each language); map them to sections.
function section(){var c=value('category');if(c)return c;if(value('vc')=='true')return 'Virtual Console';var p=value('platform').toLowerCase();if(p.indexOf('wiiware')>=0)return 'WiiWare';if(p.indexOf('virtual')>=0)return 'Virtual Console';if(/channel|kanal|cha.ne|canal/.test(p))return 'Channels';return '';}
function pageURL(number){var old=location.search.substring(1).split('&');var kept=[];for(var j=0;j<old.length;j++){if(old[j]&&old[j].split('=')[0]!='p')kept.push(old[j]);}kept.push('p='+number);return '/oss/serv/B_04.html?'+kept.join('&');}
// The list's heading, as on the original page: one plain title, no links.
function nav(){
 var info=document.getElementById('listinfo');if(!info)return;
 var cur=section(),hb=value('hb')=='1',t;
 if(value('title'))t='Results for '+value('title');
 else if(hb&&value('genre'))t=value('genre').charAt(0).toUpperCase()+value('genre').substring(1);
 else if(hb&&value('popular')=='true')t='Popular Homebrew';
 else if(hb&&value('order')=='new')t='Newest Homebrew';
 else if(hb)t=cur=='Virtual Console'?'Homebrew Emulators':'Homebrew';
 else if(value('genre'))t=value('genre')=='flash'?'Flash':value('genre');
 else if(/channel|kanal|cha.ne|canal/i.test(value('platform')))t='Wii Channels';
 else if(value('order')=='new')t='Newest Additions';
 else if(value('popular')=='true')t='Popular Titles';
 else if(value('publisher'))t=value('publisher');
 else t={'Virtual Console':'Virtual Console','WiiWare':'WiiWare','Channels':'Wii Channels','DLC':'DLC'}[cur]||'Catalog';
 info.innerHTML=esc(t);
}
function render(){
 var frame=document.getElementById('catalogFrame');if(!frame||typeof gameslist=='undefined'){if(retries++<20)setTimeout(render,250);return;}
 nav();
 var page=parseInt(value('p')||'1',10);if(!page||page<1)page=1;
 var title=value('title').toLowerCase();var genre=value('genre').toLowerCase();var cat=section().toLowerCase();var publisher=value('publisher').toLowerCase();var list=[];var i;
 // A section lists only its own titles: the MarioCube ones in Virtual
 // Console/WiiWare/DLC/Wii Channels. Homebrew shows only behind the
 // Homebrew tile of the VC (emulators) and WiiWare (games) choosers (hb=1).
 function inSection(g){
  if(value('hb')=='1')return cat=='virtual console'?g=='emulators':(g=='games'||g=='utilities'||g=='media'||g=='demos');
  if(cat)return g==cat;
  return true;
 }
 function matchGenre(game){
  var g=String(game.genre||'').toLowerCase();
  if(genre=='flash')return g=='flash';
  if(!inSection(g))return false;
  if(!genre)return true;
  if(genre=='utilities-or-so')return g=='utilities'||g=='media'||g=='demos';
  if(g==genre)return true;
  // Licensed titles keep their real genre(s) in displayGenre ("Puzzle, Adventure").
  var d=String(game.displayGenre||'').toLowerCase().split(/\s*,\s*/);
  for(var k=0;k<d.length;k++)if(d[k]==genre)return true;
  return false;
 }
 for(i=1;i<gameslist.length;i++){
  var game=gameslist[i];if(!game||!game.id)continue;
  if(title&&((game.title1||'')+' '+(game.title2||'')+' '+(game.publisher||'')).toLowerCase().indexOf(title)<0)continue;
  if(!matchGenre(game))continue;
  if(publisher&&String(game.publisher||'').toLowerCase().indexOf(publisher)<0)continue;
  if(!pcAllowed(game))continue;
  list.push(game);
 }
 // A section lists its own licensed titles (the real Wii channels, the real
 // Virtual Console games) before the homebrew that shares it.
 function rank(g){var x=String(g.genre||'').toLowerCase();return x==cat?0:1;}
 function byTitle(a,b){return String(a.title1||'').toLowerCase()<String(b.title1||'').toLowerCase()?-1:1;}
 // Popular Titles: most downloads first (counts from /shroombrew-popular.js).
 var pop=window.shroomPopular||{};
 function downloads(g){var l=String(g.link||'');return l.indexOf('/download/')==0?(pop[l.substring(10)]||0):0;}
 if(value('popular')=='true')list.sort(function(a,b){return (downloads(b)-downloads(a))||byTitle(a,b);});
 else if(value('order')=='new')list.sort(function(a,b){return (rank(a)-rank(b))||((b.releaseUnix||0)-(a.releaseUnix||0));});
 else if(genre||title)list.sort(byTitle);
 else list.sort(function(a,b){var r=rank(a)-rank(b);if(r)return r;var g=String(a.genre||'').toLowerCase()<String(b.genre||'').toLowerCase()?-1:1;return g?g:(String(a.title1||'').toLowerCase()<String(b.title1||'').toLowerCase()?-1:1);});
 var pages=Math.max(1,Math.ceil(list.length/3));if(page>pages)page=pages;
 var first=(page-1)*3;var html='';
 for(i=0;i<3&&first+i<list.length;i++){
  var g=list[first+i];var top=i*92;var icon=g.thumbnail||('/oss/ccs/'+g.id+'/FFFD0001');
  html+='<div style="position:absolute;left:0px;top:'+top+'px;width:488px;height:91px;z-index:1">';
  html+='<div class="contentsBlack" id="Platform">'+esc(g.console||'Homebrew')+'</div><div class="contentsBlack" id="Publisher">'+esc(g.publisher||'')+'</div>';
  html+='<div style="overflow:hidden" nowrap id="TitleName1stline"><span class="catalogTitleBlack_01">'+esc(g.title1)+'</span></div>';
  html+='<div style="overflow:hidden" nowrap id="TitleName2stline"><span class="catalogTitleBlack_01">'+esc(g.title2)+'</span></div>';
  html+='<div id="ImageforCatalogpage"><img src="'+esc(icon)+'" width="96" height="72" style="background-color:white" /></div>';
  if(g.isNew)html+='<div class="shroom-new" data-id="'+esc(g.id)+'" style="position:absolute;left:18px;top:9px;width:34px;height:14px;z-index:12"><img src="/oss/oss/common/images/banner/NEW_en.gif" width="34" height="14" border="0" /></div>';
  // "Updated!" tab above the tile: the shop's own new_update.png outline with
  // the blue word on top. Shown when an older version is installed.
  if(g.titleVersion)html+='<div class="shroom-updated" data-id="'+esc(g.id)+'" data-ver="'+g.titleVersion+'" style="display:none;position:absolute;left:18px;top:-10px;width:150px;height:23px;z-index:14"><img src="/oss/oss/common/images/banner/new_update.png" width="150" height="23" style="position:absolute;left:0;top:0;border:0" /><span style="position:absolute;left:0;top:5px;width:150px;text-align:center;color:#34beed;font-size:11px;font-weight:bold">Updated!</span></div>';
  html+='<div id="regularTitle"><div id="spacer"><a onclick="if(new Date().getTime()<shroomListReady)return false;shroomNavigate(this.href);return false" href="/oss/serv/B_05.html?titleId='+encodeURIComponent(g.id)+'"><img src="/oss/oss/common/images/spacer.gif" width="488" height="91" border="0" /></a></div><div id="points"><div align="right" class="catalogTitleBlackBold">'+(g.notAvailable?'Not Available':(g.demo?'Free':(parseInt(g.points,10)===0?'Downloadable':esc(g.points)+' Wii Points')))+'</div></div></div>';
  html+='<img src="/oss/oss/common/images/banner/B_04_softbanner_a.gif" width="488" height="91" border="0" /></div>';
 }
 // A press that opened this list must not also open the row now under the pointer.
 window.shroomListReady=new Date().getTime()+700;
 frame.innerHTML=html;frame.style.display='';frame.style.overflow='visible';frame.style.left='60px';frame.style.width='488px';frame.style.background='white';frame.style.border='none';
  try{var badges=frame.querySelectorAll('.shroom-updated');for(var bi=0;bi<badges.length;bi++){var b=badges[bi];var id=b.getAttribute('data-id');var ver=parseInt(b.getAttribute('data-ver'),10)||0;var st=sessionStorage.getItem('shroomInstalled_'+id);if(st!==null){var have=parseInt(st,10)||0;if(ver>have && ver>0){b.style.display='block';var nu=frame.querySelector('.shroom-new[data-id="'+id+'"]');if(nu)nu.style.display='none';}}}}catch(e){}
 var count=document.getElementById('pageNum');if(count)count.innerHTML=page+'/'+pages;
 var left=document.getElementById('backArrow');if(left){left.href=pageURL(page>1?page-1:1);left.onclick=function(){shroomNavigate(left.href);return false;};}
 var right=document.getElementById('nextArrow');if(right){right.href=pageURL(page<pages?page+1:pages);right.onclick=function(){shroomNavigate(right.href);return false;};}
 var lbox=document.getElementById('arrowL');var rbox=document.getElementById('arrowR');
 if(lbox)lbox.style.display=page>1?'':'none';if(rbox)rbox.style.display=page<pages?'':'none';
 var ls=document.getElementById('arrowLshadow');var rs=document.getElementById('arrowRshadow');
 if(ls)ls.style.display=page>1?'':'none';if(rs)rs.style.display=page<pages?'':'none';
 var li=document.getElementById('arrowLImg');if(li)li.onclick=null;
 var ri=document.getElementById('arrowRImg');if(ri)ri.onclick=null;
 var news=document.getElementById('shroomWhatsNew');if(news)news.style.display='none';
 var empty=document.getElementById('notFound');if(empty)empty.style.display=list.length?'none':'';
}
if(window.setTimeout){setTimeout(render,250);if(window.addEventListener)window.addEventListener('load',function(){setTimeout(render,100);},false);}else render();
})();`)
}

// serveShopIcon serves a title's icon: a homebrew PNG from ./icons/, the best
// archived image from site/oss/ccs/<titleId>/ (licenses keep the icon at a
// numbered file, not always FFFD0000), a regional variant's or sibling
// release's artwork, or finally a generated tile so the thumbnail is never
// broken.
func (s *storefront) serveShopIcon(w http.ResponseWriter, r *http.Request, slug string) bool {
	app, ok := s.bySlug[slug]
	if !ok || !safeSlug.MatchString(slug) {
		return false
	}
	if ic, ok := s.shopIconFor(app); ok {
		w.Header().Set("Content-Type", ic.mime)
		w.Header().Set("Cache-Control", "public, max-age=3600")
		http.ServeFile(w, r, ic.path)
		return true
	}
	data := placeholderIcon(app)
	if len(data) == 0 {
		return false
	}
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))
	w.Write(data)
	return true
}

// downloadTarget returns the redirect URL for a title's WAD/package download.
// Licensed games are fetched directly from their external host (mariocube);
// homebrew uses the Open Shop Channel package mirror.
// catalogDownloadURL returns the WAD URL for a licensed title. Some catalogue
// rows keep the URL only on the variant matching the primary title ID.
func catalogDownloadURL(app catalogApp) string {
	if strings.TrimSpace(app.URL.Zip) != "" {
		return app.URL.Zip
	}
	for _, v := range app.Variants {
		if v.TitleID == app.Shop.TitleId && strings.TrimSpace(v.Zip) != "" {
			return v.Zip
		}
	}
	return ""
}

func (s *storefront) downloadTarget(app catalogApp) (string, bool) {
	if app.Licensed {
		u := catalogDownloadURL(app)
		if u == "" {
			return "", false
		}
		return u, true
	}
	u, err := url.Parse(app.URL.Zip)
	if err != nil || u.Scheme != "https" || u.Host != "hbb1.oscwii.org" || u.User != nil || !strings.HasPrefix(u.Path, "/api/contents/") {
		return "", false
	}
	return u.String(), true
}

func (s *storefront) classicDownload(w http.ResponseWriter, r *http.Request) {
	slug := strings.TrimPrefix(r.URL.Path, "/download/")
	app, ok := s.bySlug[slug]
	if !ok {
		http.NotFound(w, r)
		return
	}
	target, ok := s.downloadTarget(s.regionApp(app, r))
	if !ok {
		http.Error(w, "Download unavailable", http.StatusServiceUnavailable)
		return
	}
	logDownload(r.Context(), app.Slug, "download")
	if id, err := walletID(w, r); err == nil {
		awardPlayPoints(r.Context(), id, app.Slug)
	}
	http.Redirect(w, r, target, http.StatusFound)
}

func (s *storefront) classicSiteHandler(soap http.Handler) http.Handler {
	shopAssets, _ := fs.Sub(shopFiles, "shop")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isStaticShopAsset(r.URL.Path) {
			// Let the console revalidate archived scripts and images instead of
			// downloading them again on every page.
			w.Header().Set("Cache-Control", "no-cache")
		} else {
			w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate")
			w.Header().Set("Pragma", "no-cache")
		}
		if r.Header.Get("SOAPAction") != "" {
			soap.ServeHTTP(w, r)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/shop/") {
			http.StripPrefix("/shop/", http.FileServer(http.FS(shopAssets))).ServeHTTP(w, r)
			return
		}
		if r.URL.Path == "/healthz" {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.Write([]byte("ok\n"))
			return
		}
		if r.URL.Path == "/api/catalog" || r.URL.Path == "/api/v1/catalog" {
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			if r.Method != http.MethodHead {
				json.NewEncoder(w).Encode(s.apps)
			}
			return
		}
		if r.URL.Path == "/points" {
			s.freePointsPage(w, r)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/points/") {
			s.pointsFlow(w, r)
			return
		}
		if r.URL.Path == "/shroombrew-balance.js" {
			s.balanceJS(w, r)
			return
		}
		if r.URL.Path == "/help" || r.URL.Path == "/activity" || r.URL.Path == "/downloaded" || r.URL.Path == "/settings" || r.URL.Path == "/controllers" || r.URL.Path == "/news" || r.URL.Path == "/oss/serv/W_02.html" {
			s.simpleInfo(w, r)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/api/install/") {
			s.installs.serveHTTP(w, r, s.bySlug)
			return
		}
		if r.URL.Path == "/wii-install/start" {
			s.startWiiInstall(w, r)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/wii-install/title/") {
			s.startWiiInstallByTitle(w, r)
			return
		}
		if r.URL.Path == "/tools/wii-installer.zip" {
			w.Header().Set("Content-Type", "application/zip")
			w.Header().Set("Content-Disposition", `attachment; filename="Shroombrew-Installer.zip"`)
			http.ServeFile(w, r, "./downloads/Shroombrew-Installer.zip")
			return
		}
		if r.URL.Path == "/source/shroombrew-source.tar.gz" {
			w.Header().Set("Content-Type", "application/gzip")
			http.ServeFile(w, r, "./shroombrew-source.tar.gz")
			return
		}
		if r.URL.Path == "/tools/install-sd-windows.exe" {
			w.Header().Set("Content-Type", "application/octet-stream")
			http.ServeFile(w, r, "./downloads/Shroombrew-install-sd.exe")
			return
		}
		registrationPost := r.Method == http.MethodPost && func() bool {
			p := strings.ToLower(path.Clean(r.URL.Path))
			switch p {
			case "/oss/serv/register.jsp", "/oss/serv/register.html",
				"/oss/serv/checkbalance.jsp", "/oss/serv/checkbalance.html":
				return true
			}
			return false
		}()
		if r.Method != http.MethodGet && r.Method != http.MethodHead && !registrationPost {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		isWiiShop := strings.Contains(r.UserAgent(), "Wii Shop Channel/")
		if isWiiShop {
			canonical := map[string]string{
				"/startup":    "/oss/serv/W_01.html",
				"/main":       "/oss/serv/W_03.html",
				"/browse":     "/oss/serv/B_23.html",
				"/catalog":    "/oss/serv/B_04.html",
				"/search":     "/oss/serv/B_04.html",
				"/categories": "/oss/serv/B_16.html",
			}
			if destination, ok := canonical[r.URL.Path]; ok {
				if r.URL.RawQuery != "" {
					destination += "?" + r.URL.RawQuery
				}
				http.Redirect(w, r, destination, http.StatusFound)
				return
			}
		}
		switch r.URL.Path {
		case "/download/":
			http.NotFound(w, r)
			return
		case "/oss/serv/games.js":
			w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
			w.Write(s.classicGamesJS())
			return
		case "/games.js":
			w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
			w.Write(s.classicFeaturedGamesJS(r))
			return
		case "/oss/serv/home.js", "/home.js":
			w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
			w.Write(s.classicHomeJS(r))
			return
		case "/oss/common/js/sound.js", "/oss/oss/common/js/sound.js":
			w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
			w.Write(classicSoundJS())
			return
		case "/shroombrew-welcome.js":
			w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
			w.Write(classicWelcomeFallbackJS())
			return
		case "/shroombrew-detail.js":
			w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
			w.Write(classicDetailFallbackJS())
			return
		case "/shroombrew-navigation.js":
			w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
			w.Write(classicNavigationJS())
			return
		case "/shroombrew-popular.js":
			popularScript(w, r)
			return
		case "/shroombrew-catalog.js":
			w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
			w.Write(classicCatalogFallbackJS())
			return
		case "/shroombrew-browse.js":
			w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
			w.Write(classicBrowseJS())
			return
		}
		if strings.HasPrefix(r.URL.Path, "/download/") {
			s.classicDownload(w, r)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/icons/") {
			slug := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/icons/"), ".png")
			if !s.serveShopIcon(w, r, slug) {
				http.NotFound(w, r)
			}
			return
		}
		if strings.HasPrefix(r.URL.Path, "/oss/ccs/") {
			parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/oss/ccs/"), "/")
			if len(parts) == 2 {
				for _, app := range s.apps {
					if classicTitleIDForApp(app) == strings.ToUpper(parts[0]) {
						if s.serveShopIcon(w, r, app.Slug) {
							return
						}
					}
				}
			}
			http.NotFound(w, r)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/title/") {
			slug := strings.TrimPrefix(r.URL.Path, "/title/")
			app, ok := s.bySlug[slug]
			if !ok {
				http.NotFound(w, r)
				return
			}
			q := url.Values{"titleId": {classicTitleIDForApp(app)}}
			http.Redirect(w, r, "/oss/serv/B_05.html?"+q.Encode(), http.StatusFound)
			return
		}
		filePath := path.Clean(r.URL.Path)
		switch filePath {
		case "/":
			if isWiiShop {
				filePath = "/oss/serv/W_01.html"
			} else {
				filePath = "/index.html"
			}
		case "/startup":
			filePath = "/oss/serv/W_01.html"
		case "/main":
			filePath = "/oss/serv/W_03.html"
		case "/browse":
			filePath = "/oss/serv/B_23.html"
		case "/catalog", "/search":
			filePath = "/oss/serv/B_04.html"
		case "/categories":
			filePath = "/oss/serv/B_16.html"
		case "/settings":
			filePath = "/oss/serv/S_01.html"
		case "/news":
			filePath = "/oss/serv/W_02.html"
		case "/help", "/points", "/activity":
			filePath = "/oss/serv/W_03.html"
		case "/oss/serv":
			filePath = "/oss/serv/index.html"
		}
		// Keep the catalog entry point addressable directly by the Wii Shop
		// client. Query parameters are intentionally left on r.URL unchanged
		// so B_04.html can apply p/order/title filters as before.
		if r.URL.Path == "/oss/serv/B_04.html" {
			filePath = "/oss/serv/B_04.html"
		}
		if r.URL.Path == "/oss/serv/" {
			filePath = "/oss/serv/index.html"
		}
		if (r.URL.Path == "/catalog" || r.URL.Path == "/search") && r.URL.Query().Get("q") != "" {
			q := r.URL.Query()
			q.Set("title", q.Get("q"))
			r.URL.RawQuery = q.Encode()
		}
		if strings.HasSuffix(filePath, "/") || strings.Contains(filePath, "..") {
			http.NotFound(w, r)
			return
		}
		if strings.HasPrefix(filePath, "/oss/serv/") && strings.HasSuffix(filePath, ".jsp") {
			filePath = strings.TrimSuffix(filePath, ".jsp") + ".html"
		}
		if strings.HasPrefix(filePath, "/oss/serv/") && filepath.Ext(filePath) == "" {
			candidate := filepath.Join(nathanSiteRoot, filepath.FromSlash(strings.TrimPrefix(filePath+".html", "/")))
			if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
				filePath += ".html"
			}
		}
		if strings.HasPrefix(filePath, "/oss/ext/") {
			candidate := filepath.Join(nathanSiteRoot, "oss", "oss", "ext", filepath.FromSlash(strings.TrimPrefix(filePath, "/oss/ext/")))
			if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
				filePath = "/oss/oss/ext/" + strings.TrimPrefix(filePath, "/oss/ext/")
			}
		}
		if strings.HasPrefix(filePath, "/oss/common/") {
			candidate := filepath.Join(nathanSiteRoot, "oss", "oss", "common", filepath.FromSlash(strings.TrimPrefix(filePath, "/oss/common/")))
			if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
				filePath = "/oss/oss/common/" + strings.TrimPrefix(filePath, "/oss/common/")
			}
		}
		if strings.HasPrefix(filePath, "/img/") {
			aliasPath := filepath.Join(nathanSiteRoot, filepath.FromSlash(strings.TrimPrefix(filePath, "/img/")))
			if info, err := os.Stat(aliasPath); err == nil && !info.IsDir() {
				w.Header().Set("Cache-Control", "public, max-age=3600")
				w.Header().Del("Pragma")
				w.Header().Set("X-Content-Type-Options", "nosniff")
				http.ServeFile(w, r, aliasPath)
				return
			}
		}
		// The community site is now one page; old per-page URLs redirect to it.
		if strings.HasPrefix(filePath, "/community/") && strings.HasSuffix(filePath, ".html") && path.Base(filePath) != "index.html" {
			name := strings.TrimSuffix(path.Base(filePath), ".html")
			http.Redirect(w, r, "/community/#"+name, http.StatusMovedPermanently)
			return
		}
		localPath := filepath.Join(nathanSiteRoot, filepath.FromSlash(strings.TrimPrefix(filePath, "/")))
		if st, statErr := os.Stat(localPath); (statErr != nil || st.IsDir()) && strings.HasSuffix(filePath, ".html") {
			// The archive stores pages as B_04, B_05, ...; a request for
			// B_04.html should find the same page even when no .html copy exists.
			alt := filepath.Join(nathanSiteRoot, filepath.FromSlash(strings.TrimSuffix(strings.TrimPrefix(filePath, "/"), ".html")))
			if ai, aerr := os.Stat(alt); aerr == nil && !ai.IsDir() {
				localPath = alt
			}
		}
		info, err := os.Stat(localPath)
		if err != nil || info.IsDir() {
			http.NotFound(w, r)
			return
		}
		if strings.HasSuffix(filePath, ".html") || strings.HasSuffix(filePath, ".js") {
			body, err := os.ReadFile(localPath)
			if err != nil {
				http.NotFound(w, r)
				return
			}
			text := strings.ReplaceAll(string(body), "https://wiishopchannel.net", "")
			text = strings.ReplaceAll(text, "https://wiishopchannel.neocities.org", "")
			text = strings.ReplaceAll(text, "http://wiishopchannel.net", "")
			text = strings.ReplaceAll(text, "http://wiishopchannel.neocities.org", "")
			text = shopServiceHosts(text, shopBaseDomain(r.Host))
			// The archived registration pages are JSP: the original server filled
			// their hidden "next" field into the page. Preserve that behaviour so
			// CheckBalance returns to W_01 instead of stalling.
			if r.Method == http.MethodPost && (filePath == "/oss/serv/CheckBalance.html" || filePath == "/oss/serv/Register.html") {
				if r.ParseForm() == nil {
					if next := r.Form.Get("next"); next != "" {
						if quoted, err := json.Marshal(next); err == nil {
							text = strings.Replace(text, "decodeURIComponent('')", string(quoted), 1)
						}
					}
				}
			}
			text = strings.ReplaceAll(text, "top_L_arrow_S.png", "top_L_arrow_00.png")
			text = strings.ReplaceAll(text, "top_R_arrow_S.png", "top_R_arrow_00.png")
			if strings.HasSuffix(filePath, ".html") {
				text = strings.Replace(text, "<head>", "<head><script src=\""+compatURL+"\"></script>", 1)
			}

			if strings.HasPrefix(filePath, "/oss/serv/") && strings.HasSuffix(filePath, ".html") {
				// Mocks for Wii-only objects so the archived page scripts run in a
				// browser: ec, wiiShop, catalogue lists and the shop's region
				// setting (pages read parent.region / parent.showallgames from the
				// shell that hosts them in an iframe). A real console must keep its
				// native ec/wiiShop objects or ECS purchases and downloads break.
				if !isWiiClient(r) {
					mockScript := `<script>try{var ec={SupportsSession:function(){return false},getSessionValue:function(){return ''},getDeviceInfo:function(){var m=/(?:^|;\s*)shroomPC=([a-z0-9]+)/.exec(document.cookie);var lvl=m?m[1]:'';var age=lvl=='e'?6:lvl=='e10'?10:lvl=='t'?13:lvl=='m'?18:null;return {blockSize:16384,totalBlocks:2000,usedBlocks:0,freeBlocks:2000,region:'USA',language:'en',country:'US',isParentalControlEnabled:age!==null,userAge:age===null?20:age,netContentRestrictions:0};}};var wiiShop=function(){this.connecting='Connecting. Please wait...';this.endWaiting=function(){};this.setWallpaper=function(){};this.disableHRP=function(){};this.enableHRP=function(){};this.jumpToEULAViewer=function(){};this.jumpToManualChannel=function(){};this.returnToUpdate=function(){};this.returnToMenu=function(){};this.jumpDataMng=function(){};this.retry=function(){};this.beginWaiting=function(){};this.endWaiting=function(){};this.error=function(){};this.closeManual=function(){};};var ECPrice=function(){};var ECAccountPayment=function(){};var ECTitleLimits=function(){};var ecCheck=true;var buttonsCheck=true;var imagesCheck=true;var soundCheck=true;var errorCheck=true;var homedata=[[],[],[],[],[],[],[]];window.region='US';try{window.parent.region='US';window.parent.showallgames=false;window.parent.hidelinkedgames=false;}catch(e){}if(window.parent===window&&typeof window.keyboard!='function'){window.keyboard=function(input,message){var v=window.prompt(message||'','');if(v===null)return;input.value=v;if(typeof input.onchange=='function')input.onchange();};}}catch(e){}</script>`
					text = strings.Replace(text, "<head>", "<head>"+mockScript, 1)
				}
				text = strings.Replace(text, "</body>", "<script type=\"text/javascript\" src=\"/shroombrew-navigation.js\"></script><script type=\"text/javascript\" src=\"/shroombrew-balance.js\"></script></body>", 1)
			}
			if filePath == "/oss/serv/B_04.html" {
				// Mocks for Wii-specific objects (ec, wiiShop, homedata, window.region)
				// so the page's other inline scripts don't throw in a browser. One
				// renderer everywhere: the ES3 catalog in shroombrew-catalog.js, so
				// search, paging and the NEW/Updated badges look the same on the
				// console and in a browser (the archived initPage needs Wii objects).
				text = shopInlineScripts.ReplaceAllStringFunc(text, func(block string) string {
					if strings.Contains(block, "function initPage()") && strings.Contains(block, "URLSearchParams") {
						return `<script>function initPage(){shroomInstallBridge();if(typeof setUnderButtonL==='function')setUnderButtonL(true,'Back','javascript:showBack()','wiiCancelSound()');}</script>`
					}
					return block
				})
				text = strings.Replace(text, "</head>", "<style type=\"text/css\">#shroomWhatsNew{display:none!important}</style></head>", 1)
				text = strings.Replace(text, "</body>", "<script type=\"text/javascript\" src=\"/shroombrew-popular.js\"></script><script type=\"text/javascript\" src=\"/shroombrew-catalog.js\"></script></body>", 1)
			}
			if filePath == "/oss/serv/B_16.html" || filePath == "/oss/serv/B_17.html" {
				// The archived genre/publisher renderer is ES6 (template literals,
				// let/const, arrow-free but still unparsable on Wii Opera). Drop
				// those blocks and use the ES3 renderer below.
				text = shopInlineScripts.ReplaceAllStringFunc(text, func(block string) string {
					if strings.Contains(block, "`") || strings.Contains(block, "=>") ||
						strings.Contains(block, "const ") || strings.Contains(block, "let ") {
						return ""
					}
					return block
				})
				text = strings.Replace(text, "</body>", "<script type=\"text/javascript\" src=\"/shroombrew-browse.js\"></script></body>", 1)
			}
			if filePath == "/oss/serv/W_01.html" {
				// The connecting screen draws the shop's two dotted rules, but the
				// captured W_01 page has none, so they vanished as soon as the shop
				// opened. Put the same rules back on the first shop page.
				dots := `<div class="dot" id="line01">` + shellDots + `</div><div class="dot" id="line02">` + shellDots + `</div>`
				script := `<script type="text/javascript" src="/shroombrew-welcome.js"></script>`
				if !strings.Contains(text, `id="line01"`) {
					text = strings.Replace(text, "</body>", dots+script+"</body>", 1)
				} else {
					text = strings.Replace(text, "</body>", script+"</body>", 1)
				}
			}
			if strings.HasPrefix(filePath, "/oss/serv/") && strings.HasSuffix(filePath, ".html") {
				// Keep the two rules on every normal shop page, including pages
				// that are not wired into the current navigation. B_13 is a
				// full-screen health warning and M_01 is the manual viewer; both
				// deliberately have no frame.
				if filePath != "/oss/serv/B_13.html" && filePath != "/oss/serv/M_01.html" &&
					!strings.Contains(text, `id="line01"`) && !strings.Contains(text, `id="line02"`) {
					dots := `<div class="dot" id="line01">` + shellDots + `</div><div class="dot" id="line02">` + shellDots + `</div>`
					text = strings.Replace(text, "</body>", dots+"</body>", 1)
				}
			}
			if filePath == "/oss/serv/B_05.html" {
				text = strings.Replace(text, "</body>", "<script type=\"text/javascript\" src=\"/shroombrew-detail.js\"></script></body>", 1)
			}
			if filePath == "/oss/serv/B_06.html" {
				if id := strings.ToUpper(r.URL.Query().Get("titleId")); id != "" {
					if app, ok := s.appByTitleID(id); ok {
						text = classicMoreDetailsPage(text, app, langFor(r))
					}
				}
			}
			if filePath == "/oss/serv/B_07.html" {
				if id := strings.ToUpper(r.URL.Query().Get("titleId")); id != "" {
					text = strings.Replace(text, `<iframe src=""`, `<iframe src="/rating?titleId=`+url.QueryEscape(id)+`"`, 1)
				}
			}
			if filePath == "/oss/serv/M_01.html" {
				// M_01 is the orphaned captured manual viewer. It hardcodes a
				// demo title; point it at the real manual archive for whatever
				// titleId was requested.
				text = strings.Replace(text, "<title>M_01.title</title>", "<title>Operations Guide</title>", 1)
				text = strings.Replace(text, "</body>", `<script type="text/javascript">function initPage(){var f=document.getElementById('manframe');if(f){f.src=location.search&&location.search.length>1?'/manual'+location.search:'/oss/serv/W_03.html';}if(typeof setUnderButtonL=='function')setUnderButtonL(true,'Back','javascript:showBack()','wiiCancelSound()');}</script></body>`, 1)
			}
			if strings.Contains(r.UserAgent(), "Wii Shop Channel/") || strings.Contains(r.UserAgent(), "Nintendo Wii") {
				text = legacyWiiFrontend(text)
			}
			body = []byte(text)
			if strings.HasSuffix(filePath, ".js") {
				w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
			} else {
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
				// Pages are rewritten per request; never let a browser keep an old copy.
				w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
			}
			if strings.HasSuffix(filePath, ".js") && isStaticShopAsset(filePath) {
				// The body is rewritten per request, so tag the bytes actually sent.
				sum := sha1.Sum(body)
				w.Header().Set("ETag", fmt.Sprintf("\"%x\"", sum[:8]))
				http.ServeContent(w, r, filePath, info.ModTime(), bytes.NewReader(body))
				return
			}
			w.Write(body)
			return
		}
		ext := strings.ToLower(filepath.Ext(filePath))
		switch ext {
		case ".png", ".gif", ".jpg", ".jpeg", ".bmp", ".ico", ".ttf", ".woff", ".css", ".wav", ".mid", ".mp3":
			w.Header().Set("Cache-Control", "public, max-age=3600")
			w.Header().Del("Pragma")
		}
		w.Header().Set("X-Content-Type-Options", "nosniff")
		http.ServeFile(w, r, localPath)
	})
}

// isStaticShopAsset reports whether a path is an archived Shop file that does
// not change between requests. Generated catalog scripts live outside /oss/.
func isStaticShopAsset(p string) bool {
	if !strings.HasPrefix(p, "/oss/") || strings.HasSuffix(p, "/games.js") || strings.HasSuffix(p, "/home.js") {
		return false
	}
	switch strings.ToLower(path.Ext(p)) {
	case ".js", ".css", ".png", ".gif", ".jpg", ".jpeg", ".ico", ".swf":
		return true
	}
	return false
}
