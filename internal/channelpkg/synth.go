package channelpkg

// Building a Wii Menu channel for a homebrew app.
//
// The channel is three contents:
//
//	0  the banner shown in the Wii Menu (IMET header + banner/icon/sound)
//	1  the forwarder program, patched with the app's path and version
//	2  the app itself, as a ZIP, which the forwarder unpacks to the SD card
//
// The result is an ordinary WAD, fakesigned the same way imported packages are,
// ready for Import and for native EC download from the shop.

import (
	"crypto/rand"
	"errors"
	"fmt"

	"github.com/wii-tools/wadlib"
)

const (
	ContentBanner  = 0
	ContentProgram = 1
	ContentPayload = 2

	// The forwarder streams the package onto the SD card instead of holding it
	// in memory, so this only bounds what the server will build and what a
	// console has to download. The largest app in the Open Shop Channel
	// catalog is 79 MB.
	MaxPayload = 128 << 20
)

type SynthOptions struct {
	TitleID uint64 // 00010001xxxxxxxx
	Version uint16
	IOS     uint32 // IOS the channel runs on; 58 when zero
	Program []byte // the patched forwarder DOL
	Payload []byte // the app's ZIP
	Banner  []byte // IMET content; a blank placeholder is used when empty (tests only)
}

// Synthesize returns the bytes of an installable WAD.
func Synthesize(o SynthOptions) ([]byte, error) {
	if o.TitleID>>32 != 0x00010001 {
		return nil, errors.New("title ID must be an application channel (00010001xxxxxxxx)")
	}
	if len(o.Program) == 0 || len(o.Payload) == 0 {
		return nil, errors.New("program and payload are required")
	}
	if len(o.Payload) > MaxPayload {
		return nil, fmt.Errorf("payload is %d bytes; the limit is %d", len(o.Payload), MaxPayload)
	}
	if o.IOS == 0 {
		o.IOS = 58
	}
	if o.IOS < 3 || o.IOS > 255 {
		return nil, errors.New("IOS must be 3-255")
	}
	banner := o.Banner
	if len(banner) == 0 {
		banner = make([]byte, 64)
	}

	w := &wadlib.WAD{}
	if err := w.LoadTMD(wadlib.TMDTemplate); err != nil {
		return nil, err
	}
	if err := w.LoadTicket(wadlib.TicketTemplate); err != nil {
		return nil, err
	}
	w.CertificateChain = append([]byte(nil), wadlib.CertChainTemplate...)

	var key [16]byte
	if _, err := rand.Read(key[:]); err != nil {
		return nil, err
	}
	var ticketID [8]byte
	if _, err := rand.Read(ticketID[:]); err != nil {
		return nil, err
	}

	w.Ticket.TitleID = o.TitleID
	w.Ticket.TitleVersion = o.Version
	w.Ticket.ConsoleID = 0
	w.Ticket.TicketID = uint64(ticketID[0])<<56 | uint64(ticketID[1])<<48 | uint64(ticketID[2])<<40 | uint64(ticketID[3])<<32 | uint64(ticketID[4])<<24 | uint64(ticketID[5])<<16 | uint64(ticketID[6])<<8 | uint64(ticketID[7])
	w.Ticket.UpdateTitleKey(key) // after TitleID: the title ID is the IV

	contents := [][]byte{banner, o.Program, o.Payload}
	w.TMD.TitleID = o.TitleID
	w.TMD.TitleVersion = o.Version
	w.TMD.TitleType = 1
	w.TMD.SystemVersionHigh = 1
	w.TMD.SystemVersionLow = o.IOS
	w.TMD.Region = 0xffff // region free
	w.TMD.NumberOfContents = uint16(len(contents))
	w.TMD.BootIndex = ContentProgram
	w.TMD.Contents = make([]wadlib.ContentRecord, len(contents))
	w.Data = make([]wadlib.WADFile, len(contents))
	for i, c := range contents {
		w.TMD.Contents[i] = wadlib.ContentRecord{ID: uint32(i), Index: uint16(i), Type: wadlib.TitleTypeNormal}
		w.Data[i] = wadlib.WADFile{Record: &w.TMD.Contents[i]}
		w.Data[i].UpdateData(c, key)
	}

	// Fakesign last: the signed hash covers the content records.
	tmd, err := w.GetTMD()
	if err != nil {
		return nil, err
	}
	if err = FakesignTMDAt(tmd, tmdSynthFakesignAt); err != nil {
		return nil, err
	}
	if err = w.LoadTMD(tmd); err != nil {
		return nil, err
	}
	for i := range w.Data { // LoadTMD replaced the record slice
		w.Data[i].Record = &w.TMD.Contents[i]
	}
	ticket, err := w.GetTicket()
	if err != nil {
		return nil, err
	}
	if err = FakesignTicket(ticket); err != nil {
		return nil, err
	}
	if err = w.LoadTicket(ticket); err != nil {
		return nil, err
	}
	return w.GetWAD(wadlib.WADTypeCommon)
}
