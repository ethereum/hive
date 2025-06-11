package libhive

import (
	"strings"
	"testing"
	"time"
)

func TestGenerateHiveInstanceID(t *testing.T) {
	id1 := GenerateHiveInstanceID()
	
	// Sleep briefly to ensure different timestamp
	time.Sleep(time.Millisecond)
	id2 := GenerateHiveInstanceID()

	if id1 == id2 {
		t.Error("GenerateHiveInstanceID should generate unique IDs")
	}

	if len(id1) == 0 {
		t.Error("GenerateHiveInstanceID should not return empty string")
	}

	// Should start with "hive-"
	if len(id1) < 5 || id1[:5] != "hive-" {
		t.Errorf("GenerateHiveInstanceID should start with 'hive-', got: %s", id1)
	}
}

func TestNewBaseLabels(t *testing.T) {
	instanceID := "test-instance-123"
	version := "commit-abc123"

	labels := NewBaseLabels(instanceID, version)

	// Check that all required base labels are present
	requiredLabels := []string{LabelHiveInstance, LabelHiveVersion, LabelHiveCreated}
	for _, label := range requiredLabels {
		if _, exists := labels[label]; !exists {
			t.Errorf("NewBaseLabels should include label %s", label)
		}
	}

	// Check values
	if labels[LabelHiveInstance] != instanceID {
		t.Errorf("Expected instance ID %s, got %s", instanceID, labels[LabelHiveInstance])
	}

	if labels[LabelHiveVersion] != version {
		t.Errorf("Expected version %s, got %s", version, labels[LabelHiveVersion])
	}

	// Check that created timestamp is valid RFC3339
	createdStr := labels[LabelHiveCreated]
	_, err := time.Parse(time.RFC3339, createdStr)
	if err != nil {
		t.Errorf("Created timestamp should be valid RFC3339 format: %v", err)
	}
}

func TestContainerTypeConstants(t *testing.T) {
	// Verify container type constants have expected values
	expectedTypes := map[string]string{
		ContainerTypeClient:    "client",
		ContainerTypeSimulator: "simulator",
		ContainerTypeProxy:     "proxy",
	}

	for constant, expected := range expectedTypes {
		if constant != expected {
			t.Errorf("Expected %s to equal %s", constant, expected)
		}
	}
}

func TestLabelConstants(t *testing.T) {
	// Verify all label constants have the expected hive prefix
	labels := []string{
		LabelHiveInstance,
		LabelHiveVersion,
		LabelHiveType,
		LabelHiveTestSuite,
		LabelHiveTestCase,
		LabelHiveClientName,
		LabelHiveClientImage,
		LabelHiveCreated,
		LabelHiveSimulator,
	}

	for _, label := range labels {
		if len(label) < 5 || label[:5] != "hive." {
			t.Errorf("Label %s should start with 'hive.' prefix", label)
		}
	}
}

func TestGenerateContainerName(t *testing.T) {
	name1 := GenerateContainerName("client", "go-ethereum")
	name2 := GenerateContainerName("simulator", "devp2p")
	name3 := GenerateContainerName("proxy", "")

	// All names should start with "hive-"
	names := []string{name1, name2, name3}
	for _, name := range names {
		if len(name) < 5 || name[:5] != "hive-" {
			t.Errorf("Container name %s should start with 'hive-'", name)
		}
	}

	// Names should be different
	if name1 == name2 {
		t.Error("Different container types should generate different names")
	}

	// Proxy name should not have identifier
	if len(name3) < len("hive-proxy-") {
		t.Errorf("Proxy name too short: %s", name3)
	}
}

func TestGenerateClientContainerName(t *testing.T) {
	name := GenerateClientContainerName("go-ethereum", TestSuiteID(1), TestID(2))
	
	if len(name) < 5 || name[:5] != "hive-" {
		t.Errorf("Client container name should start with 'hive-', got: %s", name)
	}

	// Should contain client name and IDs
	if !strings.Contains(name, "client") {
		t.Errorf("Client container name should contain 'client', got: %s", name)
	}
	if !strings.Contains(name, "go-ethereum") {
		t.Errorf("Client container name should contain client name, got: %s", name)
	}
}

func TestGenerateSimulatorContainerName(t *testing.T) {
	name := GenerateSimulatorContainerName("devp2p")
	
	if len(name) < 5 || name[:5] != "hive-" {
		t.Errorf("Simulator container name should start with 'hive-', got: %s", name)
	}

	if !strings.Contains(name, "simulator") {
		t.Errorf("Simulator container name should contain 'simulator', got: %s", name)
	}
	if !strings.Contains(name, "devp2p") {
		t.Errorf("Simulator container name should contain simulator name, got: %s", name)
	}
}

func TestGenerateProxyContainerName(t *testing.T) {
	name := GenerateProxyContainerName()
	
	if len(name) < 5 || name[:5] != "hive-" {
		t.Errorf("Proxy container name should start with 'hive-', got: %s", name)
	}

	if !strings.Contains(name, "proxy") {
		t.Errorf("Proxy container name should contain 'proxy', got: %s", name)
	}
}

func TestSanitizeContainerNameComponent(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"ethereum/rpc-compat", "ethereum-rpc-compat"},
		{"go-ethereum", "go-ethereum"},
		{"test_123", "test_123"},
		{"test.version", "test.version"},
		{"/invalid", "invalid"},
		{"_invalid", "invalid"},
		{".invalid", "invalid"},
		{"", ""},
		{"123valid", "123valid"},
		{"special@chars#", "special-chars-"},
		{"a", "a"},
		{"_", "container"},
	}

	for _, test := range tests {
		result := SanitizeContainerNameComponent(test.input)
		if result != test.expected {
			t.Errorf("SanitizeContainerNameComponent(%q) = %q, expected %q", test.input, result, test.expected)
		}
	}
}

func TestSanitizedContainerNames(t *testing.T) {
	// Test that names with slashes get properly sanitized
	name := GenerateSimulatorContainerName("ethereum/rpc-compat")
	
	if strings.Contains(name, "/") {
		t.Errorf("Container name should not contain '/', got: %s", name)
	}
	
	if !strings.Contains(name, "ethereum-rpc-compat") {
		t.Errorf("Container name should contain sanitized simulator name, got: %s", name)
	}
}