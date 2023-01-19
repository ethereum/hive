package pow

import (
	"github.com/ethereum/go-ethereum/consensus"
	"github.com/ethereum/go-ethereum/consensus/ethash"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	execution_config "github.com/ethereum/hive/simulators/eth2/common/config/execution"
)

var Defaults = ethash.Config{
	PowMode:        ethash.ModeNormal,
	CachesInMem:    2,
	DatasetsOnDisk: 2,
	DatasetDir:     "/ethash",
}

type ChainGenerator struct {
	BlockCount int
	ethash.Config
	GenFunction func(int, *core.BlockGen)
	blocks      []*types.Block
}

// instaSeal wraps a consensus engine with instant block sealing. When a block is produced
// using FinalizeAndAssemble, it also applies Seal.
type instaSeal struct{ consensus.Engine }

// FinalizeAndAssemble implements consensus.Engine, accumulating the block and uncle rewards,
// setting the final state and assembling the block.
func (e instaSeal) FinalizeAndAssemble(
	chain consensus.ChainHeaderReader,
	header *types.Header,
	state *state.StateDB,
	txs []*types.Transaction,
	uncles []*types.Header,
	receipts []*types.Receipt,
	withdrawals []*types.Withdrawal,
) (*types.Block, error) {
	block, err := e.Engine.FinalizeAndAssemble(
		chain,
		header,
		state,
		txs,
		uncles,
		receipts,
		withdrawals,
	)
	if err != nil {
		return nil, err
	}
	sealedBlock := make(chan *types.Block, 1)
	if err = e.Engine.Seal(nil, block, sealedBlock, nil); err != nil {
		return nil, err
	}
	return <-sealedBlock, nil
}

func (p *ChainGenerator) Generate(
	genesis *execution_config.ExecutionGenesis,
) ([]*types.Block, error) {
	// We generate a new chain only if the generator had not generated one already.
	// This is done because the chain generators can be reused on different clients to ensure
	// they start with the same chain.
	if p.blocks != nil {
		return p.blocks, nil
	}
	engine := ethash.New(p.Config, nil, false)
	insta := instaSeal{engine}
	_, p.blocks, _ = core.GenerateChainWithGenesis(
		genesis.Genesis,
		insta,
		p.BlockCount,
		p.GenFunction,
	)
	return p.blocks, nil
}

func (p *ChainGenerator) Blocks() []*types.Block {
	return p.blocks
}

func (p *ChainGenerator) Head() *types.Block {
	if p.blocks == nil || len(p.blocks) == 0 {
		return nil
	}
	return p.blocks[len(p.blocks)-1]
}
