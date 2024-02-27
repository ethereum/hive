package interfaces

import (
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/protolambda/zrnt/eth2/beacon/common"
)

type StateViewGenesis interface {
	common.BeaconState
	ForkVersion() common.Version
	PreviousForkVersion() common.Version
	EmptyBodyRoot() common.Root
	SetGenesisExecutionHeader(genesisBlock *types.Block) error
	ToJson() ([]byte, error)
}
