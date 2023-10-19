package main

import (
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/trie"
)

func init() {
	register("uncles", func() blockModifier {
		return &modUncles{
			info: make(map[uint64]unclesInfo),
		}
	})
}

type modUncles struct {
	info    map[uint64]unclesInfo
	counter int
}

type unclesInfo struct {
	Hashes []common.Hash `json:"hashes"`
}

func (m *modUncles) apply(ctx *genBlockContext) bool {
	merge := ctx.ChainConfig().MergeNetsplitBlock
	if merge != nil && merge.Cmp(ctx.Number()) <= 0 {
		return false // no uncles after merge
	}
	if ctx.NumberU64() < 3 {
		return false
	}
	info := m.info[ctx.NumberU64()]
	if len(info.Hashes) >= 2 {
		return false
	}

	parent := ctx.ParentBlock()
	uncle := &types.Header{
		Number:     parent.Number(),
		ParentHash: parent.ParentHash(),
		Extra:      []byte(fmt.Sprintf("hivechain uncle %d", m.counter)),
	}
	ub := types.NewBlock(uncle, nil, nil, nil, trie.NewStackTrie(nil))
	uncle = ub.Header()
	ctx.block.AddUncle(uncle)
	info.Hashes = append(info.Hashes, uncle.Hash())
	m.info[ctx.NumberU64()] = info
	m.counter++
	return true
}

func (m *modUncles) txInfo() any {
	return m.info
}
