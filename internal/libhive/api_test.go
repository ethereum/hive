package libhive_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/ethereum/hive/internal/fakes"
	"github.com/ethereum/hive/internal/libhive"
	"github.com/ethereum/hive/internal/simapi"
)

func TestStartClientFailureRemovesContainer(t *testing.T) {
	tests := []struct {
		name       string
		config     simapi.NodeConfig
		wantStatus int
	}{
		{
			name:       "network connect error",
			config:     simapi.NodeConfig{Client: "test-client", Networks: []string{"testnet"}},
			wantStatus: http.StatusInternalServerError,
		},
		{
			name: "invalid check-live port",
			config: simapi.NodeConfig{
				Client:      "test-client",
				Environment: map[string]string{"HIVE_CHECK_LIVE_PORT": "not-a-port"},
			},
			wantStatus: http.StatusBadRequest,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var (
				mu         sync.Mutex
				containers = make(map[string]bool)
			)
			backend := fakes.NewContainerBackend(&fakes.BackendHooks{
				CreateContainer: func(string, libhive.ContainerOptions) (string, error) {
					mu.Lock()
					defer mu.Unlock()
					id := fmt.Sprintf("%08x", len(containers)+1)
					containers[id] = true
					return id, nil
				},
				DeleteContainer: func(id string) error {
					mu.Lock()
					defer mu.Unlock()
					delete(containers, id)
					return nil
				},
				ConnectContainer: func(string, string) error {
					return errors.New("test connect failure")
				},
			})

			clients := []*libhive.ClientDefinition{{Name: "test-client", Image: "test-client-image"}}
			tm := libhive.NewTestManager(libhive.SimEnv{}, backend, clients, libhive.HiveInfo{})
			srv := httptest.NewServer(tm.API())
			defer srv.Close()

			suiteID, err := tm.StartTestSuite("test-suite", "test suite description")
			if err != nil {
				t.Fatal("StartTestSuite:", err)
			}
			if err := tm.CreateNetwork(suiteID, "testnet"); err != nil {
				t.Fatal("CreateNetwork:", err)
			}
			testID, err := tm.StartTest(suiteID, "test", "test description")
			if err != nil {
				t.Fatal("StartTest:", err)
			}

			if status := postStartClient(t, srv.URL, suiteID, testID, test.config); status != test.wantStatus {
				t.Fatalf("startClient returned status %d, want %d", status, test.wantStatus)
			}
			if err := tm.EndTest(suiteID, testID, &libhive.TestResult{Pass: true}); err != nil {
				t.Fatal("EndTest:", err)
			}

			mu.Lock()
			defer mu.Unlock()
			for id := range containers {
				t.Errorf("container %s was not removed", id)
			}
		})
	}
}

// postStartClient sends a client start request via the HTTP API and returns the response status.
func postStartClient(t *testing.T, baseURL string, suiteID libhive.TestSuiteID, testID libhive.TestID, nodeConfig simapi.NodeConfig) int {
	t.Helper()

	config, err := json.Marshal(nodeConfig)
	if err != nil {
		t.Fatal("marshal NodeConfig:", err)
	}

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	writer.WriteField("config", string(config))
	writer.Close()

	url := fmt.Sprintf("%s/testsuite/%d/test/%d/node", baseURL, suiteID, testID)
	resp, err := http.Post(url, writer.FormDataContentType(), &body)
	if err != nil {
		t.Fatal("POST startClient:", err)
	}
	resp.Body.Close()
	return resp.StatusCode
}
