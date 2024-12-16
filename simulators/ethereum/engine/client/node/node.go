package node

import (
	"context"
	"encoding/binary"
	"fmt"
	"math/big"
	"os"
	"sync"
	"time"

	beacon "github.com/ethereum/go-ethereum/beacon/engine"
	"github.com/ethereum/go-ethereum/cmd/utils"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/stateless"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/eth"
	ethcatalyst "github.com/ethereum/go-ethereum/eth/catalyst"
	"github.com/ethereum/go-ethereum/eth/downloader"
	"github.com/ethereum/go-ethereum/eth/ethconfig"
	ethprotocol "github.com/ethereum/go-ethereum/eth/protocols/eth"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/node"
	"github.com/ethereum/go-ethereum/p2p"
	"github.com/ethereum/go-ethereum/p2p/enode"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/ethereum/hive/hivesim"
	"github.com/ethereum/hive/simulators/ethereum/engine/client"
	typ "github.com/ethereum/hive/simulators/ethereum/engine/types"
	"github.com/pkg/errors"
)

type GethNodeTestConfiguration struct {
	// Node name
	Name string

	// To allow testing terminal block gossiping on the other clients, restrict this client
	// to only connect to the boot node
	MaxPeers *big.Int

	// Max number of gossiped blocks to receive
	ExpectedGossipNewBlocksCount *big.Int

	// Chain to import
	ChainFile string
}
type GethNodeEngineStarter struct {
	// Client parameters used to launch the default client
	ChainFile string

	// Test specific configuration
	Config GethNodeTestConfiguration
}

var (
	DefaultMaxPeers = big.NewInt(25)
)

func (s GethNodeEngineStarter) StartClient(T *hivesim.T, testContext context.Context, genesis *core.Genesis, ClientParams hivesim.Params, ClientFiles hivesim.Params, bootClients ...client.EngineClient) (client.EngineClient, error) {
	return s.StartGethNode(T, testContext, genesis, ClientParams, ClientFiles, bootClients...)
}

func (s GethNodeEngineStarter) StartGethNode(T *hivesim.T, testContext context.Context, genesis *core.Genesis, ClientParams hivesim.Params, ClientFiles hivesim.Params, bootClients ...client.EngineClient) (*GethNode, error) {
	var err error

	var enodes []string
	if bootClients != nil && len(bootClients) > 0 {
		enodes = make([]string, len(bootClients))
		for i, bootClient := range bootClients {
			enodes[i], err = bootClient.EnodeURL()
			if err != nil {
				return nil, fmt.Errorf("Unable to obtain bootnode: %v", err)
			}
		}
	}

	if s.Config.MaxPeers == nil {
		s.Config.MaxPeers = DefaultMaxPeers
	}

	if s.ChainFile != "" {
		s.Config.ChainFile = "./chains/" + s.ChainFile
	} else {
		if chainFilePath, ok := ClientFiles["/chain.rlp"]; ok {
			s.Config.ChainFile = chainFilePath
		}
	}

	g, err := newNode(s.Config, enodes, genesis)
	if err != nil {
		return nil, err
	}

	go func(ctx context.Context) {
		select {
		case <-ctx.Done():
			// Close the node when the context is done
			if err := g.Close(); err != nil {
				panic(err)
			}
		}
	}(testContext)

	return g, nil
}

type AccountTransactionInfo struct {
	PreviousBlock common.Hash
	PreviousNonce uint64
}

type GethNode struct {
	node *node.Node
	eth  *eth.Ethereum

	mustHeadBlock *types.Header

	datadir    string
	genesis    *core.Genesis
	api        *ethcatalyst.ConsensusAPI
	running    context.Context
	closing    context.CancelFunc
	closeMutex sync.Mutex

	// Engine updates info
	latestFcUStateSent *beacon.ForkchoiceStateV1
	latestPAttrSent    *typ.PayloadAttributes
	latestFcUResponse  *beacon.ForkChoiceResponse

	latestPayloadSent          *typ.ExecutableData
	latestPayloadStatusReponse *beacon.PayloadStatusV1

	// Test specific configuration
	accTxInfoMap                      map[common.Address]*AccountTransactionInfo
	accTxInfoMapLock                  sync.Mutex
	totalReceivedOrAnnouncedNewBlocks *big.Int

	config GethNodeTestConfiguration
}

var _ client.EngineClient = (*GethNode)(nil)

func newNode(config GethNodeTestConfiguration, bootnodes []string, genesis *core.Genesis) (*GethNode, error) {
	// Define the basic configurations for the Ethereum node
	datadir, _ := os.MkdirTemp("", "")

	return restart(config, bootnodes, datadir, genesis)
}

func restart(startConfig GethNodeTestConfiguration, bootnodes []string, datadir string, genesis *core.Genesis) (*GethNode, error) {
	if startConfig.Name == "" {
		startConfig.Name = "Modified Geth Module"
	}
	config := &node.Config{
		Name:    startConfig.Name,
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
	if genesis == nil || genesis.Config == nil {
		return nil, fmt.Errorf("genesis configuration is nil")
	}
	econfig := &ethconfig.Config{
		Genesis:         genesis,
		NetworkId:       genesis.Config.ChainID.Uint64(),
		SyncMode:        downloader.FullSync,
		DatabaseCache:   256,
		DatabaseHandles: 256,
		StateScheme:     rawdb.PathScheme,
		TxPool:          ethconfig.Defaults.TxPool,
		GPO:             ethconfig.Defaults.GPO,
		Miner:           ethconfig.Defaults.Miner,
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
	if bootnodes != nil && len(bootnodes) > 0 {
		for _, bootnode := range bootnodes {
			node := enode.MustParse(bootnode)
			stack.Server().AddTrustedPeer(node)
			stack.Server().AddPeer(node)
		}
	}

	stack.Server().EnableMsgEvents = true

	g := &GethNode{
		node:         stack,
		eth:          ethBackend,
		datadir:      datadir,
		genesis:      genesis,
		api:          ethcatalyst.NewConsensusAPI(ethBackend),
		accTxInfoMap: make(map[common.Address]*AccountTransactionInfo),
		// Test related configuration
		totalReceivedOrAnnouncedNewBlocks: big.NewInt(0),
		config:                            startConfig,
	}

	g.running, g.closing = context.WithCancel(context.Background())

	// Start thread to monitor the amount of gossiped blocks this node receives
	go g.SubscribeP2PEvents()

	return g, err
}

func (n *GethNode) AddPeer(ec client.EngineClient) error {
	bootnode, err := ec.EnodeURL()
	if err != nil {
		return err
	}
	node := enode.MustParse(bootnode)
	n.node.Server().AddTrustedPeer(node)
	n.node.Server().AddPeer(node)
	return nil
}

// Sets the canonical block to an ancestor of the provided block.
// `N==1` means to re-org to the parent of the provided block.
func (n *GethNode) ReOrgBackBlockChain(N uint64, currentBlock *types.Header) (*types.Header, error) {
	if N == 0 {
		return currentBlock, nil
	}
	for ; N > 0; N-- {
		currentBlock = n.eth.BlockChain().GetHeaderByHash(currentBlock.ParentHash)
		if currentBlock == nil {
			return nil, fmt.Errorf("Unable to re-org back")
		}
	}

	block := n.eth.BlockChain().GetBlock(currentBlock.Hash(), currentBlock.Number.Uint64())

	_, err := n.eth.BlockChain().SetCanonical(block)
	n.mustHeadBlock = currentBlock
	if err != nil {
		return nil, err
	}
	return currentBlock, nil
}

func (n *GethNode) SubscribeP2PEvents() {
	eventChan := make(chan *p2p.PeerEvent)
	subscription := n.node.Server().SubscribeEvents(eventChan)
	for {
		select {
		case event := <-eventChan:
			var msgCode uint64
			if event.MsgCode != nil {
				msgCode = *event.MsgCode
			}
			if event.Type == p2p.PeerEventTypeMsgRecv {
				if msgCode == ethprotocol.NewBlockMsg {
					n.totalReceivedOrAnnouncedNewBlocks.Add(n.totalReceivedOrAnnouncedNewBlocks, common.Big1)
					fmt.Printf("DEBUG (%s) [%v]: Received new block: Peer=%s, MsgCode=%d, Type=%s, Protocol=%s, RemoteAddress=%s\n", n.config.Name, time.Now(), event.Peer.String(), msgCode, event.Type, event.Protocol, event.RemoteAddress)
				} else if msgCode == ethprotocol.NewBlockHashesMsg {
					n.totalReceivedOrAnnouncedNewBlocks.Add(n.totalReceivedOrAnnouncedNewBlocks, common.Big1)
					fmt.Printf("DEBUG (%s) [%v]: Received new block announcement: Peer=%s, MsgCode=%d, Type=%s, Protocol=%s, RemoteAddress=%s\n", n.config.Name, time.Now(), event.Peer.String(), msgCode, event.Type, event.Protocol, event.RemoteAddress)
				}
			}

		case <-n.running.Done():
			subscription.Unsubscribe()
			return
		}
	}
}

func (n *GethNode) Close() error {
	n.closeMutex.Lock()
	defer n.closeMutex.Unlock()
	select {
	case <-n.running.Done():
		// Node already closed
		return nil
	default:
	}
	n.closing()
	err := n.node.Close()
	if err != nil {
		return err
	}
	if err := os.RemoveAll(n.datadir); err != nil {
		panic(err)
	}
	return nil
}

// Dummy validator that always returns success, used to set invalid blocks at the head of the chain, which then can be
// served to other clients.
type validator struct{}

func (v *validator) ValidateBody(block *types.Block) error {
	return nil
}
func (v *validator) ValidateState(block *types.Block, state *state.StateDB, result *core.ProcessResult, stateless bool) error {
	return nil
}
func (v *validator) ValidateWitness(witness *stateless.Witness, receiptRoot common.Hash, stateRoot common.Hash) error {
	return nil
}

type processor struct{}

func (p *processor) Process(block *types.Block, statedb *state.StateDB, cfg vm.Config) (*core.ProcessResult, error) {
	return &core.ProcessResult{GasUsed: 21000}, nil
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
	db := n.eth.ChainDb()
	rawdb.WriteTd(db, block.Hash(), block.NumberU64(), parentTd.Add(parentTd, block.Difficulty()))
	rawdb.WriteBlock(db, block)

	// write real info (fixes fake number test)
	data, err := rlp.EncodeToBytes(block.Header())
	if err != nil {
		log.Crit("Failed to RLP encode header", "err", err)
	}
	key := headerKey(parentNumber+1, block.Hash())
	if err := db.Put(key, data); err != nil {
		log.Crit("Failed to store header", "err", err)
	}

	rawdb.WriteHeaderNumber(db, block.Hash(), block.NumberU64())
	bc := n.eth.BlockChain()
	bc.SetBlockValidatorAndProcessorForTesting(new(validator), n.eth.BlockChain().Processor())

	statedb, err := state.New(parentRoot, bc.StateCache())
	if err != nil {
		return errors.Wrap(err, "failed to create state db")
	}
	statedb.StartPrefetcher("chain", nil)
	var failedProcessing bool
	result, err := n.eth.BlockChain().Processor().Process(block, statedb, *n.eth.BlockChain().GetVMConfig())
	if err != nil {
		failedProcessing = true
	}
	rawdb.WriteReceipts(db, block.Hash(), block.NumberU64(), result.Receipts)
	root, err := statedb.Commit(block.NumberU64(), false)
	if err != nil {
		return errors.Wrap(err, "failed to commit state")
	}

	// If node is running in path mode, skip explicit gc operation
	// which is unnecessary in this mode.
	if triedb := bc.StateCache().TrieDB(); triedb.Scheme() != rawdb.PathScheme {
		if err := triedb.Commit(block.Root(), true); err != nil {
			return errors.Wrapf(err, "failed to commit block trie, pathScheme=%v", triedb.Scheme())
		}
		if err := triedb.Commit(root, true); err != nil {
			return errors.Wrapf(err, "failed to commit root trie, pathScheme=%v", triedb.Scheme())
		}
	}

	rawdb.WriteHeadHeaderHash(db, block.Hash())
	rawdb.WriteHeadFastBlockHash(db, block.Hash())
	rawdb.WriteCanonicalHash(db, block.Hash(), block.NumberU64())
	rawdb.WriteTxLookupEntriesByBlock(db, block)
	rawdb.WriteHeadBlockHash(db, block.Hash())
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
func (n *GethNode) NewPayload(ctx context.Context, version int, pl *typ.ExecutableData) (beacon.PayloadStatusV1, error) {
	switch version {
	case 1:
		return n.NewPayloadV1(ctx, pl)
	case 2:
		return n.NewPayloadV2(ctx, pl)
	case 3:
		return n.NewPayloadV3(ctx, pl)
	}
	return beacon.PayloadStatusV1{}, fmt.Errorf("unknown version %d", version)
}

func (n *GethNode) NewPayloadV1(ctx context.Context, pl *typ.ExecutableData) (beacon.PayloadStatusV1, error) {
	n.latestPayloadSent = pl
	edConverted, err := typ.ToBeaconExecutableData(pl)
	if err != nil {
		return beacon.PayloadStatusV1{}, err
	}
	resp, err := n.api.NewPayloadV1(edConverted)
	n.latestPayloadStatusReponse = &resp
	return resp, err
}

func (n *GethNode) NewPayloadV2(ctx context.Context, pl *typ.ExecutableData) (beacon.PayloadStatusV1, error) {
	n.latestPayloadSent = pl
	ed, err := typ.ToBeaconExecutableData(pl)
	if err != nil {
		return beacon.PayloadStatusV1{}, err
	}
	resp, err := n.api.NewPayloadV2(ed)
	n.latestPayloadStatusReponse = &resp
	return resp, err
}

func (n *GethNode) NewPayloadV3(ctx context.Context, pl *typ.ExecutableData) (beacon.PayloadStatusV1, error) {
	n.latestPayloadSent = pl
	ed, err := typ.ToBeaconExecutableData(pl)
	if err != nil {
		return beacon.PayloadStatusV1{}, err
	}
	if pl.VersionedHashes == nil {
		return beacon.PayloadStatusV1{}, fmt.Errorf("versioned hashes are nil")
	}
	resp, err := n.api.NewPayloadV3(ed, *pl.VersionedHashes, pl.ParentBeaconBlockRoot)
	n.latestPayloadStatusReponse = &resp
	return resp, err
}

func (n *GethNode) ForkchoiceUpdated(ctx context.Context, version int, fcs *beacon.ForkchoiceStateV1, payload *typ.PayloadAttributes) (beacon.ForkChoiceResponse, error) {
	switch version {
	case 1:
		return n.ForkchoiceUpdatedV1(ctx, fcs, payload)
	case 2:
		return n.ForkchoiceUpdatedV2(ctx, fcs, payload)
	case 3:
		return n.ForkchoiceUpdatedV3(ctx, fcs, payload)
	default:
		return beacon.ForkChoiceResponse{}, fmt.Errorf("unknown version %d", version)
	}
}

func GethPayloadAttributes(payload *typ.PayloadAttributes) *beacon.PayloadAttributes {
	if payload == nil {
		return nil
	}
	return &beacon.PayloadAttributes{
		Timestamp:             payload.Timestamp,
		Random:                payload.Random,
		SuggestedFeeRecipient: payload.SuggestedFeeRecipient,
		Withdrawals:           payload.Withdrawals,
		BeaconRoot:            payload.BeaconRoot,
	}
}

func (n *GethNode) ForkchoiceUpdatedV1(ctx context.Context, fcs *beacon.ForkchoiceStateV1, payload *typ.PayloadAttributes) (beacon.ForkChoiceResponse, error) {
	n.latestFcUStateSent = fcs
	n.latestPAttrSent = payload
	fcr, err := n.api.ForkchoiceUpdatedV1(*fcs, GethPayloadAttributes(payload))
	n.latestFcUResponse = &fcr
	return fcr, err
}

func (n *GethNode) ForkchoiceUpdatedV2(ctx context.Context, fcs *beacon.ForkchoiceStateV1, payload *typ.PayloadAttributes) (beacon.ForkChoiceResponse, error) {
	n.latestFcUStateSent = fcs
	n.latestPAttrSent = payload
	fcr, err := n.api.ForkchoiceUpdatedV2(*fcs, GethPayloadAttributes(payload))
	n.latestFcUResponse = &fcr
	return fcr, err
}

func (n *GethNode) ForkchoiceUpdatedV3(ctx context.Context, fcs *beacon.ForkchoiceStateV1, payload *typ.PayloadAttributes) (beacon.ForkChoiceResponse, error) {
	n.latestFcUStateSent = fcs
	n.latestPAttrSent = payload
	fcr, err := n.api.ForkchoiceUpdatedV3(*fcs, GethPayloadAttributes(payload))
	n.latestFcUResponse = &fcr
	return fcr, err
}

func (n *GethNode) GetPayloadV1(ctx context.Context, payloadId *beacon.PayloadID) (typ.ExecutableData, error) {
	p, err := n.api.GetPayloadV1(*payloadId)
	if p == nil || err != nil {
		return typ.ExecutableData{}, err
	}
	return typ.FromBeaconExecutableData(p)
}

func (n *GethNode) GetPayloadV2(ctx context.Context, payloadId *beacon.PayloadID) (typ.ExecutableData, *big.Int, error) {
	p, err := n.api.GetPayloadV2(*payloadId)
	if p == nil || err != nil {
		return typ.ExecutableData{}, nil, err
	}
	ed, err := typ.FromBeaconExecutableData(p.ExecutionPayload)
	return ed, p.BlockValue, err
}

func (n *GethNode) GetPayloadV3(ctx context.Context, payloadId *beacon.PayloadID) (typ.ExecutableData, *big.Int, *typ.BlobsBundle, *bool, error) {
	p, err := n.api.GetPayloadV3(*payloadId)
	if p == nil || err != nil {
		return typ.ExecutableData{}, nil, nil, nil, err
	}
	ed, err := typ.FromBeaconExecutableData(p.ExecutionPayload)
	blobsBundle := &typ.BlobsBundle{}
	if err := blobsBundle.FromBeaconBlobsBundle(p.BlobsBundle); err != nil {
		return typ.ExecutableData{}, nil, nil, nil, err
	}
	return ed, p.BlockValue, blobsBundle, &p.Override, err
}

func (n *GethNode) GetPayload(ctx context.Context, version int, payloadId *beacon.PayloadID) (typ.ExecutableData, *big.Int, *typ.BlobsBundle, *bool, error) {

	switch version {
	case 1:
		ed, err := n.GetPayloadV1(ctx, payloadId)
		return ed, nil, nil, nil, err
	case 2:
		ed, value, err := n.GetPayloadV2(ctx, payloadId)
		return ed, value, nil, nil, err
	case 3:
		return n.GetPayloadV3(ctx, payloadId)
	default:
		return typ.ExecutableData{}, nil, nil, nil, fmt.Errorf("unknown version %d", version)
	}
}

func (n *GethNode) GetPayloadBodiesByRangeV1(ctx context.Context, start uint64, count uint64) ([]*typ.ExecutionPayloadBodyV1, error) {
	return nil, fmt.Errorf("not implemented")
}

func (n *GethNode) GetPayloadBodiesByHashV1(ctx context.Context, hashes []common.Hash) ([]*typ.ExecutionPayloadBodyV1, error) {
	return nil, fmt.Errorf("not implemented")
}

func (n *GethNode) GetBlobsBundleV1(ctx context.Context, payloadId *beacon.PayloadID) (*typ.BlobsBundle, error) {
	return nil, fmt.Errorf("not implemented")
}

// Eth JSON RPC

func parseBlockNumber(number *big.Int) rpc.BlockNumber {
	if number == nil {
		return rpc.LatestBlockNumber
	}
	return rpc.BlockNumber(number.Int64())
}

func (n *GethNode) BlockByNumber(ctx context.Context, number *big.Int) (*types.Block, error) {
	if number == nil && n.mustHeadBlock != nil {
		mustHeadHash := n.mustHeadBlock.Hash()
		block := n.eth.BlockChain().GetBlock(mustHeadHash, n.mustHeadBlock.Number.Uint64())
		return block, nil
	}
	return n.eth.APIBackend.BlockByNumber(ctx, parseBlockNumber(number))
}

func (n *GethNode) BlockNumber(ctx context.Context) (uint64, error) {
	if n.mustHeadBlock != nil {
		return n.mustHeadBlock.Number.Uint64(), nil
	}
	return n.eth.APIBackend.CurrentBlock().Number.Uint64(), nil
}

func (n *GethNode) BlockByHash(ctx context.Context, hash common.Hash) (*types.Block, error) {
	return n.eth.APIBackend.BlockByHash(ctx, hash)
}

func (n *GethNode) HeaderByNumber(ctx context.Context, number *big.Int) (*types.Header, error) {
	if number == nil && n.mustHeadBlock != nil {
		return n.mustHeadBlock, nil
	}
	b, err := n.eth.APIBackend.BlockByNumber(ctx, parseBlockNumber(number))
	if err != nil {
		return nil, err
	}
	if b == nil {
		return nil, fmt.Errorf("Block not found (%v)", number)
	}
	return b.Header(), err
}

func (n *GethNode) HeaderByHash(ctx context.Context, hash common.Hash) (*types.Header, error) {
	b, err := n.eth.APIBackend.BlockByHash(ctx, hash)
	if err != nil {
		return nil, err
	}
	if b == nil {
		return nil, fmt.Errorf("Header not found (%v)", hash)
	}
	return b.Header(), err
}

func (n *GethNode) SendTransaction(ctx context.Context, tx typ.Transaction) error {
	if v, ok := tx.(*types.Transaction); ok {
		return n.eth.APIBackend.SendTx(ctx, v)
	}
	return fmt.Errorf("invalid transaction type")
}

func (n *GethNode) SendTransactions(ctx context.Context, txs ...typ.Transaction) []error {
	for _, tx := range txs {
		if v, ok := tx.(*types.Transaction); ok {
			err := n.eth.APIBackend.SendTx(ctx, v)
			if err != nil {
				return []error{err}
			}
		} else {
			return []error{fmt.Errorf("invalid transaction type")}
		}

	}
	return nil
}

func (n *GethNode) getStateDB(ctx context.Context, blockNumber *big.Int) (*state.StateDB, error) {
	b, err := n.eth.APIBackend.BlockByNumber(ctx, parseBlockNumber(blockNumber))
	if err != nil {
		return nil, err
	}
	return state.New(b.Root(), n.eth.BlockChain().StateCache())
}

func (n *GethNode) BalanceAt(ctx context.Context, account common.Address, blockNumber *big.Int) (*big.Int, error) {
	stateDB, err := n.getStateDB(ctx, blockNumber)
	if err != nil {
		return nil, err
	}
	return stateDB.GetBalance(account).ToBig(), nil
}

func (n *GethNode) StorageAt(ctx context.Context, account common.Address, key common.Hash, blockNumber *big.Int) ([]byte, error) {
	stateDB, err := n.getStateDB(ctx, blockNumber)
	if err != nil {
		return nil, err
	}
	return stateDB.GetState(account, key).Bytes(), nil
}

func (n *GethNode) StorageAtKeys(ctx context.Context, account common.Address, keys []common.Hash, blockNumber *big.Int) (map[common.Hash]*common.Hash, error) {
	return nil, fmt.Errorf("StorageAtKeys not implemented")
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

func (n *GethNode) TransactionByHash(ctx context.Context, hash common.Hash) (tx *types.Transaction, isPending bool, err error) {
	panic("NOT IMPLEMENTED")
}

func (n *GethNode) PendingTransactionCount(ctx context.Context) (uint, error) {
	panic("NOT IMPLEMENTED")
}

func (n *GethNode) EnodeURL() (string, error) {
	return n.node.Server().NodeInfo().Enode, nil
}

func (n *GethNode) ID() string {
	return n.node.Config().Name
}

func (n *GethNode) GetLastAccountNonce(testCtx context.Context, account common.Address, head *types.Header) (uint64, error) {

	// First get the current head of the client where we will send the tx
	if head == nil {
		block, err := n.eth.APIBackend.BlockByNumber(testCtx, rpc.LatestBlockNumber)
		if err != nil {
			return 0, err
		}
		head = block.Header()
	}

	// Then check if we have any info about this account, and when it was last updated
	if accTxInfo, ok := n.accTxInfoMap[account]; ok && accTxInfo != nil && (accTxInfo.PreviousBlock == head.Hash() || accTxInfo.PreviousBlock == head.ParentHash) {
		// We have info about this account and is up to date (or up to date until the very last block).
		// Return the previous nonce
		return accTxInfo.PreviousNonce, nil
	}
	// We don't have info about this account, so there is no previous nonce
	return 0, fmt.Errorf("no previous nonce for account %s", account.String())
}

func (n *GethNode) GetNextAccountNonce(testCtx context.Context, account common.Address, head *types.Header) (uint64, error) {
	if head == nil {
		// First get the current head of the client where we will send the tx
		block, err := n.eth.APIBackend.BlockByNumber(testCtx, rpc.LatestBlockNumber)
		if err != nil {
			return 0, err
		}
		head = block.Header()
	}
	// Check if we have any info about this account, and when it was last updated
	if accTxInfo, ok := n.accTxInfoMap[account]; ok && accTxInfo != nil && (accTxInfo.PreviousBlock == head.Hash() || accTxInfo.PreviousBlock == head.ParentHash) {
		// We have info about this account and is up to date (or up to date until the very last block).
		// Increase the nonce and return it
		accTxInfo.PreviousBlock = head.Hash()
		accTxInfo.PreviousNonce++
		return accTxInfo.PreviousNonce, nil
	}
	// We don't have info about this account, or is outdated, or we re-org'd, we must request the nonce
	nonce, err := n.NonceAt(testCtx, account, head.Number)
	if err != nil {
		return 0, err
	}
	n.accTxInfoMapLock.Lock()
	defer n.accTxInfoMapLock.Unlock()
	n.accTxInfoMap[account] = &AccountTransactionInfo{
		PreviousBlock: head.Hash(),
		PreviousNonce: nonce,
	}
	return nonce, nil
}

func (n *GethNode) UpdateNonce(testCtx context.Context, account common.Address, newNonce uint64) error {
	// First get the current head of the client where we will send the tx
	head, err := n.eth.APIBackend.BlockByNumber(testCtx, rpc.LatestBlockNumber)
	if err != nil {
		return err
	}
	n.accTxInfoMap[account] = &AccountTransactionInfo{
		PreviousBlock: head.Hash(),
		PreviousNonce: newNonce,
	}
	return nil
}

func (n *GethNode) PostRunVerifications() error {
	// Check that the node did not receive more gossiped blocks than expected
	if n.config.ExpectedGossipNewBlocksCount != nil {
		if n.config.ExpectedGossipNewBlocksCount.Cmp(n.totalReceivedOrAnnouncedNewBlocks) != 0 {
			return fmt.Errorf("Node received gossiped blocks count different than expected: %d != %d", n.totalReceivedOrAnnouncedNewBlocks, n.config.ExpectedGossipNewBlocksCount)
		}
	}
	return nil
}

func (n *GethNode) LatestForkchoiceSent() (fcState *beacon.ForkchoiceStateV1, pAttributes *typ.PayloadAttributes) {
	return n.latestFcUStateSent, n.latestPAttrSent
}

func (n *GethNode) LatestNewPayloadSent() (payload *typ.ExecutableData) {
	return n.latestPayloadSent
}

func (n *GethNode) LatestForkchoiceResponse() (fcuResponse *beacon.ForkChoiceResponse) {
	return n.latestFcUResponse
}

func (n *GethNode) LatestNewPayloadResponse() (payloadResponse *beacon.PayloadStatusV1) {
	return n.latestPayloadStatusReponse
}
