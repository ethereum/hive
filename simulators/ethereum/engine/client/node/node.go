package node

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"math/big"
	"os"
	"time"

	"github.com/ethereum/go-ethereum/cmd/utils"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/beacon"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/eth"
	ethcatalyst "github.com/ethereum/go-ethereum/eth/catalyst"
	"github.com/ethereum/go-ethereum/eth/downloader"
	"github.com/ethereum/go-ethereum/eth/ethconfig"
	ethprotocol "github.com/ethereum/go-ethereum/eth/protocols/eth"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/miner"
	"github.com/ethereum/go-ethereum/node"
	"github.com/ethereum/go-ethereum/p2p"
	"github.com/ethereum/go-ethereum/p2p/enode"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/ethereum/hive/hivesim"
	"github.com/ethereum/hive/simulators/ethereum/engine/client"
	"github.com/ethereum/hive/simulators/ethereum/engine/globals"
	"github.com/ethereum/hive/simulators/ethereum/engine/helper"
)

type GethNodeTestConfiguration struct {
	// Node name
	Name string

	// The node mines proof of work blocks
	PoWMiner          bool
	PoWMinerEtherBase common.Address

	// In PoW production mode, produce many terminal blocks which shall be gossiped
	TerminalBlockSiblingCount *big.Int
	TerminalBlockSiblingDepth *big.Int

	// Of all terminal blocks produced, the index of the one to send as terminal block when queried
	NthTerminalBlockToReturnAsHead *big.Int

	// To allow testing terminal block gossiping on the other clients, restrict this client
	// to only connect to the boot node
	MaxPeers *big.Int

	// Max number of gossiped blocks to receive
	ExpectedGossipNewBlocksCount *big.Int

	// Chain to import
	ChainFile string

	// How many seconds to delay PoW mining start
	MiningStartDelaySeconds *big.Int
}
type GethNodeEngineStarter struct {
	// Client parameters used to launch the default client
	ChainFile               string
	TerminalTotalDifficulty *big.Int

	// Test specific configuration
	Config GethNodeTestConfiguration
}

var (
	DefaultMaxPeers                  = big.NewInt(25)
	DefaultMiningStartDelaySeconds   = big.NewInt(10)
	DefaultTerminalBlockSiblingCount = big.NewInt(1)
	DefaultTerminalBlockSiblingDepth = big.NewInt(1)
)

func (s GethNodeEngineStarter) StartClient(T *hivesim.T, testContext context.Context, ClientParams hivesim.Params, ClientFiles hivesim.Params, bootClient client.EngineClient) (client.EngineClient, error) {
	return s.StartGethNode(T, testContext, ClientParams, ClientFiles, bootClient)
}

func (s GethNodeEngineStarter) StartGethNode(T *hivesim.T, testContext context.Context, ClientParams hivesim.Params, ClientFiles hivesim.Params, bootClient client.EngineClient) (*GethNode, error) {
	var (
		ttd      = s.TerminalTotalDifficulty
		bootnode string
		err      error
	)
	genesisPath, ok := ClientFiles["/genesis.json"]
	if !ok {
		return nil, fmt.Errorf("Cannot start without genesis file")
	}
	genesis := helper.LoadGenesis(genesisPath)

	if ttd == nil {
		if ttdStr, ok := ClientParams["HIVE_TERMINAL_TOTAL_DIFFICULTY"]; ok {
			// Retrieve TTD from parameters
			ttd, ok = new(big.Int).SetString(ttdStr, 10)
			if !ok {
				return nil, fmt.Errorf("Unable to parse TTD from parameters")
			}
		}
	} else {
		ttd = big.NewInt(helper.CalculateRealTTD(genesisPath, ttd.Int64()))
		ClientParams = ClientParams.Set("HIVE_TERMINAL_TOTAL_DIFFICULTY", fmt.Sprintf("%d", ttd))
	}
	genesis.Config.TerminalTotalDifficulty = ttd

	if bootClient != nil {
		bootnode, err = bootClient.EnodeURL()
		if err != nil {
			return nil, fmt.Errorf("Unable to obtain bootnode: %v", err)
		}
	}

	if s.Config.MaxPeers == nil {
		s.Config.MaxPeers = DefaultMaxPeers
	}

	if s.ChainFile != "" {
		s.Config.ChainFile = "./chains/" + s.ChainFile
	}

	if s.Config.PoWMiner && s.Config.PoWMinerEtherBase == (common.Address{}) {
		s.Config.PoWMinerEtherBase = common.HexToAddress("0x" + globals.MinerAddrHex)
	}

	if s.Config.MiningStartDelaySeconds == nil {
		s.Config.MiningStartDelaySeconds = DefaultMiningStartDelaySeconds
	}

	if s.Config.TerminalBlockSiblingCount == nil {
		s.Config.TerminalBlockSiblingCount = DefaultTerminalBlockSiblingCount
	}

	if s.Config.TerminalBlockSiblingDepth == nil {
		s.Config.TerminalBlockSiblingDepth = DefaultTerminalBlockSiblingDepth
	}

	g, err := newNode(s.Config, bootnode, &genesis)
	if err != nil {
		return nil, err
	}

	go func(ctx context.Context) {
		select {
		case <-ctx.Done():
			// Close the node when the context is done
			g.Close()
		}
	}(testContext)

	return g, nil
}

type AccountTransactionInfo struct {
	PreviousBlock common.Hash
	PreviousNonce uint64
}
type GethNode struct {
	node     *node.Node
	eth      *eth.Ethereum
	datadir  string
	bootnode string
	genesis  *core.Genesis
	ttd      *big.Int
	api      *ethcatalyst.ConsensusAPI
	running  context.Context
	closing  context.CancelFunc

	// Engine updates info
	latestFcUStateSent *beacon.ForkchoiceStateV1
	latestPAttrSent    *beacon.PayloadAttributesV1
	latestFcUResponse  *beacon.ForkChoiceResponse

	latestPayloadSent          *beacon.ExecutableDataV1
	latestPayloadStatusReponse *beacon.PayloadStatusV1

	// Test specific configuration
	accTxInfoMap           map[common.Address]*AccountTransactionInfo
	totalReceivedNewBlocks *big.Int
	terminalBlocksMined    *big.Int

	config GethNodeTestConfiguration
}

func newNode(config GethNodeTestConfiguration, bootnode string, genesis *core.Genesis) (*GethNode, error) {
	// Define the basic configurations for the Ethereum node
	datadir, _ := os.MkdirTemp("", "")

	return restart(config, bootnode, datadir, genesis)
}

func restart(startConfig GethNodeTestConfiguration, bootnode, datadir string, genesis *core.Genesis) (*GethNode, error) {
	if startConfig.Name == "" {
		startConfig.Name = "Modified Geth Module"
	}
	config := &node.Config{
		Name:    startConfig.Name,
		Version: params.Version,
		DataDir: datadir,
		P2P: p2p.Config{
			ListenAddr:  "0.0.0.0:0",
			NoDiscovery: true,
			MaxPeers:    int(startConfig.MaxPeers.Int64()),
		},
		UseLightweightKDF: true,
	}
	// Create the node and configure a full Ethereum node on it
	stack, err := node.New(config)
	if err != nil {
		return nil, err
	}
	ethHashConfig := ethconfig.Defaults.Ethash
	minerConfig := ethconfig.Defaults.Miner
	if startConfig.PoWMiner {
		ethHashConfig.CacheDir = "/ethash"
		ethHashConfig.DatasetDir = ethHashConfig.CacheDir
		minerConfig = miner.Config{
			GasFloor:  genesis.GasLimit * 9 / 10,
			GasCeil:   genesis.GasLimit * 11 / 10,
			GasPrice:  big.NewInt(1),
			Recommit:  1 * time.Second, // Disable the recommit
			Etherbase: startConfig.PoWMinerEtherBase,
		}
	}
	econfig := &ethconfig.Config{
		Genesis:          genesis,
		NetworkId:        genesis.Config.ChainID.Uint64(),
		SyncMode:         downloader.FullSync,
		DatabaseCache:    256,
		DatabaseHandles:  256,
		TxPool:           core.DefaultTxPoolConfig,
		GPO:              ethconfig.Defaults.GPO,
		Ethash:           ethHashConfig,
		Miner:            minerConfig,
		LightServ:        100,
		LightPeers:       int(startConfig.MaxPeers.Int64()) - 1,
		LightNoSyncServe: true,
	}
	ethBackend, err := eth.New(stack, econfig)
	if err != nil {
		return nil, err
	}
	if startConfig.ChainFile != "" {
		err = utils.ImportChain(ethBackend.BlockChain(), startConfig.ChainFile)
		if err != nil {
			return nil, err
		}
	}

	err = stack.Start()
	for stack.Server().NodeInfo().Ports.Listener == 0 {
		time.Sleep(250 * time.Millisecond)
	}
	// Connect the node to the bootnode
	node := enode.MustParse(bootnode)
	stack.Server().AddPeer(node)

	stack.Server().EnableMsgEvents = true

	g := &GethNode{
		node:         stack,
		eth:          ethBackend,
		datadir:      datadir,
		bootnode:     bootnode,
		genesis:      genesis,
		ttd:          genesis.Config.TerminalTotalDifficulty,
		api:          ethcatalyst.NewConsensusAPI(ethBackend),
		accTxInfoMap: make(map[common.Address]*AccountTransactionInfo),
		// Test related configuration
		totalReceivedNewBlocks: big.NewInt(0),
		terminalBlocksMined:    big.NewInt(0),
		config:                 startConfig,
	}

	g.running, g.closing = context.WithCancel(context.Background())
	if startConfig.PoWMiner {
		// Enable mining
		go g.EnablePoWMining()
	}

	// Start thread to monitor the amount of gossiped blocks this node receives
	go g.SubscribeP2PEvents()

	return g, err
}

func (n *GethNode) isBlockTerminal(b *types.Block) (bool, *big.Int, error) {
	ctx, cancel := context.WithTimeout(n.running, globals.RPCTimeout)
	defer cancel()
	parentTD, err := n.GetBlockTotalDifficulty(ctx, b.ParentHash())
	if err != nil {
		return false, nil, err
	}
	blockTD := new(big.Int).Add(parentTD, b.Difficulty())
	return blockTD.Cmp(n.ttd) >= 0 && parentTD.Cmp(n.ttd) < 0, blockTD, nil
}

func (n *GethNode) EnablePoWMining() {
	// Delay mining start
	select {
	case <-time.After(time.Second * time.Duration(n.config.MiningStartDelaySeconds.Int64())):
		n.eth.Miner().SetEtherbase(n.config.PoWMinerEtherBase)
		err := n.eth.StartMining(1)
		if err != nil {
			n.Close()
			return
		}
	case <-n.running.Done():
		return
	}

	// Start PoW Processing
	go n.StartPoWMiningProcessing()

}

func (n *GethNode) StartPoWMiningProcessing() {
	// Subscribe and process all mined blocks
	minedBlockSub := n.node.EventMux().Subscribe(core.NewMinedBlockEvent{}).Chan()
	for {
		select {
		case obj := <-minedBlockSub:
			if obj != nil {
				if ev, ok := obj.Data.(core.NewMinedBlockEvent); ok {
					n.ProcessNewMinedPoWBlock(ev.Block)
				}
			}
		case <-n.running.Done():
			return
		}
	}
}

func (n *GethNode) ProcessNewMinedPoWBlock(b *types.Block) {
	if t, td, err := n.isBlockTerminal(b); t {
		fmt.Printf("DEBUG (%s): Mined a New Terminal Block: hash=%v, td=%d, ttd=%d\n", n.config.Name, b.Hash(), td, n.ttd)
		n.terminalBlocksMined.Add(n.terminalBlocksMined, common.Big1)
		if n.config.TerminalBlockSiblingCount.Cmp(n.terminalBlocksMined) > 0 {
			// Shuffle the extra bytes of the miner before re-org to force having a different sealhash
			newExtra := make([]byte, 8)
			rand.Read(newExtra)
			n.eth.Miner().SetExtra(newExtra)
			// Reset the canonical chain to the parent of the terminal block to keep the miner mining.
			if _, err := n.ReOrgBackBlockChain(n.config.TerminalBlockSiblingDepth.Uint64(), b); err != nil {
				panic(err)
			}
		}
	} else if err == nil {
		fmt.Printf("DEBUG (%s): Mined a New Block: hash=%v, td=%d, ttd=%d\n", n.config.Name, b.Hash(), td, n.ttd)
	} else if err != nil {
		fmt.Printf("DEBUG (%s): ERROR during terminal block calc=%v\n", n.config.Name, err)
	}
}

// Sets the canonical block to an ancestor of the provided block.
// `N==1` means to re-org to the parent of the provided block.
func (n *GethNode) ReOrgBackBlockChain(N uint64, currentBlock *types.Block) (common.Hash, error) {
	if N == 0 {
		return currentBlock.Hash(), nil
	}
	for ; N > 0; N-- {
		currentBlock = n.eth.BlockChain().GetBlockByHash(currentBlock.ParentHash())
		if currentBlock == nil {
			return common.Hash{}, fmt.Errorf("Unable to re-org back")
		}
	}
	return n.eth.BlockChain().SetCanonical(currentBlock)
}

func (n *GethNode) SubscribeP2PEvents() {
	eventChan := make(chan *p2p.PeerEvent)
	n.node.Server().SubscribeEvents(eventChan)
	for {
		select {
		case event := <-eventChan:
			var msgCode uint64
			if event.MsgCode != nil {
				msgCode = *event.MsgCode
			}
			if event.Type == p2p.PeerEventTypeMsgRecv && msgCode == ethprotocol.NewBlockMsg {
				n.totalReceivedNewBlocks.Add(n.totalReceivedNewBlocks, common.Big1)
				fmt.Printf("DEBUG (%s): Received new block: Peer=%s, MsgCode=%d, Type=%s, Protocol=%s, RemoteAddress=%s\n", n.config.Name, event.Peer.String(), msgCode, event.Type, event.Protocol, event.RemoteAddress)
			}
		case <-n.running.Done():
			return
		}
	}
}

func (n *GethNode) Close() error {
	select {
	case <-n.running.Done():
		// Node already closed
		return nil
	default:
	}
	defer n.closing()
	err := n.node.Close()
	if err != nil {
		return err
	}
	os.RemoveAll(n.datadir)
	return nil
}

type validator struct{}

func (v *validator) ValidateBody(block *types.Block) error {
	return nil
}
func (v *validator) ValidateState(block *types.Block, state *state.StateDB, receipts types.Receipts, usedGas uint64) error {
	return nil
}

type processor struct{}

func (p *processor) Process(block *types.Block, statedb *state.StateDB, cfg vm.Config) (types.Receipts, []*types.Log, uint64, error) {
	return types.Receipts{}, []*types.Log{}, 21000, nil
}

var headerPrefix = []byte("h") // headerPrefix + num (uint64 big endian) + hash -> header
func headerKey(number uint64, hash common.Hash) []byte {
	return append(append(headerPrefix, encodeBlockNumber(number)...), hash.Bytes()...)
}

func encodeBlockNumber(number uint64) []byte {
	enc := make([]byte, 8)
	binary.BigEndian.PutUint64(enc, number)
	return enc
}

func (n *GethNode) SetBlock(block *types.Block, parentNumber uint64, parentRoot common.Hash) error {
	parentTd := n.eth.BlockChain().GetTd(block.ParentHash(), block.NumberU64()-1)
	rawdb.WriteTd(n.eth.ChainDb(), block.Hash(), block.NumberU64(), parentTd.Add(parentTd, block.Difficulty()))
	rawdb.WriteBlock(n.eth.ChainDb(), block)

	// write real info (fixes fake number test)
	data, err := rlp.EncodeToBytes(block.Header())
	if err != nil {
		log.Crit("Failed to RLP encode header", "err", err)
	}
	key := headerKey(parentNumber+1, block.Hash())
	if err := n.eth.ChainDb().Put(key, data); err != nil {
		log.Crit("Failed to store header", "err", err)
	}

	rawdb.WriteHeaderNumber(n.eth.ChainDb(), block.Hash(), block.NumberU64())
	bc := n.eth.BlockChain()
	bc.SetBlockValidatorAndProcessorForTesting(new(validator), n.eth.BlockChain().Processor())

	statedb, err := state.New(parentRoot, bc.StateCache(), bc.Snapshots())
	if err != nil {
		return err
	}
	statedb.StartPrefetcher("chain")
	var failedProcessing bool
	receipts, _, _, err := n.eth.BlockChain().Processor().Process(block, statedb, *n.eth.BlockChain().GetVMConfig())
	if err != nil {
		failedProcessing = true
	}
	rawdb.WriteReceipts(n.eth.ChainDb(), block.Hash(), block.NumberU64(), receipts)
	root, err := statedb.Commit(false)
	if err != nil {
		return err
	}
	_ = root
	triedb := bc.StateCache().TrieDB()
	if err := triedb.Commit(block.Root(), true, nil); err != nil {
		return err
	}

	if err := triedb.Commit(root, true, nil); err != nil {
		return err
	}

	rawdb.WriteHeadHeaderHash(n.eth.ChainDb(), block.Hash())
	rawdb.WriteHeadFastBlockHash(n.eth.ChainDb(), block.Hash())
	rawdb.WriteCanonicalHash(n.eth.ChainDb(), block.Hash(), block.NumberU64())
	rawdb.WriteTxLookupEntriesByBlock(n.eth.ChainDb(), block)
	rawdb.WriteHeadBlockHash(n.eth.ChainDb(), block.Hash())
	oldProcessor := bc.Processor()
	if failedProcessing {
		bc.SetBlockValidatorAndProcessorForTesting(new(validator), new(processor))
	}

	if _, err := bc.SetCanonical(block); err != nil {
		panic(err)
	}
	// Restore processor
	if failedProcessing {
		bc.SetBlockValidatorAndProcessorForTesting(new(validator), oldProcessor)
	}
	return nil
}

// Engine API
func (n *GethNode) NewPayloadV1(ctx context.Context, pl *beacon.ExecutableDataV1) (beacon.PayloadStatusV1, error) {
	n.latestPayloadSent = pl
	resp, err := n.api.NewPayloadV1(*pl)
	n.latestPayloadStatusReponse = &resp
	return resp, err
}

func (n *GethNode) ForkchoiceUpdatedV1(ctx context.Context, fcs *beacon.ForkchoiceStateV1, payload *beacon.PayloadAttributesV1) (beacon.ForkChoiceResponse, error) {
	n.latestFcUStateSent = fcs
	n.latestPAttrSent = payload
	fcr, err := n.api.ForkchoiceUpdatedV1(*fcs, payload)
	n.latestFcUResponse = &fcr
	return fcr, err
}

func (n *GethNode) GetPayloadV1(ctx context.Context, payloadId *beacon.PayloadID) (beacon.ExecutableDataV1, error) {
	p, err := n.api.GetPayloadV1(*payloadId)
	if p == nil || err != nil {
		return beacon.ExecutableDataV1{}, err
	}
	return *p, err
}

// Eth JSON RPC
const (
	SafeBlockNumber      = rpc.BlockNumber(-4) // This is not yet true
	FinalizedBlockNumber = rpc.BlockNumber(-3)
	PendingBlockNumber   = rpc.BlockNumber(-2)
	LatestBlockNumber    = rpc.BlockNumber(-1)
	EarliestBlockNumber  = rpc.BlockNumber(0)
)

var (
	Head      *big.Int // Nil
	Pending   = big.NewInt(-2)
	Finalized = big.NewInt(-3)
	Safe      = big.NewInt(-4)
)

func parseBlockNumber(number *big.Int) rpc.BlockNumber {
	if number == nil {
		return LatestBlockNumber
	}
	return rpc.BlockNumber(number.Int64())
}

func (n *GethNode) BlockByNumber(ctx context.Context, number *big.Int) (*types.Block, error) {
	return n.eth.APIBackend.BlockByNumber(ctx, parseBlockNumber(number))
}

func (n *GethNode) BlockNumber(ctx context.Context) (uint64, error) {
	return n.eth.APIBackend.CurrentBlock().NumberU64(), nil
}

func (n *GethNode) BlockByHash(ctx context.Context, hash common.Hash) (*types.Block, error) {
	return n.eth.APIBackend.BlockByHash(ctx, hash)
}

func (n *GethNode) HeaderByNumber(ctx context.Context, number *big.Int) (*types.Header, error) {
	b, err := n.eth.APIBackend.BlockByNumber(ctx, parseBlockNumber(number))
	if err != nil {
		return nil, err
	}
	if b == nil {
		return nil, fmt.Errorf("Block not found (%v)", number)
	}
	return b.Header(), err
}

func (n *GethNode) SendTransaction(ctx context.Context, tx *types.Transaction) error {
	return n.eth.APIBackend.SendTx(ctx, tx)
}

func (n *GethNode) getStateDB(ctx context.Context, blockNumber *big.Int) (*state.StateDB, error) {
	b, err := n.eth.APIBackend.BlockByNumber(ctx, parseBlockNumber(blockNumber))
	if err != nil {
		return nil, err
	}
	return state.New(b.Root(), n.eth.BlockChain().StateCache(), n.eth.BlockChain().Snapshots())
}

func (n *GethNode) BalanceAt(ctx context.Context, account common.Address, blockNumber *big.Int) (*big.Int, error) {
	stateDB, err := n.getStateDB(ctx, blockNumber)
	if err != nil {
		return nil, err
	}
	return stateDB.GetBalance(account), nil
}

func (n *GethNode) StorageAt(ctx context.Context, account common.Address, key common.Hash, blockNumber *big.Int) ([]byte, error) {
	stateDB, err := n.getStateDB(ctx, blockNumber)
	if err != nil {
		return nil, err
	}
	return stateDB.GetState(account, key).Bytes(), nil
}

func (n *GethNode) TransactionReceipt(ctx context.Context, txHash common.Hash) (*types.Receipt, error) {
	return nil, fmt.Errorf("TransactionReceipt not implemented")
}

func (n *GethNode) NonceAt(ctx context.Context, account common.Address, blockNumber *big.Int) (uint64, error) {
	stateDB, err := n.getStateDB(ctx, blockNumber)
	if err != nil {
		return 0, err
	}
	return stateDB.GetNonce(account), nil
}

func (n *GethNode) GetBlockTotalDifficulty(ctx context.Context, hash common.Hash) (*big.Int, error) {
	currentBlock := n.eth.BlockChain().GetBlockByHash(hash)
	return n.eth.BlockChain().GetTd(hash, currentBlock.NumberU64()), nil
}

func (n *GethNode) GetTotalDifficulty(ctx context.Context) (*big.Int, error) {
	return n.GetBlockTotalDifficulty(ctx, n.eth.BlockChain().CurrentHeader().Hash())
}

func (n *GethNode) TerminalTotalDifficulty() *big.Int {
	if n.ttd != nil {
		return n.ttd
	}
	return n.genesis.Config.TerminalTotalDifficulty
}

func (n *GethNode) EnodeURL() (string, error) {
	return n.node.Server().NodeInfo().Enode, nil
}

func (n *GethNode) ID() string {
	return n.node.Config().Name
}

func (n *GethNode) GetNextAccountNonce(testCtx context.Context, account common.Address) (uint64, error) {
	// First get the current head of the client where we will send the tx
	head, err := n.eth.APIBackend.BlockByNumber(testCtx, LatestBlockNumber)
	if err != nil {
		return 0, err
	}
	// Check if we have any info about this account, and when it was last updated
	if accTxInfo, ok := n.accTxInfoMap[account]; ok && accTxInfo != nil && (accTxInfo.PreviousBlock == head.Hash() || accTxInfo.PreviousBlock == head.ParentHash()) {
		// We have info about this account and is up to date (or up to date until the very last block).
		// Increase the nonce and return it
		accTxInfo.PreviousBlock = head.Hash()
		accTxInfo.PreviousNonce++
		return accTxInfo.PreviousNonce, nil
	}
	// We don't have info about this account, or is outdated, or we re-org'd, we must request the nonce
	nonce, err := n.NonceAt(testCtx, account, head.Number())
	if err != nil {
		return 0, err
	}
	n.accTxInfoMap[account] = &AccountTransactionInfo{
		PreviousBlock: head.Hash(),
		PreviousNonce: nonce,
	}
	return nonce, nil
}

func (n *GethNode) PostRunVerifications() error {
	// Check that the node did not receive more gossiped blocks than expected
	if n.config.ExpectedGossipNewBlocksCount != nil {
		if n.config.ExpectedGossipNewBlocksCount.Cmp(n.totalReceivedNewBlocks) != 0 {
			return fmt.Errorf("Node received gossiped blocks count different than expected: %d != %d", n.totalReceivedNewBlocks, n.config.ExpectedGossipNewBlocksCount)
		}
	}
	if n.config.PoWMiner {
		if n.terminalBlocksMined.Cmp(n.config.TerminalBlockSiblingCount) != 0 {
			return fmt.Errorf("PoW Miner node could not mine expected amount of terminal blocks: %d != %d", n.terminalBlocksMined, n.config.TerminalBlockSiblingCount)

		}
	}
	return nil
}

func (n *GethNode) LatestForkchoiceSent() (fcState *beacon.ForkchoiceStateV1, pAttributes *beacon.PayloadAttributesV1) {
	return n.latestFcUStateSent, n.latestPAttrSent
}

func (n *GethNode) LatestNewPayloadSent() (payload *beacon.ExecutableDataV1) {
	return n.latestPayloadSent
}

func (n *GethNode) LatestForkchoiceResponse() (fcuResponse *beacon.ForkChoiceResponse) {
	return n.latestFcUResponse
}

func (n *GethNode) LatestNewPayloadResponse() (payloadResponse *beacon.PayloadStatusV1) {
	return n.latestPayloadStatusReponse
}
