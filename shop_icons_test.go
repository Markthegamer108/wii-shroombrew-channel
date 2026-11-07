package main

import (
	"bytes"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

// gifWithSize writes a GIF of the given size and returns its bytes.
func gifWithSize(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewPaletted(image.Rect(0, 0, w, h), color.Palette{color.Black, color.White})
	var buf bytes.Buffer
	if err := gif.Encode(&buf, img, nil); err != nil {
		t.Fatalf("encode gif: %v", err)
	}
	return buf.Bytes()
}

// TestCCSIconSkipsManualZIP is the regression that broke many catalog
// thumbnails: FFFD0000 is often the manual ZIP, and the icon is FFFD0001.
func TestCCSIconSkipsManualZIP(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "0001000158494745")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "FFFD0000"), []byte("PK\x03\x04manual"), 0o644); err != nil {
		t.Fatal(err)
	}
	icon := gifWithSize(t, 96, 72)
	if err := os.WriteFile(filepath.Join(dir, "FFFD0001"), icon, 0o644); err != nil {
		t.Fatal(err)
	}
	got, ok := ccsShopIconAt(root, "0001000158494745")
	if !ok {
		t.Fatal("no icon found")
	}
	if filepath.Base(got.path) != "FFFD0001" || got.mime != "image/gif" {
		t.Fatalf("picked %q (%s), want FFFD0001 (image/gif)", got.path, got.mime)
	}
}

// TestCCSIconPrefersSmallIconOverLargeArtwork checks the scoring skips large
// screenshots in favour of the real 96x72 icon.
func TestCCSIconPrefersSmallIconOverLargeArtwork(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "0001000148415A41")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	big := image.NewRGBA(image.Rect(0, 0, 640, 480))
	for i := range big.Pix {
		big.Pix[i] = 0x40
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, big, nil); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "FFFD0000"), buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "FFFD0001"), gifWithSize(t, 160, 120), 0o644); err != nil {
		t.Fatal(err)
	}
	got, ok := ccsShopIconAt(root, "0001000148415A41")
	if !ok {
		t.Fatal("no icon found")
	}
	if filepath.Base(got.path) != "FFFD0001" {
		t.Fatalf("picked %q, want FFFD0001", got.path)
	}
}

func TestIconImageKind(t *testing.T) {
	gif := gifWithSize(t, 96, 72)
	if mime, w, h, ok := iconImageKind(gif); !ok || mime != "image/gif" || w != 96 || h != 72 {
		t.Fatalf("gif: %s %dx%d %v", mime, w, h, ok)
	}
	var pbytes bytes.Buffer
	if err := png.Encode(&pbytes, image.NewRGBA(image.Rect(0, 0, 160, 120))); err != nil {
		t.Fatal(err)
	}
	if mime, w, h, ok := iconImageKind(pbytes.Bytes()); !ok || mime != "image/png" || w != 160 || h != 120 {
		t.Fatalf("png: %s %dx%d %v", mime, w, h, ok)
	}
	var jbytes bytes.Buffer
	if err := jpeg.Encode(&jbytes, image.NewRGBA(image.Rect(0, 0, 160, 120)), nil); err != nil {
		t.Fatal(err)
	}
	if mime, w, h, ok := iconImageKind(jbytes.Bytes()); !ok || mime != "image/jpeg" || w != 160 || h != 120 {
		t.Fatalf("jpeg: %s %dx%d %v", mime, w, h, ok)
	}
	if _, _, _, ok := iconImageKind([]byte("PK\x03\x04not an image")); ok {
		t.Fatal("zip detected as an image")
	}
}

func TestPlaceholderIconIsPNG(t *testing.T) {
	app := catalogApp{Slug: "test-placeholder", Name: "\"Aha! I Got It!\" Escape Game", Category: "WiiWare"}
	data := placeholderIcon(app)
	if len(data) == 0 {
		t.Fatal("placeholder is empty")
	}
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("placeholder is not a PNG: %v", err)
	}
	if b := img.Bounds(); b.Dx() != 192 || b.Dy() != 144 {
		t.Fatalf("placeholder size %dx%d, want 192x144", b.Dx(), b.Dy())
	}
	// The letter is taken from the first alphanumeric character.
	if got := iconLetter(app); got != "A" {
		t.Fatalf("iconLetter = %q, want A", got)
	}
}
