package main

import (
	"bytes"
	"encoding/json"
	"testing"
	"testing/fstest"
)

// The suite contains two real tests (one pass, one fail) and a
// multi-test context entry that owns shared clients. The context entry
// must not be counted as a test, but its clients must still be listed.
const listingTestSuite = `{
	"id": 0,
	"name": "sim/test-suite",
	"description": "A test suite.",
	"clientVersions": {"client-a": "1.0"},
	"simLog": "1699000000-simulator-a1.log",
	"testDetailsLog": "details.log",
	"testCases": {
		"1": {
			"name": "sim/test-suite-multi-test-clients",
			"description": "Multi-test client context",
			"start": "2023-11-03T09:00:00Z",
			"end": "2023-11-03T10:00:00Z",
			"summaryResult": {"pass": true},
			"clientInfo": {
				"aaaa": {"id": "aaaa", "name": "client-a", "logFile": "client-a/aaaa.log"}
			},
			"multiTestContext": true
		},
		"2": {
			"name": "test-pass",
			"start": "2023-11-03T09:00:01Z",
			"end": "2023-11-03T09:00:02Z",
			"summaryResult": {"pass": true},
			"clientInfo": {}
		},
		"3": {
			"name": "test-fail",
			"start": "2023-11-03T09:00:02Z",
			"end": "2023-11-03T09:00:03Z",
			"summaryResult": {"pass": false},
			"clientInfo": {}
		}
	}
}`

func TestGenerateListingSkipsMultiTestContext(t *testing.T) {
	fsys := fstest.MapFS{
		"results/1699000001-suite.json": &fstest.MapFile{Data: []byte(listingTestSuite)},
	}

	var out bytes.Buffer
	if err := generateListing(fsys, "results", &out, 100); err != nil {
		t.Fatalf("generateListing failed: %v", err)
	}

	var entry listingEntry
	if err := json.Unmarshal(out.Bytes(), &entry); err != nil {
		t.Fatalf("can't decode listing entry: %v", err)
	}
	if entry.NTests != 2 {
		t.Errorf("wrong NTests: got %d, want 2", entry.NTests)
	}
	if entry.Passes != 1 {
		t.Errorf("wrong Passes: got %d, want 1", entry.Passes)
	}
	if entry.Fails != 1 {
		t.Errorf("wrong Fails: got %d, want 1", entry.Fails)
	}
	// The context entry's clients must still be reported.
	if len(entry.Clients) != 1 || entry.Clients[0] != "client-a" {
		t.Errorf("wrong Clients: got %v, want [client-a]", entry.Clients)
	}
	// The context entry starts first, so it defines the start time.
	if got := entry.Start.Format("15:04:05"); got != "09:00:00" {
		t.Errorf("wrong Start: got %s, want 09:00:00", got)
	}
}
