package channelpkg

import (
	"bytes"
	"crypto/sha1"
	"os"
	"path/filepath"
	"testing"

	"github.com/wii-tools/wadlib"
)

func TestSynthesizeRoundTripAndImport(t *testing.T) {
	program := bytes.Repeat([]byte{0xAB}, 5000)
	payload := bytes.Repeat([]byte("zip-ish payload "), 3000)
	banner := bytes.Repeat([]byte{0x49}, 100)
	wad, err := Synthesize(SynthOptions{TitleID: 0x000100014d414c31, Version: 3, Program: program, Payload: payload, Banner: banner})
	if err != nil {
		t.Fatal(err)
	}
	w, err := wadlib.LoadWAD(wad)
	if err != nil {
		t.Fatal(err)
	}
	if w.TMD.TitleID != 0x000100014d414c31 || w.Ticket.TitleID != w.TMD.TitleID || w.TMD.TitleVersion != 3 {
		t.Fatalf("identity wrong: %x %x v%d", w.TMD.TitleID, w.Ticket.TitleID, w.TMD.TitleVersion)
	}
	if w.TMD.SystemVersionLow != 58 || w.TMD.BootIndex != ContentProgram || len(w.TMD.Contents) != 3 {
		t.Fatalf("tmd wrong: ios=%d boot=%d n=%d", w.TMD.SystemVersionLow, w.TMD.BootIndex, len(w.TMD.Contents))
	}
	for i, want := range [][]byte{banner, program, payload} {
		got, err := w.GetContent(i) // decrypts and checks the TMD hash
		if err != nil || !bytes.Equal(got, want) {
			t.Fatalf("content %d did not round trip: %v", i, err)
		}
	}
	tmd, _ := w.GetTMD()
	tik, _ := w.GetTicket()
	if !IsFakesigned(tmd[:mustTMDSize(t, tmd)]) || !IsFakesigned(tik[:ticketSize]) {
		t.Fatal("TMD and ticket must be fakesigned")
	}

	// ...and again on the bytes actually written, because wadlib rebuilds both
	// structures from its parsed form and could drop signature bytes.
	tikAt := bytes.Index(wad, []byte("Root-CA00000001-XS00000003")) - 0x140
	tmdAt := bytes.Index(wad, []byte("Root-CA00000001-CP00000004")) - 0x140
	t.Logf("header % X; ticket at %#x, tmd at %#x", wad[:32], tikAt, tmdAt)
	if tikAt < 0 || tmdAt < 0 {
		t.Fatal("could not locate the ticket/TMD in the WAD")
	}
	rawTicket, rawTMD := wad[tikAt:tikAt+ticketSize], wad[tmdAt:tmdAt+mustTMDSize(t, tmd)]
	if !IsFakesigned(rawTMD) || !IsFakesigned(rawTicket) {
		t.Fatalf("raw tmd=%v ticket=%v (sha tmd=%02x ticket=%02x)",
			IsFakesigned(rawTMD), IsFakesigned(rawTicket),
			sha1.Sum(rawTMD[0x140:]), sha1.Sum(rawTicket[0x140:]))
	}
	if !bytes.Equal(rawTMD, tmd) {
		t.Fatal("TMD written differs from the parsed one")
	}

	// the package must pass the same checks as an imported one
	dir := t.TempDir()
	in := filepath.Join(dir, "x.wad")
	if err = os.WriteFile(in, wad, 0600); err != nil {
		t.Fatal(err)
	}
	m, err := Import(in, filepath.Join(dir, "store"))
	if err != nil {
		t.Fatalf("import of a synthesized package: %v", err)
	}
	if m.TitleID != "000100014d414c31" || m.Version != 3 || len(m.Contents) != 3 {
		t.Fatalf("manifest %+v", m)
	}
}

func mustTMDSize(t *testing.T, tmd []byte) int {
	n, err := TMDSize(tmd)
	if err != nil {
		t.Fatal(err)
	}
	return n
}

// splitWAD returns the TMD and ticket exactly as stored in the file.
func splitWAD(t *testing.T, wad []byte) (tmd, ticket []byte) {
	t.Helper()
	be := func(b []byte) uint32 { return uint32(b[0])<<24 | uint32(b[1])<<16 | uint32(b[2])<<8 | uint32(b[3]) }
	if len(wad) < 0x20 || be(wad) != 0x20 {
		t.Fatal("bad WAD header")
	}
	cert, crl, tik, tm, data, meta := be(wad[8:]), be(wad[12:]), be(wad[16:]), be(wad[20:]), be(wad[24:]), be(wad[28:])
	align := func(n uint32) uint32 { return (n + 63) &^ 63 }
	off := uint32(0x20)
	off += align(cert) + align(crl)
	ticket = wad[off : off+tik]
	off += align(tik)
	tmd = wad[off : off+tm]
	off += align(tm) + align(data) + align(meta)
	if off > uint32(len(wad)) {
		t.Fatalf("WAD sections overrun the file: %d > %d", off, len(wad))
	}
	return tmd, ticket
}

func TestSynthesizeRejectsBadInput(t *testing.T) {
	ok := SynthOptions{TitleID: 0x000100014d414c31, Program: []byte{1}, Payload: []byte{1}}
	for name, mut := range map[string]func(*SynthOptions){
		"system title": func(o *SynthOptions) { o.TitleID = 0x0000000100000002 },
		"ios title":    func(o *SynthOptions) { o.TitleID = 0x0000000100000024 },
		"no program":   func(o *SynthOptions) { o.Program = nil },
		"no payload":   func(o *SynthOptions) { o.Payload = nil },
		"huge payload": func(o *SynthOptions) { o.Payload = make([]byte, MaxPayload+1) },
		"bad ios":      func(o *SynthOptions) { o.IOS = 999 },
	} {
		o := ok
		mut(&o)
		if _, err := Synthesize(o); err == nil {
			t.Errorf("%s was accepted", name)
		}
	}
	if _, err := Synthesize(ok); err != nil {
		t.Fatal(err)
	}
}
