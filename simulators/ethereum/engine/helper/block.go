package helper

import (
	"encoding/json"
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
	TimeSecondsInFuture  uint64
}

func (m PoWBlockModifier) ModifyUnsealedBlock(baseBlock *types.Block) (*types.Block, error) {
	modifiedHeader := types.CopyHeader(baseBlock.Header())

	if m.Difficulty != nil {
		modifiedHeader.Difficulty = m.Difficulty
	}
	if m.RandomStateRoot {
		rand.Read(modifiedHeader.Root[:])
	}
	if m.TimeSecondsInFuture > 0 {
		modifiedHeader.Time += m.TimeSecondsInFuture
	}

	modifiedBlock := types.NewBlockWithHeader(modifiedHeader)
	modifiedBlock = modifiedBlock.WithBody(baseBlock.Transactions(), baseBlock.Uncles())

	js, _ := json.MarshalIndent(modifiedBlock.Header(), "", "  ")
	fmt.Printf("DEBUG: Modified unsealed block with hash %v:\n%s\n", modifiedBlock.Hash(), js)

	return modifiedBlock, nil
}

func (m PoWBlockModifier) ModifySealedBlock(f func(*types.Header) bool, baseBlock *types.Block) (*types.Block, error) {
	modifiedHeader := types.CopyHeader(baseBlock.Header())

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

	js, _ := json.MarshalIndent(modifiedBlock.Header(), "", "  ")
	fmt.Printf("DEBUG: Modified sealed block with hash %v:\n%s\n", modifiedBlock.Hash(), js)
	return modifiedBlock, nil
}

func GenerateInvalidPayloadBlock(baseBlock *types.Block, uncle *types.Block, payloadField InvalidPayloadBlockField) (*types.Block, error) {
	if payloadField == InvalidOmmers {
		if uncle == nil {
			return nil, fmt.Errorf("No ommer provided")
		}
		uncles := []*types.Header{
			uncle.Header(),
		}
		newHeader := types.CopyHeader(baseBlock.Header())
		newHeader.UncleHash = types.CalcUncleHash(uncles)

		modifiedBlock := types.NewBlockWithHeader(newHeader).WithBody(baseBlock.Transactions(), uncles)
		fmt.Printf("DEBUG: hash=%s, ommerLen=%d, ommersHash=%v\n", modifiedBlock.Hash(), len(modifiedBlock.Uncles()), modifiedBlock.UncleHash())
		return modifiedBlock, nil
	}

	return baseBlock, nil
}
