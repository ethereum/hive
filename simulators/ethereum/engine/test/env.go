package test

import (
	"context"
	"math/big"
	"net/http"
	"time"

	"github.com/ethereum/hive/simulators/ethereum/engine/client"
	"github.com/ethereum/hive/simulators/ethereum/engine/client/hive_rpc"
	"github.com/ethereum/hive/simulators/ethereum/engine/clmock"
	"github.com/ethereum/hive/simulators/ethereum/engine/globals"
	"github.com/ethereum/hive/simulators/ethereum/engine/helper"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/hive/hivesim"
)

// Env is the environment of a single test.
type Env struct {
	*hivesim.T
	TestName    string
	Client      *hivesim.Client
	TestContext context.Context

	// RPC Clients
	Engine     client.EngineClient
	Eth        client.Eth
	TestEngine *TestEngineClient
	HiveEngine *hive_rpc.HiveRPCEngineClient

	// Consensus Layer Mocker Instance
	CLMock *clmock.CLMocker

	// Client parameters used to launch the default client
	ClientParams hivesim.Params
	ClientFiles  hivesim.Params

	// Sets the type of transactions to use during the test
	TestTransactionType helper.TestTransactionType
}

func Run(testName string, ttd *big.Int, slotsToSafe *big.Int, slotsToFinalized *big.Int, timeout time.Duration, t *hivesim.T, c *hivesim.Client, fn func(*Env), cParams hivesim.Params, cFiles hivesim.Params, testTransactionType helper.TestTransactionType) {
	// Setup the CL Mocker for this test
	clMocker := clmock.NewCLMocker(t, slotsToSafe, slotsToFinalized)
	// Defer closing all clients
	defer func() {
		clMocker.CloseClients()
	}()

	// Create Engine client from main hivesim.Client to be used by tests
	ec := hive_rpc.NewHiveRPCEngineClient(c, globals.EnginePortHTTP, globals.EthPortHTTP, globals.DefaultJwtTokenSecretBytes, ttd, &helper.LoggingRoundTrip{
		T:     t,
		Hc:    c,
		Inner: http.DefaultTransport,
	})
	defer ec.Close()

	// Add main client to CLMocker
	clMocker.AddEngineClient(ec)

	env := &Env{
		T:                   t,
		TestName:            testName,
		Client:              c,
		Engine:              ec,
		Eth:                 ec,
		HiveEngine:          ec,
		CLMock:              clMocker,
		ClientParams:        cParams,
		ClientFiles:         cFiles,
		TestTransactionType: testTransactionType,
	}

	// Before running the test, make sure Eth and Engine ports are open for the client
	if err := hive_rpc.CheckEthEngineLive(c); err != nil {
		t.Fatalf("FAIL (%s): Ports were never open for client: %v", env.TestName, err)
	}

	// Setup context timeout
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	env.TestContext = ctx
	// CL Mocker has some extra time to be able to produce the last block after the test has finished
	ctx, cancel = context.WithTimeout(context.Background(), timeout+(time.Second*10))
	defer cancel()
	clMocker.TestContext = ctx

	// Create the test-expect object
	env.TestEngine = NewTestEngineClient(env, ec)

	// Defer producing one last block to verify Execution client did not break after the test
	defer func() {
		// Only run if the TTD was reached during test, and test had not failed at this point.
		if clMocker.TTDReached && !t.Failed() {
			clMocker.ProduceSingleBlock(clmock.BlockProcessCallbacks{})
		}
	}()

	// Run the test
	fn(env)
}

func (t *Env) MainTTD() *big.Int {
	return t.Engine.TerminalTotalDifficulty()
}

// Verify that the client progresses after a certain PoW block still in PoW mode
func (t *Env) VerifyPoWProgress(lastBlockHash common.Hash) {
	// Get the block number first
	ctx, cancel := context.WithTimeout(t.TestContext, globals.RPCTimeout)
	defer cancel()
	lb, err := t.Eth.BlockByHash(ctx, lastBlockHash)
	if err != nil {
		t.Fatalf("FAIL (%s): Unable to fetch block: %v", t.TestName, err)
	}
	nextNum := lb.Number().Int64() + 1
	for {
		ctx, cancel = context.WithTimeout(t.TestContext, globals.RPCTimeout)
		defer cancel()
		nh, err := t.Eth.HeaderByNumber(ctx, big.NewInt(nextNum))
		if err == nil && nh != nil {
			// Chain has progressed, check that the next block is also PoW
			// Difficulty must NOT be zero
			if nh.Difficulty.Cmp(common.Big0) == 0 {
				t.Fatalf("FAIL (%s): Expected PoW chain to progress in PoW mode, but following block difficulty==%v", t.TestName, nh.Difficulty)
			}
			// Chain is still PoW/Clique
			return
		}
		t.Logf("INFO (%s): Error getting block, will try again: %v", t.TestName, err)
		select {
		case <-t.TestContext.Done():
			t.Fatalf("FAIL (%s): Timeout while waiting for PoW chain to progress", t.TestName)
		case <-time.After(time.Second):
		}
	}
}
