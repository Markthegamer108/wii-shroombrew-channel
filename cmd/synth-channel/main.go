// Command synth-channel builds one Wii Menu channel WAD from a homebrew app.
//
// It is the offline equivalent of what the shop does on the first download:
// a patched forwarder, a generated banner, and the app's ZIP packed into one
// fakesigned WAD. The result can be imported with cmd/import-channel.
//
//	synth-channel -slug WiiDonut -name "Wii Donut" -zip app.zip \
//	    -title 000100014e2278d0 -version 1.0.0 -out WiiDonut.wad
package main

import (
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/Markthegamer108/wii-shroombrew-channel/internal/channelpkg"
)

func main() {
	slug := flag.String("slug", "", "SD app folder name (letters, digits, _ and -)")
	name := flag.String("name", "", "channel title")
	author := flag.String("author", "", "author shown on the banner")
	version := flag.String("version", "1.0.0", "display version")
	description := flag.String("description", "", "one-line description")
	category := flag.String("category", "utilities", "catalog category")
	zipPath := flag.String("zip", "", "app package (ZIP)")
	iconPath := flag.String("icon", "", "optional 128x48 PNG icon")
	titleText := flag.String("title", "", "title ID, 00010001 followed by 8 hex digits")
	titleVersion := flag.Uint("title-version", 1, "TMD title version")
	release := flag.String("release", "", "release date (YYYY-MM-DD)")
	out := flag.String("out", "channel.wad", "output WAD")
	flag.Parse()

	if *slug == "" || *name == "" || *zipPath == "" || *titleText == "" {
		fmt.Fprintln(os.Stderr, "slug, name, zip and title are required")
		os.Exit(2)
	}
	id, err := strconv.ParseUint(strings.TrimPrefix(strings.ToLower(*titleText), "0x"), 16, 64)
	if err != nil {
		fail("title: %v", err)
	}
	payload, err := os.ReadFile(*zipPath)
	if err != nil {
		fail("zip: %v", err)
	}
	var icon []byte
	if *iconPath != "" {
		icon, err = os.ReadFile(*iconPath)
		if err != nil {
			fail("icon: %v", err)
		}
	}
	var released time.Time
	if *release != "" {
		released, err = time.Parse("2006-01-02", *release)
		if err != nil {
			fail("release: %v", err)
		}
	}

	wad, err := channelpkg.BuildChannel(channelpkg.Channel{
		Slug:         *slug,
		Name:         *name,
		Author:       *author,
		Version:      *version,
		Description:  *description,
		Category:     *category,
		ReleaseDate:  released,
		IconPNG:      icon,
		Payload:      payload,
		TitleID:      id,
		TitleVersion: uint16(*titleVersion),
	})
	if err != nil {
		fail("build: %v", err)
	}

	// Import into a scratch store first: a WAD that does not pass the same
	// validation the server applies is not worth writing.
	dir, err := os.MkdirTemp("", "synth-channel-")
	if err != nil {
		fail("temp: %v", err)
	}
	defer os.RemoveAll(dir)
	staged := dir + "/channel.wad"
	if err = os.WriteFile(staged, wad, 0600); err != nil {
		fail("write: %v", err)
	}
	manifest, err := channelpkg.Import(staged, dir+"/store")
	if err != nil {
		fail("verify: %v", err)
	}
	if err = os.WriteFile(*out, wad, 0644); err != nil {
		fail("output: %v", err)
	}
	fmt.Printf("wrote %s (%d bytes)\n", *out, len(wad))
	fmt.Printf("title %s version %d, contents %v\n", manifest.TitleID, manifest.Version, manifest.Contents)
}

func fail(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "synth-channel: "+format+"\n", args...)
	os.Exit(1)
}
