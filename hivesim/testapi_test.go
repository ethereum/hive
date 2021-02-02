package hivesim

import (
	"reflect"
	"testing"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/ethereum/hive/internal/libhive"
)

// This test verifies that test errors are reported correctly through the API.
func TestSuiteReporting(t *testing.T) {
	suite := Suite{
		Name:        "test suite",
		Description: "tests error reporting",
	}
	suite.Add(TestSpec{
		Name:        "passing test",
		Description: "this test passes",
		Run: func(t *T) {
			t.Log("message from the passing test")
		},
	})
	suite.Add(TestSpec{
		Name:        "failing test",
		Description: "this test fails",
		Run: func(t *T) {
			t.Fatal("message from the failing test")
		},
	})

	tm, srv := newFakeAPI(nil)
	defer srv.Close()

	err := RunSuite(NewAt(srv.URL), suite)
	if err != nil {
		t.Fatal("suite run failed:", err)
	}

	tm.Terminate()
	results := tm.Results()
	removeTimestamps(results)

	wantResults := map[libhive.TestSuiteID]*libhive.TestSuite{
		0: {
			ID:             0,
			Name:           suite.Name,
			Description:    suite.Description,
			ClientVersions: make(map[string]string),
			TestCases: map[libhive.TestID]*libhive.TestCase{
				1: {
					Name:        "passing test",
					Description: "this test passes",
					SummaryResult: libhive.TestResult{
						Pass:    true,
						Details: "message from the passing test\n",
					},
				},
				2: {
					Name:        "failing test",
					Description: "this test fails",
					SummaryResult: libhive.TestResult{
						Pass:    false,
						Details: "message from the failing test\n",
					},
				},
			},
		},
	}
	if !reflect.DeepEqual(results, wantResults) {
		t.Fatal("wrong results reported:", spew.Sdump(results))
	}
}

// removeTimestamps removes test timestamps in results so they can be
// compared using reflect.DeepEqual.
func removeTimestamps(result map[libhive.TestSuiteID]*libhive.TestSuite) {
	for _, suite := range result {
		for _, test := range suite.TestCases {
			test.Start = time.Time{}
			test.End = time.Time{}
		}
	}
}
