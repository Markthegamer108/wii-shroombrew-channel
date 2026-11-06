package main

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"

	"github.com/jackc/pgx/v4"
)

type config struct {
	SQLAddress string `xml:"SQLAddress"`
	SQLUser    string `xml:"SQLUser"`
	SQLPass    string `xml:"SQLPass"`
	SQLDB      string `xml:"SQLDB"`
}

type content struct {
	Shop struct {
		TitleID      string `json:"title_id"`
		TitleVersion int    `json:"title_version"`
	} `json:"shop"`
}

func main() {
	var data []byte
	var err error
	for _, p := range []string{"../config.xml", "../../config.xml", "config.xml"} {
		data, err = os.ReadFile(p)
		if err == nil {
			break
		}
	}
	if err != nil {
		log.Fatalf("read config: %v", err)
	}
	var cfg config
	if err := xml.Unmarshal(data, &cfg); err != nil {
		log.Fatalf("parse config: %v", err)
	}
	resp, err := http.Get("https://hbb1.oscwii.org/api/v3/contents")
	if err != nil {
		log.Fatalf("fetch contents: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		log.Fatalf("fetch contents: %s", resp.Status)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatalf("read contents: %v", err)
	}
	var apps []content
	if err := json.Unmarshal(body, &apps); err != nil {
		log.Fatalf("parse contents: %v", err)
	}
	u := &url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(cfg.SQLUser, cfg.SQLPass),
		Host:   cfg.SQLAddress,
		Path:   "/" + cfg.SQLDB,
	}
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, u.String())
	if err != nil {
		log.Fatalf("db connect: %v", err)
	}
	defer conn.Close(ctx)
	var maxID int
	if err := conn.QueryRow(ctx, "SELECT COALESCE(MAX(item_id), 999) FROM service_titles").Scan(&maxID); err != nil {
		log.Fatalf("max item_id: %v", err)
	}
	nextID := maxID + 1
	if nextID < 1000 {
		nextID = 1000
	}
	added, skipped := 0, 0
	for _, a := range apps {
		tid := a.Shop.TitleID
		if tid == "" {
			continue
		}
		var one int
		err := conn.QueryRow(ctx, "SELECT 1 FROM service_titles WHERE title_id=$1", tid).Scan(&one)
		if err == nil {
			skipped++
			continue
		}
		if err != pgx.ErrNoRows {
			log.Fatalf("query %s: %v", tid, err)
		}
		if _, err := conn.Exec(ctx, "INSERT INTO service_titles (item_id, price_code, price, title_id, reference_id) VALUES ($1, 0, 0, $2, NULL)", nextID, tid); err != nil {
			log.Fatalf("insert %s: %v", tid, err)
		}
		nextID++
		added++
	}
	fmt.Printf("added: %d, skipped: %d\n", added, skipped)
}
