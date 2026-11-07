#!/usr/bin/env python3
"""Enrich the licensed catalog with REAL Wii Shop Channel data.

Sources:
- WiiMart revival repo: www/oss/oss/common/js/games.json  -> real metadata
  (name, publisher, genre, points/price, rating, console/platform, date).
  https://github.com/WiiMart/WiiMart
- WiiMart MoreDetails/*_en.jsp -> verbatim Nintendo English descriptions.
  https://github.com/WiiMart/WiiMart/tree/main/www/MoreDetails

Usage: apply_wii_data.py catalog_generated.json <wiimart_root> out_catalog.json
  wiimart_root is the extracted WiiMart-main/www directory.
"""

import json
import sys
import os
import glob
import re
import html
import time
import calendar
import collections

# Canonical platform labels shown in the Wii Shop for each real console value.
PLATFORM_MAP = {
    "wiiware": "WiiWare",
    "wii channels": "Wii Channel",
    "wii channel": "Wii Channel",
    "wii": "Wii",
    "canales wii": "Wii Channel",
    "nes": "NES",
    "nintendo entertainment system": "NES",
    "super nes": "Super NES",
    "super nintendo": "Super NES",
    "sfc": "Super NES",
    "snes": "Super NES",
    "nintendo 64": "Nintendo 64",
    "n64": "Nintendo 64",
    "sega genesis": "Sega Genesis",
    "sega mega drive": "Sega Genesis",
    "mega drive": "Sega Genesis",
    "genesis": "Sega Genesis",
    "master system": "Master System",
    "sega master system": "Master System",
    "sms": "Master System",
    "turbografx16": "TurboGrafx-16",
    "turbografx (pc engine)": "TurboGrafx-16",
    "turbografx": "TurboGrafx-16",
    "pce": "TurboGrafx-16",
    "neogeo": "Neo Geo",
    "commodore 64": "Commodore 64",
    "c64": "Commodore 64",
    "virtual console arcade": "Virtual Console Arcade",
    "arcade": "Virtual Console Arcade",
}

# Region of a title id in our catalog, derived from the slug region suffix.
def slug_region(slug):
    low = slug.lower()
    for r in ("-japan", "-korea", "-france", "-germany", "-italy", "-spain", "-canada", "-usa", "-europe"):
        if low.endswith(r):
            return r[1:]
    return "europe"

def clean_name(nm):
    # The real Wii Shop catalog stores names with HTML <sup> markers
    # (e.g. "Mario Bros.<sup>TM</sup>"). Our shop UI escapes HTML, so turn
    # the trademark/registered/copyright superscripts into their symbols and
    # strip any other tags so the name reads cleanly as plain text.
    nm = re.sub(r"<sup>\s*(TM|™)\s*</sup>", "\u2122", nm, flags=re.I)
    nm = re.sub(r"<sup>\s*(R|®)\s*</sup>", "\u00ae", nm, flags=re.I)
    nm = re.sub(r"<sup>\s*(C|©)\s*</sup>", "\u00a9", nm, flags=re.I)
    nm = re.sub(r"<[^>]+>", "", nm)
    return re.sub(r"\s+", " ", nm).strip()

def parse_date(s):
    if not s:
        return 0
    s = s.strip()
    # Real shop dates are "MM/DD/YYYY", "M/D/YYYY", or a year like "1988".
    m = re.match(r"^(\d{1,2})/(\d{1,2})/(\d{4})$", s)
    if m:
        mo, d, y = int(m.group(1)), int(m.group(2)), int(m.group(3))
        try:
            return calendar.timegm((y, mo, d, 0, 0, 0))
        except Exception:
            return 0
    m = re.match(r"^(\d{4})$", s)
    if m:
        return calendar.timegm((int(m.group(1)), 1, 1, 0, 0, 0))
    return 0

def extract_description(jsp_path):
    """Pull the verbatim description out of a Wii Shop More Details page."""
    try:
        raw = open(jsp_path, encoding="utf-8", errors="replace").read()
    except Exception:
        return ""
    # The description body lives in <div id="goodsfree">...<div id="text" ...>...</div>
    m = re.search(r'<div[^>]*id="goodsfree"[^>]*>(.*?)</div>\s*<div[^>]*id="image02"', raw, re.S)
    if not m:
        m = re.search(r'<div[^>]*id="goodsfree"[^>]*>(.*?)</div>', raw, re.S)
    if not m:
        return ""
    inner = m.group(1)
    # Grab only the #text block (the actual prose), drop screen-shot images.
    tm = re.search(r'<div[^>]*id="text"[^>]*>(.*?)</div>', inner, re.S)
    if tm:
        inner = tm.group(1)
    txt = re.sub(r'<br\s*/?>', "\n", inner, flags=re.I)
    txt = re.sub(r"<[^>]+>", "", txt)
    txt = html.unescape(txt)
    txt = re.sub(r"\s*\n\s*", "\n", txt)
    lines = [ln.strip() for ln in txt.split("\n")]
    txt = "\n".join(ln for ln in lines if ln)
    return txt.strip()

def main():
    if len(sys.argv) != 4:
        print("usage: apply_wii_data.py catalog_generated.json <wiimart_root> out.json", file=sys.stderr)
        return 2
    cat = json.load(open(sys.argv[1], encoding="utf-8"))
    root = sys.argv[2]

    # --- WiiMart games.json indexed by title id -------------------------------
    gjson = os.path.join(root, "oss", "oss", "common", "js", "games.json")
    games = json.load(open(gjson, encoding="utf-8"))
    byid = collections.defaultdict(list)
    for x in games:
        byid[x.get("id", "").upper()].append(x)
    def best_entry(tid):
        cands = byid.get(tid.upper(), [])
        if not cands:
            return None
        # Prefer English North-America/GB entries for real display names.
        for pref in ("US", "GB", "AU"):
            for c in cands:
                if c["language"].upper() == "EN" and c["region"].upper() == pref:
                    return c
        for c in cands:
            if c["language"].upper() == "EN":
                return c
        return cands[0]

    # --- English More Details descriptions by title id -------------------------
    descs = {}
    md = os.path.join(root, "MoreDetails")
    for p in glob.glob(os.path.join(md, "*_en.jsp")):
        tid = os.path.basename(p).split("_", 1)[0].upper()
        d = extract_description(p)
        if d:
            descs[tid] = d
    print(f"games.json ids: {len(byid)}  english descriptions: {len(descs)}", file=sys.stderr)

    changed_name = changed_price = changed_pub = changed_date = changed_desc = 0
    out = []
    for a in cat:
        tid = a.get("shop", {}).get("title_id", "")
        wm = best_entry(tid) if tid else None
        entry = dict(a)
        # description
        if tid.upper() in descs:
            full = descs[tid.upper()]
            first = full.split("\n", 1)[0].strip()
            if len(first) > 200:
                first = first[:197].rsplit(" ", 1)[0] + "..."
            entry["description"] = {"short": first, "long": full}
            changed_desc += 1
        # real name (English title1 only)
        if wm and wm["language"].upper() == "EN":
            nm = wm.get("title1", "").strip()
            if nm:
                entry["name"] = clean_name(nm)
                changed_name += 1
            pub = wm.get("publisher", "").strip()
            if pub:
                entry["author"] = pub
                changed_pub += 1
            pts = wm.get("points", "")
            try:
                p = int(pts)
            except Exception:
                p = 0
            if p > 0:
                entry["price"] = p
                changed_price += 1
            dt = parse_date(wm.get("date", ""))
            if dt:
                entry["release_date"] = dt
                changed_date += 1
            cons = (wm.get("console", "") or "").lower().strip()
            platform = PLATFORM_MAP.get(cons, "")
            genre = (wm.get("genre", "") or "").strip()
            rating = (wm.get("rating", "") or "").strip()
            # Store optional real fields for the shop UI (games.js).
            if platform:
                entry["platform"] = platform
            if genre:
                entry["genre"] = genre
            if rating:
                entry["rating"] = rating
        out.append(entry)

    print(f"changed: name {changed_name} price {changed_price} pub {changed_pub} date {changed_date} desc {changed_desc}", file=sys.stderr)
    with open(sys.argv[3], "w", encoding="utf-8") as f:
        json.dump(out, f, ensure_ascii=False, indent=1)
    print(f"wrote {sys.argv[3]} ({len(out)} apps)", file=sys.stderr)
    return 0

if __name__ == "__main__":
    sys.exit(main())
