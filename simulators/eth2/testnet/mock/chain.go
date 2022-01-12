package mock

import (
	"fmt"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus"
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

type MockClient struct {
	t      *hivesim.T
	chain  *core.BlockChain
	db     ethdb.Database
	engine consensus.Engine
	gspec  *core.Genesis

	peers            []*Conn
	cancelKeepAlives []chan struct{}
}

func NewMockClient(t *hivesim.T, genesis *core.Genesis) (*MockClient, error) {
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
		return nil, err
	}

	bc, err := core.NewBlockChain(db, nil, genesis.Config, engine, vm.Config{}, nil, nil)
	if err != nil {
		return nil, err
	}

	return &MockClient{
		t:      t,
		chain:  bc,
		db:     db,
		engine: engine,
		gspec:  genesis,
	}, nil
}

// Mines a proof-of-work chain, announcing each block to all peers. `handle` is
// called after each block is mined to perform test-specific operations and
// notify the miner if it should continue.
func (m *MockClient) MineChain(handle func(*MockClient, *types.Block) (bool, error)) error {
	for {
		parent := m.chain.CurrentHeader()
		block, err := m.MineBlock(parent)
		if block == nil {
			m.t.Logf("mock: Failed to mine block: %s", err)
			continue
		}
		if err != nil {
			return fmt.Errorf("error mining block: %v", err)
		}
		stop, err := handle(m, block)
		if err != nil {
			return err
		}
		if stop {
			return nil
		}
	}
}

func (m *MockClient) Peer(addr string) error {
	n, err := enode.Parse(enode.ValidSchemes, addr)
	if err != nil {
		return fmt.Errorf("malformatted enode address (%q): %v", addr, err)
	}
	peer, err := Dial(n)
	if err != nil {
		return fmt.Errorf("unable to connect to client: %v", err)
	}
	if err := peer.Peer(m.chain, nil); err != nil {
		return fmt.Errorf("unable to peer with client: %v", err)
	}
	m.t.Logf("mock: Simulator connected to eth1 client successfully")

	// Keep peer connection alive until after the transition
	cancel := make(chan struct{})
	go peer.KeepAlive(cancel)

	m.peers = append(m.peers, peer)
	m.cancelKeepAlives = append(m.cancelKeepAlives, cancel)

	return nil
}

func (m *MockClient) MineBlock(parent *types.Header) (*types.Block, error) {
	header := &types.Header{
		ParentHash: parent.Hash(),
		Number:     new(big.Int).Add(parent.Number, common.Big1),
		GasLimit:   parent.GasLimit,
		Time:       uint64(time.Now().Unix()),
	}

	config := m.chain.Config()
	if config.IsLondon(header.Number) {
		header.BaseFee = misc.CalcBaseFee(config, parent)
		// At the transition, double the gas limit so the gas target is equal to the old gas limit.
		if !config.IsLondon(parent.Number) {
			header.GasLimit = parent.GasLimit * params.ElasticityMultiplier
		}
	}

	// Calculate difficulty
	if err := m.engine.Prepare(m.chain, header); err != nil {
		return nil, fmt.Errorf("failed to prepare header for mining: %v", err)
	}

	// Finalize block
	statedb, err := state.New(parent.Root, state.NewDatabase(m.db), nil)
	if err != nil {
		panic(err)
	}
	block, err := m.engine.FinalizeAndAssemble(m.chain, header, statedb, nil, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to finalize and assemble block: %v", err)
	}

	// Seal block
	results := make(chan *types.Block)
	if err := m.engine.Seal(m.chain, block, results, nil); err != nil {
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
	_, err = m.chain.InsertChain(types.Blocks{block})
	if err != nil {
		return nil, err
	}

	return block, nil
}

func (m *MockClient) AnnounceBlock(block *types.Block) error {
	for _, peer := range m.peers {
		newBlock := eth.NewBlockPacket{Block: block, TD: m.TotalDifficulty()}
		if err := peer.Write66(&newBlock, 23); err != nil {
			return fmt.Errorf("failed to msg peer: %v", err)
		}
	}
	return nil
}

func (m *MockClient) TotalDifficulty() *big.Int {
	return m.chain.GetTd(m.chain.CurrentHeader().ParentHash, m.chain.CurrentHeader().Number.Uint64())
}

func (m *MockClient) IsTerminalTotalDifficulty() bool {
	td := m.TotalDifficulty()
	ttd := new(big.Int).SetUint64(m.gspec.Config.TerminalTotalDifficulty.Uint64())
	return td.Cmp(ttd) >= 0
}

func (m *MockClient) Close() error {
	err := m.engine.Close()
	if err != nil {
		m.t.Fatalf("failed closing consensus engine: %s", err)
	}
	err = m.db.Close()
	if err != nil {
		m.t.Fatalf("failed closing db: %s", err)
	}
	for _, cancel := range m.cancelKeepAlives {
		close(cancel)
	}
	return nil
}
