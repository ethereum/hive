package libhive_test

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/ethereum/hive/hivesim"
	"github.com/ethereum/hive/internal/fakes"
	"github.com/ethereum/hive/internal/libhive"
)

func TestRunner(t *testing.T) {
	var (
		allClients = []libhive.ClientDesignator{{Client: "client-1"}, {Client: "client-2"}, {Client: "client-3"}}
		simClients = allClients[1:]
	)

	inv := makeTestInventory()
	b := fakes.NewBuilder(&fakes.BuilderHooks{})
	cb := fakes.NewContainerBackend(&fakes.BackendHooks{
		StartContainer: func(image, containerID string, opt libhive.ContainerOptions) (*libhive.ContainerInfo, error) {
			t.Logf("StartContainer(image=%s, id=%s)", image, containerID)
			if strings.Contains(image, "/simulator/") {
				simURL := opt.Env["HIVE_SIMULATOR"]
				t.Log("HIVE_SIMULATOR =", simURL)

				sim := hivesim.NewAt(simURL)
				defs, err := sim.ClientTypes()
				if err != nil {
					t.Fatal("error getting client types:", err)
				}
				if names := clientDefinitionNames(defs); !reflect.DeepEqual(names, clientDesignatorNames(simClients)) {
					t.Fatal("wrong client names:", names)
				}
			}
			return new(libhive.ContainerInfo), nil
		},
	})

	var (
		runner    = libhive.NewRunner(inv, b, cb)
		simList   = []libhive.SimulatorDesignator{{Simulator: "sim-1"}}
		simOpt    = libhive.SimEnv{LogDir: t.TempDir(), ClientList: simClients}
		ctx       = context.Background()
		buildArgs = map[string]string{"SomeArg": "SomeValue"}
	)
	if err := runner.Build(ctx, allClients, simList, buildArgs); err != nil {
		t.Fatal("Build() failed:", err)
	}
	if _, err := runner.Run(context.Background(), "sim-1", simOpt, libhive.HiveInfo{}); err != nil {
		t.Fatal("Run() failed:", err)
	}

	content, _ := os.ReadFile(filepath.Join(simOpt.LogDir, "hive.json"))
	t.Logf("hive.json content: %s", content)
}

func makeTestInventory() libhive.Inventory {
	var inv libhive.Inventory
	inv.AddClient("client-1", nil)
	inv.AddClient("client-2", nil)
	inv.AddClient("client-3", nil)
	inv.AddSimulator("sim-1")
	return inv
}

func clientDefinitionNames(defs []*hivesim.ClientDefinition) []string {
	names := make([]string, 0, len(defs))
	for _, def := range defs {
		names = append(names, def.Name)
	}
	sort.Strings(names)
	return names
}

func clientDesignatorNames(clients []libhive.ClientDesignator) []string {
	names := make([]string, len(clients))
	for i, c := range clients {
		names[i] = c.Client
	}
	return names
}

func TestRunnerSimulatorBuildArgs(t *testing.T) {
	inv := makeTestInventory()
	inv.AddSimulator("sim-2")
	list := []libhive.SimulatorDesignator{
		{Simulator: "sim-1", DockerfileExt: "git", BuildArgs: map[string]string{"branch": "main", "fixtures": "stable"}},
		{Simulator: "sim-2", BuildArgs: map[string]string{"tag": "latest"}},
	}
	overrides := map[string]string{"branch": "override"}
	var built []libhive.SimulatorDesignator
	builder := fakes.NewBuilder(&fakes.BuilderHooks{
		BuildSimulatorImage: func(ctx context.Context, sim libhive.SimulatorDesignator) (string, error) {
			built = append(built, sim)
			return sim.Simulator, nil
		},
	})
	runner := libhive.NewRunner(inv, builder, fakes.NewContainerBackend(nil))
	if err := runner.Build(context.Background(), []libhive.ClientDesignator{{Client: "client-1"}}, list, overrides); err != nil {
		t.Fatal(err)
	}
	want := []libhive.SimulatorDesignator{
		{Simulator: "sim-1", DockerfileExt: "git", BuildArgs: map[string]string{"branch": "override", "fixtures": "stable"}},
		{Simulator: "sim-2", BuildArgs: map[string]string{"branch": "override", "tag": "latest"}},
	}
	if !reflect.DeepEqual(built, want) {
		t.Fatalf("built %+v, want %+v", built, want)
	}
	if list[0].BuildArgs["branch"] != "main" || len(overrides) != 1 {
		t.Fatal("build modified input arguments")
	}
}
