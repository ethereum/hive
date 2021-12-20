package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/big"
	"net/http"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
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

	// Client's state on syncing to the latest PoS block
	PoSSyncMutex *sync.Mutex
	PoSSyncState *bool

	// This holds most recent context created by the Ctx method.
	// Every time Ctx is called, it creates a new context with the default
	// timeout and cancels the previous one.
	lastCtx    context.Context
	lastCancel context.CancelFunc
}

func runTest(testName string, t *hivesim.T, c *hivesim.Client, ec *EngineClient, v *Vault, cl *CLMocker, pm *sync.Mutex, ps *bool, fn func(*TestEnv)) {
	// This sets up debug logging of the requests and responses.
	client := &http.Client{
		Transport: &loggingRoundTrip{
			t:     t,
			inner: http.DefaultTransport,
		},
	}

	rpcClient, _ := rpc.DialHTTPWithClient(fmt.Sprintf("http://%v:8545/", c.IP), client)
	defer rpcClient.Close()
	env := &TestEnv{
		T:            t,
		TestName:     testName,
		RPC:          rpcClient,
		Eth:          ethclient.NewClient(rpcClient),
		Engine:       ec,
		CLMock:       cl,
		Vault:        v,
		PoSSyncMutex: pm,
		PoSSyncState: ps,
	}

	fn(env)
	if env.lastCtx != nil {
		env.lastCancel()
	}
}

func (t *TestEnv) WaitForPoSSync() bool {
	t.PoSSyncMutex.Lock()
	defer t.PoSSyncMutex.Unlock()
	return *t.PoSSyncState
}

// Many tests need to wait for the client to sync with the latest PoS block,
// so instead of having them all query for the latest block to match the CLMocker,
// this function updates the value and releases a lock when the client is synced.
func WaitClientForPoSSync(t *hivesim.T, c *hivesim.Client, cancelF **context.CancelFunc, m *sync.Mutex, s *bool) {
	m.Lock()
	defer m.Unlock()
	client := &http.Client{
		Transport: &loggingRoundTrip{
			t:     t,
			inner: http.DefaultTransport,
		},
	}

	rpcClient, _ := rpc.DialHTTPWithClient(fmt.Sprintf("http://%v:8545/", c.IP), client)
	defer rpcClient.Close()
	eth := ethclient.NewClient(rpcClient)

	for i := 0; i < 90; i++ {
		if clMocker.TTDReached {
			ctx, thisCancelF := context.WithTimeout(context.Background(), rpcTimeout)
			*cancelF = &thisCancelF
			bn, err := eth.BlockNumber(ctx)
			*cancelF = nil
			if err != nil {
				fmt.Printf("FAIL (WaitClientForPoSSync): error on get block number: %v\n", err)
				break
			}
			if clMocker.LatestFinalizedNumber != nil && bn >= clMocker.LatestFinalizedNumber.Uint64() {
				*s = true
				break
			}
		}
		time.Sleep(time.Second)
	}
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
	inner http.RoundTripper
}

func (rt *loggingRoundTrip) RoundTrip(req *http.Request) (*http.Response, error) {
	// Read and log the request body.
	reqBytes, err := ioutil.ReadAll(req.Body)
	req.Body.Close()
	if err != nil {
		return nil, err
	}
	rt.t.Logf(">>  %s", bytes.TrimSpace(reqBytes))
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
	rt.t.Logf("<<  %s", bytes.TrimSpace(respBytes))
	return &respCopy, nil
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
