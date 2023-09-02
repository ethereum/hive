package types

import (
	"math/big"

	geth_beacon "github.com/ethereum/go-ethereum/beacon/engine"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
)

type BinaryMarshable interface {
	MarshalBinary() ([]byte, error)
}

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

func (edv1 *ExecutableDataV1) ToExecutableData() ExecutableData {
	return ExecutableData{
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

func (edv1 *ExecutableDataV1) FromExecutableData(ed *ExecutableData) {
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

//go:generate go run github.com/fjl/gencodec -type PayloadAttributes -field-override payloadAttributesMarshaling -out gen_blockparams.go

// PayloadAttributes describes the environment context in which a block should
// be built.
type PayloadAttributes struct {
	Timestamp             uint64              `json:"timestamp"             gencodec:"required"`
	Random                common.Hash         `json:"prevRandao"            gencodec:"required"`
	SuggestedFeeRecipient common.Address      `json:"suggestedFeeRecipient" gencodec:"required"`
	Withdrawals           []*types.Withdrawal `json:"withdrawals"`
	BeaconRoot            *common.Hash        `json:"parentBeaconBlockRoot"`
}

// JSON type overrides for PayloadAttributes.
type payloadAttributesMarshaling struct {
	Timestamp hexutil.Uint64
}

//go:generate go run github.com/fjl/gencodec -type ExecutableData -field-override executableDataMarshaling -out gen_ed.go

// ExecutableData is the data necessary to execute an EL payload.
type ExecutableData struct {
	ParentHash    common.Hash         `json:"parentHash"    gencodec:"required"`
	FeeRecipient  common.Address      `json:"feeRecipient"  gencodec:"required"`
	StateRoot     common.Hash         `json:"stateRoot"     gencodec:"required"`
	ReceiptsRoot  common.Hash         `json:"receiptsRoot"  gencodec:"required"`
	LogsBloom     []byte              `json:"logsBloom"     gencodec:"required"`
	Random        common.Hash         `json:"prevRandao"    gencodec:"required"`
	Number        uint64              `json:"blockNumber"   gencodec:"required"`
	GasLimit      uint64              `json:"gasLimit"      gencodec:"required"`
	GasUsed       uint64              `json:"gasUsed"       gencodec:"required"`
	Timestamp     uint64              `json:"timestamp"     gencodec:"required"`
	ExtraData     []byte              `json:"extraData"     gencodec:"required"`
	BaseFeePerGas *big.Int            `json:"baseFeePerGas" gencodec:"required"`
	BlockHash     common.Hash         `json:"blockHash"     gencodec:"required"`
	Transactions  [][]byte            `json:"transactions"  gencodec:"required"`
	Withdrawals   []*types.Withdrawal `json:"withdrawals"`
	BlobGasUsed   *uint64             `json:"blobGasUsed,omitempty"`
	ExcessBlobGas *uint64             `json:"excessBlobGas,omitempty"`

	// NewPayload parameters
	VersionedHashes       *[]common.Hash `json:"-"`
	ParentBeaconBlockRoot *common.Hash   `json:"-"`

	// Payload Attributes used to build the block
	PayloadAttributes PayloadAttributes `json:"-"`
}

// JSON type overrides for executableData.
type executableDataMarshaling struct {
	Number        hexutil.Uint64
	GasLimit      hexutil.Uint64
	GasUsed       hexutil.Uint64
	Timestamp     hexutil.Uint64
	BaseFeePerGas *hexutil.Big
	ExtraData     hexutil.Bytes
	LogsBloom     hexutil.Bytes
	Transactions  []hexutil.Bytes
	BlobGasUsed   *hexutil.Uint64
	ExcessBlobGas *hexutil.Uint64
}

//go:generate go run github.com/fjl/gencodec -type ExecutionPayloadEnvelope -field-override executionPayloadEnvelopeMarshaling -out gen_epe.go

type ExecutionPayloadEnvelope struct {
	ExecutionPayload      *ExecutableData `json:"executionPayload"       gencodec:"required"`
	BlockValue            *big.Int        `json:"blockValue"             gencodec:"required"`
	BlobsBundle           *BlobsBundle    `json:"blobsBundle,omitempty"`
	ShouldOverrideBuilder *bool           `json:"shouldOverrideBuilder,omitempty"`
}

type executionPayloadEnvelopeMarshaling struct {
	BlockValue *hexutil.Big
}

// Convert Execution Payload Types
func ToBeaconExecutableData(pl *ExecutableData) (geth_beacon.ExecutableData, error) {
	return geth_beacon.ExecutableData{
		ParentHash:    pl.ParentHash,
		FeeRecipient:  pl.FeeRecipient,
		StateRoot:     pl.StateRoot,
		ReceiptsRoot:  pl.ReceiptsRoot,
		LogsBloom:     pl.LogsBloom,
		Random:        pl.Random,
		Number:        pl.Number,
		GasLimit:      pl.GasLimit,
		GasUsed:       pl.GasUsed,
		Timestamp:     pl.Timestamp,
		ExtraData:     pl.ExtraData,
		BaseFeePerGas: pl.BaseFeePerGas,
		BlockHash:     pl.BlockHash,
		Transactions:  pl.Transactions,
		Withdrawals:   pl.Withdrawals,
		BlobGasUsed:   pl.BlobGasUsed,
		ExcessBlobGas: pl.ExcessBlobGas,
	}, nil
}

func FromBeaconExecutableData(ed *geth_beacon.ExecutableData) (ExecutableData, error) {
	return ExecutableData{
		ParentHash:    ed.ParentHash,
		FeeRecipient:  ed.FeeRecipient,
		StateRoot:     ed.StateRoot,
		ReceiptsRoot:  ed.ReceiptsRoot,
		LogsBloom:     ed.LogsBloom,
		Random:        ed.Random,
		Number:        ed.Number,
		GasLimit:      ed.GasLimit,
		GasUsed:       ed.GasUsed,
		Timestamp:     ed.Timestamp,
		ExtraData:     ed.ExtraData,
		BaseFeePerGas: ed.BaseFeePerGas,
		BlockHash:     ed.BlockHash,
		Transactions:  ed.Transactions,
		Withdrawals:   ed.Withdrawals,
		BlobGasUsed:   ed.BlobGasUsed,
		ExcessBlobGas: ed.ExcessBlobGas,
	}, nil
}

func ExecutableDataToBlock(ed ExecutableData) (*types.Block, error) {
	gethEd, err := ToBeaconExecutableData(&ed)
	if err != nil {
		return nil, err
	}
	var versionedHashes []common.Hash
	if ed.VersionedHashes != nil {
		versionedHashes = *ed.VersionedHashes
	}
	return geth_beacon.ExecutableDataToBlock(gethEd, versionedHashes, ed.ParentBeaconBlockRoot)
}
