package main

import (
	"encoding/json"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/common/math"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/types"
)

type testcase struct {
	// test meta data
	name       string
	filepath   string
	clientType string
	failedErr  error
	// test fixture data
	fixture   fixtureTest
	genesis   *core.Genesis
	postAlloc *core.GenesisAlloc
	payloads  []*engineNewPayload
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

//go:generate go run github.com/fjl/gencodec -type blockHeader -field-override blockHeaderUnmarshaling -out gen_bh.go
//go:generate go run github.com/fjl/gencodec -type transaction -field-override transactionUnmarshaling -out gen_txs.go
//go:generate go run github.com/fjl/gencodec -type withdrawals -field-override withdrawalsUnmarshaling -out gen_wds.go
type fixtureJSON struct {
	Blocks           []block             `json:"blocks"`
	Genesis          blockHeader         `json:"genesisBlockHeader"`
	Pre              core.GenesisAlloc   `json:"pre"`
	Post             core.GenesisAlloc   `json:"postState"`
	Fork             string              `json:"network"`
	EngineFcuVersion math.HexOrDecimal64 `json:"engineFcuVersion"`
}

type block struct {
	Rlp              string            `json:"rlp"`
	BlockHeader      *blockHeader      `json:"blockHeader"`
	Transactions     []transaction     `json:"transactions"`
	UncleHeaders     []byte            `json:"uncleHeaders"`
	Withdrawals      []withdrawals     `json:"withdrawals"`
	Exception        string            `json:"expectException"`
	EngineNewPayload *engineNewPayload `json:"engineNewPayload"`
}

type blockHeader struct {
	ParentHash      common.Hash      `json:"parentHash"`
	UncleHash       common.Hash      `json:"uncleHash"`
	Coinbase        common.Address   `json:"coinbase"`
	StateRoot       common.Hash      `json:"stateRoot"`
	TransactionTrie common.Hash      `json:"transactionTrie"`
	ReceiptTrie     common.Hash      `json:"receiptTrie"`
	Bloom           types.Bloom      `json:"bloom"`
	Difficulty      *big.Int         `json:"difficulty"`
	Number          *big.Int         `json:"number"`
	GasLimit        uint64           `json:"gasLimit"`
	GasUsed         uint64           `json:"gasUsed"`
	Timestamp       *big.Int         `json:"timestamp"`
	ExtraData       []byte           `json:"extraData"`
	MixHash         common.Hash      `json:"mixHash"`
	Nonce           types.BlockNonce `json:"nonce"`
	BaseFee         *big.Int         `json:"baseFeePerGas"`
	WithdrawalsRoot common.Hash      `json:"withdrawalsRoot"`
	BlobGasUsed     *uint64          `json:"blobGasUsed"`
	ExcessBlobGas   *uint64          `json:"excessBlobGas"`
	BeaconRoot      *common.Hash     `json:"parentBeaconBlockRoot"`
	Hash            common.Hash      `json:"hash"`
}

type blockHeaderUnmarshaling struct {
	Difficulty    *math.HexOrDecimal256 `json:"difficulty"`
	Number        *math.HexOrDecimal256 `json:"number"`
	GasLimit      math.HexOrDecimal64   `json:"gasLimit"`
	GasUsed       math.HexOrDecimal64   `json:"gasUsed"`
	Timestamp     *math.HexOrDecimal256 `json:"timestamp"`
	ExtraData     hexutil.Bytes         `json:"extraData"`
	BaseFee       *math.HexOrDecimal256 `json:"baseFeePerGas"`
	BlobGasUsed   *math.HexOrDecimal64  `json:"dataGasUsed"`
	ExcessBlobGas *math.HexOrDecimal64  `json:"excessDataGas"`
}

type transaction struct {
	Type                 *uint64           `json:"type"`
	ChainId              *big.Int          `json:"chainId"`
	Nonce                uint64            `json:"nonce"`
	MaxPriorityFeePerGas *big.Int          `json:"maxPriorityFeePerGas"`
	MaxFeePerGas         *big.Int          `json:"maxFeePerGas"`
	To                   string            `json:"to"`
	Gas                  uint64            `json:"gasLimit"`
	Value                *big.Int          `json:"value"`
	Input                []byte            `json:"data"`
	AccessList           *types.AccessList `json:"accessList"`
	MaxFeePerBlobGas     *big.Int          `json:"maxFeePerBlobGas"`
	BlobVersionedHashes  []*common.Hash    `json:"blobVersionedHashes"`
	V                    *big.Int          `json:"v"`
	R                    *big.Int          `json:"r"`
	S                    *big.Int          `json:"s"`
	Sender               common.Address    `json:"sender"`
}

type transactionUnmarshaling struct {
	Type                 *math.HexOrDecimal64  `json:"type"`
	ChainId              *math.HexOrDecimal256 `json:"chainId"`
	Nonce                math.HexOrDecimal64   `json:"nonce"`
	MaxPriorityFeePerGas *math.HexOrDecimal256 `json:"maxPriorityFeePerGas"`
	MaxFeePerGas         *math.HexOrDecimal256 `json:"maxFeePerGas"`
	Gas                  math.HexOrDecimal64   `json:"gasLimit"`
	Value                *math.HexOrDecimal256 `json:"value"`
	Input                hexutil.Bytes         `json:"data"`
	MaxFeePerBlobGas     *math.HexOrDecimal256 `json:"maxFeePerBlobGas"`
	V                    *math.HexOrDecimal256 `json:"v"`
	R                    *math.HexOrDecimal256 `json:"r"`
	S                    *math.HexOrDecimal256 `json:"s"`
}

type withdrawals struct {
	Index          uint64         `json:"index"`
	ValidatorIndex uint64         `json:"validatorIndex"`
	Address        common.Address `json:"address"`
	Amount         uint64         `json:"amount"`
}

type withdrawalsUnmarshaling struct {
	Index          math.HexOrDecimal64 `json:"index"`
	ValidatorIndex math.HexOrDecimal64 `json:"validatorIndex"`
	Amount         math.HexOrDecimal64 `json:"amount"`
}

type engineNewPayload struct {
	Payload               interface{}         `json:"executionPayload"`
	BlobVersionedHashes   []common.Hash       `json:"expectedBlobVersionedHashes"`
	ParentBeaconBlockRoot *common.Hash        `json:"parentBeaconBlockRoot"`
	Version               math.HexOrDecimal64 `json:"version"`
	ErrorCode             int64               `json:"errorCode,string"`
}
