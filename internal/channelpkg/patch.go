package channelpkg

// Patching the one forwarder program the server builds channels from.

import (
	"bytes"
	_ "embed"
	"errors"
	"fmt"
	"regexp"
	"strings"
)

const (
	forwarderMagic      = "SHROOMBREW-FWD-CFG\x00"
	forwarderConfigArea = 192
)

//go:embed forwarder.dol
var forwarderProgram []byte

var appSlugPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

// ValidSDApp reports whether slug can safely become a directory name under
// sd:/apps and a channel's payload label.
func ValidSDApp(slug string) bool {
	return appSlugPattern.MatchString(slug)
}

// ForwarderDOL returns a copy of the compiled forwarder with its configuration
// block filled in. The block is "<path>\0<marker>\0" in a fixed 192-byte area.
func ForwarderDOL(path, marker string) ([]byte, error) {
	if !strings.HasPrefix(path, "sd:/apps/") || !strings.HasSuffix(path, "/boot.dol") || strings.ContainsAny(path, "\x00\r\n") {
		return nil, errors.New("forwarder path must be sd:/apps/<slug>/boot.dol")
	}
	if slug := strings.TrimSuffix(strings.TrimPrefix(path, "sd:/apps/"), "/boot.dol"); !ValidSDApp(slug) {
		return nil, errors.New("forwarder path must be sd:/apps/<slug>/boot.dol")
	}
	if len(path) > 150 {
		return nil, errors.New("forwarder path is too long")
	}
	if marker == "" || len(marker) > 63 || strings.ContainsAny(marker, "\x00\r\n") {
		return nil, errors.New("forwarder version marker is empty or too long")
	}
	dol := append([]byte(nil), forwarderProgram...)
	n := bytes.Count(dol, []byte(forwarderMagic))
	if n != 1 {
		return nil, fmt.Errorf("forwarder has %d configuration blocks, expected one", n)
	}
	at := bytes.Index(dol, []byte(forwarderMagic)) + len(forwarderMagic)
	blob := []byte(path + "\x00" + marker + "\x00")
	if len(blob) > forwarderConfigArea {
		return nil, errors.New("forwarder configuration does not fit")
	}
	copy(dol[at:at+forwarderConfigArea], make([]byte, forwarderConfigArea))
	copy(dol[at:], blob)
	return dol, nil
}
