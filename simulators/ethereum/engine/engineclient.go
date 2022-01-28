package main

import (
	"context"
	"fmt"
	"net"
	"net/http"

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
	c   *rpc.Client
	Eth *ethclient.Client
	IP  net.IP

	// This holds most recent context created by the Ctx method.
	// Every time Ctx is called, it creates a new context with the default
	// timeout and cancels the previous one.
	lastCtx    context.Context
	lastCancel context.CancelFunc
}

// NewClient creates a engine client that uses the given RPC client.
func NewEngineClient(t *hivesim.T, hc *hivesim.Client) *EngineClient {
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
	return &EngineClient{t, hc, rpcHttpClient, eth, hc.IP, nil, nil}
}

func (ec *EngineClient) Equals(ec2 *EngineClient) bool {
	return ec.Container == ec2.Container
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

// Engine API Call Methods
func (ec *EngineClient) EngineForkchoiceUpdatedV1(ctx context.Context, fcState *beacon.ForkchoiceStateV1, pAttributes *beacon.PayloadAttributesV1) (beacon.ForkChoiceResponse, error) {
	var result beacon.ForkChoiceResponse
	err := ec.c.CallContext(ctx, &result, "engine_forkchoiceUpdatedV1", fcState, pAttributes)
	return result, err
}

func (ec *EngineClient) EngineGetPayloadV1(ctx context.Context, payloadId *beacon.PayloadID) (beacon.ExecutableDataV1, error) {
	var result beacon.ExecutableDataV1
	err := ec.c.CallContext(ctx, &result, "engine_getPayloadV1", payloadId)
	return result, err
}

func (ec *EngineClient) EngineNewPayloadV1(ctx context.Context, payload *beacon.ExecutableDataV1) (beacon.PayloadStatusV1, error) {
	var result beacon.PayloadStatusV1
	err := ec.c.CallContext(ctx, &result, "engine_newPayloadV1", payload)
	return result, err
}
