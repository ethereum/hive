package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/eth/catalyst"
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
	http_c *rpc.Client
	ws_c   *rpc.Client
	c      *rpc.Client
	Eth    *ethclient.Client
	IP     net.IP

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
	// Prepare WS Client
	ctx, done := context.WithTimeout(context.Background(), 5*time.Second)
	rpcWsClient, err := rpc.DialWebsocket(ctx, fmt.Sprintf("ws://%v:%v/", hc.IP, EnginePortWS), "")
	done()
	if err != nil {
		t.Fatal("NewEngineClient: WebSocket connection failed:", err)
	}
	// Prepare ETH Client (Use HTTP)
	client = &http.Client{
		Transport: &loggingRoundTrip{
			t:     t,
			hc:    hc,
			inner: http.DefaultTransport,
		},
	}
	rpcClient, _ := rpc.DialHTTPWithClient(fmt.Sprintf("http://%v:8545/", hc.IP), client)
	eth := ethclient.NewClient(rpcClient)
	// By default we send requests via HTTP
	return &EngineClient{t, hc, rpcHttpClient, rpcWsClient, rpcHttpClient, eth, hc.IP, nil, nil}
}

// This is not necessarily true in all cases, but for this simulator it's ok.
func (ec *EngineClient) Equals(ec2 *EngineClient) bool {
	return ec.IP.Equal(ec2.IP)
}

func (ec *EngineClient) Close() {
	ec.http_c.Close()
	ec.ws_c.Close()
	ec.Eth.Close()
	if ec.lastCtx != nil {
		ec.lastCancel()
	}
}

func (ec *EngineClient) SetHTTP() {
	ec.c = ec.http_c
}

func (ec *EngineClient) SetWS() {
	ec.c = ec.ws_c
}

func (ec *EngineClient) SwitchProtocol() {
	if ec.c == ec.http_c {
		ec.c = ec.ws_c
	} else {
		ec.c = ec.http_c
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
func (ec *EngineClient) EngineForkchoiceUpdatedV1(ctx context.Context, fcState *catalyst.ForkchoiceStateV1, pAttributes *catalyst.PayloadAttributesV1) (catalyst.ForkChoiceResponse, error) {
	var result catalyst.ForkChoiceResponse
	err := ec.http_c.CallContext(ctx, &result, "engine_forkchoiceUpdatedV1", fcState, pAttributes)
	return result, err
}

func (ec *EngineClient) EngineGetPayloadV1(ctx context.Context, payloadId *hexutil.Bytes) (catalyst.ExecutableDataV1, error) {
	var result catalyst.ExecutableDataV1
	err := ec.http_c.CallContext(ctx, &result, "engine_getPayloadV1", payloadId)
	return result, err
}

func (ec *EngineClient) EngineExecutePayloadV1(ctx context.Context, payload *catalyst.ExecutableDataV1) (catalyst.ExecutePayloadResponse, error) {
	var result catalyst.ExecutePayloadResponse
	err := ec.http_c.CallContext(ctx, &result, "engine_executePayloadV1", payload)
	return result, err
}
