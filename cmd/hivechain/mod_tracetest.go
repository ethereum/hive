package main

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
)

func init() {
	register("tx-calltree", func() blockModifier {
		return &modTraceTx{
			recipient: common.HexToAddress(calltreeAddr),
			gasLimit:  600000,
		}
	})
	register("tx-callrevert", func() blockModifier {
		return &modTraceTx{
			recipient: common.HexToAddress(calltreeCallrevertAddr),
			calldata:  []byte{0x01},
			gasLimit:  100000,
		}
	})
}

// modTraceTx creates transactions for trace RPC testing:
// tx-calltree produces a trace containing every call frame type,
// tx-callrevert a whole-transaction revert with a decodable reason.
type modTraceTx struct {
	recipient common.Address
	calldata  []byte
	gasLimit  uint64

	txs []traceTxInfo
}

type traceTxInfo struct {
	TxHash common.Hash    `json:"txhash"`
	Sender common.Address `json:"sender"`
	Block  hexutil.Uint64 `json:"block"`
	Index  int            `json:"indexInBlock"`
}

func (m *modTraceTx) apply(ctx *genBlockContext) bool {
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
	m.txs = append(m.txs, traceTxInfo{
		TxHash: tx.Hash(),
		Sender: sender.addr,
		Block:  hexutil.Uint64(ctx.NumberU64()),
		Index:  txindex,
	})
	return true
}

func (m *modTraceTx) txInfo() any {
	return m.txs
}
