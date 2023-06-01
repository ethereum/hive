package libhive

import (
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
		{"client_b", "client", map[string]string{"branch": "b"}},
		{"the_client_b", "the_client", map[string]string{"branch": "b"}},
		{"the_client_name_has_many_underscores_b", "the_client_name_has_many_underscores", map[string]string{"branch": "b"}},
	}
	for _, test := range tests {
		cInfo, _ := parseClientDesignator(test.name)
		if cInfo.Client != test.wantClient || !reflect.DeepEqual(test.wantBuildParams, cInfo.BuildArgs) {
			t.Errorf("parseClientDesignator(%q) -> (%q, %q), want (%q, %q)", test.name, cInfo.Client, cInfo.BuildArgs, test.wantClient, test.wantBuildParams)
		}
	}
}

func TestInvalidSplitClientName(t *testing.T) {
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

func TestClientDesignatorString(t *testing.T) {
	tests := []struct {
		client ClientDesignator
		string string
	}{
		{
			client: ClientDesignator{Client: "client"},
			string: "client",
		},
		{
			client: ClientDesignator{Client: "client", BuildArgs: map[string]string{"repo": "myrepo", "branch": "mybranch"}},
			string: "client_branch_mybranch_repo_myrepo",
		},
		{
			client: ClientDesignator{Client: "client", DockerfileExt: "mydockerfile", BuildArgs: map[string]string{"user": "myuser"}},
			string: "client_mydockerfile_user_myuser",
		},
	}
	for _, test := range tests {
		if test.client.String() != test.string {
			t.Errorf("wrong name %q, want %q", test.client.String(), test.string)
		}
	}
}

func TestParseClientListYAML(t *testing.T) {
	yamlInput := `
- client: go-ethereum
  dockerfile: git
- client: go-ethereum
  dockerfile: local
  build_args:
    branch: latest
- client: supereth3000
  build_args:
    some_other_arg: some_other_value
`

	expectedOutput := []ClientDesignator{
		{Client: "go-ethereum", DockerfileExt: "git"},
		{Client: "go-ethereum", DockerfileExt: "local", BuildArgs: map[string]string{"branch": "latest"}},
		{Client: "supereth3000", BuildArgs: map[string]string{"some_other_arg": "some_other_value"}},
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
		t.Errorf("ParseClientListYAML -> %v, want %v", clientInfo, expectedOutput)
	}
}

func TestInventory(t *testing.T) {
	basedir := filepath.FromSlash("../..")
	inv, err := LoadInventory(basedir)
	if err != nil {
		t.Fatal(err)
	}
	spew.Dump(inv)

	t.Run("HasClient", func(t *testing.T) {
		clientInfo, err := ParseClientList(&inv, "go-ethereum,go-ethereum_latest,lighthouse-vc")
		if err != nil {
			t.Fatal(err)
		}
		if len(clientInfo) != 3 {
			t.Fatal("wrong number of clients")
		}
		if !inv.HasClient(clientInfo[0]) {
			t.Error("can't find go-ethereum client")
		}
		if !inv.HasClient(clientInfo[1]) {
			t.Error("can't find go-ethereum_latest client")
		}
		if inv.HasClient(ClientDesignator{Client: "supereth3000"}) {
			t.Error("returned true for unknown client")
		}
	})
	t.Run("HasSimulator", func(t *testing.T) {
		if !inv.HasSimulator("smoke/genesis") {
			t.Error("can't find smoke/genesis simulator")
		}
		if inv.HasSimulator("unknown simulator name") {
			t.Error("returned true for unknown simulator name")
		}
	})
}
