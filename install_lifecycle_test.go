package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCanceledPreparationStopsWorkerBeforeCleanup(t *testing.T) {
	m := newInstallManager()
	m.root = t.TempDir()
	entered := make(chan struct{})
	m.client = &http.Client{Transport: installRoundTrip(func(r *http.Request) (*http.Response, error) {
		close(entered)
		<-r.Context().Done()
		return nil, r.Context().Err()
	})}
	app := catalogApp{Slug: "demo"}
	app.URL.Zip = "https://hbb1.oscwii.org/api/contents/demo/demo.zip"
	job, err := m.start(app)
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("worker did not start")
	}
	response := httptest.NewRecorder()
	m.serveHTTP(response, httptest.NewRequest("POST", "/api/install/"+job.ID+"/cancel", nil), nil)
	if response.Code != 204 {
		t.Fatal(response.Code)
	}
	deadline := time.Now().Add(time.Second)
	for {
		job.mu.RLock()
		working := job.preparing
		retired := job.retired
		job.mu.RUnlock()
		if !working {
			if !retired {
				t.Fatal("job not retired")
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("canceled worker did not stop")
		}
		time.Sleep(time.Millisecond)
	}
	if _, err = os.Stat(job.Workspace); !os.IsNotExist(err) {
		t.Fatalf("canceled workspace retained: %v", err)
	}
	if _, ok := m.find(job.ID); ok {
		t.Fatal("canceled job still registered")
	}
}

type cancelDuringWrite struct {
	*httptest.ResponseRecorder
	once func()
}

func (w *cancelDuringWrite) Write(b []byte) (int, error) {
	if w.once != nil {
		f := w.once
		w.once = nil
		f()
	}
	return w.ResponseRecorder.Write(b)
}

func TestFileResponseRetainsWorkspaceUntilFinished(t *testing.T) {
	m := newInstallManager()
	root := t.TempDir()
	id := strings.Repeat("a", 48)
	extracted := filepath.Join(root, "extracted", "apps", "demo")
	if err := os.MkdirAll(extracted, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(extracted, "boot.dol"), []byte("homebrew"), 0600); err != nil {
		t.Fatal(err)
	}
	job := &installJob{ID: id, Code: "12345678", State: "ready", Workspace: root, Updated: time.Now()}
	job.Manifest.Files = []installFile{{Path: "apps/demo/boot.dol"}}
	m.jobs[id] = job
	m.codes[job.Code] = id
	w := &cancelDuringWrite{ResponseRecorder: httptest.NewRecorder()}
	unknown := httptest.NewRecorder()
	m.serveHTTP(unknown, httptest.NewRequest("GET", "/api/install/"+id+"/file/apps/demo", nil), nil)
	if unknown.Code != 404 {
		t.Fatal("directory endpoint must not expose a file listing")
	}
	w.once = func() {
		canceled := httptest.NewRecorder()
		m.serveHTTP(canceled, httptest.NewRequest("POST", "/api/install/"+id+"/cancel", nil), nil)
		if canceled.Code != 204 {
			t.Fatal(canceled.Code)
		}
		if _, err := os.Stat(root); err != nil {
			t.Fatalf("workspace removed during active response: %v", err)
		}
	}
	m.serveHTTP(w, httptest.NewRequest("GET", "/api/install/"+id+"/file/apps/demo/boot.dol", nil), nil)
	if w.Code != 200 || w.Body.String() != "homebrew" {
		t.Fatalf("response %d %q", w.Code, w.Body.String())
	}
	if _, err := os.Stat(root); !os.IsNotExist(err) {
		t.Fatalf("workspace not cleaned after response: %v", err)
	}
}

func TestActiveDownloadDoesNotExpire(t *testing.T) {
	m := newInstallManager()
	root := t.TempDir()
	job := &installJob{ID: "active", Workspace: root, Updated: time.Now().Add(-2 * installJobTTL), State: "ready", readers: 1}
	m.jobs[job.ID] = job
	m.removeExpired()
	if _, ok := m.find(job.ID); !ok {
		t.Fatal("expired an active transfer")
	}
	job.release()
	job.mu.Lock()
	job.Updated = time.Now().Add(-2 * installJobTTL)
	job.mu.Unlock()
	m.removeExpired()
	if _, ok := m.find(job.ID); ok {
		t.Fatal("idle job not expired")
	}
}

func TestFATPathsRejected(t *testing.T) {
	for _, path := range []string{"apps/demo/file.", "apps/demo/file ", "apps/demo/file?.dol", "apps/demo/CON.txt", "apps/demo/LPT1", "apps/demo/a|b"} {
		if _, err := safeInstallPath(path); err == nil {
			t.Errorf("accepted %q", path)
		}
	}
}

func TestServerRejectsUnbootableAndFATDuplicatePackages(t *testing.T) {
	for _, entries := range []map[string]string{
		{"apps/demo/meta.xml": "no executable"},
		{"apps/demo/boot.dol": "boot", "apps/demo/BOOT.DOL": "alias"},
	} {
		zipPath := filepath.Join(t.TempDir(), "package.zip")
		if err := os.WriteFile(zipPath, testPackage(t, entries), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := extractInstallArchive(zipPath, t.TempDir(), "demo"); err == nil {
			t.Fatal("accepted incompatible package")
		}
	}
}
