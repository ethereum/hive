package main

import (
	"encoding/json"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/common/math"
	"github.com/ethereum/go-ethereum/core"
	api "github.com/ethereum/go-ethereum/core/beacon"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rlp"
)

type testcase struct {
	name       string
	filepath   string
	clientType string
	fixture    fixtureTest
	genesis    *core.Genesis
	payloads   []*api.ExecutableData
	postAlloc  *core.GenesisAlloc
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
	Rlp          string         `json:"rlp"`
	BlockHeader  *blockHeader   `json:"blockHeader"`
	Transactions []transactions `json:"transactions"`
	UncleHeaders []byte         `json:"uncleHeaders"`
	Withdrawals  []withdrawals  `json:"withdrawals"`
	Exception    string         `json:"expectException"`
}

//go:generate go run github.com/fjl/gencodec -type blockHeader -field-override blockHeaderUnmarshaling -out gen_bh.go
type blockHeader struct {
	ParentHash          common.Hash      `json:"parentHash"`
	UncleHash           common.Hash      `json:"sha3Uncles"`
	UncleHashAlt        common.Hash      `json:"uncleHash"` // name in fixtures
	Coinbase            common.Address   `json:"coinbase"`
	CoinbaseAlt         common.Address   `json:"author"` // nethermind/parity/oe name
	CoinbaseAlt2        common.Address   `json:"miner"`  // geth/besu name
	StateRoot           common.Hash      `json:"stateRoot"`
	TransactionsTrie    common.Hash      `json:"transactionsRoot"`
	TransactionsTrieAlt common.Hash      `json:"transactionsTrie"` // name in fixtures
	ReceiptTrie         common.Hash      `json:"receiptsRoot"`
	ReceiptTrieAlt      common.Hash      `json:"receiptTrie"` // name in fixturse
	Bloom               types.Bloom      `json:"bloom"`
	Difficulty          *big.Int         `json:"difficulty"`
	Number              *big.Int         `json:"number"`
	GasLimit            uint64           `json:"gasLimit"`
	GasUsed             uint64           `json:"gasUsed"`
	Timestamp           *big.Int         `json:"timestamp"`
	ExtraData           []byte           `json:"extraData"`
	MixHash             common.Hash      `json:"mixHash"`
	Nonce               types.BlockNonce `json:"nonce"`
	BaseFee             *big.Int         `json:"baseFeePerGas"`
	Hash                common.Hash      `json:"hash"`
	WithdrawalsRoot     common.Hash      `json:"withdrawalsRoot"`
}

type blockHeaderUnmarshaling struct {
	Difficulty *math.HexOrDecimal256 `json:"difficulty"`
	Number     *math.HexOrDecimal256 `json:"number"`
	GasLimit   math.HexOrDecimal64   `json:"gasLimit"`
	GasUsed    math.HexOrDecimal64   `json:"gasUsed"`
	Timestamp  *math.HexOrDecimal256 `json:"timestamp"`
	ExtraData  hexutil.Bytes         `json:"extraData"`
	BaseFee    *math.HexOrDecimal256 `json:"baseFeePerGas"`
}

//go:generate go run github.com/fjl/gencodec -type transactions -field-override transactionsUnmarshaling -out gen_txs.go
type transactions struct {
	Nonce     uint64         `json:"nonce"`
	To        common.Address `json:"to.omitempty"`
	Value     *big.Int       `json:"value"`
	Data      []byte         `json:"data"`
	GasLimit  uint64         `json:"gasLimit"`
	GasUsed   uint64         `json:"gasUsed"`
	SecretKey common.Hash    `json:"secretKey"`
}

type transactionsUnmarshaling struct {
	Nonce    math.HexOrDecimal64   `json:"nonce"`
	Value    *math.HexOrDecimal256 `json:"value"`
	Data     hexutil.Bytes         `json:"data"`
	GasLimit math.HexOrDecimal64   `json:"gasLimit"`
	GasUsed  math.HexOrDecimal64   `json:"gasUsed"`
}

//go:generate go run github.com/fjl/gencodec -type withdrawals -field-override withdrawalsUnmarshaling -out gen_wds.go
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

// -------------------------------------------------------------------------- //
// extractFixtureFields() extracts the genesis, payloads and post allocation  //
// fields from the given fixture test and stores them in the testcase struct. //
// -------------------------------------------------------------------------- //
func (tc *testcase) extractFixtureFields(fixture fixtureJSON) {
	// extract genesis fields from fixture test
	tc.genesis = extractGenesis(fixture)

	// extract payloads from each block
	payloads := []*api.ExecutableData{}
	for _, block := range fixture.Blocks {
		block, _ := block.decodeBlock()
		payload := api.BlockToExecutableData(block, common.Big0).ExecutionPayload
		payloads = append(payloads, payload)
	}
	tc.payloads = payloads

	// extract post account information
	tc.postAlloc = &fixture.Post
}

// ------------------------------------------------------------------------------ //
// extractGenesis() extracts the genesis block information from the given fixture //
// and returns a core.Genesis struct containing the extracted information.        //
// ------------------------------------------------------------------------------ //
func extractGenesis(fixture fixtureJSON) *core.Genesis {
	genesis := &core.Genesis{
		Timestamp:  fixture.Genesis.Timestamp.Uint64(),
		Nonce:      fixture.Genesis.Nonce.Uint64(),
		Difficulty: fixture.Genesis.Difficulty,
		GasLimit:   fixture.Genesis.GasLimit,
		ExtraData:  fixture.Genesis.ExtraData,
		Mixhash:    fixture.Genesis.MixHash,
		Coinbase:   fixture.Genesis.Coinbase,
		Alloc:      fixture.Pre,
		BaseFee:    fixture.Genesis.BaseFee,
	}
	return genesis
}

// ------------------------------------------------------------------------- //
// decodeBlock() decodes the RLP-encoded block data in the block struct and  //
// returns a types.Block struct containing the decoded information.          //
// ------------------------------------------------------------------------- //
func (bl *block) decodeBlock() (*types.Block, error) {
	data, err := hexutil.Decode(bl.Rlp)
	if err != nil {
		return nil, err
	}
	var b types.Block
	err = rlp.DecodeBytes(data, &b)
	return &b, err
}
