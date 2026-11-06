package main

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/Markthegamer108/wii-shroombrew-channel/internal/channelpkg"
)

func TestChannelPackageMatchesVersion(t *testing.T) {
	channel := channelpkg.Manifest{TitleID: "00010001a1b2c3d4", Version: synthVersionFor(7), SDApp: "Demo", Kind: "channel"}
	if !channelPackageMatchesVersion("Demo", channel, 7) {
		t.Fatal("a current synthesized channel should match")
	}
	if channelPackageMatchesVersion("Other", channel, 7) {
		t.Fatal("a channel for another app matched")
	}
	if !channelPackageMatchesVersion("Demo", channel, 6) {
		t.Fatal("a newer synthesized channel should still install after a catalog update")
	}
	if channelPackageMatchesVersion("Demo", channel, 8) {
		t.Fatal("an older synthesized channel matched a newer catalog version")
	}

	installer := channelpkg.Manifest{Version: 7, SDApp: "Demo", Kind: "installer"}
	if !channelPackageMatchesVersion("Demo", installer, 7) {
		t.Fatal("a matching installer did not match")
	}
	if channelPackageMatchesVersion("Demo", installer, 6) {
		t.Fatal("an installer matched the wrong version")
	}

	imported := channelpkg.Manifest{Version: 7}
	if !channelPackageMatchesVersion("Demo", imported, 7) {
		t.Fatal("an imported channel did not match its catalog version")
	}
	if channelPackageMatchesVersion("Demo", imported, 8) {
		t.Fatal("an imported channel matched the wrong version")
	}
}

func TestSynthVersionIsAboveCatalog(t *testing.T) {
	for _, version := range []int{0, 1, 5, 42, 40000} {
		if got := synthVersionFor(version); int(got) <= version {
			t.Fatalf("synthVersionFor(%d) = %d, not above the catalog version", version, got)
		}
	}
}

func TestEnsureChannelBuildsAndReuses(t *testing.T) {
	s := testShop(t)
	s.channelsRoot = t.TempDir()
	s.iconsRoot = t.TempDir()
	t.Setenv("SHROOMBREW_DIRECT_SD", "1")
	t.Setenv("SHROOMBREW_NATIVE_EC", "1")

	archive := testPackage(t, map[string]string{"apps/demo/boot.dol": "channel-program", "apps/demo/meta.xml": "<app/>"})
	fetches := 0
	s.installs.client = &http.Client{Transport: installRoundTrip(func(r *http.Request) (*http.Response, error) {
		fetches++
		return &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewReader(archive)), ContentLength: int64(len(archive)), Header: make(http.Header)}, nil
	})}

	app := s.bySlug["demo"]
	if !canSynthesize(app) {
		t.Fatal("the test app should be packageable as a channel")
	}
	if s.hasNativeSDPackage(app) {
		t.Fatal("a channel was found before one was built")
	}
	if err := s.ensureChannel(context.Background(), app); err != nil {
		t.Fatal(err)
	}
	if !s.hasNativeSDPackage(app) {
		t.Fatal("the built channel was not recognized")
	}
	manifest, err := channelpkg.Load(s.channelsRoot, strings.ToLower(classicTitleIDForApp(app)))
	if err != nil {
		t.Fatal(err)
	}
	if manifest.Kind != "channel" || manifest.SDApp != "demo" {
		t.Fatalf("manifest = %+v", manifest)
	}
	if int(manifest.Version) != int(synthVersionFor(0)) {
		t.Fatalf("channel version = %d, want %d", manifest.Version, synthVersionFor(0))
	}
	if fetches != 1 {
		t.Fatalf("package fetched %d times, want 1", fetches)
	}
	// A second request finds the stored package and downloads nothing.
	if err = s.ensureChannel(context.Background(), app); err != nil {
		t.Fatal(err)
	}
	if fetches != 1 {
		t.Fatalf("package fetched again (%d times)", fetches)
	}
	if _, err = os.Stat(s.channelsRoot + "/" + strings.ToLower(classicTitleIDForApp(app)) + "/ticket.bin"); err != nil {
		t.Fatal("ticket was not stored for the native EC download:", err)
	}
}

func TestCanSynthesizeRequiresDirectSD(t *testing.T) {
	s := testShop(t)
	app := s.bySlug["demo"]
	t.Setenv("SHROOMBREW_DIRECT_SD", "")
	t.Setenv("SHROOMBREW_NATIVE_EC", "1")
	if canSynthesize(app) {
		t.Fatal("channel building was enabled without SHROOMBREW_DIRECT_SD")
	}
	t.Setenv("SHROOMBREW_DIRECT_SD", "1")
	t.Setenv("SHROOMBREW_NATIVE_EC", "")
	if canSynthesize(app) {
		t.Fatal("channel building was enabled without SHROOMBREW_NATIVE_EC")
	}
	t.Setenv("SHROOMBREW_NATIVE_EC", "1")
	if !canSynthesize(app) {
		t.Fatal("channel building was disabled with both modes on")
	}
	if canSynthesize(s.bySlug["badurl"]) {
		t.Fatal("a non-OSC package URL was accepted")
	}
}
