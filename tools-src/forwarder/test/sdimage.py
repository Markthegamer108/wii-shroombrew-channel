"""Build and read Dolphin SD card images (superfloppy FAT32) without Dolphin's folder sync.
  python -I sdimage.py make  image.raw  folder  blank.raw   # copy folder contents into a copy of a blank Dolphin image
  python -I sdimage.py read  image.raw  /path/in/sd # print a file from the image
  python -I sdimage.py list  image.raw              # list every file
"""
import os, sys
from pyfatfs.PyFat import PyFat
from pyfatfs.PyFatFS import PyFatFS

SIZE = 128 * 1024 * 1024


def make(image, folder, template):
    """pyfatfs cannot format FAT32 reliably, so start from a blank image Dolphin made itself."""
    import shutil
    if os.path.exists(image):
        os.remove(image)
    shutil.copyfile(template, image)
    fs = PyFatFS(image)
    n = 0
    for root, dirs, files in os.walk(folder):
        rel = os.path.relpath(root, folder).replace("\\", "/")
        base = "/" if rel == "." else "/" + rel
        for d in dirs:
            fs.makedirs(base.rstrip("/") + "/" + d, recreate=True)
        for f in files:
            with open(os.path.join(root, f), "rb") as src:
                fs.writebytes(base.rstrip("/") + "/" + f, src.read())
            n += 1
    fs.close()
    print("image ready: %d files" % n)


def read(image, path):
    fs = PyFatFS(image)
    try:
        sys.stdout.write(fs.readbytes(path).decode("utf-8", "replace"))
    finally:
        fs.close()


def lst(image):
    fs = PyFatFS(image)
    for p in fs.walk.files():
        print(p)
    fs.close()


if __name__ == "__main__":
    {"make": lambda: make(sys.argv[2], sys.argv[3], sys.argv[4]), "read": lambda: read(sys.argv[2], sys.argv[3]), "list": lambda: lst(sys.argv[2])}[sys.argv[1]]()
