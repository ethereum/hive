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
}

//go:generate go run github.com/fjl/gencodec -type blockHeader -field-override blockHeader -out gen_bh.go
type blockHeader struct {
	ParentHash          *common.Hash          `json:"parentHash"`
	UncleHash           *common.Hash          `json:"sha3Uncles"` // name in std json
	UncleHashAlt        *common.Hash          `json:"uncleHash"`  // name in fixtures
	Coinbase            *common.Address       `json:"coinbase"`   // name in fixtures
	CoinbaseAlt         *common.Address       `json:"author"`     // nethermind/parity/oe name
	CoinbaseAlt2        *common.Address       `json:"miner"`      // geth/besu name
	StateRoot           *common.Hash          `json:"stateRoot"`
	TransactionsTrie    *common.Hash          `json:"transactionsRoot"` // name in std json
	TransactionsTrieAlt *common.Hash          `json:"transactionsTrie"` // name in fixtures
	ReceiptTrie         *common.Hash          `json:"receiptsRoot"`     // name in std json
	ReceiptTrieAlt      *common.Hash          `json:"receiptTrie"`      // name in fixturse
	Bloom               *types.Bloom          `json:"bloom"`
	Difficulty          *math.HexOrDecimal256 `json:"difficulty"`
	Number              *math.HexOrDecimal256 `json:"number"`
	GasLimit            *math.HexOrDecimal64  `json:"gasLimit"`
	GasUsed             *math.HexOrDecimal64  `json:"gasUsed"`
	Timestamp           *math.HexOrDecimal256 `json:"timestamp"`
	ExtraData           *hexutil.Bytes        `json:"extraData"`
	MixHash             *common.Hash          `json:"mixHash"`
	Nonce               *types.BlockNonce     `json:"nonce"`
	BaseFee             *math.HexOrDecimal256 `json:"baseFeePerGas"`
	Hash                *common.Hash          `json:"hash"`
	WithdrawalsRoot     *common.Hash          `json:"withdrawalsRoot"`
}

//go:generate go run github.com/fjl/gencodec -type transactions -field-override transactions -out gen_txs.go
type transactions struct {
	Nonce     *math.HexOrDecimal256 `json:"nonce"`
	To        *common.Address       `json:"to.omitempty"`
	Value     *math.HexOrDecimal256 `json:"value"`
	Data      *hexutil.Bytes        `json:"data"`
	GasLimit  *math.HexOrDecimal64  `json:"gasLimit"`
	GasUsed   *math.HexOrDecimal64  `json:"gasUsed"`
	SecretKey *common.Hash          `json:"secretKey"`
}

//go:generate go run github.com/fjl/gencodec -type withdrawals -field-override withdrawals -out gen_wds.go
type withdrawals struct {
	Index          *math.HexOrDecimal256 `json:"index"`
	ValidatorIndex *math.HexOrDecimal256 `json:"validatorIndex"`
	Address        *common.Address       `json:"address"`
	Amount         *math.HexOrDecimal256 `json:"amount"`
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
		Timestamp:  (*big.Int)(fixture.Genesis.Timestamp).Uint64(),
		Number:     (*big.Int)(fixture.Genesis.Number).Uint64(),
		Difficulty: (*big.Int)(fixture.Genesis.Difficulty),
		BaseFee:    (*big.Int)(fixture.Genesis.BaseFee),
		GasLimit:   uint64(*fixture.Genesis.GasLimit),
		GasUsed:    uint64(*fixture.Genesis.GasUsed),
		Nonce:      fixture.Genesis.Nonce.Uint64(),
		ParentHash: *fixture.Genesis.ParentHash,
		ExtraData:  *fixture.Genesis.ExtraData,
		Mixhash:    *fixture.Genesis.MixHash,
		Coinbase:   *fixture.Genesis.Coinbase,
		Alloc:      fixture.Pre,
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
