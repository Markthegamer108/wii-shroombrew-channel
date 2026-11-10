/* Translation helper. Language files live in lang/<code>.json and fall back to
   English line by line, so a translation can be finished over time.
   data-i18n="key" sets text, data-i18n-ph="key" sets a placeholder.
   <nav id="nav" data-page="..."> is filled in here so every page shares one menu. */
(function () {
  var LANGS = [["en", "English"], ["ja", "日本語"], ["de", "Deutsch"], ["fr", "Français"], ["es", "Español"],
    ["it", "Italiano"], ["nl", "Nederlands"], ["ko", "한국어"], ["zh-cn", "简体中文"], ["zh-tw", "繁體中文"], ["pt", "Português"]];
  var MENU = [["index", "#index", "n_community"], ["cards", "#cards", "n_cards"], ["link", "#link", "n_link"],
    ["friends", "#friends", "n_friends"], ["history", "#history", "h_title"], ["titles", "#titles", "n_titles"],
    ["support", "#support", "n_support"], ["status", "#status", "n_status"], ["shop", "https://shop.shroombrew.org/", "n_shop"], ["wad", "#wad", "n_wad"]];
  var en = {}, cur = {}, code = "en", waiting = [], loaded = false;

  function getCookie(n) { var m = document.cookie.match(new RegExp("(?:^|; )" + n + "=([^;]*)")); return m ? m[1] : null; }
  function setCookie(n, v) { document.cookie = n + "=" + v + "; path=/; max-age=157680000; SameSite=Lax"; }
  function known(c) { for (var i = 0; i < LANGS.length; i++) if (LANGS[i][0] === c) return true; return false; }

  function pick() {
    var c = getCookie("shroombrew_lang");
    try { c = c || localStorage.getItem("shroombrewLang"); } catch (e) {}
    if (c && known(c)) return c;
    var n = (navigator.language || "en").toLowerCase().replace("_", "-");
    if (n.indexOf("zh") === 0) return /tw|hk|mo|hant/.test(n) ? "zh-tw" : "zh-cn";
    n = n.slice(0, 2);
    return known(n) ? n : "en";
  }

  function get(url, cb) {
    var x = new XMLHttpRequest();
    x.open("GET", url, true);
    x.onreadystatechange = function () {
      if (x.readyState !== 4) return;
      var d = {};
      if (x.status === 200) { try { d = JSON.parse(x.responseText); } catch (e) {} }
      cb(d);
    };
    x.send();
  }

  function t(key, vars) {
    var s = cur[key];
    if (s === undefined || s === "") s = en[key];
    if (s === undefined) s = key;
    if (vars) for (var k in vars) if (vars.hasOwnProperty(k)) s = s.split("{" + k + "}").join(vars[k]);
    return s;
  }

  function apply() {
    var i, els = document.querySelectorAll("[data-i18n]");
    for (i = 0; i < els.length; i++) els[i].textContent = t(els[i].getAttribute("data-i18n"));
    els = document.querySelectorAll("[data-i18n-ph]");
    for (i = 0; i < els.length; i++) els[i].setAttribute("placeholder", t(els[i].getAttribute("data-i18n-ph")));
    document.documentElement.setAttribute("lang", code);
    buildNav();
    var wrap = document.querySelector(".wrap");
    if (wrap) {
      var cr = document.getElementById("credit");
      if (!document.getElementById("bottomrule")) {
        var br = document.createElement("div"); br.id = "bottomrule"; br.className = "rule";
        wrap.appendChild(br);
      }
      if (!cr) { cr = document.createElement("p"); cr.id = "credit"; cr.className = "muted"; wrap.appendChild(cr); }
      cr.textContent = t("ft_credit");
    }
    var sites = document.querySelectorAll(".site");
    for (i = 0; i < sites.length; i++) sites[i].textContent = location.host;
  }

  function buildNav() {
    var nav = document.getElementById("nav");
    if (!nav) return;
    var page = nav.getAttribute("data-page"), i, keep = document.getElementById("langpick");
    while (nav.firstChild) nav.removeChild(nav.firstChild);
    for (i = 0; i < MENU.length; i++) {
      var el = document.createElement(MENU[i][0] === page ? "span" : "a");
      if (MENU[i][0] === page) el.className = "cur"; else el.href = MENU[i][1];
      el.textContent = t(MENU[i][2]);
      nav.appendChild(el);
    }
    if (!keep) {
      keep = document.createElement("select");
      keep.id = "langpick";
      for (i = 0; i < LANGS.length; i++) {
        var o = document.createElement("option");
        o.value = LANGS[i][0]; o.textContent = LANGS[i][1];
        keep.appendChild(o);
      }
      keep.onchange = function () { I18N.set(keep.value); };
    }
    keep.value = code;
    keep.title = t("lang");
    nav.appendChild(keep);
  }

  function load(c, done) {
    code = c;
    get("lang/en.json", function (e) {
      en = e;
      if (c === "en") { cur = {}; return done(); }
      get("lang/" + c + ".json", function (d) { cur = d; done(); });
    });
  }

  function finish() {
    loaded = true; apply();
    var f = waiting; waiting = [];
    for (var i = 0; i < f.length; i++) f[i]();
  }

  window.I18N = {
    t: t,
    refresh: function () { apply(); },
    lang: function () { return code; },
    ready: function (fn) { if (loaded) fn(); else waiting.push(fn); },
    onChange: null,
    set: function (c) {
      setCookie("shroombrew_lang", c);
      try { localStorage.setItem("shroombrewLang", c); } catch (e) {}
      load(c, function () { apply(); if (I18N.onChange) I18N.onChange(); });
    }
  };
  load(pick(), finish);
})();
