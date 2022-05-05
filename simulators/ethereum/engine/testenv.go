package main

import (
	"context"
	"fmt"
	"math/big"
	"net/http"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/ethereum/hive/hivesim"
)

var (
	// This is the account that sends vault funding transactions.
	vaultAccountAddr = common.HexToAddress("0xcf49fda3be353c69b41ed96333cd24302da4556f")
	vaultKey, _      = crypto.HexToECDSA("63b508a03c3b5937ceb903af8b1b0c191012ef6eb7e9c3fb7afa94e5d214d376")
)

// TestEnv is the environment of a single test.
type TestEnv struct {
	*hivesim.T
	TestName string
	Client   *hivesim.Client

	// RPC Clients
	RPC        *rpc.Client
	Eth        *ethclient.Client
	Engine     *EngineClient
	TestEngine *TestEngineClient
	TestEth    *TestEthClient

	// Consensus Layer Mocker Instance
	CLMock *CLMocker

	// Global test timeout
	Timeout <-chan time.Time

	// Client parameters used to launch the default client
	ClientParams hivesim.Params
	ClientFiles  hivesim.Params

	// This tracks the account nonce of the vault account.
	nonce uint64

	// This holds most recent context created by the Ctx method.
	// Every time Ctx is called, it creates a new context with the default
	// timeout and cancels the previous one.
	lastCtx    context.Context
	lastCancel context.CancelFunc
	syncCancel context.CancelFunc
}

func RunTest(testName string, ttd *big.Int, timeout time.Duration, t *hivesim.T, c *hivesim.Client, fn func(*TestEnv), cParams hivesim.Params, cFiles hivesim.Params) {
	// Setup the CL Mocker for this test
	clMocker := NewCLMocker(t, ttd)
	// Defer closing all clients
	defer func() {
		clMocker.CloseClients()
	}()

	// Add main client to CLMocker
	clMocker.AddEngineClient(t, c, ttd)

	// This sets up debug logging of the requests and responses.
	client := &http.Client{
		Transport: &loggingRoundTrip{
			t:     t,
			hc:    c,
			inner: http.DefaultTransport,
		},
	}

	// Create Engine client from main hivesim.Client to be used by tests
	ec := NewEngineClient(t, c, ttd)
	defer ec.Close()

	rpcClient, _ := rpc.DialHTTPWithClient(fmt.Sprintf("http://%v:%v/", c.IP, EthPortHTTP), client)
	defer rpcClient.Close()
	env := &TestEnv{
		T:            t,
		TestName:     testName,
		Client:       c,
		RPC:          rpcClient,
		Eth:          ethclient.NewClient(rpcClient),
		Engine:       ec,
		CLMock:       clMocker,
		ClientParams: cParams,
		ClientFiles:  cFiles,
	}
	env.TestEngine = NewTestEngineClient(env, ec)
	env.TestEth = NewTestEthClient(env, env.Eth)

	// Defer closing the last context
	defer func() {
		if env.lastCtx != nil {
			env.lastCancel()
		}
	}()

	// Create test end channel and defer closing it
	testend := make(chan interface{})
	defer func() { close(testend) }()

	// Start thread to wait for client to be synced to the latest PoS block
	defer func() {
		if env.syncCancel != nil {
			env.syncCancel()
		}
	}()

	// Setup timeouts
	env.Timeout = time.After(timeout)
	clMocker.Timeout = time.After(timeout)

	// Defer producing one last block to verify Execution client did not break after the test
	defer func() {
		// Only run if the TTD was reached during test, and test had not failed at this point.
		if clMocker.TTDReached && !t.Failed() {
			clMocker.produceSingleBlock(BlockProcessCallbacks{})
		}
	}()

	// Run the test
	fn(env)
}

func (t *TestEnv) MainTTD() *big.Int {
	return t.Engine.TerminalTotalDifficulty
}

func (t *TestEnv) StartClient(clientType string, params hivesim.Params, ttd *big.Int) (*hivesim.Client, *EngineClient, error) {
	c := t.T.StartClient(clientType, params, hivesim.WithStaticFiles(t.ClientFiles))
	ec := NewEngineClient(t.T, c, ttd)
	return c, ec, nil
}

func (t *TestEnv) makeNextTransaction(recipient common.Address, amount *big.Int, payload []byte) *types.Transaction {

	gasLimit := uint64(75000)

	tx := types.NewTransaction(t.nonce, recipient, amount, gasLimit, gasPrice, payload)
	signer := types.NewEIP155Signer(chainID)
	signedTx, err := types.SignTx(tx, signer, vaultKey)
	if err != nil {
		t.Fatal("FAIL (%s): could not sign new tx: %v", t.TestName, err)
	}
	t.nonce++
	return signedTx
}

func (t *TestEnv) sendNextTransaction(sender *EngineClient, recipient common.Address, amount *big.Int, payload []byte) *types.Transaction {
	tx := t.makeNextTransaction(recipient, amount, payload)
	for {
		err := sender.Eth.SendTransaction(sender.Ctx(), tx)
		if err == nil {
			return tx
		}
		select {
		case <-time.After(time.Second):
		case <-t.Timeout:
			t.Fatalf("FAIL (%s): Timeout while trying to send transaction: %v", t.TestName, err)
		}
	}
}

// CallContext is a helper method that forwards a raw RPC request to
// the underlying RPC client. This can be used to call RPC methods
// that are not supported by the ethclient.Client.
func (t *TestEnv) CallContext(ctx context.Context, result interface{}, method string, args ...interface{}) error {
	return t.RPC.CallContext(ctx, result, method, args...)
}

// Ctx returns a context with the default timeout.
// For subsequent calls to Ctx, it also cancels the previous context.
func (t *TestEnv) Ctx() context.Context {
	if t.lastCtx != nil {
		t.lastCancel()
	}
	t.lastCtx, t.lastCancel = context.WithTimeout(context.Background(), rpcTimeout)
	return t.lastCtx
}
