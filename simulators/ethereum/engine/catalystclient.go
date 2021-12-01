package main

import (
	"context"
	"fmt"
	"net/http"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/eth/catalyst"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/ethereum/hive/hivesim"
)

// CatalystClient wrapper for Ethereum Engine RPC.
type CatalystClient struct {
	*hivesim.T
	c   *rpc.Client
	Eth *ethclient.Client

	// This holds most recent context created by the Ctx method.
	// Every time Ctx is called, it creates a new context with the default
	// timeout and cancels the previous one.
	lastCtx    context.Context
	lastCancel context.CancelFunc
}

// NewClient creates a engine client that uses the given RPC client.
func NewCatalystClient(t *hivesim.T, hc *hivesim.Client) *CatalystClient {
	client := &http.Client{
		Transport: &loggingRoundTrip{
			t:     t,
			inner: http.DefaultTransport,
		},
	}

	rpcClient, _ := rpc.DialHTTPWithClient(fmt.Sprintf("http://%v:8545/", hc.IP), client)

	eth := ethclient.NewClient(rpcClient)
	return &CatalystClient{t, rpcClient, eth, nil, nil}
}

func (ec *CatalystClient) Close() {
	ec.c.Close()
}

func (ec *CatalystClient) Ctx() context.Context {
	if ec.lastCtx != nil {
		ec.lastCancel()
	}
	ec.lastCtx, ec.lastCancel = context.WithTimeout(context.Background(), rpcTimeout)
	return ec.lastCtx
}

// Engine API Call Methods
func (ec *CatalystClient) EngineForkchoiceUpdatedV1(ctx context.Context, fcState *catalyst.ForkchoiceStateV1, pAttributes *catalyst.PayloadAttributesV1) (catalyst.ForkChoiceResponse, error) {
	var result catalyst.ForkChoiceResponse
	err := ec.c.CallContext(ctx, &result, "engine_forkchoiceUpdatedV1", fcState, pAttributes)
	return result, err
}

func (ec *CatalystClient) EngineGetPayloadV1(ctx context.Context, payloadId *hexutil.Bytes) (catalyst.ExecutableDataV1, error) {
	var result catalyst.ExecutableDataV1
	err := ec.c.CallContext(ctx, &result, "engine_getPayloadV1", payloadId)
	return result, err
}

func (ec *CatalystClient) EngineExecutePayloadV1(ctx context.Context, payload *catalyst.ExecutableDataV1) (catalyst.ExecutePayloadResponse, error) {
	var result catalyst.ExecutePayloadResponse
	err := ec.c.CallContext(ctx, &result, "engine_executePayloadV1", payload)
	return result, err
}
