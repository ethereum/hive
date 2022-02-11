package main

import (
	"encoding/json"
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
	logfiles, err := fs.ReadDir(fsys, dir)
	if err != nil {
		return err
	}

	// Sort by name.
	sort.Slice(logfiles, func(i, j int) bool {
		return logfiles[i].Name() < logfiles[j].Name()
	})

	// The files are prefixed by timestamp, so to get the latest 200 items, we just need
	// to read the listing in reverse until we have 200.
	var entries []listingEntry
	for i := len(logfiles) - 1; i > 0; i-- {
		name := logfiles[i].Name()
		if !strings.HasSuffix(name, ".json") || skipFile(name) {
			continue
		}
		file, err := fsys.Open(path.Join(dir, name))
		if err != nil {
			continue
		}

		entry, err := convertSummaryFile(file)
		file.Close()

		if err != nil {
			continue
		}
		entries = append(entries, entry)
		if len(entries) >= listLimit {
			break
		}
	}

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
	Clients  []string  `json:"clients"`  // client names involved in this run
	Start    time.Time `json:"start"`    // timestamp of test start (ISO 8601 format)
	FileName string    `json:"fileName"` // hive output file
	Size     int64     `json:"size"`     // size of hive output file
	SimLog   string    `json:"simLog"`   // simulator log file
}

func convertSummaryFile(file fs.File) (listingEntry, error) {
	fileInfo, err := file.Stat()
	if err != nil {
		log.Printf("Can't access summary file: %s", err)
	}

	var info libhive.TestSuite
	if err := json.NewDecoder(file).Decode(&info); err != nil {
		log.Printf("Skipping invalid summary file %s: %v", fileInfo.Name(), err)
		return listingEntry{}, err
	}
	if !suiteValid(&info) {
		log.Printf("Skipping invalid summary file %s", fileInfo.Name())
		return listingEntry{}, err
	}
	return suiteToEntry(&info, fileInfo), nil
}

func suiteValid(s *libhive.TestSuite) bool {
	return s.SimulatorLog != ""
}

func skipFile(f string) bool {
	return f == "errorReport.json" || f == "containerErrorReport.json" || strings.HasPrefix(f, ".")
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
