"""Builds a package larger than the old 32 MB limit for the forwarder streaming test.

  python -I make_bigzip.py out.zip testapp.dol manifest.json

The manifest records the SHA-256 and size of every file the forwarder must write, so
verify_sd.py can check the card image after the run.
"""
import hashlib, json, os, sys, zipfile

out, dol, manifest = sys.argv[1], sys.argv[2], sys.argv[3]
rng = os.urandom
files = {}


def add(z, name, data, method):
    z.writestr(zipfile.ZipInfo(name, (2026, 11, 4, 12, 0, 0)), data, compress_type=method)
    files[name] = {"size": len(data), "sha256": hashlib.sha256(data).hexdigest()}


with zipfile.ZipFile(out, "w", allowZip64=False) as z:
    add(z, "apps/fwdtest/boot.dol", open(dol, "rb").read(), zipfile.ZIP_DEFLATED)
    add(z, "apps/fwdtest/data/random60.bin", rng(60 * 1024 * 1024), zipfile.ZIP_DEFLATED)  # barely compresses
    add(z, "apps/fwdtest/data/zeros40.bin", bytes(40 * 1024 * 1024), zipfile.ZIP_DEFLATED)  # compresses to nothing
    add(z, "apps/fwdtest/stored5.bin", rng(5 * 1024 * 1024), zipfile.ZIP_STORED)
    add(z, "apps/fwdtest/empty.txt", b"", zipfile.ZIP_STORED)
    z.writestr(zipfile.ZipInfo("apps/fwdtest/emptydir/", (2026, 11, 4, 12, 0, 0)), b"")
    z.writestr("README.txt", "outside apps/: must be ignored")
json.dump(files, open(manifest, "w"), indent=1)
print(out, os.path.getsize(out), "bytes;", sum(f["size"] for f in files.values()), "bytes once unpacked")
