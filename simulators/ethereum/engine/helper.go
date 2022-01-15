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

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/eth/catalyst"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/ethereum/go-ethereum/trie"
	"github.com/ethereum/hive/hivesim"
)

// default timeout for RPC calls
var rpcTimeout = 10 * time.Second

// TestClient is the environment of a single test.
type TestEnv struct {
	*hivesim.T
	TestName string
	RPC      *rpc.Client
	Eth      *ethclient.Client
	Engine   *EngineClient
	CLMock   *CLMocker
	Vault    *Vault

	// This holds most recent context created by the Ctx method.
	// Every time Ctx is called, it creates a new context with the default
	// timeout and cancels the previous one.
	lastCtx    context.Context
	lastCancel context.CancelFunc
}

func runTest(testName string, t *hivesim.T, c *hivesim.Client, v *Vault, cl *CLMocker, fn func(*TestEnv)) {
	// This sets up debug logging of the requests and responses.
	client := &http.Client{
		Transport: &loggingRoundTrip{
			t:     t,
			hc:    c,
			inner: http.DefaultTransport,
		},
	}

	ec := NewEngineClient(t, c)
	defer ec.Close()

	rpcClient, _ := rpc.DialHTTPWithClient(fmt.Sprintf("http://%v:8545/", c.IP), client)
	defer rpcClient.Close()
	env := &TestEnv{
		T:        t,
		TestName: testName,
		RPC:      rpcClient,
		Eth:      ethclient.NewClient(rpcClient),
		Engine:   ec,
		CLMock:   cl,
		Vault:    v,
	}

	fn(env)
	if env.lastCtx != nil {
		env.lastCancel()
	}
}

// Wait for a client to reach sync status past the PoS transition, with 90 second timeout
func (t *TestEnv) WaitForPoSSync() bool {

	for i := 0; i < 90; i++ {
		if clMocker.TTDReached {
			bn, err := t.Eth.BlockNumber(t.Ctx())
			if err != nil {
				t.Fatalf("FAIL (%v): error on get block number: %v", t.TestName, err)
			}
			if clMocker.LatestFinalizedNumber != nil && bn >= clMocker.LatestFinalizedNumber.Uint64() {
				t.Logf("INFO (%v): Client is now synced to latest PoS block", t.TestName)
				return true
			}
		}
		time.Sleep(time.Second)
	}
	return false
}

// Naive generic function that works in all situations.
// A better solution is to use logs to wait for confirmations.
func waitForTxConfirmations(t *TestEnv, txHash common.Hash, n uint64) (*types.Receipt, error) {
	var (
		receipt *types.Receipt
		err     error
	)

	for i := 0; i < 90; i++ {
		receipt, err = t.Eth.TransactionReceipt(t.Ctx(), txHash)
		if err != nil && err != ethereum.NotFound {
			return nil, err
		}
		if receipt != nil {
			fmt.Printf("waitForTxConfirmations: Got receipt for %v\n", txHash)
			break
		}
		time.Sleep(time.Second)
	}
	if receipt == nil {
		return nil, ethereum.NotFound
	}

	for i := 0; i < 90; i++ {
		currentBlock, err := t.Eth.BlockByNumber(t.Ctx(), nil)
		if err != nil {
			return nil, err
		}

		if currentBlock.NumberU64() >= receipt.BlockNumber.Uint64()+n {
			fmt.Printf("waitForTxConfirmations: Reached confirmation block (%v) for %v\n", currentBlock.NumberU64(), txHash)
			if checkReceipt, err := t.Eth.TransactionReceipt(t.Ctx(), txHash); checkReceipt != nil {
				if bytes.Compare(receipt.PostState, checkReceipt.PostState) == 0 && receipt.BlockHash == checkReceipt.BlockHash {
					return checkReceipt, nil
				} else { // chain reorg
					return waitForTxConfirmations(t, txHash, n)
				}
			} else {
				return nil, err
			}
		}

		time.Sleep(time.Second)
	}

	return nil, ethereum.NotFound
}

func waitForBlock(t *TestEnv, blockNumber *big.Int) (*types.Block, error) {
	for i := 0; i < 90; i++ {
		currentHeader, err := t.Eth.BlockByNumber(t.Ctx(), nil)
		if err != nil {
			return nil, err
		}
		if currentHeader.Number().Cmp(blockNumber) == 0 {
			return currentHeader, nil
		} else if currentHeader.Number().Cmp(blockNumber) > 0 {
			prevHeader, err := t.Eth.BlockByNumber(t.Ctx(), blockNumber)
			if err != nil {
				return nil, err
			}
			return prevHeader, nil
		}
		time.Sleep(time.Second)
	}
	return nil, nil
}

func getBigIntAtStorage(t *TestEnv, account common.Address, key common.Hash, blockNumber *big.Int) (*big.Int, error) {
	stor, err := t.Eth.StorageAt(t.Ctx(), account, key, blockNumber)
	if err != nil {
		return nil, err
	}
	bigint := big.NewInt(0)
	bigint.SetBytes(stor)
	return bigint, nil
}

// From ethereum/rpc:

// CallContext is a helper method that forwards a raw RPC request to
// the underlying RPC client. This can be used to call RPC methods
// that are not supported by the ethclient.Client.
func (t *TestEnv) CallContext(ctx context.Context, result interface{}, method string, args ...interface{}) error {
	return t.RPC.CallContext(ctx, result, method, args...)
}

// Ctx returns a context with the default timeout.
// For subsequent calls to Ctx, it also cancels the previous context.
func (t *TestEnv) Ctx() context.Context {
	if t.lastCtx != nil {
		t.lastCancel()
	}
	t.lastCtx, t.lastCancel = context.WithTimeout(context.Background(), rpcTimeout)
	return t.lastCtx
}

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
func customizePayload(basePayload *catalyst.ExecutableDataV1, customData *CustomPayloadData) (*catalyst.ExecutableDataV1, error) {
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
	return &catalyst.ExecutableDataV1{
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

func loadGenesis() *types.Block {
	contents, err := ioutil.ReadFile("init/genesis.json")
	if err != nil {
		panic(fmt.Errorf("can't to read genesis file: %v", err))
	}
	var genesis core.Genesis
	if err := json.Unmarshal(contents, &genesis); err != nil {
		panic(fmt.Errorf("can't parse genesis JSON: %v", err))
	}
	return genesis.ToBlock(nil)
}
