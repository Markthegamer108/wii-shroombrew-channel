package main

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	installJobTTL      = 15 * time.Minute
	maxPackageBytes    = int64(128 << 20)
	maxExtractedBytes  = int64(256 << 20)
	maxManifestEntries = 4096
	maxActiveJobs      = 4
)

type installFile struct {
	Path   string `json:"path"`
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256"`
}

type installManifest struct {
	Slug  string        `json:"slug"`
	Files []installFile `json:"files"`
}

type installJob struct {
	mu        sync.RWMutex
	ID        string
	Code      string
	Slug      string
	State     string
	Error     string
	Bytes     int64
	Total     int64
	Created   time.Time
	Updated   time.Time
	Workspace string
	Manifest  installManifest
	context   context.Context
	cancel    context.CancelFunc
	preparing bool
	retired   bool
	readers   int
}

type installManager struct {
	mu     sync.RWMutex
	jobs   map[string]*installJob
	codes  map[string]string
	root   string
	client *http.Client
}

func newInstallManager() *installManager {
	m := &installManager{
		jobs:  make(map[string]*installJob),
		codes: make(map[string]string),
		root:  filepath.Join(os.TempDir(), "shroombrew", "install-jobs"),
		client: &http.Client{
			Timeout: 5 * time.Minute,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= 5 {
					return errors.New("too many package redirects")
				}
				_, err := approvedPackageURL(req.URL.String())
				return err
			},
		},
	}
	go func() {
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			m.removeExpired()
		}
	}()
	return m
}

func newInstallID() (string, error) {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func newInstallCode() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(100000000))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%08d", n.Int64()), nil
}

func approvedPackageURL(raw string) (*url.URL, error) {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Host != "hbb1.oscwii.org" || u.User != nil || !strings.HasPrefix(u.Path, "/api/contents/") {
		return nil, errors.New("package URL is not an approved Open Shop Channel source")
	}
	return u, nil
}

func (m *installManager) removeExpired() {
	cutoff := time.Now().Add(-installJobTTL)
	m.mu.Lock()
	for id, job := range m.jobs {
		job.mu.Lock()
		old := job.readers == 0 && job.Updated.Before(cutoff)
		if old {
			delete(m.jobs, id)
			delete(m.codes, job.Code)
			job.retireLocked()
		}
		job.mu.Unlock()
	}
	m.mu.Unlock()
}

// Hold the job lock while retiring. A worker or response may still be using
// its workspace; that last user performs cleanup after it finishes.
func (job *installJob) cleanupLocked() {
	if job.retired && !job.preparing && job.readers == 0 {
		_ = os.RemoveAll(job.Workspace)
	}
}

func (job *installJob) retireLocked() {
	job.retired = true
	if job.cancel != nil {
		job.cancel()
	}
	job.cleanupLocked()
}

func (job *installJob) release() {
	job.mu.Lock()
	job.readers--
	job.Updated = time.Now().UTC()
	job.cleanupLocked()
	job.mu.Unlock()
}

func (m *installManager) start(app catalogApp) (*installJob, error) {
	if _, err := approvedPackageURL(app.URL.Zip); err != nil {
		return nil, err
	}
	m.removeExpired()
	m.mu.RLock()
	busy := len(m.jobs) >= maxActiveJobs
	m.mu.RUnlock()
	if busy {
		return nil, errors.New("too many active downloads")
	}
	id, err := newInstallID()
	if err != nil {
		return nil, err
	}
	workspace := filepath.Join(m.root, id)
	if err := os.MkdirAll(workspace, 0700); err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	jobCtx, cancel := context.WithCancel(context.Background())
	job := &installJob{ID: id, Slug: app.Slug, State: "preparing", Created: now, Updated: now, Workspace: workspace, context: jobCtx, cancel: cancel, preparing: true}
	m.mu.Lock()
	if len(m.jobs) >= maxActiveJobs {
		m.mu.Unlock()
		cancel()
		_ = os.RemoveAll(workspace)
		return nil, errors.New("too many active downloads")
	}
	for {
		code, codeErr := newInstallCode()
		if codeErr != nil {
			m.mu.Unlock()
			cancel()
			_ = os.RemoveAll(workspace)
			return nil, codeErr
		}
		if _, exists := m.codes[code]; !exists {
			job.Code = code
			m.codes[code] = id
			break
		}
	}
	m.jobs[id] = job
	m.mu.Unlock()
	go m.prepare(job, app)
	return job, nil
}

func (m *installManager) prepare(job *installJob, app catalogApp) {
	err := m.downloadAndExtract(job, app)
	job.mu.Lock()
	defer job.mu.Unlock()
	job.preparing = false
	if job.retired {
		job.cleanupLocked()
		return
	}
	if err != nil {
		job.State = "failed"
		job.Error = "package preparation failed"
		job.Updated = time.Now().UTC()
		_ = os.RemoveAll(job.Workspace)
		return
	}
	job.State = "ready"
	job.Updated = time.Now().UTC()
}

func (m *installManager) downloadAndExtract(job *installJob, app catalogApp) error {
	u, err := approvedPackageURL(app.URL.Zip)
	if err != nil {
		return err
	}
	job.mu.Lock()
	job.State = "fetching"
	job.Updated = time.Now().UTC()
	job.mu.Unlock()
	jobContext := job.context
	if jobContext == nil {
		jobContext = context.Background()
	}
	request, err := http.NewRequestWithContext(jobContext, http.MethodGet, u.String(), nil)
	if err != nil {
		return err
	}
	response, err := m.client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("source returned %s", response.Status)
	}
	if response.ContentLength > maxPackageBytes {
		return errors.New("package exceeds size limit")
	}
	job.mu.Lock()
	job.Total = response.ContentLength
	job.Updated = time.Now().UTC()
	job.mu.Unlock()
	archivePath := filepath.Join(job.Workspace, "source.zip")
	archive, err := os.OpenFile(archivePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer archive.Close()
	reader := io.LimitReader(response.Body, maxPackageBytes+1)
	buffer := make([]byte, 32*1024)
	var downloaded int64
	for {
		n, readErr := reader.Read(buffer)
		if n > 0 {
			if downloaded+int64(n) > maxPackageBytes {
				return errors.New("package exceeds size limit")
			}
			if _, err := archive.Write(buffer[:n]); err != nil {
				return err
			}
			downloaded += int64(n)
			job.mu.Lock()
			job.Bytes = downloaded
			job.Updated = time.Now().UTC()
			job.mu.Unlock()
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return readErr
		}
	}
	if err := archive.Close(); err != nil {
		return err
	}
	job.mu.Lock()
	job.State = "extracting"
	job.Updated = time.Now().UTC()
	job.mu.Unlock()
	manifest, err := extractInstallArchive(archivePath, filepath.Join(job.Workspace, "extracted"), app.Slug)
	if err != nil {
		return err
	}
	if err := os.Remove(archivePath); err != nil && !os.IsNotExist(err) {
		return err
	}
	job.mu.Lock()
	job.Manifest = manifest
	job.Updated = time.Now().UTC()
	job.mu.Unlock()
	return nil
}

func safeInstallPath(name string) (string, error) {
	if name == "" || len(name) > 240 || strings.ContainsAny(name, "\\:\x00") || strings.HasPrefix(name, "./") || strings.Contains(name, "//") {
		return "", errors.New("unsafe archive path")
	}
	for _, char := range name {
		if char < 32 || char == 127 || strings.ContainsRune(`<>"|?*`, char) {
			return "", errors.New("unsafe archive path")
		}
	}
	clean := path.Clean(name)
	if clean == "." || clean != name || strings.HasPrefix(clean, "/") || clean == ".." || strings.HasPrefix(clean, "../") {
		return "", errors.New("unsafe archive path")
	}
	for _, segment := range strings.Split(clean, "/") {
		if strings.HasSuffix(segment, ".") || strings.HasSuffix(segment, " ") {
			return "", errors.New("FAT-ambiguous archive path")
		}
		base := strings.ToUpper(strings.SplitN(segment, ".", 2)[0])
		if base == "CON" || base == "PRN" || base == "AUX" || base == "NUL" || (len(base) == 4 && (strings.HasPrefix(base, "COM") || strings.HasPrefix(base, "LPT")) && base[3] >= '1' && base[3] <= '9') {
			return "", errors.New("reserved archive path")
		}
	}
	return clean, nil
}

func extractInstallArchive(archivePath, destination, slug string) (installManifest, error) {
	archive, err := zip.OpenReader(archivePath)
	if err != nil {
		return installManifest{}, err
	}
	defer archive.Close()
	if len(archive.File) == 0 || len(archive.File) > maxManifestEntries {
		return installManifest{}, errors.New("invalid archive entry count")
	}
	if err := os.MkdirAll(destination, 0700); err != nil {
		return installManifest{}, err
	}
	manifest := installManifest{Slug: slug}
	var extracted int64
	seen := make(map[string]bool)
	for _, entry := range archive.File {
		if entry.FileInfo().IsDir() {
			continue
		}
		if !entry.FileInfo().Mode().IsRegular() {
			return installManifest{}, errors.New("archive contains a non-regular file")
		}
		relative, err := safeInstallPath(entry.Name)
		if err != nil {
			return installManifest{}, err
		}
		key := strings.ToUpper(relative)
		if seen[key] {
			return installManifest{}, errors.New("duplicate archive path")
		}
		seen[key] = true
		if entry.UncompressedSize64 > uint64(maxExtractedBytes-extracted) {
			return installManifest{}, errors.New("extracted package exceeds size limit")
		}
		target := filepath.Join(destination, filepath.FromSlash(relative))
		if err := os.MkdirAll(filepath.Dir(target), 0700); err != nil {
			return installManifest{}, err
		}
		input, err := entry.Open()
		if err != nil {
			return installManifest{}, err
		}
		output, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
		if err != nil {
			input.Close()
			return installManifest{}, err
		}
		hash := sha256.New()
		written, copyErr := io.Copy(io.MultiWriter(output, hash), io.LimitReader(input, int64(entry.UncompressedSize64)+1))
		closeErr := output.Close()
		input.Close()
		if copyErr != nil || closeErr != nil || written != int64(entry.UncompressedSize64) {
			_ = os.Remove(target)
			if copyErr != nil {
				return installManifest{}, copyErr
			}
			return installManifest{}, errors.New("archive entry could not be extracted")
		}
		extracted += written
		manifest.Files = append(manifest.Files, installFile{Path: relative, Size: written, SHA256: hex.EncodeToString(hash.Sum(nil))})
	}
	if len(manifest.Files) == 0 {
		return installManifest{}, errors.New("archive contains no files")
	}
	bootable := false
	for _, file := range manifest.Files {
		if file.Path == "apps/"+slug+"/boot.dol" || file.Path == "apps/"+slug+"/boot.elf" {
			bootable = true
		}
	}
	if !bootable {
		return installManifest{}, errors.New("package has no matching homebrew executable")
	}
	return manifest, nil
}

func (m *installManager) find(id string) (*installJob, bool) {
	m.mu.RLock()
	job, ok := m.jobs[id]
	m.mu.RUnlock()
	return job, ok
}

func writeInstallJSON(w http.ResponseWriter, value interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(value)
}

func (m *installManager) serveHTTP(w http.ResponseWriter, r *http.Request, apps map[string]catalogApp) {
	if strings.HasPrefix(r.URL.Path, "/api/install/native/") {
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", http.MethodPost)
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		slug := strings.TrimPrefix(r.URL.Path, "/api/install/native/")
		app, ok := apps[slug]
		if !ok || !safeSlug.MatchString(slug) {
			http.NotFound(w, r)
			return
		}
		job, err := m.start(app)
		if err != nil {
			http.Error(w, "Package unavailable", http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=ascii")
		w.Header().Set("Cache-Control", "no-store")
		_, _ = fmt.Fprintf(w, "SHROOMDIRECT1\nJOB %s\n", job.ID)
		return
	}
	if strings.HasPrefix(r.URL.Path, "/api/install/wii/") {
		m.serveWiiPlan(w, r)
		return
	}
	if strings.HasPrefix(r.URL.Path, "/api/install/claim/") {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		code := strings.TrimPrefix(r.URL.Path, "/api/install/claim/")
		if len(code) != 8 {
			http.NotFound(w, r)
			return
		}
		for _, digit := range code {
			if digit < '0' || digit > '9' {
				http.NotFound(w, r)
				return
			}
		}
		m.mu.RLock()
		id, ok := m.codes[code]
		job := m.jobs[id]
		m.mu.RUnlock()
		if !ok || job == nil {
			http.NotFound(w, r)
			return
		}
		job.mu.Lock()
		result := map[string]string{"job_id": job.ID, "slug": job.Slug, "state": job.State}
		job.Updated = time.Now().UTC()
		job.mu.Unlock()
		writeInstallJSON(w, result)
		return
	}
	if r.URL.Path == "/api/install/start" {
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", http.MethodPost)
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 8192)
		if err := r.ParseForm(); err != nil {
			http.Error(w, "Invalid request", http.StatusBadRequest)
			return
		}
		slug := r.Form.Get("slug")
		app, ok := apps[slug]
		if !ok || !safeSlug.MatchString(slug) {
			http.NotFound(w, r)
			return
		}
		job, err := m.start(app)
		if err != nil {
			http.Error(w, "Package unavailable", http.StatusServiceUnavailable)
			return
		}
		writeInstallJSON(w, map[string]string{"job_id": job.ID, "code": job.Code, "status": "/api/install/" + job.ID + "/status"})
		return
	}
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/install/"), "/")
	if len(parts) < 2 || !safeSlug.MatchString(parts[0]) {
		http.NotFound(w, r)
		return
	}
	job, ok := m.find(parts[0])
	if !ok {
		http.NotFound(w, r)
		return
	}
	switch parts[1] {
	case "plan":
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", http.MethodGet)
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		writeWiiInstallPlan(w, job)
	case "status":
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		job.mu.Lock()
		result := map[string]interface{}{"job_id": job.ID, "slug": job.Slug, "state": job.State, "bytes": job.Bytes, "total": job.Total, "updated": job.Updated}
		if job.State == "failed" {
			result["error"] = job.Error
		}
		job.Updated = time.Now().UTC()
		job.mu.Unlock()
		writeInstallJSON(w, result)
	case "manifest":
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		job.mu.RLock()
		ready := job.State == "ready"
		manifest := job.Manifest
		job.mu.RUnlock()
		if !ready {
			http.Error(w, "Package is not ready", http.StatusConflict)
			return
		}
		writeInstallJSON(w, manifest)
	case "file":
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if len(parts) < 3 {
			http.NotFound(w, r)
			return
		}
		relative, err := safeInstallPath(strings.Join(parts[2:], "/"))
		if err != nil {
			http.NotFound(w, r)
			return
		}
		job.mu.Lock()
		ready := job.State == "ready" && !job.retired
		workspace := job.Workspace
		listed := false
		for _, file := range job.Manifest.Files {
			if file.Path == relative {
				listed = true
				break
			}
		}
		if ready && listed {
			job.readers++
			job.Updated = time.Now().UTC()
		}
		job.mu.Unlock()
		if !ready {
			http.Error(w, "Package is not ready", http.StatusConflict)
			return
		}
		if !listed {
			http.NotFound(w, r)
			return
		}
		defer job.release()
		target := filepath.Join(workspace, "extracted", filepath.FromSlash(relative))
		if _, err := os.Stat(target); err != nil {
			http.NotFound(w, r)
			return
		}
		http.ServeFile(w, r, target)
	case "complete", "cancel":
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", http.MethodPost)
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		m.mu.Lock()
		delete(m.jobs, job.ID)
		delete(m.codes, job.Code)
		m.mu.Unlock()
		job.mu.Lock()
		job.retireLocked()
		job.mu.Unlock()
		w.WriteHeader(http.StatusNoContent)
	default:
		http.NotFound(w, r)
	}
}

// serveWiiPlan uses a small tab-delimited protocol so the original Wii can
// parse a package without a large JSON library. Archive paths cannot contain
// tabs or line breaks, and Content-Length avoids chunked transfer decoding.
func (m *installManager) serveWiiPlan(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	code := strings.TrimPrefix(r.URL.Path, "/api/install/wii/")
	if len(code) != 8 {
		http.NotFound(w, r)
		return
	}
	for _, digit := range code {
		if digit < '0' || digit > '9' {
			http.NotFound(w, r)
			return
		}
	}
	m.mu.RLock()
	id, ok := m.codes[code]
	job := m.jobs[id]
	m.mu.RUnlock()
	if !ok || job == nil {
		http.NotFound(w, r)
		return
	}
	writeWiiInstallPlan(w, job)
}

func writeWiiInstallPlan(w http.ResponseWriter, job *installJob) {
	job.mu.Lock()
	var body bytes.Buffer
	fmt.Fprintf(&body, "SHROOM1\nSTATE %s\nJOB %s\nSLUG %s\nCOUNT %d\n", job.State, job.ID, job.Slug, len(job.Manifest.Files))
	if job.State == "ready" {
		for _, file := range job.Manifest.Files {
			fmt.Fprintf(&body, "%d\t%s\t%s\n", file.Size, file.SHA256, file.Path)
		}
	}
	job.Updated = time.Now().UTC()
	job.mu.Unlock()
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Length", fmt.Sprint(body.Len()))
	_, _ = w.Write(body.Bytes())
}
