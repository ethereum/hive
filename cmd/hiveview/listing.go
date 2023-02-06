package main

import (
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"log"
	"path"
	"sort"
	"strings"
	"time"

	"github.com/ethereum/hive/internal/libhive"
)

const listLimit = 200 // number of runs reported

// generateListing processes hive simulation output files and generates a listing file.
func generateListing(fsys fs.FS, dir string, output io.Writer) error {
	var (
		stop    = errors.New("stop")
		entries []listingEntry
	)
	// The files are walked in name order high->low. So to get the latest 200 items, we
	// just need to keep going until we have 200.
	err := walkSummaryFiles(fsys, dir, func(suite *libhive.TestSuite, fi fs.FileInfo) error {
		entry := suiteToEntry(suite, fi)
		entries = append(entries, entry)
		if len(entries) >= listLimit {
			return stop
		}
		return nil
	})
	if err != nil && err != stop {
		return err
	}

	// Write listing JSON lines to output.
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].SimLog > entries[j].SimLog
	})
	enc := json.NewEncoder(output)
	for _, e := range entries {
		if err := enc.Encode(e); err != nil {
			// No need to report write errors here: if it's writing to a file, nobody will
			// see the error anyway. If writing to HTTP, client has already started processing
			// the listing and we can't tell it about the error anymore.
			break
		}
	}
	return nil
}

type listingEntry struct {
	// Test suite information.
	Name   string `json:"name"`
	NTests int    `json:"ntests"`
	// Info about this run.
	Passes   int       `json:"passes"`
	Fails    int       `json:"fails"`
	Timeout  bool      `json:"timeout"`
	Clients  []string  `json:"clients"`  // client names involved in this run
	Start    time.Time `json:"start"`    // timestamp of test start (ISO 8601 format)
	FileName string    `json:"fileName"` // hive output file
	Size     int64     `json:"size"`     // size of hive output file
	SimLog   string    `json:"simLog"`   // simulator log file
}

func suiteToEntry(s *libhive.TestSuite, file fs.FileInfo) listingEntry {
	e := listingEntry{
		Name:     s.Name,
		FileName: file.Name(),
		Size:     file.Size(),
		SimLog:   s.SimulatorLog,
		Clients:  make([]string, 0),
	}
	for _, test := range s.TestCases {
		e.NTests++
		if test.SummaryResult.Pass {
			e.Passes++
		} else {
			e.Fails++
		}
		if test.SummaryResult.Timeout {
			e.Timeout = true
		}
		if e.Start.IsZero() || test.Start.Before(e.Start) {
			e.Start = test.Start
		}
		for _, client := range test.ClientInfo {
			if !contains(e.Clients, client.Name) {
				e.Clients = append(e.Clients, client.Name)
			}
		}
	}
	return e
}

func contains(list []string, s string) bool {
	for _, elem := range list {
		if elem == s {
			return true
		}
	}
	return false
}

type suiteCB func(*libhive.TestSuite, fs.FileInfo) error

func walkSummaryFiles(fsys fs.FS, dir string, proc suiteCB) error {
	logfiles, err := fs.ReadDir(fsys, dir)
	if err != nil {
		return err
	}
	// Sort by name newest-first.
	sort.Slice(logfiles, func(i, j int) bool {
		return logfiles[i].Name() > logfiles[j].Name()
	})

	for _, entry := range logfiles {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".json") || skipFile(name) {
			continue
		}
		suite, fileInfo := parseSuite(fsys, path.Join(dir, name))
		if suite != nil {
			if err := proc(suite, fileInfo); err != nil {
				return err
			}
		}
	}
	return nil
}

func parseSuite(fsys fs.FS, path string) (*libhive.TestSuite, fs.FileInfo) {
	file, err := fsys.Open(path)
	if err != nil {
		log.Printf("Can't access summary file: %s", err)
		return nil, nil
	}
	defer file.Close()

	fileInfo, err := file.Stat()
	if err != nil {
		log.Printf("Can't access summary file: %s", err)
		return nil, nil
	}

	var info libhive.TestSuite
	if err := json.NewDecoder(file).Decode(&info); err != nil {
		log.Printf("Skipping invalid summary file %s: %v", fileInfo.Name(), err)
		return nil, nil
	}
	if !suiteValid(&info) {
		log.Printf("Skipping invalid summary file %s", fileInfo.Name())
		return nil, nil
	}
	return &info, fileInfo
}

func suiteValid(s *libhive.TestSuite) bool {
	return s.SimulatorLog != ""
}

func skipFile(f string) bool {
	return f == "errorReport.json" || f == "containerErrorReport.json" || f == "hive.json" || strings.HasPrefix(f, ".")
}
