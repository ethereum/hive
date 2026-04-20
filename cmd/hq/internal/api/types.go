// Package api is the client for the hive result server HTTP API and the
// JSON types it produces. The types here mirror the structures emitted by
// hiveview's listing and result files.
package api

import "time"

// Discovery is one entry in discovery.json, describing a suite and where its
// results are published.
type Discovery struct {
	Name            string   `json:"name"`
	Address         string   `json:"address"`
	GithubWorkflows []string `json:"github_workflows"`
}

// ListingEntry is a single line of a suite's listing.jsonl: one test run
// with aggregate counts and client metadata.
type ListingEntry struct {
	Name     string            `json:"name"`
	NTests   int               `json:"ntests"`
	Passes   int               `json:"passes"`
	Fails    int               `json:"fails"`
	Timeout  bool              `json:"timeout"`
	Clients  []string          `json:"clients"`
	Versions map[string]string `json:"versions"`
	Start    time.Time         `json:"start"`
	FileName string            `json:"fileName"`
	Size     int64             `json:"size"`
	SimLog   string            `json:"simLog"`
}

// TestSuiteResult is the body of a per-run result JSON file. TestCases maps a
// numeric case ID to the case detail.
type TestSuiteResult struct {
	ID             int                 `json:"id"`
	Name           string              `json:"name"`
	Description    string              `json:"description"`
	ClientVersions map[string]string   `json:"clientVersions"`
	TestCases      map[string]TestCase `json:"testCases"`
	TestDetailsLog string              `json:"testDetailsLog"`
	SimLog         string              `json:"simLog"`
}

// TestCase is a single test within a suite result.
type TestCase struct {
	Name          string                `json:"name"`
	Description   string                `json:"description"`
	Start         time.Time             `json:"start"`
	End           time.Time             `json:"end"`
	SummaryResult SummaryResult         `json:"summaryResult"`
	ClientInfo    map[string]ClientInfo `json:"clientInfo"`
}

// SummaryResult is the pass/fail outcome of a test case, plus the byte range
// into the suite's details log that holds the case's log output.
type SummaryResult struct {
	Pass    bool   `json:"pass"`
	Details string `json:"details"`
	Log     struct {
		Begin int64 `json:"begin"`
		End   int64 `json:"end"`
	} `json:"log"`
}

// ClientInfo describes one client instance used by a test case.
type ClientInfo struct {
	ID             string    `json:"id"`
	IP             string    `json:"ip"`
	Name           string    `json:"name"`
	InstantiatedAt time.Time `json:"instantiatedAt"`
	LogFile        string    `json:"logFile"`
}
