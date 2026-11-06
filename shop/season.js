/* Shroombrew seasonal music. The server only sends this file between 1 and 26
   December; the date is checked again here. The tune is "Jingle Bells" (public
   domain), synthesized with Web Audio - there is no audio file to host. */
(function () {
  var d = new Date();
  if (d.getMonth() !== 11 || d.getDate() > 26) return;
  var AC = window.AudioContext || window.webkitAudioContext;
  if (!AC || window.__shroombrewSeason) return;
  window.__shroombrewSeason = true;

  var N = { C4: 261.63, D4: 293.66, E4: 329.63, F4: 349.23, G4: 392.0, A4: 440.0, B4: 493.88, C5: 523.25 };
  // [note, beats]
  var tune = [
    ["E4", 1], ["E4", 1], ["E4", 2], ["E4", 1], ["E4", 1], ["E4", 2],
    ["E4", 1], ["G4", 1], ["C4", 1.5], ["D4", 0.5], ["E4", 4],
    ["F4", 1], ["F4", 1], ["F4", 1.5], ["F4", 0.5], ["F4", 1], ["E4", 1], ["E4", 1], ["E4", 0.5], ["E4", 0.5],
    ["E4", 1], ["D4", 1], ["D4", 1], ["E4", 1], ["D4", 2], ["G4", 2],
    ["E4", 1], ["E4", 1], ["E4", 2], ["E4", 1], ["E4", 1], ["E4", 2],
    ["E4", 1], ["G4", 1], ["C4", 1.5], ["D4", 0.5], ["E4", 4],
    ["F4", 1], ["F4", 1], ["F4", 1.5], ["F4", 0.5], ["F4", 1], ["E4", 1], ["E4", 1], ["E4", 0.5], ["E4", 0.5],
    ["G4", 1], ["G4", 1], ["F4", 1], ["D4", 1], ["C4", 4]
  ];

  var ctx = null, playing = false, timer = null, btn;
  var beat = 0.36;

  function note(freq, when, dur) {
    var o = ctx.createOscillator(), g = ctx.createGain();
    o.type = "triangle";
    o.frequency.value = freq;
    g.gain.setValueAtTime(0.0001, when);
    g.gain.exponentialRampToValueAtTime(0.07, when + 0.02);
    g.gain.exponentialRampToValueAtTime(0.0001, when + dur);
    o.connect(g); g.connect(ctx.destination);
    o.start(when); o.stop(when + dur + 0.05);
  }

  function loop() {
    if (!playing) return;
    var t = ctx.currentTime + 0.1, total = 0, i;
    for (i = 0; i < tune.length; i++) {
      note(N[tune[i][0]], t + total * beat, tune[i][1] * beat * 0.9);
      total += tune[i][1];
    }
    timer = setTimeout(loop, (total * beat + 3) * 1000); // rest 3s between plays
  }

  function set(on) {
    if (on && !ctx) ctx = new AC();
    if (on && ctx.state === "suspended" && ctx.resume) ctx.resume();
    playing = on;
    clearTimeout(timer);
    if (on) loop();
    btn.firstChild.nodeValue = on ? "♫ Christmas music: on" : "♫ Christmas music: off";
    try { localStorage.setItem("shroombrewSeason", on ? "1" : "0"); } catch (e) {}
  }

  function init() {
    btn = document.createElement("button");
    btn.appendChild(document.createTextNode("♫ Christmas music: off"));
    btn.style.cssText = "position:fixed;right:10px;bottom:10px;z-index:99999;padding:6px 12px;border:0;border-radius:6px;background:#c0392b;color:#fff;font:bold 13px Arial;cursor:pointer;opacity:.9";
    btn.onclick = function () { set(!playing); };
    document.body.appendChild(btn);
    // Browsers block sound until a click, so a previous "on" resumes on the first click.
    var saved = null;
    try { saved = localStorage.getItem("shroombrewSeason"); } catch (e) {}
    if (saved === "1") {
      var first = function (e) {
        document.removeEventListener("click", first, true);
        if (e.target === btn) return; // the button's own handler toggles
        if (!playing) set(true);
      };
      document.addEventListener("click", first, true);
    }
  }

  if (document.body) init(); else document.addEventListener("DOMContentLoaded", init);
})();
