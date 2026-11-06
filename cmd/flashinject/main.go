// Command flashinject builds an installable Wii channel from a Flash movie.
//
// It injects the SWF into an official Flash runtime WAD that the operator
// supplies (the "Flash Placeholder" / Back to Nature base). Shroombrew does not
// include that copyrighted WAD. The result is fakesigned and can be served from
// the shop's CCS or installed with a WAD manager on a trucha-patched console.
//
//	flashinject -template flash-base.wad -swf game.swf -assets game-files \
//	    -title 0001000153485245 -name "Windows XP Tour" -out xp-tour.wad
package main

import (
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/Markthegamer108/wii-shroombrew-channel/internal/flashpkg"
)

func main() {
	template := flag.String("template", "", "official Flash runtime WAD (operator supplied)")
	swfPath := flag.String("swf", "", "movie to inject")
	assetDir := flag.String("assets", "", "optional folder copied next to the movie")
	title := flag.String("title", "", "new title ID, 00010001 followed by 8 hex digits")
	name := flag.String("name", "", "channel name")
	author := flag.String("author", "", "author shown on the channel banner")
	description := flag.String("description", "", "one-line description")
	category := flag.String("category", "Flash", "catalog category")
	displayVersion := flag.String("display-version", "1.0", "version shown on the banner")
	iconPath := flag.String("icon", "", "optional 128x48 PNG icon")
	titleVersion := flag.Uint("title-version", 0, "TMD title version (0 = template + 1)")
	regionName := flag.String("region", "free", "free, J, E, P, A, K or T")
	zoom := flag.String("zoom", "auto", "auto, default, fill, or half-extents H,V")
	mouse := flag.Bool("mouse", true, "use the Wii pointer as a mouse")
	qwerty := flag.Bool("qwerty", false, "enable the on-screen keyboard")
	quality := flag.String("quality", "high", "low, medium or high")
	shared := flag.Bool("shared-object", false, "allow SharedObject saves")
	strap := flag.String("strap", "none", "none, normal or no_ex")
	hbmNoSave := flag.Bool("hbm-no-save", false, "skip the unsaved-data warning on HOME")
	background := flag.String("background", "", `background colour "r g b a"`)
	keymapPath := flag.String("keymap", "", "optional keymap file (BUTTON KEY per line)")
	out := flag.String("out", "", "output WAD")
	check := flag.Bool("check", false, "only validate the movie")
	flag.Parse()

	if *swfPath == "" {
		fail("missing -swf")
	}
	swf, err := os.ReadFile(*swfPath)
	if err != nil {
		fail("swf: %v", err)
	}
	info, warnings, err := flashpkg.Validate(swf)
	if err != nil {
		fail("%v", err)
	}
	fmt.Println(info.Describe())
	for _, w := range warnings {
		fmt.Fprintf(os.Stderr, "warning: %s\n", w.Message)
	}
	if *check {
		return
	}

	if *template == "" || *name == "" || *title == "" || *out == "" {
		fail("-template, -name, -title and -out are required")
	}
	id, err := strconv.ParseUint(strings.TrimPrefix(strings.ToLower(*title), "0x"), 16, 64)
	if err != nil || id>>32 != 0x00010001 {
		fail("title must be 00010001 followed by 8 hex digits")
	}
	region, err := regionCode(*regionName)
	if err != nil {
		fail("%v", err)
	}
	base, err := os.ReadFile(*template)
	if err != nil {
		fail("template: %v", err)
	}
	var icon []byte
	if *iconPath != "" {
		icon, err = os.ReadFile(*iconPath)
		if err != nil {
			fail("icon: %v", err)
		}
	}
	assets, err := readAssets(*assetDir)
	if err != nil {
		fail("assets: %v", err)
	}
	keymap, err := readKeymap(*keymapPath)
	if err != nil {
		fail("keymap: %v", err)
	}

	wad, err := flashpkg.Inject(flashpkg.Options{
		Template:        base,
		SWF:             swf,
		Assets:          assets,
		TitleID:         id,
		TitleVersion:    uint16(*titleVersion),
		Region:          region,
		Name:            *name,
		Author:          *author,
		Description:     *description,
		Category:        *category,
		DisplayVersion:  *displayVersion,
		IconPNG:         icon,
		Zoom:            *zoom,
		Mouse:           *mouse,
		Qwerty:          *qwerty,
		Quality:         *quality,
		SharedObject:    *shared,
		StrapReminder:   *strap,
		HBMNoSave:       *hbmNoSave,
		BackgroundColor: *background,
		Keymap:          keymap,
	})
	if err != nil {
		fail("%v", err)
	}
	if err = os.WriteFile(*out, wad, 0644); err != nil {
		fail("write: %v", err)
	}
	fmt.Printf("wrote %s (%d bytes)\n", *out, len(wad))
}

func fail(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "flashinject: "+format+"\n", args...)
	os.Exit(1)
}

func regionCode(name string) (uint16, error) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "", "free", "region-free", "world":
		return 0xffff, nil
	case "j", "jp", "japan":
		return 0, nil
	case "e", "us", "usa", "america":
		return 1, nil
	case "p", "eu", "europe", "pal":
		return 2, nil
	case "a", "au", "australia":
		return 3, nil
	case "k", "kr", "korea":
		return 5, nil
	case "t", "tw", "taiwan":
		return 7, nil
	}
	return 0, fmt.Errorf("unknown region %q; use free, J, E, P, A, K or T", name)
}

func readAssets(dir string) ([]flashpkg.Asset, error) {
	if dir == "" {
		return nil, nil
	}
	var out []flashpkg.Asset
	var total int64
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if d.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("%s is a symbolic link", p)
		}
		rel, err := filepath.Rel(dir, p)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		total += int64(len(data))
		if total > 128<<20 {
			return fmt.Errorf("the asset folder is larger than 128 MiB")
		}
		if len(out) >= 10000 {
			return fmt.Errorf("the asset folder has more than 10000 files")
		}
		out = append(out, flashpkg.Asset{Path: filepath.ToSlash(rel), Data: data})
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out, nil
}

func readKeymap(name string) ([]flashpkg.KeymapEntry, error) {
	if name == "" {
		return nil, nil
	}
	data, err := os.ReadFile(name)
	if err != nil {
		return nil, err
	}
	var out []flashpkg.KeymapEntry
	for n, line := range strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) != 2 {
			return nil, fmt.Errorf("line %d: expected BUTTON KEY", n+1)
		}
		out = append(out, flashpkg.KeymapEntry{Button: fields[0], Key: fields[1]})
	}
	return out, nil
}
