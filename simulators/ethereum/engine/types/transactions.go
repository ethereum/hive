package types

import (
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/holiman/uint256"
)

// Transaction interface
type Transaction interface {
	BinaryMarshable
	Protected() bool
	Type() uint8
	ChainId() *big.Int
	Data() []byte
	AccessList() types.AccessList
	Gas() uint64
	GasPrice() *big.Int
	GasTipCap() *big.Int
	GasFeeCap() *big.Int
	Value() *big.Int
	Nonce() uint64
	To() *common.Address

	Hash() common.Hash

	// Blob stuff
	BlobGas() uint64
	BlobGasFeeCap() *big.Int
	BlobHashes() []common.Hash
}

// Geth's transaction type must implement the interface
var _ Transaction = (*types.Transaction)(nil)

type TransactionWithBlobData struct {
	Tx             *types.Transaction
	BlobData       *BlobTxWrapData
	MarshalMinimal bool
}

func (tx *TransactionWithBlobData) Protected() bool {
	return tx.Tx.Protected()
}

func (tx *TransactionWithBlobData) Type() uint8 {
	return tx.Tx.Type()
}

func (tx *TransactionWithBlobData) ChainId() *big.Int {
	return tx.Tx.ChainId()
}

func (tx *TransactionWithBlobData) Data() []byte {
	return tx.Tx.Data()
}

func (tx *TransactionWithBlobData) AccessList() types.AccessList {
	return tx.Tx.AccessList()
}

func (tx *TransactionWithBlobData) Gas() uint64 {
	return tx.Tx.Gas()
}

func (tx *TransactionWithBlobData) GasPrice() *big.Int {
	return tx.Tx.GasPrice()
}

func (tx *TransactionWithBlobData) GasTipCap() *big.Int {
	return tx.Tx.GasTipCap()
}

func (tx *TransactionWithBlobData) GasFeeCap() *big.Int {
	return tx.Tx.GasFeeCap()
}

func (tx *TransactionWithBlobData) Value() *big.Int {
	return tx.Tx.Value()
}

func (tx *TransactionWithBlobData) Nonce() uint64 {
	return tx.Tx.Nonce()
}

func (tx *TransactionWithBlobData) To() *common.Address {
	return tx.Tx.To()
}

func (tx *TransactionWithBlobData) Hash() common.Hash {
	return tx.Tx.Hash()
}

func (tx *TransactionWithBlobData) BlobGas() uint64 {
	return tx.Tx.BlobGas()
}

func (tx *TransactionWithBlobData) BlobGasFeeCap() *big.Int {
	return tx.Tx.BlobGasFeeCap()
}

func (tx *TransactionWithBlobData) BlobHashes() []common.Hash {
	return tx.Tx.BlobHashes()
}

func (tx *TransactionWithBlobData) MarshalBinary() ([]byte, error) {

	if tx.BlobData == nil || tx.MarshalMinimal {
		return tx.Tx.MarshalBinary()
	}

	type MarshalType struct {
		TxPayload   types.BlobTx
		Blobs       []Blob
		Commitments []KZGCommitment
		Proofs      []KZGProof
	}

	pTo := tx.Tx.To()
	if pTo == nil {
		return nil, fmt.Errorf("to address is nil")
	}
	to := *pTo

	v, r, s := tx.Tx.RawSignatureValues()

	marshalBlobTx := MarshalType{
		TxPayload: types.BlobTx{
			ChainID:    uint256.MustFromBig(tx.Tx.ChainId()),
			Nonce:      tx.Tx.Nonce(),
			GasTipCap:  uint256.MustFromBig(tx.Tx.GasTipCap()),
			GasFeeCap:  uint256.MustFromBig(tx.Tx.GasFeeCap()),
			Gas:        tx.Tx.Gas(),
			To:         to,
			Value:      uint256.MustFromBig(tx.Tx.Value()),
			Data:       tx.Tx.Data(),
			AccessList: tx.Tx.AccessList(),
			BlobFeeCap: uint256.MustFromBig(tx.Tx.BlobGasFeeCap()),
			BlobHashes: tx.Tx.BlobHashes(),

			// Signature values
			V: uint256.MustFromBig(v),
			R: uint256.MustFromBig(r),
			S: uint256.MustFromBig(s),
		},
		Blobs:       tx.BlobData.Blobs,
		Commitments: tx.BlobData.Commitments,
		Proofs:      tx.BlobData.Proofs,
	}
	payloadBytes, err := rlp.EncodeToBytes(marshalBlobTx)
	if err != nil {
		return nil, err
	}
	return append([]byte{tx.Tx.Type()}, payloadBytes...), nil
}

// Transaction with blob data must also implement the interface
var _ Transaction = (*TransactionWithBlobData)(nil)
