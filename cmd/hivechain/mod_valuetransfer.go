package main

import (
	"crypto/sha256"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/params"
)

func init() {
	register("valuetransfer", func() blockModifier {
		return &modValueTransfer{}
	})
}

type modValueTransfer struct {
	counter int
	txs     []valueTransferInfo
}

type valueTransferInfo struct {
	Block  hexutil.Uint64     `json:"block"`
	Sender common.Address     `json:"sender"`
	Tx     *types.Transaction `json:"tx"`
}

func (m *modValueTransfer) apply(ctx *genBlockContext) bool {
	if !ctx.HasGas(params.TxGas) {
		return false
	}

	sender := ctx.TxSenderAccount()
	var txdata types.TxData
	for txdata == nil {
		switch m.counter % 2 {
		case 0:
			// legacy tx
			r := randomRecipient(ctx.Number())
			txdata = &types.LegacyTx{
				Nonce:    ctx.AccountNonce(sender.addr),
				Gas:      params.TxGas,
				GasPrice: ctx.TxGasFeeCap(),
				To:       &r,
				Value:    big.NewInt(1),
			}
		case 1:
			// EIP1559 tx
			if !ctx.ChainConfig().IsLondon(ctx.Number()) {
				m.counter++
				continue
			}
			r := randomRecipient(ctx.Number())
			txdata = &types.DynamicFeeTx{
				Nonce:     ctx.AccountNonce(sender.addr),
				Gas:       params.TxGas,
				GasFeeCap: ctx.TxGasFeeCap(),
				GasTipCap: big.NewInt(1),
				To:        &r,
				Value:     big.NewInt(1),
			}
		}
	}

	tx := ctx.AddNewTx(sender, txdata)
	m.counter++
	m.txs = append(m.txs, valueTransferInfo{
		Block:  hexutil.Uint64(ctx.NumberU64()),
		Sender: sender.addr,
		Tx:     tx,
	})
	return true
}

func (m *modValueTransfer) txInfo() any {
	return m.txs
}

func randomRecipient(blocknum *big.Int) (a common.Address) {
	h := sha256.Sum256(blocknum.Bytes())
	a.SetBytes(h[:20])
	return a
}
