// Browser/Wii bridge for the preserved Shop pages. Keep this script ES3-compatible.
// Capture the console's native objects before oss.js can replace them.
var shroomNativeAPI = {
  shop: typeof wiiShop !== 'undefined' ? wiiShop : null,
  ec: typeof ECommerceInterface !== 'undefined' ? ECommerceInterface : null
};
if (typeof window.trace !== 'function') window.trace = function() {};
// Opera 9.30 predates String.includes; buttons.js calls it inside setUnderButtonL,
// and the throw stopped page setup after the first button.
if (typeof String.prototype.includes !== 'function') String.prototype.includes = function(s, from) { return this.indexOf(s, from || 0) !== -1; };
if (typeof String.prototype.startsWith !== 'function') String.prototype.startsWith = function(s, from) { return this.indexOf(s, from || 0) === 0; };
if (typeof Array.prototype.includes !== 'function') Array.prototype.includes = function(s, from) {
  for (var i = from || 0; i < this.length; i++) if (this[i] === s) return true;
  return false;
};
// The archived pages call setCursor, which only the desktop copy of oss.js defines;
// a declaration in oss.js replaces this stub where it exists.
if (typeof window.setCursor !== 'function') window.setCursor = function() {};
if (typeof window.URLSearchParams === 'undefined') {
  window.URLSearchParams = function(search) {
    this.values = {};
    var parts = String(search || '').replace(/^\?/, '').split('&');
    for (var i = 0; i < parts.length; i++) {
      var pair = parts[i].split('=');
      try {
        var key = decodeURIComponent(pair[0]);
        if (typeof this.values['$' + key] === 'undefined') this.values['$' + key] = decodeURIComponent((pair.slice(1).join('=') || '').replace(/\+/g, ' '));
      } catch (e) {}
    }
  };
  window.URLSearchParams.prototype.get = function(key) {
    var value = this.values['$' + key];
    return typeof value === 'undefined' ? null : value;
  };
  window.URLSearchParams.prototype.has = function(key) { return this.get(key) !== null; };
}
var shroomNativeClient = navigator.userAgent.indexOf('Wii Shop Channel/') !== -1 || navigator.userAgent.indexOf('Nintendo Wii') !== -1 || shroomNativeAPI.shop !== null;
var shroomNativeHandle = null;
if (shroomNativeClient && typeof wiiShop !== 'undefined') {
  try { shroomNativeHandle = new wiiShop(); } catch (e) {}
  setInterval(function() { try { if (shroomNativeHandle) { var waiting = shroomNativeHandle.connecting; } } catch(e) {} }, 30000);
}
if (typeof window.ECommerceInterface === 'undefined') {
  window.ECommerceInterface = function() {
    return new ShroombrewECommerceInterface();
  };
}
// History entries kept by the shell are full URLs, so an absolute address must be
// used as it is; only a bare page name ("B_04.html?p=1") belongs under /oss/serv/.
function shroomResolve(url) {
  if (/^https?:\/\//i.test(url)) return url;
  return url.charAt(0) === '/' ? url : '/oss/serv/' + url;
}
function shroomNavigate(url) {
  if (!url) return;
  if (parent !== window && typeof parent.shroomGoto === 'function') {
    parent.shroomGoto(url);
  } else {
    location.href = shroomResolve(url);
  }
}
function shroomBack() {
  if (parent !== window && parent.hist && parent.hist.length > 1) {
    var previous = parent.hist[parent.hist.length - 2];
    parent.hist.pop(); parent.hist.pop();
    shroomNavigate(previous);
  } else shroomNavigate('/oss/serv/W_03.html');
}
function shroomInstallBridge() {
  if (!window.shroomBridgeTraced) {
    window.shroomBridgeTraced = true;
    // The native Shop ignores input while it is waiting; release it once the page is ready.
    if (shroomNativeAPI.shop) { try { new shroomNativeAPI.shop().endWaiting(); } catch (e) {} }
  }
  // Bracket form: "goto" is reserved in ES3, and using it after a dot stops the console's Opera parsing this file.
  if (typeof window.shroomGoto === 'function') window['goto'] = window.shroomGoto;
  window.showPage = function(url) { shroomNavigate(url); };
  window.showOldPage = window.showPage;
  window.showBack = shroomBack;
  window.showHome = function() { shroomNavigate('/oss/serv/W_03.html'); };
  window.showHelp = function() { shroomNavigate('/help'); };
  window.showPoints = function() { shroomNavigate('/points'); };
  window.showTransactions = function() { shroomNavigate('/activity'); };
  window.showSettings = function() { shroomNavigate('/settings'); };
  window.showOwnedTitles = function() { shroomNavigate('/downloaded'); };
  window.showWiiMenu = function() {
    if (shroomNativeClient && shroomNativeHandle) {
      try { shroomNativeHandle.returnToMenu(); return; } catch (e) {}
    }
    if (parent !== window) parent.location.href = '/';
    else location.href = '/';
  };
  var help = document.getElementById('helplink');
  if (help) {
    help.href = '/help';
    help.onclick = function() { shroomNavigate('/help'); return false; };
  }
  var manual = document.getElementById('ManualImageID');
  if (manual) manual.onclick = function(event) {
    if (event) event.cancelBubble=true;
    if (window.event) window.event.cancelBubble=true;
    shroomNavigate('/help'); return false;
  };
  if (typeof ec !== 'undefined' && ec) {
    try {
      ec.getCachedBalance = function() {
        return typeof shroombrewPoints === 'number' ? shroombrewPoints : 0;
      };
    } catch (e) {}
  }
  if (shroomNativeClient) { try { if (typeof snd !== 'undefined') snd.playBGM(); } catch (e) {} }
  var path = location.pathname;
  if (path.indexOf('/W_03.') !== -1) {
    // The main menu has Wii Menu on the left and Shopping Guide on the right.
    try {
      var underL = document.getElementById('underButtonL'), underR = document.getElementById('underButtonR');
      if (underL && underL.style.display === 'none') setUnderButtonL(true, 'Wii Menu', 'javascript:showWiiMenu()', 'wiiSelectSound()');
      if (underR && underR.style.display === 'none') setUnderButtonR(true, 'Shopping Guide', 'javascript:showHelp()', 'wiiSelectSound()');
    } catch (e) {}
    var links = document.getElementsByTagName('a');
    for (var i = 0; i < links.length; i++) {
      if ((links[i].getAttribute('href') || '').indexOf('platform=Wii') !== -1) {
        links[i].href = '/oss/serv/B_04.html?category=channels';
        links[i].onclick = function() { shroomNavigate(this.href); return false; };
      }
      var target = links[i].getAttribute('href') || '';
      // "javascript:shop.beginWaiting(1);showOwnedTitles()" stops at beginWaiting
      // on the console, so Titles You've Downloaded never opened.
      if (target.indexOf('showOwnedTitles') !== -1) {
        links[i].href = '/downloaded';
        links[i].onclick = function() { shroomNavigate(this.href); return false; };
      }
      // The text "Titles You've Downloaded" sits in a table cell wrapping the anchor.
      // Clicking the cell may not reach the anchor's onclick in Opera 9.30.
      if (links[i].textContent && links[i].textContent.indexOf("Downloaded") !== -1) {
        links[i].href = '/downloaded';
        links[i].onclick = function() { shroomNavigate(this.href); return false; };
      }
    }
  }
}
function shroomSetMusic(enabled) {
  if (shroomNativeClient) {
    try { if (typeof snd !== 'undefined' && snd) {
      if (enabled) snd.playBGM(); else if (snd.stopBGM) snd.stopBGM();
    } } catch (e) {}
    return;
  }
  try {
    var shell = parent === window ? window : parent;
    shell.musicvol = enabled ? 25 : 0;
    shell.setValue('musicvol', shell.musicvol);
    shell.theme1.pause(); shell.theme2.pause();
    if (enabled) {
      shell.shroomAudioUnlocked = true;
      shell.theme2.volume = .25; shell.theme2.loop = true;
      var playing = shell.theme2.play();
      if (playing && playing['catch']) playing['catch'](function() {});
    }
  } catch (e) {}
}
function shroomUnlockAudio() {
  if (shroomNativeClient) return;
  try {
    var shell = parent === window ? window : parent;
    shell.shroomAudioUnlocked = true;
    if (shell.musicvol > 0 && shell.theme1 && shell.theme2 && shell.theme1.paused && shell.theme2.paused) {
      shell.theme2.volume = shell.musicvol / 100;
      shell.theme2.loop = true;
      var playing = shell.theme2.play();
      if (playing && playing['catch']) playing['catch'](function() {});
    }
  } catch (e) {}
}
if (document.addEventListener) document.addEventListener('click', shroomUnlockAudio, false);
if (window.addEventListener) window.addEventListener('load', shroomInstallBridge, false);
else if (window.attachEvent) window.attachEvent('onload', shroomInstallBridge);
setTimeout(shroomInstallBridge, 50);
setTimeout(shroomInstallBridge, 1000);

// ---- Console registration ---------------------------------------------------
// Once per launch the shop client reports which console it is running on, so
// the server can find that console's wallet (points, purchases, friend code)
// and use its System Menu language. Identifiers are POSTed (never put in a
// URL) and the server keeps only a keyed hash. ES3 only.
function shroomRegisterDevice() {
  try {
    if (navigator.userAgent.indexOf('Wii Shop Channel/') === -1 && !shroomNativeAPI.ec) return;
    if (document.cookie.indexOf('shroombrew_reg=1') !== -1) return;
    var Ctor = shroomNativeAPI.ec;
    var ecObj = null;
    try { if (typeof ec !== 'undefined' && ec && ec.getDeviceInfo) ecObj = ec; } catch (e0) {}
    if (!ecObj && Ctor) ecObj = new Ctor();
    if (!ecObj || !ecObj.getDeviceInfo) { return; }
    var i = ecObj.getDeviceInfo();
    // A function expression: ES3 (the console's Opera) rejects declarations inside blocks.
    var f = function (k) { try { var v = i[k]; return (v === null || typeof v === 'undefined') ? '' : String(v); } catch (e1) { return ''; } };
    var body = 'd=' + encodeURIComponent(f('deviceId')) + '&s=' + encodeURIComponent(f('serial')) +
      '&w=' + encodeURIComponent(f('wirelessMACAddr')) + '&b=' + encodeURIComponent(f('bluetoothMACAddr')) +
      '&l=' + encodeURIComponent(f('language')) + '&c=' + encodeURIComponent(f('country')) + '&r=' + encodeURIComponent(f('region'));
    document.cookie = 'shroombrew_reg=1; path=/'; // session cookie: once per launch
    var x = new XMLHttpRequest();
    x.open('POST', '/device/register', true);
    x.setRequestHeader('Content-Type', 'application/x-www-form-urlencoded');
    x.onreadystatechange = function () {
      if (x.readyState !== 4) return;
      // The first page of a launch was built before we knew the language; once the
      // server confirms it, redraw this page one time in the right language.
      if (x.status === 200 && x.responseText.indexOf('"changed":true') !== -1 && document.cookie.indexOf('shroombrew_rl=1') === -1) {
        document.cookie = 'shroombrew_rl=1; path=/';
        location.reload();
        return;
      }
      if (typeof window.shroomAfterRegister === 'function') window.shroomAfterRegister();
    };
    x.send(body);
  } catch (e) {}
}
setTimeout(shroomRegisterDevice, 30);