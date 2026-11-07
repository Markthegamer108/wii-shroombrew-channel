package main

// The welcome page (W_01): four tiles per list, five lists the arrows (and
// the page's own timer) step through, as on the real shop. Every list holds
// what its heading says, only titles whose icon exists and that have a
// release for the console's region, and the newest homebrew wears the shop's
// golden NEW badge.

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

var welcomeNonAlnum = regexp.MustCompile(`[^a-z0-9]+`)

// welcomeName is a title's name for matching: marks, case and punctuation
// ignored, the second title line included.
func welcomeName(app catalogApp) string {
	n := strings.NewReplacer("™", "", "®", "", "©", "").Replace(app.Name + " " + app.Subtitle)
	return strings.TrimSpace(welcomeNonAlnum.ReplaceAllString(strings.ToLower(n), " "))
}

// hasShopIcon reports whether serveShopIcon has an image for the title.
func hasShopIcon(app catalogApp) bool {
	if info, err := os.Stat(filepath.Join("./icons", app.Slug+".png")); err == nil && !info.IsDir() {
		return true
	}
	if !app.Licensed {
		return false
	}
	info, err := os.Stat(filepath.Join("./site/oss/ccs", classicTitleIDForApp(app), "FFFD0000"))
	return err == nil && !info.IsDir()
}

// availableIn reports whether a title has a release the console's region can
// run: homebrew and single-release titles always, region-locked licensed
// titles only with a variant for that region.
func availableIn(app catalogApp, region string) bool {
	if !app.Licensed || len(app.Variants) == 0 {
		return true
	}
	for _, v := range app.Variants {
		if v.RegionGroup == region {
			return true
		}
	}
	return false
}

type welcomeList struct {
	Label  string
	Titles []catalogApp
}

// Hand-picked lists, by name; titles the catalog lacks are skipped.
var (
	welcomeRecommended = []string{"super mario 64", "the legend of zelda ocarina of time", "world of goo", "snes9x gx", "super mario bros 3", "wiimc"}
	welcomeVC          = []string{"super mario bros", "super metroid", "the legend of zelda a link to the past", "donkey kong country", "mario kart 64", "kirby s adventure", "f zero", "punch out"}
	welcomeWiiWare     = []string{"lostwinds", "cave story", "bit trip beat", "mega man 9", "dr mario online rx", "toki tori", "world of goo", "final fantasy crystal chronicles my life as a king"}
)

// welcomeLists returns the five lists for a console in region.
func (s *storefront) welcomeLists(region string) []welcomeList {
	used := map[string]bool{}
	ok := func(app catalogApp) bool {
		return !used[app.Slug] && availableIn(app, region) && hasShopIcon(app)
	}
	take := func(list *welcomeList, app catalogApp) {
		used[app.Slug] = true
		list.Titles = append(list.Titles, app)
	}
	byName := func(label string, names []string, filter func(catalogApp) bool) welcomeList {
		list := welcomeList{Label: label}
		for _, want := range names {
			for _, app := range s.apps {
				if len(list.Titles) < 4 && filter(app) && ok(app) && welcomeName(app) == want {
					take(&list, app)
					break
				}
			}
		}
		return list
	}
	newest := func(list welcomeList, filter func(catalogApp) bool) welcomeList {
		apps := make([]catalogApp, 0, len(s.apps))
		for _, app := range s.apps {
			if filter(app) {
				apps = append(apps, app)
			}
		}
		sort.SliceStable(apps, func(i, j int) bool { return apps[i].ReleaseDate > apps[j].ReleaseDate })
		for _, app := range apps {
			if len(list.Titles) == 4 {
				break
			}
			if ok(app) {
				take(&list, app)
			}
		}
		return list
	}
	everything := func(catalogApp) bool { return true }
	homebrew := func(app catalogApp) bool { return !app.Licensed }
	section := func(category string) func(catalogApp) bool {
		return func(app catalogApp) bool { return app.Licensed && app.Category == category }
	}

	// New homebrew goes first so the recommended list cannot take it.
	fresh := newest(welcomeList{Label: "New Homebrew"}, homebrew)
	lists := []welcomeList{
		newest(byName("Recommended Titles", welcomeRecommended, everything), everything),
		fresh,
		newest(byName("Virtual Console", welcomeVC, section("Virtual Console")), section("Virtual Console")),
		newest(byName("WiiWare", welcomeWiiWare, section("WiiWare")), section("WiiWare")),
		newest(welcomeList{Label: "Homebrew Games"}, func(app catalogApp) bool { return !app.Licensed && app.Category == "games" }),
	}
	return lists
}

// welcomeNew lists the titles that get the NEW badge: the New Homebrew list.
func welcomeNew(lists []welcomeList) map[string]bool {
	isNew := map[string]bool{}
	for _, list := range lists {
		if list.Label == "New Homebrew" {
			for _, app := range list.Titles {
				isNew[classicTitleIDForApp(app)] = true
			}
		}
	}
	return isNew
}

// classicHomeJS is W_01's home.js: the news lines, the four list headings
// after the first, then each list's title IDs. homeNew names the titles that
// wear the NEW badge.
func (s *storefront) classicHomeJS(r *http.Request) []byte {
	lists := s.welcomeLists(regionFor(r))
	data := [][]string{
		{"Welcome to Shroombrew!", "Browse homebrew for your Wii.", "Homebrew for everyone.", "Download apps as Wii Menu channels."},
		{},
	}
	for _, list := range lists[1:] {
		data[1] = append(data[1], list.Label)
	}
	for _, list := range lists {
		ids := []string{}
		for _, app := range list.Titles {
			ids = append(ids, classicTitleIDForApp(app))
		}
		// The page reads exactly four per list; repeat to fill a short one.
		for i := 0; len(ids) < 4 && len(list.Titles) > 0; i++ {
			ids = append(ids, ids[i])
		}
		if len(ids) == 0 {
			ids = []string{"0000000000000000", "0000000000000000", "0000000000000000", "0000000000000000"}
		}
		data = append(data, ids)
	}
	newIDs := []string{}
	for id := range welcomeNew(lists) {
		newIDs = append(newIDs, id)
	}
	sort.Strings(newIDs)
	b, _ := json.Marshal(data)
	n, _ := json.Marshal(newIDs)
	return []byte("// Generated from Shroombrew's catalog.\nvar homedata = " + string(b) + ";\nvar homeNew = " + string(n) + ";")
}

// classicFeaturedGamesJS is the welcome page's small gameslist: just the
// titles its lists show, so Wii Opera need not parse the whole catalog.
func (s *storefront) classicFeaturedGamesJS(r *http.Request) []byte {
	lists := s.welcomeLists(regionFor(r))
	isNew := welcomeNew(lists)
	titles := []classicTitle{{}} // the shop's lists begin at index one
	seen := map[string]bool{}
	for _, list := range lists {
		for _, app := range list.Titles {
			t := classicTitleFor(app)
			if seen[t.ID] {
				continue
			}
			seen[t.ID] = true
			t.IsNew = isNew[t.ID]
			titles = append(titles, t)
		}
	}
	b, _ := json.Marshal(titles)
	return append([]byte("// Generated featured Shroombrew titles.\nvar gameslist = "), append(b, ';')...)
}
