package main

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/params"
)

func init() {
	register("tx-request-eip7002", func() blockModifier {
		return &modRequestWithdrawal{}
	})
}

// modRequestWithdrawal creates an EIP-7002 withdrawal request.
type modRequestWithdrawal struct {
	info *modRequestWithdrawalInfo
}

type modRequestWithdrawalInfo struct {
	TxHash common.Hash    `json:"txhash"`
	Block  hexutil.Uint64 `json:"block"`
}

func (m *modRequestWithdrawal) apply(ctx *genBlockContext) bool {
	if m.info != nil {
		return false // run only once
	}
	if !ctx.ChainConfig().IsPrague(ctx.Number(), ctx.Timestamp()) {
		return false
	}

	const gas = 150_000
	if !ctx.HasGas(gas) {
		return false
	}

	sender := ctx.TxSenderAccount()
	input := common.FromHex("b917cfdc0d25b72d55cf94db328e1629b7f4fde2c30cdacf873b664416f76a0c7f7cc50c9f72a3cb84be88144cde91250000000000000d80")
	tx := ctx.AddNewTx(sender, &types.DynamicFeeTx{
		ChainID:   ctx.ChainConfig().ChainID,
		Nonce:     ctx.AccountNonce(sender.addr),
		Value:     big.NewInt(1000000000),
		To:        &params.WithdrawalQueueAddress,
		Data:      input,
		GasFeeCap: ctx.TxGasFeeCap(),
		GasTipCap: big.NewInt(2),
		Gas:       gas,
	})
	m.info = &modRequestWithdrawalInfo{
		TxHash: tx.Hash(),
		Block:  hexutil.Uint64(ctx.Number().Uint64()),
	}
	return true
}

func (m *modRequestWithdrawal) txInfo() any {
	return m.info
}
