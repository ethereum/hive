package main

import (
	"bytes"
	"context"
	"fmt"
	"math/big"
	"net/http"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/ethereum/hive/hivesim"
)

// TestClient is 00the environment of a single test.
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

func RunTest(testName string, t *hivesim.T, c *hivesim.Client, v *Vault, cl *CLMocker, fn func(*TestEnv)) {
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

// Wait for a client to reach sync status past the PoS transition, with `PoSSyncTimeoutSeconds` seconds timeout
func (t *TestEnv) WaitForPoSSync() bool {
	for i := 0; i < PoSSyncTimeoutSeconds; i++ {
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
		if t.CLMock.PoSBlockProductionFinished {
			return false
		}
		time.Sleep(time.Second)
	}
	return false
}

// Naive generic function that works in all situations.
// A better solution is to use logs to wait for confirmations.
func (t *TestEnv) WaitForTxConfirmations(txHash common.Hash, n uint64) (*types.Receipt, error) {
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
			fmt.Printf("WaitForTxConfirmations: Got receipt for %v\n", txHash)
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
			fmt.Printf("WaitForTxConfirmations: Reached confirmation block (%v) for %v\n", currentBlock.NumberU64(), txHash)
			if checkReceipt, err := t.Eth.TransactionReceipt(t.Ctx(), txHash); checkReceipt != nil {
				if bytes.Compare(receipt.PostState, checkReceipt.PostState) == 0 && receipt.BlockHash == checkReceipt.BlockHash {
					return checkReceipt, nil
				} else { // chain reorg
					return t.WaitForTxConfirmations(txHash, n)
				}
			} else {
				return nil, err
			}
		}

		time.Sleep(time.Second)
	}

	return nil, ethereum.NotFound
}

func (t *TestEnv) WaitForBlock(blockNumber *big.Int) (*types.Block, error) {
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
