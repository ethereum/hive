package libhive

import (
	"os/exec"
	"runtime/debug"
	"strings"
)

var (
	// Build-time variables set by ldflags during compilation
	hiveCommit string
	buildDate  string
)

// VersionInfo contains comprehensive git version information
type VersionInfo struct {
	Commit      string    `json:"commit"`
	CommitDate  string    `json:"commitDate,omitempty"`
	BuildDate   string    `json:"buildDate,omitempty"`
	Branch      string    `json:"branch,omitempty"`
	Dirty       bool      `json:"dirty,omitempty"`
}

// ClientConfigInfo contains client configuration details
type ClientConfigInfo struct {
	FilePath string                 `json:"filePath"`
	Content  map[string]interface{} `json:"content"`
}

// RunMetadata contains comprehensive metadata about a test run
type RunMetadata struct {
	HiveCommand    []string          `json:"hiveCommand"`    // Full command with args
	HiveVersion    VersionInfo       `json:"hiveVersion"`    // Hive git info
	ClientConfig   *ClientConfigInfo `json:"clientConfig,omitempty"` // Client config file
}

// GetHiveVersion returns the current Hive version information
func GetHiveVersion() VersionInfo {
	info := VersionInfo{
		Commit:    hiveCommit,
		BuildDate: buildDate,
	}
	
	// Fall back to runtime detection if build-time info not available
	if info.Commit == "" {
		if buildInfo, ok := debug.ReadBuildInfo(); ok {
			for _, v := range buildInfo.Settings {
				switch v.Key {
				case "vcs.revision":
					info.Commit = v.Value
				case "vcs.time":
					info.CommitDate = v.Value
				}
			}
		}
	}
	
	// Try to get additional git info at runtime (only if git available)
	if output, err := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD").Output(); err == nil {
		info.Branch = strings.TrimSpace(string(output))
	}
	
	// Check if working directory is dirty
	if err := exec.Command("git", "diff", "--quiet").Run(); err != nil {
		info.Dirty = true
	}
	
	return info
}

