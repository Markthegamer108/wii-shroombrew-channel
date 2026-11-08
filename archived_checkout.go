package main

import (
	"bytes"
	"embed"
	"encoding/hex"
	"html/template"
	"io/fs"
	"net/http"
	"strings"
)

//go:embed shop/archive
var archivedShopFiles embed.FS

var archivedShopTemplates = template.Must(template.ParseFS(archivedShopFiles, "shop/archive/*.html"))

type archivedCheckoutData struct {
	Page, Name, Controllers, CSRF, Action, Back, Next, Job, State, Message, Code, Mode string
	// Health is the B_13 Health & Safety page shown after a game download;
	// channels skip it and return straight to Back.
	Health string
	// ControllerIcons are B_08's controller images (banner/B_08_<key>.gif)
	// and ControllerExtra its second note; see controllerIcons.
	ControllerIcons           []string
	ControllerExtra           string
	Balance, Price, Remaining int
	// FSBlocks is the channel's size on the console in 16 KB filesystem blocks;
	// the confirmation page shows it in 128 KB Wii blocks like the original.
	FSBlocks                            int
	TitleID, Destination, ServiceDomain string
	// TitleVersion is remembered in the browser when a PC download completes so
	// the title detail page can show Downloaded/Updated on return.
	TitleVersion int
	// FreeBlocks is the console's open Wii system-memory blocks (2163 by
	// default). The confirmation page checks it before starting a download.
	FreeBlocks int
	// Licensed selects the real-shop "software" wording on B_09 instead of
	// the homebrew package wording.
	Licensed bool
	// Points page: translated strings, the public website address and a
	// one-line result message (Flash) shown after a card is entered.
	T        map[string]string
	D        map[string]string // English -> translated text for the page script
	Site     string
	Flash    template.HTML
	FlashBad bool
	// Card is the Wii Points card artwork for the console's region.
	Card string
	// Add Wii Points screens (points_flow.go): the amount being added or on
	// the card, P_06's buttons and balance limit, the made-up card P_08 shows
	// (CardNumber is the entered number on P_04), and P_15's new balance.
	Points, NewBalance, Cap                         int
	Options                                         []pointsOption
	CardNumber, CardType, CardExpMonth, CardExpYear string
	CardCode                                        string
}

// Only return to a known local title checkout after adding free points.
func archivedPointsReturn(r *http.Request) string {
	value := r.URL.Query().Get("return")
	title := strings.TrimPrefix(strings.TrimSuffix(value, "?confirm=1"), "/wii-install/title/")
	if strings.HasPrefix(value, "/wii-install/title/") && len(title) == 16 {
		if _, err := hex.DecodeString(title); err == nil {
			return "/wii-install/title/" + strings.ToUpper(title) + "?confirm=1"
		}
	}
	return "/oss/serv/W_03.html"
}

// archiveLiterals maps every English sentence the archived checkout pages
// show (in their markup or in checkout.js) to its translation key. The page
// script looks text up in this table, so one dictionary translates both.
var archiveLiterals = map[string]string{
	"Selection Confirmation": "x_selconf", "Download this homebrew package for {n} Wii Points.": "x_buy",
	"Download this software for {n} Wii Points.": "x_buy_soft",
	"You need more Wii Points.":                  "x_need", "Download this software?": "x_confirm", "Delivery:": "x_delivery",
	"Destination:": "x_dest", "Card": "x_card", "Installation:": "x_install", "Manual": "x_manual",
	"Download": "x_download", "Add Wii Points": "c_title", "Back": "w_back", "Wii Points": "f_points",
	"Quit":                                 "c_quit",
	"Preparing software for your SD card.": "x_prep", "Your homebrew package is ready.": "x_ready",
	"Use Shroombrew Installer on your Wii, or": "x_useinst", "download the ZIP instead.": "x_zip",
	"Install on Wii": "x_inst_wii", "Install to SD Card": "x_inst_sd", "Your install code: {code}": "x_code",
	"Press HOME, open the Homebrew Channel, and launch Shroombrew Installer. Enter this code within 15 minutes. Keep your SD card inserted.": "x_press",
	"Need the installer?": "x_needinst", "Download Shroombrew Installer": "x_dlinst",
	"Connection interrupted. Return and try again.": "x_interrupted", "Package verified. Finishing...": "x_verified",
	"Download unavailable. No points were charged.": "x_failed", "Verifying software...": "x_verifying",
	"Downloading software...": "x_downloading", "Connecting. Please wait.": "x_connecting",
	"The package is unavailable. No points were charged.":             "x_unavail",
	"This download expired. Return to the shop and prepare it again.": "x_expired",
	"Download Software": "x_dl_title", "You are downloading": "x_ing", "Wii Points after Download:": "x_pts_after",
	"Blocks after Download:": "x_blocks_after", "Blocks": "x_blocks",
	"There is not enough space in the Wii System Memory.":                                                     "x_nospace",
	"Please note that Wii Points will not be deducted from your account until the transaction is successful.": "x_note",
	"Visit support.nintendo.com for assistance.":                                                              "x_support",
	"You can download this software again without using any Wii Points.":                                      "x_again",
	"This offer has expired.":                                                                                 "x_offer_exp", "You may no longer download a title for free.": "x_nofree",
	// native (on-console) download flow and the confirmation labels
	"Waiting for native EC progress...": "n_wait", "Native SD interface unavailable.": "n_nosd",
	"Insert an SD card before downloading.": "n_insert", "Install this app to SD Card?": "n_q_apps",
	"Download to SD Card?": "n_q_sd", "Download to Wii System Memory?": "n_q_nand",
	"Native channel installation. Do not turn off the console.": "n_dont_off",
	"Downloading software": "n_p_dl", "Checking registration": "n_p_check", "Registering console": "n_p_reg",
	"Syncing registration": "n_p_sync", "Starting SD installer": "n_p_start", "Preparing SD Card...": "n_prep_sd",
	"Transferring software to SD Card...": "n_transfer", "Do not remove the SD card.": "n_dont_remove",
	"Starting SD installation...": "n_start_inst", "Download complete.": "n_complete",
	"Installed to the SD Card Menu.": "n_inst_sd", "Installed to Wii System Memory.": "n_inst_nand",
	"The SD card was removed.": "n_sd_removed", "Starting the SD installer. Keep the SD card inserted.": "n_start_sd",
	"Download Confirmation": "n_confirm", "Current Wii Points:": "n_cur_pts", "Wii Points to Download:": "n_pts_dl",
	"Open Blocks:": "n_open_blocks", "Blocks to Download:": "n_blocks_dl", "Error Code:": "n_err",
	"Please note the following when downloading this software:": "n_note8",
}

// archiveDictionary builds the English-to-translation table for one language.
// English needs none, and a missing translation simply keeps the English text.
func archiveDictionary(lang string) map[string]string {
	if lang == "en" {
		return nil
	}
	d := make(map[string]string, len(archiveLiterals))
	for en, key := range archiveLiterals {
		if s := tl(lang, key); s != key {
			d[en] = s
		}
	}
	return d
}

func writeArchivedCheckout(w http.ResponseWriter, r *http.Request, data archivedCheckoutData) {
	data.D = archiveDictionary(langFor(r))
	var body bytes.Buffer
	if err := archivedShopTemplates.ExecuteTemplate(&body, data.Page+".html", data); err != nil {
		http.Error(w, "Shop page unavailable", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Shroombrew-Checkout", "archive-2019")
	w.Write(body.Bytes())
}

func archivedShopAsset(w http.ResponseWriter, r *http.Request) {
	assets, _ := fs.Sub(archivedShopFiles, "shop/archive")
	// These files are updated with the server. Always revalidate so a console
	// never keeps an old checkout script after a deploy.
	w.Header().Set("Cache-Control", "no-cache")
	http.StripPrefix("/archive-shop/", http.FileServer(http.FS(assets))).ServeHTTP(w, r)
}
