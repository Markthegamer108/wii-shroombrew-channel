package main

import (
	"net/http"
	"os"
)

const (
	// wadInstallFile is the WSC-Patcher build for the wiirw.bot.nu base domain.
	// It is kept outside the repository because it contains Nintendo code.
	wadInstallFile = "./wads/WiiShop-wiirw-20261030.wad"
	wadInstallName = "WiiShop-Shroombrew-v21.wad"
	wadInstallSHA  = "8b9e46bf8a6cd85d6c6624e366eedc4b8245f9fcc0bb014df26448cc10c3a0ca"
)

func (s *storefront) wadDownload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	info, err := os.Stat(wadInstallFile)
	if err != nil || !info.Mode().IsRegular() {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", `attachment; filename="`+wadInstallName+`"`)
	w.Header().Set("Cache-Control", "no-store")
	http.ServeFile(w, r, wadInstallFile)
}
