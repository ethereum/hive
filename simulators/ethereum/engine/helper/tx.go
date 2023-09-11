package helper

import (
	"context"
	"crypto/ecdsa"
	"encoding/binary"
	"fmt"
	"math/big"
	"strings"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto/kzg4844"
	"github.com/ethereum/go-ethereum/trie"
	"github.com/ethereum/hive/simulators/ethereum/engine/client"
	"github.com/ethereum/hive/simulators/ethereum/engine/config"
	"github.com/ethereum/hive/simulators/ethereum/engine/config/cancun"
	"github.com/ethereum/hive/simulators/ethereum/engine/globals"
	typ "github.com/ethereum/hive/simulators/ethereum/engine/types"
	"github.com/holiman/uint256"
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

func CustomizeTransaction(baseTransaction *types.Transaction, sender SenderAccount, customData *CustomTransactionData) (*types.Transaction, error) {
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
	case types.BlobTxType:
		modifiedBlobTxBase := &types.BlobTx{}

		if customData.Nonce != nil {
			modifiedBlobTxBase.Nonce = *customData.Nonce
		} else {
			modifiedBlobTxBase.Nonce = baseTransaction.Nonce()
		}
		if customData.GasPriceOrGasFeeCap != nil {
			modifiedBlobTxBase.GasFeeCap = uint256.MustFromBig(customData.GasPriceOrGasFeeCap)
		} else {
			modifiedBlobTxBase.GasFeeCap = uint256.MustFromBig(baseTransaction.GasFeeCap())
		}
		if customData.GasTipCap != nil {
			modifiedBlobTxBase.GasTipCap = uint256.MustFromBig(customData.GasTipCap)
		} else {
			modifiedBlobTxBase.GasTipCap = uint256.MustFromBig(baseTransaction.GasTipCap())
		}
		if customData.Gas != nil {
			modifiedBlobTxBase.Gas = *customData.Gas
		} else {
			modifiedBlobTxBase.Gas = baseTransaction.Gas()
		}
		to := customData.To
		if to == nil {
			to = baseTransaction.To()
		}
		if to == nil {
			to = &common.Address{}
		}
		modifiedBlobTxBase.To = *to
		if customData.Value != nil {
			modifiedBlobTxBase.Value = uint256.MustFromBig(customData.Value)
		} else {
			modifiedBlobTxBase.Value = uint256.MustFromBig(baseTransaction.Value())
		}
		if customData.Data != nil {
			modifiedBlobTxBase.Data = *customData.Data
		} else {
			modifiedBlobTxBase.Data = baseTransaction.Data()
		}
		modifiedBlobTxBase.AccessList = baseTransaction.AccessList()
		modifiedBlobTxBase.BlobFeeCap = uint256.MustFromBig(baseTransaction.BlobGasFeeCap())
		modifiedBlobTxBase.BlobHashes = baseTransaction.BlobHashes()
		modifiedBlobTxBase.Sidecar = baseTransaction.BlobTxSidecar()
		if customData.Signature != nil {
			modifiedBlobTxBase.V = uint256.MustFromBig(customData.Signature.V)
			modifiedBlobTxBase.R = uint256.MustFromBig(customData.Signature.R)
			modifiedBlobTxBase.S = uint256.MustFromBig(customData.Signature.S)
		}

		modifiedTxData = modifiedBlobTxBase

	}

	modifiedTx := types.NewTx(modifiedTxData)
	if customData.Signature == nil {
		// If a custom invalid signature was not specified, simply sign the transaction again
		if customData.ChainID == nil {
			// Use the default value if an invaild chain ID was not specified
			customData.ChainID = globals.ChainID
		}
		signer := types.NewCancunSigner(customData.ChainID)
		var err error
		if modifiedTx, err = types.SignTx(modifiedTx, signer, sender.GetKey()); err != nil {
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
	BlobTxOnly                 TestTransactionType = "BlobTransactions"
)

type SenderAccount interface {
	GetKey() *ecdsa.PrivateKey
	GetAddress() common.Address
	GetIndex() uint64
}

type TransactionCreator interface {
	MakeTransaction(sender SenderAccount, nonce uint64, blockTimestamp uint64) (typ.Transaction, error)
}

type BaseTransactionCreator struct {
	Recipient  *common.Address
	GasFee     *big.Int
	GasTip     *big.Int
	GasLimit   uint64
	BlobGasFee *big.Int
	BlobCount  *big.Int
	Amount     *big.Int
	Payload    []byte
	AccessList types.AccessList
	TxType     TestTransactionType
	ForkConfig *config.ForkConfig
}

func (tc *BaseTransactionCreator) MakeTransaction(sender SenderAccount, nonce uint64, blockTimestamp uint64) (typ.Transaction, error) {
	var newTxData types.TxData

	// Determine the type of transaction to use
	var txTypeToUse int
	switch tc.TxType {
	case UnspecifiedTransactionType:
		// Test case has no specific type of transaction to use.
		// Select the type of tx based on the nonce and the account index.
		if tc.ForkConfig == nil {
			return nil, fmt.Errorf("fork config is nil")
		}
		forkSupportedTransactionTypes := tc.ForkConfig.GetSupportedTransactionTypes(blockTimestamp)
		txTypeToUse = forkSupportedTransactionTypes[int(sender.GetIndex()+nonce)%len(forkSupportedTransactionTypes)]
		if txTypeToUse == types.BlobTxType && tc.Recipient == nil {
			// Blob txs require a recipient, revert to legacy tx
			txTypeToUse = types.LegacyTxType
		}
	case LegacyTxOnly:
		txTypeToUse = types.LegacyTxType
	case DynamicFeeTxOnly:
		txTypeToUse = types.DynamicFeeTxType
	case BlobTxOnly:
		txTypeToUse = types.BlobTxType
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
		var (
			gasFeeCap *big.Int
			gasTipCap *big.Int
		)
		if tc.GasFee != nil {
			gasFeeCap = tc.GasFee
		} else {
			gasFeeCap = new(big.Int).Set(globals.GasPrice)
		}
		if tc.GasTip != nil {
			gasTipCap = tc.GasTip
		} else {
			gasTipCap = new(big.Int).Set(globals.GasTipPrice)
		}
		newTxData = &types.DynamicFeeTx{
			Nonce:     nonce,
			Gas:       tc.GasLimit,
			GasTipCap: gasTipCap,
			GasFeeCap: gasFeeCap,
			To:        tc.Recipient,
			Value:     tc.Amount,
			Data:      tc.Payload,
		}
	case types.BlobTxType:
		if tc.Recipient == nil {
			return nil, errors.New("nil to address for blob transaction")
		}
		var (
			to         = *tc.Recipient
			chainID    = uint256.MustFromBig(globals.ChainID)
			gasFeeCap  *uint256.Int
			gasTipCap  *uint256.Int
			value      *uint256.Int
			blobGasFee *uint256.Int
			blobCount  uint64
		)
		if tc.GasFee != nil {
			gasFeeCap = uint256.MustFromBig(tc.GasFee)
		} else {
			gasFeeCap = uint256.MustFromBig(globals.GasPrice)
		}
		if tc.GasTip != nil {
			gasTipCap = uint256.MustFromBig(tc.GasTip)
		} else {
			gasTipCap = uint256.MustFromBig(globals.GasTipPrice)
		}
		if tc.Amount != nil {
			value = uint256.MustFromBig(tc.Amount)
		}
		if tc.BlobGasFee != nil {
			blobGasFee = uint256.MustFromBig(tc.BlobGasFee)
		} else {
			blobGasFee = uint256.MustFromBig(globals.BlobGasPrice)
		}
		if tc.BlobCount != nil {
			blobCount = tc.BlobCount.Uint64()
		} else {
			blobCount = cancun.MAX_BLOBS_PER_BLOCK
		}

		// Need tx wrap data that will pass blob verification
		hashes, blobData, err := BlobDataGenerator(BlobID(nonce), blobCount)
		if err != nil {
			return nil, err
		}
		sidecar := &types.BlobTxSidecar{}
		if blobData != nil {
			sidecar.Blobs = make([]kzg4844.Blob, len(blobData.Blobs))
			sidecar.Commitments = make([]kzg4844.Commitment, len(blobData.Commitments))
			sidecar.Proofs = make([]kzg4844.Proof, len(blobData.Proofs))
			for i := range blobData.Blobs {
				sidecar.Blobs[i] = kzg4844.Blob(blobData.Blobs[i])
				sidecar.Commitments[i] = kzg4844.Commitment(blobData.Commitments[i])
				sidecar.Proofs[i] = kzg4844.Proof(blobData.Proofs[i])
			}
		}

		newTxData = &types.BlobTx{
			ChainID:    chainID,
			Nonce:      nonce,
			GasTipCap:  gasTipCap,
			GasFeeCap:  gasFeeCap,
			Gas:        tc.GasLimit,
			To:         to,
			Value:      value,
			Data:       tc.Payload,
			AccessList: tc.AccessList,
			BlobFeeCap: blobGasFee,
			BlobHashes: hashes,
			Sidecar:    sidecar,
		}
	}

	tx := types.NewTx(newTxData)
	key := sender.GetKey()
	return types.SignTx(tx, types.NewCancunSigner(globals.ChainID), key)
}

// Create a contract filled with zeros without going over the specified GasLimit
type BigContractTransactionCreator struct {
	BaseTransactionCreator
}

func (tc *BigContractTransactionCreator) MakeTransaction(sender SenderAccount, nonce uint64, blockTimestamp uint64) (typ.Transaction, error) {
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
	return tc.BaseTransactionCreator.MakeTransaction(sender, nonce, blockTimestamp)
}

// Create a tx with the specified initcode length (all zeros)
type BigInitcodeTransactionCreator struct {
	BaseTransactionCreator
	InitcodeLength int
	PadByte        uint8
	Initcode       []byte
}

func (tc *BigInitcodeTransactionCreator) MakeTransaction(sender SenderAccount, nonce uint64, blockTimestamp uint64) (typ.Transaction, error) {
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
	return tc.BaseTransactionCreator.MakeTransaction(sender, nonce, blockTimestamp)
}

// Determines if the error we got from sending the raw tx is because the client
// already knew the tx (might happen if we produced a re-org where the tx was
// unwind back into the txpool)
func SentTxAlreadyKnown(err error) bool {
	return strings.Contains(err.Error(), "already known") || strings.Contains(err.Error(), "already in the TxPool") ||
		strings.Contains(err.Error(), "AlreadyKnown")
}

type TransactionSender struct {
	Accounts         []SenderAccount
	nonceMap         map[common.Address]uint64
	nonceMapLock     sync.Mutex
	transactionsSent int
}

func NewTransactionSender(accounts []*globals.TestAccount, disableInternalNonceCounter bool) *TransactionSender {
	sender := &TransactionSender{
		Accounts: make([]SenderAccount, len(accounts)),
	}
	for i, account := range accounts {
		sender.Accounts[i] = account
	}
	if !disableInternalNonceCounter {
		sender.nonceMap = make(map[common.Address]uint64)
	}
	return sender
}

func (txSender *TransactionSender) GetNextNonce(ctx context.Context, node client.EngineClient, sender SenderAccount, header *types.Header) (uint64, error) {
	var err error
	nonce := uint64(0)
	if txSender.nonceMap != nil {
		txSender.nonceMapLock.Lock()
		defer txSender.nonceMapLock.Unlock()
		nonce = txSender.nonceMap[sender.GetAddress()]
		txSender.nonceMap[sender.GetAddress()] = nonce + 1
	} else {
		nonce, err = node.GetNextAccountNonce(ctx, sender.GetAddress(), header)
		if err != nil {
			return 0, err
		}
	}
	return nonce, nil
}

func (txSender *TransactionSender) GetLastNonce(ctx context.Context, node client.EngineClient, sender SenderAccount, header *types.Header) (uint64, error) {
	if txSender.nonceMap != nil {
		txSender.nonceMapLock.Lock()
		defer txSender.nonceMapLock.Unlock()
		return txSender.nonceMap[sender.GetAddress()], nil
	} else {
		return node.GetLastAccountNonce(ctx, sender.GetAddress(), header)
	}
}

func (txSender *TransactionSender) SendTransaction(testCtx context.Context, account SenderAccount, node client.EngineClient, txCreator TransactionCreator) (typ.Transaction, error) {
	header, err := node.HeaderByNumber(testCtx, nil)
	if err != nil {
		return nil, errors.Wrap(err, "error getting header")
	}
	nonce, err := txSender.GetNextNonce(testCtx, node, account, header)
	if err != nil {
		return nil, errors.Wrap(err, "error getting next account nonce")
	}
	tx, err := txCreator.MakeTransaction(account, nonce, header.Time)
	if err != nil {
		return nil, errors.Wrap(err, "error crafting transaction")
	}
	for {
		ctx, cancel := context.WithTimeout(testCtx, globals.RPCTimeout)
		defer cancel()
		err := node.SendTransaction(ctx, tx)
		if err == nil || SentTxAlreadyKnown(err) {
			break
		}
		select {
		case <-time.After(time.Second):
		case <-testCtx.Done():
			return nil, errors.Wrapf(testCtx.Err(), "timeout retrying SendTransaction, last error: %v", err)
		}
	}
	return tx, nil
}

func (txSender *TransactionSender) SendNextTransaction(testCtx context.Context, node client.EngineClient, txCreator TransactionCreator) (typ.Transaction, error) {
	nextAccount := txSender.Accounts[txSender.transactionsSent%len(txSender.Accounts)]
	tx, err := txSender.SendTransaction(testCtx, nextAccount, node, txCreator)
	if err != nil {
		return nil, err
	}
	txSender.transactionsSent += 1
	return tx, nil
}

func (txSender *TransactionSender) SendNextTransactions(testCtx context.Context, node client.EngineClient, txCreator TransactionCreator, txCount uint64) ([]typ.Transaction, error) {
	var err error
	header, err := node.HeaderByNumber(testCtx, nil)
	if err != nil {
		return nil, errors.Wrap(err, "error getting header")
	}

	txs := make([]typ.Transaction, txCount)

	wg := sync.WaitGroup{}
	wgDone := make(chan interface{})
	errs := make(chan error, txCount)
	for i := range txs {
		wg.Add(1)
		i := i
		sender := txSender.Accounts[txSender.transactionsSent%len(txSender.Accounts)]
		go func(sender SenderAccount) {
			defer wg.Done()
			nonce, err := txSender.GetNextNonce(testCtx, node, sender, header)
			if err != nil {
				errs <- errors.Wrap(err, "error getting next account nonce")
				return
			}

			txs[i], err = txCreator.MakeTransaction(sender, nonce, header.Time)
			if err != nil {
				errs <- errors.Wrap(err, "error crafting transaction")
			}
			for {
				ctx, cancel := context.WithTimeout(testCtx, globals.RPCTimeout)
				defer cancel()
				err := node.SendTransaction(ctx, txs[i])
				if err == nil {
					break
				} else if SentTxAlreadyKnown(err) {
					break
				}
				select {
				case <-time.After(time.Second):
				case <-testCtx.Done():
					errs <- errors.Wrapf(testCtx.Err(), "timeout retrying SendTransaction (%d), last error: %v", i, err)
					return
				}
			}
		}(sender)
		txSender.transactionsSent += 1
	}
	go func() {
		wg.Wait()
		close(wgDone)
	}()
	select {
	case <-wgDone:
		return txs, nil
	case err := <-errs:
		return nil, errors.Wrap(err, "error sending transaction")
	case <-testCtx.Done():
		return txs, errors.Wrapf(testCtx.Err(), "timeout waiting for SendTransactions")
	}
}

func (txSender *TransactionSender) SendNextTransactionsBatch(testCtx context.Context, node client.EngineClient, txCreator TransactionCreator, txCount uint64) ([]typ.Transaction, error) {
	var err error
	header, err := node.HeaderByNumber(testCtx, nil)
	if err != nil {
		return nil, errors.Wrap(err, "error getting header")
	}

	txs := make([]typ.Transaction, txCount)

	for i := range txs {
		nextAccount := txSender.Accounts[txSender.transactionsSent%len(txSender.Accounts)]
		nonce, err := txSender.GetNextNonce(testCtx, node, nextAccount, header)
		if err != nil {
			return nil, errors.Wrap(err, "error getting next account nonce")
		}
		txs[i], err = txCreator.MakeTransaction(nextAccount, nonce, header.Time)
		if err != nil {
			return nil, errors.Wrap(err, "error crafting transaction")
		}
		txSender.transactionsSent += 1
	}
	ctx, cancel := context.WithTimeout(testCtx, globals.RPCTimeout)
	defer cancel()
	errs := node.SendTransactions(ctx, txs...)
	for _, err := range errs {
		if err != nil && !SentTxAlreadyKnown(err) {
			return txs, errors.Wrap(err, "error on SendTransactions")
		}
	}
	return txs, nil
}

func (txSender *TransactionSender) ReplaceTransaction(testCtx context.Context, account SenderAccount, node client.EngineClient, txCreator TransactionCreator) (typ.Transaction, error) {
	header, err := node.HeaderByNumber(testCtx, nil)
	if err != nil {
		return nil, errors.Wrap(err, "error getting header")
	}
	nonce, err := txSender.GetLastNonce(testCtx, node, account, header)
	if err != nil {
		return nil, errors.Wrap(err, "error getting last account nonce")
	}
	tx, err := txCreator.MakeTransaction(account, nonce, header.Time)
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

func (txSender *TransactionSender) ReplaceLastTransaction(testCtx context.Context, node client.EngineClient, txCreator TransactionCreator) (typ.Transaction, error) {
	if txSender.transactionsSent == 0 {
		return nil, errors.New("no transactions sent yet")
	}
	lastSender := txSender.Accounts[(txSender.transactionsSent-1)%len(txSender.Accounts)]
	return txSender.ReplaceTransaction(testCtx, lastSender, node, txCreator)
}
