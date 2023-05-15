package libhive_test

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/ethereum/hive/internal/libhive"
)

func TestParseClientBuildInfoString(t *testing.T) {
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
		cInfo, _ := libhive.ParseClientBuildInfoString(test.name)
		if cInfo.Client != test.wantClient || !reflect.DeepEqual(test.wantBuildParams, cInfo.BuildArguments) {
			t.Errorf("ParseClientBuildInfoString(%q) -> (%q, %q), want (%q, %q)", test.name, cInfo.Client, cInfo.BuildArguments, test.wantClient, test.wantBuildParams)
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
		cInfo, err := libhive.ParseClientBuildInfoString(test)
		if err == nil {
			t.Errorf("SplitClientName(%q) -> (%q, %q), want error", test, cInfo.Client, cInfo.BuildArguments)
		}
	}
}

func TestClientBuildInfoString(t *testing.T) {
	tests := []struct {
		buildInfo libhive.ClientBuildInfo
		want      string
	}{
		{libhive.ClientBuildInfo{Client: "client"}, "client"},
		{libhive.ClientBuildInfo{Client: "client", BuildArguments: map[string]string{"repo": "myrepo", "branch": "mybranch"}}, "client_repo_myrepo_branch_mybranch"},
		{libhive.ClientBuildInfo{Client: "client", DockerFile: "mydockerfile", BuildArguments: map[string]string{"user": "myuser"}}, "client_mydockerfile_user_myuser"},
	}
	for _, test := range tests {
		if test.buildInfo.String() != test.want {
			t.Errorf("ClientBuildInfoString -> %q, want %q", test.buildInfo.String(), test.want)
		}
	}
}

func TestClientBuildInfoFromFile(t *testing.T) {
	jsonYamlTests := []struct {
		json string
		yaml string
		want libhive.ClientsBuildInfo
	}{
		{
			`[
				{"client": "go-ethereum", "dockerfile": "git"},
				{"client": "go-ethereum", "dockerfile": "local", "build_args": {"branch": "latest"}},
				{"client": "supereth3000"}
			]`,
			`
- client: go-ethereum
  dockerfile: git
- client: go-ethereum
  dockerfile: local
  build_args:
    branch: latest
- client: supereth3000
`,
			libhive.ClientsBuildInfo{
				{Client: "go-ethereum", DockerFile: "git"},
				{Client: "go-ethereum", DockerFile: "local", BuildArguments: map[string]string{"branch": "latest"}},
				{Client: "supereth3000"}},
		},
	}

	t.Run("ClientBuildInfoFromYaml", func(t *testing.T) {
		for _, test := range jsonYamlTests {
			r := strings.NewReader(test.yaml)
			clientInfo, err := libhive.ClientsBuildInfoFromFile(r)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(&clientInfo, &test.want) {
				t.Errorf("ClientBuildInfoFromYaml -> %q, want %q", clientInfo, test.want)
			}
		}
	})
	t.Run("ClientBuildInfoFromJson", func(t *testing.T) {
		for _, test := range jsonYamlTests {
			r := strings.NewReader(test.json)
			clientInfo, err := libhive.ClientsBuildInfoFromFile(r)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(&clientInfo, &test.want) {
				t.Errorf("ClientBuildInfoFromYaml -> %q, want %q", clientInfo, test.want)
			}
		}
	})
}

func TestInventory(t *testing.T) {
	basedir := filepath.FromSlash("../..")
	inv, err := libhive.LoadInventory(basedir)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("HasClient", func(t *testing.T) {
		clientInfo, err := libhive.ClientsBuildInfoFromString("go-ethereum_f:git,go-ethereum_latest,supereth3000")
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
		if inv.HasClient(clientInfo[2]) {
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
