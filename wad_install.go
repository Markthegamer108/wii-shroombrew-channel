package main

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"log"
	"net/http"
	"os"
	"sync"
)

const (
	// wadInstallFile is the WSC-Patcher build for the wiirw.bot.nu base domain.
	// It is kept outside the repository because it contains Nintendo code.
	wadInstallFile = "./wads/WiiShop-wiirw-20261030.wad"
	wadInstallName = "WiiShop-Shroombrew-v21.wad"
	wadInstallSHA  = "8b9e46bf8a6cd85d6c6624e366eedc4b8245f9fcc0bb014df26448cc10c3a0ca"
)

var (
	wadVerifyOnce sync.Once
	wadVerifyErr  error
)

// verifyWadInstall hashes the WAD once per process. The download route must not
// serve a file that no longer matches the checksum published on the site.
func verifyWadInstall() error {
	wadVerifyOnce.Do(func() {
		f, err := os.Open(wadInstallFile)
		if err != nil {
			wadVerifyErr = err
			return
		}
		defer f.Close()
		h := sha256.New()
		if _, err := io.Copy(h, f); err != nil {
			wadVerifyErr = err
			return
		}
		if sum := hex.EncodeToString(h.Sum(nil)); sum != wadInstallSHA {
			wadVerifyErr = errors.New("WAD checksum mismatch: " + sum)
		}
	})
	return wadVerifyErr
}

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
	if err := verifyWadInstall(); err != nil {
		log.Printf("WAD download refused: %v", err)
		http.Error(w, "WAD integrity check failed", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", `attachment; filename="`+wadInstallName+`"`)
	w.Header().Set("Cache-Control", "no-store")
	http.ServeFile(w, r, wadInstallFile)
}
