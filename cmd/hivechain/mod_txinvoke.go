package main

import (
	"encoding/binary"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/crypto/kzg4844"
	"github.com/ethereum/go-ethereum/params"
	"github.com/holiman/uint256"
)

func init() {
	register("tx-emit-legacy", func() blockModifier {
		return &modInvokeEmit{
			txType:   types.LegacyTxType,
			gasLimit: 100000,
		}
	})
	register("tx-emit-eip2930", func() blockModifier {
		return &modInvokeEmit{
			txType:   types.AccessListTxType,
			gasLimit: 100000,
		}
	})
	register("tx-emit-eip1559", func() blockModifier {
		return &modInvokeEmit{
			txType:   types.DynamicFeeTxType,
			gasLimit: 100000,
		}
	})
	register("tx-emit-eip4844", func() blockModifier {
		return &modInvokeEmit{
			txType:   types.BlobTxType,
			gasLimit: 100000,
		}
	})
}

// modInvokeEmit creates transactions that invoke the 'emit' contract.
type modInvokeEmit struct {
	txType   byte
	gasLimit uint64

	txs []invokeEmitTxInfo
}

type invokeEmitTxInfo struct {
	TxHash    common.Hash    `json:"txhash"`
	Sender    common.Address `json:"sender"`
	Block     hexutil.Uint64 `json:"block"`
	Index     int            `json:"indexInBlock"`
	LogTopic0 common.Hash    `json:"logtopic0"`
	LogTopic1 common.Hash    `json:"logtopic1"`
}

func (m *modInvokeEmit) apply(ctx *genBlockContext) bool {
	if !ctx.HasGas(m.gasLimit) {
		return false
	}

	sender := ctx.TxSenderAccount()
	recipient := common.HexToAddress(emitAddr)
	calldata := m.genCallData(ctx)
	datahash := crypto.Keccak256Hash(calldata)

	var txdata types.TxData
	switch m.txType {
	case types.LegacyTxType:
		txdata = &types.LegacyTx{
			Nonce:    ctx.AccountNonce(sender.addr),
			Gas:      m.gasLimit,
			GasPrice: ctx.TxGasFeeCap(),
			To:       &recipient,
			Data:     calldata,
			Value:    big.NewInt(2),
		}

	case types.AccessListTxType:
		if !ctx.ChainConfig().IsBerlin(ctx.Number()) {
			return false
		}
		txdata = &types.AccessListTx{
			Nonce:    ctx.AccountNonce(sender.addr),
			Gas:      m.gasLimit,
			GasPrice: ctx.TxGasFeeCap(),
			To:       &recipient,
			Value:    big.NewInt(2),
			Data:     calldata,
			AccessList: types.AccessList{
				{
					Address:     recipient,
					StorageKeys: []common.Hash{{}, datahash},
				},
			},
		}

	case types.DynamicFeeTxType:
		if !ctx.ChainConfig().IsLondon(ctx.Number()) {
			return false
		}
		txdata = &types.DynamicFeeTx{
			Nonce:     ctx.AccountNonce(sender.addr),
			Gas:       m.gasLimit,
			GasFeeCap: ctx.TxGasFeeCap(),
			GasTipCap: big.NewInt(1),
			To:        &recipient,
			Value:     big.NewInt(2),
			Data:      calldata,
			AccessList: types.AccessList{
				{
					Address:     recipient,
					StorageKeys: []common.Hash{{}, datahash},
				},
			},
		}

	case types.BlobTxType:
		if !ctx.ChainConfig().IsCancun(ctx.Number(), ctx.Timestamp()) {
			return false
		}
		var (
			blob1     = kzg4844.Blob{0x01}
			blob1C, _ = kzg4844.BlobToCommitment(&blob1)
			blob1P, _ = kzg4844.ComputeBlobProof(&blob1, blob1C)
		)
		sidecar := &types.BlobTxSidecar{
			Blobs:       []kzg4844.Blob{blob1},
			Commitments: []kzg4844.Commitment{blob1C},
			Proofs:      []kzg4844.Proof{blob1P},
		}
		txdata = &types.BlobTx{
			Nonce:     ctx.AccountNonce(sender.addr),
			GasTipCap: uint256.NewInt(1),
			GasFeeCap: uint256.MustFromBig(ctx.TxGasFeeCap()),
			Gas:       m.gasLimit,
			To:        recipient,
			Value:     uint256.NewInt(3),
			Data:      calldata,
			AccessList: types.AccessList{
				{
					Address:     recipient,
					StorageKeys: []common.Hash{{}, datahash},
				},
			},
			BlobFeeCap: uint256.NewInt(params.BlobTxBlobGasPerBlob),
			BlobHashes: sidecar.BlobHashes(),
			Sidecar:    sidecar,
		}

	default:
		panic(fmt.Errorf("unhandled tx type %d", m.txType))
	}

	txindex := ctx.TxCount()
	tx := ctx.AddNewTx(sender, txdata)
	m.txs = append(m.txs, invokeEmitTxInfo{
		Block:     hexutil.Uint64(ctx.NumberU64()),
		Sender:    sender.addr,
		TxHash:    tx.Hash(),
		Index:     txindex,
		LogTopic0: common.HexToHash("0x00000000000000000000000000000000000000000000000000000000656d6974"),
		LogTopic1: datahash,
	})
	return true
}

func (m *modInvokeEmit) txInfo() any {
	return m.txs
}

// genCallData produces the calldata for the 'emit' contract.
func (m *modInvokeEmit) genCallData(ctx *genBlockContext) []byte {
	d := make([]byte, 8)
	binary.BigEndian.PutUint64(d, ctx.TxRandomValue())
	return append(d, "emit"...)
}
