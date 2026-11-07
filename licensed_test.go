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
		if a.Price <= 0 {
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
