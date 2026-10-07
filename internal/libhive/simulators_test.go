package libhive

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseSimulatorListYAML(t *testing.T) {
	inv := Inventory{BaseDir: t.TempDir()}
	inv.AddSimulator("test")
	dir := inv.SimulatorDirectory("test")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "Dockerfile.git"), []byte("FROM scratch"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "Dockerfile.dir"), 0755); err != nil {
		t.Fatal(err)
	}
	tests := []struct{ name, yaml, wantErr string }{
		{"default", "- simulator: test", ""},
		{"git", "- simulator: test\n  dockerfile: git\n  build_args:\n    branch: main", ""},
		{"unknown", "- simulator: missing", "unknown simulator"},
		{"missing name", "- dockerfile: git", "unknown simulator"},
		{"unknown field", "- simulator: test\n  typo: value", "field typo"},
		{"duplicate", "- simulator: test\n- simulator: test", "duplicate simulator"},
		{"missing dockerfile", "- simulator: test\n  dockerfile: missing", "doesn't have Dockerfile.missing"},
		{"directory dockerfile", "- simulator: test\n  dockerfile: dir", "not a regular file"},
		{"path", "- simulator: test\n  dockerfile: ../git", "invalid Dockerfile extension"},
		{"empty", "[]", "simulator list is empty"},
		{"comment only", "# no simulators\n", "simulator list is empty"},
		{"malformed", "[", "unable to parse"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			list, err := ParseSimulatorListYAML(&inv, strings.NewReader(tt.yaml))
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("error = %v, want %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if len(list) != 1 || list[0].Simulator != "test" {
				t.Fatalf("unexpected list: %+v", list)
			}
			if tt.name == "git" && (list[0].Dockerfile() != "Dockerfile.git" || list[0].BuildArgs["branch"] != "main") {
				t.Fatalf("lost build parameters: %+v", list[0])
			}
			if tt.name == "default" && list[0].Dockerfile() != "Dockerfile" {
				t.Fatal("wrong default Dockerfile")
			}
		})
	}
}
