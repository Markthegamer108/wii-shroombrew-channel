package flashpkg

import (
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/Markthegamer108/wii-shroombrew-channel/internal/channelpkg"
	"github.com/wii-tools/arclib"
	"github.com/wii-tools/wadlib"
)

// Asset is one extra file copied next to the injected SWF. Path is relative to
// the SWF's directory and must use forward slashes.
type Asset struct {
	Path string
	Data []byte
}

// KeymapEntry maps one Wii input to one Flash key. Button is a Wii keymap name
// such as KEY_BUTTON_A; Key is e.g. KEY_ENTER or an ASCII code such as 32.
type KeymapEntry struct {
	Button string
	Key    string
}

// Options describes one Flash channel.
type Options struct {
	Template []byte // operator-supplied official Flash runtime WAD
	SWF      []byte
	SWFPath  string // optional override inside the template's U8
	Assets   []Asset

	TitleID        uint64 // 00010001xxxxxxxx
	TitleVersion   uint16 // 0 means template version + 1
	Region         uint16 // 0xffff region-free
	Name           string
	Author         string
	Description    string
	Category       string
	ReleaseDate    time.Time
	IconPNG        []byte
	DisplayVersion string

	// Zoom is "auto" (fit the stage), "default" (Nintendo's -304/+228 field),
	// "fill" (stretch the stage to the screen), or explicit half-extents
	// written as "H,V".
	Zoom string

	Mouse           bool
	Qwerty          bool
	Quality         string
	SharedObject    bool
	StrapReminder   string
	HBMNoSave       bool
	BackgroundColor string // "r g b a"; empty uses the SWF background
	Keymap          []KeymapEntry
}

// knownSWFs are the movies FriishProduce recognises in official bases, in
// preference order.
var knownSWFs = []string{
	"content/menu.swf",
	"content/sample.swf",
	"trusted/startup.swf",
	"trusted/wii_shim.swf",
	"trusted/Gumball2.12.RC-1.swf",
}

const u8Magic = 0x55AA382D

// Inject builds a fakesigned channel WAD from the template and the uploaded
// movie. The template is never modified.
func Inject(o Options) ([]byte, error) {
	if o.TitleID>>32 != 0x00010001 {
		return nil, errors.New("title ID must be an application channel (00010001xxxxxxxx)")
	}
	if strings.TrimSpace(o.Name) == "" {
		return nil, errors.New("the channel needs a name")
	}
	if len(o.Template) == 0 {
		return nil, errors.New("no Flash base WAD was supplied")
	}
	info, _, err := Validate(o.SWF)
	if err != nil {
		return nil, err
	}

	w, err := wadlib.LoadWAD(o.Template)
	if err != nil {
		return nil, fmt.Errorf("the Flash base WAD could not be read: %w", err)
	}
	if len(w.TMD.Contents) == 0 || len(w.TMD.Contents) != len(w.Data) {
		return nil, errors.New("the Flash base WAD has an inconsistent content table")
	}

	arc, arcIndex, swfPath, err := findMovie(w, o.SWFPath)
	if err != nil {
		return nil, err
	}
	if err = replaceMovie(arc, swfPath, o.SWF, o.Assets); err != nil {
		return nil, err
	}
	background := o.BackgroundColor
	if background == "" {
		if info.Background != "" {
			var r, g, b uint8
			_, _ = fmt.Sscanf(info.Background, "#%02x%02x%02x", &r, &g, &b)
			background = fmt.Sprintf("%d %d %d 255", r, g, b)
		} else {
			background = "0 0 0 0"
		}
	}
	zoom := o.Zoom
	if zoom == "" {
		zoom = "auto"
	}
	if err = patchConfig(arc, swfPath, info, background, zoom, o); err != nil {
		return nil, err
	}
	mainContent, err := arc.Save()
	if err != nil {
		return nil, fmt.Errorf("the Flash runtime archive could not be rebuilt: %w", err)
	}

	banner, err := channelpkg.BuildBanner(channelpkg.BannerInfo{
		Title:       o.Name,
		Author:      o.Author,
		Version:     o.DisplayVersion,
		Description: o.Description,
		Category:    o.Category,
		ReleaseDate: o.ReleaseDate,
		IconPNG:     o.IconPNG,
	})
	if err != nil {
		return nil, fmt.Errorf("the channel banner could not be built: %w", err)
	}

	version := o.TitleVersion
	if version == 0 {
		version = w.TMD.TitleVersion + 1
	}
	if version == 0 {
		version = 1
	}

	// Re-encrypt the template's title key under the new title ID before any
	// content is replaced, otherwise the console derives the wrong key.
	oldKey := w.Ticket.GetTitleKey()
	w.Ticket.TitleID = o.TitleID
	w.Ticket.TitleVersion = version
	w.Ticket.ConsoleID = 0
	w.Ticket.ECDHData = [60]byte{}
	w.Ticket.UpdateTitleKey(oldKey)

	if err = w.UpdateContent(arcIndex, mainContent); err != nil {
		return nil, err
	}
	if err = w.UpdateContent(0, banner); err != nil {
		return nil, fmt.Errorf("the banner content could not be replaced: %w", err)
	}

	var fresh [16]byte
	if _, err = rand.Read(fresh[:]); err != nil {
		return nil, err
	}
	if err = w.ChangeTitleKey(fresh); err != nil {
		return nil, fmt.Errorf("the channel could not be re-encrypted: %w", err)
	}

	w.TMD.TitleID = o.TitleID
	w.TMD.TitleVersion = version
	w.TMD.Region = o.Region

	tmd, err := w.GetTMD()
	if err != nil {
		return nil, err
	}
	if err = channelpkg.FakesignTMDAt(tmd, channelpkg.FakesignFieldTMD); err != nil {
		return nil, err
	}
	if err = w.LoadTMD(tmd); err != nil {
		return nil, err
	}
	for i := range w.TMD.Contents {
		c := &w.TMD.Contents[i]
		w.Data[c.Index].Record = c
	}
	ticket, err := w.GetTicket()
	if err != nil {
		return nil, err
	}
	if err = channelpkg.FakesignTicket(ticket); err != nil {
		return nil, err
	}
	if err = w.LoadTicket(ticket); err != nil {
		return nil, err
	}
	if err = checkSigned(w); err != nil {
		return nil, err
	}
	return w.GetWAD(wadlib.WADTypeCommon)
}

func checkSigned(w *wadlib.WAD) error {
	tmd, err := w.GetTMD()
	if err != nil {
		return err
	}
	if !channelpkg.IsFakesigned(tmd) {
		return errors.New("the rebuilt TMD is not fakesigned; installing would fail")
	}
	ticket, err := w.GetTicket()
	if err != nil {
		return err
	}
	if !channelpkg.IsFakesigned(ticket) {
		return errors.New("the rebuilt ticket is not fakesigned; installing would fail")
	}
	return nil
}

// findMovie locates the runtime U8 and the SWF inside it. Back to Nature and
// the Flash Placeholder keep the movie at content/menu.swf in content 2.
func findMovie(w *wadlib.WAD, override string) (*arclib.ARC, int, string, error) {
	var candidates []int
	if len(w.TMD.Contents) > 2 {
		candidates = append(candidates, 2)
	}
	for i := range w.TMD.Contents {
		if i != 2 {
			candidates = append(candidates, i)
		}
	}
	for _, idx := range candidates {
		if idx == 0 {
			continue // the banner is an IMET, never a U8 movie archive
		}
		data, err := w.GetContent(idx)
		if err != nil || len(data) < 4 || binary.BigEndian.Uint32(data[:4]) != u8Magic {
			continue
		}
		arc, err := arclib.Load(data)
		if err != nil {
			continue
		}
		swf := override
		if swf == "" {
			swf = detectSWF(arc)
		}
		if swf == "" {
			continue
		}
		if _, err := arc.OpenFile(swf); err != nil {
			continue
		}
		return arc, idx, swf, nil
	}
	if override != "" {
		return nil, 0, "", fmt.Errorf("the base WAD has no U8 archive containing %s", override)
	}
	return nil, 0, "", errors.New("this WAD is not a Flash runtime base: none of its contents hold a recognised SWF")
}

func detectSWF(arc *arclib.ARC) string {
	for _, p := range knownSWFs {
		if _, err := arc.OpenFile(p); err == nil {
			return p
		}
	}
	var found string
	_ = walkFiles(&arc.RootRecord, "", func(p string, _ *arclib.ARCFile) error {
		if found == "" && strings.EqualFold(path.Ext(p), ".swf") {
			found = p
		}
		return nil
	})
	return found
}

func replaceMovie(arc *arclib.ARC, swfPath string, swf []byte, assets []Asset) error {
	if err := writeFile(arc, swfPath, swf); err != nil {
		return err
	}
	dir := path.Dir(swfPath)
	if dir == "." {
		dir = ""
	}
	for _, a := range assets {
		rel, err := cleanRelative(a.Path)
		if err != nil {
			return fmt.Errorf("asset %q: %w", a.Path, err)
		}
		target := rel
		if dir != "" {
			target = dir + "/" + rel
		}
		if strings.EqualFold(target, swfPath) {
			continue // the main movie wins over an asset with the same name
		}
		if err = writeFile(arc, target, a.Data); err != nil {
			return fmt.Errorf("asset %q: %w", a.Path, err)
		}
	}
	return nil
}

func cleanRelative(p string) (string, error) {
	p = strings.ReplaceAll(p, "\\", "/")
	p = strings.TrimSpace(p)
	if p == "" {
		return "", errors.New("empty path")
	}
	if strings.ContainsRune(p, 0) {
		return "", errors.New("path contains a NUL byte")
	}
	if strings.HasPrefix(p, "/") || (len(p) >= 2 && p[1] == ':') {
		return "", errors.New("absolute paths are not allowed")
	}
	clean := path.Clean(p)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") {
		return "", errors.New("path escapes the channel directory")
	}
	return clean, nil
}

func writeFile(arc *arclib.ARC, name string, data []byte) error {
	dirName := path.Dir(name)
	if dirName == "." {
		dirName = ""
	}
	dir, err := ensureDir(arc, dirName)
	if err != nil {
		return err
	}
	dir.WriteFile(path.Base(name), data)
	return nil
}

func ensureDir(arc *arclib.ARC, name string) (*arclib.ARCDir, error) {
	current := &arc.RootRecord
	if name == "" {
		return current, nil
	}
	for _, part := range strings.Split(name, "/") {
		if part == "" || part == "." {
			continue
		}
		next, err := current.GetDir(part)
		if err == nil {
			current = next
			continue
		}
		current.AddDir(arclib.ARCDir{Filename: part})
		next, err = current.GetDir(part)
		if err != nil {
			return nil, err
		}
		current = next
	}
	return current, nil
}

func walkFiles(dir *arclib.ARCDir, prefix string, fn func(string, *arclib.ARCFile) error) error {
	for i := range dir.Files {
		name := dir.Files[i].Filename
		if prefix != "" {
			name = prefix + "/" + name
		}
		if err := fn(name, &dir.Files[i]); err != nil {
			return err
		}
	}
	for i := range dir.Subdirs {
		name := dir.Subdirs[i].Filename
		if prefix != "" {
			name = prefix + "/" + name
		}
		if err := walkFiles(&dir.Subdirs[i], name, fn); err != nil {
			return err
		}
	}
	return nil
}

func patchConfig(arc *arclib.ARC, swfPath string, info Info, background, zoom string, o Options) error {
	var files []struct {
		path string
		file *arclib.ARCFile
	}
	if err := walkFiles(&arc.RootRecord, "", func(p string, f *arclib.ARCFile) error {
		if strings.HasPrefix(strings.ToLower(p), "config/") {
			files = append(files, struct {
				path string
				file *arclib.ARCFile
			}{p, f})
		}
		return nil
	}); err != nil {
		return err
	}
	if len(files) == 0 {
		return nil // this base keeps its runtime config elsewhere
	}

	var sawCommon, sawKeymap bool
	for _, entry := range files {
		lower := strings.ToLower(entry.path)
		base := path.Base(lower)
		switch {
		case strings.Contains(base, "common.pcf"):
			entry.file.Write(commonConfig(swfPath, o))
			sawCommon = true
		case base == "keymap.ini":
			entry.file.Write(keymapFile(o.Keymap))
			sawKeymap = true
		case strings.HasSuffix(lower, ".pcf"):
			entry.file.Write(patchPCF(entry.file.Data, info, background, zoom, strings.HasPrefix(swfPath, "content/")))
		}
	}
	if !sawCommon {
		if err := writeFile(arc, "config/config.common.pcf", commonConfig(swfPath, o)); err != nil {
			return err
		}
	}
	if !sawKeymap {
		for _, region := range []string{"US", "EU", "JP"} {
			if err := writeFile(arc, "config/"+region+"/keymap.ini", keymapFile(o.Keymap)); err != nil {
				return err
			}
		}
	}
	return nil
}

func commonConfig(swfPath string, o Options) []byte {
	onOff := func(v bool) string {
		if v {
			return "on"
		}
		return "off"
	}
	quality := o.Quality
	if quality == "" {
		quality = "high"
	}
	strap := o.StrapReminder
	if strap == "" {
		strap = "none"
	}
	hbm := "no"
	if o.HBMNoSave {
		hbm = "yes"
	}
	shared := "off"
	if o.SharedObject {
		shared = "on"
	}
	lines := []string{
		"# Shroombrew Flash channel",
		"# Generated by flashinject. Comments and blank lines are ignored.",
		"static_heap_size                8192                # 8192[KB] -> 8[MB]",
		"dynamic_heap_size               16384               # 16384[KB] -> 16[MB]",
		"update_frame_rate               0                   # 0 uses the movie's own rate",
		"mouse                           " + onOff(o.Mouse),
		"qwerty_keyboard                 " + onOff(o.Qwerty),
		"navigation_model                4way                # 2way / 4way / 4waywrap",
		"quality                         " + quality,
		"looping                         on",
		"text_encoding                   utf-16",
		"midi                            off",
		"# dls_file                      dls/GM16.DLS",
		"key_input                       on",
		"cursor_archive                  cursor.arc",
		"cursor_layout                   cursor.brlyt",
		"dialog_cursor_archive           cursor.arc",
		"dialog_cursor_layout            cursor.brlyt",
		"shared_object_capability        " + shared,
		"num_vff_drives                  1",
		"vff_cache_size                  96                  # 96[KB]",
		"vff_sync_on_write               off",
		"persistent_storage_root_drive   X",
		"persistent_storage_vff_file     shrdobjs.vff        # 8.3 format",
		"persistent_storage_total        96                  # 96[KB]",
		"persistent_storage_per_movie    64                  # 64[KB]",
		"strap_reminder                  " + strap,
		"supported_devices               core, freestyle, classic",
		"hbm_no_save                     " + hbm,
		"content_url                     file:///" + swfPath,
	}
	return []byte(strings.Join(lines, "\r\n") + "\r\n")
}

func patchPCF(data []byte, info Info, background, zoom string, dropContentURL bool) []byte {
	text := strings.ReplaceAll(string(data), "\r\n", "\n")
	lines := strings.Split(text, "\n")
	h, v := orthoExtents(zoom, info.Width, info.Height)
	changed := false
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		trim := strings.TrimSpace(line)
		if trim == "" || strings.HasPrefix(trim, "#") {
			out = append(out, line)
			continue
		}
		key := strings.Fields(trim)[0]
		switch {
		case key == "ortho_rect":
			out = append(out, fmt.Sprintf("ortho_rect                      -%d +%d +%d -%d # left top right bottom (608 x 456)", h, v, h, v))
			changed = true
		case strings.HasPrefix(key, "background_"):
			out = append(out, "background_color                "+background+"             # RGBA")
			changed = true
		case key == "content_url" && dropContentURL:
			changed = true // Back to Nature reads this from common.pcf
		default:
			out = append(out, line)
		}
	}
	if !changed {
		return data
	}
	return []byte(strings.Join(out, "\r\n"))
}

func orthoExtents(zoom string, width, height int) (int, int) {
	if width <= 0 || height <= 0 {
		return 304, 228
	}
	switch strings.ToLower(strings.TrimSpace(zoom)) {
	case "", "auto":
		scale := math.Min(608/float64(width), 456/float64(height))
		w, h := float64(width)*scale, float64(height)*scale
		return int(math.Round((w + 5) / 2)), int(math.Round((h + 5) / 2))
	case "default":
		return 304, 228
	case "fill":
		return int(math.Round(float64(width) / 2)), int(math.Round(float64(height) / 2))
	}
	parts := strings.FieldsFunc(zoom, func(r rune) bool { return r == 'x' || r == 'X' || r == ',' || r == '_' })
	if len(parts) == 2 {
		h, err1 := strconv.Atoi(parts[0])
		v, err2 := strconv.Atoi(parts[1])
		if err1 == nil && err2 == nil && h > 0 && v > 0 {
			return h, v
		}
	}
	return 304, 228
}

func keymapFile(entries []KeymapEntry) []byte {
	if len(entries) == 0 {
		entries = []KeymapEntry{
			{"KEY_BUTTON_LEFT", "KEY_LEFT"},
			{"KEY_BUTTON_RIGHT", "KEY_RIGHT"},
			{"KEY_BUTTON_DOWN", "KEY_DOWN"},
			{"KEY_BUTTON_UP", "KEY_UP"},
			{"KEY_BUTTON_A", "32"},
		}
	}
	var b strings.Builder
	for _, e := range entries {
		button := strings.TrimSpace(e.Button)
		key := strings.TrimSpace(e.Key)
		if button == "" || key == "" {
			continue
		}
		b.WriteString(button)
		for i := len(button); i < 17; i++ {
			b.WriteByte(' ')
		}
		b.WriteString(key)
		b.WriteString("\r\n")
	}
	return []byte(b.String())
}
