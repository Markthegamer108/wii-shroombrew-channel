// Package channelpkg prepares existing homebrew channel WADs for native EC downloads.
package channelpkg

import (
	"crypto/sha1"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/wii-tools/wadlib"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type Manifest struct {
	TitleID  string   `json:"title_id"`
	Version  uint16   `json:"version"`
	Contents []string `json:"contents"`
	SDApp    string   `json:"sd_app,omitempty"`
	// Kind is "channel" for packages synthesized by the server, "installer"
	// for a reviewed app-specific SD installer, and empty for imported WADs.
	Kind string `json:"kind,omitempty"`
}

var TitlePattern = regexp.MustCompile(`(?i)^00010001[0-9a-f]{8}$`)
var FilePattern = regexp.MustCompile(`(?i)^(tmd(\.[0-9]{1,5})?|[0-9a-f]{8})$`)

// Import validates every decrypted content before atomically publishing a new package.
// Tickets are private and must only be returned through authenticated ECS requests.
func Import(input, root string) (m Manifest, err error) {
	return importPackage(input, root, "", "", "", false)
}

// ImportSDInstaller labels an operator-reviewed, app-specific native installer.
// The executable must be built for this exact slug; ZIP files are not channels.
func ImportSDInstaller(input, root, slug string) (Manifest, error) {
	if !ValidSDApp(slug) {
		return Manifest{}, errors.New("invalid SD installer app slug")
	}
	return importPackage(input, root, slug, "installer", "", false)
}

// Publish atomically installs a freshly synthesized channel, replacing any
// older package with the same title. It is only called for packages the server
// built itself; the same validation as Import is applied first.
func Publish(wad []byte, root, slug, titleID string) (Manifest, error) {
	if !ValidSDApp(slug) {
		return Manifest{}, errors.New("invalid app slug")
	}
	titleID = strings.ToLower(titleID)
	if !TitlePattern.MatchString(titleID) {
		return Manifest{}, errors.New("invalid title ID")
	}
	if err := os.MkdirAll(root, 0755); err != nil {
		return Manifest{}, err
	}
	tmp, err := os.CreateTemp(root, ".publish-*.wad")
	if err != nil {
		return Manifest{}, err
	}
	name := tmp.Name()
	defer os.Remove(name)
	if _, err := tmp.Write(wad); err != nil {
		tmp.Close()
		return Manifest{}, err
	}
	if err := tmp.Close(); err != nil {
		return Manifest{}, err
	}
	return importPackage(name, root, slug, "channel", titleID, true)
}

func importPackage(input, root, slug, kind, expectedTitle string, replace bool) (m Manifest, err error) {
	m.SDApp = slug
	m.Kind = kind
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("malformed WAD: %v", recovered)
		}
	}()
	st, err := os.Stat(input)
	if err != nil {
		return m, err
	}
	if st.Size() < 32 || st.Size() > 512<<20 {
		return m, errors.New("WAD must be 32 bytes to 512 MiB")
	}
	w, err := wadlib.LoadWADFromFile(input)
	if err != nil {
		return m, err
	}
	m.TitleID = fmt.Sprintf("%016x", w.TMD.TitleID)
	m.TitleID = strings.ToLower(m.TitleID)
	m.Version = w.TMD.TitleVersion
	if !TitlePattern.MatchString(m.TitleID) {
		return m, errors.New("only application-channel title IDs are accepted; IOS, system channels and the shop itself are excluded")
	}
	if expectedTitle != "" && m.TitleID != strings.ToLower(expectedTitle) {
		return m, errors.New("channel title ID does not match the catalog")
	}
	if w.Ticket.TitleID != w.TMD.TitleID || len(w.TMD.Contents) == 0 || len(w.TMD.Contents) != len(w.Data) {
		return m, errors.New("inconsistent ticket/TMD/content table")
	}
	ids := map[uint32]bool{}
	indices := map[uint16]bool{}
	bootFound := false
	for i, c := range w.TMD.Contents {
		if ids[c.ID] || indices[c.Index] {
			return m, errors.New("duplicate content ID or index")
		}
		ids[c.ID] = true
		indices[c.Index] = true
		if c.Index == w.TMD.BootIndex {
			bootFound = true
		}
		b, e := w.GetContent(i)
		if e != nil {
			return m, e
		}
		if c.Size > uint64(len(b)) || sha1.Sum(b[:c.Size]) != c.Hash {
			return m, fmt.Errorf("content %08x does not match its TMD", c.ID)
		}
		m.Contents = append(m.Contents, fmt.Sprintf("%08x", c.ID))
	}
	if !bootFound {
		return m, errors.New("missing boot content")
	}
	if err = os.MkdirAll(root, 0755); err != nil {
		return m, err
	}
	dest := filepath.Join(root, m.TitleID)
	if _, e := os.Stat(dest); !os.IsNotExist(e) && !replace {
		return m, errors.New("title already exists; import into a new staging store for review")
	}
	stage, err := os.MkdirTemp(root, ".import-")
	if err != nil {
		return m, err
	}
	defer os.RemoveAll(stage)
	write := func(name string, b []byte, mode os.FileMode) error {
		return os.WriteFile(filepath.Join(stage, name), b, mode)
	}
	tmd, err := w.GetTMD()
	if err != nil {
		return m, err
	}
	if err = FakesignTMD(tmd); err != nil {
		return m, err
	}
	tmd = append(tmd, w.CertificateChain...)
	if err = write("tmd", tmd, 0644); err != nil {
		return m, err
	}
	if err = write(fmt.Sprintf("tmd.%d", m.Version), tmd, 0644); err != nil {
		return m, err
	}
	for i, c := range w.Data {
		if err = write(m.Contents[i], c.RawData, 0644); err != nil {
			return m, err
		}
	}
	// A ticket dumped from another console carries that console's ID and
	// ECDH data. Common tickets (console ID 0) install everywhere; personal
	// leftovers would otherwise fail with a device-ID mismatch on every other
	// console. FakesignTicket below signs the corrected fields.
	w.Ticket.ConsoleID = 0
	w.Ticket.ECDHData = [60]byte{}
	ticket, err := w.GetTicket()
	if err != nil {
		return m, err
	}
	if err = FakesignTicket(ticket); err != nil {
		return m, err
	}
	if err = write("ticket.bin", ticket, 0600); err != nil {
		return m, err
	}
	if err = write("certs.bin", w.CertificateChain, 0600); err != nil {
		return m, err
	}
	manifest, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return m, err
	}
	if err = write("manifest.json", manifest, 0644); err != nil {
		return m, err
	}
	if err = os.Chmod(stage, 0755); err != nil {
		return m, err
	}
	if !replace {
		return m, os.Rename(stage, dest)
	}
	// Rename the old package aside, move the new one in, then delete the old
	// files. Requests that already opened a file keep their handle; new
	// requests see one complete package or the other.
	old, err := os.MkdirTemp(root, ".old-")
	if err != nil {
		return m, err
	}
	if err = os.Remove(old); err != nil {
		return m, err
	}
	hadOld := true
	if err = os.Rename(dest, old); err != nil {
		if !os.IsNotExist(err) {
			return m, err
		}
		hadOld = false
	}
	if err = os.Rename(stage, dest); err != nil {
		if hadOld {
			_ = os.Rename(old, dest)
		}
		return m, err
	}
	if hadOld {
		_ = os.RemoveAll(old)
	}
	return m, nil
}

func Load(root, title string) (Manifest, error) {
	var m Manifest
	title = strings.ToLower(title)
	if !TitlePattern.MatchString(title) {
		return m, errors.New("invalid title ID")
	}
	b, e := os.ReadFile(filepath.Join(root, title, "manifest.json"))
	if e != nil {
		return m, e
	}
	e = json.Unmarshal(b, &m)
	if e == nil && strings.ToLower(m.TitleID) != title {
		e = errors.New("manifest title mismatch")
	}
	return m, e
}
func (m Manifest) Allows(name string) bool {
	name = strings.ToLower(name)
	if !FilePattern.MatchString(name) {
		return false
	}
	if name == "tmd" || name == fmt.Sprintf("tmd.%d", m.Version) {
		return true
	}
	if strings.HasPrefix(name, "tmd") {
		return false
	}
	for _, n := range m.Contents {
		if strings.EqualFold(n, name) {
			return true
		}
	}
	return false
}
