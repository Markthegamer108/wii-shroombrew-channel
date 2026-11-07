/* Shroombrew seasonal effects, shared by the web shop (shop.shroombrew.org)
   and the community site (shroombrew.org):
     - December: falling snow
     - 14 January ("The Mii Channel theme but it's reversed" day): reversed text
     - shroombrew.org only: the shop's music (the same track the shop plays
       today), an optional dark mode, and a settings panel for all of them.
   Settings live in one cookie on .shroombrew.org, so a choice made on either
   site applies to both. The shop's own Settings page (S_16_6) uses shroomFX. */
(function () {
  if (window.shroomFX) return;
  var isShop = !!document.getElementById("mariocanvas") || /^shop\./.test(location.hostname);
  var base = isShop ? "" : "https://shop.shroombrew.org";

  // #fxdate=YYYY-MM-DD previews another day's effects. (Not a query string:
  // the shop shell reads any query as a title to open.)
  var now = new Date(), fx = /fxdate=(\d{4})-(\d{2})-(\d{2})/.exec(location.hash);
  if (fx) now = new Date(+fx[1], +fx[2] - 1, +fx[3], 12);
  var month = now.getMonth(), day = now.getDate();

  // ---- settings ------------------------------------------------------------
  var KEYS = { music: "m", snow: "s", reverse: "r", dark: "d" };
  var OFF_BY_DEFAULT = { dark: true };
  function readCookie() {
    var m = /(?:^|;\s*)sbfx=([a-z0-9]*)/.exec(document.cookie);
    return m ? m[1] : "";
  }
  function get(name) {
    var v = readCookie(), k = KEYS[name], i = v.indexOf(k);
    return i < 0 ? !OFF_BY_DEFAULT[name] : v.charAt(i + 1) !== "0";
  }
  function set(name, on) {
    var out = "", n;
    for (n in KEYS) out += KEYS[n] + ((n === name ? on : get(n)) ? "1" : "0");
    var domain = /(^|\.)shroombrew\.org$/.test(location.hostname) ? ";domain=.shroombrew.org" : "";
    document.cookie = "sbfx=" + out + ";path=/;max-age=31536000;SameSite=Lax" + domain;
    apply();
  }

  // ---- today's track (keep in step with main.js) ---------------------------
  function track() {
    if (month === 3 && day === 1) return ["theme1deranged"];
    if (month === 11) return ["theme1christmas"];
    if (month === 5 && day === 26) return ["theme1attention"];
    if (month === 0 && day === 14) return ["theme1reversed"];
    if (month === 0 && day === 30) return ["theme1depressing"];
    if (month === 1 && day === 9) return ["theme1ss"];
    if (month === 4 && day === 5) return ["theme1sixyearold"];
    if (month === 9 && day === 31) return ["theme1discordant"];
    return ["theme1", "theme2"];
  }
  var isSnowDay, isReverseDay;
  function today() {
    if (!fx) { now = new Date(); month = now.getMonth(); day = now.getDate(); }
    isSnowDay = month === 11;
    isReverseDay = month === 0 && day === 14;
  }
  today();

  // ---- snow ----------------------------------------------------------------
  var snowCanvas = null, snowTimer = null, flakes = [];
  function startSnow() {
    if (snowCanvas || !document.body) return;
    snowCanvas = document.createElement("canvas");
    snowCanvas.setAttribute("aria-hidden", "true");
    snowCanvas.style.cssText = "position:fixed;left:0;top:0;width:100%;height:100%;pointer-events:none;z-index:2147483000";
    document.body.appendChild(snowCanvas);
    var ctx = snowCanvas.getContext("2d"), w = 0, h = 0;
    // Phones get fewer flakes than tablets and desktops, and the canvas is
    // only resized when the screen changes (rotation, resizing).
    function size() {
      w = snowCanvas.width = window.innerWidth;
      h = snowCanvas.height = window.innerHeight;
    }
    size();
    snowCanvas.__sbResize = size;
    window.addEventListener("resize", size);
    var count = Math.max(30, Math.min(90, Math.round(w * h / 12000)));
    flakes = [];
    for (var i = 0; i < count; i++) flakes.push({ x: Math.random(), y: Math.random(), r: 1 + Math.random() * 3, v: 0.0008 + Math.random() * 0.0018, d: Math.random() * 6.28 });
    snowTimer = setInterval(function () {
      if (document.hidden) return; // no work while the tab is in the background
      ctx.clearRect(0, 0, w, h);
      // Black flakes on light pages, white ones in dark mode.
      ctx.fillStyle = /(^|\s)sb-dark(\s|$)/.test(document.documentElement.className) ? "rgba(255,255,255,0.9)" : "rgba(0,0,0,0.75)";
      for (var j = 0; j < flakes.length; j++) {
        var f = flakes[j];
        f.y += f.v; f.d += 0.02;
        if (f.y > 1.02) { f.y = -0.02; f.x = Math.random(); }
        ctx.beginPath();
        ctx.arc((f.x + Math.sin(f.d) * 0.01) * w, f.y * h, f.r, 0, 6.283);
        ctx.fill();
      }
    }, 33);
  }
  function stopSnow() {
    if (!snowCanvas) return;
    clearInterval(snowTimer);
    window.removeEventListener("resize", snowCanvas.__sbResize);
    snowCanvas.parentNode.removeChild(snowCanvas);
    snowCanvas = null;
  }

  // ---- reversed text -------------------------------------------------------
  // Each text node is reversed once; the original comes back when turned off.
  var reversed = [], observers = [], reverseOn = false;
  var SKIP = { SCRIPT: 1, STYLE: 1, TEXTAREA: 1, NOSCRIPT: 1, OPTION: 1 };
  function flip(s) { return Array.from ? Array.from(s).reverse().join("") : s.split("").reverse().join(""); }
  function fixNode(t) {
    var p = t.parentNode, v = t.nodeValue;
    for (var a = p; a; a = a.parentNode) if (a.id === "sbfx-panel") return;
    // The shop's dotted divider lines are decoration, not words.
    if (!p || SKIP[p.nodeName] || /(^|\s)dot(\s|$)/.test(p.className || "") || !v || !/\S/.test(v) || t.__sbRev === v) return;
    var r = flip(v);
    t.__sbOrig = v; t.__sbRev = r; t.nodeValue = r;
    reversed.push(t);
  }
  function walk(node) {
    if (!node) return;
    if (node.nodeType === 3) { fixNode(node); return; }
    // The settings panel stays readable so the switch can still be found.
    if (node.nodeType !== 1 || SKIP[node.nodeName] || node.id === "sbfx-panel") return;
    if (node.nodeName === "IFRAME") { hookFrame(node); return; }
    for (var c = node.firstChild; c; c = c.nextSibling) walk(c);
  }
  function reverseDoc(doc) {
    if (!reverseOn || !doc || !doc.body || doc.__sbObserved) return;
    doc.__sbObserved = true;
    walk(doc.body);
    var Obs = doc.defaultView && doc.defaultView.MutationObserver;
    if (!Obs) return;
    var o = new Obs(function (list) {
      if (!reverseOn) return;
      for (var i = 0; i < list.length; i++) {
        var m = list[i];
        if (m.type === "characterData") fixNode(m.target);
        else for (var j = 0; j < m.addedNodes.length; j++) walk(m.addedNodes[j]);
      }
    });
    o.observe(doc.body, { childList: true, subtree: true, characterData: true });
    observers.push({ o: o, doc: doc });
  }
  function hookFrame(f) {
    if (!f.__sbHooked) {
      f.__sbHooked = true;
      f.addEventListener("load", function () { try { reverseDoc(f.contentDocument); } catch (e) {} });
    }
    try { reverseDoc(f.contentDocument); } catch (e) {}
  }
  function startReverse() {
    if (reverseOn) return;
    reverseOn = true;
    reverseDoc(document);
  }
  function stopReverse() {
    if (!reverseOn) return;
    reverseOn = false;
    for (var i = 0; i < observers.length; i++) { observers[i].o.disconnect(); observers[i].doc.__sbObserved = false; }
    observers = [];
    for (var j = 0; j < reversed.length; j++) {
      var t = reversed[j];
      if (t.nodeValue === t.__sbRev) t.nodeValue = t.__sbOrig;
      t.__sbRev = null;
    }
    reversed = [];
  }

  // ---- music (shroombrew.org only; the shop has its own) -------------------
  // Moving to another page keeps the song going: the position is saved for the
  // browser tab and the next page carries on from it instead of restarting.
  var audio = null, audioIndex = 0, wantsMusic = false, SAVE = "sbfxMusic";
  function savePosition() {
    if (!audio) return;
    try {
      sessionStorage.setItem(SAVE, JSON.stringify({ list: track().join(","), i: audioIndex, t: audio.currentTime, playing: !audio.paused, at: new Date().getTime() }));
    } catch (e) {}
  }
  function savedPosition() {
    try {
      var s = JSON.parse(sessionStorage.getItem(SAVE) || "null");
      if (s && s.list === track().join(",") && new Date().getTime() - s.at < 30 * 60 * 1000) return s;
    } catch (e) {}
    return null;
  }
  function startMusic() {
    wantsMusic = true;
    if (!audio) {
      var list = track(), saved = savedPosition();
      if (saved && saved.i < list.length) audioIndex = saved.i;
      audio = new Audio(base + "/snd/" + list[audioIndex] + ".mp3");
      audio.volume = 0.35;
      audio.loop = list.length === 1 || audioIndex === 1;
      if (saved) {
        // Add the moments spent loading this page so the beat lines up.
        var resumeAt = saved.t + (saved.playing ? (new Date().getTime() - saved.at) / 1000 : 0);
        audio.addEventListener("loadedmetadata", function seek() {
          audio.removeEventListener("loadedmetadata", seek);
          var d = audio.duration;
          try { audio.currentTime = d && resumeAt >= d ? (audio.loop ? resumeAt % d : Math.max(0, d - 0.1)) : resumeAt; } catch (e) {}
        });
      }
      setInterval(savePosition, 1000);
      window.addEventListener("pagehide", savePosition);
      audio.addEventListener("ended", function () {
        // The intro plays once, then the loop repeats, as in the shop.
        if (list.length > 1 && audioIndex === 0) {
          audioIndex = 1;
          audio.src = base + "/snd/" + list[1] + ".mp3";
          audio.loop = true;
          audio.play();
        }
      });
    }
    var p = audio.play();
    if (p && p["catch"]) p["catch"](function () {});
  }
  function stopMusic() { wantsMusic = false; if (audio) audio.pause(); }
  // Browsers only allow sound after the first click or key press.
  function unlock() {
    document.removeEventListener("pointerdown", unlock, true);
    document.removeEventListener("touchend", unlock, true);
    document.removeEventListener("keydown", unlock, true);
    if (wantsMusic && audio && audio.paused) startMusic();
  }

  // ---- dark mode (shroombrew.org) -------------------------------------------
  // Overrides style.css's palette: the same cyan on a deep blue-grey page.
  var DARK_CSS =
    "html.sb-dark{--ink:#e3ecf0;--mute:#93a6af;--pale:#1b3a49;--edge:#25495a;--cyan-d:#5fd0f5;background:#0f1a20;color-scheme:dark}" +
    "html.sb-dark body{background:linear-gradient(90deg,#173444 0,#0f1a20 70px,#0f1a20 calc(100% - 70px),#173444 100%)}" +
    "html.sb-dark .box,html.sb-dark nav a,html.sb-dark input,html.sb-dark select,html.sb-dark textarea,html.sb-dark button,html.sb-dark .btn{background:#15242c}" +
    "html.sb-dark nav span.cur,html.sb-dark button.main,html.sb-dark .btn.main{background:var(--cyan);color:#fff;border-color:#0f1a20}" +
    "html.sb-dark .card{box-shadow:0 0 0 3px #0f1a20,0 0 0 5px var(--cyan)}" +
    "html.sb-dark .ok{color:#4cc46e}html.sb-dark .bad{color:#ef6b5b}" +
    "html.sb-dark #sbfx-panel,html.sb-dark #sbfx-btn{background:#15242c!important;color:#e3ecf0!important}" +
    "html.sb-dark #sbfx-btn{color:#5fd0f5!important}";
  function setDark(on) {
    var root = document.documentElement;
    if (on && !document.getElementById("sbfx-dark")) {
      var st = document.createElement("style");
      st.id = "sbfx-dark";
      st.appendChild(document.createTextNode(DARK_CSS));
      (document.head || root).appendChild(st);
    }
    root.className = root.className.replace(/(^|\s)sb-dark(\s|$)/g, " ").replace(/^\s+|\s+$/g, "") + (on ? " sb-dark" : "");
  }

  // ---- settings panel (shroombrew.org) --------------------------------------
  var panel = null, gear = null, outsideHooked = false;
  function row(label, name, note) {
    var l = document.createElement("label");
    // Rows are at least 44px tall so they are easy to tap on phones and tablets.
    l.style.cssText = "display:block;margin:4px 0;padding:6px 0;min-height:32px;cursor:pointer;-webkit-tap-highlight-color:transparent";
    var c = document.createElement("input");
    c.type = "checkbox"; c.checked = get(name);
    c.style.cssText = "width:20px;height:20px;margin:0 8px 0 0;vertical-align:middle";
    c.onchange = function () { set(name, c.checked); };
    l.appendChild(c);
    l.appendChild(document.createTextNode(label));
    if (note) {
      var s = document.createElement("span");
      s.style.cssText = "display:block;margin-left:28px;color:#777;font-size:12px";
      s.appendChild(document.createTextNode(note));
      l.appendChild(s);
    }
    return l;
  }
  function buildPanel() {
    if (isShop || panel || !document.body) return;
    var btn = document.createElement("button");
    btn.type = "button";
    btn.title = "Settings";
    btn.id = "sbfx-btn";
    btn.appendChild(document.createTextNode("⚙"));
    // Safe-area insets keep the button clear of iPhone home bars and notches.
    btn.style.cssText = "position:fixed;right:12px;bottom:12px;right:calc(12px + env(safe-area-inset-right,0px));bottom:calc(12px + env(safe-area-inset-bottom,0px));z-index:2147483001;width:48px;height:48px;padding:0;border-radius:24px;border:2px solid #34beed;background:#fff;color:#34beed;font:24px/1 Arial;cursor:pointer;box-shadow:0 2px 6px rgba(0,0,0,.2);touch-action:manipulation;-webkit-tap-highlight-color:transparent";
    panel = document.createElement("div");
    panel.id = "sbfx-panel";
    panel.style.cssText = "display:none;position:fixed;right:12px;bottom:68px;right:calc(12px + env(safe-area-inset-right,0px));bottom:calc(68px + env(safe-area-inset-bottom,0px));z-index:2147483001;width:260px;max-width:calc(100vw - 24px);max-height:calc(100vh - 100px);overflow-y:auto;-webkit-overflow-scrolling:touch;box-sizing:border-box;padding:10px 14px;background:#fff;border:2px solid #34beed;border-radius:12px;font:15px Arial,sans-serif;color:#333;box-shadow:0 2px 10px rgba(0,0,0,.2)";
    // Tapping outside the panel closes it, as people expect on touch screens.
    if (!outsideHooked) {
      outsideHooked = true;
      document.addEventListener("click", function (e) {
        if (panel && panel.style.display !== "none" && e.target !== gear && !panel.contains(e.target)) panel.style.display = "none";
      }, true);
    }
    var h = document.createElement("div");
    h.style.cssText = "font-weight:bold;color:#34beed;margin-bottom:4px";
    h.appendChild(document.createTextNode("Settings"));
    panel.appendChild(h);
    panel.appendChild(row("Dark mode", "dark", "Dark background for this website."));
    panel.appendChild(row("Music", "music", "Plays the Wii Shop music, with special tracks on some days."));
    // Seasonal options only exist on their day; the next day they are gone.
    if (isSnowDay) panel.appendChild(row("Snowflakes", "snow", "Falls throughout December."));
    if (isReverseDay) panel.appendChild(row("Reversed text", "reverse", "Today only, for the reversed Mii Channel theme."));
    if (isSnowDay || isReverseDay) {
      var n = document.createElement("div");
      n.style.cssText = "color:#777;font-size:11px;margin-top:6px";
      n.appendChild(document.createTextNode("This also applies to the web shop."));
      panel.appendChild(n);
    }
    btn.onclick = function () { panel.style.display = panel.style.display === "none" ? "block" : "none"; };
    document.body.appendChild(panel);
    document.body.appendChild(btn);
    gear = btn;
  }
  function rebuildPanel() {
    if (!panel) return;
    var open = panel.style.display !== "none";
    panel.parentNode.removeChild(panel);
    gear.parentNode.removeChild(gear);
    panel = gear = null;
    buildPanel();
    if (open && panel) panel.style.display = "block";
  }

  // When the clock passes midnight, the old day's effects and their options go
  // away (and the new day's appear) even if the page stays open.
  function scheduleMidnight() {
    if (fx) return;
    var t = new Date(), next = new Date(t.getFullYear(), t.getMonth(), t.getDate() + 1, 0, 0, 5);
    setTimeout(function () {
      var snow = isSnowDay, rev = isReverseDay;
      today();
      window.shroomFX.isSnowDay = isSnowDay;
      window.shroomFX.isReverseDay = isReverseDay;
      if (snow !== isSnowDay || rev !== isReverseDay) rebuildPanel();
      apply();
      scheduleMidnight();
    }, next - t);
  }

  function apply() {
    if (isSnowDay && get("snow")) startSnow(); else stopSnow();
    if (isReverseDay && get("reverse")) startReverse(); else stopReverse();
    if (!isShop) { setDark(get("dark")); if (get("music")) startMusic(); else stopMusic(); }
  }

  window.shroomFX = {
    get: get, set: set, isSnowDay: isSnowDay, isReverseDay: isReverseDay,
    playing: function () { return !!audio && !audio.paused; }
  };
  document.addEventListener("pointerdown", unlock, true);
  document.addEventListener("touchend", unlock, true); // older iPhones/iPads
  document.addEventListener("keydown", unlock, true);
  function init() { buildPanel(); apply(); scheduleMidnight(); }
  if (document.body) init(); else document.addEventListener("DOMContentLoaded", init);
})();
