package main

import (
	"crypto/ecdsa"
	"math/big"

	"github.com/ethereum/go-ethereum/core/types"
)

// API call names
const (
	EngineForkchoiceUpdatedV1 = "engine_forkchoiceUpdatedV1"
	EngineGetPayloadV1        = "engine_getPayloadV1"
	EngineNewPayloadV1        = "engine_newPayloadV1"
	EthGetBlockByHash         = "eth_getBlockByHash"
	EthGetBlockByNumber       = "eth_getBlockByNumber"
)

// Engine API Types

type PayloadStatus string

const (
	Unknown          = ""
	Valid            = "VALID"
	Invalid          = "INVALID"
	Accepted         = "ACCEPTED"
	Syncing          = "SYNCING"
	InvalidBlockHash = "INVALID_BLOCK_HASH"
)

// Signer for all txs
type Signer struct {
	ChainID    *big.Int
	PrivateKey *ecdsa.PrivateKey
}

func (vs Signer) SignTx(
	baseTx *types.Transaction,
) (*types.Transaction, error) {
	signer := types.NewEIP155Signer(vs.ChainID)
	return types.SignTx(baseTx, signer, vs.PrivateKey)
}

var VaultSigner = Signer{
	ChainID:    CHAIN_ID,
	PrivateKey: VAULT_KEY,
}
