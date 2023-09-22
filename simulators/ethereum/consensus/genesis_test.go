package main

import (
	"fmt"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

func TestGenesisParsing(_ *testing.T) {
	// Taken from a failing nethermind test
	genesisResponse := `{
  "author": "0x2adc25665018aa1fe0e6bc666dac8fc2697ff9ba",
  "baseFeePerGas": "0x3b9aca00",
  "difficulty": "0x20000",
  "extraData": "0x00",
  "gasLimit": "0x5f5e100",
  "gasUsed": "0x0",
  "hash": "0xa29ba2a76546b56f331f02e70b24c4a1452b1b392e1d2fe51828ff57f551d62e",
  "logsBloom": "0x00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000",
  "miner": "0x2adc25665018aa1fe0e6bc666dac8fc2697ff9ba",
  "mixHash": "0x0000000000000000000000000000000000000000000000000000000000000000",
  "nonce": "0x0000000000000000",
  "number": "0x0",
  "parentHash": "0x0000000000000000000000000000000000000000000000000000000000000000",
  "receiptsRoot": "0x56e81f171bcc55a6ff8345e692c0f86e5b48e01b996cadc001622fb5e363b421",
  "sha3Uncles": "0x1dcc4de8dec75d7aab85b567b6ccd41ad312451b948a7413f0a142fd40d49347",
  "size": "0x201",
  "stateRoot": "0xf3d3787e33cb7913a304f188002f59e7b7a1e1fe3a712988c7092a213f8c2e8f",
  "timestamp": "0x0",
  "totalDifficulty": "0x20000",
  "transactions": [],
  "transactionsRoot": "0x56e81f171bcc55a6ff8345e692c0f86e5b48e01b996cadc001622fb5e363b421",
  "uncles": []
}`
	// Taken from https://github.com/ethereum/tests/blob/develop/BlockchainTests/GeneralStateTests/VMTests/vmArithmeticTest/add.json
	expHeader := btHeader{
		Bloom:            types.Bloom{},
		Coinbase:         common.HexToAddress("0x2adc25665018aa1fe0e6bc666dac8fc2697ff9ba"),
		MixHash:          common.Hash{},
		Nonce:            types.BlockNonce{},
		Number:           new(big.Int),
		Hash:             common.HexToHash("0xafa8fc2deb658d8120f4011e459159a6472b88e2dfed6518640be01dbbfd20a9"),
		ParentHash:       common.Hash{},
		ReceiptTrie:      common.HexToHash("0x56e81f171bcc55a6ff8345e692c0f86e5b48e01b996cadc001622fb5e363b421"),
		StateRoot:        common.HexToHash("0xf3d3787e33cb7913a304f188002f59e7b7a1e1fe3a712988c7092a213f8c2e8f"),
		TransactionsTrie: common.HexToHash("0x56e81f171bcc55a6ff8345e692c0f86e5b48e01b996cadc001622fb5e363b421"),
		UncleHash:        common.HexToHash("0x1dcc4de8dec75d7aab85b567b6ccd41ad312451b948a7413f0a142fd40d49347"),
		ExtraData:        []byte{0x0},
		Difficulty:       big.NewInt(0x20000),
		GasLimit:         0x05f5e100,
		GasUsed:          0,
		Timestamp:        new(big.Int),
		BaseFee:          big.NewInt(11),
	}
	wantHash := common.HexToHash("0xafa8fc2deb658d8120f4011e459159a6472b88e2dfed6518640be01dbbfd20a9")
	gotHash := common.HexToHash("0xa29ba2a76546b56f331f02e70b24c4a1452b1b392e1d2fe51828ff57f551d62e")

	fmt.Printf("genesis hash mismatch:\n  want 0x%x\n   got 0x%x\n", wantHash, gotHash)
	if diffs, err := compareGenesis(genesisResponse, expHeader); err == nil {
		fmt.Printf(diffs)
	}
}
