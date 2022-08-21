package optimism

import (
	"bytes"
	"context"
	"github.com/ethereum/hive/hivesim"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/semaphore"
	"io"
	"net/http"
	"time"
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

type TestSpec struct {
	Name        string
	Description string
	Run         func(t *hivesim.T, env *TestEnv)
}

type TestEnv struct {
	Context context.Context
	Devnet  *Devnet

	// This holds most recent context created by the Ctx method.
	// Every time Ctx is called, it creates a new context with the default
	// timeout and cancels the previous one.
	lastCtx    context.Context
	lastCancel context.CancelFunc
}

// Ctx returns a context with the default timeout.
// For subsequent calls to Ctx, it also cancels the previous context.
func (t *TestEnv) Ctx() context.Context {
	return t.TimeoutCtx(RPCTimeout)
}

func (t *TestEnv) TimeoutCtx(timeout time.Duration) context.Context {
	if t.lastCtx != nil {
		t.lastCancel()
	}
	t.lastCtx, t.lastCancel = context.WithTimeout(t.Context, timeout)
	return t.lastCtx
}

type RunTestsParams struct {
	Devnet      *Devnet
	Tests       []*TestSpec
	Concurrency int64
}

func RunTests(ctx context.Context, t *hivesim.T, params *RunTestsParams) {
	s := semaphore.NewWeighted(params.Concurrency)
	var done int
	doneCh := make(chan struct{})

	for _, test := range params.Tests {
		go func(test *TestSpec) {
			require.NoError(t, s.Acquire(ctx, 1))
			defer s.Release(1)
			env := &TestEnv{
				Context: ctx,
				Devnet:  params.Devnet,
			}

			require.NoError(t, s.Acquire(ctx, 1))
			t.Run(hivesim.TestSpec{
				Name:        test.Name,
				Description: test.Description,
				Run: func(t *hivesim.T) {
					test.Run(t, env)
					if env.lastCtx != nil {
						env.lastCancel()
					}
				},
			})
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
