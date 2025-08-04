package chaingenerators

import (
	"github.com/ethereum/go-ethereum/core/types"
	el "github.com/ethereum/hive/simulators/eth2/common/config/execution"
)

type ChainGenerator interface {
	Generate(*el.ExecutionGenesis) ([]*types.Block, error)
}
