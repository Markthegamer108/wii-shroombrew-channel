# Shroombrew

A homebrew shop built on WiiSOAP, with a permissioned copy of Nathan's wiishopchannel.net browser frontend and a separate native Wii compatibility frontend. The server's homebrew catalog is supplied by Open Shop Channel.

## Browser reference copy — October 17, 2026

`reference-site/` contains 4,160 files downloaded from https://wiishopchannel.net/, including 32 HTML documents, the original scripts, catalog data, artwork, font, music, sound effects and animation sprites. `reference-snapshot.json` records their source URLs and SHA-256 hashes. Imported files keep their original bytes and notices. The HTTP handler only rebases the reference's hard-coded origins and frame URL checks to this deployment, and reproduces its clean HTML URLs.

The browser runs the reference's original behavior, including its Nintendo catalog, external download links and local browser settings/points. This frontend is a copy of the public recreation. Earlier native Wii clients continue through `site/` and the Shroombrew compatibility scripts, homebrew catalog, wallet and package preparation handlers. The browser copy does not emulate a native installation or replace those server APIs.

## Native v2 test client — October 17, 2026

The new v21-based WAD identifies itself as `Wii Shop Channel/21.0(S)`. `native_v2.go` selects `wii-site/` for that client only, serves the welcome HTML directly at the root and connects the copied layouts to Shroombrew's catalog, points and package preparation routes. The bundle contains 29 pages and 187 scripts converted and syntax-checked against ECMAScript 3. Assets retain the downloaded reference artwork; a separate native bridge provides the state normally supplied by the desktop iframe shell.

The WAD passed all 18 TMD content hash checks, the service-domain checks, the IOS/Opera trust-store checks and the Opera allowlist check. The server tests and browser compatibility-preview checks pass. Launching Dolphin was blocked by automatic approval review, so emulator boot, physical Wii behavior, native audio/keyboard and all settings controls remain unverified. This client is a test build and does not complete automatic channel installation.

For a standalone browser preview without PostgreSQL, run `go run reference_site.go reference_preview.go -root reference-site`. A reverse proxy must set `X-Forwarded-Proto` to the actual public scheme so iframe navigation stays on the correct origin.

The archived original Nintendo frontend at https://archive.org/details/WiiShopChannelBackup is a separate asset/reference source. Public reference links that already fail upstream are recorded in the snapshot manifest; they have not been replaced with invented pages.

## Native frontend and homebrew API — October 4, 2026

- Browse games, emulators and other homebrew, with three titles per catalog page.
- View title images, descriptions, authors, controller information and stable prices of 100–500 free play points.
- Add 100 free Wii Points per click; no payment or Nintendo account is involved.
- Prepare a verified homebrew ZIP, download it, and extract its `apps` folder to an SD card for the Homebrew Channel.
- See purchases and download history; repeat downloads are free within the same wallet.
- Open local help, important info, settings and controller pages.
- Browser music begins with user interaction. Wii pages use the native sound interface when available.

The server fetches packages from the approved Open Shop Channel source, checks archive paths and size limits, extracts them into a temporary directory, and generates a file manifest containing sizes and SHA-256 hashes. It builds and verifies the delivery ZIP before deducting points. Temporary ZIPs and extracted files are removed after delivery or expiry. Failed preparation does not charge points.

The wallet is tied to an HTTP-only browser cookie. Clearing it loses access to that wallet. These play points are not Nintendo currency.

## Native Wii installation status

Downloading a ZIP in a browser **does not install a Wii channel**. Automatic SD writes and forwarder booting passed isolated Dolphin tests using a separate homebrew bootstrap. Installing that forwarder through the Shop's native ticket-import path still fails signature validation in Dolphin. Native Shop installation and physical-Wii compatibility are not verified. The experimental native routes and legacy pairing checkout are disabled in production.

Native Wii audio, physical hardware behavior and the idle timer still require verification on the target client. Copying the browser reference does not verify these native behaviors.

## Running and verification

Use `config.example.xml` to create a private `config.xml`, configure PostgreSQL and the existing WiiSOAP schema in `database.sql`, and supply `catalog.json`, `icons/`, `site/`, and `reference-site/` in the working directory. Startup creates the additive wallet, purchase and download-request tables. The optional `SHROOMBREW_ADDR` environment variable overrides the listen address.

Run `go test ./...` and `go build .`. With a configured test database, `SHOP_INTEGRATION_CONFIG=/path/to/config.xml go test ./...` also checks points, download integrity, repeated purchases, concurrent deductions and temporary-file cleanup using disposable wallets. Keep configuration secrets, console credentials, keys and NAND backups out of public source bundles.

Production uses the existing Wii-compatible proxy and certificate configuration for the patched Shop client. Native experiments remain isolated; `SHROOMBREW_NATIVE_INSTALL_TEST=1` is for an explicitly configured test service only.

## Credits and license

Thanks to Nathan / Comp Arison of [wiishopchannel.net](https://wiishopchannel.net/) for permission to adapt his frontend and for directing us to the preserved [Wii Shop Channel HTML backup](https://archive.org/details/WiiShopChannelBackup). Thanks to [Open Shop Channel](https://oscwii.org/) for its catalog and [WiiSOAP](https://github.com/OpenShopChannel/WiiSOAP) for the server foundation.

The existing AGPL-3.0-or-later license in `LICENSE` is retained. Imported frontend files retain their attribution and notices. The matching server source is available at `/source/shroombrew-source.tar.gz`.

## Archived checkout restoration (2026-10-17)

The checkout now uses the captured B_08 (selection/controllers), B_09 (download confirmation), B_10 (download progress), and P_01 (points) markup, inline styles, shared CSS and image/button assets from the WiiShopChannelBackup archive. File provenance and original SHA-256 hashes are recorded in shop/archive/provenance.json. The Go server embeds these assets; no Java/JSP runtime is required. The account adapter in shop/archive/checkout.js uses ECMAScript 3 and supplies the real Shroombrew balance, CSRF-protected actions and package progress. Payment controls are adapted to free credits. Delivery remains a verified homebrew ZIP for manual SD installation.

The original WiiMart frontend source at https://github.com/WiiMart/WiiMart (commit 1ed60c77cfeb26cfa787fe4dbef3e081ece3c208) was inspected as an additional reference for its JSP purchase flow. This update imports its page artwork and markup from the existing local archive, rather than importing WiiMart's Java runtime or service configuration.

This server update works with the existing native-v2 S-marker WAD. Dolphin launch of that WAD was reported by the user as showing no errors; this is a user-reported boot result, not a hardware compatibility result. No automated tests were run for this checkout update.

## SD companion and Mario update (2026-10-17)

The prepared-package page now animates Mario, three blocks and coins using the permissioned browser recreation's sprite/sound assets, adapted to DOM sprites for Opera. Known byte progress drives the block milestones; server readiness triggers the final sequence. This animation represents server preparation, not completed SD writes.

Install on Wii checks the prepared file manifest, records an idempotent purchase and displays the job's eight-digit pairing code. The Homebrew Channel installer mounts SD with libfat, fetches verified files into temporary paths, and copies them into sd:/apps. An installer completion notification records delivery. ZIP delivery remains available. Existing app files are not overwritten by the companion. The current source is in tools-src/wii-installer; the installer ZIP is served at /tools/wii-installer.zip.

Homebrew Browser's storage and download sources were inspected as references (commit 3f47f8e5438135e350f1334cc9e0cb291b48237c). No Homebrew Browser code was imported. The installer was rebuilt with devkitPPC. No automated tests, Dolphin launch or physical Wii run were performed for this update; native SD behavior and exact Wii rendering remain unverified.

## SD merge and download lifecycle repairs — October 18, 2026

Installer 0.4 merges existing SD folders and replaces matching package files for updates, preserving unrelated files. Downloads are extracted on the server and delivered as individually hashed files. The writer stages the package before creating verified backups; detected write failures trigger rollback, retaining backup copies if restoration fails. This is not a durable power-loss recovery journal. The existing companion handoff is still required; automatic native launch from the Shop remains unfinished.

Server cancellation now cancels the preparation request and waits for the worker before removing its files. Active HTTP file and ZIP deliveries retain their workspace, and expired idle jobs are cleaned up. Only manifest-listed files are served. Archives with FAT filename collisions, unsupported filenames or no matching app executable are rejected.

The staged Go race tests and disposable-wallet database integration passed. Seven host C filesystem scenarios, nine HTTP client scenarios and six simulated native JavaScript controller cases passed. A real WiiDonut package prepared and verified on the server in 245 ms. The PowerPC installer compiled, but this build has not been run on Wii, vWii or Dolphin. A strict modern Python HTTPS check with the service CA failed because the legacy certificate lacks an Authority Key Identifier; certificate validation remains enabled and the live certificate was not replaced.
