package test

import (
	"context"
	"math/big"
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
	TestName string

	// Timeout context signals that the test must wrap up its execution
	TimeoutContext context.Context

	// Test context will be done after timeout to allow test to gracefully finish
	TestContext context.Context

	// RPC Clients
	Clients    []client.EngineClient
	Engine     client.EngineClient
	Eth        client.Eth
	TestEngine *TestEngineClient

	// Consensus Layer Mocker Instance
	CLMock *clmock.CLMocker

	// Client parameters used to launch the default client
	ClientParams hivesim.Params
	ClientFiles  hivesim.Params

	// Sets the type of transactions to use during the test
	TestTransactionType helper.TestTransactionType
}

func Run(testName string, ttd *big.Int, slotsToSafe *big.Int, slotsToFinalized *big.Int, timeout time.Duration, t *hivesim.T, fn func(*Env), cParams hivesim.Params, cFiles hivesim.Params, testTransactionType helper.TestTransactionType, safeSlotsToImportOptimistically int64) {

	// Setup the CL Mocker for this test
	clMocker := clmock.NewCLMocker(t, slotsToSafe, slotsToFinalized, big.NewInt(safeSlotsToImportOptimistically))
	// Defer closing all clients
	defer func() {
		clMocker.CloseClients()
	}()

	// Set up test context, which has a few more seconds to finish up after timeout happens
	ctx, cancel := context.WithTimeout(context.Background(), timeout+(time.Second*10))
	defer cancel()
	clMocker.TestContext = ctx

	env := &Env{
		T:                   t,
		TestName:            testName,
		Clients:             make([]client.EngineClient, 0),
		CLMock:              clMocker,
		ClientParams:        cParams,
		ClientFiles:         cFiles,
		TestTransactionType: testTransactionType,
		TestContext:         ctx,
	}

	// Setup the main test client
	var ec client.EngineClient
	ec = env.StartNextClient(cParams, cFiles)
	defer ec.Close()

	// Create the test-expect object
	env.TestEngine = NewTestEngineClient(env, ec)

	// Add main client to CLMocker
	clMocker.AddEngineClient(ec)

	// Setup context timeout
	ctx, cancel = context.WithTimeout(ctx, timeout)
	defer cancel()
	env.TimeoutContext = ctx
	clMocker.TimeoutContext = ctx

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

func (t *Env) NextClientType() *hivesim.ClientDefinition {
	testClientTypes, err := t.Sim.ClientTypes()
	if err != nil {
		t.Fatalf("No client types")
	}
	nextClientTypeIndex := len(t.Clients) % len(testClientTypes)
	return testClientTypes[nextClientTypeIndex]
}

func (t *Env) ClientStarter(clientType string) client.EngineStarter {
	return hive_rpc.HiveRPCEngineStarter{
		ClientType: clientType,
	}
}

func (t *Env) NextClientStarter() client.EngineStarter {
	return t.ClientStarter(t.NextClientType().Name)
}

func (t *Env) StartClient(clientType string, ClientParams hivesim.Params, ClientFiles hivesim.Params, bootclients ...client.EngineClient) client.EngineClient {
	ec, err := t.ClientStarter(clientType).StartClient(t.T, t.TestContext, ClientParams, ClientFiles, bootclients...)
	if err != nil {
		t.Fatalf("FAIL (%s): Unable to start client: %v", t.TestName, err)
	}
	if len(t.Clients) == 0 {
		t.Engine = ec
		t.Eth = ec
	}
	t.Clients = append(t.Clients, ec)
	return ec
}

func (t *Env) StartNextClient(ClientParams hivesim.Params, ClientFiles hivesim.Params, bootclients ...client.EngineClient) client.EngineClient {
	ec, err := t.NextClientStarter().StartClient(t.T, t.TestContext, ClientParams, ClientFiles, bootclients...)
	if err != nil {
		t.Fatalf("FAIL (%s): Unable to start client: %v", t.TestName, err)
	}
	if len(t.Clients) == 0 {
		t.Engine = ec
		t.Eth = ec
	}
	t.Clients = append(t.Clients, ec)
	return ec
}

func (t *Env) HandleClientPostRunVerification(ec client.EngineClient) {
	if err := ec.PostRunVerifications(); err != nil {
		t.Fatalf("FAIL (%s): Client failed post-run verification: %v", t.TestName, err)
	}
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
		case <-t.TimeoutContext.Done():
			t.Fatalf("FAIL (%s): Timeout while waiting for PoW chain to progress", t.TestName)
		case <-time.After(time.Second):
		}
	}
}
