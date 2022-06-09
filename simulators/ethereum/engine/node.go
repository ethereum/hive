package main

import (
	"math/big"
	"os"
	"time"

	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/beacon"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/eth"
	ethcatalyst "github.com/ethereum/go-ethereum/eth/catalyst"
	"github.com/ethereum/go-ethereum/eth/downloader"
	"github.com/ethereum/go-ethereum/eth/ethconfig"
	"github.com/ethereum/go-ethereum/miner"
	"github.com/ethereum/go-ethereum/node"
	"github.com/ethereum/go-ethereum/p2p"
	"github.com/ethereum/go-ethereum/params"
)

type gethNode struct {
	node *node.Node
	eth  *eth.Ethereum
}

func newNode(bootnode string, genesis *core.Genesis) (*gethNode, error) {
	// Define the basic configurations for the Ethereum node
	datadir, _ := os.MkdirTemp("", "")

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
		SyncMode:        downloader.SnapSync,
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

	return &gethNode{
		node: stack,
		eth:  ethBackend,
	}, err
}

func (n *gethNode) setBlock(block *types.Block) {
	parentTd := n.eth.BlockChain().GetTd(block.ParentHash(), block.NumberU64()-1)
	rawdb.WriteTd(n.eth.ChainDb(), block.Hash(), block.NumberU64(), parentTd.Add(parentTd, block.Difficulty()))
	rawdb.WriteBlock(n.eth.ChainDb(), block)
	rawdb.WriteHeadBlockHash(n.eth.ChainDb(), block.Hash())
	rawdb.WriteHeadHeaderHash(n.eth.ChainDb(), block.Hash())
	rawdb.WriteCanonicalHash(n.eth.ChainDb(), block.Hash(), block.NumberU64())
	rawdb.WriteHeaderNumber(n.eth.ChainDb(), block.Hash(), block.NumberU64())
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
