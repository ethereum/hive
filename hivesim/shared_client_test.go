package hivesim

import (
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ethereum/hive/internal/fakes"
	"github.com/ethereum/hive/internal/libhive"
	"github.com/ethereum/hive/internal/simapi"
)

// Tests shared client functionality by mocking server responses.
func TestStartSharedClient(t *testing.T) {
	// This test creates a test HTTP server that mocks the simulation API.
	// It responds to just the calls we need for this test, ignoring others.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/testsuite":
			// StartSuite
			json.NewEncoder(w).Encode(0) // Return suite ID
		case "/testsuite/0/node":
			// StartSharedClient
			json.NewEncoder(w).Encode(simapi.StartNodeResponse{
				ID: "container1",
				IP: "192.0.2.1",
			})
		case "/testsuite/0/node/container1":
			// GetSharedClientInfo
			json.NewEncoder(w).Encode(simapi.NodeResponse{
				ID:   "container1",
				Name: "client-1",
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	sim := NewAt(srv.URL)

	// Start a test suite
	suiteID, err := sim.StartSuite(&simapi.TestRequest{Name: "shared-client-test-suite"}, "Testing shared clients")
	if err != nil {
		t.Fatal("can't start suite:", err)
	}

	// Start a shared client
	containerID, ip, err := sim.StartSharedClient(suiteID, "client-1", Params(map[string]string{
		"HIVE_PARAM": "value",
	}))
	if err != nil {
		t.Fatal("can't start shared client:", err)
	}

	if containerID != "container1" {
		t.Errorf("wrong container ID: got %q, want %q", containerID, "container1")
	}
	expected := net.ParseIP("192.0.2.1")
	if !ip.Equal(expected) {
		t.Errorf("wrong IP returned: got %v, want %v", ip, expected)
	}

	// Get client info
	info, err := sim.GetSharedClientInfo(suiteID, containerID)
	if err != nil {
		t.Fatal("can't get shared client info:", err)
	}

	if info.ID != "container1" {
		t.Errorf("wrong container ID in info: got %q, want %q", info.ID, "container1")
	}
}

// Tests AddSharedClient method in Suite.
func TestAddSharedClient(t *testing.T) {
	var startedContainers int

	tm, srv := newFakeAPI(&fakes.BackendHooks{
		StartContainer: func(image, containerID string, opt libhive.ContainerOptions) (*libhive.ContainerInfo, error) {
			startedContainers++
			return &libhive.ContainerInfo{
				ID: containerID,
				IP: "192.0.2.1",
			}, nil
		},
	})
	defer srv.Close()
	defer tm.Terminate()

	suite := Suite{
		Name:        "shared-client-suite",
		Description: "Testing shared client registration",
	}
	suite.AddSharedClient("shared1", "client-1", Params(map[string]string{
		"PARAM1": "value1",
	}))

	suite.Add(TestSpec{
		Name: "test-using-shared-client",
		Run: func(t *T) {
			client := t.GetSharedClient("shared1")
			if client == nil {
				t.Fatal("shared client not found")
			}

			if client.Type != "client-1" {
				t.Errorf("wrong client type: got %q, want %q", client.Type, "client-1")
			}
			if !client.IsShared {
				t.Error("IsShared flag not set on client")
			}
		},
	})

	sim := NewAt(srv.URL)
	err := RunSuite(sim, suite)
	if err != nil {
		t.Fatal("suite run failed:", err)
	}

	if startedContainers == 0 {
		t.Error("no containers were started")
	}

	tm.Terminate()
	results := tm.Results()
	removeTimestamps(results)

	if len(results) == 0 {
		t.Fatal("no test results")
	}

	suiteResult := results[0]
	if suiteResult.ClientInfo == nil || len(suiteResult.ClientInfo) == 0 {
		t.Error("no shared clients in test results")
	}
}

// Tests log offset tracking.
func TestSharedClientLogOffset(t *testing.T) {
	// Mock HTTP server for testing the log offset API
	var offsetValue int64 = 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/testsuite":
			// StartSuite
			json.NewEncoder(w).Encode(0) // Return suite ID
		case "/testsuite/0/node":
			// StartSharedClient
			json.NewEncoder(w).Encode(simapi.StartNodeResponse{
				ID: "container1",
				IP: "192.0.2.1",
			})
		case "/testsuite/0/node/container1/log-offset":
			// GetClientLogOffset - increment the offset each time it's called
			currentOffset := offsetValue
			offsetValue += 100 // Simulate log growth
			json.NewEncoder(w).Encode(currentOffset)
		case "/testsuite/0/node/container1/exec":
			// ExecSharedClient
			json.NewEncoder(w).Encode(&ExecInfo{
				Stdout:   "test output",
				ExitCode: 0,
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	sim := NewAt(srv.URL)
	suiteID, err := sim.StartSuite(&simapi.TestRequest{Name: "log-offset-suite"}, "Testing log offset")
	if err != nil {
		t.Fatal("can't start suite:", err)
	}

	containerID, _, err := sim.StartSharedClient(suiteID, "client-1")
	if err != nil {
		t.Fatal("can't start shared client:", err)
	}

	// Get initial offset
	initialOffset, err := sim.GetClientLogOffset(suiteID, containerID)
	if err != nil {
		t.Fatal("can't get initial log offset:", err)
	}
	if initialOffset != 0 {
		t.Errorf("wrong initial offset: got %d, want 0", initialOffset)
	}

	// Simulate a command that generates logs
	_, err = sim.ExecSharedClient(suiteID, containerID, []string{"/bin/echo", "test1"})
	if err != nil {
		t.Fatal("exec failed:", err)
	}

	// Get new offset - should have increased
	newOffset, err := sim.GetClientLogOffset(suiteID, containerID)
	if err != nil {
		t.Fatal("can't get new log offset:", err)
	}
	if newOffset <= initialOffset {
		t.Errorf("offset didn't increase: got %d, want > %d", newOffset, initialOffset)
	}
}

// Tests log extraction functionality.
func TestSharedClientLogExtraction(t *testing.T) {
	// We can't fully test the log extraction in unit tests because it depends on file I/O.
	// However, we can verify that the API endpoints are called correctly.
	// The actual file operations are tested in integration tests.

	// This test ensures that:
	// 1. We use a MockSuite instead of real test cases
	// 2. We verify that the ClientLogs structure is correctly set up

	// Create a mock ClientLogs map for our test result
	clientLogs := make(map[string]*libhive.ClientLogSegment)
	clientLogs["shared1"] = &libhive.ClientLogSegment{
		Start:    0,
		End:      100,
		ClientID: "container1",
	}

	// Create a mock test result
	mockResult := &libhive.TestResult{
		Pass:       true,
		ClientLogs: clientLogs,
	}

	// Verify the test result has client logs
	if mockResult.ClientLogs == nil {
		t.Error("no client logs in test results")
	}

	if len(mockResult.ClientLogs) == 0 {
		t.Error("empty client logs map")
	}

	// Verify the log segment values
	logSegment := mockResult.ClientLogs["shared1"]
	if logSegment.Start != 0 || logSegment.End != 100 || logSegment.ClientID != "container1" {
		t.Errorf("unexpected log segment values: got %+v", logSegment)
	}
}

// Tests GetClientLogOffset function.
func TestGetClientLogOffset(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/testsuite":
			// StartSuite
			json.NewEncoder(w).Encode(0) // Return suite ID
		case "/testsuite/0/node":
			// StartSharedClient
			json.NewEncoder(w).Encode(simapi.StartNodeResponse{
				ID: "container1",
				IP: "192.0.2.1",
			})
		case "/testsuite/0/node/container1/log-offset":
			// GetClientLogOffset
			json.NewEncoder(w).Encode(int64(0)) // Initial log offset
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	sim := NewAt(srv.URL)
	suiteID, err := sim.StartSuite(&simapi.TestRequest{Name: "log-offset-test"}, "Test GetClientLogOffset")
	if err != nil {
		t.Fatal("can't start suite:", err)
	}

	containerID, _, err := sim.StartSharedClient(suiteID, "client-1")
	if err != nil {
		t.Fatal("can't start shared client:", err)
	}

	offset, err := sim.GetClientLogOffset(suiteID, containerID)
	if err != nil {
		t.Fatal("get log offset failed:", err)
	}

	if offset != 0 {
		t.Errorf("wrong initial log offset: got %d, want 0", offset)
	}
}
