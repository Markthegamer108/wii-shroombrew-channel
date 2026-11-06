// Package flashpkg turns an Adobe Flash SWF into an installable Wii channel by
// injecting it into an operator-supplied official Flash runtime WAD.
//
// The runtime Nintendo shipped in WiiWare channels (Back to Nature and the
// "Flash Placeholder") is a Flash Lite-class player: ActionScript 1/2 and SWF
// version 9 at most. ActionScript 3 hangs the console, so every upload is
// checked before a WAD is built.
//
// Neither the base WAD nor the Wii common key is distributed with Shroombrew.
// The operator supplies the base once; this package only replaces the SWF,
// assets, config and banner inside it.
package flashpkg

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"strings"
)

const (
	// MaxSWF bounds one uploaded movie. WiiMart's largest Flash channels are
	// about 40-44 MB including the runtime, assets and banner.
	MaxSWF = 40 << 20

	// WarnStageWidth and WarnStageHeight are SWF-Checker's limits. Larger
	// stages are accepted but may clip on a 4:3 television.
	WarnStageWidth  = 700
	WarnStageHeight = 500

	maxTags = 1 << 20
)

// Info describes the parts of a SWF that matter to the Wii runtime.
type Info struct {
	Signature     string  // FWS or CWS
	Version       uint8   // SWF file format version
	Compressed    bool    // CWS (zlib)
	Width         int     // stage width in pixels
	Height        int     // stage height in pixels
	FrameRate     float64 // frames per second
	FrameCount    int
	ActionScript3 bool   // rejected by Validate
	Background    string // "#rrggbb" when SetBackgroundColor is present
	FileLength    int    // declared uncompressed length
}

// Warning is a non-fatal problem worth showing before publishing a channel.
type Warning struct {
	Code    string
	Message string
}

// Validate checks that a SWF can run in the Wii's native Flash player.
// It returns the parsed header, non-fatal warnings, and a fatal error when the
// movie cannot be used.
func Validate(data []byte) (Info, []Warning, error) {
	info, body, err := parseHeader(data)
	if err != nil {
		return info, nil, err
	}
	if info.Version > 9 {
		return info, nil, fmt.Errorf("this SWF is version %d; the Wii player supports version 9 and older", info.Version)
	}
	if err := parseBody(body, &info); err != nil {
		return info, nil, err
	}
	if info.ActionScript3 {
		return info, nil, errors.New("this SWF uses ActionScript 3, which hangs the Wii's Flash player; only ActionScript 1 or 2 movies work")
	}
	return info, warningsFor(info), nil
}

func warningsFor(info Info) []Warning {
	var out []Warning
	if info.Version == 9 {
		out = append(out, Warning{
			Code:    "version-9",
			Message: "Flash 9 movies only run reliably in some Wii Flash bases; test this one on a console before publishing it",
		})
	}
	if info.Width > WarnStageWidth || info.Height > WarnStageHeight {
		out = append(out, Warning{
			Code: "stage-too-large",
			Message: fmt.Sprintf("the stage is %dx%d; above %dx%d it may clip or overlap on a 4:3 TV",
				info.Width, info.Height, WarnStageWidth, WarnStageHeight),
		})
	}
	return out
}

func parseHeader(data []byte) (Info, []byte, error) {
	var info Info
	if len(data) < 8 {
		return info, nil, errors.New("this file is too short to be a SWF")
	}
	sig := string(data[:3])
	switch sig {
	case "FWS":
		info.Compressed = false
	case "CWS":
		info.Compressed = true
	case "ZWS":
		return info, nil, errors.New("this SWF uses LZMA (ZWS), which the Wii's Flash player cannot read; save it as an uncompressed or zlib SWF")
	default:
		return info, nil, errors.New("this file is not a SWF; the first three bytes must be FWS or CWS")
	}
	info.Signature = sig
	info.Version = data[3]
	declared := binary.LittleEndian.Uint32(data[4:8])
	if declared < 8 {
		return info, nil, errors.New("the SWF header declares an impossible file length")
	}
	if declared > MaxSWF {
		return info, nil, fmt.Errorf("the SWF is %d bytes; the limit is %d", declared, MaxSWF)
	}
	info.FileLength = int(declared)

	if !info.Compressed {
		if len(data) < int(declared) {
			return info, nil, fmt.Errorf("the SWF is truncated: header says %d bytes, file has %d", declared, len(data))
		}
		return info, data[8:declared], nil
	}

	zr, err := zlib.NewReader(bytes.NewReader(data[8:]))
	if err != nil {
		return info, nil, fmt.Errorf("the compressed SWF body is damaged: %v", err)
	}
	defer zr.Close()
	body, err := io.ReadAll(io.LimitReader(zr, int64(declared)+1))
	if err != nil {
		return info, nil, fmt.Errorf("the compressed SWF body is damaged: %v", err)
	}
	if len(body) != int(declared)-8 {
		return info, nil, fmt.Errorf("the compressed SWF expands to %d bytes but its header says %d", len(body)+8, declared)
	}
	return info, body, nil
}

func parseBody(body []byte, info *Info) error {
	bits := bitReader{data: body}
	nbits, err := bits.read(5)
	if err != nil {
		return errors.New("the SWF stage rectangle is missing")
	}
	if nbits > 31 {
		return errors.New("the SWF stage rectangle is invalid")
	}
	var rect [4]int32
	for i := range rect {
		raw, err := bits.read(int(nbits))
		if err != nil {
			return errors.New("the SWF stage rectangle is truncated")
		}
		rect[i] = signExtend(raw, int(nbits))
	}
	width := (int64(rect[1]) - int64(rect[0])) / 20
	height := (int64(rect[3]) - int64(rect[2])) / 20
	if width < 0 || height < 0 {
		return errors.New("the SWF stage rectangle is invalid")
	}
	info.Width = int(width)
	info.Height = int(height)

	// The RECT is bit-packed but the frame rate begins on the next byte, as
	// SWF-Checker and every player assume.
	pos := (bits.pos + 7) / 8
	if len(body) < pos+4 {
		return errors.New("the SWF frame header is truncated")
	}
	rate := binary.LittleEndian.Uint16(body[pos:])
	info.FrameRate = math.Round(float64(rate)/256*100) / 100
	info.FrameCount = int(binary.LittleEndian.Uint16(body[pos+2:]))
	pos += 4

	for tags := 0; tags < maxTags; tags++ {
		if len(body) < pos+2 {
			return errors.New("the SWF tag table is truncated")
		}
		short := binary.LittleEndian.Uint16(body[pos:])
		pos += 2
		code := short >> 6
		length := int(short & 0x3f)
		if length == 0x3f {
			if len(body) < pos+4 {
				return errors.New("the SWF tag table is truncated")
			}
			length = int(binary.LittleEndian.Uint32(body[pos:]))
			pos += 4
		}
		if length < 0 || len(body) < pos+length {
			return fmt.Errorf("the SWF tag table is truncated (tag %d)", code)
		}
		value := body[pos : pos+length]
		pos += length
		switch code {
		case 0: // End
			return nil
		case 69: // FileAttributes
			if len(value) >= 1 && value[0]&0x08 != 0 {
				info.ActionScript3 = true
			}
		case 72, 82: // DoABC / DoABC2
			info.ActionScript3 = true
		case 9: // SetBackgroundColor
			if len(value) >= 3 {
				info.Background = fmt.Sprintf("#%02x%02x%02x", value[0], value[1], value[2])
			}
		}
	}
	return errors.New("the SWF has too many tags")
}

type bitReader struct {
	data []byte
	pos  int // next bit
}

func (b *bitReader) read(n int) (uint32, error) {
	if n < 0 || n > 32 || b.pos+n > len(b.data)*8 {
		return 0, io.ErrUnexpectedEOF
	}
	var out uint32
	for i := 0; i < n; i++ {
		byteAt := b.pos / 8
		bitAt := 7 - uint(b.pos%8)
		out = out<<1 | uint32((b.data[byteAt]>>bitAt)&1)
		b.pos++
	}
	return out, nil
}

func signExtend(v uint32, bits int) int32 {
	if bits == 0 {
		return 0
	}
	shift := 32 - bits
	return int32(v<<shift) >> shift
}

// Describe is a short human sentence for logs and the shop's confirmation page.
func (i Info) Describe() string {
	var b strings.Builder
	fmt.Fprintf(&b, "Flash %s v%d, %dx%d, %.2f fps, %d frames", i.Signature, i.Version, i.Width, i.Height, i.FrameRate, i.FrameCount)
	if i.Background != "" {
		fmt.Fprintf(&b, ", background %s", i.Background)
	}
	return b.String()
}
