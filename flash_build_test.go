package main

// TestBuildFlashChannel verifies the runtime Flash injection wiring: a Flash
// catalog title whose SWF is fetched from an approved source is injected into
// an operator-supplied official Flash base WAD and published as a fakesigned
// Wii channel. The synthetic template WAD below mirrors the one the flashpkg
// package's own tests use.

import (
	"bytes"
	"context"
	"encoding/binary"
	"io"
	"math/bits"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Markthegamer108/wii-shroombrew-channel/internal/channelpkg"
	"github.com/wii-tools/arclib"
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

// makeSWF mirrors the flashpkg test helper: a tiny valid SWF the Inject
// validation accepts.
func makeSWF(version byte, width, height int) []byte {
	max := uint32(width * 20)
	if uint32(height*20) > max {
		max = uint32(height * 20)
	}
	n := bits.Len32(max) + 1
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
	body = append(body, []byte{0, 0}...)

	out := append([]byte("FWS"), version, 0, 0, 0, 0)
	binary.LittleEndian.PutUint32(out[4:], uint32(8+len(body)))
	return append(out, body...)
}

func lastSlash(s string) int {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == '/' {
			return i
		}
	}
	return -1
}

func ensureDir(arc *arclib.ARC, name string) (*arclib.ARCDir, error) {
	current := &arc.RootRecord
	if name == "" {
		return current, nil
	}
	for _, part := range strings.Split(name, "/") {
		if part == "" || part == "." {
			continue
		}
		next, err := current.GetDir(part)
		if err == nil {
			current = next
			continue
		}
		current.AddDir(arclib.ARCDir{Filename: part})
		next, err = current.GetDir(part)
		if err != nil {
			return nil, err
		}
		current = next
	}
	return current, nil
}

func u8Archive(files map[string][]byte) []byte {
	arc := &arclib.ARC{}
	for name, data := range files {
		dirName := ""
		if i := lastSlash(name); i >= 0 {
			dirName, name = name[:i], name[i+1:]
		}
		dir, err := ensureDir(arc, dirName)
		if err != nil {
			panic(err)
		}
		dir.WriteFile(name, data)
	}
	data, _ := arc.Save()
	return data
}

func templateWAD(archive []byte) []byte {
	banner, _ := channelpkg.BuildBanner(channelpkg.BannerInfo{Title: "Template", Version: "1.0", Author: "Shroombrew"})
	wad, _ := channelpkg.Synthesize(channelpkg.SynthOptions{
		TitleID: 0x0001000154454D50, Version: 1,
		Program: []byte{0x00, 0x00, 0x00, 0x01},
		Payload: archive,
		Banner:  banner,
	})
	return wad
}

func TestBuildFlashChannel(t *testing.T) {
	// Build a synthetic operator-supplied Flash base WAD.
	swf := makeSWF(8, 320, 240)
	template := templateWAD(u8Archive(map[string][]byte{
		"content/menu.swf":         swf,
		"config/config.common.pcf": []byte("static_heap_size 8192\r\ncontent_url file:///old.swf\r\n"),
		"config/US/config.us.pcf":  []byte("ortho_rect -304 +228 +304 -228\r\ncontent_url file:///old.swf\r\n"),
		"config/US/keymap.ini":     []byte("KEY_BUTTON_LEFT KEY_LEFT\r\n"),
		"config/EU/config.eu.pcf":  []byte("ortho_rect -304 +228 +304 -228\r\n"),
		"config/JP/config.jp.pcf":  []byte("ortho_rect -304 +228 +304 -228\r\n"),
	}))
	tDir := t.TempDir()
	tmplPath := filepath.Join(tDir, "flash-template.wad")
	if err := os.WriteFile(tmplPath, template, 0600); err != nil {
		t.Fatal(err)
	}

	// A Flash title whose SWF is served from the approved OSC source.
	p := filepath.Join(tDir, "catalog.json")
	catalog := `[{"slug":"flashdemo","name":"Flash Demo","author":"Shroombrew","category":"Flash","flash":true,` +
		`"description":{"short":"A Flash game","long":"A Flash game installed as a Wii channel."},` +
		`"url":{"zip":"https://hbb1.oscwii.org/api/contents/flashdemo/flash.swf"},` +
		`"file_size":{"zip_compressed":1000},"shop":{"title_id":"00010001464c4153","title_version":1}}]`
	if err := os.WriteFile(p, []byte(catalog), 0600); err != nil {
		t.Fatal(err)
	}
	s, err := newStorefront(p)
	if err != nil {
		t.Fatal(err)
	}
	s.channelsRoot = filepath.Join(tDir, "channels")
	s.iconsRoot = filepath.Join(tDir, "icons")
	s.flashTemplatePath = tmplPath
	t.Setenv("SHROOMBREW_DIRECT_SD", "1")
	t.Setenv("SHROOMBREW_NATIVE_EC", "1")

	s.installs.client = &http.Client{Transport: installRoundTrip(func(r *http.Request) (*http.Response, error) {
		if r.URL.Host != "hbb1.oscwii.org" || r.URL.Path != "/api/contents/flashdemo/flash.swf" {
			t.Errorf("unexpected flash fetch: %s", r.URL)
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewReader(swf)), ContentLength: int64(len(swf)), Header: make(http.Header)}, nil
	})}

	app := s.bySlug["flashdemo"]
	if !app.Flash {
		t.Fatal("flash.json app was not marked Flash")
	}
	if !canSynthesize(app) {
		t.Fatal("the Flash app should be packageable as a channel")
	}
	if s.hasNativeSDPackage(app) {
		t.Fatal("a Flash channel was found before one was built")
	}
	if err := s.ensureChannel(context.Background(), app); err != nil {
		t.Fatalf("ensureChannel: %v", err)
	}
	if !s.hasNativeSDPackage(app) {
		t.Fatal("the built Flash channel was not recognized")
	}
	manifest, err := channelpkg.Load(s.channelsRoot, strings.ToLower(classicTitleIDForApp(app)))
	if err != nil {
		t.Fatal(err)
	}
	if manifest.Kind != "channel" || manifest.SDApp != "flashdemo" {
		t.Fatalf("manifest = %+v", manifest)
	}
	// The published package must exist on disk and carry the Flash channel
	// identity (fakesigned ticket + TMD + contents + manifest).
	pkgDir := filepath.Join(s.channelsRoot, strings.ToLower(classicTitleIDForApp(app)))
	for _, name := range []string{"ticket.bin", "tmd", "certs.bin", "manifest.json"} {
		if st, err := os.Stat(filepath.Join(pkgDir, name)); err != nil || st.Size() == 0 {
			t.Fatalf("published Flash package missing %s: %v", name, err)
		}
	}
}
