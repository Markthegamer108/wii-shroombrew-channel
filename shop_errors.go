package main

// Error screens for the Wii Shop Channel. Handlers report problems with
// http.Error, which is a bare line of text: on the console that shows as a
// blank page with the message in the corner and none of the shop around it.
// shopErrorPages turns those replies into the shop's own error screen when the
// Shop Channel is loading a page. Scripts, images, SOAP and API calls keep the
// plain reply their callers expect.

import (
	"html"
	"net/http"
	"path"
	"strconv"
	"strings"
)

// shopPageRequest reports whether the console is loading a page (not a
// script, image, content file or API reply).
func shopPageRequest(r *http.Request) bool {
	if !isWiiShopUA(r) || r.Header.Get("SOAPAction") != "" || r.Header.Get("X-Requested-With") != "" {
		return false
	}
	p := r.URL.Path
	for _, prefix := range []string{"/api/", "/community/api/", "/device/", "/ccs/", "/oss/ccs/", "/ecs/", "/ias/", "/cas/", "/icons/", "/shop-download/status/", "/archive-shop/assets/"} {
		if strings.HasPrefix(p, prefix) {
			return false
		}
	}
	switch strings.ToLower(path.Ext(p)) {
	case "", ".html", ".htm", ".jsp":
		return true
	}
	return false
}

func shopErrorPages(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !shopPageRequest(r) {
			next.ServeHTTP(w, r)
			return
		}
		ew := &errorPageWriter{ResponseWriter: w}
		next.ServeHTTP(ew, r)
		if ew.code != 0 {
			writeShopError(w, r, ew.code)
		}
	})
}

// errorPageWriter holds back plain-text error replies; everything else passes
// straight through.
type errorPageWriter struct {
	http.ResponseWriter
	code    int  // set when an error reply was held back
	decided bool // headers have been written or passed on
}

func (e *errorPageWriter) WriteHeader(code int) {
	if e.decided {
		return
	}
	e.decided = true
	if code >= 400 && strings.HasPrefix(e.Header().Get("Content-Type"), "text/plain") {
		e.code = code
		return
	}
	e.ResponseWriter.WriteHeader(code)
}

func (e *errorPageWriter) Write(b []byte) (int, error) {
	if !e.decided {
		e.WriteHeader(http.StatusOK)
	}
	if e.code != 0 {
		return len(b), nil // the shop error screen replaces this text
	}
	return e.ResponseWriter.Write(b)
}

// Flush keeps streamed downloads working through the wrapper.
func (e *errorPageWriter) Flush() {
	if f, ok := e.ResponseWriter.(http.Flusher); ok && e.code == 0 {
		f.Flush()
	}
}

// writeShopError draws the shop's error screen: "Error Code:" and the message
// in red on the page, as the original error pages did, with Back and Shop Home.
func writeShopError(w http.ResponseWriter, r *http.Request, code int) {
	lang := langFor(r)
	msg := tl(lang, "er_generic")
	switch {
	case code == http.StatusNotFound:
		msg = tl(lang, "er_notfound")
	case code == http.StatusForbidden:
		msg = tl(lang, "er_reload")
	case code == http.StatusConflict || code == http.StatusBadRequest || code >= 500:
		msg = tl(lang, "er_unavailable")
	}
	h := w.Header()
	h.Del("Content-Length")
	h.Del("X-Content-Type-Options")
	h.Set("Content-Type", "text/html; charset=utf-8")
	h.Set("Cache-Control", "no-cache")
	w.WriteHeader(code)
	content := `<div class="panel plain"><p class="err" style="font-size:22px;line-height:28px;margin-top:6px">` +
		html.EscapeString(tl(lang, "n_err")) + ` ` + strconv.Itoa(code) + `</p><p class="err">` + html.EscapeString(msg) + `</p></div>` +
		`<div class="foot"><a class="opt" href="javascript:history.back()">` + html.EscapeString(tl(lang, "w_back")) + `</a>` + uiButton("/oss/serv/W_03.html", tl(lang, "w_home")) + `</div>`
	_, _ = w.Write([]byte(shopShell(tl(lang, "n_shop"), content)))
}
