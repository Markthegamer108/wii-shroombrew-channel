"""Patch the app path and version into forwarder.dol (what the server's synthesizer does per channel).
Usage: python -I patch_path.py in.dol out.dol sd:/apps/name/boot.dol [version]
"""
import sys
MAGIC = b"SHROOMBREW-FWD-CFG\0"
AREA = 192
src, dst, path = sys.argv[1], sys.argv[2], sys.argv[3]
version = sys.argv[4] if len(sys.argv) > 4 else "dev"
data = bytearray(open(src, "rb").read())
n = data.count(MAGIC)
assert n == 1, "expected exactly one config block, found %d" % n
i = data.index(MAGIC) + len(MAGIC)
blob = path.encode("ascii") + b"\0" + version.encode("ascii") + b"\0"
assert len(blob) <= AREA and len(version) < 64, "path/version too long"
data[i:i + AREA] = blob + b"\0" * (AREA - len(blob))
open(dst, "wb").write(data)
print("patched", path, version)
