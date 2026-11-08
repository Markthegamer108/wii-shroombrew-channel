package main

import (
	"fmt"
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

func (s *storefront) wadPage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	info, err := os.Stat(wadInstallFile)
	size := int64(0)
	available := err == nil && info.Mode().IsRegular()
	if available {
		size = info.Size()
	}
	status := "The WAD is not built on this host yet."
	if available {
		status = fmt.Sprintf("%s &middot; %d bytes", wadInstallSHA, size)
	}
	download := ""
	if available {
		download = `<p><a class="btn" href="/wad/` + wadInstallName + `">Download ` + wadInstallName + `</a></p>`
	}
	content := `<div class="panel">
<h1>Install Shroombrew on a real Wii</h1>
<p>This is a patched copy of the original Wii Shop Channel v21. It behaves like the
real channel, but connects to <b>wiirw.bot.nu</b> instead of Nintendo. The WAD is
already patched and includes the matching trust certificate; you do not need to
patch or rebuild anything.</p>
` + download + `
<h2>Before you install</h2>
<ul>
<li>This is the <b>USA v21</b> channel for an <b>original Wii</b>. Do not install it on a vWii or Wii U.</li>
<li>Make a NAND backup first. Replacing the Shop Channel is a system-title change.</li>
<li>Use a signature-bypassing WAD installer. YAWM ModMii Edition or Wii Mod Lite
running under IOS249 is the recommended path. Do not use an installer that refuses
fakesigned system titles.</li>
<li>Keep the original Shop Channel WAD somewhere safe so you can restore it.</li>
</ul>
<h2>Install</h2>
<ol>
<li>Copy <b>` + wadInstallName + `</b> to the WAD folder on your SD card or USB drive.</li>
<li>Launch your WAD manager and install it to the <b>Wii System Memory (NAND)</b>.</li>
<li>Return to the System Menu and open <b>Wii Shop Channel</b>.</li>
<li>The channel now shows Shroombrew and connects to this server. No DNS or
certificate changes are needed on the console.</li>
</ol>
<h2>If it does not connect</h2>
<p>Check the console date/time and internet connection first. The channel needs
TLS to <b>oss-auth.wiirw.bot.nu</b>, <b>ecs.wiirw.bot.nu</b>,
<b>ias.wiirw.bot.nu</b> and <b>ccs.wiirw.bot.nu</b>. If those names do not resolve,
the channel cannot reach the shop. If it still tries Nintendo or shows a shop
error, delete the <b>Wii Shop Channel save data</b> in Data Management (not the
channel itself) and launch it again.</p>
<h2>File</h2>
<p><code>` + status + `</code></p>
<p class="muted">The WAD contains Nintendo's original Shop Channel code. Keep it
private and only install it on a console you own.</p>
<p><a href="/">Back to Shroombrew</a></p>
</div>`
	page := `<!DOCTYPE html><html><head><meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>Install Shroombrew on a real Wii</title>
<style>
body{margin:0;background:#eaf8ff;color:#323232;font-family:Arial,Helvetica,sans-serif}
.panel{max-width:760px;margin:40px auto;background:#fff;border:1px solid #9edcf5;border-radius:14px;padding:28px 34px;box-shadow:0 4px 18px rgba(52,190,237,.18)}
h1{color:#34beed;margin-top:0}h2{color:#34beed;font-size:20px;margin-top:28px}
a{color:#087ea9}.btn{display:inline-block;background:#35beed;color:#fff;text-decoration:none;padding:12px 20px;border-radius:9px;font-weight:bold}
code{background:#f2fbff;padding:2px 5px;border-radius:4px}.muted{color:#777;font-size:13px}
li{margin:7px 0}
</style></head><body>` + content + `</body></html>`
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Write([]byte(page))
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
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", `attachment; filename="`+wadInstallName+`"`)
	w.Header().Set("Cache-Control", "no-store")
	http.ServeFile(w, r, wadInstallFile)
}
