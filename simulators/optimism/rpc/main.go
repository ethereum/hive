package main

import (
	"context"
	"encoding/json"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/types"
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
	Run   func(*TestEnv)
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

	seqWindowSize := uint64(4)
	d := optimism.NewDevnet(t)

	d.InitContracts()
	d.InitHardhatDeployConfig("earliest", 10, seqWindowSize, 30)
	d.InitL1Hardhat()
	d.AddEth1() // l1 eth1 node is required for l2 config init
	d.WaitUpEth1(0, time.Second*10)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()
	var block *types.Block
	for {
		height, err := d.GetEth1(0).EthClient().BlockNumber(ctx)
		require.NoError(t, err)
		d.T.Logf("waiting for one sequencing window. size: %d, height: %d", seqWindowSize, height)
		if height < seqWindowSize {
			time.Sleep(time.Second)
			continue
		}
		block, err = d.GetEth1(0).EthClient().BlockByNumber(ctx, big.NewInt(int64(height)))
		require.NoError(t, err)
		break
	}
	d.InitHardhatDeployConfig(block.Hash().Hex(), 10, seqWindowSize, 30)

	// deploy contracts
	d.DeployL1Hardhat()

	d.InitL2Hardhat(core.GenesisAlloc{
		common.HexToAddress("0x0000000000000000000000000000000000000314"): {
			Balance: big.NewInt(0),
			Code:    runtimeCode,
			Storage: map[common.Hash]common.Hash{
				common.Hash{}: common.BytesToHash(common.LeftPadBytes(big.NewInt(0x1234).Bytes(), 32)),
				common.HexToHash("0x6661e9d6d8b923d5bbaab1b96e1dd51ff6ea2a93520fdc9eb75d059238b8c5e9"): common.BytesToHash(common.LeftPadBytes(big.NewInt(1).Bytes(), 32)),
			},
		},
	})

	d.AddOpL2() // l2 engine is required for rollup config init
	d.WaitUpOpL2Engine(0, time.Second*10)
	d.InitRollupHardhat()

	// sequencer stack, on top of first eth1 node
	d.AddOpNode(0, 0, true)
	d.AddOpBatcher(0, 0, 0)
	// proposer does not need to run for L2 to be stable
	//d.AddOpProposer(0, 0, 0)

	// verifier stack (optional)
	//d.AddOpL2()
	//d.AddOpNode(0, 1)  // can use the same eth1 node

	expHeight := uint64(time.Now().Sub(time.Unix(int64(block.Time()), 0)).Seconds() / 2)
	ec := d.GetOpL2Engine(0).EthClient()
	for {
		height, err := ec.BlockNumber(ctx)
		require.NoError(t, err)
		d.T.Logf("waiting for l2 to create empty blocks. height: %d, expected height: %d", height, expHeight)
		if height < expHeight {
			time.Sleep(time.Second)
			continue
		}
		break
	}
	c := d.GetOpL2Engine(0).Client

	vault := newVault()
	genesis, err := json.Marshal(d.L2Cfg)
	handleErr(err)

	s := newSemaphore(40)
	for _, test := range tests {
		test := test
		s.get()
		go func() {
			defer s.put()
			t.Run(hivesim.TestSpec{
				Name:        fmt.Sprintf("%s (%s)", test.Name, "ops-l2"),
				Description: test.About,
				Run: func(t *hivesim.T) {
					switch test.Name[:strings.IndexByte(test.Name, '/')] {
					case "http":
						runHTTP(t, c, vault, genesis, test.Run)
					case "ws":
						runWS(t, c, vault, genesis, test.Run)
					default:
						panic("bad test prefix in name " + test.Name)
					}
				},
			})
			// TODO: debug re-org issue and remove
			time.Sleep(5 * time.Second)
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
