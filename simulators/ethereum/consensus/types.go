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

// A BlockTest checks handling of entire blocks.
type BlockTest struct {
	json btJSON
}

// UnmarshalJSON implements json.Unmarshaler interface.
func (t *BlockTest) UnmarshalJSON(in []byte) error {
	return json.Unmarshal(in, &t.json)
}

type btJSON struct {
	Blocks     []btBlock             `json:"blocks"`
	Genesis    btHeader              `json:"genesisBlockHeader"`
	Pre        core.GenesisAlloc     `json:"pre"`
	Post       core.GenesisAlloc     `json:"-"`
	BestBlock  common.UnprefixedHash `json:"lastblockhash"`
	Network    string                `json:"network"`
	SealEngine string                `json:"sealEngine"`
}

type btBlock struct {
	BlockHeader  *btHeader
	Rlp          string
	UncleHeaders []*btHeader
}

type btHeader struct {
	Bloom            types.Bloom      `json:"bloom"`
	Coinbase         common.Address   `json:"miner"`
	MixHash          common.Hash      `json:"mixHash"`
	Nonce            types.BlockNonce `json:"nonce"`
	Number           *big.Int         `json:"number"`
	Hash             common.Hash      `json:"hash"`
	ParentHash       common.Hash      `json:"parentHash"`
	ReceiptTrie      common.Hash      `json:"receiptsRoot"`
	StateRoot        common.Hash      `json:"stateRoot"`
	TransactionsTrie common.Hash      `json:"transactionsRoot"`
	UncleHash        common.Hash      `json:"sha3Uncles"`
	ExtraData        []byte           `json:"extraData"`
	Difficulty       *big.Int         `json:"difficulty"`
	GasLimit         uint64           `json:"gasLimit"`
	GasUsed          uint64           `json:"gasUsed"`
	Timestamp        *big.Int         `json:"timestamp"`
	BaseFee          *big.Int         `json:"baseFeePerGas"` // EIP-1559
	ExcessBlobGas    uint64           `json:"excessBlobGas"` // EIP-4844
	BlobGasUsed      uint64           `json:"blobGasUsed"`   // EIP-4844
}

func (b *btHeader) UnmarshalJSON(input []byte) error {
	type btHeader struct {
		Bloom               *types.Bloom          `json:"bloom"`
		Coinbase            *common.Address       `json:"coinbase"` // name in blocktests
		CoinbaseAlt         *common.Address       `json:"author"`   // nethermind/parity/oe name
		CoinbaseAlt2        *common.Address       `json:"miner"`    // geth/besu name
		MixHash             *common.Hash          `json:"mixHash"`
		Nonce               *types.BlockNonce     `json:"nonce"`
		Number              *math.HexOrDecimal256 `json:"number"`
		Hash                *common.Hash          `json:"hash"`
		ParentHash          *common.Hash          `json:"parentHash"`
		ReceiptTrie         *common.Hash          `json:"receiptsRoot"` // name in std json
		ReceiptTrieAlt      *common.Hash          `json:"receiptTrie"`  // name in blocktests
		StateRoot           *common.Hash          `json:"stateRoot"`
		TransactionsTrie    *common.Hash          `json:"transactionsRoot"` // name in std json
		TransactionsTrieAlt *common.Hash          `json:"transactionsTrie"` // name in blocktests
		UncleHash           *common.Hash          `json:"sha3Uncles"`       // name in std json
		UncleHashAlt        *common.Hash          `json:"uncleHash"`        // name in blocktests
		ExtraData           *hexutil.Bytes        `json:"extraData"`
		Difficulty          *math.HexOrDecimal256 `json:"difficulty"`
		GasLimit            *math.HexOrDecimal64  `json:"gasLimit"`
		GasUsed             *math.HexOrDecimal64  `json:"gasUsed"`
		Timestamp           *math.HexOrDecimal256 `json:"timestamp"`
		BaseFee             *math.HexOrDecimal256 `json:"baseFeePerGas"`
		ExcessBlobGas       *math.HexOrDecimal64  `json:"excessBlobGas"`
		BlobGasUsed         *math.HexOrDecimal64  `json:"blobGasUsed"`
	}
	var dec btHeader
	if err := json.Unmarshal(input, &dec); err != nil {
		return err
	}
	if dec.Bloom != nil {
		b.Bloom = *dec.Bloom
	}
	if dec.Coinbase != nil {
		b.Coinbase = *dec.Coinbase
	} else if dec.CoinbaseAlt != nil {
		b.Coinbase = *dec.CoinbaseAlt
	} else if dec.CoinbaseAlt2 != nil {
		b.Coinbase = *dec.CoinbaseAlt2
	}
	if dec.MixHash != nil {
		b.MixHash = *dec.MixHash
	}
	if dec.Nonce != nil {
		b.Nonce = *dec.Nonce
	}
	if dec.Number != nil {
		b.Number = (*big.Int)(dec.Number)
	}
	if dec.Hash != nil {
		b.Hash = *dec.Hash
	}
	if dec.ParentHash != nil {
		b.ParentHash = *dec.ParentHash
	}
	if dec.ReceiptTrie != nil {
		b.ReceiptTrie = *dec.ReceiptTrie
	} else if dec.ReceiptTrieAlt != nil {
		b.ReceiptTrie = *dec.ReceiptTrieAlt
	}
	if dec.StateRoot != nil {
		b.StateRoot = *dec.StateRoot
	}
	if dec.TransactionsTrie != nil {
		b.TransactionsTrie = *dec.TransactionsTrie
	} else if dec.TransactionsTrieAlt != nil {
		b.TransactionsTrie = *dec.TransactionsTrieAlt
	}
	if dec.UncleHash != nil {
		b.UncleHash = *dec.UncleHash
	} else if dec.UncleHashAlt != nil {
		b.UncleHash = *dec.UncleHashAlt
	}
	if dec.ExtraData != nil {
		b.ExtraData = *dec.ExtraData
	}
	if dec.Difficulty != nil {
		b.Difficulty = (*big.Int)(dec.Difficulty)
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
	if dec.BaseFee != nil {
		b.BaseFee = (*big.Int)(dec.BaseFee)
	}
	if dec.ExcessBlobGas != nil {
		b.ExcessBlobGas = uint64(*dec.ExcessBlobGas)
	}
	if dec.BlobGasUsed != nil {
		b.BlobGasUsed = uint64(*dec.BlobGasUsed)
	}
	return nil
}
