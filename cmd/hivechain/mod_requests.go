package main

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
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
  return false
}

func (m *modRequestWithdrawal) txInfo() any {
	return m.info
}
