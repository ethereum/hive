package helper

import (
	"context"
	"crypto/ecdsa"
	"encoding/binary"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/trie"
	"github.com/ethereum/hive/simulators/ethereum/engine/client"
	"github.com/ethereum/hive/simulators/ethereum/engine/globals"
	typ "github.com/ethereum/hive/simulators/ethereum/engine/types"
	"github.com/pkg/errors"
)

type SignatureValues struct {
	V *big.Int
	R *big.Int
	S *big.Int
}

func SignatureValuesFromRaw(v *big.Int, r *big.Int, s *big.Int) SignatureValues {
	return SignatureValues{
		V: v,
		R: r,
		S: s,
	}
}

type CustomTransactionData struct {
	Nonce               *uint64
	GasPriceOrGasFeeCap *big.Int
	GasTipCap           *big.Int
	Gas                 *uint64
	To                  *common.Address
	Value               *big.Int
	Data                *[]byte
	ChainID             *big.Int
	Signature           *SignatureValues
}

func customizeTransaction(baseTransaction *types.Transaction, pk *ecdsa.PrivateKey, customData *CustomTransactionData) (*types.Transaction, error) {
	// Create a modified transaction base, from the base transaction and customData mix
	var (
		modifiedTxData types.TxData
	)

	switch baseTransaction.Type() {
	case types.LegacyTxType:
		modifiedLegacyTxBase := &types.LegacyTx{}

		if customData.Nonce != nil {
			modifiedLegacyTxBase.Nonce = *customData.Nonce
		} else {
			modifiedLegacyTxBase.Nonce = baseTransaction.Nonce()
		}
		if customData.GasPriceOrGasFeeCap != nil {
			modifiedLegacyTxBase.GasPrice = customData.GasPriceOrGasFeeCap
		} else {
			modifiedLegacyTxBase.GasPrice = baseTransaction.GasPrice()
		}
		if customData.GasTipCap != nil {
			return nil, fmt.Errorf("GasTipCap is not supported for LegacyTx type")
		}
		if customData.Gas != nil {
			modifiedLegacyTxBase.Gas = *customData.Gas
		} else {
			modifiedLegacyTxBase.Gas = baseTransaction.Gas()
		}
		if customData.To != nil {
			modifiedLegacyTxBase.To = customData.To
		} else {
			modifiedLegacyTxBase.To = baseTransaction.To()
		}
		if customData.Value != nil {
			modifiedLegacyTxBase.Value = customData.Value
		} else {
			modifiedLegacyTxBase.Value = baseTransaction.Value()
		}
		if customData.Data != nil {
			modifiedLegacyTxBase.Data = *customData.Data
		} else {
			modifiedLegacyTxBase.Data = baseTransaction.Data()
		}

		if customData.Signature != nil {
			modifiedLegacyTxBase.V = customData.Signature.V
			modifiedLegacyTxBase.R = customData.Signature.R
			modifiedLegacyTxBase.S = customData.Signature.S
		}

		modifiedTxData = modifiedLegacyTxBase

	case types.DynamicFeeTxType:
		modifiedDynamicFeeTxBase := &types.DynamicFeeTx{}

		if customData.Nonce != nil {
			modifiedDynamicFeeTxBase.Nonce = *customData.Nonce
		} else {
			modifiedDynamicFeeTxBase.Nonce = baseTransaction.Nonce()
		}
		if customData.GasPriceOrGasFeeCap != nil {
			modifiedDynamicFeeTxBase.GasFeeCap = customData.GasPriceOrGasFeeCap
		} else {
			modifiedDynamicFeeTxBase.GasFeeCap = baseTransaction.GasFeeCap()
		}
		if customData.GasTipCap != nil {
			modifiedDynamicFeeTxBase.GasTipCap = customData.GasTipCap
		} else {
			modifiedDynamicFeeTxBase.GasTipCap = baseTransaction.GasTipCap()
		}
		if customData.Gas != nil {
			modifiedDynamicFeeTxBase.Gas = *customData.Gas
		} else {
			modifiedDynamicFeeTxBase.Gas = baseTransaction.Gas()
		}
		if customData.To != nil {
			modifiedDynamicFeeTxBase.To = customData.To
		} else {
			modifiedDynamicFeeTxBase.To = baseTransaction.To()
		}
		if customData.Value != nil {
			modifiedDynamicFeeTxBase.Value = customData.Value
		} else {
			modifiedDynamicFeeTxBase.Value = baseTransaction.Value()
		}
		if customData.Data != nil {
			modifiedDynamicFeeTxBase.Data = *customData.Data
		} else {
			modifiedDynamicFeeTxBase.Data = baseTransaction.Data()
		}
		if customData.Signature != nil {
			modifiedDynamicFeeTxBase.V = customData.Signature.V
			modifiedDynamicFeeTxBase.R = customData.Signature.R
			modifiedDynamicFeeTxBase.S = customData.Signature.S
		}

		modifiedTxData = modifiedDynamicFeeTxBase

	}

	modifiedTx := types.NewTx(modifiedTxData)
	if customData.Signature == nil {
		// If a custom invalid signature was not specified, simply sign the transaction again
		if customData.ChainID == nil {
			// Use the default value if an invaild chain ID was not specified
			customData.ChainID = globals.ChainID
		}
		signer := types.NewLondonSigner(customData.ChainID)
		var err error
		if modifiedTx, err = types.SignTx(modifiedTx, signer, pk); err != nil {
			return nil, err
		}
	}
	return modifiedTx, nil
}

func calcTxsHash(txsBytes [][]byte) (common.Hash, error) {
	txs := make([]*types.Transaction, len(txsBytes))
	for i, bytesTx := range txsBytes {
		var currentTx types.Transaction
		err := currentTx.UnmarshalBinary(bytesTx)
		if err != nil {
			return common.Hash{}, err
		}
		txs[i] = &currentTx
	}
	return types.DeriveSha(types.Transactions(txs), trie.NewStackTrie(nil)), nil
}

type TestTransactionType string

const (
	UnspecifiedTransactionType TestTransactionType = ""
	LegacyTxOnly               TestTransactionType = "LegacyTransactions"
	DynamicFeeTxOnly           TestTransactionType = "DynamicFeeTransactions"
)

type TransactionCreator interface {
	MakeTransaction(nonce uint64) (typ.Transaction, error)
	GetSourceAddress() common.Address
}

type BaseTransactionCreator struct {
	Recipient  *common.Address
	GasLimit   uint64
	Amount     *big.Int
	Payload    []byte
	TxType     TestTransactionType
	PrivateKey *ecdsa.PrivateKey
}

func (tc *BaseTransactionCreator) GetSourceAddress() common.Address {
	if tc.PrivateKey == nil {
		return globals.VaultAccountAddress
	}
	return crypto.PubkeyToAddress(tc.PrivateKey.PublicKey)
}

func (tc *BaseTransactionCreator) MakeTransaction(nonce uint64) (typ.Transaction, error) {
	var newTxData types.TxData

	var txTypeToUse int
	switch tc.TxType {
	case UnspecifiedTransactionType:
		// Test case has no specific type of transaction to use.
		// Select the type of tx based on the nonce.
		switch nonce % 2 {
		case 0:
			txTypeToUse = types.LegacyTxType
		case 1:
			txTypeToUse = types.DynamicFeeTxType
		}
	case LegacyTxOnly:
		txTypeToUse = types.LegacyTxType
	case DynamicFeeTxOnly:
		txTypeToUse = types.DynamicFeeTxType
	}

	// Build the transaction depending on the specified type
	switch txTypeToUse {
	case types.LegacyTxType:
		newTxData = &types.LegacyTx{
			Nonce:    nonce,
			To:       tc.Recipient,
			Value:    tc.Amount,
			Gas:      tc.GasLimit,
			GasPrice: globals.GasPrice,
			Data:     tc.Payload,
		}
	case types.DynamicFeeTxType:
		gasFeeCap := new(big.Int).Set(globals.GasPrice)
		gasTipCap := new(big.Int).Set(globals.GasTipPrice)
		newTxData = &types.DynamicFeeTx{
			Nonce:     nonce,
			Gas:       tc.GasLimit,
			GasTipCap: gasTipCap,
			GasFeeCap: gasFeeCap,
			To:        tc.Recipient,
			Value:     tc.Amount,
			Data:      tc.Payload,
		}
	}

	tx := types.NewTx(newTxData)
	key := tc.PrivateKey
	if key == nil {
		key = globals.VaultKey
	}
	return types.SignTx(tx, types.NewLondonSigner(globals.ChainID), key)
}

// Create a contract filled with zeros without going over the specified GasLimit
type BigContractTransactionCreator struct {
	BaseTransactionCreator
}

func (tc *BigContractTransactionCreator) MakeTransaction(nonce uint64) (typ.Transaction, error) {
	// Total GAS: Gtransaction == 21000, Gcreate == 32000, Gcodedeposit == 200
	contractLength := uint64(0)
	if tc.GasLimit > (21000 + 32000) {
		contractLength = (tc.GasLimit - 21000 - 32000) / 200
		if contractLength >= 1 {
			// Reduce by 1 to guarantee using less gas than requested
			contractLength -= 1
		}
	}
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, contractLength)

	tc.Payload = []byte{
		0x67, // PUSH8
	}
	tc.Payload = append(tc.Payload, buf...) // Size of the contract in byte length
	tc.Payload = append(tc.Payload, 0x38)   // CODESIZE == 0x00
	tc.Payload = append(tc.Payload, 0xF3)   // RETURN(offset, length)
	if tc.Recipient != nil {
		panic("invalid configuration for big contract tx creator")
	}
	return tc.BaseTransactionCreator.MakeTransaction(nonce)
}

// Create a tx with the specified initcode length (all zeros)
type BigInitcodeTransactionCreator struct {
	BaseTransactionCreator
	InitcodeLength int
	PadByte        uint8
	Initcode       []byte
}

func (tc *BigInitcodeTransactionCreator) MakeTransaction(nonce uint64) (typ.Transaction, error) {
	// This method caches the payload with the crafted initcode after first execution.
	if tc.Payload == nil {
		// Prepare initcode payload
		if tc.Initcode != nil {
			if len(tc.Initcode) > tc.InitcodeLength {
				panic(fmt.Errorf("invalid initcode (too big)"))
			}
			tc.Payload = tc.Initcode
		} else {
			tc.Payload = []byte{}
		}

		for {
			if len(tc.Payload) == tc.InitcodeLength {
				break
			}
			tc.Payload = append(tc.Payload, tc.PadByte)
		}
	}
	if tc.Recipient != nil {
		panic("invalid configuration for big contract tx creator")
	}
	return tc.BaseTransactionCreator.MakeTransaction(nonce)
}

// Determines if the error we got from sending the raw tx is because the client
// already knew the tx (might happen if we produced a re-org where the tx was
// unwind back into the txpool)
func SentTxAlreadyKnown(err error) bool {
	return strings.Contains(err.Error(), "already known") || strings.Contains(err.Error(), "already in the TxPool") ||
		strings.Contains(err.Error(), "AlreadyKnown")
}

func SendNextTransaction(testCtx context.Context, node client.EngineClient, txCreator TransactionCreator) (typ.Transaction, error) {
	nonce, err := node.GetNextAccountNonce(testCtx, txCreator.GetSourceAddress())
	if err != nil {
		return nil, errors.Wrap(err, "error getting next account nonce")
	}
	tx, err := txCreator.MakeTransaction(nonce)
	if err != nil {
		return nil, errors.Wrap(err, "error crafting transaction")
	}
	for {
		ctx, cancel := context.WithTimeout(testCtx, globals.RPCTimeout)
		defer cancel()
		err := node.SendTransaction(ctx, tx)
		if err == nil {
			return tx, nil
		} else if SentTxAlreadyKnown(err) {
			return tx, nil
		}
		select {
		case <-time.After(time.Second):
		case <-testCtx.Done():
			return nil, errors.Wrapf(testCtx.Err(), "timeout retrying SendTransaction, last error: %v", err)
		}
	}
}

func SendNextTransactions(testCtx context.Context, node client.EngineClient, txCreator TransactionCreator, txCount uint64) ([]typ.Transaction, error) {
	var err error
	nonce, err := node.GetNextAccountNonce(testCtx, txCreator.GetSourceAddress())
	if err != nil {
		return nil, errors.Wrap(err, "error getting next account nonce")
	}
	txs := make([]typ.Transaction, txCount)
	for i := range txs {
		txs[i], err = txCreator.MakeTransaction(nonce)
		if err != nil {
			return nil, errors.Wrap(err, "error crafting transaction")
		}
		nonce++
	}
	ctx, cancel := context.WithTimeout(testCtx, globals.RPCTimeout)
	defer cancel()
	errs := node.SendTransactions(ctx, txs...)
	for _, err := range errs {
		if err != nil && !SentTxAlreadyKnown(err) {
			return txs, errors.Wrap(err, "error on SendTransactions")
		}
	}
	node.UpdateNonce(testCtx, txCreator.GetSourceAddress(), nonce+txCount)
	return txs, nil
}

func ReplaceLastTransaction(testCtx context.Context, node client.EngineClient, txCreator TransactionCreator) (typ.Transaction, error) {
	nonce, err := node.GetLastAccountNonce(testCtx, txCreator.GetSourceAddress())
	if err != nil {
		return nil, errors.Wrap(err, "error getting last account nonce")
	}
	tx, err := txCreator.MakeTransaction(nonce)
	if err != nil {
		return nil, errors.Wrap(err, "error crafting transaction")
	}
	for {
		ctx, cancel := context.WithTimeout(testCtx, globals.RPCTimeout)
		defer cancel()
		err := node.SendTransaction(ctx, tx)
		if err == nil {
			return tx, nil
		} else if SentTxAlreadyKnown(err) {
			return tx, nil
		}
		select {
		case <-time.After(time.Second):
		case <-testCtx.Done():
			return nil, errors.Wrapf(testCtx.Err(), "timeout retrying ReplaceLastTransaction, last error: %v", err)
		}
	}
}
