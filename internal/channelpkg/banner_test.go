package channelpkg

import (
	"bytes"
	"crypto/md5"
	"encoding/binary"
	"image"
	"image/color"
	"image/png"
	"testing"
	"time"
	"unicode/utf16"

	"github.com/wii-tools/arclib"
	"github.com/wii-tools/lzx/lz77"
)

func testIcon(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 128, 48))
	for y := 0; y < 48; y++ {
		for x := 0; x < 128; x++ {
			img.Set(x, y, color.RGBA{uint8(x * 2), uint8(y * 5), 0xc0, 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestBuildBannerLayout(t *testing.T) {
	const title = "Demo App"
	data, err := BuildBanner(BannerInfo{
		Title:       title,
		Author:      "Shroombrew",
		Version:     "1.2.3",
		Description: "A small test application",
		ReleaseDate: time.Date(2024, 5, 6, 0, 0, 0, 0, time.UTC),
		IconPNG:     testIcon(t),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(data) <= 0x640 {
		t.Fatalf("banner is only %d bytes", len(data))
	}
	if got := string(data[0x80:0x84]); got != "IMET" {
		t.Fatalf("magic at 0x80 = %q", got)
	}
	if hashSize := binary.BigEndian.Uint32(data[0x84:]); hashSize != 0x600 {
		t.Fatalf("hash size = %#x", hashSize)
	}
	if version := binary.BigEndian.Uint32(data[0x88:]); version != 3 {
		t.Fatalf("version = %d", version)
	}
	iconSize := binary.BigEndian.Uint32(data[0x8c:])
	bannerSize := binary.BigEndian.Uint32(data[0x90:])
	soundSize := binary.BigEndian.Uint32(data[0x94:])

	header := make([]byte, imetSize)
	copy(header, data[0x40:0x640])
	copy(header[0x5f0:], make([]byte, 16))
	want := md5.Sum(header)
	if !bytes.Equal(data[0x630:0x640], want[:]) {
		t.Fatal("IMET MD5 does not match")
	}

	units := utf16.Encode([]rune(title))
	for language := 0; language < 10; language++ {
		at := 0x40 + 0x5c + language*84
		got := data[at : at+len(units)*2]
		for i := 0; i < len(units); i++ {
			if binary.BigEndian.Uint16(got[i*2:]) != units[i] {
				t.Fatalf("language %d title is wrong", language)
			}
		}
	}

	meta, err := arclib.Load(data[0x640:])
	if err != nil {
		t.Fatalf("meta archive: %v", err)
	}
	dir, err := meta.OpenDir("meta")
	if err != nil {
		t.Fatalf("meta directory: %v", err)
	}
	check := func(name string) []byte {
		t.Helper()
		file, err := dir.GetFile(name)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if len(file.Data) < 32 || string(file.Data[:4]) != "IMD5" {
			t.Fatalf("%s has no IMD5 header", name)
		}
		if got := binary.BigEndian.Uint32(file.Data[4:]); got != uint32(len(file.Data)-32) {
			t.Fatalf("%s IMD5 size = %d, file holds %d", name, got, len(file.Data)-32)
		}
		return file.Data[32:]
	}
	bannerCompressed := check("banner.bin")
	iconCompressed := check("icon.bin")
	sound := check("sound.bin")
	if len(sound) < 12 || string(sound[:4]) != "RIFF" || string(sound[8:12]) != "WAVE" {
		t.Fatalf("sound is not a RIFF WAVE")
	}
	if uint32(len(sound)) != soundSize {
		t.Fatalf("sound size = %d, IMET says %d", len(sound), soundSize)
	}
	for name, compressed := range map[string][]byte{"banner.bin": bannerCompressed, "icon.bin": iconCompressed} {
		if !bytes.HasPrefix(compressed, []byte("LZ77")) {
			t.Fatalf("%s does not start with LZ77", name)
		}
		raw, err := lz77.Decompress(compressed[4:])
		if err != nil {
			t.Fatalf("%s decompress: %v", name, err)
		}
		wantSize := bannerSize
		if name == "icon.bin" {
			wantSize = iconSize
		}
		if uint32(len(raw)) != wantSize {
			t.Fatalf("%s decompresses to %d, IMET says %d", name, len(raw), wantSize)
		}
		arc, err := arclib.Load(raw)
		if err != nil {
			t.Fatalf("%s archive: %v", name, err)
		}
		arcDir, err := arc.OpenDir("arc")
		if err != nil {
			t.Fatal(err)
		}
		timg, err := arcDir.GetDir("timg")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := timg.GetFile("D_applogo.tpl"); err != nil {
			t.Fatalf("%s has no app icon: %v", name, err)
		}
	}
}

func TestTruncateFits(t *testing.T) {
	face, err := fontFace(21)
	if err != nil {
		t.Fatal(err)
	}
	got := truncate(face, "This description is deliberately much too long to fit in a 417 pixel wide line of text", 411)
	if measure(face, got) > 411 {
		t.Fatalf("truncated text is still %0.1f wide: %q", measure(face, got), got)
	}
}
