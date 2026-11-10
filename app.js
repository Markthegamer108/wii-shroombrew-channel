/* Shared helpers for the website pages that talk to the server. */
(function () {
  // The server's address. Empty when these pages are served by the server itself;
  // the GitHub Pages build fills it in. There the session cannot be a cookie
  // (browsers block them across sites), so it is kept here and sent as a header.
  var BASE = "https://shop.shroombrew.org";
  var KEY = "shroombrew_session";
  function token() { try { return BASE ? localStorage.getItem(KEY) : null; } catch (e) { return null; } }
  function keep(t) { try { if (t) localStorage.setItem(KEY, t); else localStorage.removeItem(KEY); } catch (e) {} }

  function call(method, path, body, cb) {
    var x = new XMLHttpRequest();
    x.open(method, BASE + "/community/api/" + path, true);
    x.setRequestHeader("X-Shroombrew", "1");
    if (token()) x.setRequestHeader("Authorization", "Bearer " + token());
    if (body) x.setRequestHeader("Content-Type", "application/json");
    x.onreadystatechange = function () {
      if (x.readyState !== 4) return;
      var d = {};
      try { d = JSON.parse(x.responseText); } catch (e) {}
      if (BASE && path === "link" && x.status === 200 && d.token) { keep(d.token); delete d.token; }
      if (BASE && (path === "unlink" || x.status === 401)) keep(null);
      cb(x.status, d);
    };
    x.send(body ? JSON.stringify(body) : null);
  }

  // The server sends a translation key with every error; show it in the
  // visitor's language and fall back to the English sentence.
  function errText(d) {
    if (d && d.key) {
      var s = I18N.t(d.key, { n: d.max || d.retryAfter || "", m: Math.ceil((d.retryAfter || 60) / 60) });
      if (s !== d.key) return s;
    }
    return (d && d.error) || I18N.t("er_generic");
  }

  function say(text, bad) {
    var m = document.getElementById("msg");
    if (!m) return;
    m.textContent = text;
    m.className = "box " + (bad ? "bad" : "ok");
    window.scrollTo(0, 0);
  }

  function clear(el) { while (el.firstChild) el.removeChild(el.firstChild); }

  function copyText(text, done) {
    if (navigator.clipboard && navigator.clipboard.writeText) {
      navigator.clipboard.writeText(text).then(done, function () {});
      return;
    }
    var ta = document.createElement("textarea");
    ta.value = text; ta.style.position = "fixed"; ta.style.opacity = "0";
    document.body.appendChild(ta); ta.select();
    try { document.execCommand("copy"); done(); } catch (e) {}
    document.body.removeChild(ta);
  }

  // Resolves the linked-Wii state once per page; pages that need a link show
  // a prompt instead of their content when it is missing.
  function needLink(onLinked, onUnlinked) {
    call("GET", "me", null, function (st, d) {
      if (st === 200 && d.linked) onLinked(d); else onUnlinked();
    });
  }

  window.App = { base: BASE, call: call, errText: errText, say: say, clear: clear, copyText: copyText, needLink: needLink };
})();
