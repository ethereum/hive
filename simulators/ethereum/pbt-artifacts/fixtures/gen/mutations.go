//go:build pbtgen

package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math/big"
	"slices"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/trie/bintrie"
)

// mutations is the catalogue, one entry per way an artifact can lie. A leaf
// or snap mutation recomputes the claimed root unless keepRoot is set, so it
// reaches past check 1 to the rule it targets.
func mutations(valid *artifacts) []mutation {
	var (
		zero          [32]byte
		one           = [32]byte{31: 1}
		emptyCode     = common.Hash(crypto.Keccak256(nil))
		codeHash      = crypto.Keccak256Hash(fill(chunk))
		zeroChunkHash = crypto.Keccak256Hash(zeroChunkCode())
		surplusAddr   = common.HexToAddress("0x00000000000000000000000000000000deadbeef")
	)
	dropRecord := func(addr common.Address) func([]record) []record {
		return func(recs []record) []record {
			return slices.DeleteFunc(recs, func(r record) bool { return r.addr == addr })
		}
	}
	viaRecords := func(id, clause, note string, f func([]record) []record) mutation {
		return mutation{id: "preimages/" + id, clause: clause, note: note, records: f}
	}
	viaRawPre := func(id, clause, note string, f func([]byte) []byte) mutation {
		return mutation{id: "preimages/" + id, clause: clause, note: note, rawPre: f}
	}
	viaLeaves := func(id, clause, note string, f func([]leaf) []leaf) mutation {
		return mutation{id: "snapshot/" + id, clause: clause, note: note, leaves: f}
	}
	viaRawSnap := func(id, clause, note string, f func([]byte) []byte) mutation {
		return mutation{id: "snapshot/" + id, clause: clause, note: note, rawSnap: f}
	}
	viaSnap := func(id, clause, note string, f func(*snapshot) *snapshot) mutation {
		return mutation{id: "snapshot/" + id, clause: clause, note: note, snap: f}
	}
	viaFramed := func(id, clause, note string, f func([]framed) []framed) mutation {
		return mutation{id: "snapshot/" + id, clause: clause, note: note, framed: f}
	}
	keep := func(m mutation) mutation { m.keepRoot = true; return m }
	verbatim := func(m mutation) mutation { m.verbatim = true; return m }
	withRecords := func(m mutation, f func([]record) []record) mutation { m.records = f; return m }

	cases := []mutation{
		// The preimage file: its format is untouched by the snapshot's changes.
		viaRawPre("trailing-byte", "preimages.no-trailing-bytes", "",
			func(b []byte) []byte { return append(bytes.Clone(b), 0x00) }),
		viaRawPre("truncated-record", "preimages.record-self-delimiting", "the file ends inside a slot key",
			func(b []byte) []byte { return bytes.Clone(b)[:endOfLastSlot(b)-1] }),
		viaRawPre("slot-count-huge", "preimages.record-self-delimiting", "the first record claims 2^32-1 slots",
			func(b []byte) []byte {
				out := bytes.Clone(b)
				binary.BigEndian.PutUint32(out[common.AddressLength:], 0xffffffff)
				return out
			}),
		verbatim(viaRecords("raw-address-order", "preimages.hashed-key-order", "sorted by raw address",
			func(recs []record) []record {
				slices.SortStableFunc(recs, func(x, y record) int { return bytes.Compare(x.addr[:], y.addr[:]) })
				return recs
			})),
		verbatim(viaRecords("raw-slot-order", "preimages.hashed-key-order", "slots sorted by number",
			func(recs []record) []record {
				for i := range recs {
					slices.SortStableFunc(recs[i].slots, func(x, y common.Hash) int { return bytes.Compare(x[:], y[:]) })
				}
				return recs
			})),
		verbatim(viaRecords("duplicate-address", "preimages.address-appears-once",
			"record 1 repeated in place; a strict-ascent check reports the equal key as out of order, which is the same clause read the other way",
			func(recs []record) []record { return slices.Insert(recs, 2, recs[1]) })),
		verbatim(viaRecords("duplicate-slot", "preimages.no-duplicate-slots",
			"storageSpread's second slot repeated in place",
			func(recs []record) []record {
				i := findRecord(recs, storageSpread)
				recs[i].slots = slices.Insert(recs[i].slots, 2, recs[i].slots[1])
				return recs
			})),
		verbatim(viaRecords("address-split", "preimages.address-appears-once",
			"storageSpread's record split into two, verbatim: a loader that merges by address accepts it",
			func(recs []record) []record {
				i := findRecord(recs, storageSpread)
				r := recs[i]
				mid := len(r.slots) / 2
				return slices.Replace(recs, i, i+1,
					record{addr: r.addr, slots: r.slots[:mid]},
					record{addr: r.addr, slots: r.slots[mid:]})
			})),
		viaRecords("missing-account", "verification.records-match-preimages", "", dropRecord(eoaBalance)),
		viaRecords("missing-header-slot", "verification.records-match-preimages", "slot 63 of storageSpread",
			func(recs []record) []record {
				i := findRecord(recs, storageSpread)
				recs[i].slots = slices.DeleteFunc(recs[i].slots, func(s common.Hash) bool { return s == h(63) })
				return recs
			}),
		viaRecords("missing-overflow-slot", "verification.records-match-preimages",
			"slot 256 of storageSpread; its stem is a one-way hash, so the leaf can never be keyed",
			func(recs []record) []record {
				i := findRecord(recs, storageSpread)
				recs[i].slots = slices.DeleteFunc(recs[i].slots, func(s common.Hash) bool { return s == h(256) })
				return recs
			}),
		viaRawPre("empty-file", "verification.records-match-preimages", "", func([]byte) []byte { return []byte{} }),
		viaRecords("slot-on-wrong-account", "verification.records-match-preimages",
			"storageSpread's slot 64 named under storageValues: the set is intact, the binding is not",
			func(recs []record) []record {
				i, j := findRecord(recs, storageSpread), findRecord(recs, storageValues)
				recs[i].slots = slices.DeleteFunc(recs[i].slots, func(s common.Hash) bool { return s == h(64) })
				recs[j].slots = append(recs[j].slots, h(64))
				return recs
			}),

		// The snapshot: framing.
		viaRawSnap("wrong-claimed-root", "verification.internal-consistency", "the trailer's last byte flipped",
			func(b []byte) []byte { out := bytes.Clone(b); out[len(out)-1] ^= 1; return out }),
		viaRawSnap("truncated-stream", "snapshot.record-self-delimiting", "the file ends inside the root",
			func(b []byte) []byte { return bytes.Clone(b)[:len(b)-1] }),
		viaRawSnap("trailing-garbage", "snapshot.no-trailing-bytes", "",
			func(b []byte) []byte { return append(bytes.Clone(b), 0xde, 0xad) }),
		viaRawSnap("missing-end", "snapshot.end-tag", "the file stops after the last record, with no end tag or root",
			func(b []byte) []byte { return bytes.Clone(b)[:len(b)-1-common.HashLength] }),
		viaRawSnap("truncated-record", "snapshot.record-self-delimiting", "the file ends inside the last storage group",
			func(b []byte) []byte { return bytes.Clone(b)[:len(b)-1-common.HashLength-1] }),
		viaFramed("unknown-tag", "snapshot.known-tag", "the first header record tagged 0x08, one past the end tag",
			func(c []framed) []framed { c[0].tag = 0x08; return c }),
		viaFramed("header-after-code", "snapshot.zone-order", "the last header record moved after the first code group",
			func(c []framed) []framed {
				last := indexTag(c, tagCodeGroup) - 1
				moved := c[last]
				c = slices.Delete(c, last, last+1)
				return slices.Insert(c, last+1, moved)
			}),
		viaFramed("code-after-storage", "snapshot.zone-order", "the last code group moved after the last storage group",
			func(c []framed) []framed {
				last := indexTag(c, tagStorageAccount) - 1
				moved := c[last]
				return append(slices.Delete(c, last, last+1), moved)
			}),
		viaFramed("orphan-storage-group", "snapshot.storage-group-follows-account",
			"the first storage account record dropped, so its groups follow the last code group",
			func(c []framed) []framed {
				i := indexTag(c, tagStorageAccount)
				return slices.Delete(c, i, i+1)
			}),
		viaSnap("empty-storage-account", "snapshot.storage-account-holds-group",
			"a storage account record for eoaBalance, which has no overflow storage; the leaves are the valid ones",
			func(s *snapshot) *snapshot {
				ah := addrHashOf(eoaBalance)
				s.storage = insertSorted(s.storage, storageRecord{addrHash: ah},
					func(a, b storageRecord) int { return bytes.Compare(a.addrHash[:], b.addrHash[:]) })
				return s
			}),

		// One leaf set, a second encoding. The tree and the dual-check both
		// agree with the valid file; only the byte rules tell them apart,
		// and they are what lets producers compare digests.
		viaFramed("single-group-as-multi", "snapshot.one-leaf-group-form", "a one-leaf storage group written with tag 0x06 and n 0",
			func(c []framed) []framed {
				i := indexTag(c, tagSingleGroup)
				b := c[i].body
				c[i] = framed{tagGroup, slices.Concat(b[:common.HashLength], []byte{0}, b[common.HashLength:])}
				return c
			}),
		viaSnap("storage-account-split", "snapshot.storage-record-ascending-order",
			"an account's storage groups split over two storage account records; same leaves, two records: a loader that checks only the derived leaf order accepts it",
			func(s *snapshot) *snapshot {
				i := firstMultiGroupStorage(s)
				r := s.storage[i]
				s.storage[i].groups = r.groups[:1]
				s.storage = slices.Insert(s.storage, i+1, storageRecord{addrHash: r.addrHash, groups: r.groups[1:]})
				return s
			}),
		viaSnap("code-stem-split", "snapshot.code-group-ascending-order",
			"a code group's chunks split over two records with the same stem; same leaves, two records: a loader that checks only the derived leaf order accepts it",
			func(s *snapshot) *snapshot {
				j := slices.IndexFunc(s.code, func(g group) bool { return len(g.entries) >= 2 })
				g := s.code[j]
				s.code[j].entries = g.entries[:1]
				s.code = slices.Insert(s.code, j+1, group{stem: g.stem, entries: g.entries[1:]})
				return s
			}),

		// Ordering and duplicates: the six MUST-ascend clauses EIP-8347 states
		// (header records, header slots, code groups, storage records,
		// storage groups, group entries), one order case and one duplicate
		// case each, plus code groups' own entries.
		keep(viaSnap("headers-out-of-order", "snapshot.header-ascending-order", "the first two header records swapped",
			func(s *snapshot) *snapshot { s.headers[0], s.headers[1] = s.headers[1], s.headers[0]; return s })),
		keep(viaSnap("header-duplicate", "snapshot.header-ascending-order", "the first header record repeated in place",
			func(s *snapshot) *snapshot {
				s.headers = slices.Insert(s.headers, 1, cloneHeader(s.headers[0]))
				return s
			})),
		keep(viaSnap("header-slots-out-of-order", "snapshot.header-slot-ascending-order", "",
			func(s *snapshot) *snapshot {
				h := &s.headers[firstMultiSlotHeader(s)]
				h.slots[0], h.slots[1] = h.slots[1], h.slots[0]
				return s
			})),
		keep(viaSnap("header-slot-duplicate", "snapshot.header-slot-ascending-order", "",
			func(s *snapshot) *snapshot {
				h := &s.headers[firstMultiSlotHeader(s)]
				h.slots = slices.Insert(h.slots, 1, h.slots[0])
				return s
			})),
		keep(viaSnap("code-groups-out-of-order", "snapshot.code-group-ascending-order", "",
			func(s *snapshot) *snapshot { s.code[0], s.code[1] = s.code[1], s.code[0]; return s })),
		keep(viaSnap("code-group-duplicate", "snapshot.code-group-ascending-order",
			"the first code group repeated in place: a loader that merges by key accepts it",
			func(s *snapshot) *snapshot { s.code = slices.Insert(s.code, 1, cloneGroup(s.code[0])); return s })),
		keep(viaSnap("storage-records-out-of-order", "snapshot.storage-record-ascending-order", "",
			func(s *snapshot) *snapshot { s.storage[0], s.storage[1] = s.storage[1], s.storage[0]; return s })),
		keep(viaSnap("storage-record-duplicate", "snapshot.storage-record-ascending-order",
			"the first storage account record repeated in place: a loader that merges by key accepts it",
			func(s *snapshot) *snapshot {
				s.storage = slices.Insert(s.storage, 1, cloneStorageRecord(s.storage[0]))
				return s
			})),
		keep(viaSnap("storage-groups-out-of-order", "snapshot.storage-group-ascending-order", "",
			func(s *snapshot) *snapshot {
				r := &s.storage[firstMultiGroupStorage(s)]
				r.groups[0], r.groups[1] = r.groups[1], r.groups[0]
				return s
			})),
		keep(viaSnap("storage-group-duplicate", "snapshot.storage-group-ascending-order", "",
			func(s *snapshot) *snapshot {
				r := &s.storage[firstMultiGroupStorage(s)]
				r.groups = slices.Insert(r.groups, 1, cloneGroup(r.groups[0]))
				return s
			})),
		keep(viaSnap("group-entries-out-of-order", "snapshot.group-entry-ascending-order", "a storage group's entries swapped",
			func(s *snapshot) *snapshot {
				i, j := firstMultiEntryStorageGroup(s)
				g := &s.storage[i].groups[j]
				g.entries[0], g.entries[1] = g.entries[1], g.entries[0]
				return s
			})),
		keep(viaSnap("group-entry-duplicate", "snapshot.group-entry-ascending-order", "a storage group's first entry repeated in place",
			func(s *snapshot) *snapshot {
				i, j := firstMultiEntryStorageGroup(s)
				g := &s.storage[i].groups[j]
				g.entries = slices.Insert(g.entries, 1, g.entries[0])
				return s
			})),
		keep(viaSnap("code-entries-out-of-order", "snapshot.group-entry-ascending-order", "a code group's entries swapped",
			func(s *snapshot) *snapshot {
				g := &s.code[firstMultiEntryCodeGroup(s)]
				g.entries[0], g.entries[1] = g.entries[1], g.entries[0]
				return s
			})),
		keep(viaSnap("code-entry-duplicate", "snapshot.group-entry-ascending-order", "a code group's first entry repeated in place",
			func(s *snapshot) *snapshot {
				g := &s.code[firstMultiEntryCodeGroup(s)]
				g.entries = slices.Insert(g.entries, 1, g.entries[0])
				return s
			})),

		// Kind, size and range rules on the header record's fields.
		viaSnap("kind0-empty-account", "snapshot.kind0-not-empty", "eoaBalance's nonce and balance cleared with kind 0 kept",
			func(s *snapshot) *snapshot {
				h := &s.headers[headerIdxOf(s, eoaBalance)]
				h.nonce, h.balance = nil, nil
				return s
			}),
		viaSnap("kind1-code-size-zero", "snapshot.code-size-nonzero",
			"eoaBalance rewritten as kind 1 with codeHash keccak('') and codeSize cleared: the leaf set and root are unchanged",
			func(s *snapshot) *snapshot {
				h := &s.headers[headerIdxOf(s, eoaBalance)]
				h.kind, h.codeHash, h.codeSize = kindCode, emptyCode, nil
				return s
			}),
		viaSnap("kind1-delegation-bytecode", "verification.code-limb-not-delegation",
			"delegatedA rewritten as kind 1 whose sole code chunk holds exactly the EIP-7702 indicator",
			func(s *snapshot) *snapshot {
				indicator := delegation(delegateTarget)
				ch := crypto.Keccak256Hash(indicator)
				h := &s.headers[headerIdxOf(s, delegatedA)]
				h.kind, h.codeHash, h.codeSize = kindCode, ch, minimal(uint64(len(indicator)))
				var chunk [32]byte
				copy(chunk[1:], indicator)
				key := bintrie.CodeChunkKey(ch, 0)
				s.code = insertSorted(s.code, group{
					stem:    common.Hash(key[1:33]),
					entries: []groupEntry{{sub: key[33], value: common.TrimLeftZeroes(chunk[:])}},
				}, func(a, b group) int { return bytes.Compare(a.stem[:], b.stem[:]) })
				return s
			}),
		viaSnap("slot-out-of-range", "snapshot.header-slot-below-64", "a header slot at sub-index 64",
			func(s *snapshot) *snapshot {
				h := &s.headers[headerIdxOf(s, eoaBalance)]
				h.slots = append(h.slots, groupEntry{sub: 64, value: []byte{1}})
				return s
			}),

		// Integer canonicality: a leading zero byte and a length above the
		// field's width, on a header field and a header-slot value.
		viaSnap("integer-leading-zero", "snapshot.integer-leading-zero", "eoaNonce's nonce given a leading zero byte",
			func(s *snapshot) *snapshot {
				h := &s.headers[headerIdxOf(s, eoaNonce)]
				h.nonce = append([]byte{0}, h.nonce...)
				return s
			}),
		keep(viaSnap("integer-too-long", "snapshot.integer-width", "eoaBalance's balance widened past the 16-byte field width",
			func(s *snapshot) *snapshot {
				h := &s.headers[headerIdxOf(s, eoaBalance)]
				h.balance = append(bytes.Repeat([]byte{1}, 17), h.balance...)
				return s
			})),
		keep(viaSnap("value-too-long", "snapshot.integer-width", "storageHeader's slot 7 widened to 33 bytes",
			func(s *snapshot) *snapshot {
				h := &s.headers[headerIdxOf(s, storageHeader)]
				h.slots[0].value = bytes.Repeat([]byte{1}, 33)
				return s
			})),

		// The snapshot: what the leaves say about the state.
		keep(viaLeaves("zero-value-present", "snapshot.no-zero-values",
			"slot 5 of storageSpread held as 32 zero bytes; no tree can commit to it, so the valid root stays",
			func(leaves []leaf) []leaf {
				return insertSorted(leaves, leaf{key: bintrie.HeaderKey(storageSpread, bintrie.HeaderStorageOffset+5), value: zero}, leafCmp)
			})),
		keep(viaLeaves("zero-code-chunk-present", "snapshot.no-zero-values",
			"the zero-valued middle chunk of codeZeroChunk's code, which the converter omits",
			func(leaves []leaf) []leaf {
				return insertSorted(leaves, leaf{key: bintrie.CodeChunkKey(zeroChunkHash, 1), value: zero}, leafCmp)
			})),
		keep(viaLeaves("zero-storage-entry-present", "snapshot.no-zero-values",
			"a zero-valued entry in storageSpread's overflow group at slot 257",
			func(leaves []leaf) []leaf {
				return insertSorted(leaves, leaf{key: bintrie.StorageSlotKey(storageSpread, h(257).Bytes()), value: zero}, leafCmp)
			})),
		viaLeaves("flipped-value", "verification.consensus-anchoring", "eoaBalance's balance, root recomputed",
			func(leaves []leaf) []leaf {
				leaves[findKey(leaves, bintrie.BasicDataKey(eoaBalance))].value[31] ^= 1
				return leaves
			}),
		viaLeaves("delegation-target-changed", "verification.consensus-anchoring",
			"one byte of delegatedA's target; the MPT code hash is keccak of the indicator, which check 2 must derive",
			func(leaves []leaf) []leaf {
				leaves[findKey(leaves, bintrie.DelegationKey(delegatedA))].value[22] ^= 1
				return leaves
			}),
		viaLeaves("wrong-code-size", "verification.code-limb", "codeChunkExact claims 7 bytes",
			func(leaves []leaf) []leaf {
				binary.BigEndian.PutUint32(leaves[findKey(leaves, bintrie.BasicDataKey(codeChunkExact))].value[4:8], 7)
				return leaves
			}),
		viaLeaves("missing-code-chunk", "verification.code-limb", "chunk 0 of the 31-byte JUMPDEST code",
			func(leaves []leaf) []leaf {
				i := findKey(leaves, bintrie.CodeChunkKey(codeHash, 0))
				return slices.Delete(leaves, i, i+1)
			}),
		viaLeaves("flipped-pushdata-byte", "verification.code-limb", "chunk 0 of the 31-byte JUMPDEST code, count 0 to 1",
			func(leaves []leaf) []leaf {
				leaves[findKey(leaves, bintrie.CodeChunkKey(codeHash, 0))].value[0] = 1
				return leaves
			}),
		viaLeaves("orphan-code-leaves", "verification.no-unreferenced-code", "chunks under a code hash no account holds",
			func(leaves []leaf) []leaf {
				return insertSorted(leaves, leaf{key: bintrie.CodeChunkKey(crypto.Keccak256Hash([]byte("orphan")), 0), value: one}, leafCmp)
			}),
		viaLeaves("chunk-beyond-code-size", "verification.code-limb", "a chunk at index 1 for 31 bytes of code",
			func(leaves []leaf) []leaf {
				return insertSorted(leaves, leaf{key: bintrie.CodeChunkKey(codeHash, 1), value: one}, leafCmp)
			}),
		viaSnap("shared-code-size-mismatch", "verification.code-limb",
			"sharedB's codeSize set to 62 while sharedA holds 93 for the same code hash: a verifier that takes one holder's size and skips the rest accepts it",
			func(s *snapshot) *snapshot {
				h := &s.headers[headerIdxOf(s, sharedB)]
				h.codeSize = minimal(62)
				return s
			}),
		viaLeaves("surplus-account-leaves", "verification.records-match-preimages", "an account with no preimage record",
			func(leaves []leaf) []leaf {
				leaves = insertSorted(leaves, leaf{key: bintrie.BasicDataKey(surplusAddr), value: basicData(0, 0, big.NewInt(1))}, leafCmp)
				return insertSorted(leaves, leaf{key: bintrie.CodeHashKey(surplusAddr), value: emptyCode}, leafCmp)
			}),
		viaLeaves("storage-leaf-missing", "verification.consensus-anchoring",
			"storageSpread's slot 64 leaf dropped while the preimages still name it; its storage root no longer matches",
			func(leaves []leaf) []leaf {
				i := findKey(leaves, bintrie.StorageSlotKey(storageSpread, h(64).Bytes()))
				return slices.Delete(leaves, i, i+1)
			}),
		viaLeaves("orphan-storage-leaf", "snapshot.storage-account-has-header",
			"a storage account record, holding one leaf, for an address with no header record",
			func(leaves []leaf) []leaf {
				addr := common.HexToAddress("0x00000000000000000000000000000000cafebabe")
				return insertSorted(leaves, leaf{key: bintrie.StorageSlotKey(addr, h(64).Bytes()), value: one}, leafCmp)
			}),
		viaLeaves("delegation-with-code-leaves", "verification.no-unreferenced-code",
			"chunks under keccak(indicator), whose value is the indicator chunk itself; the MPT code hash of delegatedA is that hash, so a code-hash-to-chunks mapping would accept them",
			func(leaves []leaf) []leaf {
				indicator := delegation(delegateTarget)
				var v [32]byte
				copy(v[1:], indicator)
				return insertSorted(leaves, leaf{key: bintrie.CodeChunkKey(crypto.Keccak256Hash(indicator), 0), value: v}, leafCmp)
			}),
		viaLeaves("header-slot-in-storage-zone", "embedding.header-holds-slots-0-63", "storageSpread's slot 0 keyed as overflow",
			func(leaves []leaf) []leaf {
				overflow := bytes.Clone(leaves[findKey(leaves, bintrie.StorageSlotKey(storageSpread, h(64).Bytes()))].key)
				overflow[len(overflow)-1] = 0
				i := findKey(leaves, bintrie.HeaderKey(storageSpread, bintrie.HeaderStorageOffset))
				moved := leaf{key: overflow, value: leaves[i].value}
				return insertSorted(slices.Delete(leaves, i, i+1), moved, leafCmp)
			}),
		viaLeaves("storage-leaf-unkeyable", "embedding.storage-key-derivation", "storageSpread's slot 512 keyed under tree_index 0",
			func(leaves []leaf) []leaf {
				group0 := bytes.Clone(leaves[findKey(leaves, bintrie.StorageSlotKey(storageSpread, h(64).Bytes()))].key)
				group0[len(group0)-1] = 200
				i := findKey(leaves, bintrie.StorageSlotKey(storageSpread, h(512).Bytes()))
				moved := leaf{key: group0, value: leaves[i].value}
				return insertSorted(slices.Delete(leaves, i, i+1), moved, leafCmp)
			}),
		withRecords(viaLeaves("anchored-elsewhere", "verification.consensus-anchoring",
			"eoaBalance removed from both files: internally consistent, but not the anchor state",
			func(leaves []leaf) []leaf {
				for _, key := range [][]byte{bintrie.BasicDataKey(eoaBalance), bintrie.CodeHashKey(eoaBalance)} {
					i := findKey(leaves, key)
					leaves = slices.Delete(leaves, i, i+1)
				}
				return leaves
			}),
			dropRecord(eoaBalance)),

		// The empty snapshot: no records under the empty-tree root. It
		// passes the dual-check only against the empty MPT root; the fixture
		// anchor holds accounts.
		viaRawSnap("empty-against-nonempty-anchor", "verification.consensus-anchoring",
			"no records and the empty-tree root, against the fixture's non-empty anchor",
			func([]byte) []byte { return frame(nil, common.Hash{}) }),
	}
	return cases
}

// endOfLastSlot is the offset just past the last slot key in the file, so a
// cut before it lands inside a slot rather than inside a slotless record.
func endOfLastSlot(b []byte) int {
	end, i := 0, 0
	for i < len(b) {
		n := int(binary.BigEndian.Uint32(b[i+common.AddressLength:]))
		i += preimageRecordHeaderSize + n*common.HashLength
		if n > 0 {
			end = i
		}
	}
	return end
}

func addrHashOf(addr common.Address) common.Hash {
	return common.Hash(bintrie.BasicDataKey(addr)[1:33])
}

func leafCmp(a, b leaf) int { return bytes.Compare(a.key, b.key) }

// insertSorted keeps s sorted by cmp, inserting v at the position it belongs.
func insertSorted[T any](s []T, v T, cmp func(T, T) int) []T {
	i, _ := slices.BinarySearchFunc(s, v, cmp)
	return slices.Insert(s, i, v)
}

func findKey(leaves []leaf, key []byte) int {
	if i := slices.IndexFunc(leaves, func(l leaf) bool { return bytes.Equal(l.key, key) }); i >= 0 {
		return i
	}
	panic(fmt.Sprintf("no leaf at key %x", key))
}

func findRecord(recs []record, addr common.Address) int {
	if i := slices.IndexFunc(recs, func(r record) bool { return r.addr == addr }); i >= 0 {
		return i
	}
	panic(fmt.Sprintf("no preimage record for %x", addr))
}

func headerIdxOf(s *snapshot, addr common.Address) int {
	ah := addrHashOf(addr)
	if i := slices.IndexFunc(s.headers, func(h header) bool { return h.addrHash == ah }); i >= 0 {
		return i
	}
	panic("no header for " + addr.Hex())
}

func firstMultiSlotHeader(s *snapshot) int {
	if i := slices.IndexFunc(s.headers, func(h header) bool { return len(h.slots) >= 2 }); i >= 0 {
		return i
	}
	panic("no header with 2+ slots")
}

func firstMultiGroupStorage(s *snapshot) int {
	if i := slices.IndexFunc(s.storage, func(r storageRecord) bool { return len(r.groups) >= 2 }); i >= 0 {
		return i
	}
	panic("no storage record with 2+ groups")
}

func firstMultiEntryStorageGroup(s *snapshot) (int, int) {
	for i, r := range s.storage {
		if j := slices.IndexFunc(r.groups, func(g group) bool { return len(g.entries) >= 2 }); j >= 0 {
			return i, j
		}
	}
	panic("no storage group with 2+ entries")
}

func firstMultiEntryCodeGroup(s *snapshot) int {
	if i := slices.IndexFunc(s.code, func(g group) bool { return len(g.entries) >= 2 }); i >= 0 {
		return i
	}
	panic("no code group with 2+ entries")
}

// indexTag is the first record with the tag.
func indexTag(c []framed, tag byte) int {
	if i := slices.IndexFunc(c, func(f framed) bool { return f.tag == tag }); i >= 0 {
		return i
	}
	panic("no record has the tag")
}
