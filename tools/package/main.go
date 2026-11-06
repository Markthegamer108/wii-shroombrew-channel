package main

import (
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
)

type content struct {
	Index int    `json:"index"`
	Size  int    `json:"size"`
	SHA1  string `json:"sha1"`
}
type tmd struct {
	TitleID  string    `json:"title_id"`
	Version  int       `json:"version"`
	Contents []content `json:"contents"`
}
type ticket struct {
	TitleID string `json:"title_id"`
	Note    string `json:"note"`
}
type ment struct {
	File   string `json:"file"`
	SHA256 string `json:"sha256"`
}

func exists(p string) bool            { fi, err := os.Stat(p); return err == nil && !fi.IsDir() }
func fail(f string, a ...interface{}) { fmt.Fprintf(os.Stderr, f+"\n", a...); os.Exit(1) }
func mustWrite(p string, v interface{}) {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		fail("write %s: %v", p, err)
	}
	if err := os.WriteFile(p, b, 0644); err != nil {
		fail("write %s: %v", p, err)
	}
}
func hashOf(p string) ment {
	b, err := os.ReadFile(p)
	if err != nil {
		fail("read %s: %v", p, err)
	}
	h := sha256.Sum256(b)
	return ment{filepath.Base(p), hex.EncodeToString(h[:])}
}
func main() {
	if len(os.Args) != 4 {
		fail("usage: %s <input-dir> <title-id> <version>", os.Args[0])
	}
	in, tid, vs := os.Args[1], os.Args[2], os.Args[3]
	ver, err := strconv.Atoi(vs)
	if err != nil {
		fail("invalid version %q: %v", vs, err)
	}
	if !exists(filepath.Join(in, "boot.elf")) && !exists(filepath.Join(in, "boot.dol")) {
		fail("missing boot.elf or boot.dol in %s", in)
	}
	if !exists(filepath.Join(in, "icon.png")) {
		fail("missing icon.png in %s", in)
	}
	if !exists(filepath.Join(in, "meta.xml")) {
		fail("missing meta.xml in %s", in)
	}
	if err := os.MkdirAll("out", 0755); err != nil {
		fail("create out: %v", err)
	}
	var srcs []string
	w := func(p string, fi os.FileInfo, e error) error {
		if e != nil {
			return e
		}
		if fi.IsDir() {
			return nil
		}
		b := filepath.Base(p)
		if b == "icon.png" || b == "meta.xml" {
			return nil
		}
		srcs = append(srcs, p)
		return nil
	}
	if err := filepath.Walk(in, w); err != nil {
		fail("scan %s: %v", in, err)
	}
	sort.Strings(srcs)
	list := []content{}
	for i, s := range srcs {
		d, err := os.ReadFile(s)
		if err != nil {
			fail("read %s: %v", s, err)
		}
		h := sha1.Sum(d)
		o := fmt.Sprintf("out/content_%d.bin", i)
		if err := os.WriteFile(o, d, 0644); err != nil {
			fail("write %s: %v", o, err)
		}
		list = append(list, content{i, len(d), hex.EncodeToString(h[:])})
	}
	mustWrite("out/tmd.json", tmd{tid, ver, list})
	mustWrite("out/ticket.json", ticket{tid, "fakesigned placeholder"})
	var m []ment
	for i := range srcs {
		m = append(m, hashOf(fmt.Sprintf("out/content_%d.bin", i)))
	}
	m = append(m, hashOf("out/tmd.json"))
	m = append(m, hashOf("out/ticket.json"))
	mustWrite("out/manifest.json", m)
	fmt.Printf("packaged %d contents to out/\n", len(srcs))
}
