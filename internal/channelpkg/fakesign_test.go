package channelpkg

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/wii-tools/wadlib"
)

func TestImportFakesignsTicketAndTMD(t *testing.T) {
	temp := t.TempDir()
	input := filepath.Join(temp, "test.wad")
	raw := fixture(t, 0x0001000154455354)
	os.WriteFile(input, raw, 0600)
	root := filepath.Join(temp, "store")
	m, err := Import(input, root)
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(root, m.TitleID)
	ticket, _ := os.ReadFile(filepath.Join(dir, "ticket.bin"))
	tmd, _ := os.ReadFile(filepath.Join(dir, "tmd"))
	n, err := TMDSize(tmd)
	if err != nil {
		t.Fatal(err)
	}
	if !IsFakesigned(ticket[:ticketSize]) || !IsFakesigned(tmd[:n]) {
		t.Fatal("imported ticket or TMD is not fakesigned")
	}
	// The content table and title key must survive fakesigning unchanged.
	original, _ := wadlib.LoadWAD(raw)
	signed := new(wadlib.WAD)
	if err := signed.LoadTMD(tmd[:n]); err != nil {
		t.Fatal(err)
	}
	if signed.TMD.Contents[0].Hash != original.TMD.Contents[0].Hash || signed.TMD.TitleID != original.TMD.TitleID {
		t.Fatal("fakesigning changed the content table")
	}
	origTicket, _ := original.GetTicket()
	if !bytes.Equal(ticket[0x1bf:0x1cf], origTicket[0x1bf:0x1cf]) || !bytes.Equal(ticket[0x222:0x262], origTicket[0x222:0x262]) {
		t.Fatal("fakesigning changed the title key or content access bitmap")
	}
	if !bytes.Equal(tmd[n:], original.CertificateChain) {
		t.Fatal("certificate chain changed")
	}
}

func TestFakesignRejectsGarbage(t *testing.T) {
	if FakesignTicket(make([]byte, 10)) == nil || FakesignTMD(make([]byte, 10)) == nil {
		t.Fatal("expected errors for short input")
	}
}
