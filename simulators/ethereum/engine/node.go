package main

import (
	"encoding/binary"
	"math/big"
	"os"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/beacon"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/eth"
	ethcatalyst "github.com/ethereum/go-ethereum/eth/catalyst"
	"github.com/ethereum/go-ethereum/eth/downloader"
	"github.com/ethereum/go-ethereum/eth/ethconfig"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/miner"
	"github.com/ethereum/go-ethereum/node"
	"github.com/ethereum/go-ethereum/p2p"
	"github.com/ethereum/go-ethereum/p2p/enode"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/rlp"
)

type gethNode struct {
	node     *node.Node
	eth      *eth.Ethereum
	datadir  string
	bootnode string
	genesis  *core.Genesis
}

func newNode(bootnode string, genesis *core.Genesis) (*gethNode, error) {
	// Define the basic configurations for the Ethereum node
	datadir, _ := os.MkdirTemp("", "")

	return restart(bootnode, datadir, genesis)
}

func restart(bootnode, datadir string, genesis *core.Genesis) (*gethNode, error) {
	config := &node.Config{
		Name:    "geth",
		Version: params.Version,
		DataDir: datadir,
		P2P: p2p.Config{
			ListenAddr:  "0.0.0.0:0",
			NoDiscovery: true,
			MaxPeers:    25,
		},
		UseLightweightKDF: true,
	}
	// Create the node and configure a full Ethereum node on it
	stack, err := node.New(config)
	if err != nil {
		return nil, err
	}
	econfig := &ethconfig.Config{
		Genesis:         genesis,
		NetworkId:       genesis.Config.ChainID.Uint64(),
		SyncMode:        downloader.FullSync,
		DatabaseCache:   256,
		DatabaseHandles: 256,
		TxPool:          core.DefaultTxPoolConfig,
		GPO:             ethconfig.Defaults.GPO,
		Ethash:          ethconfig.Defaults.Ethash,
		Miner: miner.Config{
			GasFloor: genesis.GasLimit * 9 / 10,
			GasCeil:  genesis.GasLimit * 11 / 10,
			GasPrice: big.NewInt(1),
			Recommit: 10 * time.Second, // Disable the recommit
		},
		LightServ:        100,
		LightPeers:       10,
		LightNoSyncServe: true,
	}
	ethBackend, err := eth.New(stack, econfig)
	if err != nil {
		return nil, err
	}
	err = stack.Start()
	for stack.Server().NodeInfo().Ports.Listener == 0 {
		time.Sleep(250 * time.Millisecond)
	}
	// Connect the node to the bootnode
	node := enode.MustParse(bootnode)
	stack.Server().AddPeer(node)

	return &gethNode{
		node:     stack,
		eth:      ethBackend,
		datadir:  datadir,
		bootnode: bootnode,
		genesis:  genesis,
	}, err
}

type validator struct{}

func (v *validator) ValidateBody(block *types.Block) error {
	return nil
}
func (v *validator) ValidateState(block *types.Block, state *state.StateDB, receipts types.Receipts, usedGas uint64) error {
	return nil
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

func (n *gethNode) setBlock(block *types.Block, parentNumber uint64, parentRoot common.Hash) error {
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
	bc.SetBlockValidatorForTesting(new(validator))

	statedb, err := state.New(parentRoot, bc.StateCache(), bc.Snapshots())
	if err != nil {
		return err
	}
	statedb.StartPrefetcher("chain")
	receipts, _, _, err := n.eth.BlockChain().Processor().Process(block, statedb, *n.eth.BlockChain().GetVMConfig())
	if err != nil {
		// ignore error during block processing
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
	if _, err := bc.SetCanonical(block); err != nil {
		panic(err)
	}
	return nil
}

func (n *gethNode) sendNewPayload(pl *ExecutableDataV1) (beacon.PayloadStatusV1, error) {
	api := ethcatalyst.NewConsensusAPI(n.eth)
	return api.NewPayloadV1(beacon.ExecutableDataV1(execData(pl)))
}

func (n *gethNode) sendFCU(fcs *ForkchoiceStateV1, payload *PayloadAttributesV1) (beacon.ForkChoiceResponse, error) {
	api := ethcatalyst.NewConsensusAPI(n.eth)
	return api.ForkchoiceUpdatedV1(forkchoiceState(fcs), payloadAttributes(payload))
}

func execData(data *ExecutableDataV1) beacon.ExecutableDataV1 {
	return beacon.ExecutableDataV1{
		ParentHash:    data.ParentHash,
		FeeRecipient:  data.FeeRecipient,
		StateRoot:     data.StateRoot,
		ReceiptsRoot:  data.ReceiptsRoot,
		LogsBloom:     data.LogsBloom,
		Random:        data.PrevRandao,
		Number:        data.Number,
		GasLimit:      data.GasLimit,
		GasUsed:       data.GasUsed,
		Timestamp:     data.Timestamp,
		ExtraData:     data.ExtraData,
		BaseFeePerGas: data.BaseFeePerGas,
		BlockHash:     data.BlockHash,
		Transactions:  data.Transactions,
	}
}

func forkchoiceState(data *ForkchoiceStateV1) beacon.ForkchoiceStateV1 {
	return beacon.ForkchoiceStateV1{
		HeadBlockHash:      data.HeadBlockHash,
		SafeBlockHash:      data.SafeBlockHash,
		FinalizedBlockHash: data.FinalizedBlockHash,
	}
}

func payloadAttributes(data *PayloadAttributesV1) *beacon.PayloadAttributesV1 {
	if data == nil {
		return nil
	}
	return &beacon.PayloadAttributesV1{
		Timestamp:             data.Timestamp,
		Random:                data.PrevRandao,
		SuggestedFeeRecipient: data.SuggestedFeeRecipient,
	}
}
