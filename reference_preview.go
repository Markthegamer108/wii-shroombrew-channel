//go:build referencepreview

package main

import (
	"flag"
	"log"
	"net/http"
)

// Run with explicit files, independently of PostgreSQL:
// go run reference_site.go reference_preview.go -root ../reference
func main() {
	root := flag.String("root", "../reference", "Directory containing the reference snapshot")
	address := flag.String("addr", "127.0.0.1:8917", "Preview address")
	flag.Parse()
	log.Printf("Reference frontend: http://%s", *address)
	log.Fatal(http.ListenAndServe(*address, referenceSiteHandler(*root, http.NotFoundHandler())))
}
