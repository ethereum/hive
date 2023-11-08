package libhive

import (
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/davecgh/go-spew/spew"
)

func TestParseClientDesignator(t *testing.T) {
	tests := []struct {
		name            string
		wantClient      string
		wantBuildParams map[string]string
	}{
		{"client", "client", nil},
		{"client_b", "client", map[string]string{"tag": "b"}},
		{"the_client_b", "the_client", map[string]string{"tag": "b"}},
		{"the_client_name_has_many_underscores_b", "the_client_name_has_many_underscores", map[string]string{"tag": "b"}},
	}
	for _, test := range tests {
		cInfo, _ := parseClientDesignator(test.name)
		if cInfo.Client != test.wantClient || !reflect.DeepEqual(test.wantBuildParams, cInfo.BuildArgs) {
			t.Errorf("parseClientDesignator(%q) -> (%q, %q), want (%q, %q)", test.name, cInfo.Client, cInfo.BuildArgs, test.wantClient, test.wantBuildParams)
		}
	}
}

func TestParseClientDesignatorUnderscore(t *testing.T) {
	tests := []string{
		"",
		"__",
		"_somebranch",
		"client_",
	}
	for _, test := range tests {
		cInfo, err := parseClientDesignator(test)
		if err == nil {
			t.Errorf("parseClientDesignator(%q) -> (%q, %q), want error", test, cInfo.Client, cInfo.BuildArgs)
		}
	}
}

func TestClientNaming(t *testing.T) {
	var inv Inventory
	inv.AddClient("c1", &InventoryClient{Dockerfiles: []string{"git", "local"}})
	inv.AddClient("c2", nil)

	tests := []struct {
		clients []ClientDesignator
		names   []string
		wantErr error
	}{
		{
			clients: []ClientDesignator{{Client: "c1"}, {Client: "c2"}},
			names:   []string{"c1", "c2"},
		},
		{
			clients: []ClientDesignator{{Client: "c2", Nametag: "foo"}},
			names:   []string{"c2_foo"},
		},
		{
			clients: []ClientDesignator{{Client: "c1"}, {Client: "c2"}, {Client: "c2", Nametag: "foo"}},
			names:   []string{"c1", "c2", "c2_foo"},
		},
		{
			clients: []ClientDesignator{
				{Client: "c1", BuildArgs: map[string]string{"tag": "latest"}},
				{Client: "c1", BuildArgs: map[string]string{"tag": "unstable"}},
			},
			names: []string{"c1_latest", "c1_unstable"},
		},
		{
			clients: []ClientDesignator{
				{Client: "c1", BuildArgs: map[string]string{"tag": "latest"}},
				{Client: "c1", BuildArgs: map[string]string{"tag": "latest", "other": "1"}},
			},
			names: []string{"c1_latest", "c1_latest_other_1"},
		},
		{
			clients: []ClientDesignator{
				{Client: "c1", DockerfileExt: "git"},
				{Client: "c1", BuildArgs: map[string]string{"tag": "latest"}},
			},
			names: []string{"c1", "c1_latest"},
		},
		{
			clients: []ClientDesignator{
				{Client: "c1", DockerfileExt: "git"},
				{Client: "c1", DockerfileExt: "local"},
			},
			names: []string{"c1_git", "c1_local"},
		},
		// Errors:
		{
			clients: []ClientDesignator{
				{Client: "c1", BuildArgs: map[string]string{"tag": "latest"}},
				{Client: "c1", BuildArgs: map[string]string{"tag": "latest"}},
			},
			wantErr: fmt.Errorf("duplicate client name \"c1_latest\""),
		},
	}

	for i := range tests {
		test := tests[i]
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			err := validateClients(&inv, test.clients)
			if err != nil {
				if test.wantErr == nil {
					t.Fatal("unexpected error:", err)
				}
				if err.Error() != test.wantErr.Error() {
					t.Fatalf("wrong error: %v, want %v", err, test.wantErr)
				}
				return
			} else if test.wantErr != nil {
				t.Fatal("expected error")
			}

			names := make([]string, len(test.clients))
			for i, c := range test.clients {
				names[i] = c.Name()
			}
			if !reflect.DeepEqual(names, test.names) {
				t.Log("want:", test.names)
				t.Fatal("names:", names)
			}
		})
	}
}

func TestParseClientListYAML(t *testing.T) {
	yamlInput := `
- client: go-ethereum
  dockerfile: git
  build_args:
    tag: custom
- client: go-ethereum
  dockerfile: local
- client: supereth3000
  build_args:
    github: org/repository
- client: supereth3000
  nametag: thebest
`

	expectedOutput := []ClientDesignator{
		{Client: "go-ethereum", Nametag: "custom", DockerfileExt: "git", BuildArgs: map[string]string{"tag": "custom"}},
		{Client: "go-ethereum", DockerfileExt: "local"},
		{Client: "supereth3000", Nametag: "github_org/repository", BuildArgs: map[string]string{"github": "org/repository"}},
		{Client: "supereth3000", Nametag: "thebest"},
	}

	var inv Inventory
	inv.AddClient("go-ethereum", &InventoryClient{Dockerfiles: []string{"git", "local"}})
	inv.AddClient("supereth3000", nil)

	r := strings.NewReader(yamlInput)
	clientInfo, err := ParseClientListYAML(&inv, r)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(&clientInfo, &expectedOutput) {
		t.Logf("want: %+v", expectedOutput)
		t.Errorf(" got: %+v", clientInfo)
	}
	for _, c := range clientInfo {
		t.Logf("name: %v", c.Name())
	}
}

// This test ensures the real hive client definitions can be loaded.
func TestLoadInventory(t *testing.T) {
	basedir := filepath.FromSlash("../..")
	inv, err := LoadInventory(basedir)
	if err != nil {
		t.Fatal(err)
	}

	t.Log("clients:", spew.Sdump(inv.Clients))
	t.Log("simulators:", inv.Simulators)
}
