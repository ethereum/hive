package main

import (
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/params"
)

func init() {
	register("tx-largereceipt", func() blockModifier {
		const logCost = 1607055
		const pushCost = 10
		return &modLargeReceipt{
			gasLimit: logCost + pushCost + params.TxGasContractCreation + params.TxDataNonZeroGasEIP2028*8,
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

	for range m.txCount {
		var txdata = &types.LegacyTx{
			Nonce:    ctx.AccountNonce(sender.addr),
			Gas:      m.gasLimit,
			GasPrice: ctx.TxGasFeeCap(),
			Data:     modLargeReceiptCode,
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
