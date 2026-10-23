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
)

const preimageRecordHeaderSize = common.AddressLength + 4

// Widths of the snapshot's x[≤w] fields.
const (
	nonceWidth    = 8
	balanceWidth  = 16
	codeSizeWidth = 4
	valueWidth    = 32
)

// Record tags. A header record's tag is its account kind.
const (
	kindNoCode        = 0x00
	kindCode          = 0x01
	kindDelegation    = 0x02
	tagCodeGroup      = 0x03
	tagStorageAccount = 0x04
	tagSingleGroup    = 0x05
	tagGroup          = 0x06
	tagEnd            = 0x07
)

// snapshot is EIP-8347's snapshot as written. Every x[≤w] field keeps the
// bytes it was written with, so a case can carry an encoding the rules forbid.
type snapshot struct {
	root    common.Hash
	headers []header
	code    []group
	storage []storageRecord
}

type header struct {
	addrHash common.Hash
	nonce    []byte
	balance  []byte
	kind     byte           // the record's tag
	codeHash common.Hash    // kind 1
	codeSize []byte         // kind 1
	target   common.Address // kind 2
	slots    []groupEntry   // sub is the slot number
}

type group struct {
	stem    common.Hash
	entries []groupEntry
}

type groupEntry struct {
	sub   byte
	value []byte
}

type storageRecord struct {
	addrHash common.Hash
	groups   []group
}

// framed is one tagged record: its tag and the bytes after it. A case that
// breaks the framing edits the list encode joins.
type framed struct {
	tag  byte
	body []byte
}

// ruleError is a strict decoder's rejection, naming the clause it breaks.
type ruleError struct{ clause, msg string }

func (e *ruleError) Error() string { return e.clause + ": " + e.msg }

// records lays the snapshot out as tagged records, in zone order.
func (s *snapshot) records() []framed {
	var out []framed
	for _, h := range s.headers {
		out = append(out, framed{h.kind, h.body()})
	}
	for _, g := range s.code {
		out = append(out, framed{tagCodeGroup, g.body()})
	}
	for _, r := range s.storage {
		out = append(out, framed{tagStorageAccount, slices.Clone(r.addrHash[:])})
		for _, g := range r.groups {
			if len(g.entries) == 1 {
				out = append(out, framed{tagSingleGroup, g.singleBody()})
			} else {
				out = append(out, framed{tagGroup, g.body()})
			}
		}
	}
	return out
}

func (s *snapshot) encode() []byte { return frame(s.records(), s.root) }

// frame joins the records, then the end tag and the root.
func frame(recs []framed, root common.Hash) []byte {
	var b []byte
	for _, f := range recs {
		b = append(append(b, f.tag), f.body...)
	}
	return append(append(b, tagEnd), root[:]...)
}

func (h *header) body() []byte {
	b := slices.Clone(h.addrHash[:])
	b = appendVar(b, h.nonce)
	b = appendVar(b, h.balance)
	switch h.kind {
	case kindCode:
		b = append(b, h.codeHash[:]...)
		b = appendVar(b, h.codeSize)
	case kindDelegation:
		b = append(b, h.target[:]...)
	}
	b = append(b, byte(len(h.slots)))
	return appendEntries(b, h.slots)
}

func (g *group) body() []byte {
	if len(g.entries) == 0 || len(g.entries) > 256 {
		panic(fmt.Sprintf("group %x holds %d entries, which n cannot count", g.stem, len(g.entries)))
	}
	b := append(slices.Clone(g.stem[:]), byte(len(g.entries)-1))
	return appendEntries(b, g.entries)
}

// singleBody is a one-leaf group, which carries no n byte.
func (g *group) singleBody() []byte {
	return appendEntries(slices.Clone(g.stem[:]), g.entries)
}

func appendEntries(b []byte, entries []groupEntry) []byte {
	for _, e := range entries {
		b = append(b, e.sub)
		b = appendVar(b, e.value)
	}
	return b
}

func appendVar(b, x []byte) []byte { return append(append(b, byte(len(x))), x...) }

func minimal(n uint64) []byte {
	return common.TrimLeftZeroes(binary.BigEndian.AppendUint64(nil, n))
}

// leaves derives the PBT leaves per EIP-8347's leaf derivation, in the order
// the records give them. A basic-data leaf that packs to zero is absent, as
// EIP-8297 has every zero-valued key; values the file carries are kept even
// when zero, so a case holding one still shows it.
func (s *snapshot) leaves() []leaf {
	var out []leaf
	for _, h := range s.headers {
		key := func(sub byte) []byte {
			return append(append([]byte{0x00}, h.addrHash[:]...), sub)
		}
		var size uint32
		switch h.kind {
		case kindCode:
			size = uint32(new(big.Int).SetBytes(h.codeSize).Uint64())
		case kindDelegation:
			size = 23
		}
		if bd := basicData(size, new(big.Int).SetBytes(h.nonce).Uint64(), new(big.Int).SetBytes(h.balance)); bd != ([32]byte{}) {
			out = append(out, leaf{key: key(0), value: bd})
		}
		switch h.kind {
		case kindNoCode:
			out = append(out, leaf{key: key(1), value: [32]byte(crypto.Keccak256(nil))})
		case kindCode:
			out = append(out, leaf{key: key(1), value: h.codeHash})
		case kindDelegation:
			var v [32]byte
			copy(v[:], delegation(h.target))
			out = append(out, leaf{key: key(2), value: v})
		}
		for _, e := range h.slots {
			out = append(out, leaf{key: key(64 + e.sub), value: pad(e.value)})
		}
	}
	for _, g := range s.code {
		for _, e := range g.entries {
			out = append(out, leaf{key: slices.Concat([]byte{0x01}, g.stem[:], []byte{e.sub}), value: pad(e.value)})
		}
	}
	for _, r := range s.storage {
		for _, g := range r.groups {
			for _, e := range g.entries {
				out = append(out, leaf{key: slices.Concat([]byte{0xff}, r.addrHash[:], g.stem[:], []byte{e.sub}), value: pad(e.value)})
			}
		}
	}
	return out
}

func pad(v []byte) [32]byte {
	var out [32]byte
	copy(out[32-len(v):], v)
	return out
}

// fromLeaves writes a leaf set as records. It fails on a set that no snapshot
// can carry: a leaf the derivation would not give back, such as a version
// byte or a header sub-index outside the fields a record has.
func fromLeaves(root common.Hash, leaves []leaf) (*snapshot, error) {
	s := &snapshot{root: root}
	emptyCode := common.Hash(crypto.Keccak256(nil))
	for i := 0; i < len(leaves); {
		k := leaves[i].key
		switch {
		case len(k) == 34 && k[0] == 0x00:
			h := header{addrHash: common.Hash(k[1:33])}
			var basic [32]byte
			var codeHash, deleg *[32]byte
			for ; i < len(leaves) && len(leaves[i].key) == 34 && bytes.Equal(leaves[i].key[:33], k[:33]); i++ {
				v := leaves[i].value
				switch sub := leaves[i].key[33]; {
				case sub == 0:
					basic = v
				case sub == 1:
					codeHash = &v
				case sub == 2:
					deleg = &v
				case sub >= 64 && sub < 128:
					h.slots = append(h.slots, groupEntry{sub: sub - 64, value: common.TrimLeftZeroes(v[:])})
				}
			}
			size := binary.BigEndian.Uint32(basic[4:8])
			h.nonce = common.TrimLeftZeroes(basic[8:16])
			h.balance = common.TrimLeftZeroes(basic[16:32])
			switch {
			case deleg != nil:
				h.kind = kindDelegation
				h.target = common.Address(deleg[3:23])
			case codeHash != nil && *codeHash == emptyCode && size == 0:
				h.kind = kindNoCode
			case codeHash != nil:
				h.kind = kindCode
				h.codeHash = *codeHash
				h.codeSize = minimal(uint64(size))
			}
			s.headers = append(s.headers, h)
		case len(k) == 34 && k[0] == 0x01:
			var g group
			i, g = takeGroup(leaves, i, 1)
			s.code = append(s.code, g)
		case len(k) == 66 && k[0] == 0xff:
			r := storageRecord{addrHash: common.Hash(k[1:33])}
			for i < len(leaves) && len(leaves[i].key) == 66 && bytes.Equal(leaves[i].key[:33], k[:33]) {
				var g group
				i, g = takeGroup(leaves, i, 33)
				r.groups = append(r.groups, g)
			}
			s.storage = append(s.storage, r)
		default:
			return nil, fmt.Errorf("leaf %x is in no zone a snapshot has", k)
		}
	}
	got := s.leaves()
	for i := range max(len(got), len(leaves)) {
		switch {
		case i >= len(got):
			return nil, fmt.Errorf("the records drop leaf %x", leaves[i].key)
		case i >= len(leaves):
			return nil, fmt.Errorf("the records add leaf %x", got[i].key)
		case !bytes.Equal(got[i].key, leaves[i].key) || got[i].value != leaves[i].value:
			return nil, fmt.Errorf("the records do not carry leaf %x as given", leaves[i].key)
		}
	}
	return s, nil
}

// takeGroup reads the leaves from i on that share the stem ending at
// key[at+32], at being where the stem starts.
func takeGroup(leaves []leaf, i, at int) (int, group) {
	first := leaves[i].key
	g := group{stem: common.Hash(first[at : at+32])}
	for ; i < len(leaves) && len(leaves[i].key) == at+33 && bytes.Equal(leaves[i].key[:at+32], first[:at+32]); i++ {
		v := leaves[i].value
		g.entries = append(g.entries, groupEntry{sub: leaves[i].key[at+32], value: common.TrimLeftZeroes(v[:])})
	}
	return i, g
}

// decodeSnapshot parses a snapshot. Strict enforces every byte rule EIP-8347
// states; lenient reads what the layout allows and returns the records read
// with the error that stopped it.
func decodeSnapshot(blob []byte, strict bool) (*snapshot, error) {
	r := &reader{b: blob, strict: strict}
	s := &snapshot{}
	var (
		zone    int       // 0 header records, 1 code groups, 2 storage records
		open    = -1      // the storage account whose groups follow
		prev    [3][]byte // the last key read in each zone
		prevGrp []byte    // the last group stem of the open account
	)
	for r.err == nil {
		if r.off == len(blob) {
			r.fail("snapshot.end-tag", "the records end without the end tag")
			break
		}
		switch tag := r.u8(); {
		case tag <= kindDelegation:
			r.enter(&zone, 0, "header record")
			h := r.header(tag)
			r.ascending(&prev[0], h.addrHash[:], "snapshot.header-ascending-order", "header record")
			if r.err == nil {
				s.headers = append(s.headers, h)
			}
		case tag == tagCodeGroup:
			r.enter(&zone, 1, "code group")
			g := r.group(false)
			r.ascending(&prev[1], g.stem[:], "snapshot.code-group-ascending-order", "code group")
			if r.err == nil {
				s.code = append(s.code, g)
			}
		case tag == tagStorageAccount:
			r.enter(&zone, 2, "storage account")
			r.close(s, open)
			addr := common.Hash(r.take(32))
			r.ascending(&prev[2], addr[:], "snapshot.storage-record-ascending-order", "storage account")
			if r.strict && r.err == nil && !slices.ContainsFunc(s.headers, func(h header) bool { return h.addrHash == addr }) {
				r.fail("snapshot.storage-account-has-header", "storage account %x has no header record", addr)
			}
			if r.err == nil {
				s.storage = append(s.storage, storageRecord{addrHash: addr})
				open, prevGrp = len(s.storage)-1, nil
			}
		case tag == tagSingleGroup || tag == tagGroup:
			if open < 0 {
				r.fail("snapshot.storage-group-follows-account", "storage group with no storage account before it")
				break
			}
			g := r.group(tag == tagSingleGroup)
			if r.strict && r.err == nil && tag == tagGroup && len(g.entries) == 1 {
				r.fail("snapshot.one-leaf-group-form", "one-leaf storage group %x written with tag %#x", g.stem, tagGroup)
			}
			r.ascending(&prevGrp, g.stem[:], "snapshot.storage-group-ascending-order", "storage group")
			if r.err == nil {
				s.storage[open].groups = append(s.storage[open].groups, g)
			}
		case tag == tagEnd:
			r.close(s, open)
			s.root = common.Hash(r.take(32))
			if r.err == nil && r.off != len(blob) {
				r.fail("snapshot.no-trailing-bytes", "%d bytes after the root", len(blob)-r.off)
			}
			return s, r.err
		default:
			r.fail("snapshot.known-tag", "unknown tag %#x", tag)
		}
	}
	return s, r.err
}

// reader walks a snapshot, stopping at the first error.
type reader struct {
	b      []byte
	off    int
	strict bool
	err    error
}

func (r *reader) fail(clause, format string, args ...any) {
	if r.err == nil {
		r.err = &ruleError{clause, fmt.Sprintf("offset %d: ", r.off) + fmt.Sprintf(format, args...)}
	}
}

func (r *reader) take(n int) []byte {
	if r.err != nil {
		return make([]byte, n)
	}
	if len(r.b)-r.off < n {
		r.fail("snapshot.record-self-delimiting", "truncated: %d bytes wanted, %d remain", n, len(r.b)-r.off)
		return make([]byte, n)
	}
	r.off += n
	return r.b[r.off-n : r.off]
}

func (r *reader) u8() byte { return r.take(1)[0] }

// enter moves to a record's zone. Going back to an earlier zone breaks zone
// order; a lenient read carries on, so the record still shows in a diff.
func (r *reader) enter(zone *int, to int, what string) {
	switch {
	case to > *zone:
		*zone = to
	case to < *zone && r.strict:
		r.fail("snapshot.zone-order", "%s after a later zone's records", what)
	}
}

// close ends the open storage account, which must hold a group.
func (r *reader) close(s *snapshot, open int) {
	if r.strict && r.err == nil && open >= 0 && len(s.storage[open].groups) == 0 {
		r.fail("snapshot.storage-account-holds-group", "storage account %x holds no group", s.storage[open].addrHash)
	}
}

// integer reads an x[≤w] field. A length above w stops either mode, since
// the field then fits nowhere in a leaf.
func (r *reader) integer(name string, w int) []byte {
	n := int(r.u8())
	if n > w {
		r.fail("snapshot.integer-width", "%s is %d bytes, above %d", name, n, w)
		return nil
	}
	x := r.take(n)
	if r.strict && n > 0 && x[0] == 0 {
		r.fail("snapshot.integer-leading-zero", "%s %x has a leading zero byte", name, x)
	}
	return x
}

func (r *reader) header(kind byte) header {
	h := header{kind: kind, addrHash: common.Hash(r.take(32))}
	h.nonce = r.integer("nonce", nonceWidth)
	h.balance = r.integer("balance", balanceWidth)
	switch kind {
	case kindNoCode:
		if r.strict && len(h.nonce) == 0 && len(h.balance) == 0 {
			r.fail("snapshot.kind0-not-empty", "kind 0 with zero nonce and balance is an empty account")
		}
	case kindCode:
		h.codeHash = common.Hash(r.take(32))
		h.codeSize = r.integer("codeSize", codeSizeWidth)
		if r.strict && len(h.codeSize) == 0 {
			r.fail("snapshot.code-size-nonzero", "kind 1 with codeSize 0")
		}
	case kindDelegation:
		h.target = common.Address(r.take(20))
	}
	n := int(r.u8())
	for range n {
		e := groupEntry{sub: r.u8(), value: r.integer("value", valueWidth)}
		switch {
		case r.err != nil:
		case e.sub >= 192 || r.strict && e.sub >= 64:
			r.fail("snapshot.header-slot-below-64", "header slot %d is not below 64", e.sub)
		case r.strict && len(h.slots) > 0 && e.sub <= h.slots[len(h.slots)-1].sub:
			r.fail("snapshot.header-slot-ascending-order", "header slot %d does not ascend", e.sub)
		case r.strict && len(e.value) == 0:
			r.fail("snapshot.no-zero-values", "header slot %d holds zero", e.sub)
		}
		h.slots = append(h.slots, e)
	}
	return h
}

// group reads a group; a single one has no n byte and holds one leaf.
func (r *reader) group(single bool) group {
	g := group{stem: common.Hash(r.take(32))}
	n := 1
	if !single {
		n = int(r.u8()) + 1
	}
	for range n {
		e := groupEntry{sub: r.u8(), value: r.integer("value", valueWidth)}
		switch {
		case r.err != nil:
		case r.strict && len(g.entries) > 0 && e.sub <= g.entries[len(g.entries)-1].sub:
			r.fail("snapshot.group-entry-ascending-order", "group entry %d does not ascend", e.sub)
		case r.strict && len(e.value) == 0:
			r.fail("snapshot.no-zero-values", "group entry %d holds zero", e.sub)
		}
		g.entries = append(g.entries, e)
	}
	return g
}

// ascending holds a level's keys strictly ascending, in strict mode.
func (r *reader) ascending(prev *[]byte, key []byte, clause, what string) {
	if r.err == nil && r.strict && *prev != nil && bytes.Compare(*prev, key) >= 0 {
		r.fail(clause, "%s %x does not ascend", what, key)
	}
	*prev = key
}

// decodePreimages returns the records read before the error that stopped it.
// Strict also holds addresses and slots to keccak order with no repeats.
func decodePreimages(blob []byte, strict bool) ([]record, error) {
	var recs []record
	fail := func(clause, format string, args ...any) ([]record, error) {
		return recs, &ruleError{clause, fmt.Sprintf("record %d: ", len(recs)) + fmt.Sprintf(format, args...)}
	}
	for len(blob) > 0 {
		if len(blob) < preimageRecordHeaderSize {
			return fail("preimages.no-trailing-bytes", "%d bytes remain, short of a record header", len(blob))
		}
		addr := common.BytesToAddress(blob[:common.AddressLength])
		count := int(binary.BigEndian.Uint32(blob[common.AddressLength:preimageRecordHeaderSize]))
		blob = blob[preimageRecordHeaderSize:]
		if len(blob) < count*common.HashLength {
			return fail("preimages.record-self-delimiting", "%x claims %d slots, %d bytes remain", addr, count, len(blob))
		}
		if strict && len(recs) > 0 {
			switch byKeccak(recs[len(recs)-1].addr[:], addr[:]) {
			case 0:
				return fail("preimages.address-appears-once", "address %x repeats the previous record's", addr)
			case 1:
				return fail("preimages.hashed-key-order", "address %x sorts before the previous record's", addr)
			}
		}
		slots := make([]common.Hash, count)
		for i := range slots {
			slots[i] = common.BytesToHash(blob[:common.HashLength])
			blob = blob[common.HashLength:]
			if strict && i > 0 {
				switch byKeccak(slots[i-1][:], slots[i][:]) {
				case 0:
					return fail("preimages.no-duplicate-slots", "slot %x repeats", slots[i])
				case 1:
					return fail("preimages.hashed-key-order", "slot %x sorts before the previous slot", slots[i])
				}
			}
		}
		recs = append(recs, record{addr: addr, slots: slots})
	}
	return recs, nil
}

// encodePreimages writes records in keccak order.
func encodePreimages(recs []record) []byte {
	slices.SortFunc(recs, func(a, b record) int { return byKeccak(a.addr[:], b.addr[:]) })
	for _, r := range recs {
		slices.SortFunc(r.slots, func(a, b common.Hash) int { return byKeccak(a[:], b[:]) })
	}
	return encodeRecordsVerbatim(recs)
}

// encodeRecordsVerbatim writes records in the order given.
func encodeRecordsVerbatim(recs []record) []byte {
	var b []byte
	for _, r := range recs {
		b = binary.BigEndian.AppendUint32(append(b, r.addr[:]...), uint32(len(r.slots)))
		for _, s := range r.slots {
			b = append(b, s[:]...)
		}
	}
	return b
}

func byKeccak(a, b []byte) int { return bytes.Compare(crypto.Keccak256(a), crypto.Keccak256(b)) }
