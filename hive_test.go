package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/ethereum/hive/internal/libhive"
)

func TestSimulatorList(t *testing.T) {
	var inv libhive.Inventory
	inv.AddSimulator("ethereum/one")
	inv.AddSimulator("ethereum/two")
	file := filepath.Join(t.TempDir(), "simulators.yaml")
	if err := os.WriteFile(file, []byte("- simulator: ethereum/two\n- simulator: ethereum/one\n"), 0600); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name, file, pattern string
		want                []string
		wantErr             bool
	}{
		{name: "default"},
		{name: "legacy anchored", pattern: "ethereum/", want: nil},
		{name: "legacy sorted", pattern: "ethereum/.*", want: []string{"ethereum/one", "ethereum/two"}},
		{name: "file order", file: file, want: []string{"ethereum/two", "ethereum/one"}},
		{name: "file filter", file: file, pattern: "one", want: []string{"ethereum/one"}},
		{name: "file no match", file: file, pattern: "missing"},
		{name: "bad regexp", pattern: "[", wantErr: true},
		{name: "bad file regexp", file: file, pattern: "[", wantErr: true},
		{name: "missing file", file: file + ".missing", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			list, err := simulatorList(&inv, tt.file, tt.pattern)
			if (err != nil) != tt.wantErr {
				t.Fatalf("error = %v, wantErr %v", err, tt.wantErr)
			}
			var names []string
			for _, sim := range list {
				names = append(names, sim.Simulator)
			}
			if !reflect.DeepEqual(names, tt.want) {
				t.Fatalf("got %v, want %v", names, tt.want)
			}
		})
	}
}
