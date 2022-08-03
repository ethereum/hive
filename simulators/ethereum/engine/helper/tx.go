package helper

import (
	"crypto/ecdsa"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/trie"
	"github.com/ethereum/hive/simulators/ethereum/engine/globals"
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
