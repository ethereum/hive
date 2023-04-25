package libhive_test

import (
	"path/filepath"
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
		{"the_client_u:user", "the_client", "", "user", "", ""},
		{"the_client_u:user_b", "the_client", "", "user", "", "b"},
		{"the_client_u:user_b:branch", "the_client", "", "user", "", "branch"},
		{"the_client_b:branch_u:user", "the_client", "", "user", "", "branch"},
		{"the_client_r:repo_b:branch_u:user", "the_client", "", "user", "repo", "branch"},
		{"client_r:repo_b:branch_u:user", "client", "", "user", "repo", "branch"},
		{"client_b:branch_u:user_r:repo", "client", "", "user", "repo", "branch"},
		{"client_b:branch_f:git_r:repo", "client", "git", "", "repo", "branch"},
	}
	for _, test := range tests {
		cInfo := libhive.SplitClientName(test.name)
		if cInfo.Name != test.wantClient || cInfo.TagBranch != test.wantBranch || cInfo.User != test.wantUser || cInfo.Repo != test.wantRepo {
			t.Errorf("SpnlitClientName(%q) -> (%q, %q, %q, %q), want (%q, %q, %q, %q)", test.name, cInfo.Name, cInfo.TagBranch, cInfo.User, cInfo.Repo, test.wantClient, test.wantBranch, test.wantUser, test.wantRepo)
		}
	}
}

func TestInventory(t *testing.T) {
	basedir := filepath.FromSlash("../..")
	inv, err := libhive.LoadInventory(basedir)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("HasClient", func(t *testing.T) {
		if !inv.HasClient("go-ethereum") {
			t.Error("can't find go-ethereum client")
		}
		if !inv.HasClient("go-ethereum_latest") {
			t.Error("can't find go-ethereum_latest client")
		}
		if inv.HasClient("supereth3000") {
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
