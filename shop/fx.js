/* Shroombrew special days, shared by the web shop (shop.shroombrew.org) and
   the community site (shroombrew.org). Each day that has its own track also
   has its own look, in both places:
     December      All I Want For Christmas Is You       falling snow
     14 January    Mii Channel theme reversed            reversed text
     27 January    Orchestral Remix (Mozart's birthday)  concert-hall lights
     30 January    Depressing / Goodbye Wii Shop piano   grey page and rain
     31 January    Chill Lo-Fi Remix                     faded lo-fi colour
     9 February    Wii Shop Channel but it's ss          small-caps text, faded colour
     13 February   Synth/Metal Cover (Black Sabbath)     dark, red edges
     10 March      Luigi Time!!! (MAR10 Day)             falling coins
     18 March      ...uncomfortably long pauses          the page stalls now and then
     20 March      Nicky Flowers remix (spring)          falling petals
     88th day      Charles Cornell piano cover           piano keys (World Piano Day)
     1 April       wii theme but it is deranged          the page wobbles
     18 April      Eurobeat Remix (Initial D, 1998)      speed lines
     30 April      Smooth Jazz Guitar (Jazz Day)         smoky club lights
     5 May         made by a 6 year old                  crayon colours, Comic Sans
     17 May        Minecraft Note Blocks (2009)          falling music notes
     18 May        Wii Theme but its All Star (Shrek)    swamp green
     26 June       Attentiion                            the page pulses on the beat
     23 July       Bardcore Tavern Cover                 parchment
     9 September   Music Box (Teddy Bear Day)            soft colours, twinkles
     15 September  Megalovania Edition (Undertale)       dark edges, the page shakes
     21 September  Wii Theme but its September           disco colours
     31 October    Mii Channel ... discordant            glitches that get worse
     19 November   every remix (the Wii launched, 2006)  confetti
   An effect and its switch only exist on its day. shroombrew.org also plays
   the day's music and has dark mode; its settings sit behind the gear button.
   The shop's Settings > Seasonal Effects page (S_16_6) uses shroomFX.
   Settings live in one cookie on .shroombrew.org, so a choice made on either
   site applies to both. */
(function () {
  if (window.shroomFX) return;
  // The server also hosts a copy of the website under /community/.
  var isShop = !!document.getElementById("mariocanvas") || (/^shop\./.test(location.hostname) && location.pathname.indexOf("/community/") !== 0);
  var base = isShop ? "" : "https://shop.shroombrew.org";

  // #fxdate=YYYY-MM-DD previews another day's effects. (Not a query string:
  // the shop shell reads any query as a title to open.)
  var now = new Date(), fx = /fxdate=(\d{4})-(\d{2})-(\d{2})/.exec(location.hash);
  if (fx) now = new Date(+fx[1], +fx[2] - 1, +fx[3], 12);
  var month = now.getMonth(), day = now.getDate();

  // What moves on each day: the page box on shroombrew.org, the two page
  // frames in the shop (the shell scales #app itself, so leave that alone).
  var MOVE = ".wrap,#frame,#otherframe";

  // A layer over the whole top page (never the frames, or it would stack).
  function overlay(bg) {
    return "html::after{content:\"\";position:fixed;left:0;top:0;right:0;bottom:0;pointer-events:none;z-index:2147482999;background:" + bg + "}";
  }
  var VIGNETTE = "radial-gradient(ellipse at center,rgba(0,0,0,0) 55%,";
  function isDay(m, d) { return function () { return month === m && day === d; }; }
  // World Piano Day is the 88th day of the year.
  function dayOfYear() { return Math.round((new Date(now.getFullYear(), month, day) - new Date(now.getFullYear(), 0, 1)) / 86400000) + 1; }

  // ---- the special days ------------------------------------------------------
  // css goes into the top page; frameCss into every page, frames included.
  var EFFECTS = [
    { name: "snow", key: "s", label: "Snowflakes", note: "Falls throughout December.",
      when: function () { return month === 11; },
      start: function () { startParticles("snow"); }, stop: stopParticles },
    { name: "reverse", key: "r", label: "Reversed text", note: "For the reversed Mii Channel theme.",
      when: isDay(0, 14),
      start: startReverse, stop: stopReverse },
    { name: "concert", key: "a", label: "Concert-hall lights", note: "Mozart's birthday, for the Orchestral Remix.",
      when: isDay(0, 27),
      css: "html{filter:sepia(.2) saturate(1.1)}" + overlay(VIGNETTE + "rgba(60,30,0,.35) 100%)") },
    { name: "gloom", key: "g", label: "Grey and rain", note: "30 January, the day the Wii Shop Channel closed.",
      when: isDay(0, 30),
      css: "html{filter:grayscale(1) brightness(.88)}",
      start: function () { startParticles("rain"); }, stop: stopParticles },
    { name: "lofi", key: "b", label: "Lo-fi colours", note: "The day after the shop closed, for the Chill Lo-Fi Remix.",
      when: isDay(0, 31),
      css: "html{filter:sepia(.35) saturate(.8) contrast(.92) brightness(1.03)}" + overlay(VIGNETTE + "rgba(40,20,60,.3) 100%)") },
    { name: "smallcaps", key: "c", label: "ᴅᴇᴘʀᴇssɪɴɢ text", note: "For \"Wii Shop Channel but it's ss\".",
      when: isDay(1, 9),
      css: "html{filter:grayscale(.65)}",
      frameCss: "body,body *{font-variant:small-caps!important;text-transform:lowercase}" },
    { name: "metal", key: "e", label: "Heavy metal", note: "Black Sabbath's first album came out on 13 February 1970.",
      when: isDay(1, 13),
      css: "html{filter:grayscale(.6) contrast(1.25)}" + overlay(VIGNETTE + "rgba(160,0,0,.45) 100%)") },
    { name: "coins", key: "f", label: "Falling coins", note: "MAR10 Day, for \"Luigi Time!!!\".",
      when: isDay(2, 10),
      start: function () { startParticles("coins"); }, stop: stopParticles },
    { name: "stall", key: "h", label: "Awkward pauses", note: "Awkward Moments Day, for the uncomfortably long pauses.",
      when: isDay(2, 18),
      css: "html.sb-stall{filter:grayscale(.5);cursor:wait}",
      start: startStall, stop: stopStall },
    { name: "petals", key: "i", label: "Falling petals", note: "The first day of spring, for the Nicky Flowers remix.",
      when: isDay(2, 20),
      start: function () { startParticles("petals"); }, stop: stopParticles },
    { name: "piano", key: "j", label: "Piano keys", note: "World Piano Day (the 88th day of the year), for Charles Cornell's cover.",
      when: function () { return dayOfYear() === 88; },
      css: "html::after{content:\"\";position:fixed;left:0;right:0;bottom:0;height:22px;pointer-events:none;z-index:2147482999;" +
        "background:linear-gradient(#fff,#e8e8e8);border-top:2px solid #222}" +
        "html::before{content:\"\";position:fixed;left:0;right:0;bottom:8px;height:16px;pointer-events:none;z-index:2147483000;" +
        "background:repeating-linear-gradient(90deg,transparent 0,transparent 14px,#222 14px,#222 22px,transparent 22px,transparent 32px,#222 32px,#222 40px,transparent 40px,transparent 70px)}" },
    { name: "wobble", key: "w", label: "Deranged wobble", note: "April Fools' Day, for \"wii theme but it is deranged\".",
      when: isDay(3, 1),
      css: "@keyframes sbWobble{0%,100%{transform:rotate(-1.2deg) skewX(1deg)}25%{transform:rotate(.8deg) skewY(-.6deg) scale(1.01)}50%{transform:rotate(1.4deg) skewX(-1.2deg)}75%{transform:rotate(-.6deg) skewY(.8deg) scale(.99)}}" +
        MOVE + "{animation:sbWobble 5s ease-in-out infinite;transform-origin:50% 50%}" },
    { name: "speed", key: "l", label: "Speed lines", note: "Initial D first aired on 18 April 1998, for the Eurobeat Remix.",
      when: isDay(3, 18),
      start: function () { startParticles("speed"); }, stop: stopParticles },
    { name: "jazz", key: "n", label: "Jazz-club lights", note: "International Jazz Day, for the Smooth Jazz cover.",
      when: isDay(3, 30),
      css: "html{filter:sepia(.3) saturate(1.15)}" + overlay(VIGNETTE + "rgba(20,10,40,.5) 100%)") },
    { name: "crayon", key: "k", label: "Crayon style", note: "Children's Day, for \"made by a 6 year old\".",
      when: isDay(4, 5),
      css: "html{filter:saturate(2.2) hue-rotate(-12deg) contrast(1.05)}",
      frameCss: "body,body *{font-family:\"Comic Sans MS\",\"Comic Neue\",\"Chalkboard SE\",\"Marker Felt\",cursive!important}" },
    { name: "notes", key: "q", label: "Music notes", note: "Minecraft first came out on 17 May 2009, for the Note Block cover.",
      when: isDay(4, 17),
      start: function () { startParticles("notes"); }, stop: stopParticles },
    { name: "swamp", key: "t", label: "Swamp green", note: "Shrek opened on 18 May 2001, for \"Wii Theme but its All Star\".",
      when: isDay(4, 18),
      css: "html{filter:sepia(.25) hue-rotate(45deg) saturate(1.25)}" },
    { name: "pulse", key: "p", label: "Beat pulse", note: "For \"Attentiion\" (Wii Shop x Charlie Puth).",
      when: isDay(5, 26),
      css: "@keyframes sbPulse{0%,100%{transform:scale(1)}12%{transform:scale(1.015)}}" +
        MOVE + "{animation:sbPulse .61s ease-out infinite;transform-origin:50% 50%}" },
    { name: "parchment", key: "u", label: "Parchment", note: "For the Bardcore Tavern Cover, uploaded on 23 July 2020.",
      when: isDay(6, 23),
      css: "html{filter:sepia(.55) contrast(.95)}" + overlay(VIGNETTE + "rgba(90,50,10,.4) 100%)") },
    { name: "twinkle", key: "v", label: "Music-box twinkles", note: "Teddy Bear Day, for the Music Box version.",
      when: isDay(8, 9),
      css: "html{filter:saturate(.75) brightness(1.04)}",
      start: function () { startParticles("sparkles"); }, stop: stopParticles },
    { name: "shake", key: "y", label: "Screen shake", note: "Undertale came out on 15 September 2015, for the Megalovania Edition.",
      when: isDay(8, 15),
      css: overlay(VIGNETTE + "rgba(0,0,0,.55) 100%)") +
        "@keyframes sbShake{0%,100%{transform:translate(0,0)}20%{transform:translate(-6px,2px)}40%{transform:translate(5px,-3px)}60%{transform:translate(-4px,-2px)}80%{transform:translate(3px,3px)}}" +
        "html.sb-shake " + MOVE.split(",").join(",html.sb-shake ") + "{animation:sbShake .35s linear 2}",
      start: startShake, stop: stopShake },
    { name: "disco", key: "z", label: "Disco colours", note: "Do you remember the 21st night of September?",
      when: isDay(8, 21),
      css: "@keyframes sbDisco{0%,100%{filter:hue-rotate(0deg) saturate(1.2)}50%{filter:hue-rotate(35deg) saturate(1.5)}}html{animation:sbDisco 4s ease-in-out infinite}",
      start: function () { startParticles("sparkles"); }, stop: stopParticles },
    { name: "glitch", key: "x", label: "Glitches", note: "Halloween, for the discordant Mii Channel music.",
      when: isDay(9, 31),
      css: MOVE + "{transform:translate(var(--sbgx,0),var(--sbgy,0)) skewX(var(--sbgs,0deg));filter:hue-rotate(var(--sbgh,0deg)) invert(var(--sbgi,0))}",
      start: startGlitch, stop: stopGlitch },
    { name: "confetti", key: "o", label: "Confetti", note: "The Wii and the Wii Shop Channel launched on 19 November 2006. Every remix plays today.",
      when: isDay(10, 19),
      start: function () { startParticles("confetti"); }, stop: stopParticles }
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

  // ---- today's track (keep in step with the shop's main.js) -----------------
  var REMIX_DAYS = {
    "0-27": "remix15", "0-31": "remix01", "1-13": "remix16", "2-10": "remix13",
    "2-18": "remix06", "2-20": "remix02", "3-18": "remix05", "3-30": "remix10",
    "4-17": "remix08", "4-18": "remix09", "6-23": "remix14", "8-9": "remix11",
    "8-15": "remix07", "8-21": "remix04"
  };
  // Jan 30 is the shop's closing anniversary. It plays the old depressing
  // track or the piano tribute uploaded that day, one choice per browser tab.
  function jan30() {
    var key = "sbfxJan30", v = null;
    try { v = sessionStorage.getItem(key); } catch (e) {}
    if (!v) {
      v = Math.random() < 0.5 ? "theme1depressing" : "remix12";
      try { sessionStorage.setItem(key, v); } catch (e) {}
    }
    return v;
  }
  function track() {
    if (month === 3 && day === 1) return ["theme1deranged"];
    if (month === 11) return ["theme1christmas"];
    if (month === 5 && day === 26) return ["theme1attention"];
    if (month === 0 && day === 14) return ["theme1reversed"];
    if (month === 0 && day === 30) return [jan30()];
    if (month === 1 && day === 9) return ["theme1ss"];
    if (month === 4 && day === 5) return ["theme1sixyearold"];
    if (month === 9 && day === 31) return ["theme1discordant"];
    if (dayOfYear() === 88) return ["remix03"];
    if (REMIX_DAYS[month + "-" + day]) return [REMIX_DAYS[month + "-" + day]];
    if (month === 10 && day === 19) {
      var album = [];
      for (var i = 1; i <= 16; i++) album.push("remix" + (i < 10 ? "0" : "") + i);
      return album;
    }
    // The intro plays once, then the loop repeats, as in the shop.
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

  // ---- falling things -------------------------------------------------------
  // One canvas over the page. Each kind sets how many there are, how they move
  // and how one is drawn.
  var pCanvas = null, pTimer = null, parts = [], coinImgs = null;
  var CONFETTI = ["#34beed", "#f5c400", "#e8445a", "#4cc46e", "#a65fd8"];
  var KINDS = {
    snow: { per: 12000, max: 90, v: [0.0008, 0.0018], r: [1, 3] },
    rain: { per: 9000, max: 120, v: [0.012, 0.022], r: [1, 3] },
    coins: { per: 30000, max: 30, v: [0.002, 0.004], r: [0, 0] },
    petals: { per: 20000, max: 45, v: [0.0012, 0.0026], r: [4, 7] },
    notes: { per: 22000, max: 40, v: [0.0015, 0.003], r: [14, 22] },
    confetti: { per: 9000, max: 110, v: [0.002, 0.005], r: [3, 6] },
    sparkles: { per: 15000, max: 60, v: [0, 0], r: [1, 2.5] },
    speed: { per: 14000, max: 50, v: [0.02, 0.045], r: [40, 140] }
  };
  function draw(ctx, mode, f, w, h, dark, tick) {
    var x = f.x * w, y = f.y * h;
    if (mode === "rain") {
      ctx.strokeStyle = dark ? "rgba(200,215,225,0.5)" : "rgba(40,50,60,0.45)";
      ctx.beginPath(); ctx.moveTo(x, y); ctx.lineTo(x - 2, y + 8 + f.r * 3); ctx.stroke();
    } else if (mode === "coins") {
      var img = coinImgs[(Math.floor(tick / 4) + f.c) % 4];
      if (img.complete && img.width) ctx.drawImage(img, x + Math.sin(f.d) * 6, y, 12, 21);
    } else if (mode === "petals") {
      ctx.save(); ctx.translate(x + Math.sin(f.d) * 18, y); ctx.rotate(f.d);
      ctx.fillStyle = "rgba(246,170,200,0.85)";
      ctx.beginPath(); ctx.ellipse(0, 0, f.r, f.r / 2, 0, 0, 6.283); ctx.fill(); ctx.restore();
    } else if (mode === "notes") {
      ctx.fillStyle = dark ? "rgba(95,208,245,0.8)" : "rgba(52,190,237,0.8)";
      ctx.font = Math.round(f.r) + "px Arial,sans-serif";
      ctx.fillText(f.c % 2 ? "\u266B" : "\u266A", x + Math.sin(f.d) * 10, y);
    } else if (mode === "confetti") {
      ctx.save(); ctx.translate(x + Math.sin(f.d) * 12, y); ctx.rotate(f.d * 2);
      ctx.fillStyle = CONFETTI[f.c % CONFETTI.length];
      ctx.fillRect(-f.r, -f.r / 2, f.r * 2, f.r); ctx.restore();
    } else if (mode === "sparkles") {
      ctx.fillStyle = "rgba(255,240,170," + (0.25 + 0.65 * Math.abs(Math.sin(f.d))).toFixed(2) + ")";
      ctx.beginPath(); ctx.arc(x, y, f.r, 0, 6.283); ctx.fill();
    } else if (mode === "speed") {
      ctx.strokeStyle = dark ? "rgba(255,255,255,0.35)" : "rgba(52,190,237,0.35)";
      ctx.beginPath(); ctx.moveTo(x, y); ctx.lineTo(x + f.r, y); ctx.stroke();
    } else {
      // Snow: black flakes on light pages, white ones in dark mode.
      ctx.fillStyle = dark ? "rgba(255,255,255,0.9)" : "rgba(0,0,0,0.75)";
      ctx.beginPath(); ctx.arc(x + Math.sin(f.d) * 0.01 * w, y, f.r, 0, 6.283); ctx.fill();
    }
  }
  function startParticles(mode) {
    if (pCanvas || !document.body) return;
    var kind = KINDS[mode];
    if (mode === "coins" && !coinImgs) {
      coinImgs = [];
      for (var c = 1; c <= 4; c++) { var im = new Image(); im.src = base + "/archive-shop/assets/anim/coin" + c + ".png"; coinImgs.push(im); }
    }
    pCanvas = document.createElement("canvas");
    pCanvas.setAttribute("aria-hidden", "true");
    pCanvas.style.cssText = "position:fixed;left:0;top:0;width:100%;height:100%;pointer-events:none;z-index:2147483000";
    document.body.appendChild(pCanvas);
    var ctx = pCanvas.getContext("2d"), w = 0, h = 0, tick = 0;
    // Phones get fewer than tablets and desktops, and the canvas is only
    // resized when the screen changes (rotation, resizing).
    function size() { w = pCanvas.width = window.innerWidth; h = pCanvas.height = window.innerHeight; }
    size();
    pCanvas.__sbResize = size;
    window.addEventListener("resize", size);
    var count = Math.max(12, Math.min(kind.max, Math.round(w * h / kind.per)));
    function rnd(a) { return a[0] + Math.random() * (a[1] - a[0]); }
    parts = [];
    for (var i = 0; i < count; i++) parts.push({ x: Math.random(), y: Math.random(), r: rnd(kind.r), v: rnd(kind.v), d: Math.random() * 6.28, c: i });
    pTimer = setInterval(function () {
      if (document.hidden) return; // no work while the tab is in the background
      tick++;
      ctx.clearRect(0, 0, w, h);
      ctx.lineWidth = mode === "speed" ? 2 : 1;
      var dark = /(^|\s)sb-dark(\s|$)/.test(document.documentElement.className);
      for (var j = 0; j < parts.length; j++) {
        var f = parts[j];
        f.d += mode === "sparkles" ? 0.05 : 0.02;
        if (mode === "speed") {
          f.x -= f.v;
          if (f.x < -0.3) { f.x = 1.05; f.y = Math.random(); }
        } else {
          f.y += f.v;
          if (f.y > 1.02) { f.y = -0.04; f.x = Math.random(); }
        }
        draw(ctx, mode, f, w, h, dark, tick);
      }
    }, 33);
  }
  function stopParticles() {
    if (!pCanvas) return;
    clearInterval(pTimer);
    window.removeEventListener("resize", pCanvas.__sbResize);
    pCanvas.parentNode.removeChild(pCanvas);
    pCanvas = null;
  }

  // ---- class toggled now and then (awkward pauses, Megalovania shake) --------
  function every(cls, holdMs, gapMs) {
    var timer = null, root = document.documentElement;
    function off() { root.className = root.className.replace(new RegExp("(^|\\s)" + cls + "(\\s|$)", "g"), " ").replace(/^\s+|\s+$/g, ""); }
    return {
      start: function () {
        if (timer) return;
        (function next() {
          timer = setTimeout(function () {
            if (!document.hidden) { root.className += " " + cls; setTimeout(off, holdMs); }
            next();
          }, gapMs * (0.6 + Math.random() * 0.8));
        })();
      },
      stop: function () { clearTimeout(timer); timer = null; off(); }
    };
  }
  var stall = every("sb-stall", 1800, 9000), shake = every("sb-shake", 700, 7000);
  function startStall() { stall.start(); }
  function stopStall() { stall.stop(); }
  function startShake() { shake.start(); }
  function stopShake() { shake.stop(); }
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
      // theme2 is the shop's loop; one-track days loop their song.
      audio.loop = list.length === 1 || list[audioIndex] === "theme2";
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
        // theme1 then theme2 on repeat, as in the shop; the Nov 19 album goes
        // round from the first song. One-track days loop by themselves.
        if (list.length < 2) return;
        audioIndex = audioIndex + 1 < list.length ? audioIndex + 1 : (list[0] === "theme1" ? 1 : 0);
        audio.src = base + "/snd/" + list[audioIndex] + ".mp3";
        audio.loop = list[audioIndex] === "theme2";
        var p = audio.play();
        if (p && p["catch"]) p["catch"](function () {});
      });
    }
    // Plays straight away when the browser allows it (Chrome does once the
    // site has been used a few times, or with Sound set to Allow for it).
    var p = audio.play();
    if (p && p["catch"]) p["catch"](function () {});
  }
  function stopMusic() { wantsMusic = false; if (audio) audio.pause(); }
  // Otherwise the first click, tap or key anywhere on the page starts it.
  var UNLOCK = ["pointerdown", "mousedown", "touchend", "keydown"];
  function unlock() {
    if (!wantsMusic || !audio || !audio.paused) return;
    startMusic();
    if (!audio.paused) for (var i = 0; i < UNLOCK.length; i++) document.removeEventListener(UNLOCK[i], unlock, true);
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
  for (var u = 0; u < UNLOCK.length; u++) document.addEventListener(UNLOCK[u], unlock, true);
  function init() { buildPanel(); apply(); scheduleMidnight(); }
  if (document.body) init(); else document.addEventListener("DOMContentLoaded", init);
})();
