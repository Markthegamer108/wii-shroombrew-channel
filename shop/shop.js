/* ES3-compatible Wii Shop frontend. Native API reference: Open Shop Channel WSC-Web. */
var shroomShop = null;
var shroomSound = null;
var shroomPreferences = { sound: true, pointer: true };
function shroomLoadPreferences() {
 try {
  var stored = window.localStorage ? window.localStorage.getItem('shroombrew.preferences') : null;
  if (stored) { var parsed = JSON.parse(stored); if (parsed && typeof parsed.sound == 'boolean') shroomPreferences.sound = parsed.sound; if (parsed && typeof parsed.pointer == 'boolean') shroomPreferences.pointer = parsed.pointer; }
 } catch (e) {}
}
function shroomSavePreferences() { try { if (window.localStorage) window.localStorage.setItem('shroombrew.preferences', JSON.stringify(shroomPreferences)); } catch (e) {} }
function shroomApplyPreferences() {
 var sound = document.getElementById('sound-setting');
 if (sound) { sound.innerHTML = 'Sound Effects: ' + (shroomPreferences.sound ? 'On' : 'Off'); sound.className = sound.className.replace(/ is-off/g, '') + (shroomPreferences.sound ? '' : ' is-off'); }
 var pointer = document.getElementById('pointer-setting');
 if (pointer) { pointer.innerHTML = 'Pointer Highlight: ' + (shroomPreferences.pointer ? 'On' : 'Off'); pointer.className = pointer.className.replace(/ is-off/g, '') + (shroomPreferences.pointer ? '' : ' is-off'); }
}
function shroomSE(id) { try { if (shroomPreferences.sound && shroomSound) shroomSound.playSE(id); } catch (e) {} }
function shroomReady() {
 shroomLoadPreferences();
 try { if (typeof wiiSound != 'undefined') { shroomSound = new wiiSound(); shroomSound.playBGM(); } } catch (e) {}
 try { if (typeof wiiShop != 'undefined') { shroomShop = new wiiShop(); var connectingMessage = shroomShop.connecting; shroomShop.setWallpaper(0); shroomShop.enableHRP(); } } catch (e) {}
 var links = document.getElementsByTagName('a');
 for (var i = 0; i < links.length; i++) {
  links[i].onmouseover = function() { shroomSE(2); shroomHover(this, true); };
  links[i].onmouseout = function() { shroomHover(this, false); };
  links[i].onfocus = function() { shroomHover(this, true); };
  links[i].onblur = function() { shroomHover(this, false); };
  links[i].onclick = function() {
   shroomSE(this.className.indexOf('cancel') >= 0 ? 4 : 3);
   if (this.id == 'back' && document.getElementById('channel').className == 'screen-main' && shroomShop) {
    try { shroomShop.returnToMenu(); return false; } catch (e) {}
   }
   if (this.id == 'back' && document.getElementById('channel').className == 'screen-detail' && document.referrer.indexOf(location.protocol+'//'+location.host+'/catalog') == 0) {
    location.href = document.referrer; return false;
   }
   return true;
  };
 }
 var settings = document.getElementsByTagName('a');
 for (var si = 0; si < settings.length; si++) {
  if (settings[si].getAttribute('data-setting')) settings[si].onclick = function() { shroomPreferences[this.getAttribute('data-setting')] = !shroomPreferences[this.getAttribute('data-setting')]; shroomSavePreferences(); shroomApplyPreferences(); return false; };
 }
 shroomApplyPreferences();
 var form = document.getElementById('search-form');
 if (form) form.onsubmit = function() { shroomSE(3); return true; };
 var handleKey = function(event) {
  var e = event || window.event;
  var target = e.target || e.srcElement;
  if (target && (target.tagName == 'INPUT' || target.tagName == 'TEXTAREA')) return true;
  var key = e.keyCode, el = null;
  if (key == 174 || key == 187) el = document.getElementById('next-page');
  else if (key == 170 || key == 189) el = document.getElementById('previous-page');
  else if (key == 8) el = document.getElementById('back');
  if (el) { if (el.onclick() !== false) location.href = el.href; if(e.preventDefault)e.preventDefault();return false; }
  if (key == 38 || key == 40 || key == 175 || key == 176) { var divs = document.getElementsByTagName('div'); for(var j=0;j<divs.length;j++){if(divs[j].className.indexOf('scroll-panel')>=0){divs[j].scrollTop += (key==38 || key==175) ? -36 : 36; if(e.preventDefault)e.preventDefault();return false;}} }
  return true;
 };
 if (shroomShop) document.onkeypress=handleKey; else document.onkeydown=handleKey;
 var queryInput=document.getElementById('q');
 if (queryInput && typeof wiiKeyboard != 'undefined') queryInput.onmousedown=function(){try{this.blur();var keyboard=new wiiKeyboard();keyboard.call(10,1,false,'Enter the name of the title you want.');}catch(e){}};
}
function shroomHover(el, active) {
 if (!shroomPreferences.pointer) return;
 var imgs = el.getElementsByTagName('img');
 for (var i=0;i<imgs.length;i++) {
  var hover = imgs[i].getAttribute('data-hover');
  if (hover) { if (!imgs[i]._normal) imgs[i]._normal=imgs[i].src; imgs[i].src=active?'/shop/assets/'+hover:imgs[i]._normal; }
 }
}
