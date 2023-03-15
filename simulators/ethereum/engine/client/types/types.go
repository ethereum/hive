package types

import (
	"math/big"

	api "github.com/ethereum/go-ethereum/beacon/engine"
	"github.com/ethereum/go-ethereum/common"
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

// ExecutableData is the data necessary to execute an EL payload.
//
//go:generate go run github.com/fjl/gencodec -type ExecutableDataV1 -field-override executableDataV1Marshaling -out gen_edv1.go
type ExecutableDataV1 struct {
	ParentHash    common.Hash    `json:"parentHash"    gencodec:"required"`
	FeeRecipient  common.Address `json:"feeRecipient"  gencodec:"required"`
	StateRoot     common.Hash    `json:"stateRoot"     gencodec:"required"`
	ReceiptsRoot  common.Hash    `json:"receiptsRoot"  gencodec:"required"`
	LogsBloom     []byte         `json:"logsBloom"     gencodec:"required"`
	Random        common.Hash    `json:"prevRandao"    gencodec:"required"`
	Number        uint64         `json:"blockNumber"   gencodec:"required"`
	GasLimit      uint64         `json:"gasLimit"      gencodec:"required"`
	GasUsed       uint64         `json:"gasUsed"       gencodec:"required"`
	Timestamp     uint64         `json:"timestamp"     gencodec:"required"`
	ExtraData     []byte         `json:"extraData"     gencodec:"required"`
	BaseFeePerGas *big.Int       `json:"baseFeePerGas" gencodec:"required"`
	BlockHash     common.Hash    `json:"blockHash"     gencodec:"required"`
	Transactions  [][]byte       `json:"transactions"  gencodec:"required"`
}

// JSON type overrides for executableData.
type executableDataV1Marshaling struct {
	Number        hexutil.Uint64
	GasLimit      hexutil.Uint64
	GasUsed       hexutil.Uint64
	Timestamp     hexutil.Uint64
	BaseFeePerGas *hexutil.Big
	ExtraData     hexutil.Bytes
	LogsBloom     hexutil.Bytes
	Transactions  []hexutil.Bytes
}

func (edv1 *ExecutableDataV1) ToExecutableData() api.ExecutableData {
	return api.ExecutableData{
		ParentHash:    edv1.ParentHash,
		FeeRecipient:  edv1.FeeRecipient,
		StateRoot:     edv1.StateRoot,
		ReceiptsRoot:  edv1.ReceiptsRoot,
		LogsBloom:     edv1.LogsBloom,
		Random:        edv1.Random,
		Number:        edv1.Number,
		GasLimit:      edv1.GasLimit,
		GasUsed:       edv1.GasUsed,
		Timestamp:     edv1.Timestamp,
		ExtraData:     edv1.ExtraData,
		BaseFeePerGas: edv1.BaseFeePerGas,
		BlockHash:     edv1.BlockHash,
		Transactions:  edv1.Transactions,
	}
}

func (edv1 *ExecutableDataV1) FromExecutableData(ed *api.ExecutableData) {
	if ed.Withdrawals != nil {
		panic("source executable data contains withdrawals, not supported by V1")
	}
	edv1.ParentHash = ed.ParentHash
	edv1.FeeRecipient = ed.FeeRecipient
	edv1.StateRoot = ed.StateRoot
	edv1.ReceiptsRoot = ed.ReceiptsRoot
	edv1.LogsBloom = ed.LogsBloom
	edv1.Random = ed.Random
	edv1.Number = ed.Number
	edv1.GasLimit = ed.GasLimit
	edv1.GasUsed = ed.GasUsed
	edv1.Timestamp = ed.Timestamp
	edv1.ExtraData = ed.ExtraData
	edv1.BaseFeePerGas = ed.BaseFeePerGas
	edv1.BlockHash = ed.BlockHash
	edv1.Transactions = ed.Transactions
}
