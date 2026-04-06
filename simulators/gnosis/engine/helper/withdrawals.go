package helper

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/trie"
)

var (
	EmptyWithdrawalsRootHash = &types.EmptyRootHash
)

func ComputeWithdrawalsRoot(ws types.Withdrawals) common.Hash {
	// Using RLP root but might change to ssz
	return types.DeriveSha(
		ws,
		trie.NewStackTrie(nil),
	)
}
