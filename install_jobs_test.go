package main

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNativeInstallStartsWithoutEnteredCode(t *testing.T) {
	m := newInstallManager()
	m.root = t.TempDir()
	archive := testPackage(t, map[string]string{"apps/demo/boot.dol": "homebrew test"})
	m.client = &http.Client{Transport: installRoundTrip(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewReader(archive)), ContentLength: int64(len(archive)), Header: make(http.Header)}, nil
	})}
	app := catalogApp{Slug: "demo"}
	app.URL.Zip = "https://hbb1.oscwii.org/api/contents/demo/demo.zip"
	apps := map[string]catalogApp{"demo": app}

	get := httptest.NewRecorder()
	m.serveHTTP(get, httptest.NewRequest(http.MethodGet, "/api/install/native/demo", nil), apps)
	if get.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET started a job: %d", get.Code)
	}
	unknown := httptest.NewRecorder()
	m.serveHTTP(unknown, httptest.NewRequest(http.MethodPost, "/api/install/native/missing", nil), apps)
	if unknown.Code != http.StatusNotFound {
		t.Fatalf("unknown app: %d", unknown.Code)
	}

	response := httptest.NewRecorder()
	m.serveHTTP(response, httptest.NewRequest(http.MethodPost, "/api/install/native/demo", nil), apps)
	if response.Code != http.StatusOK || !strings.HasPrefix(response.Body.String(), "SHROOMDIRECT1\nJOB ") {
		t.Fatalf("native start: %d %s", response.Code, response.Body.String())
	}
	id := strings.TrimSuffix(strings.TrimPrefix(response.Body.String(), "SHROOMDIRECT1\nJOB "), "\n")
	if len(id) != 48 || strings.Contains(response.Body.String(), "CODE") {
		t.Fatalf("invalid direct job response")
	}
	deadline := time.Now().Add(time.Second)
	for {
		m.mu.RLock()
		job := m.jobs[id]
		m.mu.RUnlock()
		if job == nil {
			t.Fatal("native request has no job")
		}
		job.mu.RLock()
		state := job.State
		job.mu.RUnlock()
		if state == "ready" {
			plan := httptest.NewRecorder()
			m.serveHTTP(plan, httptest.NewRequest(http.MethodGet, "/api/install/"+id+"/plan", nil), apps)
			if plan.Code != 200 || !strings.Contains(plan.Body.String(), "apps/demo/boot.dol") {
				t.Fatal("direct manifest unavailable")
			}
			break
		}
		if state == "failed" || time.Now().After(deadline) {
			t.Fatalf("native job: %s", state)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

type installRoundTrip func(*http.Request) (*http.Response, error)

func (f installRoundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func testPackage(t *testing.T, entries map[string]string) []byte {
	t.Helper()
	var out bytes.Buffer
	w := zip.NewWriter(&out)
	for name, content := range entries {
		file, err := w.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := io.WriteString(file, content); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}

func TestInstallJobFetchVerifyAndCleanup(t *testing.T) {
	archive := testPackage(t, map[string]string{"apps/demo/boot.dol": "homebrew-test-payload", "apps/demo/meta.xml": "<app/>"})
	m := newInstallManager()
	m.root = t.TempDir()
	m.client = &http.Client{Transport: installRoundTrip(func(r *http.Request) (*http.Response, error) {
		if r.URL.Host != "hbb1.oscwii.org" || r.URL.Path != "/api/contents/demo/demo.zip" {
			t.Errorf("unexpected fetch: %s", r.URL)
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewReader(archive)), ContentLength: int64(len(archive)), Header: make(http.Header)}, nil
	})}
	app := catalogApp{Slug: "demo"}
	app.URL.Zip = "https://hbb1.oscwii.org/api/contents/demo/demo.zip"
	apps := map[string]catalogApp{"demo": app}
	request := httptest.NewRequest(http.MethodPost, "/api/install/start", strings.NewReader(url.Values{"slug": {"demo"}}.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response := httptest.NewRecorder()
	m.serveHTTP(response, request, apps)
	if response.Code != http.StatusOK {
		t.Fatalf("start: %d %s", response.Code, response.Body.String())
	}
	var started struct {
		JobID string `json:"job_id"`
		Code  string `json:"code"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &started); err != nil || started.JobID == "" || len(started.Code) != 8 {
		t.Fatalf("bad start response: %v %q", err, response.Body.String())
	}
	job, ok := m.find(started.JobID)
	if !ok {
		t.Fatal("job was not registered")
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		job.mu.RLock()
		state := job.State
		job.mu.RUnlock()
		if state == "ready" {
			break
		}
		if state == "failed" || time.Now().After(deadline) {
			t.Fatalf("job did not become ready: %s", state)
		}
		time.Sleep(10 * time.Millisecond)
	}
	claimResponse := httptest.NewRecorder()
	m.serveHTTP(claimResponse, httptest.NewRequest(http.MethodGet, "/api/install/claim/"+started.Code, nil), apps)
	if claimResponse.Code != http.StatusOK || !strings.Contains(claimResponse.Body.String(), started.JobID) {
		t.Fatalf("pair code did not resolve to ready job: %d %s", claimResponse.Code, claimResponse.Body.String())
	}
	wiiPlan := httptest.NewRecorder()
	m.serveHTTP(wiiPlan, httptest.NewRequest(http.MethodGet, "/api/install/wii/"+started.Code, nil), apps)
	if wiiPlan.Code != http.StatusOK || !strings.HasPrefix(wiiPlan.Body.String(), "SHROOM1\nSTATE ready\n") || !strings.Contains(wiiPlan.Body.String(), "apps/demo/boot.dol") {
		t.Fatalf("Wii plan was not ready: %d %s", wiiPlan.Code, wiiPlan.Body.String())
	}
	manifestResponse := httptest.NewRecorder()
	m.serveHTTP(manifestResponse, httptest.NewRequest(http.MethodGet, "/api/install/"+started.JobID+"/manifest", nil), apps)
	var manifest installManifest
	if err := json.Unmarshal(manifestResponse.Body.Bytes(), &manifest); err != nil || len(manifest.Files) != 2 {
		t.Fatalf("manifest: %v %s", err, manifestResponse.Body.String())
	}
	for _, file := range manifest.Files {
		fileResponse := httptest.NewRecorder()
		m.serveHTTP(fileResponse, httptest.NewRequest(http.MethodGet, "/api/install/"+started.JobID+"/file/"+file.Path, nil), apps)
		if fileResponse.Code != 200 || int64(fileResponse.Body.Len()) != file.Size {
			t.Fatalf("file %s: %d bytes=%d", file.Path, fileResponse.Code, fileResponse.Body.Len())
		}
		hash := sha256.Sum256(fileResponse.Body.Bytes())
		if hex.EncodeToString(hash[:]) != file.SHA256 {
			t.Fatalf("file hash mismatch: %s", file.Path)
		}
	}
	completeResponse := httptest.NewRecorder()
	m.serveHTTP(completeResponse, httptest.NewRequest(http.MethodPost, "/api/install/"+started.JobID+"/complete", nil), apps)
	if completeResponse.Code != http.StatusNoContent {
		t.Fatalf("complete: %d", completeResponse.Code)
	}
	if _, err := os.Stat(filepath.Join(m.root, started.JobID)); !os.IsNotExist(err) {
		t.Fatalf("job files were not removed: %v", err)
	}
	claimResponse = httptest.NewRecorder()
	m.serveHTTP(claimResponse, httptest.NewRequest(http.MethodGet, "/api/install/claim/"+started.Code, nil), apps)
	if claimResponse.Code != http.StatusNotFound {
		t.Fatalf("completed pair code was still valid: %d", claimResponse.Code)
	}
}

func TestInstallArchiveRejectsTraversalAndDuplicatePaths(t *testing.T) {
	for _, test := range []struct {
		name    string
		entries []string
	}{
		{"traversal", []string{"../outside.dol"}},
		{"duplicate", []string{"apps/demo/boot.dol", "apps/demo/boot.dol"}},
		{"line-break", []string{"apps/demo/boot\n.dol"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			archivePath := filepath.Join(t.TempDir(), "source.zip")
			var out bytes.Buffer
			w := zip.NewWriter(&out)
			for _, name := range test.entries {
				file, err := w.Create(name)
				if err != nil {
					t.Fatal(err)
				}
				_, _ = file.Write([]byte("payload"))
			}
			if err := w.Close(); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(archivePath, out.Bytes(), 0600); err != nil {
				t.Fatal(err)
			}
			if _, err := extractInstallArchive(archivePath, filepath.Join(t.TempDir(), "extracted"), "demo"); err == nil {
				t.Fatal("unsafe archive was accepted")
			}
		})
	}
}

func TestInstallURLRejectsUnapprovedHosts(t *testing.T) {
	for _, raw := range []string{
		"http://hbb1.oscwii.org/api/contents/demo/demo.zip",
		"https://hbb1.oscwii.org.evil.example/api/contents/demo/demo.zip",
		"https://hbb1.oscwii.org@evil.example/api/contents/demo/demo.zip",
		"https://hbb1.oscwii.org/other/demo.zip",
	} {
		if _, err := approvedPackageURL(raw); err == nil {
			t.Errorf("approved unsafe URL: %s", raw)
		}
	}
}
