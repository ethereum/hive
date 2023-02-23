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
	clientType string
	fixture    fixtureTest
	genesis    *core.Genesis
	payloads   []*api.ExecutableData
	postAlloc  core.GenesisAlloc
	filepath   string
}

type fixtureTest struct {
	json fixtureJSON
}

// UnmarshalJSON implements json.Unmarshaler interface.
func (t *fixtureTest) UnmarshalJSON(in []byte) error {
	if err := json.Unmarshal(in, &t.json); err != nil {
		return err
	}
	return nil
}

type fixtureJSON struct {
	Blocks     []block           	 `json:"blocks"`
	Genesis    blockHeader           `json:"genesisBlockHeader"`
	Pre        core.GenesisAlloc     `json:"pre"`
	Post       core.GenesisAlloc     `json:"postState"`
	BestBlock  common.UnprefixedHash `json:"lastblockhash"`
	Network    string                `json:"network"`
	SealEngine string                `json:"sealEngine"`
}

type block struct {
	Rlp          string			     `json:"rlp"`
	BlockHeader  *blockHeader        `json:"blockHeader"`
	Transactions []transactions      `json:"transactions"`
	UncleHeaders []byte              `json:"uncleHeaders"`
	Withdrawals	 []withdrawals       `json:"withdrawals"`
}

type blockHeader struct {
	ParentHash       common.Hash      `json:"parentHash"`
	UncleHash        common.Hash      `json:"sha3Uncles"`
	Coinbase         common.Address   `json:"miner"`
	StateRoot        common.Hash      `json:"stateRoot"`
	TransactionsTrie common.Hash      `json:"transactionsRoot"`
	ReceiptTrie      common.Hash      `json:"receiptsRoot"`
	Bloom            types.Bloom      `json:"bloom"`
	Difficulty       *big.Int         `json:"difficulty`
	Number           *big.Int         `json:"number"`
	GasLimit         uint64           `json:"gasLimit"`
	GasUsed          uint64           `json:"gasUsed"`
	Timestamp        *big.Int         `json:"timestamp"`
	ExtraData        []byte           `json:"extraData"`
	MixHash          common.Hash      `json:"mixHash"`
	Nonce            types.BlockNonce `json:"nonce"`
	BaseFee          *big.Int         `json:"baseFeePerGas"`
	Hash             common.Hash      `json:"hash"`
	WithdrawalsRoot  common.Hash	  `json:"withdrawalsRoot`
}

func (b *blockHeader) UnmarshalJSON(input []byte) error {
	type blockHeader struct {
		ParentHash          *common.Hash          `json:"parentHash"`
		UncleHash           *common.Hash          `json:"sha3Uncles"`       // name in std json
		UncleHashAlt        *common.Hash          `json:"uncleHash"`        // name in fixtures
		Coinbase            *common.Address       `json:"coinbase"` 		// name in fixtures
		CoinbaseAlt         *common.Address       `json:"author"`   		// nethermind/parity/oe name
		CoinbaseAlt2        *common.Address       `json:"miner"`    		// geth/besu name
		StateRoot           *common.Hash          `json:"stateRoot"`
		TransactionsTrie    *common.Hash          `json:"transactionsRoot"` // name in std json
		TransactionsTrieAlt *common.Hash          `json:"transactionsTrie"` // name in fixtures
		ReceiptTrie         *common.Hash          `json:"receiptsRoot"` 	// name in std json
		ReceiptTrieAlt      *common.Hash          `json:"receiptTrie"`  	// name in fixturse
		Bloom               *types.Bloom          `json:"bloom"`
		Difficulty          *math.HexOrDecimal256 `json:"difficulty`
		Number              *math.HexOrDecimal256 `json:"number"`
		GasLimit            *math.HexOrDecimal64  `json:"gasLimit"`
		GasUsed             *math.HexOrDecimal64  `json:"gasUsed"`
		Timestamp           *math.HexOrDecimal256 `json:"timestamp"`
		ExtraData           *hexutil.Bytes        `json:"extraData"`
		MixHash             *common.Hash          `json:"mixHash"`
		Nonce               *types.BlockNonce     `json:"nonce"`
		BaseFee             *math.HexOrDecimal256 `json:"baseFeePerGas"`
		Hash                *common.Hash          `json:"hash"`
		WithdrawalsRoot     *common.Hash		  `json:"withdrawalsRoot`
	}
	var dec blockHeader
	if err := json.Unmarshal(input, &dec); err != nil {
		return err
	}
	if dec.ParentHash != nil {
		b.ParentHash = *dec.ParentHash
	}
	if dec.UncleHash != nil {
		b.UncleHash = *dec.UncleHash
	} else if dec.UncleHashAlt != nil {
		b.UncleHash = *dec.UncleHashAlt
	}
	if dec.Coinbase != nil {
		b.Coinbase = *dec.Coinbase
	} else if dec.CoinbaseAlt != nil {
		b.Coinbase = *dec.CoinbaseAlt
	} else if dec.CoinbaseAlt2 != nil {
		b.Coinbase = *dec.CoinbaseAlt2
	}
	if dec.StateRoot != nil {
		b.StateRoot = *dec.StateRoot
	}
	if dec.TransactionsTrie != nil {
		b.TransactionsTrie = *dec.TransactionsTrie
	} else if dec.TransactionsTrieAlt != nil {
		b.TransactionsTrie = *dec.TransactionsTrieAlt
	}
	if dec.ReceiptTrie != nil {
		b.ReceiptTrie = *dec.ReceiptTrie
	} else if dec.ReceiptTrieAlt != nil {
		b.ReceiptTrie = *dec.ReceiptTrieAlt
	}
	if dec.Bloom != nil {
		b.Bloom = *dec.Bloom
	}
	if dec.Difficulty != nil {
		b.Difficulty = (*big.Int)(dec.Difficulty)
	}
	if dec.Number != nil {
		b.Number = (*big.Int)(dec.Number)
	}
	if dec.GasLimit != nil {
		b.GasLimit = uint64(*dec.GasLimit)
	}
	if dec.GasUsed != nil {
		b.GasUsed = uint64(*dec.GasUsed)
	}
	if dec.Timestamp != nil {
		b.Timestamp = (*big.Int)(dec.Timestamp)
	}
	if dec.ExtraData != nil {
		b.ExtraData = *dec.ExtraData
	}
	if dec.MixHash != nil {
		b.MixHash = *dec.MixHash
	}
	if dec.Nonce != nil {
		b.Nonce = *dec.Nonce
	}
	if dec.BaseFee != nil {
		b.BaseFee = (*big.Int)(dec.BaseFee)
	}
	if dec.Hash != nil {
		b.Hash = *dec.Hash
	}
	if dec.WithdrawalsRoot != nil {
		b.WithdrawalsRoot = *dec.WithdrawalsRoot
	}
	return nil
}

type transactions struct {
	Nonce            *big.Int         `json:"nonce"`
	To               common.Address   `json:"to.omitempty"`
	Value            *big.Int         `json:"value"`
	Data             []byte           `json:"data"`
	GasLimit         uint64           `json:"gasLimit"`
	GasUsed          uint64           `json:"gasUsed"`
	SecretKey		 common.Hash	  `json:"secretKey"`
}

func (t *transactions) UnmarshalJSON(input []byte) error {
	type transactions struct {
		Nonce            *math.HexOrDecimal256 `json:"nonce"`
		To               *common.Address       `json:"to.omitempty"`
		Value            *math.HexOrDecimal256 `json:"value"`
		Data             *hexutil.Bytes        `json:"data"`
		GasLimit         *math.HexOrDecimal64  `json:"gasLimit"`
		GasUsed          *math.HexOrDecimal64  `json:"gasUsed"`
		SecretKey		 *common.Hash	  	   `json:"secretKey"`
	}
	var dec transactions
	if err := json.Unmarshal(input, &dec); err != nil {
		return err
	}
	if dec.Nonce != nil {
		t.Nonce = (*big.Int)(dec.Nonce)
	}
	if dec.To != nil {
		t.To = *dec.To
	}
	if dec.Value != nil {
		t.Value = (*big.Int)(dec.Value)
	}
	if dec.Data != nil {
		t.Data = *dec.Data
	}
	if dec.GasLimit != nil {
		t.GasLimit = uint64(*dec.GasLimit)
	}
	if dec.GasUsed != nil {
		t.GasUsed = uint64(*dec.GasUsed)
	}
	if dec.SecretKey != nil {
		t.SecretKey = *dec.SecretKey
	}
	return nil
}

type withdrawals struct {
	Index            *big.Int       `json:"index"`
	ValidatorIndex   *big.Int       `json:"validatorIndex"`
	Address          common.Address `json:"address"`
	Amount           *big.Int       `json:"amount"`
}
func (w withdrawals) UnmarshalJSON(input []byte) error {
	type withdrawals struct {
		Index            *math.HexOrDecimal256  `json:"index"`
		ValidatorIndex   *math.HexOrDecimal256  `json:"validatorIndex"`
		Address          *common.Address	    `json:"address"`
		Amount           *math.HexOrDecimal256  `json:"amount"`
	}
	var dec withdrawals
	if err := json.Unmarshal(input, &dec); err != nil {
		return err
	}
	if dec.Index != nil {
		w.Index = (*big.Int)(dec.Index)
	}
	if dec.ValidatorIndex != nil {
		w.ValidatorIndex = (*big.Int)(dec.Index)
	}
	if dec.Address != nil {
		w.Address = *dec.Address
	}
	if dec.Amount != nil {
		w.Amount = (*big.Int)(dec.Amount)
	}
	return nil
}

func extractGenesis(fixture fixtureJSON) (*core.Genesis){
	genesis := &core.Genesis{
		Nonce:      fixture.Genesis.Nonce.Uint64(),
		Timestamp:  fixture.Genesis.Timestamp.Uint64(),
		ParentHash: fixture.Genesis.ParentHash,
		ExtraData:  fixture.Genesis.ExtraData,
		GasLimit:   fixture.Genesis.GasLimit,
		GasUsed:    fixture.Genesis.GasUsed,
		Number: 	fixture.Genesis.Number.Uint64(),
		Difficulty: fixture.Genesis.Difficulty,
		Mixhash:    fixture.Genesis.MixHash,
		Coinbase:   fixture.Genesis.Coinbase,
		Alloc:      fixture.Pre,
		BaseFee:    fixture.Genesis.BaseFee,
	}
	return genesis
}

func (bl *block) decodeBlock() (*types.Block, error) {
	data, err := hexutil.Decode(bl.Rlp)
	if err != nil {
		return nil, err
	}
	var b types.Block
	err = rlp.DecodeBytes(data, &b)
	return &b, err
}