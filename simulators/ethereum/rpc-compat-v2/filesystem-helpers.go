// Contains cross-platform compatible functions for reading and writing files.
package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// ------------------------------- Folder -------------------------------------

// FolderExistsAtRelativePath assumes that the input was constructed via filepath.Join()
func FolderExistsAtRelativePath(relPath string) bool {
	info, err := os.Stat(relPath)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			panic(err)
		}
		// does not exist
		return false

	}

	// it exists, but is it a file or a folder
	return info.IsDir()
}

func CreateFolderAtRelativePath(relPath string) {
	// what if in the end it was all about the folders we made along the way
	err := os.MkdirAll(relPath, 0o755) // rwxr-xr-x
	if err != nil {
		panic(err)
	}
}

// GetFilesInFolderRecursively returns the filepaths of any file with the given fileExtension in any subdir.
func GetFilesInFolderRecursively(relPath string, fileExtension string, re *regexp.Regexp) []string {
	if !strings.HasPrefix(fileExtension, ".") {
		fileExtension = "." + fileExtension
	}

	var fileList []string
	err := filepath.WalkDir(relPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			panic(err)
		}
		if d.IsDir() {
			return nil
		}
		// found a file with required extension (.io)
		if strings.EqualFold(filepath.Ext(d.Name()), fileExtension) {
			// skip it if it does not match provided regex
			if !re.MatchString(path) {
				fmt.Println("skip", path)
				return nil
			}

			fileList = append(fileList, path)
		}
		return nil
	})
	if err != nil {
		panic(err)
	}

	return fileList
}

// ------------------------------- File ---------------------------------------

// FileExistsAtRelativePath assumes that the input was constructed via filepath.Join()
func FileExistsAtRelativePath(relPath string) bool {
	info, err := os.Stat(relPath)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			panic(err)
		}
		// does not exist
		return false

	}

	// it exists, but is it a file or a folder
	return !info.IsDir()
}

// CreateFileAtRelativePath will overwrite any existing file.
func CreateFileAtRelativePath(relPath string, fileContent []byte) {
	// ensure all folders that would be needed exist,
	// but drop the last folder (it want to write a file, not a folder)
	folderToCreate := filepath.Dir(filepath.Clean(relPath))
	CreateFolderAtRelativePath(folderToCreate)

	err := os.WriteFile(relPath, fileContent, 0o644) // rw-r--r--
	if err != nil {
		panic(err)
	}
}

// ReadFileAtRelativePath reads a file into a slice of lines.
func ReadFileAtRelativePath(relPath string) []string {
	file, err := os.Open(relPath)
	if err != nil {
		panic(err)
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)

	// increase max token size from default of 64 KiB to 10 MiB
	// (required for e.g. tests/eth_sendRawTransaction/send-blob-tx.io)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, MAX_LINE_LENGTH)

	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		panic(err)
	}

	return lines
}

// LoadJSON is from geth's ./common/test_utils.go
func LoadJSON(file string, val interface{}) error {
	content, err := os.ReadFile(file)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(content, val); err != nil {
		if syntaxerr, ok := err.(*json.SyntaxError); ok {
			line := findLine(content, syntaxerr.Offset)
			return fmt.Errorf("JSON syntax error at %v:%v: %v", file, line, err)
		}
		return fmt.Errorf("JSON unmarshal error in %v: %v", file, err)
	}
	return nil
}

// findLine is from geth's ./common/test_utils.go
func findLine(data []byte, offset int64) (line int) {
	line = 1
	for i, r := range string(data) {
		if int64(i) >= offset {
			return
		}
		if r == '\n' {
			line++
		}
	}
	return
}
