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


# --- Channel extras -------------------------------------------------------
# The sheet predates several channels and lists a few under older names, so the
# rows below pin down the real titles the shop should show. They are applied to
# every generated catalog; the six newest carry `added` so only those wear the
# golden NEW badge (30-day window in Go).

ADDED_CHANNELS = 1794096000  # 2026-11-08, the day they joined the catalog.

CHANNEL_FIXES = {
    "netflix-for-wii": dict(name="Netflix", genre="Streaming Video",
                            players="For 1 player", release_date=1287360000),
    "bbc-iplayer": dict(name="BBC iPlayer", genre="Internet TV",
                        players="For 1 player", release_date=0),
    "kirby-tv-channel": dict(name="Kirby TV Channel", genre="TV Channel",
                             players="For 1 player", release_date=1308787200),
    "legend-of-zelda-the-skyward-sword-save-d": dict(
        name="Zelda Skyward Sword Save Data Update Channel", genre="Channel update",
        players="None", release_date=1324512000),
    "internet-channel": dict(name="Internet Channel", genre="Internet Browser",
                             players="For 1-4 players", release_date=1176249600),
    "everybody-votes-channel": dict(name="Everybody Votes Channel", genre="Voting",
                                    players="For 1-6 players", release_date=1171324800),
    "nintendo-channel": dict(name="Nintendo Channel", genre="Information",
                             players="For 1 player", release_date=1210118400),
    "today-and-tomorrow-channel": dict(name="Today and Tomorrow Channel",
                                       genre="Lifestyle", players="For 1-6 players",
                                       release_date=1252454400),
    "metroid-prime-3-preview-channel": dict(name="Metroid Prime 3 Preview",
                                            genre="Preview", players="For 1 player",
                                            release_date=1186704000),
    "jam-with-the-band-live": dict(name="Jam with the Band Live Channel",
                                   genre="Add-on Channel", players="For 1-8 players",
                                   release_date=1274400000),
    "wii-speak-channel-2-0": dict(name="Wii Speak Channel", genre="Voice Communication",
                                  players="For 1-4 players", release_date=1228435200),
}


def channel_desc(name, note):
    return {
        "short": note,
        "long": note + " It installs to your Wii Menu as its own channel and opens without the disc.",
    }


def channel_variant(region, group, tid, url):
    return {"region": region, "region_group": group, "title_id": tid, "zip": url, "price": 0}


def channel_row(slug, name, author, genre, players, release, tid, ver, url, size,
                variants, note):
    return {
        "slug": slug,
        "name": name,
        "author": author,
        "category": "Channels",
        "version": "",
        "licensed": True,
        "price": 0,
        "description": channel_desc(name, note),
        "url": {"icon": "", "zip": url},
        "file_size": {"zip_compressed": size, "zip_uncompressed": 0},
        "shop": {"title_id": tid, "title_version": ver, "contents_size": 0},
        "peripherals": [],
        "release_date": release,
        "platform": "Wii Channel",
        "genre": genre,
        "controllers": ["RvlCtrl"],
        "controller_note": "You can use this software only with the Wii Remote.",
        "players": players,
        "size": size,
        "variants": variants,
        "added": ADDED_CHANNELS,
    }


def apply_channel_extras(apps):
    by_slug = {a["slug"]: a for a in apps}
    by_tid = {}
    for a in apps:
        tid = (a.get("shop") or {}).get("title_id")
        if tid:
            by_tid[tid.upper()] = a

    def find(slug, tid=None):
        if slug in by_slug:
            return by_slug[slug]
        return by_tid.get((tid or "").upper())

    for slug, patch in CHANNEL_FIXES.items():
        a = find(slug)
        if a is None:
            continue
        a.update(patch)
        a["platform"] = "Wii Channel"
        a["description"] = channel_desc(a["name"], a["description"].get("short") or
                                        ("A Wii Shop Channel for " + a["name"] + "."))

    # Photo Channel 1.1 is the World v3 release; the sheet's row points at the
    # Korean build and uses a title ID that belonged to Photo Channel 1.0.
    photo = find("photo-channel-v1-1", "0001000048415A41")
    if photo is not None:
        world = BASE + "/P/Photo%20Channel%20v1.1%20(World)%20(v3)%20(Channel).wad"
        korea = BASE + "/P/Photo%20Channel%20v1.1%20(Korea)%20(v3)%20(Channel).wad"
        photo.update({
            "slug": "photo-channel-v1-1",
            "name": "Photo Channel 1.1",
            "author": "Nintendo",
            "platform": "Wii Channel",
            "genre": "Channel update",
            "players": "For 1-4 players",
            "release_date": 1197331200,
            "size": 8197952,
            "shop": {"title_id": "0001000048415A41", "title_version": 3, "contents_size": 0},
            "url": {"icon": "", "zip": world},
            "file_size": {"zip_compressed": 8197952, "zip_uncompressed": 0},
            "description": channel_desc("Photo Channel 1.1",
                                        "The updated Photo Channel, with support for MP3 music in slideshows."),
            "variants": [
                channel_variant("world", "OTHER", "0001000048415A41", world),
                channel_variant("korea", "KR", "000100024841594b", korea),
            ],
        })

    extras = [
        channel_row(
            "mii-contest-channel", "Mii Contest Channel", "Nintendo", "New Channel",
            "For 1 player", 0, "0001000148415050", 512,
            BASE + "/M/Mii%20Contest%20Channel%20(Europe)%20(v512)%20(Channel).wad", 23085184,
            [channel_variant("europe", "EU", "0001000148415050",
                             BASE + "/M/Mii%20Contest%20Channel%20(Europe)%20(v512)%20(Channel).wad")],
            "Enter Mii contests and vote on other players' creations."),
        channel_row(
            "wii-u-transfer-tool", "Wii U Transfer Tool", "Nintendo", "System Tool",
            "For 1 player", 0, "0001000148435350", 257,
            BASE + "/W/Wii%20U%20Transfer%20Tool%20(Europe)%20(v257)%20(Channel).wad", 16177920,
            [channel_variant("usa", "US", "0001000148435345",
                             BASE + "/W/Wii%20U%20Transfer%20Tool%20(USA)%20(v257)%20(Channel).wad"),
             channel_variant("europe", "EU", "0001000148435350",
                             BASE + "/W/Wii%20U%20Transfer%20Tool%20(Europe)%20(v257)%20(Channel).wad")],
            "Moves your Wii data to a Wii U. It needs a Wii U Transfer Tool on the other console."),
        channel_row(
            "youtube", "YouTube", "Google", "Streaming Video",
            "For 1 player", 0, "0001000148435845", 1536,
            BASE + "/Y/YouTube%20(USA)%20(v1536)%20(Channel).wad", 14666176,
            [channel_variant("usa", "US", "0001000148435845",
                             BASE + "/Y/YouTube%20(USA)%20(v1536)%20(Channel).wad"),
             channel_variant("europe", "EU", "0001000148435850",
                             BASE + "/Y/YouTube%20(Europe)%20(v1537)%20(Channel).wad")],
            "Watch YouTube videos on your TV with the Wii Remote."),
        channel_row(
            "homebrew-channel-open-source-edition",
            "Homebrew Channel - Open Source Edition", "Team Twiizers", "Homebrew",
            "For 1 player", 1742947200, "000100014F484243", 0,
            "https://github.com/FIX94/hbc/releases/download/1.1.4-1/Homebrew.Channel.-.OHBC.wad", 1642240,
            [channel_variant("world", "OTHER", "000100014F484243",
                             "https://github.com/FIX94/hbc/releases/download/1.1.4-1/Homebrew.Channel.-.OHBC.wad")],
            "The open-source Homebrew Channel, built by the community for easy homebrew launching."),
        channel_row(
            "lovefilm", "LOVEFiLM", "LOVEFiLM", "Movies and TV Shows",
            "For 1 player", 1354492800, "0001000148435750", 0, "", 0,
            [channel_variant("europe", "EU", "0001000148435750", "")],
            "Stream films and TV shows from LOVEFiLM. The channel is listed for completeness;"
            " the WAD is no longer available."),
        channel_row(
            "zelda-save-data-update-channel-2", "Zelda Save Data Update Channel 2",
            "Nintendo", "Channel update", "None", 1324512000, "00010000534F5550", 1,
            BASE + "/_Clean/S/Skyward%20Sword%20Save%20Data%20Update%202%20(Europe)%20(v1)%20(SOUP).wad", 7308416,
            [channel_variant("usa", "US", "00010000534F5545",
                             BASE + "/_Clean/S/Skyward%20Sword%20Save%20Data%20Update%202%20(USA)%20(v1)%20(SOUE).wad"),
             channel_variant("europe", "EU", "00010000534F5550",
                             BASE + "/_Clean/S/Skyward%20Sword%20Save%20Data%20Update%202%20(Europe)%20(v1)%20(SOUP).wad")],
            "The second Skyward Sword save data update, used to fix saves affected by the Song of the Hero bug."),
    ]
    for entry in extras:
        old = find(entry["slug"], entry["shop"]["title_id"])
        if old is None:
            apps.append(entry)
        else:
            old.update(entry)
    return apps


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

    apply_channel_extras(apps)
    print(f"catalog apps generated: {len(apps)}; unmatched: {missing}", file=sys.stderr)
    with open(sys.argv[2], "w", encoding="utf-8") as f:
        json.dump(apps, f, ensure_ascii=False, indent=1)
    print(f"wrote {sys.argv[2]}", file=sys.stderr)
    return 0


if __name__ == "__main__":
    sys.exit(main())
