package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/big"
	"net/http"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/beacon"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/trie"
	"github.com/ethereum/hive/hivesim"
)

// default timeout for RPC calls
var rpcTimeout = 10 * time.Second

// Retrieves contract storage as BigInt
func getBigIntAtStorage(eth *ethclient.Client, ctx context.Context, account common.Address, key common.Hash, blockNumber *big.Int) (*big.Int, error) {
	stor, err := eth.StorageAt(ctx, account, key, blockNumber)
	if err != nil {
		return nil, err
	}
	bigint := big.NewInt(0)
	bigint.SetBytes(stor)
	return bigint, nil
}

// From ethereum/rpc:

// loggingRoundTrip writes requests and responses to the test log.
type loggingRoundTrip struct {
	t     *hivesim.T
	hc    *hivesim.Client
	inner http.RoundTripper
}

func (rt *loggingRoundTrip) RoundTrip(req *http.Request) (*http.Response, error) {
	// Read and log the request body.
	reqBytes, err := ioutil.ReadAll(req.Body)
	req.Body.Close()
	if err != nil {
		return nil, err
	}
	rt.t.Logf(">> (%s) %s", rt.hc.Container, bytes.TrimSpace(reqBytes))
	reqCopy := *req
	reqCopy.Body = ioutil.NopCloser(bytes.NewReader(reqBytes))

	// Do the round trip.
	resp, err := rt.inner.RoundTrip(&reqCopy)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Read and log the response bytes.
	respBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	respCopy := *resp
	respCopy.Body = ioutil.NopCloser(bytes.NewReader(respBytes))
	rt.t.Logf("<< (%s) %s", rt.hc.Container, bytes.TrimSpace(respBytes))
	return &respCopy, nil
}

type CustomPayloadData struct {
	ParentHash    *common.Hash
	FeeRecipient  *common.Address
	StateRoot     *common.Hash
	ReceiptsRoot  *common.Hash
	LogsBloom     *[]byte
	Random        *common.Hash
	Number        *uint64
	GasLimit      *uint64
	GasUsed       *uint64
	Timestamp     *uint64
	ExtraData     *[]byte
	BaseFeePerGas *big.Int
	BlockHash     *common.Hash
	Transactions  *[][]byte
}

func calcTxsHash(txsBytes [][]byte) (common.Hash, error) {
	txs := make([]*types.Transaction, len(txsBytes))
	for i, bytesTx := range txsBytes {
		var currentTx types.Transaction
		err := currentTx.UnmarshalBinary(bytesTx)
		if err != nil {
			return common.Hash{}, err
		}
		txs[i] = &currentTx
	}
	return types.DeriveSha(types.Transactions(txs), trie.NewStackTrie(nil)), nil
}

// Construct a customized payload by taking an existing payload as base and mixing it CustomPayloadData
// BlockHash is calculated automatically.
func customizePayload(basePayload *beacon.ExecutableDataV1, customData *CustomPayloadData) (*beacon.ExecutableDataV1, error) {
	txs := basePayload.Transactions
	if customData.Transactions != nil {
		txs = *customData.Transactions
	}
	txsHash, err := calcTxsHash(txs)
	if err != nil {
		return nil, err
	}
	fmt.Printf("txsHash: %v\n", txsHash)
	// Start by filling the header with the basePayload information
	customPayloadHeader := types.Header{
		ParentHash:  basePayload.ParentHash,
		UncleHash:   types.EmptyUncleHash, // Could be overwritten
		Coinbase:    basePayload.FeeRecipient,
		Root:        basePayload.StateRoot,
		TxHash:      txsHash,
		ReceiptHash: basePayload.ReceiptsRoot,
		Bloom:       types.BytesToBloom(basePayload.LogsBloom),
		Difficulty:  big.NewInt(0), // could be overwritten
		Number:      big.NewInt(int64(basePayload.Number)),
		GasLimit:    basePayload.GasLimit,
		GasUsed:     basePayload.GasUsed,
		Time:        basePayload.Timestamp,
		Extra:       basePayload.ExtraData,
		MixDigest:   basePayload.Random,
		Nonce:       types.BlockNonce{0}, // could be overwritten
		BaseFee:     basePayload.BaseFeePerGas,
	}

	// Overwrite custom information
	if customData.ParentHash != nil {
		customPayloadHeader.ParentHash = *customData.ParentHash
	}
	if customData.FeeRecipient != nil {
		customPayloadHeader.Coinbase = *customData.FeeRecipient
	}
	if customData.StateRoot != nil {
		customPayloadHeader.Root = *customData.StateRoot
	}
	if customData.ReceiptsRoot != nil {
		customPayloadHeader.ReceiptHash = *customData.ReceiptsRoot
	}
	if customData.LogsBloom != nil {
		customPayloadHeader.Bloom = types.BytesToBloom(*customData.LogsBloom)
	}
	if customData.Random != nil {
		customPayloadHeader.MixDigest = *customData.Random
	}
	if customData.Number != nil {
		customPayloadHeader.Number = big.NewInt(int64(*customData.Number))
	}
	if customData.GasLimit != nil {
		customPayloadHeader.GasLimit = *customData.GasLimit
	}
	if customData.GasUsed != nil {
		customPayloadHeader.GasUsed = *customData.GasUsed
	}
	if customData.Timestamp != nil {
		customPayloadHeader.Time = *customData.Timestamp
	}
	if customData.ExtraData != nil {
		customPayloadHeader.Extra = *customData.ExtraData
	}
	if customData.BaseFeePerGas != nil {
		customPayloadHeader.BaseFee = customData.BaseFeePerGas
	}

	// Return the new payload
	return &beacon.ExecutableDataV1{
		ParentHash:    customPayloadHeader.ParentHash,
		FeeRecipient:  customPayloadHeader.Coinbase,
		StateRoot:     customPayloadHeader.Root,
		ReceiptsRoot:  customPayloadHeader.ReceiptHash,
		LogsBloom:     customPayloadHeader.Bloom[:],
		Random:        customPayloadHeader.MixDigest,
		Number:        customPayloadHeader.Number.Uint64(),
		GasLimit:      customPayloadHeader.GasLimit,
		GasUsed:       customPayloadHeader.GasUsed,
		Timestamp:     customPayloadHeader.Time,
		ExtraData:     customPayloadHeader.Extra,
		BaseFeePerGas: customPayloadHeader.BaseFee,
		BlockHash:     customPayloadHeader.Hash(),
		Transactions:  txs,
	}, nil
}

func loadGenesis(path string) core.Genesis {
	contents, err := ioutil.ReadFile(path)
	if err != nil {
		panic(fmt.Errorf("can't to read genesis file: %v", err))
	}
	var genesis core.Genesis
	if err := json.Unmarshal(contents, &genesis); err != nil {
		panic(fmt.Errorf("can't parse genesis JSON: %v", err))
	}
	return genesis
}

func loadGenesisBlock(path string) *types.Block {
	genesis := loadGenesis(path)
	return genesis.ToBlock(nil)
}

// Helper structs to fetch the TotalDifficulty
type TD struct {
	TotalDifficulty *hexutil.Big `json:"totalDifficulty"`
}
type TotalDifficultyHeader struct {
	types.Header
	TD
}

func (tdh *TotalDifficultyHeader) UnmarshalJSON(data []byte) error {
	if err := json.Unmarshal(data, &tdh.Header); err != nil {
		return err
	}
	if err := json.Unmarshal(data, &tdh.TD); err != nil {
		return err
	}
	return nil
}
