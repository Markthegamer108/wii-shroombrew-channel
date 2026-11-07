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
  var isSnowDay = month === 11, isReverseDay = month === 0 && day === 14;

  // ---- snow ----------------------------------------------------------------
  var snowCanvas = null, snowTimer = null, flakes = [];
  function startSnow() {
    if (snowCanvas || !document.body) return;
    snowCanvas = document.createElement("canvas");
    snowCanvas.setAttribute("aria-hidden", "true");
    snowCanvas.style.cssText = "position:fixed;left:0;top:0;width:100%;height:100%;pointer-events:none;z-index:2147483000";
    document.body.appendChild(snowCanvas);
    var ctx = snowCanvas.getContext("2d");
    flakes = [];
    for (var i = 0; i < 90; i++) flakes.push({ x: Math.random(), y: Math.random(), r: 1 + Math.random() * 3, v: 0.0008 + Math.random() * 0.0018, d: Math.random() * 6.28 });
    snowTimer = setInterval(function () {
      var w = snowCanvas.width = window.innerWidth, h = snowCanvas.height = window.innerHeight;
      ctx.clearRect(0, 0, w, h);
      ctx.fillStyle = "rgba(255,255,255,0.9)";
      ctx.shadowColor = "rgba(120,170,220,0.8)";
      ctx.shadowBlur = 3;
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
    // The shop's dotted divider lines are decoration, not words.
    if (!p || SKIP[p.nodeName] || /(^|\s)dot(\s|$)/.test(p.className || "") || !v || !/\S/.test(v) || t.__sbRev === v) return;
    var r = flip(v);
    t.__sbOrig = v; t.__sbRev = r; t.nodeValue = r;
    reversed.push(t);
  }
  function walk(node) {
    if (!node) return;
    if (node.nodeType === 3) { fixNode(node); return; }
    if (node.nodeType !== 1 || SKIP[node.nodeName]) return;
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
  var audio = null, audioIndex = 0, wantsMusic = false;
  function startMusic() {
    wantsMusic = true;
    if (!audio) {
      var list = track();
      audio = new Audio(base + "/snd/" + list[0] + ".mp3");
      audio.volume = 0.35;
      audio.loop = list.length === 1;
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
  var panel = null;
  function row(label, name, note) {
    var l = document.createElement("label");
    l.style.cssText = "display:block;margin:6px 0;cursor:pointer";
    var c = document.createElement("input");
    c.type = "checkbox"; c.checked = get(name);
    c.style.marginRight = "6px";
    c.onchange = function () { set(name, c.checked); };
    l.appendChild(c);
    l.appendChild(document.createTextNode(label));
    if (note) {
      var s = document.createElement("span");
      s.style.cssText = "display:block;margin-left:22px;color:#777;font-size:11px";
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
    btn.style.cssText = "position:fixed;right:12px;bottom:12px;z-index:2147483001;width:40px;height:40px;border-radius:20px;border:2px solid #34beed;background:#fff;color:#34beed;font:20px/1 Arial;cursor:pointer;box-shadow:0 2px 6px rgba(0,0,0,.2)";
    panel = document.createElement("div");
    panel.id = "sbfx-panel";
    panel.style.cssText = "display:none;position:fixed;right:12px;bottom:60px;z-index:2147483001;width:230px;padding:10px 14px;background:#fff;border:2px solid #34beed;border-radius:10px;font:13px Arial,sans-serif;color:#333;box-shadow:0 2px 10px rgba(0,0,0,.2)";
    var h = document.createElement("div");
    h.style.cssText = "font-weight:bold;color:#34beed;margin-bottom:4px";
    h.appendChild(document.createTextNode("Settings"));
    panel.appendChild(h);
    panel.appendChild(row("Dark mode", "dark", "Dark background for this website."));
    panel.appendChild(row("Music", "music", "Plays the Wii Shop music, with special tracks on some days."));
    panel.appendChild(row("Snowflakes", "snow", "Falls throughout December."));
    panel.appendChild(row("Reversed text", "reverse", "On 14 January, for the reversed Mii Channel theme."));
    var n = document.createElement("div");
    n.style.cssText = "color:#777;font-size:11px;margin-top:6px";
    n.appendChild(document.createTextNode("Snowflakes and reversed text also apply to the web shop."));
    panel.appendChild(n);
    btn.onclick = function () { panel.style.display = panel.style.display === "none" ? "block" : "none"; };
    document.body.appendChild(panel);
    document.body.appendChild(btn);
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
  document.addEventListener("keydown", unlock, true);
  function init() { buildPanel(); apply(); }
  if (document.body) init(); else document.addEventListener("DOMContentLoaded", init);
})();
