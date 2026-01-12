package libhive

import (
	"context"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"
)

// noopBackend is a minimal ContainerBackend for testing.
type noopBackend struct{}

func (noopBackend) Build(context.Context, Builder) error                      { return nil }
func (noopBackend) SetHiveInstanceInfo(string, string)                        {}
func (noopBackend) GetDockerClient() interface{}                              { return nil }
func (noopBackend) ServeAPI(context.Context, http.Handler) (APIServer, error) { return nil, nil }
func (noopBackend) CreateContainer(context.Context, string, ContainerOptions) (string, error) {
	return "", nil
}
func (noopBackend) StartContainer(context.Context, string, ContainerOptions) (*ContainerInfo, error) {
	return nil, nil
}
func (noopBackend) DeleteContainer(string) error  { return nil }
func (noopBackend) PauseContainer(string) error   { return nil }
func (noopBackend) UnpauseContainer(string) error { return nil }
func (noopBackend) RunProgram(context.Context, string, []string) (*ExecInfo, error) {
	return nil, nil
}
func (noopBackend) NetworkNameToID(string) (string, error)     { return "", nil }
func (noopBackend) CreateNetwork(string) (string, error)       { return "", nil }
func (noopBackend) RemoveNetwork(string) error                 { return nil }
func (noopBackend) ContainerIP(string, string) (net.IP, error) { return nil, nil }
func (noopBackend) ConnectContainer(string, string) error      { return nil }
func (noopBackend) DisconnectContainer(string, string) error   { return nil }

func TestRegisterMultiTestNode(t *testing.T) {
	// Set up a TestManager with a temporary log directory.
	logDir := t.TempDir()
	config := SimEnv{LogDir: logDir}
	tm := &TestManager{
		config:            config,
		backend:           noopBackend{},
		runningTestSuites: make(map[TestSuiteID]*TestSuite),
		runningTestCases:  make(map[TestID]*TestCase),
		results:           make(map[TestSuiteID]*TestSuite),
		networks:          make(map[TestSuiteID]map[string]string),
	}

	// Create a test suite.
	suiteID, err := tm.StartTestSuite("test-suite", "test suite description")
	if err != nil {
		t.Fatal("StartTestSuite:", err)
	}

	// Create source and target tests.
	sourceTestID, err := tm.StartTest(suiteID, "source-test", "source test")
	if err != nil {
		t.Fatal("StartTest (source):", err)
	}
	targetTestID, err := tm.StartTest(suiteID, "target-test", "target test")
	if err != nil {
		t.Fatal("StartTest (target):", err)
	}

	// Create a fake log file with some content.
	clientLogDir := filepath.Join(logDir, "test-client")
	if err := os.MkdirAll(clientLogDir, 0755); err != nil {
		t.Fatal("MkdirAll:", err)
	}
	logFilePath := filepath.Join(clientLogDir, "client-abc123.log")
	initialContent := []byte("line 1\nline 2\nline 3\n")
	if err := os.WriteFile(logFilePath, initialContent, 0644); err != nil {
		t.Fatal("WriteFile:", err)
	}

	// Register a client with the source test (simulates startClient).
	logPath := "test-client/client-abc123.log"
	sourceClient := &ClientInfo{
		ID:         "abc123",
		IP:         "192.168.1.1",
		Name:       "test-client",
		LogFile:    logPath,
		LogOffsets: &TestLogOffsets{Begin: 0},
		wait:       func() {}, // non-nil: source test owns the lifecycle
	}
	if err := tm.RegisterNode(sourceTestID, "abc123", sourceClient); err != nil {
		t.Fatal("RegisterNode (source):", err)
	}

	// Simulate registerMultiTestNode: register client with target test (wait=nil).
	currentLogSize := logFileSize(logFilePath)
	multiTestClient := &ClientInfo{
		ID:         "abc123",
		IP:         "192.168.1.1",
		Name:       "test-client",
		LogFile:    logPath,
		LogOffsets: &TestLogOffsets{Begin: currentLogSize},
		// wait is nil: target test should NOT stop the container
	}
	if err := tm.RegisterNode(targetTestID, "abc123", multiTestClient); err != nil {
		t.Fatal("RegisterNode (target):", err)
	}

	// Append more content to the log (simulates client activity during target test).
	additionalContent := []byte("line 4\nline 5\n")
	f, err := os.OpenFile(logFilePath, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		t.Fatal("OpenFile:", err)
	}
	f.Write(additionalContent)
	f.Close()

	// End the target test — client should NOT be stopped (wait=nil).
	targetResult := &TestResult{Pass: true, Details: "target test passed"}
	if err := tm.EndTest(suiteID, targetTestID, targetResult); err != nil {
		t.Fatal("EndTest (target):", err)
	}

	// Verify: target test is no longer running.
	if _, running := tm.IsTestRunning(targetTestID); running {
		t.Fatal("Target test should no longer be running")
	}

	// Verify: the source test's client should still be running.
	sourceCase, running := tm.IsTestRunning(sourceTestID)
	if !running {
		t.Fatal("Source test should still be running")
	}
	sourceClientInfo := sourceCase.ClientInfo["abc123"]
	if sourceClientInfo == nil {
		t.Fatal("Source client info should exist")
	}
	if sourceClientInfo.wait == nil {
		t.Fatal("Source client's wait should NOT be nil (it owns lifecycle)")
	}

	// Verify: log byte offsets were set correctly on the multi-test client.
	if multiTestClient.LogOffsets.Begin != int64(len(initialContent)) {
		t.Fatalf("Multi-test client LogOffsets.Begin = %d, want %d",
			multiTestClient.LogOffsets.Begin, len(initialContent))
	}
	expectedEnd := int64(len(initialContent) + len(additionalContent))
	if multiTestClient.LogOffsets.End != expectedEnd {
		t.Fatalf("Multi-test client LogOffsets.End = %d, want %d",
			multiTestClient.LogOffsets.End, expectedEnd)
	}

	// End the source test — client SHOULD be stopped (wait != nil).
	sourceResult := &TestResult{Pass: true, Details: "source test passed"}
	if err := tm.EndTest(suiteID, sourceTestID, sourceResult); err != nil {
		t.Fatal("EndTest (source):", err)
	}

	// Verify: source client LogOffsets.End was also captured.
	if sourceClient.LogOffsets.End != expectedEnd {
		t.Fatalf("Source client LogOffsets.End = %d, want %d",
			sourceClient.LogOffsets.End, expectedEnd)
	}
}

func TestLogFileSize(t *testing.T) {
	// Non-existent file should return 0.
	if got := logFileSize("/nonexistent/path/file.log"); got != 0 {
		t.Fatalf("logFileSize(nonexistent) = %d, want 0", got)
	}

	// Create a temp file with known content.
	tmpFile := filepath.Join(t.TempDir(), "test.log")
	content := []byte("hello world\n")
	if err := os.WriteFile(tmpFile, content, 0644); err != nil {
		t.Fatal("WriteFile:", err)
	}

	if got := logFileSize(tmpFile); got != int64(len(content)) {
		t.Fatalf("logFileSize = %d, want %d", got, len(content))
	}
}
