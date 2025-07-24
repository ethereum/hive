package hivesim

import (
	"reflect"
	"sort"
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

// removeTimestamps removes test timestamps and runtime metadata in results so they can be
// compared using reflect.DeepEqual.
func removeTimestamps(result map[libhive.TestSuiteID]*libhive.TestSuite) {
	for _, suite := range result {
		// Clear runtime metadata that varies between test runs
		suite.RunMetadata = nil
		for _, test := range suite.TestCases {
			test.Start = time.Time{}
			test.End = time.Time{}
		}
	}
}

var testPatternTests = []struct {
	Pattern string
	WantRun []string
}{
	{
		Pattern: "/test-b",
		WantRun: []string{
			"suite-a.always",
			"suite-a.test-b",
			"suite-b.always",
			"suite-b.test-b",
		},
	},
	{
		Pattern: "suite-a",
		WantRun: []string{
			"suite-a.always",
			"suite-a.test-a",
			"suite-a.test-b",
		},
	},
}

// This test verifies that suites and test cases are skipped when the test
// pattern does not match.
func TestSkipping(t *testing.T) {
	suiteA := Suite{Name: "suite-a"}
	suiteA.Add(TestSpec{Name: "test-a", Run: func(t *T) {}})
	suiteA.Add(TestSpec{Name: "test-b", Run: func(t *T) {}})
	suiteA.Add(TestSpec{Name: "always", Run: func(t *T) {}, AlwaysRun: true})

	suiteB := Suite{Name: "suite-b"}
	suiteB.Add(TestSpec{Name: "test-a", Run: func(t *T) {}})
	suiteB.Add(TestSpec{Name: "test-b", Run: func(t *T) {}})
	suiteB.Add(TestSpec{Name: "always", Run: func(t *T) {}, AlwaysRun: true})

	for _, test := range testPatternTests {
		tm, srv := newFakeAPI(nil)
		defer srv.Close()

		sim := NewAt(srv.URL)
		sim.SetTestPattern(test.Pattern)

		err := Run(sim, suiteA, suiteB)
		if err != nil {
			t.Fatal("run failed:", err)
		}
		srv.Close()

		// Collect names of executed test cases.
		tm.Terminate()
		results := tm.Results()
		removeTimestamps(results)
		var cases []string
		for _, suite := range results {
			for _, testCase := range suite.TestCases {
				name := suite.Name + "." + testCase.Name
				cases = append(cases, name)
			}
		}
		sort.Strings(cases)

		if !reflect.DeepEqual(cases, test.WantRun) {
			t.Errorf("pattern %q wrong executed test cases: %v", test.Pattern, cases)
		}
	}
}
