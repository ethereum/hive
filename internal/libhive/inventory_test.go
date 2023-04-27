package libhive_test

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/ethereum/hive/internal/libhive"
)

func TestSplitClientName(t *testing.T) {
	tests := []struct {
		name                                                       string
		wantClient, wantDockerFile, wantUser, wantRepo, wantBranch string
	}{
		{"client", "client", "", "", "", ""},
		{"client_b", "client", "", "", "", "b"},
		{"the_client_b", "the_client", "", "", "", "b"},
		{"the_client_name_has_many_underscores_b", "the_client_name_has_many_underscores", "", "", "", "b"},
	}
	for _, test := range tests {
		cInfo, _ := libhive.ParseClientBuildInfoString(test.name)
		if cInfo.Name != test.wantClient || cInfo.TagBranch != test.wantBranch || cInfo.User != test.wantUser || cInfo.Repo != test.wantRepo {
			t.Errorf("SpnlitClientName(%q) -> (%q, %q, %q, %q), want (%q, %q, %q, %q)", test.name, cInfo.Name, cInfo.TagBranch, cInfo.User, cInfo.Repo, test.wantClient, test.wantBranch, test.wantUser, test.wantRepo)
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
			t.Errorf("SplitClientName(%q) -> (%q, %q, %q, %q), want error", test, cInfo.Name, cInfo.TagBranch, cInfo.User, cInfo.Repo)
		}
	}
}

func TestClientBuildInfoString(t *testing.T) {
	tests := []struct {
		buildInfo libhive.ClientBuildInfo
		want      string
	}{
		{libhive.ClientBuildInfo{Name: "client"}, "client"},
		{libhive.ClientBuildInfo{Name: "client", Repo: "myrepo", TagBranch: "mytag"}, "client_myrepo_mytag"},
		{libhive.ClientBuildInfo{Name: "client", DockerFile: "mydockerfile", User: "myuser"}, "client_mydockerfile_myuser"},
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
				{"name": "go-ethereum", "dockerfile": "git"},
				{"name": "go-ethereum", "branch": "latest", "dockerfile": "local"},
				{"name": "supereth3000"}
			]`,
			`
- name: go-ethereum
  dockerfile: git
- name: go-ethereum
  branch: latest
  dockerfile: local
- name: supereth3000
`,
			libhive.ClientsBuildInfo{
				{Name: "go-ethereum", DockerFile: "git"},
				{Name: "go-ethereum", TagBranch: "latest", DockerFile: "local"},
				{Name: "supereth3000"}},
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
