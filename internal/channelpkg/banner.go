package channelpkg

// The Wii Menu banner for a synthesized channel.
//
// A channel's first content is a 0x40-byte pad, the 0x600-byte IMET header
// (title text for all ten menu languages, plus the sizes of the three files
// below), and a U8 archive holding meta/banner.bin, meta/icon.bin and
// meta/sound.bin. banner.bin and icon.bin are LZ77-compressed U8 archives of
// layout textures; each file is wrapped in an IMD5 checksum block.
//
// The blank layout and icon templates come from the Open Shop Channel's
// BannerGenerator (MIT, Copyright 2024 Open Shop Channel); the art drawn into
// them here is original to Shroombrew. The Kanit typeface is under the SIL
// Open Font License.

import (
	"bytes"
	"crypto/md5"
	_ "embed"
	"encoding/binary"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math"
	"strings"
	"sync"
	"time"
	"unicode/utf16"

	"github.com/wii-tools/arclib"
	"github.com/wii-tools/libtpl"
	"github.com/wii-tools/lzx/lz77"
	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
	xdraw "golang.org/x/image/draw"
)

//go:embed bannerdata/banner.u8
var bannerTemplate []byte

//go:embed bannerdata/icon.u8
var iconTemplate []byte

//go:embed bannerdata/meta.u8
var metaTemplate []byte

//go:embed bannerdata/Kanit-Light.ttf
var kanitLight []byte

const (
	imetSize     = 0x600
	imetPrefix   = 0x40 // zeroes in front of the IMET structure
	imetMagicAt  = 0x40 // IMET magic within the structure
	imetTitlesAt = 0x5c
	imetMD5At    = 0x5f0
	imetLanguages = 10
	imetTitleLen = 42
)

// BannerInfo is everything the channel banner and icon show.
type BannerInfo struct {
	Title       string
	Author      string
	Version     string
	Description string
	Category    string
	ReleaseDate time.Time
	IconPNG     []byte // a 128x48 image is ideal; anything else is scaled
}

var (
	fontOnce  sync.Once
	fontData  *opentype.Font
	fontErr   error
	soundOnce sync.Once
	soundData []byte
)

func loadFont() (*opentype.Font, error) {
	fontOnce.Do(func() {
		fontData, fontErr = opentype.Parse(kanitLight)
	})
	return fontData, fontErr
}

// BuildBanner returns the whole first content of a channel: the padded IMET
// header followed by the meta archive.
func BuildBanner(info BannerInfo) ([]byte, error) {
	title := strings.TrimSpace(info.Title)
	if title == "" {
		return nil, errors.New("channel banner needs a title")
	}

	bannerArc, err := buildBannerArchive(info)
	if err != nil {
		return nil, err
	}
	iconArc, err := buildIconArchive(info)
	if err != nil {
		return nil, err
	}
	sound := bannerSound()

	meta, err := buildMetaArchive(bannerArc, iconArc, sound)
	if err != nil {
		return nil, err
	}
	header := buildIMET(title, len(iconArc), len(bannerArc), len(sound))

	out := make([]byte, imetPrefix+imetSize+len(meta))
	copy(out[imetPrefix:], header)
	copy(out[imetPrefix+imetSize:], meta)
	return out, nil
}

func buildIMET(title string, iconSize, bannerSize, soundSize int) []byte {
	b := make([]byte, imetSize)
	copy(b[imetMagicAt:], "IMET")
	binary.BigEndian.PutUint32(b[0x44:], imetSize)
	binary.BigEndian.PutUint32(b[0x48:], 3)
	binary.BigEndian.PutUint32(b[0x4c:], uint32(iconSize))
	binary.BigEndian.PutUint32(b[0x50:], uint32(bannerSize))
	binary.BigEndian.PutUint32(b[0x54:], uint32(soundSize))

	units := utf16.Encode([]rune(title))
	if len(units) > imetTitleLen {
		units = units[:imetTitleLen]
	}
	for language := 0; language < imetLanguages; language++ {
		at := imetTitlesAt + language*imetTitleLen*2
		for i, unit := range units {
			binary.BigEndian.PutUint16(b[at+i*2:], unit)
		}
	}

	sum := md5.Sum(b) // the hash field is still zero here
	copy(b[imetMD5At:], sum[:])
	return b
}

func buildMetaArchive(bannerArc, iconArc, sound []byte) ([]byte, error) {
	arc, err := arclib.Load(metaTemplate)
	if err != nil {
		return nil, fmt.Errorf("meta template: %w", err)
	}
	dir, err := arc.OpenDir("meta")
	if err != nil {
		return nil, fmt.Errorf("meta template directory: %w", err)
	}
	writeCompressed := func(name string, data []byte) error {
		compressed, err := lz77.Compress(data)
		if err != nil {
			return err
		}
		dir.WriteFile(name, append(imd5Header(compressed), compressed...))
		return nil
	}
	if err = writeCompressed("banner.bin", bannerArc); err != nil {
		return nil, err
	}
	if err = writeCompressed("icon.bin", iconArc); err != nil {
		return nil, err
	}
	dir.WriteFile("sound.bin", append(imd5Header(sound), sound...))
	return arc.Save()
}

func buildBannerArchive(info BannerInfo) ([]byte, error) {
	arc, err := arclib.Load(bannerTemplate)
	if err != nil {
		return nil, fmt.Errorf("banner template: %w", err)
	}
	dir, err := arc.OpenDir("arc")
	if err != nil {
		return nil, err
	}
	images, err := dir.GetDir("timg")
	if err != nil {
		return nil, err
	}

	logo, err := channelIcon(info.IconPNG, info.Title)
	if err != nil {
		return nil, err
	}
	name, err := drawTextTile(info.Title, 335, 35, 30, 13, 29)
	if err != nil {
		return nil, err
	}
	versionText := "Version: " + strings.TrimSpace(info.Version)
	if strings.TrimSpace(info.Version) == "" {
		versionText = "Version: 1.0"
	}
	if author := strings.TrimSpace(info.Author); author != "" {
		versionText += "  |  " + author
	}
	version, err := drawTextTile(versionText, 417, 28, 21, 10, 22)
	if err != nil {
		return nil, err
	}
	description, err := drawTextTile(info.Description, 417, 28, 21, 10, 22)
	if err != nil {
		return nil, err
	}
	released := "Release Date: not available"
	if !info.ReleaseDate.IsZero() {
		released = "Release Date: " + info.ReleaseDate.Format("2006-01-02")
	}
	release, err := drawTextTile(released, 417, 28, 21, 10, 22)
	if err != nil {
		return nil, err
	}
	badge, err := drawBadge(134, 45, "SHROOMBREW")
	if err != nil {
		return nil, err
	}

	for name, data := range map[string][]byte{
		"D_applogo.tpl":        logo,
		"D_appname.tpl":        name,
		"D_author_version.tpl": version,
		"D_shortdesc.tpl":      description,
		"D_files_reldate.tpl":  release,
		"C_badge.tpl":          badge,
	} {
		images.WriteFile(name, data)
	}
	return arc.Save()
}

func buildIconArchive(info BannerInfo) ([]byte, error) {
	arc, err := arclib.Load(iconTemplate)
	if err != nil {
		return nil, fmt.Errorf("icon template: %w", err)
	}
	dir, err := arc.OpenDir("arc")
	if err != nil {
		return nil, err
	}
	images, err := dir.GetDir("timg")
	if err != nil {
		return nil, err
	}
	logo, err := channelIcon(info.IconPNG, info.Title)
	if err != nil {
		return nil, err
	}
	badge, err := drawBadge(100, 42, "SHROOMBREW")
	if err != nil {
		return nil, err
	}
	images.WriteFile("D_applogo.tpl", logo)
	images.WriteFile("osc-badge.tpl", badge)
	return arc.Save()
}

func imd5Header(data []byte) []byte {
	header := make([]byte, 32)
	copy(header, "IMD5")
	binary.BigEndian.PutUint32(header[4:], uint32(len(data)))
	sum := md5.Sum(data)
	copy(header[16:], sum[:])
	return header
}

// channelIcon scales an app's 128x48 icon into the banner, or draws a simple
// Shroombrew tile when the catalog has no icon.
func channelIcon(data []byte, title string) ([]byte, error) {
	dst := image.NewRGBA(image.Rect(0, 0, 128, 48))
	if len(data) > 0 {
		src, err := png.Decode(bytes.NewReader(data))
		if err == nil {
			xdraw.CatmullRom.Scale(dst, dst.Bounds(), src, src.Bounds(), draw.Src, nil)
			return libtpl.ToRGB5A3(dst)
		}
	}
	fillShroombrewTile(dst, title)
	return libtpl.ToRGB5A3(dst)
}

func fillShroombrewTile(dst *image.RGBA, title string) {
	draw.Draw(dst, dst.Bounds(), &image.Uniform{color.RGBA{43, 27, 61, 255}}, image.Point{}, draw.Src)
	for y := 0; y < 48; y++ {
		shade := uint8(18 + y/3)
		row := image.Rect(0, y, 128, y+1)
		draw.Draw(dst, row, &image.Uniform{color.RGBA{43 + shade/3, 27 + shade/4, 61 + shade, 255}}, image.Point{}, draw.Src)
	}
	draw.Draw(dst, image.Rect(0, 43, 128, 48), &image.Uniform{color.RGBA{126, 217, 87, 255}}, image.Point{}, draw.Src)
	label := initials(title)
	if label == "" {
		label = "SB"
	}
	if face, err := fontFace(26); err == nil {
		d := font.Drawer{Dst: dst, Src: image.White, Face: face, Dot: fixed.P(8, 33)}
		d.DrawString(label)
	}
}

func initials(title string) string {
	var out []rune
	for _, r := range strings.Fields(title) {
		if len(out) == 2 {
			break
		}
		out = append(out, []rune(strings.ToUpper(r))[0])
	}
	return string(out)
}

// drawTextTile draws one line of menu text, shrinking it (and finally
// truncating it) until it fits the texture.
func drawTextTile(text string, width, height int, maxSize, minSize, baseline float64) ([]byte, error) {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	text = strings.TrimSpace(text)
	if text != "" {
		size := float64(maxSize)
		for ; size >= float64(minSize); size-- {
			face, err := fontFace(size)
			if err != nil {
				return nil, err
			}
			if measure(face, text) <= float64(width-2) {
				d := font.Drawer{Dst: img, Src: image.White, Face: face, Dot: fixed.P(0, int(baseline))}
				d.DrawString(text)
				break
			}
		}
		if size < float64(minSize) {
			face, err := fontFace(float64(minSize))
			if err != nil {
				return nil, err
			}
			d := font.Drawer{Dst: img, Src: image.White, Face: face, Dot: fixed.P(0, int(baseline))}
			d.DrawString(truncate(face, text, float64(width-6)))
		}
	}
	return libtpl.ToIA4(img)
}

func fontFace(size float64) (font.Face, error) {
	f, err := loadFont()
	if err != nil {
		return nil, err
	}
	return opentype.NewFace(f, &opentype.FaceOptions{Size: size, DPI: 72, Hinting: font.HintingFull})
}

func measure(face font.Face, text string) float64 {
	return float64(font.MeasureString(face, text)) / 64
}

func truncate(face font.Face, text string, maxWidth float64) string {
	runes := []rune(text)
	ellipsis := "..."
	for len(runes) > 0 && measure(face, string(runes)+ellipsis) > maxWidth {
		runes = runes[:len(runes)-1]
	}
	return string(runes) + ellipsis
}

// drawBadge draws Shroombrew's own corner badge: a dark tile with a green
// strip and the shop name. No third-party badge artwork is reused.
func drawBadge(width, height int, label string) ([]byte, error) {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		t := float64(y) / float64(height)
		c := color.RGBA{
			R: uint8(38 + 18*t),
			G: uint8(24 + 10*t),
			B: uint8(56 + 30*t),
			A: 255,
		}
		draw.Draw(img, image.Rect(0, y, width, y+1), &image.Uniform{c}, image.Point{}, draw.Src)
	}
	draw.Draw(img, image.Rect(0, height-4, width, height), &image.Uniform{color.RGBA{126, 217, 87, 255}}, image.Point{}, draw.Src)
	size := 18.0
	if width < 120 {
		size = 14
	}
	face, err := fontFace(size)
	if err != nil {
		return nil, err
	}
	text := label
	for measure(face, text) > float64(width-8) && size > 8 {
		size--
		face, err = fontFace(size)
		if err != nil {
			return nil, err
		}
	}
	d := font.Drawer{Dst: img, Src: image.White, Face: face, Dot: fixed.P(4, height/2+int(size)/3+1)}
	d.DrawString(text)
	return libtpl.ToRGB5A3(img)
}

// bannerSound is a short original two-note chime, rendered once as 16-bit
// stereo PCM in a RIFF WAVE container.
func bannerSound() []byte {
	soundOnce.Do(func() { soundData = renderChime() })
	return soundData
}

func renderChime() []byte {
	const (
		rate     = 44100
		channels = 2
		seconds  = 0.9
	)
	samples := int(rate * seconds)
	pcm := make([]byte, samples*channels*2)
	notes := []struct {
		start, length float64
		freq          float64
	}{
		{0.00, 0.42, 659.255}, // E5
		{0.30, 0.58, 987.767}, // B5
	}
	for i := 0; i < samples; i++ {
		t := float64(i) / rate
		var value float64
		for _, note := range notes {
			if t < note.start || t >= note.start+note.length {
				continue
			}
			u := (t - note.start) / note.length
			envelope := math.Sin(math.Pi * math.Min(1, u*1.15))
			envelope *= math.Exp(-2.6 * u)
			value += envelope * (math.Sin(2*math.Pi*note.freq*t) + 0.25*math.Sin(2*math.Pi*note.freq*2*t))
		}
		value = math.Max(-1, math.Min(1, value*0.22))
		s := int16(value * 32767)
		for channel := 0; channel < channels; channel++ {
			at := (i*channels + channel) * 2
			binary.LittleEndian.PutUint16(pcm[at:], uint16(s))
		}
	}
	var out bytes.Buffer
	out.WriteString("RIFF")
	binary.Write(&out, binary.LittleEndian, uint32(36+len(pcm)))
	out.WriteString("WAVEfmt ")
	binary.Write(&out, binary.LittleEndian, uint32(16))
	binary.Write(&out, binary.LittleEndian, uint16(1))
	binary.Write(&out, binary.LittleEndian, uint16(channels))
	binary.Write(&out, binary.LittleEndian, uint32(rate))
	binary.Write(&out, binary.LittleEndian, uint32(rate*channels*2))
	binary.Write(&out, binary.LittleEndian, uint16(channels*2))
	binary.Write(&out, binary.LittleEndian, uint16(16))
	out.WriteString("data")
	binary.Write(&out, binary.LittleEndian, uint32(len(pcm)))
	out.Write(pcm)
	return out.Bytes()
}
