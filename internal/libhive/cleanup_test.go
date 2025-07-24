package libhive

import (
	"testing"
	"time"
)

func TestCleanupOptions(t *testing.T) {
	opts := CleanupOptions{
		InstanceID:    "test-instance",
		OlderThan:     time.Hour,
		DryRun:        true,
		ContainerType: ContainerTypeClient,
	}

	// Verify options are set correctly
	if opts.InstanceID != "test-instance" {
		t.Errorf("Expected InstanceID 'test-instance', got %s", opts.InstanceID)
	}

	if opts.OlderThan != time.Hour {
		t.Errorf("Expected OlderThan 1h, got %v", opts.OlderThan)
	}

	if !opts.DryRun {
		t.Error("Expected DryRun to be true")
	}

	if opts.ContainerType != ContainerTypeClient {
		t.Errorf("Expected ContainerType %s, got %s", ContainerTypeClient, opts.ContainerType)
	}
}

func TestCleanupOptionsDefaults(t *testing.T) {
	opts := CleanupOptions{}

	// Verify default values
	if opts.InstanceID != "" {
		t.Errorf("Expected empty InstanceID, got %s", opts.InstanceID)
	}

	if opts.OlderThan != 0 {
		t.Errorf("Expected OlderThan 0, got %v", opts.OlderThan)
	}

	if opts.DryRun {
		t.Error("Expected DryRun to be false by default")
	}

	if opts.ContainerType != "" {
		t.Errorf("Expected empty ContainerType, got %s", opts.ContainerType)
	}
}