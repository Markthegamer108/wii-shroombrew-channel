package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"
)

// Every download started from the shop is logged; Popular Titles sorts by
// these counts.

func initDownloadLog() error {
	_, err := pool.Exec(context.Background(), `
	 CREATE TABLE IF NOT EXISTS shroombrew_downloads (
	  slug text NOT NULL, kind text NOT NULL, at timestamptz NOT NULL DEFAULT now());
	 CREATE INDEX IF NOT EXISTS shroombrew_downloads_slug ON shroombrew_downloads(slug);`)
	return err
}

// logDownload records one download of slug. kind is how it was delivered:
// "pc" (WAD/ZIP file), "channel" (native install) or "sd" (SD installer).
func logDownload(ctx context.Context, slug, kind string) {
	if _, err := pool.Exec(ctx, `INSERT INTO shroombrew_downloads(slug, kind) VALUES($1, $2)`, slug, kind); err != nil {
		log.Printf("download log %s: %v", slug, err)
	}
	popular.mu.Lock()
	popular.at = time.Time{}
	popular.mu.Unlock()
}

var popular struct {
	mu   sync.Mutex
	at   time.Time
	body []byte
}

// popularScript serves the counts as `var shroomPopular={slug:count,...}` for
// the catalog page. It is rebuilt at most once a minute.
func popularScript(w http.ResponseWriter, r *http.Request) {
	popular.mu.Lock()
	defer popular.mu.Unlock()
	if popular.body == nil || time.Since(popular.at) > time.Minute {
		counts := map[string]int{}
		rows, err := pool.Query(r.Context(), `SELECT slug, count(*) FROM shroombrew_downloads GROUP BY slug`)
		if err == nil {
			for rows.Next() {
				var slug string
				var n int
				if rows.Scan(&slug, &n) == nil {
					counts[slug] = n
				}
			}
			rows.Close()
		} else {
			log.Printf("popular titles: %v", err)
		}
		data, _ := json.Marshal(counts)
		popular.body = append(append([]byte("var shroomPopular="), data...), ';', '\n')
		popular.at = time.Now()
	}
	w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Write(popular.body)
}
