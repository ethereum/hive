package main

import (
	"github.com/ethereum/go-ethereum/core/types"
)

func init() {
	register("randomlogs", func() blockModifier {
		return &modCreateTx{
			code: genlogsCode,
			gas:  20000,
		}
	})
	register("randomcode", func() blockModifier {
		return &modCreateTx{
			code: gencodeCode,
			gas:  30000,
		}
	})
	register("randomstorage", func() blockModifier {
		return &modCreateTx{
			code: genstorageCode,
			gas:  80000,
		}
	})
}

type modCreateTx struct {
	code []byte
	gas  uint64
}

func (m *modCreateTx) apply(ctx *genBlockContext) bool {
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

func (m *modCreateTx) txInfo() any {
	return nil
}
