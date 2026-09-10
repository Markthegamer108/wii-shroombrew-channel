package main

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
)

const oscAPIUrl = "https://hbb1.oscwii.org/api/v3/contents"

type OSCApp struct {
	Shop Shop `json:"shop"`
}

type Shop struct {
	TitleId string `json:"title_id"`
	Version int    `json:"title_version"`
}

func GetOSCApp(titleId string) (*OSCApp, error) {
	resp, err := http.Get(oscAPIUrl)
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
		if app.Shop.TitleId == titleId {
			return &app, nil
		}
	}

	return nil, nil
}
