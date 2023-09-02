package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/ethereum/hive/internal/libhive"
)

func logdirGC(dir string, cutoff time.Time, keepMin int) error {
	var (
		fsys       = os.DirFS(dir)
		usedFiles  = make(map[string]struct{})
		keptSuites = 0
		oldest     time.Time
	)

	// Avoid deleting the status/version file.
	usedFiles["hive.json"] = struct{}{}

	// Walk all suite files and pouplate the usedFiles set.
	err := walkSummaryFiles(fsys, ".", func(suite *libhive.TestSuite, fi fs.FileInfo) error {
		// Skip when too old and when above the minimum.
		// Note we rely on getting called in descending time order here.
		if suiteStart(suite).Before(cutoff) && keptSuites >= keepMin {
			return nil
		}
		if oldest.IsZero() || suiteStart(suite).Before(oldest) {
			oldest = suiteStart(suite)
		}

		// Add suite files and client logs.
		keptSuites++
		usedFiles[fi.Name()] = struct{}{}
		usedFiles[suite.SimulatorLog] = struct{}{}
		if suite.TestDetailsLog != "" {
			usedFiles[suite.TestDetailsLog] = struct{}{}
		}
		for _, test := range suite.TestCases {
			for _, client := range test.ClientInfo {
				usedFiles[client.LogFile] = struct{}{}
			}
		}
		return nil
	})
	if err != nil {
		return err
	}

	fmt.Printf("keeping %d suites (%d files)\n", keptSuites, len(usedFiles))
	fmt.Println("oldest suite date:", oldest)

	// Delete all files which aren't in usedFiles.
	return fs.WalkDir(fsys, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // Ignore scan errors.
		}
		if d.IsDir() {
			return nil // Don't delete directories.
		}
		if _, used := usedFiles[path]; !used {
			file := filepath.Join(dir, filepath.FromSlash(path))
			// fmt.Println("rm", file)
			err := os.Remove(file)
			if err != nil {
				fmt.Println("error:", err)
			}
		}
		return nil
	})
}

func suiteStart(suite *libhive.TestSuite) time.Time {
	for _, test := range suite.TestCases {
		return test.Start
	}
	return time.Time{}
}
