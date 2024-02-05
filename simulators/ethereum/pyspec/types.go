package main

import (
	"encoding/json"
	"math/big"

	api "github.com/ethereum/go-ethereum/beacon/engine"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/common/math"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/types"

	typ "github.com/ethereum/hive/simulators/ethereum/engine/types"
)

type testcase struct {
	// test meta data
	name       string
	filepath   string
	clientType string
	failedErr  error
	// test fixture data
	fixture           fixtureTest
	genesis           *core.Genesis
	postAlloc         *core.GenesisAlloc
	engineNewPayloads []engineNewPayload
}

type fixtureTest struct {
	json fixtureJSON
}

func (t *fixtureTest) UnmarshalJSON(in []byte) error {
	if err := json.Unmarshal(in, &t.json); err != nil {
		return err
	}
	return nil
}

type fixtureJSON struct {
	Fork              string              `json:"network"`
	Genesis           genesisBlock        `json:"genesisBlockHeader"`
	EngineNewPayloads []engineNewPayload  `json:"engineNewPayloads"`
	EngineFcuVersion  math.HexOrDecimal64 `json:"engineFcuVersion"`
	Pre               core.GenesisAlloc   `json:"pre"`
	Post              core.GenesisAlloc   `json:"postState"`
}

//go:generate go run github.com/fjl/gencodec -type genesisBlock -field-override genesisBlockUnmarshaling -out gen_gb.go
type genesisBlock struct {
	Coinbase      common.Address   `json:"coinbase"`
	Difficulty    *big.Int         `json:"difficulty"`
	GasLimit      uint64           `json:"gasLimit"`
	Timestamp     *big.Int         `json:"timestamp"`
	ExtraData     []byte           `json:"extraData"`
	MixHash       common.Hash      `json:"mixHash"`
	Nonce         types.BlockNonce `json:"nonce"`
	BaseFee       *big.Int         `json:"baseFeePerGas"`
	BlobGasUsed   *uint64          `json:"blobGasUsed"`
	ExcessBlobGas *uint64          `json:"excessBlobGas"`

	Hash common.Hash `json:"hash"`
}

type genesisBlockUnmarshaling struct {
	Difficulty    *math.HexOrDecimal256 `json:"difficulty"`
	GasLimit      math.HexOrDecimal64   `json:"gasLimit"`
	Timestamp     *math.HexOrDecimal256 `json:"timestamp"`
	ExtraData     hexutil.Bytes         `json:"extraData"`
	BaseFee       *math.HexOrDecimal256 `json:"baseFeePerGas"`
	BlobGasUsed   *math.HexOrDecimal64  `json:"dataGasUsed"`
	ExcessBlobGas *math.HexOrDecimal64  `json:"excessDataGas"`
}

type engineNewPayload struct {
	ExecutionPayload      *api.ExecutableData `json:"executionPayload"`
	BlobVersionedHashes   []common.Hash       `json:"expectedBlobVersionedHashes"`
	ParentBeaconBlockRoot *common.Hash        `json:"parentBeaconBlockRoot"`
	Version               math.HexOrDecimal64 `json:"version"`
	ValidationError       *string             `json:"validationError"`
	ErrorCode             int64               `json:"errorCode,string"`

	HiveExecutionPayload *typ.ExecutableData
}
