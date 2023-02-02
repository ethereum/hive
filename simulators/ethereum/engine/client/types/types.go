package types

import (
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
)

//go:generate go run github.com/fjl/gencodec -type ExecutionPayloadBodyV1 -field-override executionPayloadBodyV1Marshaling -out gen_epbv1.go
type ExecutionPayloadBodyV1 struct {
	Transactions [][]byte            `json:"transactions"  gencodec:"required"`
	Withdrawals  []*types.Withdrawal `json:"withdrawals"`
}

// JSON type overrides for executableData.
type executionPayloadBodyV1Marshaling struct {
	Transactions []hexutil.Bytes
}
