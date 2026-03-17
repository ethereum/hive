package libhive

import (
	"os"
	"path/filepath"
	"testing"
)

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
