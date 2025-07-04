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
}

// Tests suites that use shared clients.
func TestSharedClientSuite(t *testing.T) {
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

	sim := NewAt(srv.URL)

	suiteWithSharedClients := Suite{
		Name:        "shared-client-suite",
		Description: "Testing shared client registration",
	}
	suiteWithSharedClients.SharedClientsOptions = func(clientDefinition *ClientDefinition) []StartOption {
		return []StartOption{
			Params(map[string]string{
				"PARAM1": "value1",
			}),
		}
	}

	sharedClientContainerID := ""
	suiteWithSharedClients.Add(TestSpec{
		Name: "test-using-shared-client",
		Run: func(t *T) {
			sharedClientIDs := t.GetSharedClientIDs()
			if len(sharedClientIDs) != 1 {
				t.Fatal("wrong number of shared clients:", len(sharedClientIDs))
			}
			sharedClientContainerID = sharedClientIDs[0]
			client := t.GetSharedClient(sharedClientContainerID)
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
	suiteWithSharedClients.Add(TestSpec{
		Name: "another-test-using-shared-client",
		Run: func(t *T) {
			sharedClientIDs := t.GetSharedClientIDs()
			if len(sharedClientIDs) != 1 {
				t.Fatal("wrong number of shared clients:", len(sharedClientIDs))
			}
			if sharedClientIDs[0] != sharedClientContainerID {
				t.Fatal("wrong shared client container ID:", sharedClientIDs[0], "!=", sharedClientContainerID)
			}
			client := t.GetSharedClient(sharedClientContainerID)
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

	err := RunSuite(sim, suiteWithSharedClients)
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
