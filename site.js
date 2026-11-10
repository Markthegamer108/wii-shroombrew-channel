/* One page for the whole site: index.html holds a <template id="p-NAME"> per
   page and PAGES below starts it. Addresses are #NAME (&options after, like
   #status&down); 404.html sends old cards.html-style URLs here. */
(function () {
  function $(id) { return document.getElementById(id); }
  var gamesWaiting = null;
  // games.js (the whole catalog, ~1 MB) is only fetched by the pages that list titles.
  function withGames(cb) {
    if (window.gameslist) return cb();
    if (gamesWaiting) return gamesWaiting.push(cb);
    gamesWaiting = [cb];
    var s = document.createElement("script");
    s.src = App.base + "/oss/serv/games.js";
    s.onload = function () { var w = gamesWaiting; gamesWaiting = null; for (var i = 0; i < w.length; i++) w[i](); };
    document.head.appendChild(s);
  }

  var PAGES = {
    index: { title: "i_title" },

    wad: { title: "wad_h" },

    cards: {
      title: "k_title",
      start: function () {
        var KEY = "shroombrewCards", NEXT = "shroombrewNextCard", linked = false, self = this;
        function load() { try { return JSON.parse(localStorage.getItem(KEY) || "[]"); } catch (e) { return []; } }
        function save(list) { try { localStorage.setItem(KEY, JSON.stringify(list.slice(0, 25))); } catch (e) {} }
        function remove(code) { save(load().filter(function (c) { return c.code !== code; })); render(); }
        function render() {
          var box = $("cards"); if (!box) return; App.clear(box);
          load().forEach(function (c) {
            var pts = c.points || 1000;
            var d = document.createElement("div"); d.className = "card"; d.setAttribute("data-pts", pts);
            var face = document.createElement("div"); face.className = "face";
            var s = document.createElement("small"); s.textContent = I18N.t("k_card"); face.appendChild(s);
            var p = document.createElement("span"); p.className = "pts"; p.textContent = pts; face.appendChild(p);
            var n = document.createElement("div"); n.className = "num"; n.textContent = c.code; face.appendChild(n);
            d.appendChild(face);
            var body = document.createElement("div"); body.className = "body";
            var row = document.createElement("div"); row.className = "row";
            var cp = document.createElement("button"); cp.textContent = I18N.t("k_copy");
            cp.onclick = function () { App.copyText(c.code.replace(/ /g, ""), function () { cp.textContent = I18N.t("k_copied"); setTimeout(function () { cp.textContent = I18N.t("k_copy"); }, 1500); }); };
            row.appendChild(cp);
            if (linked) {
              var add = document.createElement("button"); add.className = "main"; add.textContent = I18N.t("k_addnow");
              add.onclick = function () {
                add.disabled = true;
                App.call("POST", "cards/redeem", { code: c.code }, function (st, r) {
                  if (st === 200) { App.say(I18N.t("k_added", { n: r.added })); remove(c.code); }
                  else { App.say(App.errText(r), true); if (r.key === "er_card_bad") remove(c.code); else add.disabled = false; }
                });
              };
              row.appendChild(add);
            }
            body.appendChild(row);
            d.appendChild(body); box.appendChild(d);
          });
        }
        function tick() {
          var left = 0, b = $("make");
          if (!b) return;
          try { left = Math.ceil((parseInt(localStorage.getItem(NEXT) || "0", 10) - Date.now()) / 1000); } catch (e) {}
          if (left > 0) { b.disabled = true; $("timer").textContent = I18N.t("k_wait", { n: left }); }
          else { b.disabled = false; $("timer").textContent = ""; }
        }
        function wait(sec) {
          try { localStorage.setItem(NEXT, String(Date.now() + sec * 1000)); } catch (e) {}
          tick();
        }
        $("make").onclick = function () {
          $("make").disabled = true;
          var pts = parseInt($("cardValue").value, 10) || 1000;
          App.call("POST", "cards/generate", { points: pts }, function (st, d) {
            if (st === 200) {
              var list = load(); list.unshift({ code: d.code, points: d.points || pts, at: Date.now() }); save(list);
              render(); $("msg").className = ""; // the new card itself is the result
              wait(d.cooldown || 15);
            } else {
              App.say(App.errText(d), true);
              wait(st === 429 ? Math.min(d.retryAfter || 15, 3600) : 5);
            }
          });
        };
        App.call("GET", "me", null, function (st, d) {
          linked = st === 200 && d.linked;
          if ($("linkhint")) $("linkhint").style.display = linked ? "none" : "block";
          render();
        });
        render(); tick();
        self.timer = setInterval(tick, 500);
        I18N.onChange = render;
      },
      stop: function () { clearInterval(this.timer); }
    },

    link: {
      title: "lk_title",
      start: function () {
        function show(linked) { $("form").style.display = linked ? "none" : "block"; $("done").style.display = linked ? "block" : "none"; }
        $("go").onclick = function () {
          $("go").disabled = true;
          App.call("POST", "link", { code: $("code").value }, function (st, d) {
            if (!$("go")) return;
            $("go").disabled = false;
            if (st === 200) { App.say(I18N.t("lk_ok")); $("code").value = ""; show(true); }
            else App.say(App.errText(d), true);
          });
        };
        $("code").onkeydown = function (e) { if (e.keyCode === 13) $("go").onclick(); };
        $("out").onclick = function () { App.call("POST", "unlink", {}, function () { $("msg").className = ""; if ($("form")) show(false); }); };
        App.call("GET", "me", null, function (st, d) { if ($("form")) show(st === 200 && d.linked); });
      }
    },

    friends: {
      title: "f_title",
      start: function (params) {
        var data = null;
        function btn(label, fn) { var b = document.createElement("button"); b.className = "sm"; b.textContent = label; b.onclick = fn; b.style.marginRight = "6px"; return b; }
        function giftRow(g, incoming) {
          var p = document.createElement("p");
          var s = document.createElement("b"); s.textContent = g.name; p.appendChild(s);
          p.appendChild(document.createTextNode(" (" + g.price + ") " + I18N.t(incoming ? "f_from" : "f_to") + " " + g.other + (g.message ? ' - "' + g.message + '" ' : " ")));
          p.appendChild(document.createElement("br"));
          var act = function (action) { return function () { App.call("POST", "gift/" + action, { id: g.id }, function (st, d) { if (st === 200) load(); else App.say(App.errText(d), true); }); }; };
          if (incoming) { p.appendChild(btn(I18N.t("f_accept"), act("accept"))); p.appendChild(btn(I18N.t("f_decline"), act("decline"))); }
          else p.appendChild(btn(I18N.t("f_cancel"), act("cancel")));
          return p;
        }
        function empty(el, key) { var e = document.createElement("p"); e.className = "muted"; e.textContent = I18N.t(key); el.appendChild(e); }
        function render() {
          if (!data || !$("code")) return;
          $("code").textContent = data.code;
          $("tier").textContent = data.tier || I18N.t("f_none");
          $("refs").textContent = data.referrals;
          $("points").textContent = data.points;
          $("redeemBox").style.display = data.referred ? "none" : "block";
          var inc = $("incoming"), out = $("outgoing"), up = $("updates");
          App.clear(inc); App.clear(out); App.clear(up);
          if (!data.incoming.length) empty(inc, "f_nogifts");
          data.incoming.forEach(function (g) { inc.appendChild(giftRow(g, true)); });
          if (!data.outgoing.length) empty(out, "f_nogifts");
          data.outgoing.forEach(function (g) { out.appendChild(giftRow(g, false)); });
          if (!data.updates.length) empty(up, "f_noupdates");
          data.updates.forEach(function (u) {
            var p = document.createElement("p"); p.appendChild(document.createTextNode(u.name + " (v" + u.installed + " → v" + u.latest + ") "));
            var a = document.createElement("a"); a.href = App.base + "/download/" + encodeURIComponent(u.slug); a.textContent = I18N.t("f_update_btn"); p.appendChild(a); up.appendChild(p);
          });
        }
        function load() {
          App.call("GET", "me", null, function (st, d) {
            if (!$("main")) return;
            if (st === 200 && d.linked) { $("needlink").style.display = "none"; $("main").style.display = "block"; data = d; render(); }
            else { $("main").style.display = "none"; $("needlink").style.display = "block"; }
          });
        }
        function fillApps() {
          var sel = $("gapp"); if (!sel) return; App.clear(sel);
          var o = document.createElement("option"); o.value = ""; o.textContent = I18N.t("f_choose"); sel.appendChild(o);
          var list = (window.gameslist || []).filter(function (g) { return g.id && g.link && g.link.indexOf("/download/") === 0; }).sort(function (a, b) { return a.title1.localeCompare(b.title1); });
          list.forEach(function (g) { var op = document.createElement("option"); op.value = g.link.slice(10); op.textContent = g.title1; sel.appendChild(op); });
          if (params.gift) sel.value = params.gift;
        }
        $("copy").onclick = function () { App.copyText($("code").textContent, function () { $("copy").textContent = I18N.t("f_copied"); setTimeout(function () { if ($("copy")) $("copy").textContent = I18N.t("f_copy"); }, 1500); }); };
        $("redeem").onclick = function () {
          App.call("POST", "referral", { code: $("rcode").value }, function (st, d) {
            if (st === 200) { App.say(I18N.t("f_done", { n: d.reward })); load(); } else App.say(App.errText(d), true);
          });
        };
        $("gsend").onclick = function () {
          App.call("POST", "gift/send", { to: $("gcode").value, slug: $("gapp").value, message: $("gmsg").value }, function (st, d) {
            if (st === 200) { App.say(I18N.t("f_sent")); $("gmsg").value = ""; load(); } else App.say(App.errText(d), true);
          });
        };
        fillApps(); load();
        withGames(fillApps);
        I18N.onChange = function () { fillApps(); render(); };
      }
    },

    history: {
      title: "h_title",
      start: function () {
        var hist = [];
        function label(r) { var s = I18N.t("h_" + r.reason); return s === "h_" + r.reason ? r.reason : s; }
        function render() {
          var body = $("rows"); if (!body) return; App.clear(body);
          $("empty").style.display = hist.length ? "none" : "block";
          hist.forEach(function (r) {
            var tr = document.createElement("tr"), a = document.createElement("td"), b = document.createElement("td"), c = document.createElement("td"), d = document.createElement("td");
            a.textContent = new Date(r.at).toLocaleString();
            b.textContent = label(r) + (r.ref && r.reason !== "card" && r.reason !== "referral_bonus" ? " (" + r.ref + ")" : "");
            c.textContent = (r.delta > 0 ? "+" : "") + r.delta; c.className = r.delta > 0 ? "ok" : "bad";
            d.textContent = r.balance;
            tr.appendChild(a); tr.appendChild(b); tr.appendChild(c); tr.appendChild(d); body.appendChild(tr);
          });
        }
        App.needLink(function (me) {
          if (!$("main")) return;
          $("main").style.display = "block"; $("points").textContent = me.points;
          App.call("GET", "history", null, function (st, d) { if (st === 200) { hist = d.history || []; render(); } else App.say(App.errText(d), true); });
        }, function () { if ($("needlink")) $("needlink").style.display = "block"; });
        I18N.onChange = render;
      }
    },

    titles: {
      title: "t_title",
      start: function () {
        var G = [], LIMIT = 300, gs = $("genre"), LICENSED = { "Virtual Console": 1, WiiWare: 1, DLC: 1, Channels: 1 };
        function norm(s) { return String(s || "").toLowerCase().replace(/[^a-z0-9]/g, ""); }
        function mb(b) { return b ? (b / 1048576 < 1 ? (b / 1024).toFixed(0) + " KB" : (b / 1048576).toFixed(1) + " MB") : ""; }
        function slugOf(g) { return g.link && g.link.indexOf("/download/") === 0 ? g.link.slice(10) : ""; }
        // Licensed titles carry their shop section in genre and the real genre in
        // displayGenre; homebrew uses genre for its own category (games, utilities...).
        function sectionOf(g) { return LICENSED[g.genre] ? g.genre : "Homebrew"; }
        function genreOf(g) { var d = LICENSED[g.genre] ? g.displayGenre : g.genre; return d ? d.charAt(0).toUpperCase() + d.slice(1) : ""; }
        // "Puzzle, Adventure" counts as both genres.
        function genresOf(g) { return genreOf(g).split(/\s*[,\/]\s*/); }
        function fillGenres() {
          var se = $("section").value, keep = gs.value, seen = {}, names = [];
          while (gs.options.length > 1) gs.removeChild(gs.options[1]);
          G.forEach(function (g) {
            if (se && sectionOf(g) !== se) return;
            genresOf(g).forEach(function (n) { if (n && !seen[n]) { seen[n] = 1; names.push(n); } });
          });
          names.sort().forEach(function (v) { var e = document.createElement("option"); e.value = v; e.textContent = v; gs.appendChild(e); });
          gs.value = seen[keep] ? keep : "";
        }
        function render() {
          if (!$("q")) return;
          var q = norm($("q").value), se = $("section").value, ge = gs.value, s = $("sort").value;
          var L = G.filter(function (g) {
            if (se && sectionOf(g) !== se) return false;
            if (ge && genresOf(g).indexOf(ge) < 0) return false;
            return !q || norm(g.title1 + g.title2 + g.publisher + g.console + genreOf(g) + g.description).indexOf(q) > -1;
          });
          L.sort(function (a, b) {
            if (s === "new") return (b.releaseUnix || 0) - (a.releaseUnix || 0);
            if (s === "old") return (a.releaseUnix || 0) - (b.releaseUnix || 0);
            if (s === "small") return (a.size || 0) - (b.size || 0);
            if (s === "big") return (b.size || 0) - (a.size || 0);
            return a.title1.localeCompare(b.title1);
          });
          var tb = document.querySelector("#t tbody"); tb.innerHTML = "";
          L.slice(0, LIMIT).forEach(function (g) {
            var tr = document.createElement("tr");
            [g.title1, g.console, genreOf(g), g.publisher, g.points == null ? "" : String(g.points), g.date, mb(g.size)].forEach(function (v) { var td = document.createElement("td"); td.textContent = v || ""; tr.appendChild(td); });
            var td = document.createElement("td");
            if (g.link) {
              var a = document.createElement("a"); a.href = (g.link.charAt(0) === "/" ? App.base : "") + g.link; a.textContent = I18N.t("t_download"); td.appendChild(a);
              var sl = slugOf(g);
              if (sl) { td.appendChild(document.createTextNode(" | ")); var b = document.createElement("a"); b.href = "#friends&gift=" + encodeURIComponent(sl); b.textContent = I18N.t("t_gift"); td.appendChild(b); }
            }
            tr.appendChild(td); tb.appendChild(tr);
          });
          $("count").textContent = I18N.t("t_count", { n: L.length });
          $("more").textContent = L.length > LIMIT ? I18N.t("t_more", { n: LIMIT }) : "";
        }
        $("q").addEventListener("input", render);
        $("section").addEventListener("change", function () { fillGenres(); render(); });
        gs.addEventListener("change", render);
        $("sort").addEventListener("change", render);
        $("q").focus();
        withGames(function () {
          if (!$("genre")) return;
          G = window.gameslist.filter(function (g) { return g.id; });
          fillGenres();
          render();
        });
        I18N.onChange = render;
      }
    },

    support: {
      title: "s_title",
      start: function () {
        var SPECIFIC = { 204034: 1, 204036: 1, 204058: 1, 201022: 1, 205100: 1, 205502: 1, 205151: 1, 205958: 1, 209600: 1, 209601: 1, 209557: 1, 109139: 1, 51330: 1, 51331: 1 };
        var last = null;
        function explain(c) {
          var n = parseInt(c, 10);
          if (SPECIFIC[n]) return ["bad", I18N.t("e_" + n)];
          if (n >= 204700 && n <= 204800) return ["bad", I18N.t("e_http", { n: n - 204300 })];
          if ([209622, 209569, 209531, 209577].indexOf(n) > -1) return ["bad", I18N.t("e_server")];
          if (c.length === 5 && c.charAt(0) === "5") return ["bad", I18N.t("e_conn")];
          if (n >= 205000 && n <= 205999) return ["bad", I18N.t("e_backend")];
          return ["muted", I18N.t("e_unknown")];
        }
        function show() {
          if (last === null || !$("res")) return;
          var e = explain(last), b = document.createElement("b"), s = document.createElement("span");
          b.textContent = last; s.className = e[0]; s.textContent = e[1];
          var r = $("res"); r.innerHTML = ""; r.appendChild(b); r.appendChild(document.createTextNode(" - ")); r.appendChild(s);
        }
        $("go").onclick = function () { var c = $("code").value.replace(/\D/g, ""); if (!c) { last = null; $("res").innerHTML = ""; return; } last = c; show(); };
        $("code").onkeydown = function (e) { if (e.keyCode === 13) $("go").onclick(); };
        $("clean").onclick = function () {
          var v = $("card").value.replace(/[^A-Za-z0-9]/g, "");
          $("cardout").textContent = v.length === 16 ? v : I18N.t("s_card_err", { n: v.length });
        };
        I18N.onChange = show;
      }
    },

    status: {
      title: "st_title",
      // Checks the shop server every 10 seconds. #status&down checks an address
      // nothing answers on, so the "offline" view can be tried safely.
      start: function (params) {
        var self = this, state = null, pending = null, sim = !!params.down;
        var EN = { st_noresp: "No response from the server.", st_every: "Checked again every 10 seconds.", st_sim: "Simulated outage: the page is checking a server address that cannot answer.", st_sim_on: "Simulate an outage", st_sim_off: "Stop the simulation" };
        function T(k, v) { var s = I18N.t(k, v); return (s === k && EN[k]) ? EN[k] : s; }
        var origin = App.base || location.protocol + "//" + location.hostname;
        function show() {
          if (!$("s")) return;
          $("every").textContent = T("st_every");
          $("simnote").textContent = T("st_sim");
          $("simnote").style.display = sim ? "" : "none";
          $("simlink").textContent = T(sim ? "st_sim_off" : "st_sim_on");
          $("simlink").setAttribute("href", sim ? "#status" : "#status&down");
          if (!state) { $("s").textContent = T("st_checking"); return; }
          $("s").innerHTML = state.ok ? "<span class='ok'>" + T("st_online") + "</span>" : "<span class='bad'>" + T("st_offline") + "</span>";
          $("d").textContent = state.ok ? T("st_resp", { ms: state.ms, time: state.time }) : (state.code ? T("st_fail", { n: state.code }) : T("st_noresp")) + " " + state.time;
        }
        function check() {
          if (pending) pending.abort();
          var x = new XMLHttpRequest(), t0 = new Date().getTime();
          pending = x;
          x.open("GET", (sim ? origin.replace(/^(https?:\/\/[^\/:]+).*$/, "$1:9") : origin) + "/healthz?t=" + t0, true);
          x.timeout = 8000;
          x.onreadystatechange = function () {
            if (x.readyState !== 4 || pending !== x) return;
            pending = null;
            state = { ok: x.status === 200, code: x.status, ms: new Date().getTime() - t0, time: new Date().toLocaleTimeString() };
            show();
          };
          x.send();
        }
        self.visible = function () { if (!document.hidden) check(); };
        document.addEventListener("visibilitychange", self.visible);
        self.timer = setInterval(function () { if (!document.hidden) check(); }, 10000);
        self.abort = function () { if (pending) pending.abort(); pending = null; };
        show(); check();
        I18N.onChange = show;
      },
      stop: function () {
        clearInterval(this.timer);
        document.removeEventListener("visibilitychange", this.visible);
        this.abort();
      }
    }
  };

  var current = null;
  function route() {
    var parts = location.hash.replace(/^#/, "").split("&"), name = parts[0], params = {};
    for (var i = 1; i < parts.length; i++) {
      var kv = parts[i].split("=");
      params[decodeURIComponent(kv[0])] = kv.length > 1 ? decodeURIComponent(kv[1]) : "1";
    }
    // Unknown names (or a bare #fxdate=... preview) show the start page.
    if (!PAGES[name] || !$("p-" + name)) name = "index";
    if (current && current.stop) current.stop();
    I18N.onChange = null;
    current = PAGES[name];
    var view = $("view");
    App.clear(view);
    view.appendChild(document.importNode($("p-" + name).content, true));
    $("nav").setAttribute("data-page", name);
    $("title").setAttribute("data-i18n", current.title);
    $("msg").className = "";
    I18N.refresh();
    document.title = name === "index" ? "Shroombrew" : "Shroombrew - " + I18N.t(current.title);
    if (current.start) current.start(params);
    window.scrollTo(0, 0);
  }
  // A language change re-titles the tab too.
  window.addEventListener("hashchange", route);
  function boot() { I18N.ready(route); }
  if (document.readyState === "loading") document.addEventListener("DOMContentLoaded", boot); else boot();
})();
