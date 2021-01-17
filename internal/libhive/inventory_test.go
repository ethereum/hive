package libhive

import (
	"path/filepath"
	"testing"
)

func TestSplitClientName(t *testing.T) {
	tests := []struct{
		name string
		wantClient, wantBranch string
	}{
		{"client", "client", ""},
		{"client_b", "client", "b"},
		{"the_client_b", "the_client", "b"},
	}
	for _, test := range tests {
		c, b := SplitClientName(test.name)
		if c != test.wantClient || b != test.wantBranch {
			t.Errorf("SplitClientName(%q) -> (%q, %q), want (%q, %q)", test.name, c, b, test.wantClient, test.wantBranch)
		}
	}
}

func TestInventory(t *testing.T) {
	basedir := filepath.FromSlash("../..")
	inv, err := LoadInventory(basedir)
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
