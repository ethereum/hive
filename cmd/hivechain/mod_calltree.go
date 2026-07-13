package main

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
)

func init() {
	register("tx-calltree", func() blockModifier {
		return &modCallTree{
			recipient: common.HexToAddress(calltreeAddr),
			gasLimit:  600000,
		}
	})
	register("tx-callrevert", func() blockModifier {
		return &modCallTree{
			recipient: common.HexToAddress(calltreeCallrevertAddr),
			calldata:  []byte{0x01},
			gasLimit:  100000,
		}
	})
}

// modCallTree creates transactions invoking the calltree contract (a trace
// with every call frame type) or the callrevert contract (a whole-tx revert).
type modCallTree struct {
	recipient common.Address
	calldata  []byte
	gasLimit  uint64

	txs []callTreeTxInfo
}

type callTreeTxInfo struct {
	TxHash common.Hash    `json:"txhash"`
	Sender common.Address `json:"sender"`
	Block  hexutil.Uint64 `json:"block"`
	Index  int            `json:"indexInBlock"`
}

func (m *modCallTree) apply(ctx *genBlockContext) bool {
	if !ctx.ChainConfig().IsLondon(ctx.Number()) {
		return false
	}
	if !ctx.HasGas(m.gasLimit) {
		return false
	}

	sender := ctx.TxSenderAccount()
	txdata := &types.DynamicFeeTx{
		Nonce:     ctx.AccountNonce(sender.addr),
		Gas:       m.gasLimit,
		GasFeeCap: ctx.TxGasFeeCap(),
		GasTipCap: big.NewInt(1),
		To:        &m.recipient,
		Data:      m.calldata,
	}

	txindex := ctx.TxCount()
	tx := ctx.AddNewTx(sender, txdata)
	m.txs = append(m.txs, callTreeTxInfo{
		TxHash: tx.Hash(),
		Sender: sender.addr,
		Block:  hexutil.Uint64(ctx.NumberU64()),
		Index:  txindex,
	})
	return true
}

func (m *modCallTree) txInfo() any {
	return m.txs
}
