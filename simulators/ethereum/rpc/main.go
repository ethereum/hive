package main

import (
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/hive/hivesim"
)

var (
	// parameters used for signing transactions
	chainID  = big.NewInt(7)
	gasPrice = big.NewInt(30 * params.GWei)

	// would be nice to use a networkID that's different from chainID,
	// but some clients don't support the distinction properly.
	networkID = big.NewInt(7)
)

var clientEnv = hivesim.Params{
	"HIVE_NETWORK_ID":     networkID.String(),
	"HIVE_CHAIN_ID":       chainID.String(),
	"HIVE_FORK_HOMESTEAD": "0",
	"HIVE_FORK_TANGERINE": "0",
	"HIVE_FORK_SPURIOUS":  "0",
	// All tests use clique PoA to mine new blocks.
	"HIVE_CLIQUE_PERIOD":     "1",
	"HIVE_CLIQUE_PRIVATEKEY": "9c647b8b7c4e7c3490668fb6c11473619db80c93704c70893d3813af4090c39c",
	"HIVE_MINER":             "658bdf435d810c91414ec09147daa6db62406379",
}

var files = map[string]string{
	"/genesis.json": "./init/genesis.json",
}

type testSpec struct {
	Name  string
	About string
	Run   func(*hivesim.T, *hivesim.Client)
}

var tests = []testSpec{
	// HTTP RPC tests.
	{Name: "http/BalanceAndNonceAt", Run: runHTTP(balanceAndNonceAtTest)},
	{Name: "http/CanonicalChain", Run: runHTTP(canonicalChainTest)},
	{Name: "http/CodeAt", Run: runHTTP(CodeAtTest)},
	{Name: "http/ContractDeployment", Run: runHTTP(deployContractTest)},
	{Name: "http/ContractDeploymentOutOfGas", Run: runHTTP(deployContractOutOfGasTest)},
	{Name: "http/EstimateGas", Run: runHTTP(estimateGasTest)},
	{Name: "http/GenesisBlockByHash", Run: runHTTP(genesisBlockByHashTest)},
	{Name: "http/GenesisBlockByNumber", Run: runHTTP(genesisBlockByNumberTest)},
	{Name: "http/GenesisHeaderByHash", Run: runHTTP(genesisHeaderByHashTest)},
	{Name: "http/GenesisHeaderByNumber", Run: runHTTP(genesisHeaderByNumberTest)},
	{Name: "http/Receipt", Run: runHTTP(receiptTest)},
	{Name: "http/SyncProgress", Run: runHTTP(syncProgressTest)},
	{Name: "http/TransactionCount", Run: runHTTP(transactionCountTest)},
	{Name: "http/TransactionInBlock", Run: runHTTP(transactionInBlockTest)},
	{Name: "http/TransactionReceipt", Run: runHTTP(TransactionReceiptTest)},

	// HTTP ABI tests.
	{Name: "http/ABICall", Run: runHTTP(callContractTest)},
	{Name: "http/ABITransact", Run: runHTTP(transactContractTest)},

	// WebSocket RPC tests.
	{Name: "ws/BalanceAndNonceAt", Run: runWS(balanceAndNonceAtTest)},
	{Name: "ws/CanonicalChain", Run: runWS(canonicalChainTest)},
	{Name: "ws/CodeAt", Run: runWS(CodeAtTest)},
	{Name: "ws/ContractDeployment", Run: runWS(deployContractTest)},
	{Name: "ws/ContractDeploymentOutOfGas", Run: runWS(deployContractOutOfGasTest)},
	{Name: "ws/EstimateGas", Run: runWS(estimateGasTest)},
	{Name: "ws/GenesisBlockByHash", Run: runWS(genesisBlockByHashTest)},
	{Name: "ws/GenesisBlockByNumber", Run: runWS(genesisBlockByNumberTest)},
	{Name: "ws/GenesisHeaderByHash", Run: runWS(genesisHeaderByHashTest)},
	{Name: "ws/GenesisHeaderByNumber", Run: runWS(genesisHeaderByNumberTest)},
	{Name: "ws/Receipt", Run: runWS(receiptTest)},
	{Name: "ws/SyncProgress", Run: runWS(syncProgressTest)},
	{Name: "ws/TransactionCount", Run: runWS(transactionCountTest)},
	{Name: "ws/TransactionInBlock", Run: runWS(transactionInBlockTest)},
	{Name: "ws/TransactionReceipt", Run: runWS(TransactionReceiptTest)},

	// WebSocket subscription tests.
	{Name: "ws/NewHeadSubscription", Run: runWS(newHeadSubscriptionTest)},
	{Name: "ws/LogSubscription", Run: runWS(logSubscriptionTest)},
	{Name: "ws/TransactionInBlockSubscription", Run: runWS(transactionInBlockSubscriptionTest)},

	// WebSocket ABI tests.
	{Name: "ws/ABICall", Run: runWS(callContractTest)},
	{Name: "ws/ABITransact", Run: runWS(transactContractTest)},
}

func main() {
	suite := hivesim.Suite{
		Name: "rpc",
		Description: `
The RPC test suite runs a set of RPC related tests against a running node. It tests
several real-world scenarios such as sending value transactions, deploying a contract or
interacting with one.`[1:],
	}
	suite.Add(&hivesim.ClientTestSpec{
		Name:        "client launch",
		Description: `This test launches the client and collects its logs.`,
		Parameters:  clientEnv,
		Files:       files,
		Run:         runAllTests,
	})
	hivesim.MustRunSuite(hivesim.New(), suite)
}

// runAllTests runs the tests against a client instance.
// Most tests simply wait for tx inclusion in a block so we can run many tests concurrently.
func runAllTests(t *hivesim.T, c *hivesim.Client) {
	s := newSemaphore(16)
	for _, test := range tests {
		test := test
		s.get()
		go func() {
			defer s.put()
			t.Run(hivesim.TestSpec{
				Name:        fmt.Sprintf("%s (%s)", test.Name, c.Type),
				Description: test.About,
				Run:         func(t *hivesim.T) { test.Run(t, c) },
			})
		}()
	}
	s.drain()
}

type semaphore chan struct{}

func newSemaphore(n int) semaphore {
	s := make(semaphore, n)
	for i := 0; i < n; i++ {
		s <- struct{}{}
	}
	return s
}

func (s semaphore) get() { <-s }
func (s semaphore) put() { s <- struct{}{} }

func (s semaphore) drain() {
	for i := 0; i < cap(s); i++ {
		<-s
	}
}
