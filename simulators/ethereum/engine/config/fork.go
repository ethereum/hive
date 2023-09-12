package config

import (
	"math/big"

	"github.com/ethereum/go-ethereum/core/types"
)

type Fork string

const (
	NA       Fork = ""
	Paris    Fork = "Paris"
	Shanghai Fork = "Shanghai"
	Cancun   Fork = "Cancun"
)

func (f Fork) PreviousFork() Fork {
	switch f {
	case Shanghai:
		return Paris
	case Cancun:
		return Shanghai
	default:
		return NA
	}
}

type ForkConfig struct {
	ShanghaiTimestamp *big.Int
	CancunTimestamp   *big.Int
}

func (f *ForkConfig) IsShanghai(blockTimestamp uint64) bool {
	return f.ShanghaiTimestamp != nil && new(big.Int).SetUint64(blockTimestamp).Cmp(f.ShanghaiTimestamp) >= 0
}

func (f *ForkConfig) IsCancun(blockTimestamp uint64) bool {
	return f.CancunTimestamp != nil && new(big.Int).SetUint64(blockTimestamp).Cmp(f.CancunTimestamp) >= 0
}

func (f *ForkConfig) ForkchoiceUpdatedVersion(headTimestamp uint64, payloadAttributesTimestamp *uint64) int {
	// If the payload attributes timestamp is nil, use the head timestamp
	// to calculate the FcU version.
	timestamp := headTimestamp
	if payloadAttributesTimestamp != nil {
		timestamp = *payloadAttributesTimestamp
	}

	if f.IsCancun(timestamp) {
		return 3
	} else if f.IsShanghai(timestamp) {
		return 2
	}
	return 1
}

func (f *ForkConfig) NewPayloadVersion(timestamp uint64) int {
	if f.IsCancun(timestamp) {
		return 3
	} else if f.IsShanghai(timestamp) {
		return 2
	}
	return 1
}

func (f *ForkConfig) GetPayloadVersion(timestamp uint64) int {
	if f.IsCancun(timestamp) {
		return 3
	} else if f.IsShanghai(timestamp) {
		return 2
	}
	return 1
}

func (f *ForkConfig) GetSupportedTransactionTypes(timestamp uint64) []int {
	if f.IsCancun(timestamp) {
		// Put the blob type at the start to try to guarantee at least one blob tx makes it into the test
		return []int{types.BlobTxType, types.LegacyTxType /* types.AccessListTxType,*/, types.DynamicFeeTxType}
	}
	return []int{types.LegacyTxType /* types.AccessListTxType,*/, types.DynamicFeeTxType}
}
