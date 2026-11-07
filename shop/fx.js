/* Shroombrew special days, shared by the web shop (shop.shroombrew.org) and
   the community site (shroombrew.org). Each day that has its own track also
   has its own look, in both places:
     December      All I Want For Christmas Is You   falling snow
     14 January    Mii Channel theme reversed        reversed text
     30 January    Wii Shop Channel but Depressing   grey page and rain
     9 February    Wii Shop Channel but it's ss      small-caps text, faded colour
     1 April       wii theme but it is deranged      the page wobbles
     5 May         made by a 6 year old              crayon colours, Comic Sans
     26 June       Attentiion                        the page pulses on the beat
     31 October    Mii Channel ... discordant        glitches that get worse
   An effect and its switch only exist on its day. shroombrew.org also plays
   the day's music and has dark mode; its settings sit behind the gear button.
   The shop's Settings > Seasonal Effects page (S_16_6) uses shroomFX.
   Settings live in one cookie on .shroombrew.org, so a choice made on either
   site applies to both. */
(function () {
  if (window.shroomFX) return;
  var isShop = !!document.getElementById("mariocanvas") || /^shop\./.test(location.hostname);
  var base = isShop ? "" : "https://shop.shroombrew.org";

  // #fxdate=YYYY-MM-DD previews another day's effects. (Not a query string:
  // the shop shell reads any query as a title to open.)
  var now = new Date(), fx = /fxdate=(\d{4})-(\d{2})-(\d{2})/.exec(location.hash);
  if (fx) now = new Date(+fx[1], +fx[2] - 1, +fx[3], 12);
  var month = now.getMonth(), day = now.getDate();

  // What moves on each day: the page box on shroombrew.org, the two page
  // frames in the shop (the shell scales #app itself, so leave that alone).
  var MOVE = ".wrap,#frame,#otherframe";

  // ---- the special days ------------------------------------------------------
  // css goes into the top page; frameCss into every page, frames included.
  var EFFECTS = [
    { name: "snow", key: "s", label: "Snowflakes", note: "Falls throughout December.",
      when: function () { return month === 11; },
      start: function () { startParticles("snow"); }, stop: stopParticles },
    { name: "reverse", key: "r", label: "Reversed text", note: "For the reversed Mii Channel theme.",
      when: function () { return month === 0 && day === 14; },
      start: startReverse, stop: stopReverse },
    { name: "gloom", key: "g", label: "Grey and rain", note: "30 January, the day the Wii Shop Channel closed.",
      when: function () { return month === 0 && day === 30; },
      css: "html{filter:grayscale(1) brightness(.88)}",
      start: function () { startParticles("rain"); }, stop: stopParticles },
    { name: "smallcaps", key: "c", label: "ᴅᴇᴘʀᴇssɪɴɢ text", note: "For \"Wii Shop Channel but it's ss\".",
      when: function () { return month === 1 && day === 9; },
      css: "html{filter:grayscale(.65)}",
      frameCss: "body,body *{font-variant:small-caps!important;text-transform:lowercase}" },
    { name: "wobble", key: "w", label: "Deranged wobble", note: "April Fools' Day, for \"wii theme but it is deranged\".",
      when: function () { return month === 3 && day === 1; },
      css: "@keyframes sbWobble{0%,100%{transform:rotate(-1.2deg) skewX(1deg)}25%{transform:rotate(.8deg) skewY(-.6deg) scale(1.01)}50%{transform:rotate(1.4deg) skewX(-1.2deg)}75%{transform:rotate(-.6deg) skewY(.8deg) scale(.99)}}" +
        MOVE + "{animation:sbWobble 5s ease-in-out infinite;transform-origin:50% 50%}" },
    { name: "crayon", key: "k", label: "Crayon style", note: "Children's Day, for \"made by a 6 year old\".",
      when: function () { return month === 4 && day === 5; },
      css: "html{filter:saturate(2.2) hue-rotate(-12deg) contrast(1.05)}",
      frameCss: "body,body *{font-family:\"Comic Sans MS\",\"Comic Neue\",\"Chalkboard SE\",\"Marker Felt\",cursive!important}" },
    { name: "pulse", key: "p", label: "Beat pulse", note: "For \"Attentiion\" (Wii Shop x Charlie Puth).",
      when: function () { return month === 5 && day === 26; },
      css: "@keyframes sbPulse{0%,100%{transform:scale(1)}12%{transform:scale(1.015)}}" +
        MOVE + "{animation:sbPulse .61s ease-out infinite;transform-origin:50% 50%}" },
    { name: "glitch", key: "x", label: "Glitches", note: "Halloween, for the discordant Mii Channel music.",
      when: function () { return month === 9 && day === 31; },
      css: MOVE + "{transform:translate(var(--sbgx,0),var(--sbgy,0)) skewX(var(--sbgs,0deg));filter:hue-rotate(var(--sbgh,0deg)) invert(var(--sbgi,0))}",
      start: startGlitch, stop: stopGlitch }
  ];
  function todays() {
    var out = [];
    for (var i = 0; i < EFFECTS.length; i++) if (EFFECTS[i].when()) out.push(EFFECTS[i]);
    return out;
  }

  // ---- settings ------------------------------------------------------------
  var KEYS = { music: "m", dark: "d" }, OFF_BY_DEFAULT = { dark: true };
  for (var e = 0; e < EFFECTS.length; e++) KEYS[EFFECTS[e].name] = EFFECTS[e].key;
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
  function today() {
    if (!fx) { now = new Date(); month = now.getMonth(); day = now.getDate(); }
  }

  // ---- every page, frames included -------------------------------------------
  // The shop shows its pages in iframes; each newly loaded page gets the day's
  // styles (and reversed text) too.
  var active = {};
  function frameCss(top) {
    var css = "";
    for (var i = 0; i < EFFECTS.length; i++) {
      var fxi = EFFECTS[i];
      if (!active[fxi.name]) continue;
      if (fxi.frameCss) css += fxi.frameCss;
      if (top && fxi.css) css += fxi.css;
    }
    return css;
  }
  function styleDoc(doc) {
    var css = frameCss(doc === document), st = doc.getElementById("sbfx-day");
    if (!css) { if (st) st.parentNode.removeChild(st); return; }
    if (!st) {
      st = doc.createElement("style");
      st.id = "sbfx-day";
      (doc.head || doc.documentElement).appendChild(st);
    }
    if (st.textContent !== css) st.textContent = css;
  }
  function hookDoc(doc) {
    if (!doc || !doc.documentElement) return;
    styleDoc(doc);
    if (reverseOn) reverseDoc(doc);
    var frames = doc.getElementsByTagName("iframe");
    for (var i = 0; i < frames.length; i++) hookFrame(frames[i]);
  }
  function hookFrame(f) {
    if (!f.__sbHooked) {
      f.__sbHooked = true;
      f.addEventListener("load", function () { try { hookDoc(f.contentDocument); } catch (e) {} });
    }
    try { hookDoc(f.contentDocument); } catch (e) {}
  }

  // ---- falling snow / rain ---------------------------------------------------
  var pCanvas = null, pTimer = null, parts = [];
  function startParticles(mode) {
    if (pCanvas || !document.body) return;
    pCanvas = document.createElement("canvas");
    pCanvas.setAttribute("aria-hidden", "true");
    pCanvas.style.cssText = "position:fixed;left:0;top:0;width:100%;height:100%;pointer-events:none;z-index:2147483000";
    document.body.appendChild(pCanvas);
    var ctx = pCanvas.getContext("2d"), w = 0, h = 0, rain = mode === "rain";
    // Phones get fewer flakes than tablets and desktops, and the canvas is
    // only resized when the screen changes (rotation, resizing).
    function size() { w = pCanvas.width = window.innerWidth; h = pCanvas.height = window.innerHeight; }
    size();
    pCanvas.__sbResize = size;
    window.addEventListener("resize", size);
    var count = Math.max(30, Math.min(rain ? 120 : 90, Math.round(w * h / (rain ? 9000 : 12000))));
    parts = [];
    for (var i = 0; i < count; i++) parts.push({ x: Math.random(), y: Math.random(), r: 1 + Math.random() * 3, v: rain ? 0.012 + Math.random() * 0.01 : 0.0008 + Math.random() * 0.0018, d: Math.random() * 6.28 });
    pTimer = setInterval(function () {
      if (document.hidden) return; // no work while the tab is in the background
      ctx.clearRect(0, 0, w, h);
      var dark = /(^|\s)sb-dark(\s|$)/.test(document.documentElement.className);
      if (rain) {
        ctx.strokeStyle = dark ? "rgba(200,215,225,0.5)" : "rgba(40,50,60,0.45)";
        ctx.lineWidth = 1;
        ctx.beginPath();
      } else {
        // Black flakes on light pages, white ones in dark mode.
        ctx.fillStyle = dark ? "rgba(255,255,255,0.9)" : "rgba(0,0,0,0.75)";
      }
      for (var j = 0; j < parts.length; j++) {
        var f = parts[j];
        f.y += f.v; f.d += 0.02;
        if (f.y > 1.02) { f.y = -0.02; f.x = Math.random(); }
        if (rain) {
          ctx.moveTo(f.x * w, f.y * h);
          ctx.lineTo(f.x * w - 2, f.y * h + 8 + f.r * 3);
        } else {
          ctx.beginPath();
          ctx.arc((f.x + Math.sin(f.d) * 0.01) * w, f.y * h, f.r, 0, 6.283);
          ctx.fill();
        }
      }
      if (rain) ctx.stroke();
    }, 33);
  }
  function stopParticles() {
    if (!pCanvas) return;
    clearInterval(pTimer);
    window.removeEventListener("resize", pCanvas.__sbResize);
    pCanvas.parentNode.removeChild(pCanvas);
    pCanvas = null;
  }

  // ---- Halloween glitches ----------------------------------------------------
  // Rare and small at first, more often and harsher the longer the page is
  // open ("insidiously gets discordant").
  var gTimer = null, gStart = 0;
  function glitchVars(on) {
    var s = document.documentElement.style, r = function (n) { return (Math.random() * 2 - 1) * n; };
    var k = Math.min(1, (new Date().getTime() - gStart) / 180000);
    s.setProperty("--sbgx", on ? r(4 + 10 * k).toFixed(1) + "px" : "0px");
    s.setProperty("--sbgy", on ? r(2 + 6 * k).toFixed(1) + "px" : "0px");
    s.setProperty("--sbgs", on ? r(1 + 4 * k).toFixed(1) + "deg" : "0deg");
    s.setProperty("--sbgh", on ? r(40 + 140 * k).toFixed(0) + "deg" : "0deg");
    s.setProperty("--sbgi", on && Math.random() < 0.15 * k ? "1" : "0");
  }
  function startGlitch() {
    if (gTimer) return;
    gStart = new Date().getTime();
    (function next() {
      var k = Math.min(1, (new Date().getTime() - gStart) / 180000);
      gTimer = setTimeout(function () {
        if (!document.hidden) { glitchVars(true); setTimeout(function () { glitchVars(false); }, 90 + Math.random() * 120); }
        next();
      }, (7000 - 5500 * k) * (0.5 + Math.random()));
    })();
  }
  function stopGlitch() { clearTimeout(gTimer); gTimer = null; glitchVars(false); }

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
    // Special-day options only exist on their day; the next day they are gone.
    var list = todays();
    for (var i = 0; i < list.length; i++) panel.appendChild(row(list[i].label, list[i].name, list[i].note));
    if (list.length) {
      var n = document.createElement("div");
      n.style.cssText = "color:#777;font-size:11px;margin-top:6px";
      n.appendChild(document.createTextNode("Today's effect also shows in the web shop."));
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
      var before = names();
      today();
      if (names() !== before) rebuildPanel();
      apply();
      scheduleMidnight();
    }, next - t);
  }
  function names() {
    var l = todays(), s = [];
    for (var i = 0; i < l.length; i++) s.push(l[i].name);
    return s.join(",");
  }

  function apply() {
    for (var i = 0; i < EFFECTS.length; i++) {
      var fxi = EFFECTS[i], on = fxi.when() && get(fxi.name);
      if (on && !active[fxi.name]) { active[fxi.name] = true; if (fxi.start) fxi.start(); }
      else if (!on && active[fxi.name]) { active[fxi.name] = false; if (fxi.stop) fxi.stop(); }
    }
    hookDoc(document);
    if (!isShop) { setDark(get("dark")); if (get("music")) startMusic(); else stopMusic(); }
  }

  window.shroomFX = {
    get: get, set: set,
    // [{name, label, note}] for today's special-day effects (S_16_6 lists these).
    today: function () {
      var l = todays(), out = [];
      for (var i = 0; i < l.length; i++) out.push({ name: l[i].name, label: l[i].label, note: l[i].note });
      return out;
    },
    playing: function () { return !!audio && !audio.paused; }
  };
  document.addEventListener("pointerdown", unlock, true);
  document.addEventListener("touchend", unlock, true); // older iPhones/iPads
  document.addEventListener("keydown", unlock, true);
  function init() { buildPanel(); apply(); scheduleMidnight(); }
  if (document.body) init(); else document.addEventListener("DOMContentLoaded", init);
})();
