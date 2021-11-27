package main

import (
	"context"
	"fmt"
	"math/big"
	"net/http"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/eth/catalyst"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/ethereum/hive/hivesim"
)

// Catalyst defines typed wrappers for the Ethereum RPC API.
type Catalyst struct {
	*hivesim.T
	c                         *rpc.Client
	Eth                       *ethclient.Client
	PoSBlockProductionEnabled bool

	// This holds most recent context created by the Ctx method.
	// Every time Ctx is called, it creates a new context with the default
	// timeout and cancels the previous one.
	lastCtx    context.Context
	lastCancel context.CancelFunc
}

// NewClient creates a client that uses the given RPC client.
func NewCatalyst(t *hivesim.T, hc *hivesim.Client) *Catalyst {
	client := &http.Client{
		Transport: &loggingRoundTrip{
			t:     t,
			inner: http.DefaultTransport,
		},
	}

	rpcClient, _ := rpc.DialHTTPWithClient(fmt.Sprintf("http://%v:8545/", hc.IP), client)

	eth := ethclient.NewClient(rpcClient)
	return &Catalyst{t, rpcClient, eth, true, nil, nil}
}

func (ec *Catalyst) Close() {
	ec.c.Close()
}

func (ec *Catalyst) Ctx() context.Context {
	if ec.lastCtx != nil {
		ec.lastCancel()
	}
	ec.lastCtx, ec.lastCancel = context.WithTimeout(context.Background(), rpcTimeout)
	return ec.lastCtx
}

// Engine Block Production Methods

// PoS related functions
func (ec *Catalyst) isPoS() bool {
	return true
}

func (ec *Catalyst) isPoSBlockProductionEnabled() bool {
	return ec.PoSBlockProductionEnabled
}

func (ec *Catalyst) enablePoSBlockProduction() {
	ec.PoSBlockProductionEnabled = true
	time.AfterFunc(blockProductionPoS, ec.minePOSBlock)
}

func (ec *Catalyst) disablePoSBlockProduction() {
	ec.PoSBlockProductionEnabled = false
}

// Mine a PoS block by using the catalyst engine api
func (ec *Catalyst) minePOSBlock() {
	lastBlockN, err := ec.Eth.BlockNumber(ec.Ctx())
	if err != nil {
		ec.Fatalf("Could not get block number: %v", err)
	}

	lastBlock, err := ec.Eth.BlockByNumber(ec.Ctx(), big.NewInt(int64(lastBlockN)))
	if err != nil {
		ec.Fatalf("Could not get block: %v", err)
	}

	lastBlockHash := lastBlock.Hash()

	fmt.Printf("lastBlock.Hash(): %v\n", lastBlockHash)

	forkchoiceState := catalyst.ForkchoiceStateV1{
		HeadBlockHash:      lastBlockHash,
		SafeBlockHash:      lastBlockHash,
		FinalizedBlockHash: lastBlockHash,
	}

	payloadAttributes := catalyst.PayloadAttributesV1{
		Timestamp:    lastBlock.Time() + 1,
		Random:       common.Hash{},
		FeeRecipient: [20]byte{},
	}

	resp, err := ec.EngineForkchoiceUpdatedV1(ec.Ctx(), &forkchoiceState, &payloadAttributes)
	if err != nil {
		ec.Fatalf("Could not send forkchoiceUpdatedV1: %v", err)
	}
	fmt.Printf("Forkchoice Status: %v\n", resp.Status)
	fmt.Printf("resp.PayloadID: %v\n", resp.PayloadID)

	payload, err := ec.EngineGetPayloadV1(ec.Ctx(), resp.PayloadID)
	if err != nil {
		ec.Fatalf("Could not getPayload (%v): %v", resp.PayloadID, err)
	}

	fmt.Printf("payload.BlockHash: %v\n", payload.BlockHash)
	fmt.Printf("payload.StateRoot: %v\n", payload.StateRoot)
	for i := 0; i < len(payload.Transactions); i++ {
		fmt.Printf("payload.Transactions[%v]: %v\n", i, hexutil.Bytes(payload.Transactions[i]).String())
	}

	execPayloadResp, err := ec.EngineExecutePayloadV1(ec.Ctx(), &payload)

	if err != nil {
		ec.Fatalf("Could not executePayload (%v): %v", resp.PayloadID, err)
	}
	fmt.Printf("execPayloadResp.Status: %v\n", execPayloadResp.Status)

	forkchoiceState = catalyst.ForkchoiceStateV1{
		HeadBlockHash:      payload.BlockHash,
		SafeBlockHash:      payload.BlockHash,
		FinalizedBlockHash: payload.BlockHash,
	}

	resp, err = ec.EngineForkchoiceUpdatedV1(ec.Ctx(), &forkchoiceState, nil)
	if err != nil {
		ec.Fatalf("Could not send forkchoiceUpdatedV1: %v", err)
	}

	fmt.Printf("resp.Status: %v\n", resp.Status)

	if ec.PoSBlockProductionEnabled {
		time.AfterFunc(blockProductionPoS, ec.minePOSBlock)
	}
}

// Engine API Call Methods

func (ec *Catalyst) EngineForkchoiceUpdatedV1(ctx context.Context, fcState *catalyst.ForkchoiceStateV1, pAttributes *catalyst.PayloadAttributesV1) (catalyst.ForkChoiceResponse, error) {
	var result catalyst.ForkChoiceResponse
	err := ec.c.CallContext(ctx, &result, "engine_forkchoiceUpdatedV1", fcState, pAttributes)
	return result, err
}

func (ec *Catalyst) EngineGetPayloadV1(ctx context.Context, payloadId *hexutil.Bytes) (catalyst.ExecutableDataV1, error) {
	var result catalyst.ExecutableDataV1
	err := ec.c.CallContext(ctx, &result, "engine_getPayloadV1", payloadId)
	return result, err
}

func (ec *Catalyst) EngineExecutePayloadV1(ctx context.Context, payload *catalyst.ExecutableDataV1) (catalyst.ExecutePayloadResponse, error) {
	var result catalyst.ExecutePayloadResponse
	err := ec.c.CallContext(ctx, &result, "engine_executePayloadV1", payload)
	return result, err
}

func toBlockNumArg(number *big.Int) string {
	if number == nil {
		return "latest"
	}
	pending := big.NewInt(-1)
	if number.Cmp(pending) == 0 {
		return "pending"
	}
	return hexutil.EncodeBig(number)
}

func toCallArg(msg ethereum.CallMsg) interface{} {
	arg := map[string]interface{}{
		"from": msg.From,
		"to":   msg.To,
	}
	if len(msg.Data) > 0 {
		arg["data"] = hexutil.Bytes(msg.Data)
	}
	if msg.Value != nil {
		arg["value"] = (*hexutil.Big)(msg.Value)
	}
	if msg.Gas != 0 {
		arg["gas"] = hexutil.Uint64(msg.Gas)
	}
	if msg.GasPrice != nil {
		arg["gasPrice"] = (*hexutil.Big)(msg.GasPrice)
	}
	return arg
}
