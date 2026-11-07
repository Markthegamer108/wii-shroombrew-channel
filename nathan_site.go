package main

import (
	"bytes"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

const nathanSiteRoot = "./site"

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
	return strings.ReplaceAll(text, "shop.wii.com", domain)
}

type classicTitle struct {
	ID            string `json:"id"`
	Title1        string `json:"title1"`
	Title2        string `json:"title2"`
	Console       string `json:"console"`
	Region        string `json:"region"`
	Language      string `json:"language"`
	Publisher     string `json:"publisher"`
	Genre         string `json:"genre"`
	Date          string `json:"date"`
	ReleaseUnix   int64  `json:"releaseUnix"`
	Added         string `json:"added"`
	Points        string `json:"points"`
	Players       string `json:"players"`
	Attributes    string `json:"attributes"`
	Controllers   string `json:"controllers"`
	Rating        string `json:"rating"`
	RatingDetails string `json:"ratingdetails"`
	Size          int64  `json:"size"`
	Link          string `json:"link"`
	Thumbnail     string `json:"thumbnail"`
	Description   string `json:"description"`
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

// classicPriceFor returns the authentic licensed price when the catalog sets
// one, 0 for Flash titles (free), and the derived price otherwise.
func classicPriceFor(app catalogApp) int {
	if app.Price > 0 {
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

func (s *storefront) classicTitles() []classicTitle {
	titles := make([]classicTitle, 0, len(s.apps)+1)
	titles = append(titles, classicTitle{}) // The imported catalog UI begins at index one.
	for _, app := range s.apps {
		category := app.Category
		if category == "" {
			category = "Homebrew"
		}
		releaseDate := ""
		if app.ReleaseDate > 0 {
			releaseDate = time.Unix(app.ReleaseDate, 0).UTC().Format("Jan 2, 2006")
		}
		size := app.FileSize.ZipUncompressed
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
		titles = append(titles, classicTitle{
			ID: classicTitleIDForApp(app), Title1: app.Name, Console: console,
			Region: "US", Language: "EN", Publisher: app.Author, Genre: category,
			Date: releaseDate, ReleaseUnix: app.ReleaseDate, Added: releaseDate, Points: strconv.Itoa(classicPriceFor(app)), Players: "", Attributes: "",
			Controllers: strings.Join(app.Peripherals, ", "), Rating: app.Rating, RatingDetails: "", Size: size,
			Link:      "/download/" + app.Slug,
			Thumbnail: "/icons/" + app.Slug + ".png", Description: app.Description.Short,
		})
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

// The welcome page only displays twenty titles. Keeping this script small
// avoids making Wii Opera parse the full catalog before it can paint the
// recommendation tiles. The catalog page still receives classicGamesJS.
func (s *storefront) classicFeaturedGamesJS() []byte {
	titles := s.classicTitles()
	if len(titles) > 21 {
		titles = titles[:21]
	}
	b, _ := json.Marshal(titles)
	return append([]byte("// Generated featured Shroombrew titles.\nvar gameslist = "), append(b, ';')...)
}

func (s *storefront) classicHomeJS() []byte {
	titles := s.classicTitles()[1:]
	ids := make([]string, 0, 20)
	for _, title := range titles {
		ids = append(ids, title.ID)
		if len(ids) == 20 {
			break
		}
	}
	if len(ids) == 0 {
		ids = []string{"0000000000000000"}
	}
	seedCount := len(ids)
	data := [][]string{
		{"Welcome to Shroombrew!", "Browse homebrew for your Wii.", "Homebrew for everyone.", "Download apps as Wii Menu channels."},
		{"Featured Homebrew", "New Homebrew", "Games", "Utilities"},
	}
	for len(ids) < 20 {
		ids = append(ids, ids[len(ids)%seedCount])
	}
	for i := 0; i < 5; i++ {
		data = append(data, ids[i*4:(i+1)*4])
	}
	b, _ := json.Marshal(data)
	return append([]byte("// Generated from Shroombrew's homebrew catalog.\nvar homedata = "), append(b, ';')...)
}

func classicWelcomeFallbackJS() []byte {
	return []byte(`(function(){
var page=0;var fillRetries=0;
function tile(title,position){
 if(!title)return;
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
 if(typeof showElement!='undefined')showElement('rec0'+position);
}

function fill(){
 if(typeof games=='undefined'||!games||!games.length){if(fillRetries++<20)setTimeout(fill,250);return;}
 for(var i=0;i<4;i++)tile(games[page*4+i],i+1);
 var heading=document.getElementById('txtSoftList');
 if(heading){var labels=['Recommended Titles','Featured Homebrew','New Homebrew','Games','Utilities'];
  if(page==0)heading.innerHTML='<span class="style1">Wii Shop Channel</span><span class="style2">Recommended Titles</span>';
  else heading.innerHTML='<nobr><span class="style1">'+labels[page]+'</span></nobr>';
 }
}
window.add_scroll_counter=function(){};
// start_animation_arrow is left to the page: the arrows bob as on the shop.
window.change_up_RecommendedTitles=function(){page=(page+1)%5;fill();};
window.change_down_RecommendedTitles=function(){page=(page+4)%5;fill();};
function arrows(){
 // Keep the page's own arrow positions (top 118px, stopped frames at 108px);
 // only wire the clicks to the homebrew pages.
 var ids=[['posLeftArrowID',change_down_RecommendedTitles],['posStopLeftArrowID',change_down_RecommendedTitles],['posRightArrowID',change_up_RecommendedTitles],['posStopRightArrowID',change_up_RecommendedTitles]];
 for(var i=0;i<ids.length;i++){
  var node=document.getElementById(ids[i][0]);if(!node)continue;
  node.style.cursor='pointer';node.onclick=(function(go){return function(){go();return false;};})(ids[i][1]);
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
// W_01 has no rules of dots; the shop and its revivals draw none on this page.
if(window.setTimeout)setTimeout(function(){fill();arrows();},900);else{fill();arrows();}
if(window.setTimeout)setTimeout(directShopHome,100);else directShopHome();
})();`)
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
 put('Title1',game.title1);put('Title2',game.title2);put('DisplayCategory',game.genre);
 put('PublisherName',game.publisher);put('Description',game.description);put('ReleaseDate',game.date?'Released '+game.date:'');
 put('Console',game.console||'Homebrew');put('NumPlayers',game.players||'');
 var thumb=document.getElementById('Thumbnail');if(thumb)thumb.src=game.thumbnail;
 var button=document.getElementById('buyButton'); if(button)button.style.display='';
 var caption=document.getElementById('text03-01'); if(caption)caption.innerHTML='Download';
 var price=document.getElementById('price'); if(price)price.innerHTML=game.points+' Wii Points';
 var link=document.getElementById('BuyAnchor');
 if(link){
  link.href='/wii-install/title/'+selected;
  link.target='_self';
  link.onclick=function(){try{wiiSelectSound();}catch(e){} shroomNavigate('/wii-install/title/'+selected);return false;};
 }
 var control=document.getElementById('ControlAnchor');if(control){control.href='/controllers?titleId='+selected;control.onclick=function(){shroomNavigate(control.href);return false;};}
 var buyImage=document.getElementById('Image2');if(buyImage)buyImage.onclick=function(){try{wiiSelectSound();}catch(e){}};
 var ratingIds=['Rating','ESRB_Rating','RatingPic'];for(var r=0;r<ratingIds.length;r++){var rating=document.getElementById(ratingIds[r]);if(rating)rating.style.display='none';}
 var unavailable=document.getElementById('notAvailableMsg');if(unavailable)unavailable.style.display='none';
 var gift=document.getElementById('giftButton');if(gift)gift.style.display='none';
}

window.initPage=function(){shroomInstallBridge();if(typeof setUnderButtonL==='function')setUnderButtonL(true,'Back','javascript:showBack()','wiiCancelSound()');makeDownloadButton();};

if(window.setTimeout){setTimeout(makeDownloadButton,700);if(window.addEventListener)window.addEventListener('load',function(){setTimeout(makeDownloadButton,100);},false);}else makeDownloadButton();
})();`)
}

// The imported website version of sound.js defines wiiSound with an ES6 class
// and tries to play HTML audio from its parent document. Wii Opera rejects
// that syntax and the Shop channel already supplies a native wiiSound object.
func classicSoundJS() []byte {
	return []byte(`var cSE_Slide=1,cSE_Forcus=2,cSE_Decide=3,cSE_Cancel=4,cSE_Choise=5,cSE_Error=6,cSE_AddPoint=7,cSE_Finish=7;
var snd;var shroomNativeSound=navigator.userAgent.indexOf('Wii Shop Channel/')!=-1||navigator.userAgent.indexOf('Nintendo Wii')!=-1;
if(shroomNativeSound){try{if(typeof wiiSound!='undefined')snd=new wiiSound();}catch(e){}}
if(!snd)snd={playSE:function(code){try{var names={2:'cSE_Forcus',3:'cSE_Decide',4:'cSE_Cancel',7:'cSE_Finish'};var a=parent.document.getElementById(names[code]);if(a){a.currentTime=0;var p=a.play();if(p&&p['catch'])p['catch'](function(){});}}catch(e){}},playBGM:function(){}};
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
function pageURL(number){var old=location.search.substring(1).split('&');var kept=[];for(var j=0;j<old.length;j++){if(old[j]&&old[j].split('=')[0]!='p')kept.push(old[j]);}kept.push('p='+number);return '/oss/serv/B_04.html?'+kept.join('&');}
function secURL(section){var old=location.search.substring(1).split('&');var kept=[];for(var j=0;j<old.length;j++){if(old[j]&&old[j].split('=')[0]!='category'&&old[j].split('=')[0]!='genre'&&old[j].split('=')[0]!='p')kept.push(old[j]);}kept.push('category='+encodeURIComponent(section));kept.push('p=1');return '/oss/serv/B_04.html?'+kept.join('&');}
function subURL(section,genre){var old=location.search.substring(1).split('&');var kept=[];for(var j=0;j<old.length;j++){var kv=old[j];if(!kv)continue;var k=kv.split('=')[0];if(k!='category'&&k!='genre'&&k!='p')kept.push(kv);}if(section)kept.push('category='+encodeURIComponent(section));if(genre)kept.push('genre='+encodeURIComponent(genre));kept.push('p=1');return '/oss/serv/B_04.html?'+kept.join('&');}
function subNav(subs,genre,section){var h='';for(var j=0;j<subs.length;j++){var name=subs[j][0],val=subs[j][1];var active=(genre==val)?'#e60000':'#000';h+='<a href="'+subURL(section,val)+'" style="color:'+active+';text-decoration:underline;margin-right:10px" onclick="wiiSelectSound();shroomNavigate(this.href);return false">'+name+'</a>';}return h;}
function nav(){
 var info=document.getElementById('listinfo');if(!info)return;
 var cur=value('category');
 var genre=value('genre');
 var html='';
 if(cur==''){
  var secs=[['All Titles',''],['Virtual Console','Virtual Console'],['WiiWare','WiiWare'],['Wii Channels','Channels'],['DLC','DLC']];
  html='<span style="color:#333">Browse section:</span> ';
  for(var j=0;j<secs.length;j++){
   var name=secs[j][0],val=secs[j][1];
   html+='<a href="'+secURL(val)+'" style="color:'+(cur==val?'#e60000':'#000')+';text-decoration:underline;margin-right:10px" onclick="wiiSelectSound();shroomNavigate(this.href);return false">'+name+'</a>';
  }
  info.innerHTML=html;return;
 }
 html='<a href="'+secURL('')+'" style="color:#000;text-decoration:underline;margin-right:10px" onclick="wiiSelectSound();shroomNavigate(this.href);return false">All Titles</a>';
 if(cur=='Virtual Console'){
  html+='<span style="color:#333;margin-right:10px">Virtual Console:</span>';
  html+=subNav([['Nintendo licensed','Virtual Console'],['Homebrew emulators','emulators']],genre,cur);
  info.innerHTML=html;return;
 }
 if(cur=='WiiWare'){
  html+='<span style="color:#333;margin-right:10px">WiiWare (licensed)</span>';
  info.innerHTML=html;return;
 }
 if(cur=='Channels'){
  html+='<span style="color:#333;margin-right:10px">Wii Channels:</span>';
  html+=subNav([['Licensed','Channels'],['Homebrew','homebrew-channels']],genre,cur);
  if(genre=='homebrew-channels'){
   html+='<br><span style="color:#333;margin-right:10px">Homebrew:</span>';
   html+=subNav([['Utilities or so','utilities-or-so'],['Normal','games']],genre,cur);
  }
  info.innerHTML=html;return;
 }
 if(cur=='DLC'){
  html+='<span style="color:#333;margin-right:10px">DLC (licensed)</span>';
  info.innerHTML=html;return;
 }
}
function render(){
 var frame=document.getElementById('catalogFrame');if(!frame||typeof gameslist=='undefined'){if(retries++<20)setTimeout(render,250);return;}
 nav();
 var page=parseInt(value('p')||'1',10);if(!page||page<1)page=1;
 var title=value('title').toLowerCase();var genre=value('genre').toLowerCase();var cat=value('category').toLowerCase();var publisher=value('publisher').toLowerCase();var list=[];var i;
 function matchGenre(g){
  g=String(g||'').toLowerCase();
  if(genre){
   if(genre=='utilities-or-so')return g=='utilities'||g=='media'||g=='demos';
   if(genre=='games')return g=='games';
   if(genre=='emulators')return g=='emulators';
   if(genre=='homebrew-channels')return g=='games'||g=='utilities'||g=='media'||g=='demos';
   return g==genre;
  }
  if(cat){
   if(cat=='virtual console')return g=='virtual console'||g=='emulators';
   if(cat=='channels')return g=='channels'||g=='games'||g=='utilities'||g=='media'||g=='demos';
   if(cat=='wiware')return g=='wiware';
   if(cat=='dlc')return g=='dlc';
  }
  return true;
 }
 for(i=1;i<gameslist.length;i++){
  var game=gameslist[i];if(!game||!game.id)continue;
  if(title&&((game.title1||'')+' '+(game.title2||'')+' '+(game.publisher||'')).toLowerCase().indexOf(title)<0)continue;
  if(!matchGenre(String(game.genre||'')))continue;
  if(publisher&&String(game.publisher||'').toLowerCase().indexOf(publisher)<0)continue;
  list.push(game);
 }
 if(value('order')=='new')list.sort(function(a,b){return (b.releaseUnix||0)-(a.releaseUnix||0);});
 else if(genre)list.sort(function(a,b){return String(a.title1||'').toLowerCase()<String(b.title1||'').toLowerCase()?-1:1;});
 else list.sort(function(a,b){var g=String(a.genre||'').toLowerCase()<String(b.genre||'').toLowerCase()?-1:1;return g?g:(String(a.title1||'').toLowerCase()<String(b.title1||'').toLowerCase()?-1:1);});
 var pages=Math.max(1,Math.ceil(list.length/3));if(page>pages)page=pages;
 var first=(page-1)*3;var html='';
 for(i=0;i<3&&first+i<list.length;i++){
  var g=list[first+i];var top=i*92;var icon=g.thumbnail||('/oss/ccs/'+g.id+'/FFFD0001');
  html+='<div style="position:absolute;left:0px;top:'+top+'px;width:488px;height:91px;z-index:1">';
  html+='<div class="contentsBlack" id="Platform">'+esc(g.console||'Homebrew')+'</div><div class="contentsBlack" id="Publisher">'+esc(g.publisher||'')+'</div>';
  html+='<div style="overflow:hidden" nowrap id="TitleName1stline"><span class="catalogTitleBlack_01">'+esc(g.title1)+'</span></div>';
  html+='<div style="overflow:hidden" nowrap id="TitleName2stline"><span class="catalogTitleBlack_01">'+esc(g.title2)+'</span></div>';
  html+='<div id="ImageforCatalogpage"><img src="'+esc(icon)+'" width="96" height="72" style="background-color:white" /></div>';
  html+='<div id="regularTitle"><div id="spacer"><a onclick="if(new Date().getTime()<shroomListReady)return false;shroomNavigate(this.href);return false" href="/oss/serv/B_05.html?titleId='+encodeURIComponent(g.id)+'"><img src="/oss/oss/common/images/spacer.gif" width="488" height="91" border="0" /></a></div><div id="points"><div align="right" class="catalogTitleBlackBold">'+esc(g.points)+' Points</div></div></div>';
  html+='<img src="/oss/oss/common/images/banner/B_04_softbanner_a.gif" width="488" height="91" border="0" /></div>';
 }
 // A press that opened this list must not also open the row now under the pointer.
 window.shroomListReady=new Date().getTime()+700;
 frame.innerHTML=html;frame.style.display='';frame.style.overflow='hidden';frame.style.left='60px';frame.style.width='488px';frame.style.background='white';frame.style.border='none';
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

// serveShopIcon serves a title's icon. Homebrew titles (and any pre-rendered
// PNG in ./icons/) are served as PNG. Real licensed Wii Shop titles have no
// PNG in ./icons/; their authentic icon is a GIF texture in the original shop
// layout site/oss/ccs/<titleId>/FFFD0000, which is served directly so the
// catalog shows the real icon without a conversion step.
func (s *storefront) serveShopIcon(w http.ResponseWriter, r *http.Request, slug string) bool {
	app, ok := s.bySlug[slug]
	if !ok || !safeSlug.MatchString(slug) {
		return false
	}
	icon := filepath.Join("./icons", slug+".png")
	if info, err := os.Stat(icon); err == nil && !info.IsDir() {
		w.Header().Set("Content-Type", "image/png")
		w.Header().Set("Cache-Control", "public, max-age=3600")
		http.ServeFile(w, r, icon)
		return true
	}
	if app.Licensed {
		ccs := filepath.Join("./site/oss/ccs", classicTitleIDForApp(app), "FFFD0000")
		if info, err := os.Stat(ccs); err == nil && !info.IsDir() {
			w.Header().Set("Content-Type", "image/gif")
			w.Header().Set("Cache-Control", "public, max-age=3600")
			http.ServeFile(w, r, ccs)
			return true
		}
	}
	return false
}

// downloadTarget returns the redirect URL for a title's WAD/package download.
// Licensed games are fetched directly from their external host (mariocube);
// homebrew uses the Open Shop Channel package mirror.
func (s *storefront) downloadTarget(app catalogApp) (string, bool) {
	if app.Licensed {
		return app.URL.Zip, true
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
		if r.URL.Path == "/points/card" {
			s.cardEntryPage(w, r)
			return
		}
		if r.URL.Path == "/points/buy" {
			s.buyPointsPage(w, r)
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
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
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
			w.Write(s.classicFeaturedGamesJS())
			return
		case "/oss/serv/home.js", "/home.js":
			w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
			w.Write(s.classicHomeJS())
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
		case "/shroombrew-catalog.js":
			w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
			w.Write(classicCatalogFallbackJS())
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
		localPath := filepath.Join(nathanSiteRoot, filepath.FromSlash(strings.TrimPrefix(filePath, "/")))
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
			text = strings.ReplaceAll(text, "top_L_arrow_S.png", "top_L_arrow_00.png")
			text = strings.ReplaceAll(text, "top_R_arrow_S.png", "top_R_arrow_00.png")
			if strings.HasSuffix(filePath, ".html") {
				text = strings.Replace(text, "<head>", "<head><script src=\""+compatURL+"\"></script>", 1)
			}

			if strings.HasPrefix(filePath, "/oss/serv/") && strings.HasSuffix(filePath, ".html") {
				text = strings.Replace(text, "</body>", "<script type=\"text/javascript\" src=\"/shroombrew-navigation.js\"></script><script type=\"text/javascript\" src=\"/shroombrew-balance.js\"></script></body>", 1)
			}
			if filePath == "/oss/serv/B_04.html" {
				// Use one catalog renderer on both engines. The imported renderer
				// requires arrow functions and template literals absent in Wii Opera.
				text = shopInlineScripts.ReplaceAllStringFunc(text, func(block string) string {
					if strings.Contains(block, "function initPage()") && strings.Contains(block, "URLSearchParams") {
						return `<script>function initPage(){shroomInstallBridge();if(typeof setUnderButtonL==='function')setUnderButtonL(true,'Back','javascript:showBack()','wiiCancelSound()');}</script>`
					}
					return block
				})
				text = strings.Replace(text, "</head>", "<style type=\"text/css\">#shroomWhatsNew{display:none!important}</style></head>", 1)
				text = strings.Replace(text, "</body>", "<script type=\"text/javascript\" src=\"/shroombrew-catalog.js\"></script></body>", 1)
			}
			if filePath == "/oss/serv/W_01.html" {
				text = strings.Replace(text, "</body>", "<script type=\"text/javascript\" src=\"/shroombrew-welcome.js\"></script></body>", 1)
			}
			if filePath == "/oss/serv/B_05.html" {
				text = strings.Replace(text, "</body>", "<script type=\"text/javascript\" src=\"/shroombrew-detail.js\"></script></body>", 1)
			}
			if strings.Contains(r.UserAgent(), "Wii Shop Channel/") || strings.Contains(r.UserAgent(), "Nintendo Wii") {
				text = legacyWiiFrontend(text)
			}
			body = []byte(text)
			if strings.HasSuffix(filePath, ".js") {
				w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
			} else {
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
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
