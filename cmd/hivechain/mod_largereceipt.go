package main

import (
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/params"
)

func init() {
	register("tx-largereceipt", func() blockModifier {
		// Gas parameters here are calculated to create a block with > 10MB of receipts.
		const logCost = 1607055
		const pushCost = 10
		return &modLargeReceipt{
			gasLimit: logCost + pushCost + params.TxGasContractCreation + params.TxDataNonZeroGasEIP2028*8,
			txCount:  56,
		}
	})
}

// modLargeReceipt creates blocks with large receipts. It emits multiple transactions
// within a single block, where each transaction has a single log with large data.
type modLargeReceipt struct {
	didRun   bool
	gasLimit uint64 // gas of single transaction
	txCount  uint64 // number of transactions added to block
	block    uint64 // block number where txs were included
}

func (m *modLargeReceipt) apply(ctx *genBlockContext) bool {
	if m.didRun || !ctx.HasGas(m.gasLimit*m.txCount) {
		return false
	}

	sender := ctx.TxSenderAccount()
	for range m.txCount {
		ctx.AddNewTx(sender, &types.LegacyTx{
			Nonce:    ctx.AccountNonce(sender.addr),
			Gas:      m.gasLimit,
			GasPrice: ctx.TxGasFeeCap(),
			Data:     modLargeReceiptCode,
		})
	}
	m.block = ctx.NumberU64()
	m.didRun = true
	return true
}

// txInfo is just the block number that has the receipts.
func (m *modLargeReceipt) txInfo() any {
	if m.didRun {
		return m.block
	}
	return nil
}
