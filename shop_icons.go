package main

// Catalog thumbnails. The archived shop keeps a title's artwork among the
// numbered files in site/oss/ccs/<titleId>/. The number is not fixed: for
// many titles FFFD0000 is the manual ZIP and the icon is FFFD0001 (or later),
// so the resolver scans the folder and picks the best icon image instead of
// trusting one filename. A title with no archived artwork falls back to a
// regional variant, then to another release of the same title, and finally to
// a generated tile, so no catalog row shows a broken image.

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

type shopIcon struct {
	path string
	mime string
}

// iconsDir is where pre-rendered homebrew PNGs live.
func (s *storefront) iconsDir() string {
	if s != nil && s.iconsRoot != "" {
		return s.iconsRoot
	}
	return "./icons"
}

// shopIconFor resolves the thumbnail for app, memoized per storefront. ok is
// false when the catalog has no archived artwork, in which case callers can
// use placeholderIcon.
func (s *storefront) shopIconFor(app catalogApp) (shopIcon, bool) {
	if s != nil {
		s.iconMu.Lock()
		if s.iconCache == nil {
			s.iconCache = make(map[string]shopIcon)
		}
		if ic, ok := s.iconCache[app.Slug]; ok {
			s.iconMu.Unlock()
			return ic, ic.path != ""
		}
		s.iconMu.Unlock()
	}
	ic, ok := s.resolveShopIcon(app)
	if s != nil {
		s.iconMu.Lock()
		s.iconCache[app.Slug] = ic
		s.iconMu.Unlock()
	}
	return ic, ok
}

func (s *storefront) resolveShopIcon(app catalogApp) (shopIcon, bool) {
	if ic, ok := s.ownShopIcon(app); ok {
		return ic, true
	}
	if !app.Licensed {
		return shopIcon{}, false
	}
	for _, v := range app.Variants {
		if ic, ok := ccsShopIcon(v.TitleID); ok {
			return ic, true
		}
	}
	// Some releases share a title with a sibling that does have artwork
	// (region splits, bundled/standalone versions, base game and add-on).
	key := iconNameKey(app.Name)
	for _, other := range s.apps {
		if other.Slug == app.Slug || iconNameKey(other.Name) != key {
			continue
		}
		if ic, ok := s.ownShopIcon(other); ok {
			return ic, true
		}
	}
	return shopIcon{}, false
}

// ownShopIcon is a title's own artwork: its homebrew PNG, or the best image
// in its archived shop folder.
func (s *storefront) ownShopIcon(app catalogApp) (shopIcon, bool) {
	png := filepath.Join(s.iconsDir(), app.Slug+".png")
	if info, err := os.Stat(png); err == nil && !info.IsDir() && info.Size() > 0 {
		return shopIcon{path: png, mime: "image/png"}, true
	}
	if app.Licensed {
		return ccsShopIcon(classicTitleIDForApp(app))
	}
	return shopIcon{}, false
}

// shopCCSRoot is the archived shop contents tree; a variable so tests can
// point it at a fixture.
var shopCCSRoot = filepath.Join("site", "oss", "ccs")

// ccsShopIcon picks the best icon image from a title's archived shop folder.
// Manuals are ZIPs and screenshots can be much larger, so candidates are
// scored: an actual 96x72 (or 160x120) GIF wins, then other small images.
func ccsShopIcon(titleID string) (shopIcon, bool) {
	return ccsShopIconAt(shopCCSRoot, titleID)
}

func ccsShopIconAt(root, titleID string) (shopIcon, bool) {
	id := strings.ToUpper(strings.TrimSpace(titleID))
	if len(id) != 16 {
		return shopIcon{}, false
	}
	dir := filepath.Join(root, id)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return shopIcon{}, false
	}
	var best shopIcon
	bestScore, bestSize := -1, int64(0)
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		info, err := e.Info()
		if err != nil || info.Size() == 0 || info.Size() > 200<<10 {
			continue
		}
		p := filepath.Join(dir, e.Name())
		f, err := os.Open(p)
		if err != nil {
			continue
		}
		var head [64]byte
		n, _ := io.ReadFull(f, head[:])
		f.Close()
		if n < 12 {
			continue
		}
		mime, w, h, ok := iconImageKind(head[:n])
		if !ok {
			continue
		}
		score := 0
		switch mime {
		case "image/gif":
			score += 40
		case "image/png":
			score += 30
		case "image/jpeg":
			score += 20
		default:
			score += 10
		}
		switch {
		case w == 96 && h == 72:
			score += 50
		case w == 160 && h == 120:
			score += 40
		case w > 0 && h > 0 && float64(w)/float64(h) >= 0.9 && float64(w)/float64(h) <= 1.8:
			score += 10
		}
		if info.Size() <= 30<<10 {
			score += 5
		}
		if score > bestScore || (score == bestScore && info.Size() < bestSize) {
			best = shopIcon{path: p, mime: mime}
			bestScore, bestSize = score, info.Size()
		}
	}
	return best, bestScore >= 0
}

// iconImageKind recognizes the image formats the shop archived, returning its
// MIME type and pixel size. Debugging headers are cheap and let the resolver
// ignore ZIP manuals disguised as icons.
func iconImageKind(b []byte) (mime string, w, h int, ok bool) {
	switch {
	case len(b) >= 10 && (string(b[:6]) == "GIF89a" || string(b[:6]) == "GIF87a"):
		return "image/gif", int(binary.LittleEndian.Uint16(b[6:8])), int(binary.LittleEndian.Uint16(b[8:10])), true
	case len(b) >= 24 && bytes.Equal(b[:8], []byte("\x89PNG\r\n\x1a\n")):
		return "image/png", int(binary.BigEndian.Uint32(b[16:20])), int(binary.BigEndian.Uint32(b[20:24])), true
	case len(b) >= 26 && b[0] == 'B' && b[1] == 'M':
		w = int(int32(binary.LittleEndian.Uint32(b[18:22])))
		h = int(int32(binary.LittleEndian.Uint32(b[22:26])))
		if w < 0 {
			w = -w
		}
		if h < 0 {
			h = -h
		}
		return "image/bmp", w, h, true
	case len(b) >= 4 && b[0] == 0xFF && b[1] == 0xD8:
		w, h = jpegIconSize(b)
		return "image/jpeg", w, h, true
	}
	return "", 0, 0, false
}

// jpegIconSize finds the SOF frame header in the first bytes of a JPEG.
func jpegIconSize(b []byte) (int, int) {
	i := 2
	for i+3 < len(b) {
		if b[i] != 0xFF {
			i++
			continue
		}
		for i < len(b) && b[i] == 0xFF {
			i++
		}
		if i >= len(b) {
			return 0, 0
		}
		marker := b[i]
		i++
		if marker == 0xD8 || marker == 0xD9 || (marker >= 0xD0 && marker <= 0xD7) {
			continue
		}
		if i+1 >= len(b) {
			return 0, 0
		}
		segLen := int(binary.BigEndian.Uint16(b[i : i+2]))
		if segLen < 2 {
			return 0, 0
		}
		if marker >= 0xC0 && marker <= 0xCF && marker != 0xC4 && marker != 0xC8 && marker != 0xCC {
			if i+7 > len(b) {
				return 0, 0
			}
			h := int(binary.BigEndian.Uint16(b[i+3 : i+5]))
			w := int(binary.BigEndian.Uint16(b[i+5 : i+7]))
			return w, h
		}
		i += segLen
	}
	return 0, 0
}

// iconNameKey matches releases of the same title across punctuation and
// symbols, mirroring the welcome page's name matching.
func iconNameKey(name string) string {
	n := strings.NewReplacer("™", "", "®", "", "©", "").Replace(name)
	n = strings.ReplaceAll(strings.ToLower(n), "&", " and ")
	return strings.TrimSpace(welcomeNonAlnum.ReplaceAllString(n, " "))
}

// ---------------------------------------------------------------------------
// Generated placeholder tiles

var (
	placeholderFaceOnce sync.Once
	placeholderFace     font.Face
	placeholderCache    sync.Map // slug -> []byte
)

func placeholderIcon(app catalogApp) []byte {
	if v, ok := placeholderCache.Load(app.Slug); ok {
		return v.([]byte)
	}
	data := drawPlaceholder(app)
	placeholderCache.Store(app.Slug, data)
	return data
}

// drawPlaceholder paints a Wii-shop-blue tile with the title's initial, so a
// title without archived art still looks deliberate rather than broken.
func drawPlaceholder(app catalogApp) []byte {
	const w, h = 192, 144
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	top, bottom := placeholderColors(app)
	for y := 0; y < h; y++ {
		t := float64(y) / float64(h-1)
		c := color.RGBA{
			R: uint8(float64(top.R)*(1-t) + float64(bottom.R)*t),
			G: uint8(float64(top.G)*(1-t) + float64(bottom.G)*t),
			B: uint8(float64(top.B)*(1-t) + float64(bottom.B)*t),
			A: 255,
		}
		draw.Draw(img, image.Rect(0, y, w, y+1), &image.Uniform{c}, image.Point{}, draw.Src)
	}
	frame := color.RGBA{255, 255, 255, 72}
	for _, r := range []image.Rectangle{
		image.Rect(4, 4, w-4, 6),
		image.Rect(4, h-6, w-4, h-4),
		image.Rect(4, 4, 6, h-4),
		image.Rect(w-6, 4, w-4, h-4),
	} {
		draw.Draw(img, r, &image.Uniform{frame}, image.Point{}, draw.Over)
	}
	if face := placeholderFaceFor(); face != nil {
		if letter := iconLetter(app); letter != "" {
			drawer := &font.Drawer{Dst: img, Src: image.NewUniform(color.NRGBA{0, 0, 0, 90})}
			drawer.Face = face
			width := drawer.MeasureString(letter).Ceil()
			ascent := face.Metrics().Ascent.Ceil()
			descent := face.Metrics().Descent.Ceil()
			x := (w - width) / 2
			y := (h-(ascent+descent))/2 + ascent
			drawer.Dot = fixed.P(x+2, y+2)
			drawer.DrawString(letter)
			drawer.Src = image.NewUniform(color.White)
			drawer.Dot = fixed.P(x, y)
			drawer.DrawString(letter)
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil
	}
	return buf.Bytes()
}

func placeholderFaceFor() font.Face {
	placeholderFaceOnce.Do(func() {
		f, err := opentype.Parse(gobold.TTF)
		if err != nil {
			return
		}
		placeholderFace, _ = opentype.NewFace(f, &opentype.FaceOptions{Size: 80, DPI: 72, Hinting: font.HintingFull})
	})
	return placeholderFace
}

// iconLetter is the first ASCII letter or digit of the title, the mark the
// placeholder shows.
func iconLetter(app catalogApp) string {
	for _, r := range strings.ToUpper(app.Name) {
		if (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			return string(r)
		}
	}
	for _, r := range strings.ToUpper(app.Category) {
		if (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			return string(r)
		}
	}
	return "?"
}

func placeholderColors(app catalogApp) (color.RGBA, color.RGBA) {
	switch strings.ToLower(app.Category) {
	case "wiiware":
		return color.RGBA{0xE8, 0x7D, 0x2E, 255}, color.RGBA{0x8A, 0x40, 0x12, 255}
	case "virtual console":
		return color.RGBA{0x3D, 0x8F, 0xD1, 255}, color.RGBA{0x1B, 0x4F, 0x72, 255}
	case "channels":
		return color.RGBA{0x35, 0xA8, 0x5C, 255}, color.RGBA{0x17, 0x5A, 0x32, 255}
	case "dlc":
		return color.RGBA{0x8E, 0x5B, 0xB8, 255}, color.RGBA{0x4A, 0x2B, 0x63, 255}
	}
	return color.RGBA{0x4A, 0x90, 0xC4, 255}, color.RGBA{0x1F, 0x3F, 0x5E, 255}
}
