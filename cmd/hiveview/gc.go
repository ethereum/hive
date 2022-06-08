package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/ethereum/hive/internal/libhive"
)

func logdirGC(dir string, cutoff time.Time) error {
	var (
		fsys      = os.DirFS(dir)
		usedFiles = make(map[string]struct{})
	)

	// Walk all suite files and pouplate the usedFiles set.
	err := walkSummaryFiles(fsys, ".", func(suite *libhive.TestSuite, fi fs.FileInfo) error {
		if suiteStart(suite).Before(cutoff) {
			return nil // skip
		}
		// Add suite file itself.
		usedFiles[fi.Name()] = struct{}{}
		usedFiles[suite.SimulatorLog] = struct{}{}
		// Add log files.
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

	// Delete all files which aren't in usedFiles.
	return fs.WalkDir(fsys, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // Ignore scan errors.
		}
		if d.IsDir() {
			return nil // Don't delete directories.
		}
		if _, used := usedFiles[path]; !used {
			path := filepath.Join(dir, filepath.FromSlash(path))
			fmt.Println("rm", path)
			err := os.Remove(path)
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
