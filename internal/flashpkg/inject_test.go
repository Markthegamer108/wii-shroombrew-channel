package flashpkg

import (
	"bytes"
	"testing"
	"time"

	"github.com/Markthegamer108/wii-shroombrew-channel/internal/channelpkg"
	"github.com/wii-tools/arclib"
	"github.com/wii-tools/wadlib"
)

func u8Archive(t *testing.T, files map[string][]byte) []byte {
	t.Helper()
	arc := &arclib.ARC{}
	for name, data := range files {
		dirName := ""
		if i := lastSlash(name); i >= 0 {
			dirName, name = name[:i], name[i+1:]
		}
		dir, err := ensureDir(arc, dirName)
		if err != nil {
			t.Fatal(err)
		}
		dir.WriteFile(name, data)
	}
	data, err := arc.Save()
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func lastSlash(s string) int {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == '/' {
			return i
		}
	}
	return -1
}

func templateWAD(t *testing.T, archive []byte) []byte {
	t.Helper()
	banner, err := channelpkg.BuildBanner(channelpkg.BannerInfo{
		Title: "Template", Version: "1.0", Author: "Shroombrew",
	})
	if err != nil {
		t.Fatal(err)
	}
	wad, err := channelpkg.Synthesize(channelpkg.SynthOptions{
		TitleID: 0x0001000154454D50, Version: 1,
		Program: []byte{0x00, 0x00, 0x00, 0x01},
		Payload: archive,
		Banner:  banner,
	})
	if err != nil {
		t.Fatal(err)
	}
	return wad
}

func TestInjectFlashChannel(t *testing.T) {
	oldSWF := makeSWF(8, 320, 240, nil, false)
	newSWF := makeSWF(8, 320, 240, nil, true)
	template := templateWAD(t, u8Archive(t, map[string][]byte{
		"content/menu.swf":         oldSWF,
		"content/readme.txt":       []byte("keep me"),
		"config/config.common.pcf": []byte("static_heap_size 8192\r\ncontent_url file:///old.swf\r\n"),
		"config/US/config.us.pcf":  []byte("ortho_rect -304 +228 +304 -228\r\ncontent_url file:///old.swf\r\n"),
		"config/US/keymap.ini":     []byte("KEY_BUTTON_LEFT KEY_LEFT\r\n"),
		"config/EU/config.eu.pcf":  []byte("ortho_rect -304 +228 +304 -228\r\n"),
		"config/JP/config.jp.pcf":  []byte("ortho_rect -304 +228 +304 -228\r\n"),
	}))

	out, err := Inject(Options{
		Template:       template,
		SWF:            newSWF,
		Assets:         []Asset{{Path: "sounds/theme.mp3", Data: []byte("mp3")}},
		TitleID:        0x0001000153485245,
		TitleVersion:   2,
		Region:         0xffff,
		Name:           "Test Flash",
		Author:         "Shroombrew",
		Description:    "A test movie",
		Category:       "Flash",
		ReleaseDate:    time.Date(2026, 11, 6, 0, 0, 0, 0, time.UTC),
		DisplayVersion: "1.0",
		Mouse:          true,
		Qwerty:         true,
		SharedObject:   true,
	})
	if err != nil {
		t.Fatal(err)
	}

	w, err := wadlib.LoadWAD(out)
	if err != nil {
		t.Fatal(err)
	}
	if got := w.TMD.TitleID; got != 0x0001000153485245 {
		t.Fatalf("TMD title = %016x", got)
	}
	if w.Ticket.TitleID != w.TMD.TitleID {
		t.Fatalf("ticket title = %016x", w.Ticket.TitleID)
	}
	if w.TMD.TitleVersion != 2 || w.Ticket.TitleVersion != 2 {
		t.Fatalf("version TMD=%d ticket=%d", w.TMD.TitleVersion, w.Ticket.TitleVersion)
	}
	if w.TMD.Region != 0xffff {
		t.Fatalf("region = %04x", w.TMD.Region)
	}
	tmd, _ := w.GetTMD()
	if !channelpkg.IsFakesigned(tmd) {
		t.Fatal("TMD is not fakesigned")
	}
	ticket, _ := w.GetTicket()
	if !channelpkg.IsFakesigned(ticket) {
		t.Fatal("ticket is not fakesigned")
	}

	banner, err := w.GetContent(0)
	if err != nil {
		t.Fatal(err)
	}
	if len(banner) < 0x84 || string(banner[0x80:0x84]) != "IMET" {
		end := len(banner)
		if end > 128 {
			end = 128
		}
		t.Fatalf("content 0 is not an IMET banner (len %d): % x", len(banner), banner[:end])
	}

	main, err := w.GetContent(2)
	if err != nil {
		t.Fatal(err)
	}
	arc, err := arclib.Load(main)
	if err != nil {
		t.Fatal(err)
	}
	got, err := arc.ReadFile("content/menu.swf")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, newSWF) {
		t.Fatal("the movie was not replaced")
	}
	if asset, err := arc.ReadFile("content/sounds/theme.mp3"); err != nil || string(asset) != "mp3" {
		t.Fatalf("asset = %q, %v", asset, err)
	}
	if keep, err := arc.ReadFile("content/readme.txt"); err != nil || string(keep) != "keep me" {
		t.Fatalf("existing file was lost: %q, %v", keep, err)
	}
	common, err := arc.ReadFile("config/config.common.pcf")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(common, []byte("content_url                     file:///content/menu.swf")) {
		t.Fatalf("common config did not get the new URL:\n%s", common)
	}
	regional, err := arc.ReadFile("config/US/config.us.pcf")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(regional, []byte("ortho_rect                      -307 +231 +307 -231")) {
		t.Fatalf("regional config was not patched:\n%s", regional)
	}
	if bytes.Contains(regional, []byte("content_url")) {
		t.Fatalf("Back to Nature regional config still contains content_url:\n%s", regional)
	}
	keymap, err := arc.ReadFile("config/US/keymap.ini")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(keymap, []byte("KEY_BUTTON_A     32")) {
		t.Fatalf("keymap was not rewritten:\n%s", keymap)
	}
}

func TestInjectRejectsTraversal(t *testing.T) {
	template := templateWAD(t, u8Archive(t, map[string][]byte{
		"content/menu.swf": makeSWF(8, 320, 240, nil, false),
	}))
	_, err := Inject(Options{
		Template: template,
		SWF:      makeSWF(8, 320, 240, nil, false),
		Assets:   []Asset{{Path: "../escape.swf", Data: []byte("x")}},
		TitleID:  0x0001000153485245,
		Region:   0xffff,
		Name:     "Bad Asset",
	})
	if err == nil {
		t.Fatal("path traversal was accepted")
	}
}

func TestInjectRejectsNonFlashBase(t *testing.T) {
	template := templateWAD(t, u8Archive(t, map[string][]byte{
		"readme.txt": []byte("no movie here"),
	}))
	_, err := Inject(Options{
		Template: template,
		SWF:      makeSWF(8, 320, 240, nil, false),
		TitleID:  0x0001000153485245,
		Region:   0xffff,
		Name:     "No Base",
	})
	if err == nil {
		t.Fatal("a WAD without a Flash runtime was accepted")
	}
}

func TestInjectRejectsBadSWF(t *testing.T) {
	template := templateWAD(t, u8Archive(t, map[string][]byte{
		"content/menu.swf": makeSWF(8, 320, 240, nil, false),
	}))
	bad := makeSWF(9, 320, 240, nil, false)
	copy(bad, "ZWS")
	_, err := Inject(Options{
		Template: template,
		SWF:      bad,
		TitleID:  0x0001000153485245,
		Region:   0xffff,
		Name:     "Bad Movie",
	})
	if err == nil {
		t.Fatal("an LZMA movie was accepted")
	}
}

func TestOrthoExtents(t *testing.T) {
	if h, v := orthoExtents("default", 320, 240); h != 304 || v != 228 {
		t.Fatalf("default = %d,%d", h, v)
	}
	if h, v := orthoExtents("fill", 320, 240); h != 160 || v != 120 {
		t.Fatalf("fill = %d,%d", h, v)
	}
	if h, v := orthoExtents("200,150", 320, 240); h != 200 || v != 150 {
		t.Fatalf("explicit = %d,%d", h, v)
	}
}
