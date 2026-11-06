package main

// Translations for the Wii-facing pages. They are read from the same JSON files
// the website uses (lang/<code>.json), so a translator edits one place and both
// the console and the site change. Missing keys fall back to English.

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

var langDir = "./site/community/lang"

// Every language the Wii system menu can be set to, plus Portuguese.
var supportedLangs = map[string]string{
	"en": "English", "ja": "日本語", "de": "Deutsch", "fr": "Français", "es": "Español",
	"it": "Italiano", "nl": "Nederlands", "ko": "한국어", "zh-cn": "简体中文", "zh-tw": "繁體中文", "pt": "Português",
}
var langOrder = []string{"en", "ja", "de", "fr", "es", "it", "nl", "ko", "zh-cn", "zh-tw", "pt"}

type langFile struct {
	data    map[string]string
	mod     time.Time
	checked time.Time
}

var (
	langMu    sync.Mutex
	langCache = map[string]*langFile{}
)

func loadLang(code string) map[string]string {
	if _, ok := supportedLangs[code]; !ok {
		return nil
	}
	langMu.Lock()
	defer langMu.Unlock()
	lf := langCache[code]
	now := time.Now()
	if lf != nil && now.Sub(lf.checked) < 10*time.Second {
		return lf.data
	}
	path := filepath.Join(langDir, code+".json")
	st, err := os.Stat(path)
	if err != nil {
		if lf != nil {
			lf.checked = now
			return lf.data
		}
		return nil
	}
	if lf != nil && st.ModTime().Equal(lf.mod) {
		lf.checked = now
		return lf.data
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	m := map[string]string{}
	if json.Unmarshal(b, &m) != nil {
		if lf != nil { // keep serving the last good copy if an edit broke the file
			lf.checked = now
			return lf.data
		}
		return nil
	}
	langCache[code] = &langFile{data: m, mod: st.ModTime(), checked: now}
	return m
}

// tl returns a translated string. vars are key/value pairs for {key} slots.
func tl(lang, key string, vars ...string) string {
	s, ok := loadLang(lang)[key]
	if !ok || s == "" {
		s, ok = loadLang("en")[key]
		if !ok {
			s = key
		}
	}
	for i := 0; i+1 < len(vars); i += 2 {
		s = strings.ReplaceAll(s, "{"+vars[i]+"}", vars[i+1])
	}
	return s
}

// tmap collects translated strings for templates.
func tmap(lang string, keys ...string) map[string]string {
	m := make(map[string]string, len(keys))
	for _, k := range keys {
		m[k] = tl(lang, k, "site", publicSite())
	}
	return m
}

func langFor(r *http.Request) string {
	if c, err := r.Cookie("shroombrew_lang"); err == nil {
		if _, ok := supportedLangs[c.Value]; ok {
			return c.Value
		}
	}
	return "en"
}

// wiiLangCode maps the console's system-menu language (as reported by the shop
// client's device info) to one of our language files.
func wiiLangCode(language, country string) string {
	l := strings.ToLower(strings.TrimSpace(language))
	l = strings.ReplaceAll(l, "_", "-")
	switch {
	case l == "":
		return "en"
	case strings.HasPrefix(l, "zh"):
		if strings.Contains(l, "tw") || strings.Contains(l, "hant") || strings.HasSuffix(l, "-t") {
			return "zh-tw"
		}
		if strings.Contains(l, "cn") || strings.Contains(l, "hans") || strings.HasSuffix(l, "-s") {
			return "zh-cn"
		}
		switch strings.ToUpper(country) {
		case "TW", "HK", "MO":
			return "zh-tw"
		}
		return "zh-cn"
	}
	base := strings.SplitN(l, "-", 2)[0]
	if _, ok := supportedLangs[base]; ok {
		return base
	}
	if base == "jp" {
		return "ja"
	}
	return "en"
}

// safeLocalPath only allows same-site absolute paths as redirect targets.
func safeLocalPath(p string) string {
	if !strings.HasPrefix(p, "/") || strings.HasPrefix(p, "//") || strings.ContainsAny(p, "\\\r\n") {
		return "/settings"
	}
	return p
}
