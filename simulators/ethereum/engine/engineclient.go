package main

import (
	"context"
	"fmt"
	"math/big"
	"net"
	"net/http"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/beacon"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/ethereum/hive/hivesim"
)

// Execution API v1.0.0-alpha.5 changes this to 8550
// https://github.com/ethereum/execution-apis/releases/tag/v1.0.0-alpha.5
var EnginePortHTTP = 8545
var EnginePortWS = 8546

// EngineClient wrapper for Ethereum Engine RPC for testing purposes.
type EngineClient struct {
	*hivesim.T
	*hivesim.Client
	c                       *rpc.Client
	Eth                     *ethclient.Client
	IP                      net.IP
	TerminalTotalDifficulty *big.Int

	// This holds most recent context created by the Ctx method.
	// Every time Ctx is called, it creates a new context with the default
	// timeout and cancels the previous one.
	lastCtx    context.Context
	lastCancel context.CancelFunc
}

type EngineClients []*EngineClient

// NewClient creates a engine client that uses the given RPC client.
func NewEngineClient(t *hivesim.T, hc *hivesim.Client, ttd *big.Int) *EngineClient {
	client := &http.Client{
		Transport: &loggingRoundTrip{
			t:     t,
			hc:    hc,
			inner: http.DefaultTransport,
		},
	}
	// Prepare HTTP Client
	rpcHttpClient, _ := rpc.DialHTTPWithClient(fmt.Sprintf("http://%v:%v/", hc.IP, EnginePortHTTP), client)

	// Prepare ETH Client
	client = &http.Client{
		Transport: &loggingRoundTrip{
			t:     t,
			hc:    hc,
			inner: http.DefaultTransport,
		},
	}
	rpcClient, _ := rpc.DialHTTPWithClient(fmt.Sprintf("http://%v:8545/", hc.IP), client)
	eth := ethclient.NewClient(rpcClient)
	return &EngineClient{
		T:                       t,
		Client:                  hc,
		c:                       rpcHttpClient,
		Eth:                     eth,
		IP:                      hc.IP,
		TerminalTotalDifficulty: ttd,
		lastCtx:                 nil,
		lastCancel:              nil,
	}
}

func (ec *EngineClient) Equals(ec2 *EngineClient) bool {
	return ec.Container == ec2.Container
}

func (ec *EngineClient) checkTTD() bool {
	var td *TotalDifficultyHeader
	if err := ec.c.CallContext(ec.Ctx(), &td, "eth_getBlockByNumber", "latest", false); err != nil {
		panic(err)
	}
	return td.TotalDifficulty.ToInt().Cmp(ec.TerminalTotalDifficulty) >= 0
}

// Wait until the TTD is reached by a single client.
func (ec *EngineClient) waitForTTD(done chan<- *EngineClient, cancel <-chan interface{}) {
	for {
		select {
		case <-time.After(tTDCheckPeriod):
			if ec.checkTTD() {
				select {
				case done <- ec:
				case <-cancel:
				}
				return
			}
		case <-cancel:
			return
		}
	}
}

// Wait until the TTD is reached by a single client with a timeout.
// Returns true if the TTD has been reached, false when timeout occurred.
func (ec *EngineClient) waitForTTDWithTimeout(timeout <-chan time.Time) bool {
	for {
		select {
		case <-time.After(tTDCheckPeriod):
			if ec.checkTTD() {
				return true
			}
		case <-timeout:
			return false
		}
	}
}

// Wait until the TTD is reached by any of the engine clients
func (ecs EngineClients) waitForTTD(timeout <-chan time.Time) *EngineClient {
	done := make(chan *EngineClient)
	cancel := make(chan interface{})
	defer func() {
		close(cancel)
	}()
	for _, ec := range ecs {
		go ec.waitForTTD(done, cancel)
	}
	select {
	case ec := <-done:
		return ec
	case <-timeout:
		return nil
	}
}

func (ec *EngineClient) Close() {
	ec.c.Close()
	ec.Eth.Close()
	if ec.lastCtx != nil {
		ec.lastCancel()
	}
}

func (ec *EngineClient) Ctx() context.Context {
	if ec.lastCtx != nil {
		ec.lastCancel()
	}
	ec.lastCtx, ec.lastCancel = context.WithTimeout(context.Background(), rpcTimeout)
	return ec.lastCtx
}

type PayloadStatusV1 struct {
	Status          string       `json:"status"`
	LatestValidHash *common.Hash `json:"latestValidHash"`
	ValidationError *string      `json:"validationError"`
}
type ForkChoiceResponse struct {
	PayloadStatus PayloadStatusV1   `json:"payloadStatus"`
	PayloadID     *beacon.PayloadID `json:"payloadId"`
}

// Engine API Call Methods
func (ec *EngineClient) EngineForkchoiceUpdatedV1(ctx context.Context, fcState *beacon.ForkchoiceStateV1, pAttributes *beacon.PayloadAttributesV1) (ForkChoiceResponse, error) {
	var result ForkChoiceResponse
	err := ec.c.CallContext(ctx, &result, "engine_forkchoiceUpdatedV1", fcState, pAttributes)
	return result, err
}

func (ec *EngineClient) EngineGetPayloadV1(ctx context.Context, payloadId *beacon.PayloadID) (beacon.ExecutableDataV1, error) {
	var result beacon.ExecutableDataV1
	err := ec.c.CallContext(ctx, &result, "engine_getPayloadV1", payloadId)
	return result, err
}

func (ec *EngineClient) EngineNewPayloadV1(ctx context.Context, payload *beacon.ExecutableDataV1) (PayloadStatusV1, error) {
	var result PayloadStatusV1
	err := ec.c.CallContext(ctx, &result, "engine_newPayloadV1", payload)
	return result, err
}
