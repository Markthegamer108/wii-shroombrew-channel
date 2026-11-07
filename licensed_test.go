package main

import (
	"os"
	"testing"
)

// Loads the generated licensed catalog and checks the server can consume it:
// unique slugs in bySlug, real prices surfaced by classicPriceFor, int versions.
// Skips when the generated catalog (a local build artifact) is absent.
func TestLicensedCatalogLoad(t *testing.T) {
	if _, err := os.Stat("../catalog_generated.json"); err != nil {
		t.Skip("catalog_generated.json not present (local build artifact)")
	}
	s, err := newStorefront("../catalog_generated.json")
	if err != nil {
		t.Fatalf("newStorefront: %v", err)
	}
	licensed := 0
	seen := map[string]bool{}
	for _, a := range s.apps {
		if !a.Licensed {
			continue
		}
		licensed++
		// Channels such as the Internet Channel were free on the shop.
		if a.Price < 0 || (a.Price == 0 && a.Category != "Channels") {
			t.Errorf("licensed app %s has price %d", a.Slug, a.Price)
		}
		if seen[a.Slug] {
			t.Errorf("duplicate slug %s", a.Slug)
		}
		seen[a.Slug] = true
		if _, ok := s.bySlug[a.Slug]; !ok {
			t.Errorf("slug %s missing from bySlug", a.Slug)
		}
		if a.Shop.Version < 0 {
			t.Errorf("app %s negative version", a.Slug)
		}
	}
	t.Logf("licensed apps: %d, unique slugs: %d, bySlug entries: %d", licensed, len(seen), len(s.bySlug))
	if licensed < 2600 {
		t.Errorf("expected ~2694 licensed apps, got %d", licensed)
	}
}

// Loads the merged homebrew + licensed catalog.json (a local build artifact)
// and asserts the storefront consumes it with unique slugs.
func TestMergedCatalogLoad(t *testing.T) {
	if _, err := os.Stat("../catalog.json"); err != nil {
		t.Skip("catalog.json not present (local build artifact)")
	}
	s, err := newStorefront("../catalog.json")
	if err != nil {
		t.Fatalf("newStorefront: %v", err)
	}
	if len(s.bySlug) != len(s.apps) {
		t.Fatalf("bySlug entries %d != apps %d", len(s.bySlug), len(s.apps))
	}
	lic := 0
	for _, a := range s.apps {
		if a.Licensed {
			lic++
		}
	}
	t.Logf("apps: %d, bySlug: %d, licensed: %d", len(s.apps), len(s.bySlug), lic)
	// Region releases are collapsed into one entry per game (merge_catalog.py).
	if lic < 1200 {
		t.Errorf("expected ~1351 licensed games, got %d", lic)
	}
}
