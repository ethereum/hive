//go:build pbtgen

package main

import (
	"bytes"
	_ "embed"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

// pbtRootPy computes a genesis's PBT root with execution-specs' state model.
//
//go:embed pbt_root.py
var pbtRootPy string

// Digests of the admitted pair: execution-specs computes its root and
// nethermind writes the same bytes. To move it on purpose, run validate.sh
// with an execution-specs checkout and update both.
var (
	pinnedSnapshot  = common.HexToHash("0x5d688e1b25e248b6e0a64d6436391b008bb505da8e849261ff0d934642949c0c")
	pinnedPreimages = common.HexToHash("0xe0af5df37c748df3eb3ba0adb07b138e19ffaeccfd6e2eaf7c46ebaf7dc60d2b")
)

// admit checks the pair against the pinned digests and, given -ref, its root
// against execution-specs. Neither rests on geth's PBT code.
func admit(valid *artifacts, genesisPath, ref string) error {
	if ref != "" {
		root, err := specRoot(ref, genesisPath)
		if err != nil {
			return err
		}
		if root != valid.root {
			return fmt.Errorf("the spec reference roots the genesis at %x, the converter at %x", root, valid.root)
		}
	}
	snap := crypto.Keccak256Hash(valid.snapshotBlob)
	pre := crypto.Keccak256Hash(valid.preimageBlob)
	if snap != pinnedSnapshot || pre != pinnedPreimages {
		return fmt.Errorf("the canonical pair moved: snapshot %x, preimages %x; pinned %x, %x", snap, pre, pinnedSnapshot, pinnedPreimages)
	}
	return nil
}

// specRoot runs pbt_root.py on a genesis with the checkout's own interpreter.
func specRoot(ref, genesisPath string) (common.Hash, error) {
	var stderr bytes.Buffer
	cmd := exec.Command(filepath.Join(ref, ".venv", "bin", "python"), "-", genesisPath)
	cmd.Env = append(os.Environ(), "PYTHONPATH="+filepath.Join(ref, "src"))
	cmd.Stdin = strings.NewReader(pbtRootPy)
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return common.Hash{}, fmt.Errorf("spec reference on %s: %w\n%s", genesisPath, err, stderr.String())
	}
	return common.HexToHash(strings.TrimSpace(string(out))), nil
}
