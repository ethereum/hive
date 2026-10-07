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
	configured := []libhive.SimulatorDesignator{{Simulator: "ethereum/two"}, {Simulator: "ethereum/one"}}
	tests := []struct {
		name, pattern string
		configured    []libhive.SimulatorDesignator
		want          []string
		wantErr       bool
	}{
		{name: "default"},
		{name: "inventory anchored", pattern: "ethereum/", want: nil},
		{name: "inventory sorted", pattern: "ethereum/.*", want: []string{"ethereum/one", "ethereum/two"}},
		{name: "config order", configured: configured, want: []string{"ethereum/two", "ethereum/one"}},
		{name: "config filter", configured: configured, pattern: "one", want: []string{"ethereum/one"}},
		{name: "config no match", configured: configured, pattern: "missing"},
		{name: "bad regexp", pattern: "[", wantErr: true},
		{name: "bad config regexp", configured: configured, pattern: "[", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			list, err := simulatorList(&inv, tt.configured, tt.pattern)
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

func TestParseConfigFile(t *testing.T) {
	var inv libhive.Inventory
	inv.AddClient("go-ethereum", nil)
	inv.AddSimulator("ethereum/one")
	file := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(file, []byte("- client: go-ethereum\n- simulator: ethereum/one\n"), 0600); err != nil {
		t.Fatal(err)
	}
	cfg, err := parseConfigFile(&inv, file)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Clients) != 1 || cfg.Clients[0].Client != "go-ethereum" {
		t.Fatalf("unexpected clients: %+v", cfg.Clients)
	}
	if len(cfg.Simulators) != 1 || cfg.Simulators[0].Simulator != "ethereum/one" {
		t.Fatalf("unexpected simulators: %+v", cfg.Simulators)
	}
	if _, err := parseConfigFile(&inv, file+".missing"); err == nil {
		t.Fatal("expected error for missing file")
	}
}
