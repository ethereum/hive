package helper

import (
	"fmt"
	"math/big"
	"math/rand"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

type BlockModifier interface {
	ModifyUnsealedBlock(*types.Block) (*types.Block, error)
	ModifySealedBlock(func(*types.Header) bool, *types.Block) (*types.Block, error)
}

type PoWBlockModifier struct {
	Difficulty           *big.Int
	RandomStateRoot      bool
	InvalidSealedMixHash bool
	InvalidSealedNonce   bool
}

func (m PoWBlockModifier) ModifyUnsealedBlock(baseBlock *types.Block) (*types.Block, error) {
	modifiedHeader := types.CopyHeader(baseBlock.Header())

	if m.Difficulty != nil {
		modifiedHeader.Difficulty = m.Difficulty
	}
	if m.RandomStateRoot {
		rand.Read(modifiedHeader.Root[:])
	}

	modifiedBlock := types.NewBlockWithHeader(modifiedHeader)
	modifiedBlock = modifiedBlock.WithBody(baseBlock.Transactions(), baseBlock.Uncles())

	return modifiedBlock, nil
}

func (m PoWBlockModifier) ModifySealedBlock(f func(*types.Header) bool, baseBlock *types.Block) (*types.Block, error) {
	modifiedHeader := types.CopyHeader(baseBlock.Header())

	// Set the extra to be able to identify this block
	modifiedHeader.Extra = []byte("modified block")

	if m.InvalidSealedMixHash {
		modifiedHeader.MixDigest = common.Hash{}
		for f(modifiedHeader) {
			// Increase the hash until it's not valid
			modifiedHeader.MixDigest[len(modifiedHeader.MixDigest[:])]++
		}
	}
	if m.InvalidSealedNonce {
		modifiedHeader.Nonce = types.BlockNonce{}
		for f(modifiedHeader) {
			// Increase the nonce until it's not valid
			modifiedHeader.Nonce[len(modifiedHeader.Nonce[:])]++
		}
	}

	modifiedBlock := types.NewBlockWithHeader(modifiedHeader)
	modifiedBlock = modifiedBlock.WithBody(baseBlock.Transactions(), baseBlock.Uncles())

	fmt.Printf("DEBUG: Modified block with hash %v\n", modifiedBlock.Hash())
	return modifiedBlock, nil
}
