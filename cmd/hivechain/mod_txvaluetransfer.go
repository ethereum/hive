package main

import (
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/params"
)

func init() {
	register("tx-transfer-legacy", func() blockModifier {
		return &modValueTransfer{
			txType:   types.LegacyTxType,
			gasLimit: params.TxGas,
		}
	})
	register("tx-transfer-eip2930", func() blockModifier {
		return &modValueTransfer{
			txType:   types.AccessListTxType,
			gasLimit: params.TxGas,
		}
	})
	register("tx-transfer-eip1559", func() blockModifier {
		return &modValueTransfer{
			txType:   types.DynamicFeeTxType,
			gasLimit: params.TxGas,
		}
	})

}

type modValueTransfer struct {
	txType   byte
	gasLimit uint64

	txs []valueTransferInfo
}

type valueTransferInfo struct {
	TxHash common.Hash    `json:"txhash"`
	Sender common.Address `json:"sender"`
	Block  hexutil.Uint64 `json:"block"`
	Index  int            `json:"indexInBlock"`
}

func (m *modValueTransfer) apply(ctx *genBlockContext) bool {
	if !ctx.HasGas(m.gasLimit) {
		return false
	}

	sender := ctx.TxSenderAccount()
	recipient := pickRecipient(ctx)

	var txdata types.TxData
	switch m.txType {
	case types.LegacyTxType:
		txdata = &types.LegacyTx{
			Nonce:    ctx.AccountNonce(sender.addr),
			Gas:      m.gasLimit,
			GasPrice: ctx.TxGasFeeCap(),
			To:       &recipient,
			Value:    big.NewInt(1),
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
			Value:    big.NewInt(1),
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
			Value:     big.NewInt(1),
		}

	default:
		panic(fmt.Errorf("unhandled tx type %d", m.txType))
	}

	txindex := ctx.TxCount()
	tx := ctx.AddNewTx(sender, txdata)
	m.txs = append(m.txs, valueTransferInfo{
		Block:  hexutil.Uint64(ctx.NumberU64()),
		Sender: sender.addr,
		TxHash: tx.Hash(),
		Index:  txindex,
	})
	return true
}

func (m *modValueTransfer) txInfo() any {
	return m.txs
}

func pickRecipient(ctx *genBlockContext) common.Address {
	i := ctx.TxRandomValue() % uint64(len(ctx.gen.accounts))
	return ctx.gen.accounts[i].addr
}
