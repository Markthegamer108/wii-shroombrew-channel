package channelpkg

import (
	"bytes"
	"testing"
)

func TestForwarderDOL(t *testing.T) {
	dol, err := ForwarderDOL("sd:/apps/Demo_App/boot.dol", "1.0.0+deadbeef")
	if err != nil {
		t.Fatal(err)
	}
	if len(dol) != len(forwarderProgram) {
		t.Fatalf("patched DOL is %d bytes, was %d", len(dol), len(forwarderProgram))
	}
	if n := bytes.Count(dol, []byte(forwarderMagic)); n != 1 {
		t.Fatalf("found %d config blocks", n)
	}
	at := bytes.Index(dol, []byte(forwarderMagic)) + len(forwarderMagic)
	area := dol[at : at+forwarderConfigArea]
	if !bytes.HasPrefix(area, []byte("sd:/apps/Demo_App/boot.dol\x001.0.0+deadbeef\x00")) {
		t.Fatalf("config block is %q", area[:64])
	}
	if !bytes.Equal(area[len("sd:/apps/Demo_App/boot.dol\x001.0.0+deadbeef\x00"):], make([]byte, forwarderConfigArea-len("sd:/apps/Demo_App/boot.dol\x001.0.0+deadbeef\x00"))) {
		t.Fatal("config padding was not cleared")
	}

	for _, path := range []string{"", "sd:/apps/x", "/apps/x/boot.dol", "sd:/apps/../boot.dol", "sd:/apps/x/boot.elf"} {
		if _, err := ForwarderDOL(path, "1"); err == nil {
			t.Fatalf("accepted path %q", path)
		}
	}
	if _, err := ForwarderDOL("sd:/apps/x/boot.dol", ""); err == nil {
		t.Fatal("accepted an empty marker")
	}
}
