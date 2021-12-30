package mock

import (
	"context"
	"fmt"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus/ethash"
	"github.com/ethereum/go-ethereum/consensus/misc"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/eth/protocols/eth"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/p2p/enode"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/hive/hivesim"
)

func MineChain(t *hivesim.T, ctx context.Context, genesis *core.Genesis, addr string) {
	ethashCfg := ethash.Config{
		PowMode:        ethash.ModeNormal,
		DatasetDir:     "",
		CacheDir:       "",
		DatasetsInMem:  1,
		DatasetsOnDisk: 2,
		CachesInMem:    2,
		CachesOnDisk:   3,
	}
	engine := ethash.New(ethashCfg, nil, false)

	db := rawdb.NewMemoryDatabase()

	_, err := genesis.Commit(db)
	if err != nil {
		panic(err)
	}

	bc, err := core.NewBlockChain(db, nil, genesis.Config, engine, vm.Config{}, nil, nil)
	if err != nil {
		panic(err)
	}

	// Dial the peer to feed the POW blocks to
	n, err := enode.Parse(enode.ValidSchemes, addr)
	if err != nil {
		panic(fmt.Errorf("malformatted enode address (%q): %v", addr, err))
	}
	peer, err := Dial(n)
	if err != nil {
		panic(fmt.Errorf("unable to connect to client: %v", err))
	}
	if err := peer.Peer(bc, nil); err != nil {
		panic(fmt.Errorf("unable to peer with client: %v", err))
	}
	ctx, cancelPeer := context.WithCancel(ctx)
	defer cancelPeer()
	t.Logf("simulator connected to eth1 client successfully")

	// Keep peer connection alive until after the transition
	go peer.KeepAlive(ctx)

	defer bc.Stop()
	defer engine.Close()

	// Send pre-transition blocks
	for {
		parent := bc.CurrentHeader()

		t.Logf("mining new block")
		block, err := mineBlock(db, bc, engine, parent)
		if block == nil {
			t.Logf("Failed to mine block: %s", err)
			continue
		}
		if err != nil {
			panic(fmt.Errorf("failed to mine block: %v", err))
		}

		// announce block
		t.Logf("announcing mined block\thash=%s", block.Hash())
		td := bc.GetTd(bc.CurrentHeader().ParentHash, bc.CurrentHeader().Number.Uint64())
		newBlock := eth.NewBlockPacket{Block: block, TD: td}
		if err := peer.Write66(&newBlock, 23); err != nil {
			panic(fmt.Errorf("failed to msg peer: %v", err))
		}

		// check if terminal total difficulty is reached
		ttd := new(big.Int).SetUint64(genesis.Config.TerminalTotalDifficulty.Uint64())
		t.Logf("Comparing TD to terminal TD\ttd: %d, ttd: %d", td, ttd)
		if td.Cmp(ttd) >= 0 {
			t.Logf("Terminal total difficulty reached, transitioning to POS")
			break
			// return bc.CurrentHeader().Number.Uint64(), nil
		}
	}
}

func mineBlock(db ethdb.Database, bc *core.BlockChain, engine *ethash.Ethash, parent *types.Header) (*types.Block, error) {
	header := &types.Header{
		ParentHash: parent.Hash(),
		Number:     new(big.Int).Add(parent.Number, common.Big1),
		GasLimit:   parent.GasLimit,
		Time:       uint64(time.Now().Unix()),
	}

	config := bc.Config()
	if config.IsLondon(header.Number) {
		header.BaseFee = misc.CalcBaseFee(config, parent)
		// At the transition, double the gas limit so the gas target is equal to the old gas limit.
		if !config.IsLondon(parent.Number) {
			header.GasLimit = parent.GasLimit * params.ElasticityMultiplier
		}
	}

	// Calculate difficulty
	if err := engine.Prepare(bc, header); err != nil {
		return nil, fmt.Errorf("failed to prepare header for mining: %v", err)
	}

	// Finalize block
	statedb, err := state.New(parent.Root, state.NewDatabase(db), nil)
	if err != nil {
		panic(err)
	}
	block, err := engine.FinalizeAndAssemble(bc, header, statedb, nil, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to finalize and assemble block: %v", err)
	}

	// Seal block
	results := make(chan *types.Block)
	if err := engine.Seal(bc, block, results, nil); err != nil {
		panic(fmt.Sprintf("failed to seal block: %v", err))
	}
	select {
	case found := <-results:
		block = found
	case <-time.NewTimer(10000 * time.Second).C:
		return nil, fmt.Errorf("sealing result timeout")
	}

	// Write state changes to db
	root, err := statedb.Commit(config.IsEIP158(header.Number))
	if err != nil {
		return nil, fmt.Errorf("state write error: %v", err)
	}
	if err := statedb.Database().TrieDB().Commit(root, false, nil); err != nil {
		return nil, fmt.Errorf("trie write error: %v", err)
	}

	// Insert block into chain
	_, err = bc.InsertChain(types.Blocks{block})
	if err != nil {
		return nil, err
	}

	return block, nil
}
