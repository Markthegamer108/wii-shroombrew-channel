package channelpkg

import (
	"crypto/sha1"
	"encoding/binary"
	"errors"
)

// Offsets of unused u16 fields covered by the signed hash. Brute-forcing them
// until the SHA-1 starts with zero is what patched-IOS signature checks accept.
const (
	signedStart      = 0x140
	tmdHeaderSize    = 0x1e4
	tmdFakesignAt    = 0x1e2
	ticketSize       = 0x2a4
	ticketFakesignAt = 0x262

	// tmdSynthFakesignAt is used for TMDs built by Synthesize. 0x1e2 is the
	// conventional field, but it is alignment padding inside the TMD struct and
	// gets zeroed when the struct is serialised again for the WAD, which throws
	// the signature away. Reserved2 is unused, is a real field, and survives.
	tmdSynthFakesignAt = 0x1c6
)

func fakesign(b []byte, field int) error {
	if len(b) <= signedStart || field+2 > len(b) || binary.BigEndian.Uint32(b) != 0x00010001 {
		return errors.New("not an RSA-2048 signed structure")
	}
	for i := 4; i < 4+256; i++ {
		b[i] = 0
	}
	for v := 0; v <= 0xffff; v++ {
		binary.BigEndian.PutUint16(b[field:], uint16(v))
		if sum := sha1.Sum(b[signedStart:]); sum[0] == 0 {
			return nil
		}
	}
	return errors.New("fakesign brute force exhausted")
}

// TMDSize returns the signed length of a TMD, excluding any appended certificates.
func TMDSize(tmd []byte) (int, error) {
	if len(tmd) < tmdHeaderSize {
		return 0, errors.New("TMD too short")
	}
	n := tmdHeaderSize + 36*int(binary.BigEndian.Uint16(tmd[0x1de:]))
	if n > len(tmd) {
		return 0, errors.New("TMD content table truncated")
	}
	return n, nil
}

// FakesignTMD fakesigns the TMD portion in place; trailing certificates are left untouched.
func FakesignTMD(tmd []byte) error {
	n, err := TMDSize(tmd)
	if err != nil {
		return err
	}
	return fakesign(tmd[:n], tmdFakesignAt)
}

// FakesignTMDAt fakesigns using a chosen field inside the signed area. Synthesize
// uses this to pick a field its WAD writer will not overwrite.
func FakesignTMDAt(tmd []byte, field int) error {
	n, err := TMDSize(tmd)
	if err != nil {
		return err
	}
	if field < signedStart || field+2 > n {
		return errors.New("fakesign field is outside the signed TMD")
	}
	return fakesign(tmd[:n], field)
}

func FakesignTicket(ticket []byte) error {
	if len(ticket) < ticketSize {
		return errors.New("ticket too short")
	}
	return fakesign(ticket[:ticketSize], ticketFakesignAt)
}

// FakesignFieldTMD is an unused TMD field that survives serialization. Callers
// that rebuild a WAD from a template must fakesign this field, not 0x1e2,
// because the trailing alignment padding is zeroed every time the TMD struct is
// written back out.
const FakesignFieldTMD = tmdSynthFakesignAt

// IsFakesigned reports whether a signed structure has a zero signature and a zero-prefixed hash.
func IsFakesigned(b []byte) bool {
	if len(b) <= signedStart {
		return false
	}
	for _, c := range b[4 : 4+256] {
		if c != 0 {
			return false
		}
	}
	sum := sha1.Sum(b[signedStart:])
	return sum[0] == 0
}
