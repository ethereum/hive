package main

import (
	"fmt"

	"github.com/ethereum/go-ethereum/beacon/engine"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

// writeEngineNewPayload writes engine API newPayload requests for the chain.
// Note this only works for post-merge blocks.
func (g *generator) writeEngineNewPayload() error {
	list := make([]*rpcRequest, 0)
	start, ok := g.mergeBlock()
	if ok {
		last := g.blockchain.CurrentBlock().Number.Uint64()
		for num := start; num <= last; num++ {
			b := g.blockchain.GetBlockByNumber(num)
			list = append(list, g.block2newpayload(b))
		}
	}
	return g.writeJSON("newpayload.json", list)
}

// writeEngineFcU writes engine API forkchoiceUpdated requests for the chain.
// Note this only works for post-merge blocks.
func (g *generator) writeEngineFcU() error {
	list := make([]*rpcRequest, 0)
	start, ok := g.mergeBlock()
	if ok {
		last := g.blockchain.CurrentBlock().Number.Uint64()
		for num := start; num <= last; num++ {
			b := g.blockchain.GetBlockByNumber(num)
			list = append(list, g.block2fcu(b))
		}
	}
	return g.writeJSON("fcu.json", list)
}

// writeEngineHeadNewPayload writes an engine API newPayload request for the head block.
func (g *generator) writeEngineHeadNewPayload() error {
	h := g.blockchain.CurrentBlock()
	b := g.blockchain.GetBlock(h.Hash(), h.Number.Uint64())
	np := g.block2newpayload(b)
	return g.writeJSON("headnewpayload.json", np)
}

// writeEngineHeadFcU writes an engine API forkchoiceUpdated request for the head block.
func (g *generator) writeEngineHeadFcU() error {
	h := g.blockchain.CurrentBlock()
	b := g.blockchain.GetBlock(h.Hash(), h.Number.Uint64())
	fcu := g.block2fcu(b)
	return g.writeJSON("headfcu.json", fcu)
}

func (g *generator) block2newpayload(b *types.Block) *rpcRequest {
	ed := engine.ExecutableData{
		ParentHash:    b.ParentHash(),
		FeeRecipient:  b.Coinbase(),
		StateRoot:     b.Root(),
		ReceiptsRoot:  b.ReceiptHash(),
		LogsBloom:     b.Bloom().Bytes(),
		Random:        b.MixDigest(),
		Number:        b.NumberU64(),
		GasLimit:      b.GasLimit(),
		GasUsed:       b.GasUsed(),
		Timestamp:     b.Time(),
		ExtraData:     b.Extra(),
		BaseFeePerGas: b.BaseFee(),
		BlockHash:     b.Hash(),
		Transactions:  [][]byte{},
		Withdrawals:   b.Withdrawals(),
		BlobGasUsed:   b.BlobGasUsed(),
		ExcessBlobGas: b.ExcessBlobGas(),
	}
	var blobHashes = make([]common.Hash, 0)
	for _, tx := range b.Transactions() {
		// Fill in transactions list.
		bin, err := tx.MarshalBinary()
		if err != nil {
			panic(err)
		}
		ed.Transactions = append(ed.Transactions, bin)
		// Collect blob hashes for post-Cancun blocks.
		blobHashes = append(blobHashes, tx.BlobHashes()...)
	}

	var method string
	var params = []any{ed}
	cfg := g.genesis.Config
	switch {
	case cfg.IsPrague(b.Number(), b.Time()):
		method = "engine_newPayloadV4"
		requests, ok := g.clRequests[b.NumberU64()]
		if !ok {
			panic(fmt.Sprintf("missing execution requests for block %d", b.NumberU64()))
		}
		params = append(params, blobHashes, b.BeaconRoot(), requests)
	case cfg.IsCancun(b.Number(), b.Time()):
		method = "engine_newPayloadV3"
		params = append(params, blobHashes, b.BeaconRoot())
	case cfg.IsShanghai(b.Number(), b.Time()):
		method = "engine_newPayloadV2"
	default:
		method = "engine_newPayloadV1"
	}
	id := fmt.Sprintf("np%d", b.NumberU64())
	return &rpcRequest{JsonRPC: "2.0", ID: id, Method: method, Params: params}
}

func (g *generator) block2fcu(b *types.Block) *rpcRequest {
	finalized := g.blockchain.CurrentFinalBlock()
	fc := engine.ForkchoiceStateV1{
		HeadBlockHash:      b.Hash(),
		SafeBlockHash:      b.Hash(),
		FinalizedBlockHash: b.Hash(),
	}
	// Set finalized to the block at the configured distance,
	// but only when notifying about a block that is above finalized.
	if b.NumberU64() > finalized.Number.Uint64() {
		fc.FinalizedBlockHash = finalized.Hash()
	}

	var method string
	cfg := g.genesis.Config
	switch {
	case cfg.IsCancun(b.Number(), b.Time()):
		method = "engine_forkchoiceUpdatedV3"
	case cfg.IsShanghai(b.Number(), b.Time()):
		method = "engine_forkchoiceUpdatedV2"
	default:
		method = "engine_forkchoiceUpdatedV1"
	}
	id := fmt.Sprintf("fcu%d", b.NumberU64())
	return &rpcRequest{JsonRPC: "2.0", ID: id, Method: method, Params: []any{&fc, nil}}
}

type rpcRequest struct {
	JsonRPC string `json:"jsonrpc"`
	ID      string `json:"id"`
	Method  string `json:"method"`
	Params  []any  `json:"params"`
}
