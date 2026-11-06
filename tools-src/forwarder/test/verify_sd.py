"""Checks a Dolphin SD image against the manifest from make_bigzip.py.

  python -I verify_sd.py image.raw manifest.json
"""
import hashlib, json, sys
from pyfatfs.PyFatFS import PyFatFS

image, manifest = sys.argv[1], sys.argv[2]
want = json.load(open(manifest))
fs = PyFatFS(image)


def find(path):
    """FAT names are case-insensitive; the library's are not. Returns the real path or None."""
    cur = "/"
    for part in [p for p in path.split("/") if p]:
        hit = [n for n in fs.listdir(cur) if n.lower() == part.lower()]
        if not hit:
            return None
        cur = cur.rstrip("/") + "/" + hit[0]
    return cur


bad = 0
for name, meta in sorted(want.items()):
    path = find("/" + name) or "/" + name
    try:
        h, n = hashlib.sha256(), 0
        with fs.openbin(path, "r") as f:
            while True:
                b = f.read(1 << 20)
                if not b:
                    break
                h.update(b)
                n += len(b)
        ok = n == meta["size"] and h.hexdigest() == meta["sha256"]
    except Exception as e:
        ok, n = False, "missing (%s)" % type(e).__name__
    print(("ok   " if ok else "FAIL ") + name, n)
    bad += 0 if ok else 1
print("README.txt (outside apps/) written:", find("/README.txt") is not None)
d = find("/apps/fwdtest/emptydir")
print("empty dir exists:", bool(d and fs.isdir(d)))
m = find("/apps/fwdtest/.shroombrew-version")
print("marker:", fs.readtext(m) if m else "MISSING")
sys.exit(1 if bad else 0)
