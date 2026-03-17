package libhive_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/ethereum/hive/internal/fakes"
	"github.com/ethereum/hive/internal/libhive"
	"github.com/ethereum/hive/internal/simapi"
)

func TestRegisterMultiTestNode(t *testing.T) {
	// Track container deletions to verify lifecycle behavior.
	var deletedContainers []string
	backend := fakes.NewContainerBackend(&fakes.BackendHooks{
		DeleteContainer: func(containerID string) error {
			deletedContainers = append(deletedContainers, containerID)
			return nil
		},
	})

	logDir := t.TempDir()
	clients := []*libhive.ClientDefinition{{Name: "test-client", Image: "test-client-image"}}
	tm := libhive.NewTestManager(libhive.SimEnv{LogDir: logDir}, backend, clients, libhive.HiveInfo{})
	handler := tm.API()
	srv := httptest.NewServer(handler)
	defer srv.Close()

	// Create suite and tests via exported Go methods.
	suiteID, err := tm.StartTestSuite("test-suite", "test suite description")
	if err != nil {
		t.Fatal("StartTestSuite:", err)
	}
	sourceTestID, err := tm.StartTest(suiteID, "source-test", "source test")
	if err != nil {
		t.Fatal("StartTest (source):", err)
	}
	targetTestID, err := tm.StartTest(suiteID, "target-test", "target test")
	if err != nil {
		t.Fatal("StartTest (target):", err)
	}

	// Create the client log directory (file is written after we know the container ID).
	clientLogDir := filepath.Join(logDir, "test-client")
	if err := os.MkdirAll(clientLogDir, 0755); err != nil {
		t.Fatal("MkdirAll:", err)
	}

	// Start a client on the source test via HTTP (this sets wait via the backend).
	containerID := startClientHTTP(t, srv.URL, suiteID, sourceTestID)

	// Write initial log content now that we know the container ID.
	logFilePath := filepath.Join(clientLogDir, "client-"+containerID+".log")
	initialContent := []byte("line 1\nline 2\nline 3\n")
	if err := os.WriteFile(logFilePath, initialContent, 0644); err != nil {
		t.Fatal("WriteFile:", err)
	}

	// Register multi-test node on target test via HTTP.
	registerMultiTestNodeHTTP(t, srv.URL, suiteID, sourceTestID, containerID, targetTestID)

	// Append more content to the log (simulates client activity during target test).
	additionalContent := []byte("line 4\nline 5\n")
	f, err := os.OpenFile(logFilePath, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		t.Fatal("OpenFile:", err)
	}
	f.Write(additionalContent)
	f.Close()

	// End target test — container should NOT be deleted (wait is nil for multi-test copy).
	targetResult := &libhive.TestResult{Pass: true, Details: "target test passed"}
	if err := tm.EndTest(suiteID, targetTestID, targetResult); err != nil {
		t.Fatal("EndTest (target):", err)
	}
	if len(deletedContainers) != 0 {
		t.Fatalf("Expected no container deletions after ending target test, got %v", deletedContainers)
	}

	// End source test — container SHOULD be deleted (wait is non-nil, source owns lifecycle).
	sourceResult := &libhive.TestResult{Pass: true, Details: "source test passed"}
	if err := tm.EndTest(suiteID, sourceTestID, sourceResult); err != nil {
		t.Fatal("EndTest (source):", err)
	}
	if len(deletedContainers) != 1 || deletedContainers[0] != containerID {
		t.Fatalf("Expected container %q to be deleted, got %v", containerID, deletedContainers)
	}

	// End the suite so results are available.
	if err := tm.EndTestSuite(suiteID); err != nil {
		t.Fatal("EndTestSuite:", err)
	}

	// Verify log offsets via Results().
	results := tm.Results()
	suite, ok := results[suiteID]
	if !ok {
		t.Fatal("Suite not found in results")
	}

	// Check target test's client log offsets.
	targetCase := suite.TestCases[targetTestID]
	if targetCase == nil {
		t.Fatal("Target test case not found in results")
	}
	targetClient := targetCase.ClientInfo[containerID]
	if targetClient == nil {
		t.Fatal("Target client info not found")
	}
	if targetClient.LogOffsets.Begin != int64(len(initialContent)) {
		t.Fatalf("Target client LogOffsets.Begin = %d, want %d",
			targetClient.LogOffsets.Begin, len(initialContent))
	}
	expectedEnd := int64(len(initialContent) + len(additionalContent))
	if targetClient.LogOffsets.End != expectedEnd {
		t.Fatalf("Target client LogOffsets.End = %d, want %d",
			targetClient.LogOffsets.End, expectedEnd)
	}

	// Check source test's client log offsets.
	sourceCase := suite.TestCases[sourceTestID]
	if sourceCase == nil {
		t.Fatal("Source test case not found in results")
	}
	sourceClient := sourceCase.ClientInfo[containerID]
	if sourceClient == nil {
		t.Fatal("Source client info not found")
	}
	if sourceClient.LogOffsets.Begin != 0 {
		t.Fatalf("Source client LogOffsets.Begin = %d, want 0", sourceClient.LogOffsets.Begin)
	}
	if sourceClient.LogOffsets.End != expectedEnd {
		t.Fatalf("Source client LogOffsets.End = %d, want %d",
			sourceClient.LogOffsets.End, expectedEnd)
	}
}

// startClientHTTP starts a client on the given test via the HTTP API and returns the container ID.
func startClientHTTP(t *testing.T, baseURL string, suiteID libhive.TestSuiteID, testID libhive.TestID) string {
	t.Helper()

	config, err := json.Marshal(simapi.NodeConfig{Client: "test-client"})
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
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("startClient returned status %d", resp.StatusCode)
	}

	var nodeResp simapi.StartNodeResponse
	if err := json.NewDecoder(resp.Body).Decode(&nodeResp); err != nil {
		t.Fatal("decode StartNodeResponse:", err)
	}
	if nodeResp.ID == "" {
		t.Fatal("startClient returned empty container ID")
	}
	return nodeResp.ID
}

// registerMultiTestNodeHTTP registers an existing node with a target test via the HTTP API.
func registerMultiTestNodeHTTP(t *testing.T, baseURL string, suiteID libhive.TestSuiteID, sourceTestID libhive.TestID, nodeID string, targetTestID libhive.TestID) {
	t.Helper()

	url := fmt.Sprintf("%s/testsuite/%d/test/%d/node/%s/register/%d",
		baseURL, suiteID, sourceTestID, nodeID, targetTestID)
	resp, err := http.Post(url, "", nil)
	if err != nil {
		t.Fatal("POST registerMultiTestNode:", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("registerMultiTestNode returned status %d", resp.StatusCode)
	}
}
