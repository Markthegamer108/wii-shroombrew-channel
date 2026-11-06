package channelpkg

// Turning one homebrew app into one installable Wii Menu channel.

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Channel is the input to BuildChannel.
type Channel struct {
	Slug        string    // sd:/apps/<slug>/ and the catalog's app slug
	Name        string    // shown in the channel title and banner
	Author      string
	Version     string    // display version, e.g. "1.2.0"
	Description string
	Category    string
	ReleaseDate time.Time
	IconPNG     []byte
	Payload     []byte // the app's ZIP as downloaded from the catalog
	TitleID     uint64 // 00010001xxxxxxxx
	TitleVersion uint16
	IOS         uint32
}

// BuildChannel returns a complete, fakesigned WAD.
func BuildChannel(c Channel) ([]byte, error) {
	if !ValidSDApp(c.Slug) {
		return nil, errors.New("invalid app slug")
	}
	if len(c.Payload) == 0 {
		return nil, errors.New("the app package is empty")
	}
	if len(c.Payload) > MaxPayload {
		return nil, fmt.Errorf("the app package is %d bytes; a channel can hold %d", len(c.Payload), MaxPayload)
	}
	if c.Name == "" || strings.TrimSpace(c.Name) == "" {
		return nil, errors.New("the app has no name")
	}

	marker := strings.TrimSpace(c.Version)
	if marker == "" {
		marker = "1.0.0"
	}
	sum := sha256.Sum256(c.Payload)
	marker += "+" + hex.EncodeToString(sum[:4])

	program, err := ForwarderDOL("sd:/apps/"+c.Slug+"/boot.dol", marker)
	if err != nil {
		return nil, err
	}
	banner, err := BuildBanner(BannerInfo{
		Title:       c.Name,
		Author:      c.Author,
		Version:     c.Version,
		Description: c.Description,
		Category:    c.Category,
		ReleaseDate: c.ReleaseDate,
		IconPNG:     c.IconPNG,
	})
	if err != nil {
		return nil, err
	}
	return Synthesize(SynthOptions{
		TitleID: c.TitleID,
		Version: c.TitleVersion,
		IOS:     c.IOS,
		Program: program,
		Payload: c.Payload,
		Banner:  banner,
	})
}
