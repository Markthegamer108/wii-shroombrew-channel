package main

// Building an app's Wii Menu channel the first time somebody downloads it.

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/Markthegamer108/wii-shroombrew-channel/internal/channelpkg"
	"github.com/Markthegamer108/wii-shroombrew-channel/internal/flashpkg"
)

// flashFetchLimit bounds a Flash movie download. Flash games are often bigger
// than the native homebrew package limit, so use a generous cap.
const flashFetchLimit = 512 << 20

// Synthesized channels use a title version above the catalog's, so a console
// that already has the old Open Shop Channel release downloads the new one.
const channelVersionBase = 20000

type channelBuild struct {
	done chan struct{}
	err  error
}

type channelBuilder struct {
	mu     sync.Mutex
	builds map[string]*channelBuild
	// Channels are built one at a time. Each build holds a whole app package
	// and its finished WAD in memory; a queue keeps a burst of first-time
	// downloads from exhausting the server.
	slot chan struct{}
}

func newChannelBuilder() *channelBuilder {
	return &channelBuilder{builds: make(map[string]*channelBuild), slot: make(chan struct{}, 1)}
}

// synthVersion is the TMD version written into a synthesized channel. It must
// be greater than any catalog version the console may already have installed.
func synthVersionFor(version int) uint16 {
	if version < 1 {
		version = 1
	}
	if version > 40000 {
		return uint16(version)
	}
	return uint16(channelVersionBase + version)
}

func synthVersion(app catalogApp) uint16 {
	return synthVersionFor(app.Shop.Version)
}

// channelPackageMatchesVersion reports whether a stored package is the
// installable channel for this app slug and catalog version. Imported
// NAND/SD packages keep the catalog version; synthesized channels carry the
// raised version.
func channelPackageMatchesVersion(slug string, manifest channelpkg.Manifest, catalogVersion int) bool {
	if manifest.SDApp == "" {
		return int(manifest.Version) == catalogVersion
	}
	if manifest.SDApp != slug {
		return false
	}
	if manifest.Kind == "channel" {
		return int(manifest.Version) >= int(synthVersionFor(catalogVersion))
	}
	return int(manifest.Version) == catalogVersion
}

func channelPackageMatches(app catalogApp, m channelpkg.Manifest) bool {
	return channelPackageMatchesVersion(app.Slug, m, app.Shop.Version)
}

// channelFSBlocks is how much space the stored channel needs on the console,
// counted in the Wii's 16 KiB filesystem blocks. The confirmation page shows
// it in 128 KiB Wii blocks, like the original shop.
func (s *storefront) channelFSBlocks(app catalogApp) int {
	var total int64
	if m, err := channelpkg.Load(s.channelsRoot, strings.ToLower(classicTitleIDForApp(app))); err == nil {
		names := append([]string{"tmd", "ticket.bin"}, m.Contents...)
		for _, name := range names {
			if st, err := os.Stat(filepath.Join(s.channelsRoot, m.TitleID, name)); err == nil {
				total += st.Size()
			}
		}
	}
	// A channel that is only in the catalogue (icons imported, WAD on the
	// external host) still has a real size there. Use it so the confirmation
	// and download screens show the true block count instead of 0.
	if total <= 0 {
		total = app.Size
	}
	if total <= 0 {
		return 0
	}
	return int((total + 16383) / 16384)
}

func canSynthesize(app catalogApp) bool {
	// Real licensed WADs (Virtual Console, WiiWare, ...) are imported as-is
	// from their external host and never need the SD installer / env gating.
	if app.Licensed {
		return true
	}
	if os.Getenv("SHROOMBREW_DIRECT_SD") != "1" || os.Getenv("SHROOMBREW_NATIVE_EC") != "1" {
		return false
	}
	if !channelpkg.ValidSDApp(app.Slug) {
		return false
	}
	// Flash titles are injected from an SWF (not a native app package), so the
	// homebrew package-size limit does not apply to them.
	if !app.Flash && app.FileSize.ZipCompressed > channelpkg.MaxPayload {
		return false
	}
	_, err := approvedPackageURL(app.URL.Zip)
	return err == nil
}

// ensureChannel returns once a matching channel package exists. Concurrent
// requests for the same app share one build.
func (s *storefront) ensureChannel(ctx context.Context, app catalogApp) error {
	if s.hasNativeSDPackage(app) {
		return nil
	}
	if !canSynthesize(app) {
		return errors.New("this app cannot be packaged as a channel")
	}
	id := strings.ToLower(classicTitleIDForApp(app))
	s.channels.mu.Lock()
	if existing := s.channels.builds[id]; existing != nil {
		s.channels.mu.Unlock()
		select {
		case <-existing.done:
			return existing.err
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	build := &channelBuild{done: make(chan struct{})}
	s.channels.builds[id] = build
	s.channels.mu.Unlock()

	buildCtx, cancel := context.WithTimeout(context.Background(), 6*time.Minute)
	err := s.buildChannel(buildCtx, app)
	cancel()

	s.channels.mu.Lock()
	build.err = err
	delete(s.channels.builds, id)
	close(build.done)
	s.channels.mu.Unlock()
	return err
}

func (s *storefront) buildChannel(ctx context.Context, app catalogApp) error {
	id := classicTitleIDForApp(app)
	started := time.Now()
	log.Printf("channel %s: building for %s version %d", id, app.Slug, app.Shop.Version)

	select {
	case s.channels.slot <- struct{}{}:
		defer func() { <-s.channels.slot }()
	case <-ctx.Done():
		return ctx.Err()
	}

	if app.Flash {
		return s.buildFlashChannel(ctx, app, id, started)
	}
	if app.Licensed {
		return s.buildLicensedChannel(ctx, app, id, started)
	}

	u, err := approvedPackageURL(app.URL.Zip)
	if err != nil {
		return err
	}
	payload, err := fetchBytes(ctx, s.installs.client, u.String(), channelpkg.MaxPayload)
	if err != nil {
		return fmt.Errorf("download app package: %w", err)
	}
	version := synthVersion(app)

	work, err := os.MkdirTemp("", "shroombrew-channel-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(work)

	zipPath := filepath.Join(work, "app.zip")
	if err := os.WriteFile(zipPath, payload, 0600); err != nil {
		return err
	}
	// The same validation the ZIP installer uses: safe names, no duplicates,
	// every file regular, and apps/<slug>/boot.dol present.
	if _, err := extractInstallArchive(zipPath, filepath.Join(work, "extracted"), app.Slug); err != nil {
		return fmt.Errorf("validate app package: %w", err)
	}

	icon := s.iconFor(ctx, app)
	wad, err := channelpkg.BuildChannel(channelpkg.Channel{
		Slug:         app.Slug,
		Name:         app.Name,
		Author:       app.Author,
		Version:      app.Version,
		Description:  app.Description.Short,
		Category:     app.Category,
		ReleaseDate:  time.Unix(app.ReleaseDate, 0),
		IconPNG:      icon,
		Payload:      payload,
		TitleID:      mustTitleID(id),
		TitleVersion: version,
	})
	if err != nil {
		return fmt.Errorf("build channel: %w", err)
	}
	manifest, err := channelpkg.Publish(wad, s.channelsRoot, app.Slug, id)
	if err != nil {
		return fmt.Errorf("publish channel: %w", err)
	}
	log.Printf("channel %s: ready, %d bytes, version %d, took %s", id, len(wad), manifest.Version, time.Since(started).Round(time.Second))
	return nil
}

// buildFlashChannel injects a catalog Flash movie into the operator-supplied
// official Flash runtime WAD and publishes it as a fakesigned Wii channel.
func (s *storefront) buildFlashChannel(ctx context.Context, app catalogApp, id string, started time.Time) error {
	swfURL, err := approvedPackageURL(app.URL.Zip)
	if err != nil {
		return err
	}
	swf, err := fetchBytes(ctx, s.installs.client, swfURL.String(), flashFetchLimit)
	if err != nil {
		return fmt.Errorf("download Flash movie: %w", err)
	}
	template, err := s.flashTemplate()
	if err != nil {
		return err
	}
	icon := s.iconFor(ctx, app)
	version := synthVersion(app)

	wad, err := flashpkg.Inject(flashpkg.Options{
		Template:        template,
		SWF:             swf,
		TitleID:         mustTitleID(id),
		TitleVersion:    version,
		Region:          0xffff, // region-free
		Name:            app.Name,
		Author:          app.Author,
		Description:     app.Description.Short,
		Category:        app.Category,
		ReleaseDate:     time.Unix(app.ReleaseDate, 0),
		IconPNG:         icon,
		DisplayVersion:  app.Version,
		Zoom:            "auto",
		Mouse:           true,
		Quality:         "high",
	})
	if err != nil {
		return fmt.Errorf("inject Flash channel: %w", err)
	}
	manifest, err := channelpkg.Publish(wad, s.channelsRoot, app.Slug, id)
	if err != nil {
		return fmt.Errorf("publish Flash channel: %w", err)
	}
	log.Printf("channel %s: ready (Flash), %d bytes, version %d, took %s", id, len(wad), manifest.Version, time.Since(started).Round(time.Second))
	return nil
}

// buildLicensedChannel imports a real retail Wii Shop WAD from its external
// host (mariocube) the first time it is downloaded. Shroombrew does not host
// these files; it fetches them on demand and caches the split package under
// ./channels/<titleid>/, which the console's native CCS/ECS download serves.
func (s *storefront) buildLicensedChannel(ctx context.Context, app catalogApp, id string, started time.Time) error {
	u, err := approvedPackageURL(app.URL.Zip)
	if err != nil {
		return err
	}
	wad, err := fetchBytes(ctx, s.installs.client, u.String(), 512<<20)
	if err != nil {
		return fmt.Errorf("download licensed WAD: %w", err)
	}
	manifest, err := channelpkg.ImportChannel(wad, s.channelsRoot, strings.ToLower(id))
	if err != nil {
		return fmt.Errorf("import licensed WAD: %w", err)
	}
	log.Printf("channel %s: ready (licensed WAD), %d bytes, version %d, took %s", id, len(wad), manifest.Version, time.Since(started).Round(time.Second))
	return nil
}

// flashTemplate returns the operator-supplied official Flash runtime WAD. Its
// path is configured with SHROOMBREW_FLASH_TEMPLATE (default ./flash-template.wad).
func (s *storefront) flashTemplate() ([]byte, error) {
	data, err := os.ReadFile(s.flashTemplatePath)
	if err != nil {
		return nil, fmt.Errorf("Flash base WAD (%s) could not be read: %w", s.flashTemplatePath, err)
	}
	return data, nil
}

// iconFor prefers the icon already served by the shop, then the catalog URL.
func (s *storefront) iconFor(ctx context.Context, app catalogApp) []byte {
	if data, err := os.ReadFile(filepath.Join(s.iconsRoot, app.Slug+".png")); err == nil && len(data) <= 1<<20 {
		return data
	}
	if app.URL.Icon == "" {
		return nil
	}
	u, err := approvedPackageURL(app.URL.Icon)
	if err != nil {
		return nil
	}
	data, err := fetchBytes(ctx, s.installs.client, u.String(), 1<<20)
	if err != nil {
		log.Printf("channel %s: icon: %v", app.Slug, err)
		return nil
	}
	return data
}

func fetchBytes(ctx context.Context, client *http.Client, address string, limit int64) ([]byte, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
	if err != nil {
		return nil, err
	}
	response, err := client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("source returned %s", response.Status)
	}
	if response.ContentLength > limit {
		return nil, errors.New("package exceeds size limit")
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, errors.New("package exceeds size limit")
	}
	return data, nil
}

func mustTitleID(id string) uint64 {
	var value uint64
	for _, r := range strings.ToLower(id) {
		value <<= 4
		switch {
		case r >= '0' && r <= '9':
			value |= uint64(r - '0')
		case r >= 'a' && r <= 'f':
			value |= uint64(r-'a') + 10
		}
	}
	return value
}
