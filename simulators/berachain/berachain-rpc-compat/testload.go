package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/ethereum/hive/hivesim"
	"github.com/tidwall/gjson"
)

// Copied and adapted from rpc-compat/testload.go
type rpcTest struct {
	name     string
	comment  string
	speconly bool
	messages []rpcTestMessage
}

type rpcTestMessage struct {
	data string
	// if true, the message is a send (>>), otherwise it's a receive (<<)
	send bool
}

func loadTestFile(name string, r io.Reader) (rpcTest, error) {
	var (
		rdr      = bufio.NewReader(r)
		scan     = bufio.NewScanner(rdr)
		buf      = make([]byte, 0, 64*1024)
		inHeader = true
		test     = rpcTest{name: name}
	)
	scan.Buffer(buf, 1024*1024)
	for scan.Scan() {
		line := strings.TrimSpace(scan.Text())
		switch {
		case len(line) == 0:
			continue

		case strings.HasPrefix(line, "//"):
			if !inHeader {
				continue // ignore comments after requests
			}
			text := strings.TrimPrefix(strings.TrimPrefix(line, "//"), " ")
			test.comment += text + "\n"
			if strings.HasPrefix(text, "speconly:") {
				test.speconly = true
			}

		case strings.HasPrefix(line, ">>") || strings.HasPrefix(line, "<<"):
			inHeader = false
			data := strings.TrimSpace(line[2:])
			if !gjson.Valid(data) {
				return test, fmt.Errorf("invalid JSON in line %q", line)
			}
			test.messages = append(test.messages, rpcTestMessage{
				data: data,
				send: strings.HasPrefix(line, ">>"),
			})

		default:
			return test, fmt.Errorf("invalid test line: %q", line)
		}
	}
	return test, scan.Err()
}

// loadTests walks the given directory looking for *.io files to load.
func loadTests(t *hivesim.T, root string, re *regexp.Regexp) []rpcTest {
	var tests []rpcTest
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			t.Logf("unable to walk path: %s", err)
			return err
		}
		if info.IsDir() {
			return nil
		}
		if fname := info.Name(); !strings.HasSuffix(fname, ".io") {
			return nil
		}
		pathname := strings.TrimSuffix(strings.TrimPrefix(path, root+"/"), ".io")
		if !re.MatchString(pathname) {
			fmt.Println("skip", pathname)
			return nil // skip
		}
		fd, err := os.Open(path)
		if err != nil {
			return err
		}
		defer fd.Close()
		test, err := loadTestFile(pathname, fd)
		if err != nil {
			return fmt.Errorf("invalid test %s: %v", info.Name(), err)
		}
		tests = append(tests, test)
		return nil
	})
	if err != nil {
		t.Fatalf("failed to load tests: %s", err)
	}
	return tests
}