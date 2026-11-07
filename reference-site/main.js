// Dev Settings (can only be changed via console)
var trace = false; // Whether the console prints calls to the trace command. The trace command was part of the Wii Shop's code, and as far as I know, it was just for debug purposes, and was basically just console.log.
var sndlogs = false; // Whether the console prints every sound effect that plays.
var showfps = false; // Shows the Frames Per Second.


// Yummy Cookie Stuff
function setValue(cname, cvalue) {
  /*const d = new Date();
  d.setTime(d.getTime() + (exdays * 24 * 60 * 60 * 1000));
  let expires = "expires="+d.toUTCString();
  document.cookie = cname + "=" + cvalue + ";" + expires + ";path=/;SameSite=Strict";*/
  localStorage.setItem(cname, String(cvalue));
}

function getValue(cname) {
  /*let name = cname + "=";
  let ca = document.cookie.split(';');
  for(let i = 0; i < ca.length; i++) {
    let c = ca[i];
    while (c.charAt(0) == ' ') {
      c = c.substring(1);
    }
    if (c.indexOf(name) == 0) {
      return c.substring(name.length, c.length);
    }
  }
  return "";*/
  return localStorage.getItem(cname);
}


// Default Settings
var musicvol = 25;
var soundvol = 25;
var oneToOne = false;
var cursor = false;
var cursoranim = true;
var hdmode = false;
var ownedgames = [];
var region = "US";
var showallgames = false;
var hidelinkedgames = false;
var points = 0;

if (getValue("musicvol") == null) {
  setValue("musicvol", musicvol);
} else {
  musicvol = parseInt(getValue("musicvol"));
}

if (getValue("soundvol") == null) {
  setValue("soundvol", soundvol);
} else {
  soundvol = parseInt(getValue("soundvol"));
}

if (getValue("oneToOne") == null) {
  setValue("oneToOne", oneToOne);
} else {
  oneToOne = (getValue("oneToOne") === "true");
}

if (getValue("cursor") == null) {
  setValue("cursor", cursor);
} else {
  cursor = (getValue("cursor") === "true");
}

if (getValue("cursoranim") == null) {
  setValue("cursoranim", cursoranim);
} else {
  cursoranim = (getValue("cursoranim") === "true");
}

if (getValue("region") == null) {
  setValue("region", region);
} else {
  region = getValue("region");
}

if (getValue("showallgames") == null) {
  setValue("showallgames", showallgames);
} else {
  showallgames = (getValue("showallgames") === "true");
}

if (getValue("hidelinkedgames") == null) {
  setValue("hidelinkedgames", hidelinkedgames);
} else {
  hidelinkedgames = (getValue("hidelinkedgames") === "true");
}

if (getValue("points") == null) {
  setValue("points", points);
} else {
  points = parseInt(getValue("points"));
}


/*if (getValue("musicvol") == null || getValue("soundvol") == null || getValue("oneToOne") == null || getValue("cursor") == null || getValue("cursoranim") == null || getValue("region") == null || getValue("showallgames") == null) {
  //document.getElementById("note").innerHTML = "This website uses cookies to save settings.<br>This message will only appear once.<br>Click the ring to continue.";
  setValue("musicon", true);
  setValue("soundon", true);
  setValue("musicvol", musicvol);
  setValue("soundvol", soundvol);
  setValue("oneToOne", oneToOne);
  setValue("cursor", cursor);
  setValue("cursoranim", cursoranim);
  setValue("region", region);
  setValue("showallgames", showallgames);
  setValue("hidelinkedgames", hidelinkedgames);
} else {
  musicvol = parseInt(getValue("musicvol"));
  soundvol = parseInt(getValue("soundvol"));
  oneToOne = (getValue("oneToOne") === "true");
  cursor = (getValue("cursor") === "true");
  cursoranim = (getValue("cursoranim") === "true");
  region = getValue("region");
  showallgames = (getValue("showallgames") === "true");
  hidelinkedgames = (getValue("hidelinkedgames") === "true");
}*/

// Declares a bunch of variables.
var imgloader = new Image();
imgloader.src = "img/ringgray.png"; // I should probably have it load some other stuff.
var loadsnd = new Audio('snd/load.mp3');
var now = new Date();
if (now.getMonth() == 3 && now.getDate() == 1) {
  document.title = "Wiizer Shop Channel";
  var theme1 = new Audio('snd/theme1.mp3');
  var theme2 = new Audio('snd/theme2fool.mp3');
} else if (now.getMonth() == 11) {
  // December: the Wii Shop Christmas mashup plays all month. One looping
  // track instead of the usual intro/loop pair, so the whole song plays.
  var theme1 = new Audio('snd/theme1christmas.mp3');
  theme1.loop = true;
  var theme2 = theme1;
} else if (now.getMonth() == 5 && now.getDate() == 26) {
  // June 26: "Attentiion" (Wii Shop x Charlie Puth) upload anniversary.
  var theme1 = new Audio('snd/theme1attention.mp3');
  theme1.loop = true;
  var theme2 = theme1;
} else if(Math.random() < 0.01) {
  if (now.getDay() == 3) {
    var theme1 = new Audio('snd/theme1.mp3');
    var theme2 = new Audio('snd/theme2nirvanna.mp3');
  } else {
    var theme1 = new Audio('snd/theme1bossa.mp3');
    var theme2 = new Audio('snd/theme2bossa.mp3');
  }
  console.log("Watch carefully, special event!");
} else {
  var theme1 = new Audio('snd/theme1.mp3');
  var theme2 = new Audio('snd/theme2.mp3');
}

var kbd_open = new Audio("snd/kbd_open.wav");
var kbd_hover = new Audio("snd/kbd_hover.wav");
var kbd_key = new Audio("snd/kbd_key.wav");
var kbd_space = new Audio("snd/kbd_space.wav");
var kbd_backspace = new Audio("snd/kbd_backspace.wav");
var kbd_nobackspace = new Audio("snd/kbd_nobackspace.wav");
var kbd_caps = new Audio("snd/kbd_caps.wav");
var kbd_denied = new Audio("snd/kbd_denied.wav");
var kbd_okhover = new Audio("snd/kbd_okhover.wav");
var kbd_ok = new Audio("snd/kbd_ok.wav");
var soundlist = [document.getElementById("cSE_Forcus"), document.getElementById("cSE_Decide"), document.getElementById("cSE_Cancel"), document.getElementById("cSE_Finish"), loadsnd, kbd_open, kbd_hover, kbd_key, kbd_space, kbd_backspace, kbd_denied, kbd_okhover, kbd_ok];
var introplayed = false;
var loading = true;
// Browsers block audio before the first user interaction, so the BGM started by
// the landing auto-enter (not a click) is paused. Resume it on first interaction.
var shroomResumed = false;
function shroomResumeAudio() {
  if (shroomResumed) { return; }
  shroomResumed = true;
  try {
    if (introplayed) {
      theme2.play();
    } else {
      theme1.play();
    }
    if (loading) {
      var p = loadsnd.play();
      if (p !== undefined && p.catch) { p.catch(function(){}); }
    }
  } catch (e) {}
}
document.addEventListener('pointerdown', shroomResumeAudio, true);
document.addEventListener('touchstart', shroomResumeAudio, true);
document.addEventListener('keydown', shroomResumeAudio, true);
var ring1 = document.getElementById("load1");
var ring2 = document.getElementById("load2");
var hist = [];
var fade = false;
var fadeframe = 0;
var prevcursor = !cursor;
var cursorrot = 0;
var prevx = 0; // Previous X position of the cursor as a percent of the screen width.
var keyshown = false;
var inputelement = null;
var caps = false;
var shift = false;
var iframeone = document.getElementById("frame");
var iframetwo = document.getElementById("otherframe");
iframeone.contentWindow.location.href = "https://wiishopchannel.net/oss/serv/index.html";
iframetwo.contentWindow.location.href = "https://wiishopchannel.net/oss/serv/index.html";
var currentiframe = iframeone;
var otheriframe = iframetwo;

// Calls when the page starts loading.
var content_start_loading = function() {
  loading = true;
  document.getElementById("note").style.display = "none";
  ring2.src = "img/ringgray.png";
  ring1.style = "top: 17.5px; left: 17.5px; width: 75px; height: 75px; opacity: 0.1; z-index: 3;";
  ring2.style = "top: 10px; left: 10px; width: 75px; height: 75px; z-index: 4;";
  ring1.style.display = 'block';
  ring2.style.display = 'block';
  document.getElementById("link").removeAttribute("href");
  document.getElementById("cursorfield").style.zIndex = 999;
  loadsnd.pause();
  loadsnd.currentTime = 0;
};

// Calls when the page is finished loading.
var content_finished_loading = function(iframe) {
  if (iframe.contentWindow.location.href.includes("wiishopchannel.net/oss/serv/W_03")) {
    hist = [];
  } else if (iframe.contentWindow.location.href.includes("wiishopchannel.net/oss/serv/S_01")) {
    setValue("musicvol", musicvol);
    setValue("soundvol", soundvol);
    setValue("oneToOne", oneToOne);
    setValue("cursor", cursor);
    setValue("cursoranim", cursoranim);
  }
  if (iframe.contentWindow.location.href.includes("wiishopchannel.net/oss/serv/B_13")) {
    document.getElementById("fadescreen").style.zIndex = 0;
    document.getElementById("fadescreen").style.opacity = 1;
  } else {
    document.getElementById("fadescreen").style.zIndex = -1002;
    document.getElementById("fadescreen").style.opacity = 0;
  }
  if (hist.length == 0 || !(hist[hist.length - 1].startsWith(iframe.contentWindow.location.href.split("?")[0]))) {
    hist.push(iframe.contentWindow.location.href);
  }
  if (hist[hist.length - 1].startsWith(iframe.contentWindow.location.href.split("?")[0])) {
    hist[hist.length - 1] = iframe.contentWindow.location.href;
  }
  if (iframe.contentWindow.location.href.endsWith("wiishopchannel.net/oss/serv/index.html") || iframe.contentWindow.location.href.endsWith("wiishopchannel.net/oss/serv/")) {
    if (iframe == currentiframe) {
      // Shroombrew landing: show the original channel's "Connecting. Please wait..."
      // message (no ring) and auto-enter the shop once the page has rendered.
      ring1.style.display = 'none';
      ring2.style.display = 'none';
      var queryString = window.location.search.substr(1);
      var gotolink = "";
      if (queryString == "") {
        gotolink = "W_01.html";
      } else {
        gotolink = "B_05.html?titleId=" + queryString;
      }
      document.getElementById("link").removeAttribute("href");
      var landingMsg = iframe.contentWindow.document.getElementById("wiiMessage");
      if (landingMsg) { landingMsg.innerHTML = "Connecting. Please wait..."; }
      loading = true;
      introplayed = false;
      theme1.pause();
      theme1.currentTime = 0;
      theme2.pause();
      theme2.currentTime = 0;
      // Auto-enter the shop once the landing page has rendered.
      setTimeout(function() { goto(gotolink); }, 900);
    }
  } else {
    if (iframe.contentWindow.location.href.includes("wiishopchannel.net/oss/serv/W_01")) {
      document.getElementById("dots").style.display = "none";
    } else {
      document.getElementById("dots").style.display = "block";
    }
    if (!iframe.contentWindow.location.href.includes("wiishopchannel.net/oss/serv/B_10")) {
      ring1.style.display = 'none';
      ring2.style.display = 'none';
      loading = false;
      loadsnd.pause();
      loadsnd.currentTime = 0;
    }
    if (!introplayed) {
      var tp = theme1.play();
      if (tp !== undefined && tp.catch) { tp.catch(function(){}); }
    }
  }
  iframe.contentWindow.onmousemove = framecursormove;
  document.getElementById("cursorfield").style.zIndex = -999;
  iframe.style.zIndex = 2;
  otheriframe.style.zIndex = 1;
  iframe.contentWindow.onunload = content_start_loading;
};

// Opens a page. Can be used in the console.
// If the parameter isn't a direct link, it will assume the file is in https://wiishopchannel.net/oss/serv/
function goto(page) {
  if (currentiframe.id == "frame") {
    currentiframe = iframetwo;
    otheriframe = iframeone;
  } else {
    currentiframe = iframeone;
    otheriframe = iframetwo;
  }
  if (page.includes("wiishopchannel.net")) {
    currentiframe.contentWindow.location.href = page;
  } else {
    currentiframe.contentWindow.location.href = "https://wiishopchannel.net/oss/serv/" + page;
  }
}


// --------------
// KEYBOARD STUFF
// --------------

// Opens the keyboard.
// input: The element to put the user's text into.
// message: The prompt text to display.
function keyboard(input, message) {
  keyshown = true;
  inputelement = input;
  //document.getElementById("keyboard").style.transition = "top 0.5s cubic-bezier(.33,.75,.5,1) 0s, opacity 0.5s cubic-bezier(.33,.75,.5,1) 0s, z-index 0s ease 0s";
  document.getElementById("keyboard").style.transition = "top 0.5s ease-in-out 0s, opacity 0.5s ease-in-out 0s, z-index 0s ease 0s";
  document.getElementById("keyboard").style.opacity = 1;
  document.getElementById("keyboard").style.zIndex = 1000;
  document.getElementById("keygradient1").style.transition = "top 0.5s ease-in-out 0s, opacity 0.5s ease-in-out 0s, z-index 0s ease 0s";
  document.getElementById("keygradient1").style.opacity = 1;
  document.getElementById("keygradient1").style.zIndex = 997;
  document.getElementById("keygradient2").style.transition = "top 0.5s ease-in-out 0s, opacity 0.5s ease-in-out 0s, z-index 0s ease 0s";
  document.getElementById("keygradient2").style.opacity = 1;
  document.getElementById("keygradient2").style.zIndex = 999;
  document.getElementById("keygbg1").style.transition = "top 0.5s ease-in-out 0s, opacity 0.5s ease-in-out 0s, z-index 0s ease 0s";
  document.getElementById("keygbg1").style.opacity = 1;
  document.getElementById("keygbg1").style.zIndex = 996;
  document.getElementById("keygbg2").style.transition = "height 0.5s ease-in-out 0s, opacity 0.5s ease-in-out 0s, z-index 0s ease 0s";
  document.getElementById("keygbg2").style.opacity = 1;
  document.getElementById("keygbg2").style.zIndex = 998;
  document.getElementById("keybg").style.transition = "opacity 0.5s ease-in-out 0s, z-index 0s ease 0s";
  document.getElementById("keybg").style.opacity = 1;
  document.getElementById("keybg").style.zIndex = 995;
  document.getElementById("keyprompt").placeholder = message;
  document.getElementById("keyprompt").value = input.value;
  input.blur();
  document.getElementById("keyprompt").focus();
  kbd_open.currentTime = 0;
  kbd_open.play();
}

// Hides the keyboard.
// This doesn't do much on its own. Refer to confirm() or exit()
function hidekey() {
  keyshown = false;
  //document.getElementById("keyboard").style.transition = "top 0.5s cubic-bezier(.5,0,.67,.25) 0s, opacity 0.5s cubic-bezier(.5,0,.67,.25) 0s, z-index 0s ease 0.5s";
  document.getElementById("keyboard").style.transition = "top 0.5s ease-in-out 0s, opacity 0.5s ease-in-out 0s, z-index 0s ease 0.5s";
  document.getElementById("keyboard").style.opacity = -0.5;
  document.getElementById("keyboard").style.zIndex = -1000;
  document.getElementById("keygradient1").style.transition = "top 0.5s ease-in-out 0s, opacity 0.5s ease-in-out 0s, z-index 0s ease 0.5s";
  document.getElementById("keygradient1").style.opacity = -0.5;
  document.getElementById("keygradient1").style.zIndex = -997;
  document.getElementById("keygradient2").style.transition = "top 0.5s ease-in-out 0s, opacity 0.5s ease-in-out 0s, z-index 0s ease 0.5s";
  document.getElementById("keygradient2").style.opacity = -0.5;
  document.getElementById("keygradient2").style.zIndex = -999;
  document.getElementById("keygbg1").style.transition = "top 0.5s ease-in-out 0s, opacity 0.5s ease-in-out 0s, z-index 0s ease 0.5s";
  document.getElementById("keygbg1").style.opacity = -0.5;
  document.getElementById("keygbg1").style.zIndex = -996;
  document.getElementById("keygbg2").style.transition = "height 0.5s ease-in-out 0s, opacity 0.5s ease-in-out 0s, z-index 0s ease 0.5s";
  document.getElementById("keygbg2").style.opacity = -0.5;
  document.getElementById("keygbg2").style.zIndex = -998;
  document.getElementById("keybg").style.transition = "opacity 0.5s ease-in-out 0s, z-index 0s ease 0.5s";
  document.getElementById("keybg").style.opacity = -0.5;
  document.getElementById("keybg").style.zIndex = -995;
}

// Toggles the caps
// setcaps: Optional true/false. Whether the caps should be set on or off. If blank, it will toggle the current value.
function togglecaps(setcaps) {
  const letters = ["Q", "W", "E", "R", "T", "Y", "U", "I", "O", "P", "A", "S", "D", "F", "G", "H", "J", "K", "L", "Z", "X", "C", "V", "B", "N", "M"];
  if (setcaps == null) {
    caps = !caps;
  } else {
    caps = setcaps;
  }
  if (caps) {
    for (i = 0; i < letters.length; i++) {
      document.getElementById("Key" + letters[i]).getElementsByClassName("keytext")[0].innerHTML = letters[i];
    }
  } else {
    for (i = 0; i < letters.length; i++) {
      document.getElementById("Key" + letters[i]).getElementsByClassName("keytext")[0].innerHTML = letters[i].toLowerCase();
    }
  }
}

// Activates the hover effect of a given key.
// key: Element to give the effect to.
function hoverkey(key) {
  key.style.zIndex = 1;
  if (key.id == "Backspace" || key.id == "Space" || key.id == "CapsLock" || key.id == "ShiftLeft") {
    key.getElementsByClassName("keycolor")[0].style.backgroundColor = "#C1D0EF";
    key.style.transform = "scale(1.275)";
  } else {
    key.getElementsByClassName("keycolor")[0].style.backgroundColor = "#C8C8EB";
    key.style.transform = "scale(1.5)";
  }
  kbd_hover.currentTime = 0;
  kbd_hover.play();
}

// Sets a hovered key back to normal.
// key: Element to set back to normal.
function unhoverkey(key) {
  key.style.zIndex = 0;
  key.style.transform = "scale(1)";
  if (key.id == "Backspace" || key.id == "Space" || key.id == "CapsLock"  || key.id == "ShiftLeft") {
    key.getElementsByClassName("keycolor")[0].style.backgroundColor = "#B8ECFF";
  } else {
    key.getElementsByClassName("keycolor")[0].style.backgroundColor = "#FFFFFF";
  }
  kbd_hover.currentTime = 0;
  kbd_hover.play();
}

// Clicks a key
// key: Element to press.
function downkey(key) {
  key.style.zIndex = 1;
  key.getElementsByClassName("keycolor")[0].style.backgroundColor = "#CDD264";
  if (key.id == "Backspace") {
    key.style.transform = "scale(1.1)";
    if (document.getElementById("keyprompt").value.length == 0) {
      kbd_nobackspace.currentTime = 0;
      kbd_nobackspace.play();
    } else {
      document.getElementById("keyprompt").value = document.getElementById("keyprompt").value.slice(0, -1);
      kbd_backspace.currentTime = 0;
      kbd_backspace.play();
    }
  } else if (key.id == "Space") {
    key.style.transform = "scale(1.1)";
    document.getElementById("keyprompt").value += " ";
    kbd_space.currentTime = 0;
    kbd_space.play();
  } else if (key.id == "CapsLock") {
    key.style.transform = "scale(1.1)";
    togglecaps();
    kbd_caps.currentTime = 0;
    kbd_caps.play();
  } else if (key.id == "ShiftLeft") {
    key.style.transform = "scale(1.1)";
    togglecaps();
    shift = caps;
    kbd_caps.currentTime = 0;
    kbd_caps.play();
  } else {
    key.style.transform = "scale(1.4)";
    document.getElementById("keyprompt").value += key.getElementsByClassName("keytext")[0].innerHTML;
    if (shift) {
      shift = false;
      togglecaps(false);
    }
    kbd_key.currentTime = 0;
    kbd_key.play();
  }
  document.getElementById("keyprompt").focus();
}

// Unclicks a key.
// key: Element to unclick.
function upkey(key) {
  key.style.zIndex = 1;
  if (key.id == "Backspace" || key.id == "Space" || key.id == "CapsLock" || key.id == "ShiftLeft") {
    key.getElementsByClassName("keycolor")[0].style.backgroundColor = "#C1D0EF";
    key.style.transform = "scale(1.275)";
  } else {
    key.getElementsByClassName("keycolor")[0].style.backgroundColor = "#C8C8EB";
    key.style.transform = "scale(1.5)";
  }
  document.getElementById("keyprompt").focus();
}

// Activates the hover effect of a confirm/exit key.
// key: Element to give the effect to.
function hoverconfirm(key) {
  key.style.transform = "scale(1.1)";
  kbd_okhover.currentTime = 0;
  kbd_okhover.play();
}

// Sets a confirm/exit key back to normal.
// key: Element to set back to normal.
function unhoverconfirm(key) {
  key.style.transform = "scale(1)";
  kbd_okhover.currentTime = 0;
  kbd_okhover.play();
}

// Closes the keyboard, puts the user's text into the input element, and calls the onchange event of the element.
function confirm() {
  kbd_ok.currentTime = 0;
  kbd_ok.play();
  hidekey();
  if (!document.getElementById("keyprompt").value.replace(/\s/g, '').length) {
    inputelement.value = "";
  } else {
    inputelement.value = document.getElementById("keyprompt").value;
  }
  document.getElementById("keyprompt").value = "";
  inputelement.onchange();
}

// Closes the keyboard and clears the user's text.
function exit() {
  kbd_ok.currentTime = 0;
  kbd_ok.play();
  hidekey();
  document.getElementById("keyprompt").value = "";
}

// Calls when a key is pressed on the user's actual keyboard.
document.addEventListener('keydown', function(event) {
  if (keyshown) {
    if (event.code == "Enter") {
      confirm();
    } else if (event.code == "Escape") {
      exit();
    } else if (event.code == "Backspace" || event.code == "Space") {
      document.getElementById(event.code).style.zIndex = 1;
      document.getElementById(event.code).style.transform = "scale(1.1)";
      document.getElementById(event.code).getElementsByClassName("keycolor")[0].style.backgroundColor = "#CDD264";
      if (document.getElementById("keyprompt").value.length == 0) {
        kbd_nobackspace.currentTime = 0;
        kbd_nobackspace.play();
      } else {
        kbd_backspace.currentTime = 0;
        kbd_backspace.play();
      }
    } else if (event.code == "CapsLock") {
      //if (event.getModifierState('CapsLock')) {
        document.getElementById(event.code).style.zIndex = 1;
        document.getElementById(event.code).style.transform = "scale(1.1)";
        document.getElementById(event.code).getElementsByClassName("keycolor")[0].style.backgroundColor = "#CDD264";
        kbd_caps.currentTime = 0;
        kbd_caps.play();
      //} else {
      //  document.getElementById(event.code).style.zIndex = 0;
      //  document.getElementById(event.code).style.transform = "scale(1)";
      //  document.getElementById(event.code).getElementsByClassName("keycolor")[0].style.backgroundColor = "#BBDAE5";
      //}
      togglecaps(event.getModifierState('CapsLock'));
    } else if (event.code == "ShiftLeft") {
      document.getElementById(event.code).style.zIndex = 1;
      document.getElementById(event.code).style.transform = "scale(1.1)";
      document.getElementById(event.code).getElementsByClassName("keycolor")[0].style.backgroundColor = "#CDD264";
      togglecaps(!event.getModifierState('CapsLock'));
      shift = caps;
      kbd_caps.currentTime = 0;
      kbd_caps.play();
    } else if (event.code == "Digit2" && event.shiftKey) {
      document.getElementById("AtSymbol").style.zIndex = 1;
      document.getElementById("AtSymbol").style.transform = "scale(1.4)";
      document.getElementById("AtSymbol").getElementsByClassName("keycolor")[0].style.backgroundColor = "#CDD264";
      kbd_key.currentTime = 0;
      kbd_key.play();
    } else {
      try {
        document.getElementById(event.code).style.zIndex = 1;
        document.getElementById(event.code).style.transform = "scale(1.4)";
        document.getElementById(event.code).getElementsByClassName("keycolor")[0].style.backgroundColor = "#CDD264";
        kbd_key.currentTime = 0;
        kbd_key.play();
      } catch {
        // Error. Nothing happens.
      }
    }
  }
});

// Calls when a key is released on the user's actual keyboard.
document.addEventListener('keyup', function(event) {
  if (event.code == "Digit2") {
    document.getElementById("AtSymbol").style.zIndex = 0;
    document.getElementById("AtSymbol").style.transform = "scale(1)";
    document.getElementById("AtSymbol").getElementsByClassName("keycolor")[0].style.backgroundColor = "#FFFFFF";
  }
  if (event.code != "Enter" && event.code != "Escape") {
    try {
      document.getElementById(event.code).style.zIndex = 0;
      document.getElementById(event.code).style.transform = "scale(1)";
      if (document.getElementById(event.code).id == "ShiftLeft") {
        document.getElementById(event.code).getElementsByClassName("keycolor")[0].style.backgroundColor = "#B8ECFF";
        togglecaps(event.getModifierState('CapsLock'));
        shift = caps;
      } else if (document.getElementById(event.code).id == "Backspace" || document.getElementById(event.code).id == "Space" || document.getElementById(event.code).id == "CapsLock") {
        document.getElementById(event.code).getElementsByClassName("keycolor")[0].style.backgroundColor = "#B8ECFF";
      } else {
        document.getElementById(event.code).getElementsByClassName("keycolor")[0].style.backgroundColor = "#FFFFFF";
      }
    } catch {
      // Error. Nothing happens.
    }
  }
});

// End of keyboard stuff.


// Calls every time the mouse moves when outside the iframe.
window.onmousemove = cursormove;
function cursormove(event) {
  if (cursor) {
    document.getElementById("cursor").style.top = event.clientY - 2 + "px";
    document.getElementById("cursor").style.left = event.clientX - 17 + "px";
    if (cursoranim) {
      cursorrot = Math.max(Math.min(cursorrot + Math.min(((event.clientX / (608 * Math.min(window.innerWidth / 608, window.innerHeight / 456))) - prevx) * 304, 10), 45), -45);
      prevx = event.clientX / (608 * Math.min(window.innerWidth / 608, window.innerHeight / 456));
    }
  }
}

// Calls every time the mouse moves when inside the iframe.
// This is the same as the function above, but it compensates for the fact that event.clientX/Y begins at the top left of the frame.
// Unfortunately, this means one of these functions must be made for every page with an iframe.
currentiframe.contentWindow.onmousemove = framecursormove;
function framecursormove(event) {
  if (cursor) {
    document.getElementById("cursor").style.top = event.clientY * Math.min(window.innerWidth / 608, window.innerHeight / 456) + (window.innerHeight - 456 * Math.min(window.innerWidth / 608, window.innerHeight / 456)) / 2 - 2 + "px";
    document.getElementById("cursor").style.left = event.clientX * Math.min(window.innerWidth / 608, window.innerHeight / 456) + (window.innerWidth - 608 * Math.min(window.innerWidth / 608, window.innerHeight / 456)) / 2 - 17 + "px";
    if (cursoranim) {
      cursorrot = Math.max(Math.min(cursorrot + Math.min((((event.clientX * Math.min(window.innerWidth / 608, window.innerHeight / 456) + (window.innerWidth - 608 * Math.min(window.innerWidth / 608, window.innerHeight / 456)) / 2) / (608 * Math.min(window.innerWidth / 608, window.innerHeight / 456))) - prevx) * 304, 10), 45), -45);
      prevx = (event.clientX * Math.min(window.innerWidth / 608, window.innerHeight / 456) + (window.innerWidth - 608 * Math.min(window.innerWidth / 608, window.innerHeight / 456)) / 2) / (608 * Math.min(window.innerWidth / 608, window.innerHeight / 456));
    }
  }
}

// Checks when the first half of the music is done playing.
theme1.addEventListener("ended", function() {
  theme1.currentTime = 0;
  introplayed = true;
});


const canvas = document.getElementById("mariocanvas");
const ctx = canvas.getContext("2d");
var downloadframe = 0;
var downloading = false;
var downloadsound = new Audio("snd/anim/download.wav");

var img_shadow = new Image();
img_shadow.src = "img/anim/shadow.png";
var img_mario1 = new Image();
img_mario1.src = "img/anim/mario1.png";
var img_mario2 = new Image();
img_mario2.src = "img/anim/mario2.png";
var img_mario3 = new Image();
img_mario3.src = "img/anim/mario3.png";
var img_mario4 = new Image();
img_mario4.src = "img/anim/mario4.png";
var img_mario5 = new Image();
img_mario5.src = "img/anim/mario5.png";
var jumpsound = new Audio("snd/anim/MARIO_JUMPL.WAV")
var mariosprites = [img_mario2, img_mario3, img_mario4, img_mario5, img_mario1];
var marioframe = 0;
var marioX = 0;
var marioY = 318;
var finaljumpframe = 0;

var img_box = new Image();
img_box.src = "img/anim/box.png";
var img_boxused = new Image();
img_boxused.src = "img/anim/boxused.png";
var img_shadowbox = new Image();
img_shadowbox.src = "img/anim/shadowbox.png";
var box1Y = 22;
var box1sprite = img_box;
var box2Y = 22;
var box2sprite = img_box;
var box3Y = 22;
var jumpingto = 0;
var jumped = false;
var boxtimer = 0;

var img_boxcoin0 = new Image();
img_boxcoin0.src = "img/anim/boxcoin0.png";
var img_boxcoin1 = new Image();
img_boxcoin1.src = "img/anim/boxcoin1.png";
var img_boxcoin2 = new Image();
img_boxcoin2.src = "img/anim/boxcoin2.png";
var img_boxcoin3 = new Image();
img_boxcoin3.src = "img/anim/boxcoin3.png";
var img_boxcoin4 = new Image();
img_boxcoin4.src = "img/anim/boxcoin4.png";
var boxcoinsprites = [img_boxcoin1, img_boxcoin2, img_boxcoin3, img_boxcoin4];
var img_shadowboxcoin0 = new Image();
img_shadowboxcoin0.src = "img/anim/shadowboxcoin0.png";
var img_shadowboxcoin1 = new Image();
img_shadowboxcoin1.src = "img/anim/shadowboxcoin1.png";
var img_shadowboxcoin2 = new Image();
img_shadowboxcoin2.src = "img/anim/shadowboxcoin2.png";
var img_shadowboxcoin3 = new Image();
img_shadowboxcoin3.src = "img/anim/shadowboxcoin3.png";
var img_shadowboxcoin4 = new Image();
img_shadowboxcoin4.src = "img/anim/shadowboxcoin4.png";
var shadowboxcoinsprites = [img_shadowboxcoin1, img_shadowboxcoin2, img_shadowboxcoin3, img_shadowboxcoin4];
var boxcoinX = 1000;
var boxcoinY = 1000;
var boxcoinsprite = 0;
var boxcoinsound = new Audio("snd/anim/MARIO_COIN.WAV");

var img_coin1 = new Image();
img_coin1.src = "img/anim/coin1.png";
var img_coin2 = new Image();
img_coin2.src = "img/anim/coin2.png";
var img_coin3 = new Image();
img_coin3.src = "img/anim/coin3.png";
var img_coin4 = new Image();
img_coin4.src = "img/anim/coin4.png";
var coinsprites = [img_coin1, img_coin2, img_coin3, img_coin4];
var coins = [];
var coinscollected = 0;
var finishframe = 0;


function download(maxblocks) {
  //currentiframe.contentWindow.handleReportDownloadDone();
  ring2.src = "img/ring.png";
  ring1.style = "top: 264px; left: 264px; width: 100px; height: 100px; opacity: 0.25; z-index: 3;";
  ring2.style = "top: 254px; left: 254px; width: 100px; height: 100px; z-index: 4;";
  ring1.style.display = 'block';
  ring2.style.display = 'block';
  loadsnd.pause();
  loadsnd.currentTime = 0;
  loading = true;
  downloadframe = 0;
  coinscollected = 0;
  finishframe = 0;
  boxcoinX = 1000;
  boxcoinY = 1000;
  coins = [];
  boxcoinsprite = 0;
  jumpingto = 0;
  jumped = false;
  boxtimer = 0;
  marioframe = 0;
  marioX = 0;
  marioY = 318;
  finaljumpframe = 0;
  box1sprite = img_box;
  box2sprite = img_box;
  box1Y = 22;
  box2Y = 22;
  box3Y = 22;
  setTimeout(function(){
    ring1.style.display = 'none';
    ring2.style.display = 'none';
    loadsnd.pause();
    loadsnd.currentTime = 0;
    loading = false;
    downloading = true;
    canvas.style.zIndex = "5";
  }, 1250);
}

var scale = Math.min(window.innerWidth / 608, window.innerHeight / 456);
// FPS stuff stolen from StackOverflow.
// The higher this value, the less the fps will reflect temporary variations.
// A value of 1 will only keep the last value.
var filterStrength = 20;
var frameTime = 0, lastLoop = new Date, thisLoop;
var fps = 0;

// Main loop. Called every frame.
function mainloop() {
  // Moves and scales everything to the window size.
  scale = Math.min(window.innerWidth / 608, window.innerHeight / 456);
  document.getElementById("dots").style.transform = "scale(" + scale + ")";
  document.getElementById("dots").style.top = (window.innerHeight - 456) / 2 + "px";
  document.getElementById("dots").style.left = (window.innerWidth - 608) / 2 + "px";
  document.getElementById("app").style.transform = "scale(" + scale + ")";
  document.getElementById("app").style.top = (window.innerHeight - 456) / 2 + "px";
  document.getElementById("app").style.left = (window.innerWidth - 608) / 2 + "px";
  document.getElementById("keyboard").style.transform = "scale(" + Math.min(window.innerWidth / 608, (window.innerHeight * 1.777777) / 608) + ", " + scale + ")";
  document.getElementById("keyboard").style.left = (window.innerWidth - 608) / 2 + "px";
  document.getElementById("keyprompt").style.transform = "scale(" + 1 / Math.min(window.innerWidth / 608, (window.innerHeight * 1.777777) / 608) * scale + ", 1)";
  document.getElementById("keygradient1").style.transform = "scale(1, " + scale + ")";
  document.getElementById("keygradient2").style.transform = "scale(1, " + scale + ")";
  document.getElementById("keybg").style.top = (window.innerHeight - 456) / 2 + "px";
  document.getElementById("keybg").style.transform = "scale(1, " + scale + ")";
  document.getElementById("gradient1").style.left = Math.min(0, (window.innerWidth - 608 * scale) / 2 - 0.103703704 * window.innerHeight) + "px";
  document.getElementById("gradient2").style.right = Math.min(0, (window.innerWidth - 608 * scale) / 2 - 0.103703704 * window.innerHeight) + "px";
  document.getElementById("cursor").style.transform = "scale(" + scale * 0.666666 + ") rotate(" + cursorrot + "deg)";
  /*canvas.width = Math.max(608, 608 * ((window.innerWidth / 608) / (window.innerHeight / 456)));
  canvas.style.top = (window.innerHeight - 456) / 2 + "px";
  canvas.style.left = (window.innerWidth - Math.max(608, 608 * ((window.innerWidth / 608) / (window.innerHeight / 456)))) / 2 + "px";
  canvas.style.transform = "scale(" + scale + ")";*/
  canvas.width = window.innerWidth;
  canvas.height = window.innerHeight;
  //canvas.style.top = (window.innerHeight - canvas.height) / 2 + "px";
  ctx.webkitImageSmoothingEnabled = false;// Took me hours to realize this needs to come after the resizing for some god forsaken reason.
  ctx.imageSmoothingEnabled = false;
  ctx.clearRect(0, 0, canvas.width, canvas.height);
  if (cursorrot > 0) {
    cursorrot = Math.max(Math.floor(cursorrot * 0.85), 0);
  } else if (cursorrot < 0) {
    cursorrot = Math.min(Math.ceil(cursorrot * 0.85), 0);
  }
  // Hides/shows the OS cursor if the custom pointer option is toggled.
  if (cursor != prevcursor) {
    var el = document.querySelectorAll('*'); // This selects literally every element. Lovely.
    if (cursor) {
      document.getElementById("cursor").style.display = "block";
      document.body.style.cursor = "none";
      for (var i = 0; i < el.length; i++){
        el[i].style.cursor = "none";
      }
    } else {
      document.getElementById("cursor").style.display = "none";
      document.body.style.cursor = "auto";
      for (var i = 0; i < el.length; i++){
        el[i].style.cursor = "auto";
      }
    }
  }
  // Plays the loading sound if it can.
  if (loading) {
    var promise = loadsnd.play();
    if (promise !== undefined) {
      promise.then(_ => {
        // Autoplay started!
      }).catch(error => {
        // Autoplay was prevented.
        // This is only here so the browser will stop screaming in agony when it tries to play audio before the user interacts with the site.
      });
    }
  }
  // Plays the second half of the music if the first half is finished.
  if (introplayed) {
    var tp2 = theme2.play();
    if (tp2 !== undefined && tp2.catch) { tp2.catch(function(){}); }
  }
  //document.getElementById("line01").textContent = "";
  //document.getElementById("line02").textContent = "";
  //for (i = 0; i < window.innerWidth / 11; i++) {
  //  document.getElementById("line01").textContent += "･";
  //  document.getElementById("line02").textContent += "･";
  //}
  
  // Moves and scales the keyboard to the window size.
  if (keyshown) {
    document.getElementById("keyboard").style.top = (window.innerHeight - 456) / 2 + "px";
    document.getElementById("keygradient1").style.top = (window.innerHeight - 456) / 2 + "px";
    document.getElementById("keygradient2").style.top = (window.innerHeight - 456) / 2 + "px";
    document.getElementById("keygbg1").style.top = (window.innerHeight + 456 * scale) / 2 - 1 + "px";
    document.getElementById("keygbg2").style.height = (window.innerHeight - 456 * scale) / 2 + 1 + "px";
    document.getElementById("keyprompt").focus();
  } else {
    document.getElementById("keyboard").style.top = (window.innerHeight - 456) / 2 + (304 * scale) + "px";
    document.getElementById("keygradient1").style.top = (window.innerHeight - 456) / 2 + (228 * scale) + "px";
    document.getElementById("keygradient2").style.top = (window.innerHeight - 456) / 2 + (-38 * scale) + "px";
    document.getElementById("keygbg1").style.top = (window.innerHeight + 912 * scale) / 2 + "px";
    document.getElementById("keygbg2").style.height = Math.max((window.innerHeight - 532 * scale) / 2, 0) + "px";
  }
  // Do the weird fade thing if the user clicks the Wii Menu button.
  if (fade) {
    fadeframe += 0.1;
    document.getElementById("fadescreen").style.zIndex = 1002;
    document.getElementById("fadescreen").style.opacity = Math.min(fadeframe, 1);
    loadsnd.volume = (soundvol / 100.0) * Math.max(1 - fadeframe, 0);
    theme1.volume = (musicvol / 100.0) * Math.max(1 - fadeframe, 0);
    theme2.volume = (musicvol / 100.0) * Math.max(1 - fadeframe, 0);
    if (fadeframe >= 15) {
      document.getElementById("fadescreen").style.zIndex = -1002;
      document.getElementById("fadescreen").style.opacity = 0;
      loadsnd.volume = soundvol / 100.0;
      theme1.volume = musicvol / 100.0;
      theme2.volume = musicvol / 100.0;
      fadeframe = 0;
      fade = false;
    }
  } else {
    theme1.volume = musicvol / 100.0;
    theme2.volume = musicvol / 100.0;
    for (var i = 0; i < soundlist.length; i++) {
      soundlist[i].volume = soundvol / 100.0;
    }
  }
  
  if (downloading) {
    if (downloadframe < finishframe || finishframe == 0) {
      theme1.volume = (musicvol / 100.0) * Math.max((260 - downloadframe) / 260.5, 0.5);
      theme2.volume = (musicvol / 100.0) * Math.max((260 - downloadframe) / 260.5, 0.5);
      ctx.globalAlpha = 0.37 * Math.min(downloadframe / 130, 1);
      ctx.webkitImageSmoothingEnabled = true;
      ctx.imageSmoothingEnabled = true;
      ctx.drawImage(shadowboxcoinsprites[Math.floor(boxcoinsprite)], (boxcoinX - 4) * scale + canvas.width / 2, (boxcoinY - 8) * scale + canvas.height / 2, shadowboxcoinsprites[Math.floor(boxcoinsprite)].width * scale * 1.5, shadowboxcoinsprites[Math.floor(boxcoinsprite)].height * scale * 1.5);
      ctx.drawImage(img_shadowbox, -199 * scale + canvas.width / 2, (box1Y - 12) * scale + canvas.height / 2, img_shadowbox.width * scale, img_shadowbox.height * scale);
      ctx.drawImage(img_shadowbox, -28 * scale + canvas.width / 2, (box2Y - 12) * scale + canvas.height / 2, img_shadowbox.width * scale, img_shadowbox.height * scale);
      ctx.drawImage(img_shadowbox, 142 * scale + canvas.width / 2, (box3Y - 12) * scale + canvas.height / 2, img_shadowbox.width * scale, img_shadowbox.height * scale);
      ctx.drawImage(img_shadow, (marioX + 2) * scale + canvas.width / 2, (marioY + 30) * scale + canvas.height / 2, 14 * scale * 2, 3 * scale * 2);
      ctx.globalAlpha = 1 * Math.min(downloadframe / 130, 1);
      ctx.webkitImageSmoothingEnabled = false;
      ctx.imageSmoothingEnabled = false;
      ctx.drawImage(boxcoinsprites[Math.floor(boxcoinsprite)], boxcoinX * scale + canvas.width / 2, boxcoinY * scale + canvas.height / 2, boxcoinsprites[Math.floor(boxcoinsprite)].width * scale * 1.5, boxcoinsprites[Math.floor(boxcoinsprite)].height * scale * 1.5);
      ctx.drawImage(box1sprite, -187 * scale + canvas.width / 2, box1Y * scale + canvas.height / 2, box1sprite.width * scale * 2, box1sprite.height * scale * 2);
      ctx.drawImage(box2sprite, -16 * scale + canvas.width / 2, box2Y * scale + canvas.height / 2, box2sprite.width * scale * 2, box2sprite.height * scale * 2);
      ctx.drawImage(img_box, 154 * scale + canvas.width / 2, box3Y * scale + canvas.height / 2, img_box.width * scale * 2, img_box.height * scale * 2);
      ctx.drawImage(mariosprites[Math.floor(marioframe)], marioX * scale + canvas.width / 2, marioY * scale + canvas.height / 2, mariosprites[Math.floor(marioframe)].width * scale * 2, mariosprites[Math.floor(marioframe)].height * scale * 2);
      if (finishframe != 0) {
        ctx.fillStyle = 'rgba(255,255,255,' + (1 / (((downloadframe - finishframe) / 30) ** 2 + 1) ** 2) + ')';
        ctx.fillRect(0,0,canvas.width,canvas.height);
      }
    
      if (downloadframe > 100) {
        downloadsound.volume = (soundvol / 100.0);
        downloadsound.play();
        if (downloadframe % 30 == 0) {
          coins.push({xpos: (canvas.width / 2) / scale, sprite: 0, sound: new Audio("snd/anim/MARIO_COIN.WAV")});
        }
        for (var c = 0; c < coins.length; c++) {
          ctx.globalAlpha = 0.37 * Math.min(downloadframe / 130, 1);
          ctx.drawImage(img_shadow, (coins[c].xpos + 5) * scale + canvas.width / 2, 121 * scale + canvas.height / 2, 14 * scale, 4.5 * scale);
          ctx.globalAlpha = 1 * Math.min(downloadframe / 130, 1);
          ctx.drawImage(coinsprites[Math.floor(coins[c].sprite)], coins[c].xpos * scale + canvas.width / 2, 95 * scale + canvas.height / 2, coinsprites[Math.floor(coins[c].sprite)].width * scale * 1.5, coinsprites[Math.floor(coins[c].sprite)].height * scale * 1.5);
          coins[c].xpos = Math.min(coins[c].xpos - 3.5, (canvas.width / 2) / scale);
          coins[c].sprite = (coins[c].sprite + 0.166667) % 4;
          if (coins[c].xpos < marioX + 16) {
            coins[c].sound.volume = (soundvol / 200.0);
            coins[c].sound.play();
            coins.splice(c, 1);
            c--;
            coinscollected++;
          }
        }
        
        marioframe = (marioframe + 0.25) % 3;
        if (boxtimer > 0) {
          boxtimer += 2;
        }
        if (marioY < 51 && jumped == false) {
          jumped = true;
          boxtimer = 2;
          boxcoinsound.volume = (soundvol / 200.0);
          boxcoinsound.play();
        }
        if (jumpingto == 1) {
          marioY = Math.min((0.05 * ((marioX + 187) ** 2)) + 46, 90);
          box1Y = Math.min((0.1 * ((boxtimer - 11) ** 2)) + 10, (marioX < -184 || marioX > -171 ? 22 : 24));
          boxcoinY = Math.min((0.075 * ((boxtimer - 39) ** 2)) - 90, 24);
          boxcoinX = -177;
          boxcoinsprite = (boxcoinsprite + 0.25) % 4;
          if (box1Y < 22) {
            box1sprite = img_boxused;
          }
        } else if (jumpingto == 2) {
          marioY = Math.min((0.05 * ((marioX + 16) ** 2)) + 46, 90);
          box2Y = Math.min((0.1 * ((boxtimer - 11) ** 2)) + 10, (marioX < -16 || marioX > 0 ? 22 : 24));
          boxcoinY = Math.min((0.075 * ((boxtimer - 39) ** 2)) - 90, 24);
          boxcoinX = -6;
          boxcoinsprite = (boxcoinsprite + 0.25) % 4;
          if (box2Y < 22) {
            box2sprite = img_boxused;
          }
        } else if (jumpingto == 3) {
          if (marioX >= 154) {
            finaljumpframe += 3;
            if (finishframe == 0) {
              finishframe = downloadframe + 60 + Math.floor(Math.random() * 540);
            }
          }
          marioY = Math.min((0.05 * ((finaljumpframe - 29.7) ** 2)) + 46, 90);
          box3Y = Math.min((0.1 * ((boxtimer - 11) ** 2)) + 10, (boxtimer < 8 || boxtimer > 16 ? 22 : 24));
          boxcoinY = Math.min((0.2 * ((boxtimer - 24) ** 2)) - 90, 24);
          boxcoinX = 164;
          boxcoinsprite = (boxcoinsprite + 0.25) % 4;
        } else {
          marioY = 90;
        }
        if (marioY < 90) {
          if (!(jumpingto == 3 && marioX >= 154)) {
            marioX += 2;
          }
          marioframe = 3;
          jumpsound.volume = (soundvol / 100.0);
          jumpsound.play();
        } else {
          jumped = false;
          finaljumpframe = 0;
          if (jumpingto == 3) {
            if (marioX >= 154) {
              marioframe = 4;
            } else {
              marioX += Math.min(Math.sqrt(-(marioX - 154.1)), 4);
            }
          } else {
            marioX += 4;
          }
        }
        if (marioX > (canvas.width / 2) / scale || marioX < -(canvas.width / 2) / scale - 32) {
          marioX = -(canvas.width / 2) / scale - 32;
          boxtimer = 0;
          if (jumpingto == 1 || jumpingto == 2) {
            jumpingto += 0.5;
          }
          if (coinscollected >= 15 && jumpingto <= 1) {
            jumpingto = 1;
          }
          if (coinscollected >= 30 && jumpingto <= 2) {
            jumpingto = 2;
          }
          if (coinscollected >= 45 && jumpingto <= 3) {
            jumpingto = 3;
          }
        }
      }
    } else {
      ctx.fillStyle = 'rgba(255,255,255,' + (1 / (((downloadframe - finishframe) / 15) ** 2 + 1) ** 2) + ')';
      ctx.fillRect(0,0,canvas.width,canvas.height);
      if (downloadframe > finishframe + 60) {
        currentiframe.contentWindow.handleReportDownloadDone();
        document.getElementById("cSE_Finish").play();
        downloading = false;
        canvas.style.zIndex = "-5";
      }
      theme1.volume = (musicvol / 100.0);
      theme2.volume = (musicvol / 100.0);
    }
    downloadframe++;
  } else {
    marioX = -(canvas.width / 2) / scale - 32;
  }
  
  prevcursor = cursor;
  // FPS stuff
  var thisFrameTime = (thisLoop=new Date) - lastLoop;
  frameTime+= (thisFrameTime - frameTime) / filterStrength;
  lastLoop = thisLoop;
  fps = (1000/thisFrameTime).toFixed(1);
  if (showfps) {
    document.getElementById("fps").style.display = "block";
    document.getElementById("fps").innerHTML = fps + " fps";
  } else {
    document.getElementById("fps").style.display = "none";
  }
  //setTimeout(mainloop, Math.min(1, 16.666666));
  window.requestAnimationFrame(mainloop);
}
// Do the thing.
mainloop();