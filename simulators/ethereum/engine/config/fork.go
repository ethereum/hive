package config

import (
	"math/big"
)

type Fork string

const (
	Paris    Fork = "Paris"
	Shanghai Fork = "Shanghai"
	Cancun   Fork = "Cancun"
)

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
