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

type fixtureJSON struct {
	Blocks     []block               `json:"blocks"`
	Genesis    blockHeader           `json:"genesisBlockHeader"`
	Pre        core.GenesisAlloc     `json:"pre"`
	Post       core.GenesisAlloc     `json:"postState"`
	BestBlock  common.UnprefixedHash `json:"lastblockhash"`
	Network    string                `json:"network"`
	SealEngine string                `json:"sealEngine"`
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

//go:generate go run github.com/fjl/gencodec -type blockHeader -field-override blockHeaderUnmarshaling -out gen_bh.go
//go:generate go run github.com/fjl/gencodec -type transaction -field-override transactionUnmarshaling -out gen_txs.go
//go:generate go run github.com/fjl/gencodec -type withdrawals -field-override withdrawalsUnmarshaling -out gen_wds.go
//go:generate go run github.com/fjl/gencodec -type engineNewPayload -field-override engineNewPayloadUnmarshaling -out gen_enp.go
type blockHeader struct {
	ParentHash         common.Hash      `json:"parentHash"`
	UncleHash          common.Hash      `json:"sha3Uncles"`
	UncleHashAlt       common.Hash      `json:"uncleHash"` // name in fixtures
	Coinbase           common.Address   `json:"coinbase"`
	CoinbaseAlt        common.Address   `json:"author"` // nethermind name
	CoinbaseAlt2       common.Address   `json:"miner"`  // geth/besu name
	StateRoot          common.Hash      `json:"stateRoot"`
	TransactionTrie    common.Hash      `json:"transactionRoot"`
	TransactionTrieAlt common.Hash      `json:"transactionTrie"` // name in fixtures
	ReceiptTrie        common.Hash      `json:"receiptsRoot"`
	ReceiptTrieAlt     common.Hash      `json:"receiptTrie"` // name in fixtures
	Bloom              types.Bloom      `json:"bloom"`
	Difficulty         *big.Int         `json:"difficulty"`
	Number             *big.Int         `json:"number"`
	GasLimit           uint64           `json:"gasLimit"`
	GasUsed            uint64           `json:"gasUsed"`
	Timestamp          *big.Int         `json:"timestamp"`
	ExtraData          []byte           `json:"extraData"`
	MixHash            common.Hash      `json:"mixHash"`
	Nonce              types.BlockNonce `json:"nonce"`
	BaseFee            *big.Int         `json:"baseFeePerGas"`
	Hash               common.Hash      `json:"hash"`
	WithdrawalsRoot    common.Hash      `json:"withdrawalsRoot"`
	BlobGasUsed        *uint64          `json:"blobGasUsed"`
	ExcessBlobGas      *uint64          `json:"excessBlobGas"`
	BeaconRoot         *common.Hash     `json:"parentBeaconBlockRoot"`
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
	GasPrice             *big.Int          `json:"gasPrice"`
	MaxPriorityFeePerGas *big.Int          `json:"maxPriorityFeePerGas"`
	MaxFeePerGas         *big.Int          `json:"maxFeePerGas"`
	Gas                  uint64            `json:"gasLimit"`
	Value                *big.Int          `json:"value"`
	Input                []byte            `json:"data"`
	To                   string            `json:"to"`
	Protected            bool              `json:"protected"`
	AccessList           *types.AccessList `json:"accessList"`
	SecretKey            *common.Hash      `json:"secretKey"`
	V                    *big.Int          `json:"v"`
	R                    *big.Int          `json:"r"`
	S                    *big.Int          `json:"s"`
	MaxFeePerDataGas     *big.Int          `json:"maxFeePerDataGas"`
	BlobVersionedHashes  []*common.Hash    `json:"blobVersionedHashes"`
}

type transactionUnmarshaling struct {
	Type                 *math.HexOrDecimal64  `json:"type"`
	ChainId              *math.HexOrDecimal256 `json:"chainId"`
	Nonce                math.HexOrDecimal64   `json:"nonce"`
	GasPrice             *math.HexOrDecimal256 `json:"gasPrice"`
	MaxPriorityFeePerGas *math.HexOrDecimal256 `json:"maxPriorityFeePerGas"`
	MaxFeePerGas         *math.HexOrDecimal256 `json:"maxFeePerGas"`
	Gas                  math.HexOrDecimal64   `json:"gasLimit"`
	Value                *math.HexOrDecimal256 `json:"value"`
	Input                hexutil.Bytes         `json:"data"`
	V                    *math.HexOrDecimal256 `json:"v"`
	R                    *math.HexOrDecimal256 `json:"r"`
	S                    *math.HexOrDecimal256 `json:"s"`
	MaxFeePerDataGas     *math.HexOrDecimal256 `json:"maxFeePerDataGas"`
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
	Payload               *api.ExecutableData `json:"executionPayload"`
	BlobVersionedHashes   []common.Hash       `json:"expectedBlobVersionedHashes"`
	ParentBeaconBlockRoot *common.Hash        `json:"parentBeaconBlockRoot"`
	Version               uint64              `json:"version"`
	ErrorCode             int64               `json:"errorCode,string"`
}

type engineNewPayloadUnmarshaling struct {
	Version math.HexOrDecimal64 `json:"version"`
}

func (p *engineNewPayload) ToExecutableData() (*typ.ExecutableData, error) {
	ed, err := typ.FromBeaconExecutableData(p.Payload)
	if err != nil {
		return nil, err
	}
	ed.VersionedHashes = &p.BlobVersionedHashes
	ed.ParentBeaconBlockRoot = p.ParentBeaconBlockRoot
	return &ed, nil
}
