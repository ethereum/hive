//go:build pbtgen

package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math/big"
	"slices"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/trie/bintrie"
)

// deriveLeaves is the leaf set EIP-8297 implies for the allocation. Only key
// derivation is shared with the reference converter; values, chunking and
// presence are independent, so a converter bug there cannot reach the
// fixtures.
func deriveLeaves(alloc types.GenesisAlloc) []leaf {
	var (
		leaves []leaf
		seen   = map[common.Hash]bool{}
		add    = func(key []byte, value [32]byte) {
			if value != ([32]byte{}) {
				leaves = append(leaves, leaf{key: key, value: value})
			}
		}
	)
	for addr, acct := range alloc {
		code := acct.Code
		delegated := len(code) == 23 && bytes.HasPrefix(code, []byte{0xef, 0x01, 0x00})

		add(bintrie.BasicDataKey(addr), basicData(uint32(len(code)), acct.Nonce, acct.Balance))
		if delegated {
			var v [32]byte
			copy(v[:], code)
			add(bintrie.DelegationKey(addr), v)
		} else {
			add(bintrie.CodeHashKey(addr), crypto.Keccak256Hash(code))
			if h := crypto.Keccak256Hash(code); len(code) > 0 && !seen[h] {
				seen[h] = true
				for i, c := range chunkify(code) {
					add(bintrie.CodeChunkKey(h, uint64(i)), c)
				}
			}
		}
		for slot, value := range acct.Storage {
			add(bintrie.StorageSlotKey(addr, slot[:]), value)
		}
	}
	slices.SortFunc(leaves, func(a, b leaf) int { return bytes.Compare(a.key, b.key) })
	return leaves
}

func basicData(codeSize uint32, nonce uint64, balance *big.Int) [32]byte {
	var v [32]byte
	binary.BigEndian.PutUint32(v[4:8], codeSize)
	binary.BigEndian.PutUint64(v[8:16], nonce)
	if balance != nil {
		balance.FillBytes(v[16:32])
	}
	return v
}

// chunkify is EIP-8297's chunkify_code, transliterated.
func chunkify(code []byte) [][32]byte {
	if len(code)%31 != 0 {
		code = append(slices.Clone(code), make([]byte, 31-len(code)%31)...)
	}
	execData := make([]int, len(code)+32)
	for pos := 0; pos < len(code); {
		pushdata := 0
		if code[pos] >= 0x60 && code[pos] <= 0x7f {
			pushdata = int(code[pos]) - 0x5f
		}
		pos++
		for x := range pushdata {
			execData[pos+x] = pushdata - x
		}
		pos += pushdata
	}
	var chunks [][32]byte
	for pos := 0; pos < len(code); pos += 31 {
		var c [32]byte
		c[0] = byte(min(execData[pos], 31))
		copy(c[1:], code[pos:pos+31])
		chunks = append(chunks, c)
	}
	return chunks
}

func checkLeaves(got, want []leaf) error {
	for i := 0; i < len(got) || i < len(want); i++ {
		switch {
		case i >= len(got):
			return fmt.Errorf("converter omits leaf %x", want[i].key)
		case i >= len(want):
			return fmt.Errorf("converter emits unexpected leaf %x", got[i].key)
		case !bytes.Equal(got[i].key, want[i].key):
			return fmt.Errorf("leaf %d: converter key %x, derived %x", i, got[i].key, want[i].key)
		case got[i].value != want[i].value:
			return fmt.Errorf("leaf %x: converter value %x, derived %x", got[i].key, got[i].value, want[i].value)
		}
	}
	return nil
}
