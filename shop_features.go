package main

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Markthegamer108/wii-shroombrew-channel/internal/channelpkg"
	"github.com/jackc/pgx/v4"
)

func initShopPurchases() error {
	_, err := pool.Exec(context.Background(), `
	 CREATE TABLE IF NOT EXISTS shroombrew_purchases (
	  wallet text NOT NULL REFERENCES shroombrew_wallets(id), slug text NOT NULL,
	  price integer NOT NULL CHECK(price >= 0), created_at timestamptz NOT NULL DEFAULT now(),
	  delivered_at timestamptz, PRIMARY KEY(wallet, slug));
	 CREATE TABLE IF NOT EXISTS shroombrew_download_requests (
	  id text PRIMARY KEY, wallet text NOT NULL REFERENCES shroombrew_wallets(id),
	  slug text NOT NULL, created_at timestamptz NOT NULL DEFAULT now());`)
	return err
}

func csrfToken(id string) string {
	h := sha256.Sum256([]byte("shroombrew-form:" + id))
	return hex.EncodeToString(h[:])
}

func validShopPost(r *http.Request, id string) bool {
	if r.Method != http.MethodPost {
		return false
	}
	if origin := r.Header.Get("Origin"); origin != "" {
		u, err := url.Parse(origin)
		if err != nil || !strings.EqualFold(u.Host, r.Host) {
			return false
		}
	}
	r.Body = http.MaxBytesReader(nil, r.Body, 8192)
	return r.ParseForm() == nil && r.Form.Get("csrf") == csrfToken(id)
}

func (s *storefront) featureHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		if r.Header.Get("SOAPAction") != "" {
			next.ServeHTTP(w, r)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/community/api/") || r.URL.Path == "/healthz" || strings.HasPrefix(r.URL.Path, "/archive-shop/assets/") {
			if pagesCORS(w, r) {
				return
			}
		}
		if (r.Method == http.MethodGet || r.Method == http.MethodHead) && !isWiiShopUA(r) && siteHostMatches(r.Host) {
			if rewriteSiteRoute(w, r) {
				return
			}
		}
		switch {
		case r.URL.Path == "/" && (r.Method == http.MethodGet || r.Method == http.MethodHead) && !isWiiShopUA(r) && siteHostMatches(r.Host):
			// The public website host lands on the community site, not the shop front page.
			http.Redirect(w, r, "/community/index.html", http.StatusFound)
		case r.URL.Path == "/community":
			http.Redirect(w, r, "/community/", http.StatusFound)
		case r.URL.Path == "/community/":
			// Serve the community single-page site directly (index.html with hash routing)
			localPath := filepath.Join(nathanSiteRoot, "community", "index.html")
			http.ServeFile(w, r, localPath)
			return
		case strings.HasSuffix(r.URL.Path, ".html") && (strings.HasPrefix(r.URL.Path, "/community/") || strings.HasPrefix(r.URL.Path, "/shroombrew/")):
			name := strings.TrimSuffix(filepath.Base(r.URL.Path), ".html")
			if name == "index" {
				http.Redirect(w, r, "/community/", http.StatusMovedPermanently)
			} else {
				http.Redirect(w, r, "/community/#"+name, http.StatusMovedPermanently)
			}
			return
		case strings.HasPrefix(r.URL.Path, "/archive-shop/assets/") || r.URL.Path == "/archive-shop/checkout.js" || r.URL.Path == "/archive-shop/mario.js" || r.URL.Path == "/archive-shop/native-download.js":
			archivedShopAsset(w, r)
		case r.Method == http.MethodPost && strings.HasPrefix(r.URL.Path, "/api/install/") && strings.HasSuffix(r.URL.Path, "/complete"):
			parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/install/"), "/")
			if len(parts) == 2 && len(parts[0]) == 48 {
				_, err := pool.Exec(r.Context(), `UPDATE shroombrew_purchases p SET delivered_at=now() FROM shroombrew_download_requests d WHERE d.id=$1 AND p.wallet=d.wallet AND p.slug=d.slug`, parts[0])
				if err != nil {
					http.Error(w, "Completion unavailable; retry", 500)
					return
				}
			}
			next.ServeHTTP(w, r)
		case r.URL.Path == "/shroombrew-compat.js":
			data, _ := shopFiles.ReadFile("shop/compat.js")
			w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
			// Pages ask for it as compatURL (?v=<hash>); revalidate anyway so a
			// console holding an old unversioned copy picks up fixes.
			w.Header().Set("Cache-Control", "no-cache")
			w.Write(data)
		case strings.HasPrefix(r.URL.Path, "/community/api/"):
			s.communityAPI(w, r)
		case r.URL.Path == "/friends":
			s.friendsPage(w, r)
		case r.URL.Path == "/gift":
			s.giftPage(w, r)
		case r.URL.Path == "/rating":
			s.ratingPage(w, r)
		case r.URL.Path == "/manual":
			s.manualPage(w, r)
		case strings.HasPrefix(r.URL.Path, "/manual/"):
			s.manualFile(w, r)
		case r.URL.Path == "/link":
			s.linkPage(w, r)
		case r.URL.Path == "/notice":
			s.noticePage(w, r)
		case r.URL.Path == "/device/register":
			s.deviceRegister(w, r)
		case (r.URL.Path == "/" || r.URL.Path == "/startup") && r.Method == http.MethodGet && isWiiShopUA(r):
			s.connectingPage(w, r)
		case r.URL.Path == "/language":
			s.setLanguage(w, r)
		case r.URL.Path == "/shroombrew-season.js" || r.URL.Path == "/shroombrew-fx.js":
			s.seasonScript(w, r)
		case r.URL.Path == "/points":
			s.freePointsPage(w, r)
		case strings.HasPrefix(r.URL.Path, "/points/"):
			s.pointsFlow(w, r)
		case strings.HasPrefix(r.URL.Path, "/settings/pc"):
			s.parentalPage(w, r)
		case r.URL.Path == "/help" || r.URL.Path == "/activity" || r.URL.Path == "/downloaded" || r.URL.Path == "/settings" || r.URL.Path == "/controllers" || r.URL.Path == "/news" || r.URL.Path == "/oss/serv/W_02.html":
			s.featureInfo(w, r)
		case strings.HasPrefix(r.URL.Path, "/wii-install/title/"):
			id := strings.ToUpper(strings.TrimPrefix(r.URL.Path, "/wii-install/title/"))
			app, ok := s.appByTitleID(id)
			if !ok {
				http.NotFound(w, r)
				return
			}
			app = s.regionApp(app, r)
			if r.Method == http.MethodGet && isWiiClient(r) && s.hasNativeSDPackage(app) {
				// The title page picks the destination: the Wii System
				// Memory / SD Card choice for channels, or the SD
				// installer for packages that are one.
				http.Redirect(w, r, "/native-ec/title/"+classicTitleIDForApp(app), http.StatusSeeOther)
				return
			}
			s.checkout(w, r, app)
		case strings.HasPrefix(r.URL.Path, "/download/"):
			app, ok := s.bySlug[strings.TrimPrefix(r.URL.Path, "/download/")]
			if !ok {
				http.NotFound(w, r)
				return
			}
			http.Redirect(w, r, "/wii-install/title/"+classicTitleIDForApp(app), http.StatusSeeOther)
		case strings.HasPrefix(r.URL.Path, "/download-file/"):
			s.ownedFileDownload(w, r)
		case strings.HasPrefix(r.URL.Path, "/shop-download/"):
			s.downloadFlow(w, r)
		case r.URL.Path == "/wii-install/start" || r.URL.Path == "/api/install/start" || strings.HasPrefix(r.URL.Path, "/api/install/native/"):
			if strings.HasPrefix(r.URL.Path, "/api/install/native/") && os.Getenv("SHROOMBREW_DIRECT_SD") == "1" {
				slug := strings.TrimPrefix(r.URL.Path, "/api/install/native/")
				app, exists := s.bySlug[slug]
				if exists {
					manifest, err := channelpkg.Load(s.channelsRoot, strings.ToLower(classicTitleIDForApp(app)))
					if err == nil && channelPackageMatches(app, manifest) {
						next.ServeHTTP(w, r)
						return
					}
				}
				http.Error(w, "Reviewed SD installer package required", http.StatusConflict)
				return
			}
			// The prototype's native bootstrap is deliberately limited to isolated tests.
			if os.Getenv("SHROOMBREW_NATIVE_INSTALL_TEST") == "1" {
				next.ServeHTTP(w, r)
				return
			}
			http.Error(w, "Use the title checkout to prepare a homebrew package.", http.StatusConflict)
		default:
			next.ServeHTTP(w, r)
		}
	})
}

// siteProjects maps the public website's project routes to the folder that holds
// each project's pages under ./site. Shroombrew keeps its original /community/
// folder because the Wii pages and translations already point there.
var siteProjects = map[string]string{"shroombrew": "community"}

// rewriteSiteRoute adds /<project>/ routes on the public website host. It never
// touches "/" (the Wii Shop mirror) or "/community/". It redirects when the
// request needs a trailing slash (relative asset links depend on it) and
// otherwise rewrites the path in place. It reports whether the response was
// already written.
func rewriteSiteRoute(w http.ResponseWriter, r *http.Request) bool {
	p := r.URL.Path
	name := strings.Trim(p, "/")
	rest := ""
	if i := strings.Index(name, "/"); i >= 0 {
		name, rest = name[:i], name[i+1:]
	}
	dir, ok := siteProjects[name]
	if !ok {
		return false
	}
	if rest == "" && !strings.HasSuffix(p, "/") {
		http.Redirect(w, r, "/"+name+"/", http.StatusMovedPermanently)
		return true
	}
	if rest == "" {
		rest = "index.html"
	}
	// Only the project's own files are reachable through its route.
	if strings.Contains(rest, "..") || strings.Contains(rest, "\\") {
		http.NotFound(w, r)
		return true
	}
	r.URL.Path = "/" + dir + "/" + rest
	return false
}

func shopLink(url, text string) string { return uiButton(url, text) }

// healthSafetyURL points a finished game download at B_13 and carries the
// title page to return to after the warning. Channels and homebrew skip it,
// as the original shop did.
func healthSafetyURL(app catalogApp) string {
	switch app.Category {
	case "Virtual Console", "WiiWare", "DLC":
		id := classicTitleIDForApp(app)
		back := "/oss/serv/B_05.html?titleId=" + id
		return "/oss/serv/B_13.html?titleId=" + id + "&back=" + url.QueryEscape(back)
	}
	return ""
}

func (s *storefront) checkout(w http.ResponseWriter, r *http.Request, app catalogApp) {
	id, err := walletID(w, r)
	if err != nil {
		http.Error(w, "Wallet unavailable", 500)
		return
	}
	points, err := walletBalance(id)
	if err != nil {
		http.Error(w, "Wallet unavailable", 500)
		return
	}
	price := classicPriceFor(app)
	var owned bool
	if err = pool.QueryRow(r.Context(), `SELECT EXISTS(SELECT 1 FROM shroombrew_purchases WHERE wallet=$1 AND slug=$2)`, id, app.Slug).Scan(&owned); err != nil {
		http.Error(w, "Wallet unavailable", 500)
		return
	}
	if owned {
		price = 0
	}
	if r.Method == http.MethodPost {
		if !validShopPost(r, id) {
			http.Error(w, "Reload the page and try again", 403)
			return
		}
		if points < price {
			http.Redirect(w, r, "/points?return="+url.QueryEscape(r.URL.Path+"?confirm=1"), 303)
			return
		}
		// A PC browser cannot use the console's installer: it gets the WAD or
		// homebrew ZIP as an ordinary file in its Downloads folder.
		if !isWiiClient(r) {
			if _, ok := s.downloadTarget(s.regionApp(app, r)); !ok {
				writeArchivedCheckout(w, r, archivedCheckoutData{Page: "B_10", Name: app.Name, Balance: points, Back: r.URL.Path, Message: "The package is unavailable. No points were charged."})
				return
			}
			if err = purchaseOnce(r.Context(), id, app.Slug, price); err != nil {
				http.Error(w, "Not enough points or wallet unavailable. Add points and retry.", 409)
				return
			}
			recordPurchaseVersion(r.Context(), id, app.Slug, app.Shop.Version)
			logDownload(r.Context(), app.Slug, "pc")
			if points, err = walletBalance(id); err != nil {
				http.Error(w, "Wallet unavailable", 500)
				return
			}
			writeArchivedCheckout(w, r, archivedCheckoutData{Page: "B_10", Mode: "pc", Name: app.Name, Balance: points, Action: "/download-file/" + app.Slug, FSBlocks: s.channelFSBlocks(app), Back: "/oss/serv/B_05.html?titleId=" + classicTitleIDForApp(app), Health: healthSafetyURL(app), TitleID: classicTitleIDForApp(app), TitleVersion: app.Shop.Version})
			return
		}
		// Apps that fit a channel are downloaded by the Shop's own native
		// installer. The channel is built here, before any points are taken.
		if canSynthesize(app) {
			if !s.hasNativeSDPackage(app) {
				if err = s.ensureChannel(r.Context(), app); err != nil {
					log.Printf("channel for %s: %v", app.Slug, err)
					writeArchivedCheckout(w, r, archivedCheckoutData{Page: "B_10", Name: app.Name, Balance: points, Back: r.URL.Path, Message: "The channel could not be prepared. No points were charged."})
					return
				}
			}
			if err = purchaseOnce(r.Context(), id, app.Slug, price); err != nil {
				http.Error(w, "Not enough points or wallet unavailable. Add points and retry.", 409)
				return
			}
			recordPurchaseVersion(r.Context(), id, app.Slug, app.Shop.Version)
			logDownload(r.Context(), app.Slug, "channel")
			// The next screen asks Wii System Memory or SD Card, like B_09 did.
			http.Redirect(w, r, "/native-ec/title/"+classicTitleIDForApp(app), http.StatusSeeOther)
			return
		}
		job, err := s.installs.start(app)
		if err != nil {
			writeArchivedCheckout(w, r, archivedCheckoutData{Page: "B_10", Name: app.Name, Balance: points, Back: r.URL.Path, Message: "The package is unavailable. No points were charged."})
			return
		}
		if _, err = pool.Exec(r.Context(), `INSERT INTO shroombrew_download_requests(id,wallet,slug) VALUES($1,$2,$3)`, job.ID, id, app.Slug); err != nil {
			http.Error(w, "Download unavailable", 500)
			return
		}
		logDownload(r.Context(), app.Slug, "sd")
		http.Redirect(w, r, "/shop-download/prepare/"+job.ID, 303)
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", 405)
		return
	}
	controls := controllerText(app)
	if controls == "" {
		controls = "Controller information not supplied"
	}
	data := archivedCheckoutData{Page: "B_08", Name: app.Name, Controllers: controls, ControllerIcons: controllerIcons(app), ControllerExtra: controllerExtra(app), Balance: points, Price: price, Remaining: points - price, CSRF: csrfToken(id), Action: r.URL.Path, Back: "/oss/serv/B_05.html?titleId=" + classicTitleIDForApp(app), Next: r.URL.Path + "?confirm=1"}
	if r.URL.Query().Get("confirm") == "1" {
		data.Page = "B_09"
		data.Back = r.URL.Path
		data.FSBlocks = s.channelFSBlocks(app)
	}
	if data.Remaining < 0 {
		data.Remaining = 0
	}
	writeArchivedCheckout(w, r, data)
}

// ownedFileDownload sends a PC browser to the WAD or homebrew ZIP of a title
// its wallet has bought, so the browser saves it to the Downloads folder.
func (s *storefront) ownedFileDownload(w http.ResponseWriter, r *http.Request) {
	app, ok := s.bySlug[strings.TrimPrefix(r.URL.Path, "/download-file/")]
	if !ok {
		http.NotFound(w, r)
		return
	}
	id, err := walletID(w, r)
	if err != nil {
		http.Error(w, "Wallet unavailable", 500)
		return
	}
	var owned bool
	if err = pool.QueryRow(r.Context(), `SELECT EXISTS(SELECT 1 FROM shroombrew_purchases WHERE wallet=$1 AND slug=$2)`, id, app.Slug).Scan(&owned); err != nil || !owned {
		http.Redirect(w, r, "/wii-install/title/"+classicTitleIDForApp(app), http.StatusSeeOther)
		return
	}
	target, ok := s.downloadTarget(s.regionApp(app, r))
	if !ok {
		http.Error(w, "Download unavailable", http.StatusServiceUnavailable)
		return
	}
	pool.Exec(r.Context(), `UPDATE shroombrew_purchases SET delivered_at=now() WHERE wallet=$1 AND slug=$2`, id, app.Slug)
	// First download of this title pays the play reward.
	awardPlayPoints(r.Context(), id, app.Slug)
	http.Redirect(w, r, target, http.StatusFound)
}

func (s *storefront) ownedJob(r *http.Request, id, wallet string) (*installJob, catalogApp, bool) {
	var slug string
	if err := pool.QueryRow(r.Context(), `SELECT slug FROM shroombrew_download_requests WHERE id=$1 AND wallet=$2`, id, wallet).Scan(&slug); err != nil {
		return nil, catalogApp{}, false
	}
	app, ok := s.bySlug[slug]
	if !ok {
		return nil, app, false
	}
	job, ok := s.installs.find(id)
	return job, app, ok
}

func (s *storefront) downloadFlow(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/shop-download/"), "/")
	if len(parts) != 2 || len(parts[1]) != 48 {
		http.NotFound(w, r)
		return
	}
	id, err := walletID(w, r)
	if err != nil {
		http.Error(w, "Wallet unavailable", 500)
		return
	}
	job, app, ok := s.ownedJob(r, parts[1], id)
	if !ok {
		writeArchivedCheckout(w, r, archivedCheckoutData{Page: "B_10", Back: "/oss/serv/W_03.html", Message: "This download expired. Return to the shop and prepare it again."})
		return
	}
	job.mu.Lock()
	state, problem, bytes, total := job.State, job.Error, job.Bytes, job.Total
	job.Updated = time.Now().UTC()
	job.mu.Unlock()
	if parts[0] == "status" && r.Method == http.MethodGet {
		if r.URL.Query().Get("format") == "text" {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			fmt.Fprintf(w, "%s\t%d\t%d", state, bytes, total)
			return
		}
		writeInstallJSON(w, map[string]interface{}{"state": state, "error": problem, "bytes": bytes, "total": total})
		return
	}
	if parts[0] == "installer" && r.Method == http.MethodPost {
		if !validShopPost(r, id) {
			http.Error(w, "Reload the page and try again", 403)
			return
		}
		job.mu.Lock()
		ready := job.State == "ready" && !job.retired
		if ready {
			job.readers++
		}
		job.mu.Unlock()
		if !ready {
			http.Error(w, "Package is not ready", 409)
			return
		}
		defer job.release()
		if err := verifyPreparedJob(job); err != nil {
			http.Error(w, "Package verification failed. No points were charged.", 502)
			return
		}
		if err := purchaseOnce(r.Context(), id, app.Slug, classicPriceFor(app)); err != nil {
			http.Error(w, "Not enough points or wallet unavailable", 409)
			return
		}
		points, err := walletBalance(id)
		if err != nil {
			http.Error(w, "Wallet unavailable", 500)
			return
		}
		writeArchivedCheckout(w, r, archivedCheckoutData{Page: "B_10", Mode: "installer", Code: job.Code, Name: app.Name, Balance: points, Back: "/oss/serv/B_05.html?titleId=" + classicTitleIDForApp(app), Health: healthSafetyURL(app), Job: job.ID})
		return
	}
	if parts[0] == "zip" && r.Method == http.MethodPost {
		if !validShopPost(r, id) {
			http.Error(w, "Reload the page and try again", 403)
			return
		}
		if state != "ready" {
			http.Error(w, "Package is not ready", 409)
			return
		}
		// Claim a ready job once, so parallel clicks cannot race workspace cleanup.
		job.mu.Lock()
		if job.State != "ready" || job.retired || job.readers != 0 {
			job.mu.Unlock()
			http.Error(w, "Download already in progress", 409)
			return
		}
		job.State = "delivering"
		job.readers++
		job.Updated = time.Now().UTC()
		job.mu.Unlock()
		defer job.release()
		defer func() {
			job.mu.Lock()
			if job.State == "delivering" {
				job.State = "ready"
				job.Updated = time.Now().UTC()
			}
			job.mu.Unlock()
		}()
		if err := verifyPreparedJob(job); err != nil {
			http.Error(w, "Package verification failed. No points were charged.", 502)
			return
		}
		packagePath := filepath.Join(job.Workspace, "delivery.zip")
		if err := buildDeliveryZIP(job, packagePath); err != nil {
			http.Error(w, "Package unavailable. No points were charged.", 502)
			return
		}
		defer os.Remove(packagePath)
		packageFile, err := os.Open(packagePath)
		if err != nil {
			http.Error(w, "Package unavailable. No points were charged.", 502)
			return
		}
		defer packageFile.Close()
		info, err := packageFile.Stat()
		if err != nil {
			http.Error(w, "Package unavailable", 502)
			return
		}
		if err := purchaseOnce(r.Context(), id, app.Slug, classicPriceFor(app)); err != nil {
			http.Error(w, "Not enough points or wallet unavailable. Add points and retry.", 409)
			return
		}
		recordPurchaseVersion(r.Context(), id, app.Slug, app.Shop.Version)
		w.Header().Set("Content-Type", "application/zip")
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="Shroombrew-%s.zip"`, app.Slug))
		w.Header().Set("Content-Length", fmt.Sprint(info.Size()))
		if written, err := io.Copy(w, packageFile); err == nil && written == info.Size() {
			pool.Exec(r.Context(), `UPDATE shroombrew_purchases SET delivered_at=now() WHERE wallet=$1 AND slug=$2`, id, app.Slug)
		}
		s.discardJob(job)
		return
	}
	if parts[0] != "prepare" || r.Method != http.MethodGet {
		http.NotFound(w, r)
		return
	}
	points, err := walletBalance(id)
	if err != nil {
		http.Error(w, "Wallet unavailable", 500)
		return
	}
	writeArchivedCheckout(w, r, archivedCheckoutData{Page: "B_10", Name: app.Name, Balance: points, CSRF: csrfToken(id), Action: "/shop-download/zip/" + job.ID, Back: "/oss/serv/B_05.html?titleId=" + classicTitleIDForApp(app), Health: healthSafetyURL(app), Job: job.ID, State: state})
}

func purchaseOnce(ctx context.Context, wallet, slug string, price int) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var points int
	if err := tx.QueryRow(ctx, `SELECT points FROM shroombrew_wallets WHERE id=$1 FOR UPDATE`, wallet).Scan(&points); err != nil {
		return err
	}
	var existing int
	err = tx.QueryRow(ctx, `SELECT price FROM shroombrew_purchases WHERE wallet=$1 AND slug=$2`, wallet, slug).Scan(&existing)
	if err == nil {
		return tx.Commit(ctx)
	}
	if err != pgx.ErrNoRows {
		return err
	}
	if points < price {
		return fmt.Errorf("insufficient points")
	}
	if _, err = tx.Exec(ctx, `UPDATE shroombrew_wallets SET points=points-$2,updated_at=now() WHERE id=$1`, wallet, price); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO shroombrew_purchases(wallet,slug,price) VALUES($1,$2,$3)`, wallet, slug, price); err != nil {
		return err
	}
	if price > 0 {
		if err = ledgerAdd(ctx, tx, wallet, -price, "purchase", slug); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func verifyPreparedJob(job *installJob) error {
	job.mu.Lock()
	job.Updated = time.Now().UTC()
	manifest := job.Manifest
	job.mu.Unlock()
	if len(manifest.Files) == 0 {
		return fmt.Errorf("empty package")
	}
	for _, item := range manifest.Files {
		if _, err := safeInstallPath(item.Path); err != nil {
			return err
		}
		f, err := os.Open(filepath.Join(job.Workspace, "extracted", filepath.FromSlash(item.Path)))
		if err != nil {
			return err
		}
		h := sha256.New()
		n, err := io.Copy(h, f)
		f.Close()
		if err != nil {
			return err
		}
		if n != item.Size || hex.EncodeToString(h.Sum(nil)) != item.SHA256 {
			return fmt.Errorf("file integrity mismatch")
		}
	}
	return nil
}

func (s *storefront) discardJob(job *installJob) {
	s.installs.mu.Lock()
	delete(s.installs.jobs, job.ID)
	delete(s.installs.codes, job.Code)
	s.installs.mu.Unlock()
	job.mu.Lock()
	job.State = "complete"
	job.Updated = time.Now().UTC()
	job.retireLocked()
	job.mu.Unlock()
}

// Build a complete archive before debiting the wallet or sending HTTP headers.
func buildDeliveryZIP(job *installJob, destination string) error {
	out, err := os.Create(destination)
	if err != nil {
		return err
	}
	defer out.Close()
	z := zip.NewWriter(out)
	job.mu.RLock()
	manifest := job.Manifest
	job.mu.RUnlock()
	for _, item := range manifest.Files {
		f, err := os.Open(filepath.Join(job.Workspace, "extracted", filepath.FromSlash(item.Path)))
		if err != nil {
			z.Close()
			return err
		}
		dst, err := z.Create(item.Path)
		if err == nil {
			_, err = io.Copy(dst, f)
		}
		f.Close()
		if err != nil {
			z.Close()
			return err
		}
	}
	f, err := z.Create("shroombrew-manifest.json")
	if err == nil {
		err = json.NewEncoder(f).Encode(manifest)
	}
	if err != nil {
		z.Close()
		return err
	}
	if err = z.Close(); err != nil {
		return err
	}
	return out.Close()
}

// hasNativeSDPackage reports whether a title already has a matching channel
// package (built earlier or imported), so Download can use the Shop's own
// native download flow without preparing one now.
func (s *storefront) hasNativeSDPackage(app catalogApp) bool {
	if os.Getenv("SHROOMBREW_DIRECT_SD") != "1" || os.Getenv("SHROOMBREW_NATIVE_EC") != "1" {
		return false
	}
	manifest, err := channelpkg.Load(s.channelsRoot, strings.ToLower(classicTitleIDForApp(app)))
	return err == nil && channelPackageMatches(app, manifest)
}
