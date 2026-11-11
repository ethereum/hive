//go:build pbtgen

package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/ethereum/go-ethereum/common"
)

// effect is what a case changed against the valid pair, computed from the
// bytes the generator wrote; README.md says why the fixtures carry it.
type effect struct {
	Snapshot  *fileEffect `json:"snapshot,omitempty"`
	Preimages *fileEffect `json:"preimages,omitempty"`
}

// fileEffect is one artifact's change; the name lists hold at most four entries, in file order.
type fileEffect struct {
	ValidLen  int      `json:"validLen"`
	CaseLen   int      `json:"caseLen"`
	FirstDiff int      `json:"firstDiff"`
	Parses    bool     `json:"parses"`
	Added     []string `json:"added,omitempty"`
	Removed   []string `json:"removed,omitempty"`
	Changed   []string `json:"changed,omitempty"`
	Reordered bool     `json:"reordered,omitempty"`
	RootKept  bool     `json:"rootKept,omitempty"`
}

func (f *fileEffect) String() string {
	s := fmt.Sprintf("%d -> %d bytes, first differs at %d", f.ValidLen, f.CaseLen, f.FirstDiff)
	if !f.Parses {
		s += ", no longer parses"
	}
	for _, l := range []struct {
		label string
		list  []string
	}{{"added", f.Added}, {"removed", f.Removed}, {"changed", f.Changed}} {
		if len(l.list) > 0 {
			s += "; " + l.label + " " + strings.Join(l.list, " ")
		}
	}
	if f.Reordered {
		s += "; reordered"
	}
	if f.RootKept {
		s += "; root kept"
	}
	return s
}

// describeCase renders a case's manifest description: the clause, its note,
// the defect for a produce case, and each touched file's effect.
func describeCase(clause, note string, defect []string, snap, pre *fileEffect) string {
	s := "Clause: " + clause
	if note != "" {
		s += "\n" + note
	}
	if len(defect) > 0 {
		s += "\ndefect: " + strings.Join(defect, " ")
	}
	if snap != nil {
		s += "\nsnapshot: " + snap.String()
	}
	if pre != nil {
		s += "\npreimages: " + pre.String()
	}
	return s
}

// snapshotEffect diffs a case's snapshot against the valid one, which must
// decode: the generator has just written it.
func snapshotEffect(validBlob, caseBlob []byte) *fileEffect {
	f := &fileEffect{
		ValidLen:  len(validBlob),
		CaseLen:   len(caseBlob),
		FirstDiff: firstDiff(validBlob, caseBlob),
	}
	validSnap, err := decodeSnapshot(validBlob, false)
	if err != nil {
		panic(fmt.Sprintf("the valid snapshot does not decode: %v", err))
	}
	caseSnap, err := decodeSnapshot(caseBlob, false)
	f.Parses = err == nil
	f.Added, f.Removed, f.Changed, f.Reordered = diffEntries(leafEntries(validSnap.leaves()), leafEntries(caseSnap.leaves()), "=")
	f.RootKept = caseSnap.root == validSnap.root && (len(f.Added)+len(f.Removed)+len(f.Changed) > 0 || f.Reordered)
	return f
}

// preimageEffect diffs a case's preimage file against the valid one.
func preimageEffect(validBlob, caseBlob []byte) *fileEffect {
	f := &fileEffect{
		ValidLen:  len(validBlob),
		CaseLen:   len(caseBlob),
		FirstDiff: firstDiff(validBlob, caseBlob),
	}
	validRecs, err := decodePreimages(validBlob, false)
	if err != nil {
		panic(fmt.Sprintf("the valid preimage file does not decode: %v", err))
	}
	caseRecs, err := decodePreimages(caseBlob, false)
	f.Parses = err == nil
	f.Added, f.Removed, f.Changed, f.Reordered = diffEntries(recordEntries(validRecs), recordEntries(caseRecs), ":")
	return f
}

// entry is a file element's identity and a token that changes with its content.
type entry struct{ name, value string }

func leafEntries(leaves []leaf) []entry {
	out := make([]entry, len(leaves))
	for i, l := range leaves {
		out[i] = entry{
			name:  hex.EncodeToString(l.key),
			value: hex.EncodeToString(common.TrimLeftZeroes(l.value[:])),
		}
	}
	return out
}

func recordEntries(recs []record) []entry {
	out := make([]entry, len(recs))
	for i, r := range recs {
		out[i] = entry{name: hex.EncodeToString(r.addr[:]), value: slotDigest(r)}
	}
	return out
}

// slotDigest is 64 bits of hash over the slot keys, so any slot change yields a new token.
func slotDigest(r record) string {
	var keys []byte
	for _, s := range r.slots {
		keys = append(keys, s[:]...)
	}
	sum := sha256.Sum256(keys)
	return hex.EncodeToString(sum[:])[:16]
}

// diffEntries names what the case added, removed and changed, each list held
// to the first four in file order, and whether the entries they share moved.
func diffEntries(valid, cs []entry, sep string) (added, removed, changed []string, reordered bool) {
	validVal := values(valid)
	caseVal := values(cs)
	for _, e := range cs {
		switch v, ok := validVal[e.name]; {
		case !ok:
			added = append(added, e.name+sep+e.value)
		case v != e.value:
			changed = append(changed, e.name+sep+e.value)
		}
	}
	for _, e := range valid {
		if _, ok := caseVal[e.name]; !ok {
			removed = append(removed, e.name+sep+e.value)
		}
	}
	first4 := func(l []string) []string { return l[:min(len(l), 4)] }
	return first4(added), first4(removed), first4(changed),
		!slices.Equal(shared(cs, validVal), shared(valid, caseVal))
}

func values(entries []entry) map[string]string {
	out := make(map[string]string, len(entries))
	for _, e := range entries {
		out[e.name] = e.value
	}
	return out
}

// shared lists the names both files hold, in this file's order.
func shared(entries []entry, other map[string]string) []string {
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		if _, ok := other[e.name]; ok {
			out = append(out, e.name)
		}
	}
	return out
}

func firstDiff(a, b []byte) int {
	for i := range min(len(a), len(b)) {
		if a[i] != b[i] {
			return i
		}
	}
	return min(len(a), len(b))
}

// duplicateEffect names the first two cases recording the same effect.
func duplicateEffect(cases []caseEntry) (string, string) {
	seen := make(map[string]string, len(cases))
	for _, c := range cases {
		if c.Effect == nil {
			continue
		}
		key, _ := json.Marshal(c.Effect)
		if prev, dup := seen[string(key)]; dup {
			return prev, c.ID
		}
		seen[string(key)] = c.ID
	}
	return "", ""
}
