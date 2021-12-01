package main

import (
	"fmt"
	"math/big"
	"strings"
	"time"

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

	// PoS related
	terminalTotalDifficulty = big.NewInt(131072 + 25)
	blockProductionPoS      = time.Second * 1
	clMocker                = NewCLMocker(terminalTotalDifficulty)
	tTDCheck                = time.Second * 1
)

var clientEnv = hivesim.Params{
	"HIVE_NETWORK_ID":          networkID.String(),
	"HIVE_CHAIN_ID":            chainID.String(),
	"HIVE_FORK_HOMESTEAD":      "0",
	"HIVE_FORK_TANGERINE":      "0",
	"HIVE_FORK_SPURIOUS":       "0",
	"HIVE_FORK_BYZANTIUM":      "0",
	"HIVE_FORK_CONSTANTINOPLE": "0",
	"HIVE_FORK_PETERSBURG":     "0",
	"HIVE_FORK_ISTANBUL":       "0",
	"HIVE_FORK_MUIR_GLACIER":   "0",
	"HIVE_FORK_BERLIN":         "0",
	"HIVE_FORK_LONDON":         "0",
	// All tests use clique PoA to mine new blocks.
	"HIVE_CLIQUE_PERIOD":     "1",
	"HIVE_CLIQUE_PRIVATEKEY": "9c647b8b7c4e7c3490668fb6c11473619db80c93704c70893d3813af4090c39c",
	"HIVE_MINER":             "658bdf435d810c91414ec09147daa6db62406379",
	// Merge related
	"HIVE_TERMINAL_TOTAL_DIFFICULTY": terminalTotalDifficulty.String(),
}

var files = map[string]string{
	"/genesis.json": "./init/genesis.json",
}

type testSpec struct {
	Name  string
	About string
	Run   func(*TestEnv)
}

var tests = []testSpec{
	// HTTP RPC tests.
	{Name: "http/BalanceAndNonceAt", Run: balanceAndNonceAtTest},
	{Name: "http/CanonicalChain", Run: canonicalChainTest},
	{Name: "http/CodeAt", Run: CodeAtTest},
	{Name: "http/ContractDeployment", Run: deployContractTest},
	{Name: "http/ContractDeploymentOutOfGas", Run: deployContractOutOfGasTest},
	{Name: "http/EstimateGas", Run: estimateGasTest},
	{Name: "http/GenesisBlockByHash", Run: genesisBlockByHashTest},
	{Name: "http/GenesisBlockByNumber", Run: genesisBlockByNumberTest},
	{Name: "http/GenesisHeaderByHash", Run: genesisHeaderByHashTest},
	{Name: "http/GenesisHeaderByNumber", Run: genesisHeaderByNumberTest},
	{Name: "http/Receipt", Run: receiptTest},
	{Name: "http/SyncProgress", Run: syncProgressTest},
	{Name: "http/TransactionCount", Run: transactionCountTest},
	{Name: "http/TransactionInBlock", Run: transactionInBlockTest},
	{Name: "http/TransactionReceipt", Run: TransactionReceiptTest},

	// HTTP ABI tests.
	{Name: "http/ABICall", Run: callContractTest},
	{Name: "http/ABITransact", Run: transactContractTest},

	// Random opcode tests
	{Name: "http/RandomOpcodeTx", Run: randomOpcodeTx},

	/*
		// WebSocket RPC tests.
		{Name: "ws/BalanceAndNonceAt", Run: balanceAndNonceAtTest},
		{Name: "ws/CanonicalChain", Run: canonicalChainTest},
		{Name: "ws/CodeAt", Run: CodeAtTest},
		{Name: "ws/ContractDeployment", Run: deployContractTest},
		{Name: "ws/ContractDeploymentOutOfGas", Run: deployContractOutOfGasTest},
		{Name: "ws/EstimateGas", Run: estimateGasTest},
		{Name: "ws/GenesisBlockByHash", Run: genesisBlockByHashTest},
		{Name: "ws/GenesisBlockByNumber", Run: genesisBlockByNumberTest},
		{Name: "ws/GenesisHeaderByHash", Run: genesisHeaderByHashTest},
		{Name: "ws/GenesisHeaderByNumber", Run: genesisHeaderByNumberTest},
		{Name: "ws/Receipt", Run: receiptTest},
		{Name: "ws/SyncProgress", Run: syncProgressTest},
		{Name: "ws/TransactionCount", Run: transactionCountTest},
		{Name: "ws/TransactionInBlock", Run: transactionInBlockTest},
		{Name: "ws/TransactionReceipt", Run: TransactionReceiptTest},

		// WebSocket subscription tests.
		{Name: "ws/NewHeadSubscription", Run: newHeadSubscriptionTest},
		{Name: "ws/LogSubscription", Run: logSubscriptionTest},
		{Name: "ws/TransactionInBlockSubscription", Run: transactionInBlockSubscriptionTest},

		// WebSocket ABI tests.
		{Name: "ws/ABICall", Run: callContractTest},
		{Name: "ws/ABITransact", Run: transactContractTest}, */
}

func main() {
	suite := hivesim.Suite{
		Name: "engine",
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
	vault := newVault()
	ec := NewCatalystClient(t, c)
	clMocker.AddCatalystClient(ec)
	s := newSemaphore(16)
	for _, test := range tests {
		test := test
		s.get()
		go func() {
			defer s.put()
			t.Run(hivesim.TestSpec{
				Name:        fmt.Sprintf("%s (%s)", test.Name, c.Type),
				Description: test.About,
				Run: func(t *hivesim.T) {
					switch test.Name[:strings.IndexByte(test.Name, '/')] {
					case "http":
						runHTTP(t, c, vault, clMocker, test.Run)
					case "ws":
						runWS(t, c, vault, clMocker, test.Run)
					default:
						panic("bad test prefix in name " + test.Name)
					}
				},
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
