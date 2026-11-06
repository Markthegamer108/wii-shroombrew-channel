package main

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"
)

const oscAPIUrl = "https://hbb1.oscwii.org/api/v3/contents"

var oscHTTPClient = &http.Client{Timeout: 15 * time.Second}

type OSCApp struct {
	Slug string `json:"slug"`
	Shop Shop   `json:"shop"`
}

type Shop struct {
	TitleId      string `json:"title_id"`
	Version      int    `json:"title_version"`
	ContentsSize int64  `json:"contents_size"`
}

func GetOSCApp(titleId string) (*OSCApp, error) {
	resp, err := oscHTTPClient.Get(oscAPIUrl)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, errors.New("osc API returned non OK status code")
	}

	data, _ := io.ReadAll(resp.Body)
	defer resp.Body.Close()

	var apps []OSCApp
	err = json.Unmarshal(data, &apps)
	if err != nil {
		return nil, err
	}

	for _, app := range apps {
		if strings.EqualFold(app.Shop.TitleId, titleId) {
			return &app, nil
		}
	}

	return nil, nil
}
