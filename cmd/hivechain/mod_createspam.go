package main

import (
	"github.com/ethereum/go-ethereum/core/types"
)

// Here we create transactions that create spam contracts. These exist simply to fill up
// the state. We need a decent amount of state in the sync tests, for example.

func init() {
	register("randomlogs", func() blockModifier {
		return &modCreateSpam{
			code: genlogsCode,
			gas:  20000,
		}
	})
	register("randomcode", func() blockModifier {
		return &modCreateSpam{
			code: gencodeCode,
			gas:  60000,
		}
	})
	register("randomstorage", func() blockModifier {
		return &modCreateSpam{
			code: genstorageCode,
			gas:  80000,
		}
	})
}

type modCreateSpam struct {
	code []byte
	gas  uint64
}

func (m *modCreateSpam) apply(ctx *genBlockContext) bool {
	gas := ctx.TxCreateIntrinsicGas(m.code) + m.gas
	if !ctx.HasGas(gas) {
		return false
	}

	sender := ctx.TxSenderAccount()
	txdata := &types.LegacyTx{
		Nonce:    ctx.AccountNonce(sender.addr),
		Gas:      gas,
		GasPrice: ctx.TxGasFeeCap(),
		Data:     m.code,
	}
	ctx.AddNewTx(sender, txdata)
	return true
}

func (m *modCreateSpam) txInfo() any {
	return nil
}
