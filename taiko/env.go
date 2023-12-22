package taiko

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"time"

	"github.com/ethereum/hive/hivesim"
	"github.com/stretchr/testify/require"
	"github.com/taikoxyz/taiko-client/bindings"
	"golang.org/x/sync/semaphore"
)

// default timeout for RPC calls
var RPCTimeout = 10 * time.Second

// LoggingRoundTrip writes requests and responses to the test log.
type LoggingRoundTrip struct {
	T     *hivesim.T
	Inner http.RoundTripper
}

func (rt *LoggingRoundTrip) RoundTrip(req *http.Request) (*http.Response, error) {
	// Read and log the request body.
	reqBytes, err := io.ReadAll(req.Body)
	req.Body.Close()
	if err != nil {
		return nil, err
	}
	rt.T.Logf(">>  %s", bytes.TrimSpace(reqBytes))
	reqCopy := *req
	reqCopy.Body = io.NopCloser(bytes.NewReader(reqBytes))

	// Do the round trip.
	resp, err := rt.Inner.RoundTrip(&reqCopy)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Read and log the response bytes.
	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	respCopy := *resp
	respCopy.Body = io.NopCloser(bytes.NewReader(respBytes))
	rt.T.Logf("<<  %s", bytes.TrimSpace(respBytes))
	return &respCopy, nil
}

// TestEnv is the environment of a single test.
type TestEnv struct {
	T         *hivesim.T
	Context   context.Context
	Conf      *Config
	Clients   *ClientsByRole
	L1Vault   *Vault
	L2Vault   *Vault
	Net       *Devnet
	TaikoConf *bindings.TaikoDataConfig
}

func NewTestEnv(ctx context.Context, t *hivesim.T) *TestEnv {
	e := &TestEnv{
		T:       t,
		Context: ctx,
	}
	clientTypes, err := t.Sim.ClientTypes()
	require.NoError(t, err, "failed to retrieve list of client types: %v", err)
	e.Clients = Roles(t, clientTypes)

	c, err := DefaultConfig()
	require.NoError(t, err)

	e.L1Vault = NewVault(t, c.L1.ChainID)
	e.L2Vault = NewVault(t, c.L2.ChainID)

	e.Conf = c
	return e
}

func (e *TestEnv) StartSingleNodeNet() {
	e.StartL1L2Driver(WithELNodeType("full"))
	l1, l2 := e.Net.GetL1ELNode(0), e.Net.GetL2ELNode(0)
	e.Net.Apply(
		WithProverNode(e.NewProverNode(l1, l2)),
		WithProposerNode(e.NewProposerNode(l1, l2)),
	)
}

func (e *TestEnv) StopSingleNodeNet() {
	t := e.T
	for _, n := range e.Net.provers {
		t.Sim.StopClient(t.SuiteID, t.TestID, n.Container)
	}
	for _, n := range e.Net.proposers {
		t.Sim.StopClient(t.SuiteID, t.TestID, n.Container)
	}
	for _, n := range e.Net.drivers {
		t.Sim.StopClient(t.SuiteID, t.TestID, n.Container)
	}
	for _, n := range e.Net.L1Engines {
		t.Sim.StopClient(t.SuiteID, t.TestID, n.Container)
	}
	for _, n := range e.Net.L2Engines {
		t.Sim.StopClient(t.SuiteID, t.TestID, n.Container)
	}
}

func (e *TestEnv) StartL1L2Driver(l2Opts ...NodeOption) {
	e.StartL1L2(l2Opts...)
	l1, l2 := e.Net.GetL1ELNode(0), e.Net.GetL2ELNode(0)
	e.Net.Apply(WithDriverNode(e.NewDriverNode(l1, l2)))
}

func (e *TestEnv) StartL1L2(l2Opts ...NodeOption) {
	t := e.T
	l2 := e.NewL2ELNode(l2Opts...)
	l1 := e.NewL1ELNode(l2)
	taikoL1, err := l1.TaikoL1Client()
	require.NoError(t, err)
	c, err := taikoL1.GetConfig(nil)
	require.NoError(t, err)
	e.TaikoConf = &c
	opts := []DevOption{
		WithL2Node(l2),
		WithL1Node(l1),
	}
	e.Net = NewDevnet(e.T, e.Conf, opts...)
}

func (e *TestEnv) GenSomeL1Blocks(t *hivesim.T, cnt uint64) {
	t, ctx := e.T, e.Context
	require.NoError(t, GenSomeBlocks(ctx, e.Net.GetL1ELNode(0), e.L1Vault, cnt))
	t.Logf("generate %d L2 blocks", cnt)
}

func (e *TestEnv) GenCommitDelayBlocks(t *hivesim.T) {
	t, ctx := e.T, e.Context
	cnt := e.TaikoConf.CommitConfirmations.Uint64()
	if cnt == 0 {
		return
	}
	n := e.Net.GetL1ELNode(0)
	require.NotNil(t, n)
	require.NoError(t, GenSomeBlocks(ctx, n, e.L1Vault, cnt))
	t.Logf("generate %d L2 blocks", cnt)
}

func (e *TestEnv) GenSomeL2Blocks(t *hivesim.T, cnt uint64) {
	t, ctx := e.T, e.Context
	n := e.Net.GetL2ELNode(0)
	require.NotNil(t, n)
	require.NoError(t, GenSomeBlocks(ctx, n, e.L2Vault, cnt))
	t.Logf("generate %d L2 blocks", cnt)
}

type RunTestsParams struct {
	Devnet      *Devnet
	Tests       []*hivesim.TestSpec
	Concurrency int64
}

func RunTests(t *hivesim.T, ctx context.Context, params *RunTestsParams) {
	s := semaphore.NewWeighted(params.Concurrency)
	var done int
	doneCh := make(chan struct{})

	for _, test := range params.Tests {
		go func(test *hivesim.TestSpec) {
			require.NoError(t, s.Acquire(ctx, 1))
			defer s.Release(1)
			t.Run(*test)
			doneCh <- struct{}{}
		}(test)
	}

	for done < len(params.Tests) {
		select {
		case <-doneCh:
			done++
		case <-ctx.Done():
			return
		}
	}
}
