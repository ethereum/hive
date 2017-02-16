package main

import (
	"bytes"
	"fmt"
	"math/big"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
	"golang.org/x/net/context"
)

var (
	// default timeout for RPC calls
	rpcTimeout = 5 * time.Second
	// unique chain identifier used to sign transaction
	chainID = new(big.Int).SetInt64(7) // used for signing transactions
)

// TestClient is an ethclient that exposed the CallContext function.
// This allows for calling custom RPC methods that are not exposed
// by the ethclient.
type TestClient struct {
	*ethclient.Client
	rc *rpc.Client
}

// CallContext is a helper method that forwards a raw RPC request to
// the underlying RPC client. This can be used to call RPC methods
// that are not supported by the ethclient.Client.
func (c *TestClient) CallContext(ctx context.Context, result interface{}, method string, args ...interface{}) error {
	return c.rc.CallContext(ctx, result, method, args...)
}

// Naive generic function that works in all situations.
// A better solution is to use logs to wait for confirmations.
func waitForTxConfirmations(client *TestClient, txHash common.Hash, n uint64) (*types.Receipt, error) {
	var (
		receipt    *types.Receipt
		startBlock *types.Block
		err        error
	)

	for i := 0; i < 90; i++ {
		ctx, _ := context.WithTimeout(context.Background(), rpcTimeout)
		receipt, err = client.TransactionReceipt(ctx, txHash)
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

	ctx, _ := context.WithTimeout(context.Background(), rpcTimeout)
	if startBlock, err = client.BlockByNumber(ctx, nil); err != nil {
		return nil, err
	}

	for i := 0; i < 90; i++ {
		ctx, _ := context.WithTimeout(context.Background(), rpcTimeout)
		currentBlock, err := client.BlockByNumber(ctx, nil)
		if err != nil {
			return nil, err
		}

		if startBlock.NumberU64()+n >= currentBlock.NumberU64() {
			ctx, _ := context.WithTimeout(context.Background(), rpcTimeout)
			if checkReceipt, err := client.TransactionReceipt(ctx, txHash); checkReceipt != nil {
				if bytes.Compare(receipt.PostState, checkReceipt.PostState) == 0 {
					return receipt, nil
				} else { // chain reorg
					waitForTxConfirmations(client, txHash, n)
				}
			} else {
				return nil, err
			}
		}

		time.Sleep(time.Second)
	}

	return nil, ethereum.NotFound
}

func createHTTPClients(hosts []string) chan *TestClient {
	if len(hosts) == 0 {
		panic("Supply at least 1 host")
	}

	var (
		N       = 32
		clients = make(chan *TestClient, N)
	)

	for i := 0; i < N; i++ {
		client, err := rpc.Dial(fmt.Sprintf("http://%s:8545", hosts[i%len(hosts)]))
		if err != nil {
			panic(err)
		}
		clients <- &TestClient{ethclient.NewClient(client), client}
	}

	return clients
}

func createWebsocketClients(hosts []string) chan *TestClient {
	if len(hosts) == 0 {
		panic("Supply at least 1 host")
	}

	var (
		N       = 32
		clients = make(chan *TestClient, N)
	)

	for i := 0; i < N; i++ {
		client, err := rpc.Dial(fmt.Sprintf("ws://%s:8546", hosts[i%len(hosts)]))
		if err != nil {
			panic(err)
		}
		clients <- &TestClient{ethclient.NewClient(client), client}
	}

	return clients
}

// runTests is a utility function that calls the unit test with the
// client as second argument.
func runTest(test func(t *testing.T, client *TestClient), clients chan *TestClient) func(t *testing.T) {
	return func(t *testing.T) {
		client := <-clients
		test(t, client)
		clients <- client
	}
}

// SignTransaction signs the given transaction with the test account and returns it.
// It uses the EIP155 signing rules.
func SignTransaction(tx *types.Transaction, account accounts.Account) (*types.Transaction, error) {
	wallet, err := accountsManager.Find(account)
	if err != nil {
		return nil, err
	}
	return wallet.SignTxWithPassphrase(account, defaultPassword, tx, chainID)
}
