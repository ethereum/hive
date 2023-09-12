package test

import (
	"context"
	"math/big"
	"net/http"
	"time"

	"github.com/ethereum/hive/simulators/ethereum/engine/client"
	"github.com/ethereum/hive/simulators/ethereum/engine/client/hive_rpc"
	"github.com/ethereum/hive/simulators/ethereum/engine/clmock"
	"github.com/ethereum/hive/simulators/ethereum/engine/config"
	"github.com/ethereum/hive/simulators/ethereum/engine/globals"
	"github.com/ethereum/hive/simulators/ethereum/engine/helper"

	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/hive/hivesim"
)

// Env is the environment of a single test.
type Env struct {
	*hivesim.T
	*helper.TransactionSender
	TestName string
	Client   *hivesim.Client

	// Timeout context signals that the test must wrap up its execution
	TimeoutContext context.Context

	// Test context will be done after timeout to allow test to gracefully finish
	TestContext context.Context

	// RPC Clients
	Engine      client.EngineClient
	Eth         client.Eth
	TestEngine  *TestEngineClient
	HiveEngine  *hive_rpc.HiveRPCEngineClient
	Engines     []client.EngineClient
	TestEngines []*TestEngineClient

	// Consensus Layer Mocker Instance
	CLMock *clmock.CLMocker

	// Client parameters used to launch the default client
	Genesis      *core.Genesis
	ForkConfig   *config.ForkConfig
	ClientParams hivesim.Params
	ClientFiles  hivesim.Params

	// Sets the type of transactions to use during the test
	TestTransactionType helper.TestTransactionType
}

func Run(testSpec Spec, ttd *big.Int, timeout time.Duration, t *hivesim.T, c *hivesim.Client, genesis *core.Genesis, cParams hivesim.Params, cFiles hivesim.Params) {
	// Setup the CL Mocker for this test
	forkConfig := testSpec.GetForkConfig()
	clMocker := clmock.NewCLMocker(
		t,
		genesis,
		forkConfig,
	)

	// Send the CLMocker for configuration by the spec, if any.
	testSpec.ConfigureCLMock(clMocker)

	// Defer closing all clients
	defer func() {
		clMocker.CloseClients()
	}()

	// Create Engine client from main hivesim.Client to be used by tests
	ec := hive_rpc.NewHiveRPCEngineClient(c, globals.EnginePortHTTP, globals.EthPortHTTP, globals.DefaultJwtTokenSecretBytes, ttd, &helper.LoggingRoundTrip{
		Logger: t,
		ID:     c.Container,
		Inner:  http.DefaultTransport,
	})
	defer ec.Close()

	// Add main client to CLMocker
	clMocker.AddEngineClient(ec)

	env := &Env{
		T:                   t,
		TransactionSender:   helper.NewTransactionSender(globals.TestAccounts, false),
		TestName:            testSpec.GetName(),
		Client:              c,
		Engine:              ec,
		Engines:             make([]client.EngineClient, 0),
		Eth:                 ec,
		HiveEngine:          ec,
		CLMock:              clMocker,
		Genesis:             genesis,
		ForkConfig:          forkConfig,
		ClientParams:        cParams,
		ClientFiles:         cFiles,
		TestTransactionType: testSpec.GetTestTransactionType(),
	}
	env.Engines = append(env.Engines, ec)
	env.TestEngines = append(env.TestEngines, env.TestEngine)

	// Before running the test, make sure Eth and Engine ports are open for the client
	if err := hive_rpc.CheckEthEngineLive(c); err != nil {
		t.Fatalf("FAIL (%s): Ports were never open for client: %v", env.TestName, err)
	}

	// Full test context has a few more seconds to finish up after timeout happens
	ctx, cancel := context.WithTimeout(context.Background(), timeout+(time.Second*10))
	defer cancel()
	env.TestContext = ctx
	clMocker.TestContext = ctx

	// Setup context timeout
	ctx, cancel = context.WithTimeout(ctx, timeout)
	defer cancel()
	env.TimeoutContext = ctx
	clMocker.TimeoutContext = ctx

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
	testSpec.Execute(env)
}

func (t *Env) MainTTD() *big.Int {
	return t.Engine.TerminalTotalDifficulty()
}

func (t *Env) HandleClientPostRunVerification(ec client.EngineClient) {
	if err := ec.PostRunVerifications(); err != nil {
		t.Fatalf("FAIL (%s): Client failed post-run verification: %v", t.TestName, err)
	}
}
