package common

import (
	"bytes"
	"io/ioutil"
	"os"
	"testing"
)

// TestTruncation tests that the line-truncator works
func TestTruncation(t *testing.T) {

	// fill a file with random strings
	f, err := ioutil.TempFile("", "")
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 100; i++ {
		f.WriteString("BOLLorem ipsum dolor sit amet, consectetur adipiscing elit, sed do eiusmod tempor incididunt ut labore et dolore magna aliqua. Ut enim ad minim veniam, quis nostrud exercitation ullamco laborisEOL\n" +
			"BOL nisi ut aliquip ex ea commodo consequat. Duis aute irure dolor in reprehenderit in voluptate velit esse cillum dolore eu fugiat nulla pariatur. Excepteur sint occaecat cupidatat non proident, sunt inEOL\n" +
			"BOLculpa qui officia deserunt mollit anim id est laborum.EOL\n")
	}
	loc := f.Name()
	f.Close()
	defer os.Remove(loc)
	stat, err := os.Stat(loc)
	if err != nil {
		t.Fatal(err)
	}
	// size is 46500
	if got, exp := stat.Size(), int64(40000); got < exp {
		t.Errorf("File too small, got %d expected > %d", got, exp)
	}
	if err = truncateHead(loc, 1000); err != nil {
		t.Log(loc)
		t.Fatal(err)
	}
	// Now, the head should start with "BOL"
	stat, err = os.Stat(loc)
	if err != nil {
		t.Fatal(err)
	}
	if got, exp := stat.Size(), int64(1000); got > exp {
		t.Errorf("File too large, got %d expected < %d", got, exp)
	}
	data, err := ioutil.ReadFile(loc)
	if err != nil {
		t.Fatal(err)
	}
	if got, exp := data[:3], []byte("BOL"); !bytes.Equal(got, exp) {
		t.Errorf("Lines not preserved, exp %v got %v", exp, got)
	}
}
