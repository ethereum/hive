package main

import (
	"context"
	"fmt"
	"math/big"
	"sync"
	"time"

	ethcommon "github.com/ethereum/go-ethereum/common"
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
	"github.com/ethereum/hive/simulators/eth2/testnet/setup"
	"github.com/protolambda/eth2api"
	"github.com/protolambda/eth2api/client/beaconapi"
	"github.com/protolambda/zrnt/eth2/beacon/common"
)

type Testnet struct {
	t *hivesim.T

	genesisTime           common.Timestamp
	genesisValidatorsRoot common.Root

	// Consensus chain configuration
	spec *common.Spec
	// Execution chain configuration and genesis info
	eth1Genesis *setup.Eth1Genesis

	beacons    []*BeaconNode
	validators []*ValidatorClient
	eth1       []*Eth1Node
}

func (t *Testnet) GenesisTime() time.Time {
	return time.Unix(int64(t.genesisTime), 0)
}

func (t *Testnet) TrackFinality(ctx context.Context) {
	genesis := t.GenesisTime()
	slotDuration := time.Duration(t.spec.SECONDS_PER_SLOT) * time.Second
	timer := time.NewTicker(slotDuration)

	for {
		select {
		case <-ctx.Done():
			return
		case tim := <-timer.C:
			// start polling after first slot of genesis
			if tim.Before(genesis.Add(slotDuration)) {
				t.t.Logf("time till genesis: %s", genesis.Sub(tim))
				continue
			}

			// new slot, log and check status of all beacon nodes

			var wg sync.WaitGroup
			for i, b := range t.beacons {
				wg.Add(1)
				go func(ctx context.Context, i int, b *BeaconNode) {
					defer wg.Done()
					ctx, _ = context.WithTimeout(ctx, time.Second*5)

					var headInfo eth2api.BeaconBlockHeaderAndInfo
					if exists, err := beaconapi.BlockHeader(ctx, b.API, eth2api.BlockHead, &headInfo); err != nil {
						t.t.Errorf("[beacon %d] failed to poll head: %v", i, err)
						return
					} else if !exists {
						t.t.Fatalf("[beacon %d] no head block", i)
					}

					var out eth2api.FinalityCheckpoints
					if exists, err := beaconapi.FinalityCheckpoints(ctx, b.API, eth2api.StateIdRoot(headInfo.Header.Message.StateRoot), &out); err != nil {
						t.t.Errorf("[beacon %d] failed to poll finality checkpoint: %v", i, err)
						return
					} else if !exists {
						t.t.Fatalf("[beacon %d] Expected state for head block", i)
					}

					slot := headInfo.Header.Message.Slot
					t.t.Logf("beacon %d: head block root %s, slot %d, justified %s, finalized %s",
						i, headInfo.Root, slot, &out.CurrentJustified, &out.Finalized)

					if ep := t.spec.SlotToEpoch(slot); ep > out.Finalized.Epoch+2 {
						t.t.Errorf("failing to finalize, head slot %d (epoch %d) is more than 2 ahead of finality checkpoint %d", slot, ep, out.Finalized.Epoch)
					}
				}(ctx, i, b)
			}
			wg.Wait()
		}
	}
}

func (t *Testnet) MineChain(ctx context.Context) {
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

	_, err := t.eth1Genesis.Genesis.Commit(db)
	if err != nil {
		panic(err)
	}

	bc, err := core.NewBlockChain(db, nil, t.eth1Genesis.Genesis.Config, engine, vm.Config{}, nil, nil)
	if err != nil {
		panic(err)
	}

	addr, err := t.eth1[0].EnodeURL()
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
	t.t.Logf("simulator connected to eth1 client successfully")

	// Keep peer connection alive until after the transition
	go peer.KeepAlive(ctx)

	defer bc.Stop()
	defer engine.Close()

	// Send pre-transition blocks
	for {
		parent := bc.CurrentHeader()

		t.t.Logf("mining new block")
		block, err := t.mineBlock(db, bc, engine, parent)
		if err != nil {
			panic(fmt.Errorf("failed to mine block: %v", err))
		}

		// announce block
		t.t.Logf("announcing mined block\thash=%s", block.Hash())
		td := bc.GetTd(bc.CurrentHeader().ParentHash, bc.CurrentHeader().Number.Uint64())
		newBlock := eth.NewBlockPacket{Block: block, TD: td}
		if err := peer.Write66(&newBlock, 23); err != nil {
			panic(fmt.Errorf("failed to msg peer: %v", err))
		}

		// check if terminal total difficulty is reached
		ttd := new(big.Int).SetUint64(t.eth1Genesis.Genesis.Config.TerminalTotalDifficulty.Uint64())
		t.t.Logf("Comparing TD to terminal TD\ttd: %d, ttd: %d", td, ttd)
		if td.Cmp(ttd) >= 0 {
			t.t.Logf("Terminal total difficulty reached, transitioning to POS")
			break
			// return bc.CurrentHeader().Number.Uint64(), nil
		}
	}
}

func (t *Testnet) mineBlock(db ethdb.Database, bc *core.BlockChain, engine *ethash.Ethash, parent *types.Header) (*types.Block, error) {
	header := &types.Header{
		ParentHash: parent.Hash(),
		Number:     new(big.Int).Add(parent.Number, ethcommon.Big1),
		GasLimit:   parent.GasLimit,
		Time:       parent.Time + 1,
	}

	config := t.eth1Genesis.Genesis.Config
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
		return nil, fmt.Errorf("failed to insert block into chain")
	}

	return block, nil
}
