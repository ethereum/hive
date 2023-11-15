package libhive

import (
	"bytes"
	"io"
	"testing"
)

type nopCloser struct {
	io.Writer
}

func (c nopCloser) Close() error {
	// No-op
	return nil
}

type testWriter struct {
	fileMap map[string]*bytes.Buffer
}

func (tw *testWriter) Contains(file string) bool {
	for k := range tw.fileMap {
		if k == file {
			return true
		}
	}
	return false
}

func (tw *testWriter) CreateWriter(path string) (io.WriteCloser, error) {
	if _, ok := tw.fileMap[path]; !ok {
		tw.fileMap[path] = &bytes.Buffer{}
	}
	return nopCloser{tw.fileMap[path]}, nil
}

func TestSuiteDocsGen(t *testing.T) {
	suiteMap := map[TestSuiteID]*TestSuite{
		1: {
			Name:        "suite1",
			Description: `This is a description of suite1.`,
			TestCases: map[TestID]*TestCase{
				1: {
					Name:        "test1",
					Description: `This is a description of test1 in suite1.`,
				},
				2: {
					Name:        "test2",
					Description: `This is a description of test2 in suite1.`,
				},
			},
		},
		2: {
			Name:        "suite2",
			DisplayName: "Suite 2",
			Description: `This is a description of suite2.`,
			TestCases: map[TestID]*TestCase{
				1: {
					Name:        "testA",
					Description: `This is a description of testA in suite2.`,
				},
				2: {
					Name:        "testB",
					DisplayName: "Test B",
					Description: `This is a description of testB in suite2.`,
				},
			},
		},
	}
	tw := &testWriter{map[string]*bytes.Buffer{}}
	if err := GenSimulatorMarkdownFiles(tw, "sim", suiteMap); err != nil {
		t.Fatal(err)
	}
	if len(tw.fileMap) != 3 {
		t.Fatalf("expected 3 files, got %d", len(tw.fileMap))
	}

	if !tw.Contains("TESTS.md") {
		t.Fatal("TESTS.md not found")
	}

	if !tw.Contains("TESTS-SUITE1.md") {
		t.Fatal("TESTS-SUITE1.md not found")
	}

	if !tw.Contains("TESTS-SUITE2.md") {
		t.Fatal("TESTS-SUITE2.md not found")
	}
}
