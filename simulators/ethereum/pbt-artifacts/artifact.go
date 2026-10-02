package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

// manifest is the fixture set as fixtures/gen writes it. The artifacts are
// opaque bytes here: what they encode is what the clients are measured on.
type manifest struct {
	Genesis struct {
		File      string `json:"file"`
		StateRoot string `json:"stateRoot"`
	} `json:"genesis"`
	Valid struct {
		Snapshot       string `json:"snapshot"`
		Preimages      string `json:"preimages"`
		SnapshotDigest string `json:"snapshotDigest"`
		PreimageDigest string `json:"preimageDigest"`
	} `json:"valid"`
	Cases []testCase `json:"cases"`

	dir string
}

// testCase is one fixture: a verify case names an artifact pair, a produce
// case the defect a shim applies to the converter's source.
type testCase struct {
	ID          string   `json:"id"`
	Suite       string   `json:"suite"`
	Snapshot    string   `json:"snapshot"`
	Preimages   string   `json:"preimages"`
	Defect      []string `json:"defect"`
	Description string   `json:"description"`
}

// loadManifest reads the fixture set, checks every file it names is there
// and holds the valid pair to its digests.
func loadManifest(dir string) (*manifest, error) {
	blob, err := os.ReadFile(filepath.Join(dir, "manifest.json"))
	if err != nil {
		return nil, err
	}
	m := &manifest{dir: dir}
	if err := json.Unmarshal(blob, m); err != nil {
		return nil, fmt.Errorf("manifest does not parse: %w", err)
	}
	for _, c := range m.Cases {
		for _, p := range []string{c.Snapshot, c.Preimages} {
			if p == "" {
				continue
			}
			if _, err := m.read(p); err != nil {
				return nil, fmt.Errorf("case %s: %w", c.ID, err)
			}
		}
	}
	for _, f := range []struct{ path, want string }{
		{m.Valid.Snapshot, m.Valid.SnapshotDigest},
		{m.Valid.Preimages, m.Valid.PreimageDigest},
	} {
		blob, err := m.read(f.path)
		if err != nil {
			return nil, err
		}
		if got := crypto.Keccak256Hash(blob); got != common.HexToHash(f.want) {
			return nil, fmt.Errorf("%s hashes to %s, the manifest names %s", f.path, got, f.want)
		}
	}
	return m, nil
}

func (m *manifest) read(path string) ([]byte, error) {
	return os.ReadFile(filepath.Join(m.dir, path))
}

func (m *manifest) cases(suite string) []testCase {
	var out []testCase
	for _, c := range m.Cases {
		if c.Suite == suite {
			out = append(out, c)
		}
	}
	return out
}

func (m *manifest) validFile(artifact string) string {
	if artifact == "snapshot" {
		return m.Valid.Snapshot
	}
	return m.Valid.Preimages
}
