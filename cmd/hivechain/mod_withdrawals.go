package main

import (
	"github.com/ethereum/go-ethereum/core/types"
)

func init() {
	register("withdrawals", func() blockModifier {
		return &modWithdrawals{
			info: make(map[uint64]withdrawalsInfo),
		}
	})
}

type modWithdrawals struct {
	info map[uint64]withdrawalsInfo
}

type withdrawalsInfo struct {
	Withdrawals []*types.Withdrawal `json:"withdrawals"`
}

func (m *modWithdrawals) apply(ctx *genBlockContext) bool {
	if !ctx.ChainConfig().IsShanghai(ctx.Number(), ctx.Timestamp()) {
		return false
	}
	info := m.info[ctx.NumberU64()]
	if len(info.Withdrawals) >= 2 {
		return false
	}

	w := types.Withdrawal{
		Validator: 5,
		Address:   randomRecipient(ctx.Number()),
		Amount:    100,
	}
	w.Index = ctx.block.AddWithdrawal(&w)
	info.Withdrawals = append(info.Withdrawals, &w)
	m.info[ctx.NumberU64()] = info
	return true
}

func (m *modWithdrawals) txInfo() any {
	return m.info
}
