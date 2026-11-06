"""Draws the Shroombrew favicon: a mushroom in the Wii Shop's cyan.

  python -I make_favicon.py favicon.ico [preview.png]

Original artwork. It is drawn at 1024 px and reduced for each size so the small
sizes stay clean.
"""
import sys
from PIL import Image, ImageDraw

BIG = 1024
CAP = (52, 190, 237, 255)       # #34beed, the shop's cyan
CAP_DARK = (22, 140, 190, 255)
CAP_LIGHT = (140, 224, 250, 255)
STEM = (252, 252, 248, 255)
STEM_SHADE = (205, 222, 232, 255)
SPOT = (255, 255, 255, 255)
LINE = (20, 96, 132, 255)


def draw():
    im = Image.new("RGBA", (BIG, BIG), (0, 0, 0, 0))
    d = ImageDraw.Draw(im)
    w = 34  # outline width

    # stem
    d.rounded_rectangle((340, 520, 684, 930), radius=120, fill=STEM, outline=LINE, width=w)
    d.rounded_rectangle((520, 560, 650, 900), radius=60, fill=STEM_SHADE)       # shade on the right
    d.rounded_rectangle((340, 520, 684, 930), radius=120, outline=LINE, width=w)

    # cap: a dome with a slightly flared rim
    d.pieslice((70, 90, 954, 900), 180, 360, fill=CAP, outline=LINE, width=w)
    d.rounded_rectangle((70, 470, 954, 580), radius=55, fill=CAP, outline=LINE, width=w)
    d.pieslice((140, 160, 884, 760), 180, 360, fill=CAP)                        # re-fill inside the outline
    d.rounded_rectangle((90, 490, 934, 560), radius=40, fill=CAP_DARK)          # rim shading
    d.pieslice((210, 170, 560, 420), 190, 300, fill=CAP_LIGHT)                  # highlight

    # spots
    for cx, cy, r in ((512, 250, 82), (300, 400, 60), (724, 392, 66)):
        d.ellipse((cx - r, cy - r, cx + r, cy + r), fill=SPOT, outline=CAP_DARK, width=10)
    return im


def main():
    out = sys.argv[1]
    big = draw()
    sizes = [16, 24, 32, 48, 64]
    frames = [big.resize((s, s), Image.LANCZOS) for s in sizes]
    frames[-1].save(out, format="ICO", sizes=[(s, s) for s in sizes], append_images=frames[:-1])
    if len(sys.argv) > 2:
        sheet = Image.new("RGBA", (256 + 40 + 128 + 20 + 64 + 20 + 32 + 20 + 16 + 20, 280), (255, 255, 255, 255))
        x = 10
        for s in (256, 128, 64, 32, 16):
            sheet.paste(big.resize((s, s), Image.LANCZOS), (x, 10), big.resize((s, s), Image.LANCZOS))
            x += s + 20
        sheet.save(sys.argv[2])
    print("wrote", out)


main()
