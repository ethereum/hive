package main

import (
	"github.com/ethereum/go-ethereum/consensus"
	"github.com/ethereum/go-ethereum/consensus/ethash"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/hive/simulators/eth2/engine/setup"
)

type ChainGenerator interface {
	Generate(*setup.Eth1Genesis) ([]*types.Block, error)
}

var PoWChainGeneratorDefaults = ethash.Config{
	PowMode:        ethash.ModeNormal,
	CachesInMem:    2,
	DatasetsOnDisk: 2,
	DatasetDir:     "/ethash",
}

type PoWChainGenerator struct {
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
func (e instaSeal) FinalizeAndAssemble(chain consensus.ChainHeaderReader, header *types.Header, state *state.StateDB, txs []*types.Transaction, uncles []*types.Header, receipts []*types.Receipt) (*types.Block, error) {
	block, err := e.Engine.FinalizeAndAssemble(chain, header, state, txs, uncles, receipts)
	if err != nil {
		return nil, err
	}
	sealedBlock := make(chan *types.Block, 1)
	if err = e.Engine.Seal(nil, block, sealedBlock, nil); err != nil {
		return nil, err
	}
	return <-sealedBlock, nil
}

func (p *PoWChainGenerator) Generate(genesis *setup.Eth1Genesis) ([]*types.Block, error) {
	// We generate a new chain only if the generator had not generated one already.
	// This is done because the chain generators can be reused on different clients to ensure
	// they start with the same chain.
	if p.blocks != nil {
		return p.blocks, nil
	}
	db := rawdb.NewMemoryDatabase()
	engine := ethash.New(p.Config, nil, false)
	insta := instaSeal{engine}
	genesisBlock := genesis.Genesis.ToBlock(db)
	p.blocks, _ = core.GenerateChain(genesis.Genesis.Config, genesisBlock, insta, db, p.BlockCount, p.GenFunction)
	return p.blocks, nil
}
