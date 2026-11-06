package flashpkg

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"math/bits"
	"strings"
	"testing"
)

type bitWriter struct {
	bytes []byte
	pos   int
}

func (b *bitWriter) write(v uint32, n int) {
	for i := n - 1; i >= 0; i-- {
		if b.pos%8 == 0 {
			b.bytes = append(b.bytes, 0)
		}
		if v>>uint(i)&1 != 0 {
			b.bytes[len(b.bytes)-1] |= 1 << uint(7-b.pos%8)
		}
		b.pos++
	}
}

func (b *bitWriter) align() {
	for b.pos%8 != 0 {
		b.write(0, 1)
	}
}

func makeSWF(version byte, width, height int, tags []byte, compressed bool) []byte {
	if tags == nil {
		tags = []byte{0, 0}
	}
	max := uint32(width * 20)
	if uint32(height*20) > max {
		max = uint32(height * 20)
	}
	n := bits.Len32(max) + 1 // signed RECT values need a sign bit
	if n < 1 {
		n = 1
	}
	if n > 31 {
		panic("test stage too large")
	}
	var rect bitWriter
	rect.write(uint32(n), 5)
	for _, v := range []uint32{0, uint32(width * 20), 0, uint32(height * 20)} {
		rect.write(v, n)
	}
	rect.align()
	body := append([]byte(nil), rect.bytes...)
	frame := make([]byte, 4)
	binary.LittleEndian.PutUint16(frame, 24<<8) // 24 fps, fixed 8.8
	binary.LittleEndian.PutUint16(frame[2:], 2)
	body = append(body, frame...)
	body = append(body, tags...)

	if compressed {
		var z bytes.Buffer
		zw := zlib.NewWriter(&z)
		_, _ = zw.Write(body)
		_ = zw.Close()
		out := append([]byte("CWS"), version, 0, 0, 0, 0)
		binary.LittleEndian.PutUint32(out[4:], uint32(8+len(body)))
		return append(out, z.Bytes()...)
	}
	out := append([]byte("FWS"), version, 0, 0, 0, 0)
	binary.LittleEndian.PutUint32(out[4:], uint32(8+len(body)))
	return append(out, body...)
}

func shortTag(code uint16, body []byte) []byte {
	if len(body) >= 0x3f {
		panic("use a long tag in tests")
	}
	out := make([]byte, 2+len(body))
	binary.LittleEndian.PutUint16(out, code<<6|uint16(len(body)))
	copy(out[2:], body)
	return out
}

func joinTags(tags ...[]byte) []byte {
	var out []byte
	for _, t := range tags {
		out = append(out, t...)
	}
	return out
}

func TestValidatePlainSWF(t *testing.T) {
	tags := joinTags(
		shortTag(9, []byte{0x11, 0x22, 0x33}),
		shortTag(1, nil),
		shortTag(0, nil),
	)
	info, warns, err := Validate(makeSWF(8, 608, 456, tags, false))
	if err != nil {
		t.Fatal(err)
	}
	if info.Signature != "FWS" || info.Compressed {
		t.Fatalf("signature = %q compressed=%v", info.Signature, info.Compressed)
	}
	if info.Width != 608 || info.Height != 456 {
		t.Fatalf("stage = %dx%d", info.Width, info.Height)
	}
	if info.FrameCount != 2 || info.FrameRate != 24 {
		t.Fatalf("frames = %d rate = %v", info.FrameCount, info.FrameRate)
	}
	if info.Background != "#112233" {
		t.Fatalf("background = %q", info.Background)
	}
	if len(warns) != 0 {
		t.Fatalf("unexpected warnings: %+v", warns)
	}
}

func TestValidateCompressedSWF(t *testing.T) {
	info, warns, err := Validate(makeSWF(9, 320, 240, nil, true))
	if err != nil {
		t.Fatal(err)
	}
	if info.Signature != "CWS" || !info.Compressed || info.Width != 320 || info.Height != 240 {
		t.Fatalf("info = %+v", info)
	}
	if len(warns) != 1 || warns[0].Code != "version-9" {
		t.Fatalf("warnings = %+v", warns)
	}
}

func TestValidateRejectsLZMA(t *testing.T) {
	data := makeSWF(13, 320, 240, nil, false)
	copy(data, "ZWS")
	_, _, err := Validate(data)
	if err == nil || !strings.Contains(err.Error(), "LZMA") {
		t.Fatalf("err = %v", err)
	}
}

func TestValidateRejectsNewerVersion(t *testing.T) {
	_, _, err := Validate(makeSWF(10, 320, 240, nil, false))
	if err == nil || !strings.Contains(err.Error(), "version 10") {
		t.Fatalf("err = %v", err)
	}
}

func TestValidateRejectsAS3FileAttributes(t *testing.T) {
	tags := joinTags(shortTag(69, []byte{0x08, 0, 0, 0}), shortTag(0, nil))
	_, _, err := Validate(makeSWF(9, 320, 240, tags, false))
	if err == nil || !strings.Contains(err.Error(), "ActionScript 3") {
		t.Fatalf("err = %v", err)
	}
}

func TestValidateRejectsDoABC(t *testing.T) {
	tags := joinTags(shortTag(82, []byte{0, 0, 0, 0}), shortTag(0, nil))
	_, _, err := Validate(makeSWF(9, 320, 240, tags, false))
	if err == nil || !strings.Contains(err.Error(), "ActionScript 3") {
		t.Fatalf("err = %v", err)
	}
}

func TestValidateWarnsOnLargeStage(t *testing.T) {
	_, warns, err := Validate(makeSWF(8, 800, 600, nil, false))
	if err != nil {
		t.Fatal(err)
	}
	if len(warns) != 1 || warns[0].Code != "stage-too-large" {
		t.Fatalf("warnings = %+v", warns)
	}
}

func TestValidateRejectsTruncatedTag(t *testing.T) {
	// A tag header that claims one body byte, with no body after it.
	tags := []byte{0x01, 0x00}
	_, _, err := Validate(makeSWF(8, 320, 240, tags, false))
	if err == nil || !strings.Contains(err.Error(), "truncated") {
		t.Fatalf("err = %v", err)
	}
}

func TestValidateRejectsOversize(t *testing.T) {
	data := makeSWF(8, 320, 240, nil, false)
	binary.LittleEndian.PutUint32(data[4:], MaxSWF+1)
	_, _, err := Validate(data)
	if err == nil || !strings.Contains(err.Error(), "limit") {
		t.Fatalf("err = %v", err)
	}
}

func TestValidateRejectsNonSWF(t *testing.T) {
	_, _, err := Validate([]byte("GIF89a not a movie"))
	if err == nil || !strings.Contains(err.Error(), "not a SWF") {
		t.Fatalf("err = %v", err)
	}
}
