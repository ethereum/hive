package main

import (
	"context"
	"encoding/json"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/stretchr/testify/require"
	"math/big"
	"time"

	"fmt"
	"strings"

	"github.com/ethereum/hive/hivesim"
	"github.com/ethereum/hive/optimism"
)

type testSpec struct {
	Name  string
	About string
	Run   func(*LegacyTestEnv)
}

var tests = []testSpec{
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

	// WebSocket ABI tests.
	{Name: "ws/ABICall", Run: callContractTest},
	{Name: "ws/ABITransact", Run: transactContractTest},

	// WebSocket subscription tests.
	{Name: "ws/NewHeadSubscription", Run: newHeadSubscriptionTest},
	{Name: "ws/LogSubscription", Run: logSubscriptionTest},
	{Name: "ws/TransactionInBlockSubscription", Run: transactionInBlockSubscriptionTest},

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
}

func main() {
	suite := hivesim.Suite{
		Name: "optimism rpc",
		Description: `
The RPC test suite runs a set of RPC related tests against a running node. It tests
several real-world scenarios such as sending value transactions, deploying a contract or
interacting with one.`[1:],
	}

	// Add tests for full nodes.
	suite.Add(&hivesim.TestSpec{
		Name:        "client launch",
		Description: `This test launches the client and collects its logs.`,
		Run:         func(t *hivesim.T) { runAllTests(t) },
	})

	sim := hivesim.New()
	hivesim.MustRunSuite(sim, suite)
}

// runAllTests runs the tests against a client instance.
// Most tests simply wait for tx inclusion in a block so we can run many tests concurrently.
func runAllTests(t *hivesim.T) {
	handleErr := func(err error) {
		if err != nil {
			t.Fatal(err)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	seqWindowSize := uint64(4)
	d := optimism.NewDevnet(t)
	require.NoError(t, optimism.StartSequencerDevnet(ctx, d, &optimism.SequencerDevnetParams{
		MaxSeqDrift:   10,
		SeqWindowSize: seqWindowSize,
		ChanTimeout:   30,
		AdditionalGenesisAllocs: core.GenesisAlloc{
			common.HexToAddress("0x0000000000000000000000000000000000000314"): {
				Balance: big.NewInt(0),
				Code:    runtimeCode,
				Storage: map[common.Hash]common.Hash{
					common.Hash{}: common.BytesToHash(common.LeftPadBytes(big.NewInt(0x1234).Bytes(), 32)),
					common.HexToHash("0x6661e9d6d8b923d5bbaab1b96e1dd51ff6ea2a93520fdc9eb75d059238b8c5e9"): common.BytesToHash(common.LeftPadBytes(big.NewInt(1).Bytes(), 32)),
				},
			},
		},
	}))

	c := d.GetOpL2Engine(0).Client
	genesis, err := json.Marshal(d.L2Cfg)
	handleErr(err)

	// Need to adapt the tests a bit to work with the common
	// libraries in the optimism package.
	adaptedTests := make([]*optimism.TestSpec, len(tests))
	for i, test := range tests {
		adaptedTests[i] = &optimism.TestSpec{
			Name:        fmt.Sprintf("%s (%s)", test.Name, "ops-l2"),
			Description: test.About,
			Run: func(t *hivesim.T, env *optimism.TestEnv) {
				switch test.Name[:strings.IndexByte(test.Name, '/')] {
				case "http":
					RunHTTP(t, c, d.L2Vault, genesis, test.Run)
				case "ws":
					RunWS(t, c, d.L2Vault, genesis, test.Run)
				default:
					panic("bad test prefix in name " + test.Name)
				}
			},
		}
	}
	optimism.RunTests(ctx, t, &optimism.RunTestsParams{
		Devnet:      d,
		Tests:       adaptedTests,
		Concurrency: 40,
	})
}
