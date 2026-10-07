package libhive

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestParseConfigYAML(t *testing.T) {
	inv := Inventory{BaseDir: t.TempDir()}
	inv.AddClient("go-ethereum", &InventoryClient{Dockerfiles: []string{"git"}})
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
	tests := []struct {
		name, yaml, wantErr string
		want                Config
	}{
		{
			name: "simulator",
			yaml: "- simulator: test",
			want: Config{Simulators: []SimulatorDesignator{{Simulator: "test"}}},
		},
		{
			name: "simulator git",
			yaml: "- simulator: test\n  dockerfile: git\n  build_args:\n    branch: main",
			want: Config{Simulators: []SimulatorDesignator{
				{Simulator: "test", DockerfileExt: "git", BuildArgs: map[string]string{"branch": "main"}},
			}},
		},
		{
			name: "client",
			yaml: "- client: go-ethereum\n  dockerfile: git",
			want: Config{Clients: []ClientDesignator{{Client: "go-ethereum", DockerfileExt: "git"}}},
		},
		{
			name: "mixed",
			yaml: "- simulator: test\n- client: go-ethereum\n  nametag: main\n  build_args:\n    tag: main",
			want: Config{
				Clients:    []ClientDesignator{{Client: "go-ethereum", Nametag: "main", BuildArgs: map[string]string{"tag": "main"}}},
				Simulators: []SimulatorDesignator{{Simulator: "test"}},
			},
		},
		{name: "unknown simulator", yaml: "- simulator: missing", wantErr: "unknown simulator"},
		{name: "unknown client", yaml: "- client: missing", wantErr: "unknown client"},
		{name: "missing kind", yaml: "- dockerfile: git", wantErr: "entry 1: missing client or simulator"},
		{name: "both kinds", yaml: "- simulator: test\n- client: go-ethereum\n  simulator: test", wantErr: "entry 2: client and simulator are mutually exclusive"},
		{name: "simulator nametag", yaml: "- simulator: test\n  nametag: x", wantErr: "doesn't support nametag"},
		{name: "unknown field", yaml: "- simulator: test\n  typo: value", wantErr: "field typo"},
		{name: "duplicate simulator", yaml: "- simulator: test\n- simulator: test", wantErr: "duplicate simulator"},
		{name: "missing dockerfile", yaml: "- simulator: test\n  dockerfile: missing", wantErr: "doesn't have Dockerfile.missing"},
		{name: "directory dockerfile", yaml: "- simulator: test\n  dockerfile: dir", wantErr: "not a regular file"},
		{name: "path", yaml: "- simulator: test\n  dockerfile: ../git", wantErr: "invalid Dockerfile extension"},
		{name: "empty", yaml: "[]", wantErr: "config file is empty"},
		{name: "comment only", yaml: "# no entries\n", wantErr: "config file is empty"},
		{name: "sections", yaml: "clients: []\nsimulators: []", wantErr: "unable to parse"},
		{name: "malformed", yaml: "[", wantErr: "unable to parse"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := ParseConfigYAML(&inv, strings.NewReader(tt.yaml))
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("error = %v, want %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(*cfg, tt.want) {
				t.Fatalf("got %+v, want %+v", *cfg, tt.want)
			}
		})
	}
}
