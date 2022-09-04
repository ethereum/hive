package helper

import (
	"encoding/json"
	"fmt"
	"math/big"
	"math/rand"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
)

type BlockModifier interface {
	ModifyUnsealedBlock(consensus.ChainHeaderReader, *state.StateDB, *types.Block) (*types.Block, error)
	ModifySealedBlock(func(*types.Header) bool, *types.Block) (*types.Block, error)
}

type PoWBlockModifier struct {
	// Modify the difficulty of the block by a given amount, block will be correctly sealed
	Difficulty *big.Int
	// Increase the balance of a random account by 1 wei
	BalanceIncrease bool
	// Randomize the state root value
	RandomStateRoot bool
	// Invalidate the sealed mixHash of the block according to ethash consensus
	InvalidSealedMixHash bool
	// Invalidate the sealed nonce of the block according to ethash consensus
	InvalidSealedNonce bool
	// Modify the timestamp by a given amount of seconds
	TimeSecondsInFuture uint64
}

func (m PoWBlockModifier) ModifyUnsealedBlock(chain consensus.ChainHeaderReader, state *state.StateDB, baseBlock *types.Block) (*types.Block, error) {
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
	if m.BalanceIncrease {
		var acc common.Address
		rand.Read(acc[:])
		state.AddBalance(acc, common.Big1)
		modifiedHeader.Root = state.IntermediateRoot(chain.Config().IsEIP158(modifiedHeader.Number))
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

func GenerateInvalidPayloadBlock(baseBlock *types.Block, uncle *types.Block, payloadField InvalidityType) (*types.Block, error) {
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
