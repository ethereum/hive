//go:build pbtgen

// Command gen writes the fixture set: the anchor genesis, the valid
// artifacts from the reference converter, and one file per way an artifact
// can lie. The simulator image runs it when it builds. -ref names an
// execution-specs checkout for the root gate, which only a local run has.
//
//	go run -tags pbtgen . -geth /path/to/geth [-ref /path/to/execution-specs] -out /tmp/fixtures
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/trie/bintrie"
)

func main() {
	var (
		gethBin = flag.String("geth", "geth", "path to a geth binary built from the PBT fork")
		outDir  = flag.String("out", "..", "fixtures directory to write")
		ref     = flag.String("ref", "", "execution-specs checkout for the spec-reference root gate")
	)
	flag.Parse()

	if err := run(*gethBin, *outDir, *ref); err != nil {
		log.Fatal(err)
	}
}

func run(gethBin, outDir, ref string) error {
	alloc := edgeCaseAlloc()
	genesisPath := filepath.Join(outDir, "genesis.json")
	if err := writeGenesis(genesisPath, alloc); err != nil {
		return err
	}
	fmt.Printf("genesis.json: %d accounts\n", len(alloc))

	valid, err := convert(gethBin, genesisPath, outDir, alloc)
	if err != nil {
		return err
	}
	valid.stateRoot = (&core.Genesis{Alloc: alloc}).ToBlock().Root()
	fmt.Printf("valid artifacts: pbtRoot %x, %d leaves, %d preimage records\n",
		valid.root, len(valid.leaves), len(valid.records))
	if err := admit(valid, genesisPath, ref); err != nil {
		return err
	}

	cases, err := writeCases(outDir, valid)
	if err != nil {
		return err
	}
	produce, err := produceCases(alloc)
	if err != nil {
		return err
	}
	return writeManifest(outDir, valid, append(cases, produce...))
}

// artifacts is the valid pair, decoded.
type artifacts struct {
	root         common.Hash
	stateRoot    common.Hash
	snap         *snapshot
	leaves       []leaf
	records      []record
	snapshotBlob []byte
	preimageBlob []byte
}

type leaf struct {
	key   []byte
	value [32]byte
}

type record struct {
	addr  common.Address
	slots []common.Hash
}

// convert runs the reference converter over the genesis and holds its output
// to what the allocation implies and to the strict decoder.
func convert(gethBin, genesisPath, outDir string, alloc types.GenesisAlloc) (*artifacts, error) {
	datadir, err := os.MkdirTemp("", "pbt-fixtures-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(datadir)

	if out, err := exec.Command(gethBin, "--datadir", datadir, "--cache.preimages", "init", genesisPath).CombinedOutput(); err != nil {
		return nil, fmt.Errorf("geth init: %w\n%s", err, out)
	}
	var (
		validDir = filepath.Join(outDir, "valid")
		snapPath = filepath.Join(validDir, "snapshot.bin")
		prePath  = filepath.Join(validDir, "preimages.bin")
	)
	if err := os.MkdirAll(validDir, 0755); err != nil {
		return nil, err
	}
	cmd := exec.Command(gethBin, "--datadir", datadir, "bintrie", "convert",
		"--snapshot-out", snapPath, "--preimages-out", prePath)
	if out, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("geth bintrie convert: %w\n%s", err, out)
	}

	snapBlob, err := os.ReadFile(snapPath)
	if err != nil {
		return nil, err
	}
	snap, err := decodeSnapshot(snapBlob, true)
	if err != nil {
		return nil, fmt.Errorf("the converter's snapshot breaks a serialization rule: %w", err)
	}
	// Every case is written by this encoder, so it must reproduce the
	// converter's bytes, or a case differs from them in more than its mutation.
	if !bytes.Equal(snap.encode(), snapBlob) {
		return nil, fmt.Errorf("the generator's encoder does not reproduce the converter's snapshot")
	}
	preBlob, err := os.ReadFile(prePath)
	if err != nil {
		return nil, err
	}
	want := derivePreimages(alloc)
	if !bytes.Equal(preBlob, want) {
		return nil, fmt.Errorf("the converter's preimage file disagrees with the layout the state implies:\nconverter %x\nderived   %x", preBlob, want)
	}
	leaves := snap.leaves()
	records, err := decodePreimages(preBlob, true)
	if err != nil {
		return nil, fmt.Errorf("the derived preimage file does not decode: %w", err)
	}
	if got := foldRoot(leaves); got != snap.root {
		return nil, fmt.Errorf("valid snapshot claims root %x, its leaves fold to %x", snap.root, got)
	}
	if err := checkLeaves(leaves, deriveLeaves(alloc)); err != nil {
		return nil, fmt.Errorf("converter disagrees with the embedding rules: %w", err)
	}
	return &artifacts{
		root: snap.root, snap: snap, leaves: leaves, records: records,
		snapshotBlob: snapBlob, preimageBlob: preBlob,
	}, nil
}

func foldRoot(leaves []leaf) common.Hash {
	b := bintrie.NewStackBuilder(nil)
	for _, l := range leaves {
		if err := b.Add(l.key, l.value[:]); err != nil {
			panic(fmt.Sprintf("valid leaves do not fold: %v", err))
		}
	}
	return b.Finish()
}

// mutation is one way an artifact can lie. One hook shapes the case; a leaf
// hook may also reshape the records.
type mutation struct {
	id     string
	clause string
	note   string

	leaves   func([]leaf) []leaf
	snap     func(*snapshot) *snapshot
	framed   func([]framed) []framed // the valid records, under the valid root
	records  func([]record) []record
	rawSnap  func([]byte) []byte
	rawPre   func([]byte) []byte
	keepRoot bool // keep the valid root, so check 1 catches it
	verbatim bool // write records in the order given, not keccak order
}

// checkClause holds a case's bytes to its clause: a snapshot.* or preimages.*
// clause is a byte rule the named strict decoder must reject with exactly
// that clause; any other clause is a dual-check rule the decoder must accept.
func checkClause(id, clause, prefix, decoder string, err error) error {
	byteRule := strings.HasPrefix(clause, prefix)
	switch {
	case byteRule && err == nil:
		return fmt.Errorf("case %s: %s is a byte rule but the strict %s decoder accepts the bytes", id, clause, decoder)
	case byteRule:
		var re *ruleError
		if !errors.As(err, &re) || re.clause != clause {
			return fmt.Errorf("case %s: %s is a byte rule but the strict %s decoder rejects it as %v", id, clause, decoder, err)
		}
	case !byteRule && err != nil:
		return fmt.Errorf("case %s: %s is a dual-check rule but the strict %s decoder rejects the bytes: %v", id, clause, decoder, err)
	}
	return nil
}

func writeCases(outDir string, valid *artifacts) ([]caseEntry, error) {
	var entries []caseEntry
	seen := make(map[string]bool)
	for _, m := range mutations(valid) {
		if seen[m.id] {
			return nil, fmt.Errorf("case %s is written twice", m.id)
		}
		seen[m.id] = true
		suite, _, _ := strings.Cut(m.id, "/")
		dir := filepath.Join(outDir, filepath.FromSlash(m.id))
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, err
		}
		entry := caseEntry{
			ID: m.id, Suite: suite, Clause: m.clause,
			Snapshot: "valid/snapshot.bin", Preimages: "valid/preimages.bin",
		}
		var snapBytes, pre []byte // nil means untouched; empty means an empty file
		switch {
		case m.leaves != nil:
			leaves := m.leaves(cloneLeaves(valid.leaves))
			root := valid.root
			if !m.keepRoot {
				root = foldRoot(leaves)
			}
			s, err := fromLeaves(root, leaves)
			if err != nil {
				return nil, fmt.Errorf("case %s: %w", m.id, err)
			}
			snapBytes = s.encode()
			if m.records != nil {
				pre = encodePreimages(m.records(cloneRecords(valid.records)))
			}
		case m.snap != nil:
			s := m.snap(cloneSnapshot(valid.snap))
			if !m.keepRoot {
				s.root = foldRoot(s.leaves())
			} else {
				s.root = valid.root
			}
			snapBytes = s.encode()
		case m.records != nil:
			recs := m.records(cloneRecords(valid.records))
			if m.verbatim {
				pre = encodeRecordsVerbatim(recs)
			} else {
				pre = encodePreimages(recs)
			}
		case m.framed != nil:
			snapBytes = frame(m.framed(valid.snap.records()), valid.root)
		case m.rawSnap != nil:
			snapBytes = m.rawSnap(valid.snapshotBlob)
		case m.rawPre != nil:
			pre = m.rawPre(valid.preimageBlob)
		default:
			return nil, fmt.Errorf("case %s shapes nothing", m.id)
		}
		if snapBytes != nil {
			_, err := decodeSnapshot(snapBytes, true)
			if err := checkClause(m.id, m.clause, "snapshot.", "snapshot", err); err != nil {
				return nil, err
			}
		}
		if pre != nil {
			_, err := decodePreimages(pre, true)
			if err := checkClause(m.id, m.clause, "preimages.", "preimage", err); err != nil {
				return nil, err
			}
		}
		changed := false
		for _, f := range []struct {
			blob  []byte
			valid []byte
			name  string
			field *string
		}{
			{snapBytes, valid.snapshotBlob, "snapshot.bin", &entry.Snapshot},
			{pre, valid.preimageBlob, "preimages.bin", &entry.Preimages},
		} {
			if f.blob == nil {
				continue
			}
			if err := os.WriteFile(filepath.Join(dir, f.name), f.blob, 0644); err != nil {
				return nil, err
			}
			*f.field = path.Join(m.id, f.name)
			changed = changed || !bytes.Equal(f.blob, f.valid)
		}
		if !changed {
			return nil, fmt.Errorf("case %s produced the valid files unchanged", m.id)
		}
		entry.Effect = &effect{}
		if snapBytes != nil {
			entry.Effect.Snapshot = snapshotEffect(valid.snapshotBlob, snapBytes)
		}
		if pre != nil {
			entry.Effect.Preimages = preimageEffect(valid.preimageBlob, pre)
		}
		entry.Description = describeCase(m.clause, m.note, nil, entry.Effect.Snapshot, entry.Effect.Preimages)
		entries = append(entries, entry)
	}
	return entries, nil
}

// produceCases each delete one preimage from the converter's source store,
// which converter step 2 must refuse. The shell has no keccak, so the store
// keys are recorded here. Deleting a slot's preimage removes it for every
// account holding that slot, so the slot target has one holder.
func produceCases(alloc types.GenesisAlloc) ([]caseEntry, error) {
	slot := h(7)
	var holders int
	for _, acct := range alloc {
		if _, ok := acct.Storage[slot]; ok {
			holders++
		}
	}
	if holders != 1 {
		return nil, fmt.Errorf("slot 7 is held by %d accounts; the missing-slot case needs one", holders)
	}
	drop := func(id, note string, preimage []byte) caseEntry {
		const clause = "converter.preimage-set-matches-leaves"
		defect := []string{"drop-preimage", crypto.Keccak256Hash(preimage).Hex()}
		return caseEntry{
			ID: "produce/" + id, Suite: "produce", Clause: clause, Defect: defect,
			Description: describeCase(clause, note, defect, nil, nil),
		}
	}
	return []caseEntry{
		drop("missing-account-preimage", "the source has no preimage for account "+eoaBalance.Hex(), eoaBalance[:]),
		drop("missing-slot-preimage", "the source has no preimage for slot 7, held by "+storageHeader.Hex(), slot[:]),
	}, nil
}

func cloneLeaves(in []leaf) []leaf {
	out := make([]leaf, len(in))
	for i, l := range in {
		out[i] = leaf{key: bytes.Clone(l.key), value: l.value}
	}
	return out
}

func cloneRecords(in []record) []record {
	out := make([]record, len(in))
	for i, r := range in {
		out[i] = record{addr: r.addr, slots: slices.Clone(r.slots)}
	}
	return out
}

type caseEntry struct {
	ID          string   `json:"id"`
	Suite       string   `json:"suite"`
	Snapshot    string   `json:"snapshot,omitempty"`
	Preimages   string   `json:"preimages,omitempty"`
	Defect      []string `json:"defect,omitempty"`
	Clause      string   `json:"clause"`
	Description string   `json:"description"`
	Effect      *effect  `json:"effect,omitempty"`
}

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
	Cases []caseEntry `json:"cases"`
}

func writeManifest(outDir string, valid *artifacts, cases []caseEntry) error {
	seen := make(map[string]bool, len(cases))
	for _, c := range cases {
		if seen[c.ID] {
			return fmt.Errorf("case id %s is written twice", c.ID)
		}
		seen[c.ID] = true
	}
	if a, b := duplicateEffect(cases); a != "" {
		return fmt.Errorf("cases %s and %s record the same effect: they are one mutation written twice", a, b)
	}
	var m manifest
	m.Genesis.File = "genesis.json"
	m.Genesis.StateRoot = valid.stateRoot.Hex()
	m.Valid.Snapshot = "valid/snapshot.bin"
	m.Valid.Preimages = "valid/preimages.bin"
	m.Valid.SnapshotDigest = crypto.Keccak256Hash(valid.snapshotBlob).Hex()
	m.Valid.PreimageDigest = crypto.Keccak256Hash(valid.preimageBlob).Hex()
	m.Cases = cases

	blob, err := json.MarshalIndent(&m, "", "  ")
	if err != nil {
		return err
	}
	fmt.Printf("manifest: %d cases\n", len(cases))
	return os.WriteFile(filepath.Join(outDir, "manifest.json"), append(blob, '\n'), 0644)
}

// cloneSnapshot deep-copies a decoded snapshot, so a mutation can edit the
// copy without disturbing the valid one other cases still read.
func cloneSnapshot(s *snapshot) *snapshot {
	out := &snapshot{root: s.root}
	for _, h := range s.headers {
		out.headers = append(out.headers, cloneHeader(h))
	}
	for _, g := range s.code {
		out.code = append(out.code, cloneGroup(g))
	}
	for _, r := range s.storage {
		out.storage = append(out.storage, cloneStorageRecord(r))
	}
	return out
}

func cloneHeader(h header) header {
	h.nonce, h.balance, h.codeSize = slices.Clone(h.nonce), slices.Clone(h.balance), slices.Clone(h.codeSize)
	h.slots = slices.Clone(h.slots)
	for i := range h.slots {
		h.slots[i].value = slices.Clone(h.slots[i].value)
	}
	return h
}

func cloneGroup(g group) group {
	g.entries = slices.Clone(g.entries)
	for i := range g.entries {
		g.entries[i].value = slices.Clone(g.entries[i].value)
	}
	return g
}

func cloneStorageRecord(r storageRecord) storageRecord {
	groups := make([]group, len(r.groups))
	for i, g := range r.groups {
		groups[i] = cloneGroup(g)
	}
	r.groups = groups
	return r
}
