package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestNativeContentRoutes(t *testing.T) {
	root := t.TempDir()
	title := "0001000154455354"
	dir := filepath.Join(root, title)
	os.MkdirAll(dir, 0755)
	files := map[string]string{"manifest.json": `{"title_id":"0001000154455354","version":1,"contents":["0000000a"]}`, "tmd": "binary metadata", "tmd.1": "version one", "0000000a": "encrypted content", "ticket.bin": "private ticket", "certs.bin": "private certs"}
	for n, b := range files {
		if e := os.WriteFile(filepath.Join(dir, n), []byte(b), 0600); e != nil {
			t.Fatal(e)
		}
	}
	h := nativeContentHandler(root, http.NotFoundHandler())
	for _, tc := range []struct {
		path string
		code int
	}{{"tmd", 200}, {"tmd.1", 200}, {"tmd.2", 404}, {"0000000a", 200}, {"0000000b", 404}, {"TMD", 200}, {"0000000A", 200}, {"ticket.bin", 404}, {"certs.bin", 404}, {"manifest.json", 404}, {"../config.xml", 404}} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", "/ccs/download/"+title+"/"+tc.path, nil))
		if w.Code != tc.code {
			t.Errorf("%s got %d", tc.path, w.Code)
		}
	}
	r := httptest.NewRequest("GET", "/ccs/download/"+title+"/0000000a", nil)
	r.Header.Set("Range", "bytes=0-3")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 206 || w.Body.String() != "encr" {
		t.Fatal("resume range failed")
	}
	w = httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("HEAD", "/ccs/download/"+title+"/tmd", nil))
	if w.Code != 200 || w.Body.Len() != 0 {
		t.Fatal("HEAD failed")
	}
	w = httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("POST", "/ccs/download/"+title+"/tmd", nil))
	if w.Code != 405 {
		t.Fatal("write method accepted")
	}
}
