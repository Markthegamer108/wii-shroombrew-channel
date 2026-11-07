#!/usr/bin/env python3
"""Generate catalog.json for Shroombrew's licensed Nintendo games.

Shroombrew does not host these WADs. Each catalog entry points at a real
mariocube WAD URL; the console's native download fetches the WAD from
mariocube on first use and caches it locally under ./channels/<titleid>/.

Sources:
  * "WAD Info.xlsx"  (mariocube): authoritative Filename <-> TitleID mapping.
  * repo.mariocube.com listings: the real files on disk, so URLs are always
    correct even when the 2019 sheet has stale renames.
        - top-level A-Z folders  : the "Wii (Digital)" scene set (matches the
          sheet names/versions exactly for most titles).
        - _Clean/ A-Z folders    : clean Nintendo-pulled WADs (recommended by
          mariocube); the complete set, used as a fallback for titles absent
          from the scene set.

Scope: Virtual Console + WiiWare + DLC + system Channels (IOS is excluded).

Usage: python build_catalog.py WAD_Info.xlsx out_catalog.json
"""

import difflib
import html
import json
import re
import sys
import urllib.request
import openpyxl

BASE = "https://repo.mariocube.com/WADs/_WiiWare,%20VC,%20DLC,%20Channels%20&%20IOS"
LETTERS = [chr(c) for c in range(ord("A"), ord("Z") + 1)]

# Console-tier pricing (real historical Wii Points per Virtual Console system).
SYSTEMS = {
    "nes": 500, "famicom disk system": 500, "fds": 500, "msx": 500,
    "commodore 64": 500, "c64": 500, "arcade": 600, "master system": 800,
    "sms": 800, "mega drive": 800, "smd": 800, "md": 800, "genesis": 800,
    "turbografx-16": 800, "turbografx": 800, "tgx": 800, "pc engine": 800,
    "pce": 800, "snes": 800, "sfc": 800, "neo geo": 900, "ng": 900,
    "n64": 1000,
}
WW_DEFAULT = 1000
DLC_DEFAULT = 1000

REGION_MARKERS = ["japan", "europe", "usa", "korea", "france", "germany",
                  "italy", "spain", "canada"]
# marker substrings that pin down a console tier (checked inside a paren token)
SYSTEM_MARKERS = ["famicom disk system", "commodore 64", "master system",
                  "mega drive", "turbografx-16", "turbografx", "neo geo",
                  "nintendo 64", "n64", "snes", "nes", "fds", "sms", "smd",
                  "genesis", "pce", "tgx", "ng", "c64", "msx", "arcade",
                  "sfc", "md"]


def fetch_listing(base_url):
    with urllib.request.urlopen(base_url, timeout=60) as r:
        page = r.read().decode("utf-8", "ignore")
    files = {}
    for m in re.finditer(r'<a href="([^"]+\.wad)">([^<]+)</a></td><td>(\d+)', page):
        href = m.group(1)
        name = html.unescape(m.group(2))
        size = int(m.group(3))
        files[name] = (href, size)
    return files


def fetch_all():
    repo = {}
    # top-level scene set
    for f in ["0-9"] + LETTERS:
        url = BASE + "/" + f + "/"
        try:
            for name, (href, size) in fetch_listing(url).items():
                repo["scene/" + name] = (BASE + "/" + f + "/" + href, size)
        except Exception as e:
            print(f"WARN scene {f}: {e}", file=sys.stderr)
    # clean set under _Clean/
    for f in ["#"] + LETTERS:
        url = BASE + "/_Clean/" + f + "/"
        try:
            for name, (href, size) in fetch_listing(url).items():
                repo["clean/" + name] = (BASE + "/_Clean/" + f + "/" + href, size)
        except Exception as e:
            print(f"WARN clean {f}: {e}", file=sys.stderr)
    # Just Dance / Guitar Hero / Rock Band DLC lives in a sibling folder
    JD = "https://repo.mariocube.com/WADs/!Rock%20Band,%20Just%20Dance,%20%26%20Guitar%20Hero%20DLC/Big%20WADs"
    try:
        for name, (href, size) in fetch_listing(JD + "/").items():
            repo["bigwads/" + name] = (JD + "/" + href, size)
    except Exception as e:
        print(f"WARN bigwads: {e}", file=sys.stderr)
    return repo


def classify(fn):
    """Return (category, system) or None if out of scope."""
    low = fn.lower()
    if "(virtual console)" in low:
        sysn = None
        for m in re.finditer(r"\(([^()]*)\)", fn):
            tok = m.group(1).strip().lower()
            if not tok:
                continue
            for sm in SYSTEM_MARKERS:
                if sm in tok:
                    sysn = sm
                    break
            if sysn:
                break
        return ("Virtual Console", sysn)
    if "(wiiware)" in low or "(wiware)" in low:
        return ("DLC", None) if "(dlc)" in low else ("WiiWare", None)
    if "(dlc)" in low:
        return ("DLC", None)
    if "(channel)" in low:
        return ("Channels", None)
    return None


def core_key(fn):
    """Pure game-name key: strip every parenthesised token and .wad."""
    s = fn.replace("'", "_")
    s = re.sub(r"\(.*?\)", "", s).replace(".wad", "").strip().lower()
    return re.sub(r"[^a-z0-9]+", " ", s).strip()


def region_key(fn):
    low = fn.lower()
    for m in REGION_MARKERS:
        if m in low:
            return m
    return "other"


def price_for(category, system):
    if category == "Virtual Console" and system:
        return SYSTEMS.get(system, WW_DEFAULT)
    if category == "DLC":
        return DLC_DEFAULT
    if category == "Channels":
        # System channels (Mii, Forecast, News, Photo, Wii Shop) were free
        # downloads from the Wii Shop, like the preinstalled channels.
        return 0
    return WW_DEFAULT


def pick(cands, sheet_system, key):
    """Pick the best candidate: prefer system-marker match, then clean dumps,
    then highest core-key similarity; avoid Demo/Promo when a full release is
    present."""
    # prefer candidates whose Virtual Console system matches the sheet
    for sysmatch in (True, False):
        pool = []
        for label, v in cands.items():
            name = label.split("/", 1)[1]
            c2 = classify(name)
            if (c2[1] == sheet_system) != sysmatch:
                continue
            pool.append((label, v, name))
        if not pool:
            continue
        pool.sort(key=lambda t: (-core_key_sim(key, t[2]), t[0]))
        # within the chosen pool, avoid Demo/Promo if a full one exists
        full = [t for t in pool if "(demo)" not in t[2].lower() and "(promo)" not in t[2].lower()]
        sel = full if full else pool
        l, v, name = sel[0]
        return (v[0], v[1], v[2]), l.split("/", 1)[0]
    return None


def core_key_sim(a, b):
    return difflib.SequenceMatcher(None, a, core_key(b)).ratio()


def match(repo, sheet_fn, category):
    """Find the real mariocube file for a sheet filename."""
    # 1) exact display-name match in the scene set
    if ("scene/" + sheet_fn) in repo:
        return (repo["scene/" + sheet_fn][0], repo["scene/" + sheet_fn][1], sheet_fn), "scene"
    key = core_key(sheet_fn)
    reg = region_key(sheet_fn)
    sheet_system = classify(sheet_fn)[1]
    cands = {}
    # 2) exact core-key match
    for label, (url, size) in repo.items():
        name = label.split("/", 1)[1]
        if core_key(name) != key or region_key(name) != reg:
            continue
        c2 = classify(name)
        if c2 is None or c2[0] != category:
            continue
        cands[label] = (url, size, name)
    if cands:
        return pick(cands, sheet_system, key)
    # 3) near-match fallback: region + category agree and core-key similarity
    #    is high (handles sheet typos, romanisation and localised names).
    for label, (url, size) in repo.items():
        name = label.split("/", 1)[1]
        if region_key(name) != reg:
            continue
        c2 = classify(name)
        if c2 is None or c2[0] != category:
            continue
        if core_key_sim(key, name) >= 0.60:
            cands[label] = (url, size, name)
    if not cands:
        return None
    return pick(cands, sheet_system, key)


def slugify(name, region):
    s = re.sub(r"[^A-Za-z0-9]+", "-", name).strip("-").lower()
    s = s[:40]
    if region and region != "other":
        s += "-" + region
    return s


def clean_game(realname):
    s = re.sub(r"\s*\(.*?\)", "", realname).replace(".wad", "").strip()
    # drop Japanese-title residue and any stray parens that survive the strip
    s = re.sub(r"[\u3040-\u30ff\u4e00-\u9fff]+", "", s)
    s = re.sub(r"[()]", "", s).strip()
    return re.sub(r"\s+", " ", s).strip()


SYSTEM_NAMES = {
    "nes": "Nintendo Entertainment System",
    "fds": "Famicom Disk System",
    "famicom disk system": "Famicom Disk System",
    "snes": "Super Nintendo Entertainment System",
    "sfc": "Super Nintendo Entertainment System",
    "n64": "Nintendo 64",
    "nintendo 64": "Nintendo 64",
    "sms": "Master System",
    "master system": "Master System",
    "md": "Mega Drive / Genesis",
    "smd": "Mega Drive / Genesis",
    "mega drive": "Mega Drive / Genesis",
    "genesis": "Mega Drive / Genesis",
    "tgx": "TurboGrafx-16 / PC Engine",
    "turbografx": "TurboGrafx-16 / PC Engine",
    "turbografx-16": "TurboGrafx-16 / PC Engine",
    "pce": "TurboGrafx-16 / PC Engine",
    "ng": "Neo Geo",
    "neo geo": "Neo Geo",
    "c64": "Commodore 64",
    "commodore 64": "Commodore 64",
    "msx": "MSX",
    "arcade": "Arcade",
}

REGION_NAMES = {
    "japan": "Japan",
    "europe": "Europe",
    "usa": "North America",
    "korea": "Korea",
    "france": "France",
    "germany": "Germany",
    "italy": "Italy",
    "spain": "Spain",
    "canada": "Canada",
}


def describe(cat, system, game, region):
    """Build an authentic, accurate description from the available metadata.
    The licensed catalog carries no verbatim Nintendo copy, so descriptions
    are written in the style of the real Wii Shop from the real title data."""
    reg = REGION_NAMES.get(region)
    regnote = (" in the %s region" % reg) if reg else ""
    if cat == "Virtual Console":
        sysname = SYSTEM_NAMES.get(system)
        if sysname:
            short = 'Play the classic %s title "%s" on your Wii.' % (sysname, game)
            long = ('This Virtual Console release brings "%s", a classic %s title, to the Wii Shop '
                    'Channel%s. It can be played with the Wii Remote, the Classic Controller, or a '
                    'GameCube controller. Downloaded from the Shop Channel, it installs to your Wii '
                    'Menu as its own channel.') % (game, sysname, regnote)
        else:
            short = 'A classic title for the Virtual Console: "%s".' % game
            long = ('"%s" is a classic game available on the Wii Virtual Console%s. It can be played '
                    'with the Wii Remote, the Classic Controller, or a GameCube controller.'
                    ) % (game, regnote)
    elif cat == "WiiWare":
        short = 'Download "%s" from the WiiWare section of the Wii Shop Channel.' % game
        long = ('"%s" is a downloadable game from the WiiWare section of the Wii Shop Channel%s. It '
                'installs to your Wii Menu as its own channel and runs directly on the console.'
                ) % (game, regnote)
    elif cat == "DLC":
        short = 'Additional downloadable content for "%s".' % game
        long = ('This is additional downloadable content for "%s", available from the Wii Shop '
                'Channel%s. It adds extra playable content to the base title.') % (game, regnote)
    elif cat == "Channels":
        short = 'A Wii Shop Channel for "%s".' % game
        long = ('"%s" is a Wii Channel available from the Wii Shop Channel%s. It installs directly '
                'to your Wii Menu alongside your other channels.') % (game, regnote)
    else:
        short = '"%s" for the Wii.' % game
        long = '"%s" is available from the Wii Shop Channel%s for your Wii.' % (game, regnote)
    return short, long


def main():
    if len(sys.argv) != 3:
        print("usage: build_catalog.py WAD_Info.xlsx out_catalog.json", file=sys.stderr)
        return 2

    wb = openpyxl.load_workbook(sys.argv[1], read_only=True, data_only=True)
    ws = wb.worksheets[0]

    print("fetching mariocube listings (scene + clean) ...", file=sys.stderr)
    repo = fetch_all()
    print(f"repo files indexed: {len(repo)}", file=sys.stderr)

    # collect in-scope rows, dedupe by title ID keeping the highest version
    rows = {}
    for i, row in enumerate(ws.iter_rows(min_row=2, values_only=True), start=2):
        fn, tid, ver, reg = row[0], row[1], row[2], row[3]
        if not fn or not tid:
            continue
        cls = classify(fn)
        if cls is None:
            continue
        key = (tid, cls[0])
        cur = rows.get(key)
        if cur is None or (ver or 0) >= (cur[3] or 0):
            rows[key] = (fn, tid, cls, ver, reg)
    print(f"in-scope rows (deduped by title id): {len(rows)}", file=sys.stderr)

    apps = []
    missing = 0
    used = set()
    for (tid, cat), (fn, tid_, cls, ver, reg) in rows.items():
        m = match(repo, fn, cat)
        if m is None:
            missing += 1
            continue
        (url, size, realname), source = m
        game = clean_game(realname)
        region = region_key(fn)
        slug = slugify(game, region)
        system = cls[1]
        price = price_for(cat, system)
        try:
            tver = int(ver)
        except (TypeError, ValueError):
            tver = 0
        # description.short is an authentic blurb (it never equals the name, so
        # Go's displayName() keeps the real clean title as the display name).
        short, long = describe(cat, system, game, region)
        entry = {
            "slug": slug,
            "name": game,
            "author": "Nintendo",
            "category": cat,
            "version": "",
            "licensed": True,
            "price": price,
            "description": {"short": short, "long": long},
            "url": {"icon": "", "zip": url},
            "file_size": {"zip_compressed": size, "zip_uncompressed": 0},
            "shop": {"title_id": tid, "title_version": tver, "contents_size": 0},
            "peripherals": [],
            "release_date": 0,
        }
        # make slugs unique (collisions lose titles in the bySlug map)
        s = slug
        if s in used:
            s = slug + "-" + tid[-4:].lower()
            i = 2
            while s in used:
                s = slug + "-" + tid[-4:].lower() + "-" + str(i)
                i += 1
        used.add(s)
        entry["slug"] = s
        apps.append(entry)

    print(f"catalog apps generated: {len(apps)}; unmatched: {missing}", file=sys.stderr)
    with open(sys.argv[2], "w", encoding="utf-8") as f:
        json.dump(apps, f, ensure_ascii=False, indent=1)
    print(f"wrote {sys.argv[2]}", file=sys.stderr)
    return 0


if __name__ == "__main__":
    sys.exit(main())
