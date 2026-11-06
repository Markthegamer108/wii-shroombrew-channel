package channelpkg

import (
	"bytes"
	"github.com/wii-tools/wadlib"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func fixture(t *testing.T, title uint64) []byte {
	t.Helper()
	w := new(wadlib.WAD)
	if e := w.LoadTMD(wadlib.TMDTemplate); e != nil {
		t.Fatal(e)
	}
	if e := w.LoadTicket(wadlib.TicketTemplate); e != nil {
		t.Fatal(e)
	}
	w.TMD.TitleID = title
	w.Ticket.TitleID = title
	w.TMD.TitleVersion = 1
	w.Ticket.TitleVersion = 1
	w.TMD.BootIndex = 0
	w.TMD.NumberOfContents = 1
	w.TMD.Contents = []wadlib.ContentRecord{{ID: 10, Index: 0, Type: 1}}
	w.Ticket.UpdateTitleKey([16]byte{1, 2, 3})
	w.CertificateChain = wadlib.CertChainTemplate
	w.Data = []wadlib.WADFile{{Record: &w.TMD.Contents[0]}}
	if e := w.UpdateContent(0, []byte("synthetic test data only, not executable")); e != nil {
		t.Fatal(e)
	}
	b, e := w.GetWAD(wadlib.WADType(0x4973))
	if e != nil {
		t.Fatal(e)
	}
	return b
}
func TestImportAndPrivateTicket(t *testing.T) {
	temp := t.TempDir()
	input := filepath.Join(temp, "test.wad")
	raw := fixture(t, 0x0001000154455354)
	os.WriteFile(input, raw, 0600)
	root := filepath.Join(temp, "store")
	m, e := Import(input, root)
	if e != nil {
		t.Fatal(e)
	}
	if !m.Allows("0000000a") || m.Allows("ticket.bin") || m.Allows("../tmd") {
		t.Fatal("file allowlist failed")
	}
	original, e := wadlib.LoadWAD(raw)
	if e != nil {
		t.Fatal(e)
	}
	encrypted, e := os.ReadFile(filepath.Join(root, m.TitleID, "0000000a"))
	if e != nil || !bytes.Equal(encrypted, original.Data[0].RawData) {
		t.Fatal("encrypted data changed")
	}
	if _, e = Import(input, root); e == nil {
		t.Fatal("overwrote existing package")
	}
}

func TestUppercaseTitleAndFileIDsAccepted(t *testing.T) {
	temp := t.TempDir()
	input := filepath.Join(temp, "uppercase.wad")
	raw := fixture(t, 0x0001000154455354)
	os.WriteFile(input, raw, 0600)
	root := filepath.Join(temp, "store")
	m, e := Import(input, root)
	if e != nil {
		t.Fatal(e)
	}
	if !TitlePattern.MatchString(strings.ToUpper(m.TitleID)) {
		t.Fatal("uppercase title IDs rejected")
	}
	if !m.Allows(strings.ToUpper("0000000a")) {
		t.Fatal("uppercase file IDs rejected")
	}
	if _, e := Load(root, strings.ToUpper(m.TitleID)); e != nil {
		t.Fatalf("upper-case title ID failed to load: %v", e)
	}
}
func TestMalformedAndSystemWADRejected(t *testing.T) {
	temp := t.TempDir()
	for _, b := range [][]byte{[]byte("bad"), make([]byte, 64), fixture(t, 0x0001000248414241)} {
		input := filepath.Join(temp, "bad.wad")
		os.WriteFile(input, b, 0600)
		if _, e := Import(input, filepath.Join(temp, "store")); e == nil {
			t.Fatal("accepted invalid/system WAD")
		}
	}
}

func TestCorruptContentNeverPublished(t *testing.T) {
	temp := t.TempDir()
	raw := fixture(t, 0x0001000154455354)
	w, e := wadlib.LoadWAD(raw)
	if e != nil {
		t.Fatal(e)
	}
	offset := bytes.Index(raw, w.Data[0].RawData)
	if offset < 0 {
		t.Fatal("fixture missing content")
	}
	raw[offset] ^= 1
	input := filepath.Join(temp, "corrupt.wad")
	os.WriteFile(input, raw, 0600)
	root := filepath.Join(temp, "store")
	if _, e = Import(input, root); e == nil {
		t.Fatal("corrupt content accepted")
	}
	if _, e = os.Stat(filepath.Join(root, "0001000154455354")); !os.IsNotExist(e) {
		t.Fatal("failed import published a title")
	}
}
