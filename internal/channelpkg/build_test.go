package channelpkg

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestBuildChannelAndPublish(t *testing.T) {
	const title = "00010001a1b2c3d4"
	payload := bytes.Repeat([]byte("shroombrew-payload-"), 5000)
	wad, err := BuildChannel(Channel{
		Slug:         "Demo",
		Name:         "Demo App",
		Author:       "Shroombrew",
		Version:      "2.0.0",
		Description:  "A channel built by a test",
		ReleaseDate:  time.Date(2023, 4, 5, 0, 0, 0, 0, time.UTC),
		IconPNG:      testIcon(t),
		Payload:      payload,
		TitleID:      0x00010001a1b2c3d4,
		TitleVersion: 20002,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(wad) < 32 || !bytes.Equal(wad[:4], []byte{0, 0, 0, 0x20}) {
		t.Fatal("not a WAD")
	}

	root := t.TempDir()
	manifest, err := Publish(wad, root, "Demo", title)
	if err != nil {
		t.Fatal(err)
	}
	if manifest.TitleID != title || manifest.Version != 20002 || manifest.SDApp != "Demo" {
		t.Fatalf("manifest = %+v", manifest)
	}
	loaded, err := Load(root, title)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Version != 20002 || loaded.SDApp != "Demo" {
		t.Fatalf("loaded = %+v", loaded)
	}

	// A second build of the same title replaces the package in place.
	wad2, err := BuildChannel(Channel{
		Slug:         "Demo",
		Name:         "Demo App",
		Author:       "Shroombrew",
		Version:      "2.0.1",
		Payload:      append(payload, '!'),
		TitleID:      0x00010001a1b2c3d4,
		TitleVersion: 20003,
	})
	if err != nil {
		t.Fatal(err)
	}
	manifest, err = Publish(wad2, root, "Demo", title)
	if err != nil {
		t.Fatalf("replacement: %v", err)
	}
	if manifest.Version != 20003 {
		t.Fatalf("replacement version = %d", manifest.Version)
	}
	entries, err := os.ReadDir(filepath.Join(root, title))
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.Name() == "tmd.20002" {
			t.Fatal("old tmd survived the replacement")
		}
	}

	// A package cannot be published under a different title than it carries.
	if _, err := Publish(wad2, root, "Demo", "0001000111111111"); err == nil {
		t.Fatal("published a mismatched title ID")
	}
}
