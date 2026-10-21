package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

const gcTestOldSuite = `{
	"id": 0,
	"name": "sim/old-suite",
	"simLog": "1577836800-simulator-old.log",
	"testDetailsLog": "details/1577836800-old-0.log",
	"testCases": {
		"1": {
			"name": "test",
			"start": "2020-01-01T00:00:00Z",
			"end": "2020-01-01T00:00:01Z",
			"summaryResult": {"pass": true},
			"clientInfo": {
				"aaaa": {"id": "aaaa", "name": "client-a", "logFile": "client-a/client-old.log"}
			}
		}
	}
}`

func TestLogdirGCKeepsRecentFiles(t *testing.T) {
	dir := t.TempDir()
	oldTime := time.Date(2020, 1, 1, 0, 0, 1, 0, time.UTC)
	oldFiles := map[string]string{
		"1577836801-old.json":             gcTestOldSuite,
		"1577836800-simulator-old.log":    "",
		"details/1577836800-old-0.log":    "",
		"client-a/client-old.log":         "",
		"1577836800-simulator-orphan.log": "",
	}
	for name, content := range oldFiles {
		writeGCTestFile(t, dir, name, content)
		if err := os.Chtimes(filepath.Join(dir, name), oldTime, oldTime); err != nil {
			t.Fatal(err)
		}
	}
	// A running simulation has written logs, but no suite file yet.
	runningFiles := []string{
		"1800000000-simulator-running.log",
		"details/1800000000-running-0.log",
		"client-a/client-running.log",
	}
	for _, name := range runningFiles {
		writeGCTestFile(t, dir, name, "")
	}

	cutoff := time.Now().Add(-30 * durationDays)
	if err := logdirGC(dir, cutoff, 0); err != nil {
		t.Fatal("logdirGC:", err)
	}

	for name := range oldFiles {
		if _, err := os.Stat(filepath.Join(dir, name)); !os.IsNotExist(err) {
			t.Errorf("old file %s was not deleted (stat error: %v)", name, err)
		}
	}
	for _, name := range runningFiles {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Errorf("recent file %s was deleted: %v", name, err)
		}
	}
}

func writeGCTestFile(t *testing.T, dir, name, content string) {
	t.Helper()
	path := filepath.Join(dir, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}
