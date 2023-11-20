package hivesim

import (
	"bytes"
	"io"
	"testing"

	"github.com/ethereum/hive/internal/simapi"
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
	suiteMap := map[SuiteID]*markdownSuite{
		1: {
			TestRequest: simapi.TestRequest{
				Name:        "suite1",
				Description: `This is a description of suite1.`,
			},
			tests: map[TestID]*markdownTestCase{
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
			TestRequest: simapi.TestRequest{
				Name:        "suite2",
				DisplayName: "Suite 2",
				Location:    "suite2",
				Description: `This is a description of suite2.`,
			},
			tests: map[TestID]*markdownTestCase{
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
	docsSim := &docsSimulation{
		suites:  suiteMap,
		simName: "sim",
	}
	tw := &testWriter{map[string]*bytes.Buffer{}}
	if err := docsSim.genSimulatorMarkdownFiles(tw); err != nil {
		t.Fatal(err)
	}
	if len(tw.fileMap) != 3 {
		t.Fatalf("expected 3 files, got %d", len(tw.fileMap))
	}

	for _, expFile := range []string{"TESTS.md", "TESTS-SUITE1.md", "suite2/TESTS.md"} {
		if !tw.Contains(expFile) {
			t.Fatalf("expected file %s not found", expFile)
		}
	}
}
