package main

import (
	"fmt"
	"os"

	"github.com/ethereum/hive/simulators/ethereum/rpc/eth/common/providers/hive"
)

var clientEnv = map[string]string{
	"HIVE_NETWORK_ID":     "7",
	"HIVE_CHAIN_ID":       "7",
	"HIVE_FORK_HOMESTEAD": "0",
	"HIVE_FORK_TANGERINE": "0",
	"HIVE_FORK_SPURIOUS":  "0",
	"HIVE_MINER":          "0xc5065c9eeebe6df2c2284d046bfc906501846c51",
	"HIVE_FAKE_POW":       "1",
}

var files = map[string]string{
	"/genesis.json": "genesis.json",
}

var tests = []hive.SingleClientTest{
	// HTTP RPC tests.
	{Name: "http/BalanceAndNonceAt", Run: runHTTP(balanceAndNonceAtTest)},
	{Name: "http/BlockByHash", Run: runHTTP(blockByHashTest)},
	{Name: "http/BlockByNumber", Run: runHTTP(blockByNumberTest)},
	{Name: "http/CodeAt", Run: runHTTP(CodeAtTest)},
	{Name: "http/CanonicalChain", Run: runHTTP(canonicalChainTest)},
	{Name: "http/ContractDeployment", Run: runHTTP(deployContractTest)},
	{Name: "http/ContractDeploymentOutOfGas", Run: runHTTP(deployContractOutOfGasTest)},
	{Name: "http/EstimateGas", Run: runHTTP(estimateGasTest)},
	{Name: "http/HeaderByHash", Run: runHTTP(headerByHashTest)},
	{Name: "http/HeaderByNumber", Run: runHTTP(headerByNumberTest)},
	{Name: "http/Receipt", Run: runHTTP(receiptTest)},
	{Name: "http/SyncProgress", Run: runHTTP(syncProgressTest)},
	{Name: "http/TransactionCount", Run: runHTTP(transactionCountTest)},
	{Name: "http/TransactionInBlock", Run: runHTTP(transactionInBlockTest)},
	{Name: "http/TransactionReceipt", Run: runHTTP(TransactionReceiptTest)},

	// HTTP ABI tests.

	// WebSocket RPC tests.

	// WebSocket ABI tests.
}

func withEnvAndFiles(tests []hive.SingleClientTest) []hive.SingleClientTest {
	out := make([]hive.SingleClientTest, len(tests))
	for i, test := range tests {
		out[i] = test
		out[i].Parameters = clientEnv
		out[i].Files = files
	}
	return out
}

func main() {
	host := hive.New()
	tests := withEnvAndFiles(tests)
	err := hive.RunAllClients(host, "JSON-RPC tests", tests)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// func TestEth(t *testing.T) {
// 	// ethclient test suite
// 	t.Run("ethclient", func(t *testing.T) {
// 		t.Run("http", func(t *testing.T) {
// 			clients := createHTTPClients(ips)
// 		})

// t.Run("websocket", func(t *testing.T) {
// 	clients := createWebsocketClients(ips)
//
// 	t.Run("BalanceAndNonceAt", runTest(balanceAndNonceAtTest, clients))
// 	t.Run("BlockByHash", runTest(blockByHashTest, clients))
// 	t.Run("BlockByNumber", runTest(blockByNumberTest, clients))
// 	t.Run("CanonicalChain", runTest(canonicalChainTest, clients))
// 	t.Run("CodeAt", runTest(CodeAtTest, clients))
// 	t.Run("ContractDeployment", runTest(deployContractTest, clients))
// 	t.Run("ContractDeploymentOutOfGas", runTest(deployContractOutOfGasTest, clients))
// 	t.Run("EstimateGas", runTest(estimateGasTest, clients))
// 	t.Run("HeaderByHash", runTest(headerByHashTest, clients))
// 	t.Run("HeaderByNumber", runTest(headerByNumberTest, clients))
// 	t.Run("Receipt", runTest(receiptTest, clients))
// 	t.Run("SyncProgress", runTest(syncProgressTest, clients))
// 	t.Run("TransactionCount", runTest(transactionCountTest, clients))
// 	t.Run("TransactionInBlock", runTest(transactionInBlockTest, clients))
// 	t.Run("TransactionReceipt", runTest(TransactionReceiptTest, clients))
//
// 	// tests that are only available on connections that support subscriptions
// 	t.Run("NewHeadSubscription", runTest(newHeadSubscriptionTest, clients))
// 	t.Run("LogSubscription", runTest(logSubscriptionTest, clients))
// 	t.Run("TransactionInBlockSubscription", runTest(transactionInBlockSubscriptionTest, clients))
// })
// })

// // ABI test suite
// t.Run("abi", func(t *testing.T) {
// 	t.Run("http", func(t *testing.T) {
// 		clients := createHTTPClients(ips)
//
// 		t.Run("CallContract", runTest(callContractTest, clients))
// 		t.Run("TransactContract", runTest(transactContractTest, clients))
// 	})
//
// 	t.Run("websocket", func(t *testing.T) {
// 		clients := createWebsocketClients(ips)
//
// 		t.Run("CallContract", runTest(callContractTest, clients))
// 		t.Run("TransactContract", runTest(transactContractTest, clients))
//
// 		// tests that are only available on connections that support subscriptions
// 		t.Run("TransactContractSubscription", runTest(transactContractSubscriptionTest, clients))
// 	})
// })
// }
