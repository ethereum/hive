package main

import (
	"flag"
	"math/rand"
	"os"
	"testing"
	"time"
)

var (
	ips []string // contains list of ip addresses of running nodes
)

// Examples:
// go test -run Eth -args 127.0.0.1 127.0.0.2                          # run all Eth related tests
// go test -run Eth/ethclient -args 127.0.0.1 127.0.0.2                # run all `ethclient` tests over all available transaction mechanisms
// go test -run Eth/abi/websocket/logs -args 127.0.0.1                 # run the logs test in the abi test suite over a websocket
// go test -run "Eth/ethclient/[http|websocket]/Block" -args 127.0.0.1 # run all tests in the ethclient group that begin with Block over http and websockets.
func TestMain(m *testing.M) {
	flag.Parse()
	ips = flag.Args() // args contains list of node ip addresses

	rand.Seed(time.Now().UTC().UnixNano())
	os.Exit(m.Run())
}

// TestEth tests if the node offers the JSON RPC as described on
// https://github.com/ethereum/wiki/wiki/JSON-RPC using go-ethereums
// ethclient package.
func TestEth(t *testing.T) {
	// ethclient test suite
	t.Run("ethclient", func(t *testing.T) {
		t.Run("http", func(t *testing.T) {
			clients := createHTTPClients(ips)

			t.Run("BalanceAndNonceAt", runTest(balanceAndNonceAtTest, clients))
			t.Run("BlockByHash", runTest(blockByHashTest, clients))
			t.Run("BlockByNumber", runTest(blockByNumberTest, clients))
			t.Run("CodeAt", runTest(CodeAtTest, clients))
			t.Run("CanonicalChain", runTest(canonicalChainTest, clients))
			t.Run("ContractDeployment", runTest(deployContractTest, clients))
			t.Run("ContractDeploymentOutOfGas", runTest(deployContractOutOfGasTest, clients))
			t.Run("EstimateGas", runTest(estimateGasTest, clients))
			t.Run("HeaderByHash", runTest(headerByHashTest, clients))
			t.Run("HeaderByNumber", runTest(headerByNumberTest, clients))
			t.Run("Receipt", runTest(receiptTest, clients))
			t.Run("SyncProgress", runTest(syncProgressTest, clients))
			t.Run("TransactionCount", runTest(transactionCountTest, clients))
			t.Run("TransactionInBlock", runTest(transactionInBlockTest, clients))
			t.Run("TransactionReceipt", runTest(TransactionReceiptTest, clients))
		})

		t.Run("websocket", func(t *testing.T) {
			clients := createWebsocketClients(ips)

			t.Run("BalanceAndNonceAt", runTest(balanceAndNonceAtTest, clients))
			t.Run("BlockByHash", runTest(blockByHashTest, clients))
			t.Run("BlockByNumber", runTest(blockByNumberTest, clients))
			t.Run("CanonicalChain", runTest(canonicalChainTest, clients))
			t.Run("CodeAt", runTest(CodeAtTest, clients))
			t.Run("ContractDeployment", runTest(deployContractTest, clients))
			t.Run("ContractDeploymentOutOfGas", runTest(deployContractOutOfGasTest, clients))
			t.Run("EstimateGas", runTest(estimateGasTest, clients))
			t.Run("HeaderByHash", runTest(headerByHashTest, clients))
			t.Run("HeaderByNumber", runTest(headerByNumberTest, clients))
			t.Run("Receipt", runTest(receiptTest, clients))
			t.Run("SyncProgress", runTest(syncProgressTest, clients))
			t.Run("TransactionCount", runTest(transactionCountTest, clients))
			t.Run("TransactionInBlock", runTest(transactionInBlockTest, clients))
			t.Run("TransactionReceipt", runTest(TransactionReceiptTest, clients))

			// tests that are only available on connections that support subscriptions
			t.Run("NewHeadSubscription", runTest(newHeadSubscriptionTest, clients))
			t.Run("LogSubscription", runTest(logSubscriptionTest, clients))
			t.Run("TransactionInBlockSubscription", runTest(transactionInBlockSubscriptionTest, clients))
		})
	})

	// ABI test suite
	t.Run("abi", func(t *testing.T) {
		t.Run("http", func(t *testing.T) {
			clients := createHTTPClients(ips)

			t.Run("CallContract", runTest(callContractTest, clients))
			t.Run("TransactContract", runTest(transactContractTest, clients))
		})

		t.Run("websocket", func(t *testing.T) {
			clients := createWebsocketClients(ips)

			t.Run("CallContract", runTest(callContractTest, clients))
			t.Run("TransactContract", runTest(transactContractTest, clients))

			// tests that are only available on connections that support subscriptions
			t.Run("TransactContractSubscription", runTest(transactContractSubscriptionTest, clients))
		})
	})
}
