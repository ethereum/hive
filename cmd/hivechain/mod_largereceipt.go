package main

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/params"
)

func init() {
	register("tx-largereceipt", func() blockModifier {
		return &modLargeReceipt{
			gasLimit: 1608055 + params.TxGas,
			txCount:  56,
		}
	})
}

type modLargeReceipt struct {
	run      bool
	gasLimit uint64
	txCount  uint64
	block    uint64
}

func (m *modLargeReceipt) apply(ctx *genBlockContext) bool {
	if m.run {
		return false
	}
	if !ctx.HasGas(m.gasLimit * m.txCount) {
		return false
	}

	sender := ctx.TxSenderAccount()
	contract := common.HexToAddress(logAddr)

	for range m.txCount {
		var txdata = &types.LegacyTx{
			Nonce:    ctx.AccountNonce(sender.addr),
			Gas:      m.gasLimit,
			GasPrice: ctx.TxGasFeeCap(),
			To:       &contract,
		}

		ctx.AddNewTx(sender, txdata)
	}
	m.run = true
	m.block = ctx.NumberU64()

	return true
}

func (m *modLargeReceipt) txInfo() any {
	if m.run {
		return m.block
	}
	return nil
}
