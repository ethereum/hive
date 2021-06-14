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
	Bloom            types.Bloom
	Coinbase         common.Address
	MixHash          common.Hash
	Nonce            types.BlockNonce
	Number           *big.Int
	Hash             common.Hash
	ParentHash       common.Hash
	ReceiptTrie      common.Hash
	StateRoot        common.Hash
	TransactionsTrie common.Hash
	UncleHash        common.Hash
	ExtraData        []byte
	//	ExtraData        hexutil.Bytes
	Difficulty *big.Int
	GasLimit   uint64
	GasUsed    uint64
	Timestamp  *big.Int
	BaseFee    *big.Int `json:"baseFeePerGas"`
}

func (b *btHeader) UnmarshalJSON(input []byte) error {
	type btHeader struct {
		Bloom            *types.Bloom
		Coinbase         *common.Address
		MixHash          *common.Hash
		Nonce            *types.BlockNonce
		Number           *math.HexOrDecimal256
		Hash             *common.Hash
		ParentHash       *common.Hash
		ReceiptTrie      *common.Hash
		StateRoot        *common.Hash
		TransactionsTrie *common.Hash
		UncleHash        *common.Hash
		ExtraData        *hexutil.Bytes
		Difficulty       *math.HexOrDecimal256
		GasLimit         *math.HexOrDecimal64
		GasUsed          *math.HexOrDecimal64
		Timestamp        *math.HexOrDecimal256
		BaseFee          *math.HexOrDecimal256 `json:"baseFeePerGas"`
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
	}
	if dec.StateRoot != nil {
		b.StateRoot = *dec.StateRoot
	}
	if dec.TransactionsTrie != nil {
		b.TransactionsTrie = *dec.TransactionsTrie
	}
	if dec.UncleHash != nil {
		b.UncleHash = *dec.UncleHash
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
	return nil
}
