package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/ethereum/hive/hivesim"
	"github.com/kr/pretty"
)

// default timeout for RPC calls
var rpcTimeout = 10 * time.Second

// TestClient is the environment of a single test.
type TestEnv struct {
	*hivesim.T
	RPC   *rpc.Client
	Eth   *ethclient.Client
	Vault *vault

	// This holds most recent context created by the Ctx method.
	// Every time Ctx is called, it creates a new context with the default
	// timeout and cancels the previous one.
	lastCtx    context.Context
	lastCancel context.CancelFunc
}

// runHTTP runs the given test function using the HTTP RPC client.
func runHTTP(t *hivesim.T, c *hivesim.Client, v *vault, fn func(*TestEnv)) {
	rpcClient, _ := rpc.DialHTTP(fmt.Sprintf("http://%v:8545/", c.IP))
	defer rpcClient.Close()

	env := &TestEnv{
		T:     t,
		RPC:   rpcClient,
		Eth:   ethclient.NewClient(rpcClient),
		Vault: v,
	}
	fn(env)
	if env.lastCtx != nil {
		env.lastCancel()
	}
}

// runWS runs the given test function using the WebSocket RPC client.
func runWS(t *hivesim.T, c *hivesim.Client, v *vault, fn func(*TestEnv)) {
	ctx, done := context.WithTimeout(context.Background(), 5*time.Second)
	rpcClient, err := rpc.DialWebsocket(ctx, fmt.Sprintf("ws://%v:8546/", c.IP), "")
	done()
	if err != nil {
		t.Fatal("WebSocket connection failed:", err)
	}
	defer rpcClient.Close()

	env := &TestEnv{
		T:     t,
		RPC:   rpcClient,
		Eth:   ethclient.NewClient(rpcClient),
		Vault: v,
	}
	fn(env)
	if env.lastCtx != nil {
		env.lastCancel()
	}
}

// CallContext is a helper method that forwards a raw RPC request to
// the underlying RPC client. This can be used to call RPC methods
// that are not supported by the ethclient.Client.
func (t *TestEnv) CallContext(ctx context.Context, result interface{}, method string, args ...interface{}) error {
	return t.RPC.CallContext(ctx, result, method, args...)
}

// Ctx returns a context with a 5s timeout.
func (t *TestEnv) Ctx() context.Context {
	if t.lastCtx != nil {
		t.lastCancel() // Cancel previous context before making a new one.
	}
	t.lastCtx, t.lastCancel = context.WithTimeout(context.Background(), rpcTimeout)
	return t.lastCtx
}

// Naive generic function that works in all situations.
// A better solution is to use logs to wait for confirmations.
func waitForTxConfirmations(t *TestEnv, txHash common.Hash, n uint64) (*types.Receipt, error) {
	var (
		receipt    *types.Receipt
		startBlock *types.Block
		err        error
	)

	for i := 0; i < 90; i++ {
		receipt, err = t.Eth.TransactionReceipt(t.Ctx(), txHash)
		if err != nil && err != ethereum.NotFound {
			return nil, err
		}
		if receipt != nil {
			break
		}
		time.Sleep(time.Second)
	}
	if receipt == nil {
		return nil, ethereum.NotFound
	}

	if startBlock, err = t.Eth.BlockByNumber(t.Ctx(), nil); err != nil {
		return nil, err
	}

	for i := 0; i < 90; i++ {
		currentBlock, err := t.Eth.BlockByNumber(t.Ctx(), nil)
		if err != nil {
			return nil, err
		}

		if startBlock.NumberU64()+n >= currentBlock.NumberU64() {
			if checkReceipt, err := t.Eth.TransactionReceipt(t.Ctx(), txHash); checkReceipt != nil {
				if bytes.Compare(receipt.PostState, checkReceipt.PostState) == 0 {
					return receipt, nil
				} else { // chain reorg
					waitForTxConfirmations(t, txHash, n)
				}
			} else {
				return nil, err
			}
		}

		time.Sleep(time.Second)
	}

	return nil, ethereum.NotFound
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

// diff checks whether x and y are deeply equal, returning a description
// of their differences if they are not equal.
func diff(x, y interface{}) (d string) {
	for _, l := range pretty.Diff(x, y) {
		d += l + "\n"
	}
	return d
}
