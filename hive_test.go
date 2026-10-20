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
	inv.AddSimulator("ethereum/three")
	configured := []libhive.SimulatorDesignator{
		{Simulator: "ethereum/two", DockerfileExt: "git", BuildArgs: map[string]string{"branch": "main"}},
		{Simulator: "ethereum/one"},
	}
	tests := []struct {
		name, pattern string
		configured    []libhive.SimulatorDesignator
		want          []libhive.SimulatorDesignator
		wantErr       bool
	}{
		{name: "default"},
		{name: "inventory anchored", pattern: "ethereum/"},
		{name: "inventory sorted", pattern: "ethereum/.*", want: []libhive.SimulatorDesignator{
			{Simulator: "ethereum/one"}, {Simulator: "ethereum/three"}, {Simulator: "ethereum/two"},
		}},
		{name: "config order without pattern", configured: configured, want: configured},
		{name: "config entry for a selected simulator", configured: configured, pattern: "two", want: configured[:1]},
		// A simulator the file does not configure is selected with its defaults.
		{name: "unconfigured simulator", configured: configured, pattern: "three", want: []libhive.SimulatorDesignator{
			{Simulator: "ethereum/three"},
		}},
		{name: "pattern over configured and unconfigured", configured: configured, pattern: "ethereum/t.*", want: []libhive.SimulatorDesignator{
			{Simulator: "ethereum/three"}, configured[0],
		}},
		{name: "no match", configured: configured, pattern: "missing"},
		{name: "bad regexp", pattern: "[", wantErr: true},
		{name: "bad regexp with config", configured: configured, pattern: "[", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			list, err := simulatorList(&inv, tt.configured, tt.pattern)
			if (err != nil) != tt.wantErr {
				t.Fatalf("error = %v, wantErr %v", err, tt.wantErr)
			}
			if !reflect.DeepEqual(list, tt.want) {
				t.Fatalf("got %+v, want %+v", list, tt.want)
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
