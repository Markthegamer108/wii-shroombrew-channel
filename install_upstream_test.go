package main

import (
	"os"
	"testing"
	"time"
)

// Explicitly opt in: fetch a small real OSC package, extract and hash it.
// This neither installs nor executes the downloaded program.
func TestRealUpstreamPackagePreparation(t *testing.T) {
	if os.Getenv("SHROOMBREW_PACKAGE_CHECK") != "1" {
		t.Skip("set SHROOMBREW_PACKAGE_CHECK=1 for a real upstream download")
	}
	s, err := newStorefront("catalog.json")
	if err != nil {
		t.Fatal(err)
	}
	app, ok := s.bySlug["WiiDonut"]
	if !ok {
		t.Fatal("reference homebrew package absent")
	}
	s.installs.root = t.TempDir()
	started := time.Now()
	job, err := s.installs.start(app)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { job.mu.Lock(); job.retireLocked(); job.mu.Unlock() }()
	deadline := time.Now().Add(55 * time.Second)
	for {
		job.mu.RLock()
		state := job.State
		job.mu.RUnlock()
		if state == "ready" {
			break
		}
		if state == "failed" || time.Now().After(deadline) {
			t.Fatalf("real preparation ended in %s", state)
		}
		time.Sleep(20 * time.Millisecond)
	}
	if err := verifyPreparedJob(job); err != nil {
		t.Fatal(err)
	}
	job.mu.RLock()
	files := len(job.Manifest.Files)
	bytes := job.Bytes
	job.mu.RUnlock()
	if files == 0 || bytes == 0 {
		t.Fatal("empty package")
	}
	t.Logf("Real upstream package: %d files; %d compressed bytes; ready in %s; hashes verified", files, bytes, time.Since(started).Round(time.Millisecond))
}
