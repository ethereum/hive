package main

import (
	"encoding/binary"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
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
}

// modInvokeEmit creates transactions that invoke the 'emit' contract.
type modInvokeEmit struct {
	txType   byte
	gasLimit uint64

	txs []invokeEmitTxInfo
}

type invokeEmitTxInfo struct {
	TxHash   common.Hash    `json:"txhash"`
	Sender   common.Address `json:"sender"`
	Block    hexutil.Uint64 `json:"block"`
	Index    int            `json:"indexInBlock"`
	DataHash common.Hash    `json:"datahash"`
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

	case types.LegacyTxType:
		txdata = &types.LegacyTx{
			Nonce:    ctx.AccountNonce(sender.addr),
			Gas:      m.gasLimit,
			GasPrice: ctx.TxGasFeeCap(),
			To:       &recipient,
			Data:     calldata,
			Value:    big.NewInt(2),
		}
	}

	txindex := ctx.TxCount()
	tx := ctx.AddNewTx(sender, txdata)
	m.txs = append(m.txs, invokeEmitTxInfo{
		Block:    hexutil.Uint64(ctx.NumberU64()),
		Sender:   sender.addr,
		TxHash:   tx.Hash(),
		Index:    txindex,
		DataHash: datahash,
	})
	return true
}

func (m *modInvokeEmit) txInfo() any {
	return m.txs
}

func (m *modInvokeEmit) genCallData(ctx *genBlockContext) []byte {
	d := make([]byte, 8)
	binary.BigEndian.PutUint64(d, ctx.TxRandomValue())
	return append(d, "emit"...)
}
