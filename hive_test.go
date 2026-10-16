package main

import (
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

func TestApplySimBuildArgs(t *testing.T) {
	list := []libhive.SimulatorDesignator{
		{Simulator: "sim-1", DockerfileExt: "git", BuildArgs: map[string]string{"branch": "main", "fixtures": "stable"}},
		{Simulator: "sim-2"},
	}
	overrides := map[string]string{"branch": "override"}
	got := applySimBuildArgs(list, overrides)
	want := []libhive.SimulatorDesignator{
		{Simulator: "sim-1", DockerfileExt: "git", BuildArgs: map[string]string{"branch": "override", "fixtures": "stable"}},
		{Simulator: "sim-2", BuildArgs: map[string]string{"branch": "override"}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
	if list[0].BuildArgs["branch"] != "main" || list[1].BuildArgs != nil {
		t.Fatal("input list was modified")
	}
}
