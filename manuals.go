package main

import (
	"archive/zip"
	"bytes"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// The CCS folders include the original operations guides as ZIP archives of
// HTML, images and page scripts. The shop opened them in the same Opera view.
// This serves the archive directly, so relative image and page links keep
// working under /manual/<titleId>/....

type manualRef struct {
	zip   string
	entry string // path relative to the archive's html/ directory
}

var manualCache = struct {
	sync.Mutex
	m map[string]manualRef
}{m: map[string]manualRef{}}

var manualZips = struct {
	sync.Mutex
	m map[string]*zip.ReadCloser
}{m: map[string]*zip.ReadCloser{}}

func manualLang(lang string) string {
	switch lang {
	case "ja":
		return "ja"
	case "ko":
		return "ko"
	case "de", "fr", "es", "it", "nl":
		return lang
	}
	return "en"
}

// findManual picks an archive and entry for a title and language. The entry is
// index.html when the archive has one, otherwise the first page in the wanted
// language, otherwise any page.
func findManual(titleID, lang string) (manualRef, bool) {
	key := titleID + "|" + manualLang(lang)
	manualCache.Lock()
	if ref, ok := manualCache.m[key]; ok {
		manualCache.Unlock()
		return ref, ref.zip != ""
	}
	manualCache.Unlock()

	dir := filepath.Join(nathanSiteRoot, "oss", "ccs", titleID)
	files, err := os.ReadDir(dir)
	if err != nil {
		return manualRef{}, false
	}
	want := "/" + manualLang(lang) + "/"
	var best manualRef
	for _, file := range files {
		if file.IsDir() || !strings.HasPrefix(file.Name(), "FFFD") {
			continue
		}
		zipPath := filepath.Join(dir, file.Name())
		zr, err := zip.OpenReader(zipPath)
		if err != nil {
			continue
		}
		var rootIndex, langIndex, langPage, anyPage string
		for _, f := range zr.File {
			name := f.Name
			lower := strings.ToLower(name)
			if !strings.HasSuffix(lower, ".html") && !strings.HasSuffix(lower, ".htm") {
				continue
			}
			rel := strings.TrimPrefix(name, "html/")
			if rel == name {
				continue
			}
			base := strings.ToLower(path.Base(rel))
			isIndex := base == "index.html" || base == "index.htm"
			if isIndex && strings.EqualFold(rel, base) {
				rootIndex = rel
			}
			if isIndex && strings.Contains(strings.ToLower(rel), want) && langIndex == "" {
				langIndex = rel
			}
			if strings.Contains(strings.ToLower(rel), want) && langPage == "" {
				langPage = rel
			}
			if anyPage == "" {
				anyPage = rel
			}
		}
		zr.Close()
		switch {
		case langIndex != "":
			best = manualRef{zip: zipPath, entry: langIndex}
		case rootIndex != "":
			best = manualRef{zip: zipPath, entry: rootIndex}
		case langPage != "":
			best = manualRef{zip: zipPath, entry: langPage}
		case anyPage != "":
			best = manualRef{zip: zipPath, entry: anyPage}
		}
		if best.zip != "" {
			break
		}
	}
	manualCache.Lock()
	manualCache.m[key] = best
	manualCache.Unlock()
	return best, best.zip != ""
}

func manualZip(path string) (*zip.ReadCloser, error) {
	manualZips.Lock()
	defer manualZips.Unlock()
	if zr, ok := manualZips.m[path]; ok {
		return zr, nil
	}
	zr, err := zip.OpenReader(path)
	if err != nil {
		return nil, err
	}
	manualZips.m[path] = zr
	return zr, nil
}

// manualPage redirects /manual?titleId=... to the archive's entry page.
func (s *storefront) manualPage(w http.ResponseWriter, r *http.Request) {
	id := strings.ToUpper(r.URL.Query().Get("titleId"))
	if len(id) != 16 {
		http.NotFound(w, r)
		return
	}
	ref, ok := findManual(id, langFor(r))
	if !ok {
		http.Error(w, "No operations guide is available for this title.", http.StatusNotFound)
		return
	}
	http.Redirect(w, r, "/manual/"+id+"/"+ref.entry, http.StatusFound)
}

// manualFile serves one file from the first archive that contains it. Pages
// reference their images and other pages relatively, so the URL path mirrors
// the archive's html/ tree.
func (s *storefront) manualFile(w http.ResponseWriter, r *http.Request) {
	rest := strings.TrimPrefix(r.URL.Path, "/manual/")
	slash := strings.Index(rest, "/")
	if slash != 16 {
		http.NotFound(w, r)
		return
	}
	id := strings.ToUpper(rest[:slash])
	rel := path.Clean(rest[slash+1:])
	if strings.HasPrefix(rel, "..") || strings.Contains(rel, "\\") {
		http.NotFound(w, r)
		return
	}
	dir := filepath.Join(nathanSiteRoot, "oss", "ccs", id)
	files, err := os.ReadDir(dir)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	for _, file := range files {
		if file.IsDir() || !strings.HasPrefix(file.Name(), "FFFD") {
			continue
		}
		zipPath := filepath.Join(dir, file.Name())
		zr, err := manualZip(zipPath)
		if err != nil {
			continue
		}
		for _, f := range zr.File {
			name := f.Name
			if !strings.EqualFold(name, "html/"+rel) {
				continue
			}
			rc, err := f.Open()
			if err != nil {
				continue
			}
			data := new(bytes.Buffer)
			_, _ = data.ReadFrom(rc)
			rc.Close()
			body := data.Bytes()
			if strings.HasSuffix(strings.ToLower(rel), ".html") || strings.HasSuffix(strings.ToLower(rel), ".htm") {
				// Manuals call the shop's native helpers for their Back button.
				// The browser viewer supplies small ES3 stand-ins.
				helper := []byte(`<script type="text/javascript">function wiiCloseManual(){history.back();}function wiiFocusSound(){}function wiiSelectSound(){}function wiiCancelSound(){}function trace(){}var snd={playSE:function(){},playBGM:function(){}};</script>`)
				if i := bytes.LastIndex(bytes.ToLower(body), []byte("</body>")); i >= 0 {
					body = append(append(append([]byte{}, body[:i]...), helper...), body[i:]...)
				} else {
					body = append(body, helper...)
				}
			}
			w.Header().Set("Cache-Control", "no-cache")
			w.Header().Set("Content-Type", manualContentType(rel))
			http.ServeContent(w, r, path.Base(rel), time.Time{}, bytes.NewReader(body))
			return
		}
	}
	http.NotFound(w, r)
}

func manualContentType(name string) string {
	switch strings.ToLower(path.Ext(name)) {
	case ".html", ".htm":
		return "text/html; charset=utf-8"
	case ".gif":
		return "image/gif"
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".css":
		return "text/css"
	case ".js":
		return "application/javascript"
	case ".swf":
		return "application/x-shockwave-flash"
	case ".txt":
		return "text/plain; charset=utf-8"
	}
	return "application/octet-stream"
}
