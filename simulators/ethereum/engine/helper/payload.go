package helper

import (
	"fmt"
	"math/big"
	"math/rand"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/trie"
	"github.com/ethereum/hive/simulators/ethereum/engine/globals"
	typ "github.com/ethereum/hive/simulators/ethereum/engine/types"
)

type PayloadCustomizer interface {
	CustomizePayload(basePayload *typ.ExecutableData, baseBeaconRoot *common.Hash) (modifiedPayload *typ.ExecutableData, modifiedBeaconRoot *common.Hash, err error)
}

type CustomPayloadData struct {
	ParentHash          *common.Hash
	FeeRecipient        *common.Address
	StateRoot           *common.Hash
	ReceiptsRoot        *common.Hash
	LogsBloom           *[]byte
	PrevRandao          *common.Hash
	Number              *uint64
	GasLimit            *uint64
	GasUsed             *uint64
	Timestamp           *uint64
	ExtraData           *[]byte
	BaseFeePerGas       *big.Int
	BlockHash           *common.Hash
	Transactions        *[][]byte
	Withdrawals         types.Withdrawals
	RemoveWithdrawals   bool
	BlobGasUsed         *uint64
	RemoveBlobGasUsed   bool
	ExcessBlobGas       *uint64
	RemoveExcessBlobGas bool
	BeaconRoot          *common.Hash
	RemoveBeaconRoot    bool
}

var _ PayloadCustomizer = (*CustomPayloadData)(nil)

// Construct a customized payload by taking an existing payload as base and mixing it CustomPayloadData
// BlockHash is calculated automatically.
func (customData *CustomPayloadData) CustomizePayload(basePayload *typ.ExecutableData, baseBeaconRoot *common.Hash) (*typ.ExecutableData, *common.Hash, error) {
	txs := basePayload.Transactions
	if customData.Transactions != nil {
		txs = *customData.Transactions
	}
	txsHash, err := calcTxsHash(txs)
	if err != nil {
		return nil, nil, err
	}
	fmt.Printf("txsHash: %v\n", txsHash)
	// Start by filling the header with the basePayload information
	customPayloadHeader := types.Header{
		ParentHash:  basePayload.ParentHash,
		UncleHash:   types.EmptyUncleHash, // Could be overwritten
		Coinbase:    basePayload.FeeRecipient,
		Root:        basePayload.StateRoot,
		TxHash:      txsHash,
		ReceiptHash: basePayload.ReceiptsRoot,
		Bloom:       types.BytesToBloom(basePayload.LogsBloom),
		Difficulty:  big.NewInt(0), // could be overwritten
		Number:      big.NewInt(int64(basePayload.Number)),
		GasLimit:    basePayload.GasLimit,
		GasUsed:     basePayload.GasUsed,
		Time:        basePayload.Timestamp,
		Extra:       basePayload.ExtraData,
		MixDigest:   basePayload.Random,
		Nonce:       types.BlockNonce{0}, // could be overwritten
		BaseFee:     basePayload.BaseFeePerGas,
	}
	// Overwrite custom information
	if customData.ParentHash != nil {
		customPayloadHeader.ParentHash = *customData.ParentHash
	}
	if customData.FeeRecipient != nil {
		customPayloadHeader.Coinbase = *customData.FeeRecipient
	}
	if customData.StateRoot != nil {
		customPayloadHeader.Root = *customData.StateRoot
	}
	if customData.ReceiptsRoot != nil {
		customPayloadHeader.ReceiptHash = *customData.ReceiptsRoot
	}
	if customData.LogsBloom != nil {
		customPayloadHeader.Bloom = types.BytesToBloom(*customData.LogsBloom)
	}
	if customData.PrevRandao != nil {
		customPayloadHeader.MixDigest = *customData.PrevRandao
	}
	if customData.Number != nil {
		customPayloadHeader.Number = big.NewInt(int64(*customData.Number))
	}
	if customData.GasLimit != nil {
		customPayloadHeader.GasLimit = *customData.GasLimit
	}
	if customData.GasUsed != nil {
		customPayloadHeader.GasUsed = *customData.GasUsed
	}
	if customData.Timestamp != nil {
		customPayloadHeader.Time = *customData.Timestamp
	}
	if customData.ExtraData != nil {
		customPayloadHeader.Extra = *customData.ExtraData
	}
	if customData.BaseFeePerGas != nil {
		customPayloadHeader.BaseFee = customData.BaseFeePerGas
	}
	if customData.RemoveWithdrawals {
		customPayloadHeader.WithdrawalsHash = nil
	} else if customData.Withdrawals != nil {
		h := types.DeriveSha(customData.Withdrawals, trie.NewStackTrie(nil))
		customPayloadHeader.WithdrawalsHash = &h
	} else if basePayload.Withdrawals != nil {
		h := types.DeriveSha(types.Withdrawals(basePayload.Withdrawals), trie.NewStackTrie(nil))
		customPayloadHeader.WithdrawalsHash = &h
	}
	if customData.RemoveBlobGasUsed {
		customPayloadHeader.BlobGasUsed = nil
	} else if customData.BlobGasUsed != nil {
		customPayloadHeader.BlobGasUsed = customData.BlobGasUsed
	} else if basePayload.BlobGasUsed != nil {
		customPayloadHeader.BlobGasUsed = basePayload.BlobGasUsed
	}
	if customData.RemoveExcessBlobGas {
		customPayloadHeader.ExcessBlobGas = nil
	} else if customData.ExcessBlobGas != nil {
		customPayloadHeader.ExcessBlobGas = customData.ExcessBlobGas
	} else if basePayload.ExcessBlobGas != nil {
		customPayloadHeader.ExcessBlobGas = basePayload.ExcessBlobGas
	}
	//if customData.RemoveBeaconRoot {
	//	customPayloadHeader.BeaconRoot = nil
	//} else if customData.BeaconRoot != nil {
	//	customPayloadHeader.BeaconRoot = customData.BeaconRoot
	//} else if baseBeaconRoot != nil {
	//	customPayloadHeader.BeaconRoot = baseBeaconRoot
	//}

	// Return the new payload
	result := &typ.ExecutableData{
		ParentHash:    customPayloadHeader.ParentHash,
		FeeRecipient:  customPayloadHeader.Coinbase,
		StateRoot:     customPayloadHeader.Root,
		ReceiptsRoot:  customPayloadHeader.ReceiptHash,
		LogsBloom:     customPayloadHeader.Bloom[:],
		Random:        customPayloadHeader.MixDigest,
		Number:        customPayloadHeader.Number.Uint64(),
		GasLimit:      customPayloadHeader.GasLimit,
		GasUsed:       customPayloadHeader.GasUsed,
		Timestamp:     customPayloadHeader.Time,
		ExtraData:     customPayloadHeader.Extra,
		BaseFeePerGas: customPayloadHeader.BaseFee,
		BlockHash:     customPayloadHeader.Hash(),
		Transactions:  txs,
		BlobGasUsed:   customPayloadHeader.BlobGasUsed,
		ExcessBlobGas: customPayloadHeader.ExcessBlobGas,
	}
	if customData.RemoveWithdrawals {
		result.Withdrawals = nil
	} else if customData.Withdrawals != nil {
		result.Withdrawals = customData.Withdrawals
	} else if basePayload.Withdrawals != nil {
		result.Withdrawals = basePayload.Withdrawals
	}
	//return result, customPayloadHeader.BeaconRoot, nil
	return result, nil, nil
}

func CustomizePayloadTransactions(basePayload *typ.ExecutableData, baseBeaconRoot *common.Hash, customTransactions types.Transactions) (*typ.ExecutableData, *common.Hash, error) {
	byteTxs := make([][]byte, 0)
	for _, tx := range customTransactions {
		bytes, err := tx.MarshalBinary()
		if err != nil {
			return nil, nil, err
		}
		byteTxs = append(byteTxs, bytes)
	}
	return (&CustomPayloadData{
		Transactions: &byteTxs,
	}).CustomizePayload(basePayload, baseBeaconRoot)
}

func (customData *CustomPayloadData) String() string {
	customFieldsList := make([]string, 0)
	if customData.ParentHash != nil {
		customFieldsList = append(customFieldsList, fmt.Sprintf("ParentHash=%s", customData.ParentHash.String()))
	}
	if customData.FeeRecipient != nil {
		customFieldsList = append(customFieldsList, fmt.Sprintf("Coinbase=%s", customData.FeeRecipient.String()))
	}
	if customData.StateRoot != nil {
		customFieldsList = append(customFieldsList, fmt.Sprintf("StateRoot=%s", customData.StateRoot.String()))
	}
	if customData.ReceiptsRoot != nil {
		customFieldsList = append(customFieldsList, fmt.Sprintf("ReceiptsRoot=%s", customData.ReceiptsRoot.String()))
	}
	if customData.LogsBloom != nil {
		customFieldsList = append(customFieldsList, fmt.Sprintf("LogsBloom=%v", types.BytesToBloom(*customData.LogsBloom)))
	}
	if customData.PrevRandao != nil {
		customFieldsList = append(customFieldsList, fmt.Sprintf("PrevRandao=%s", customData.PrevRandao.String()))
	}
	if customData.Number != nil {
		customFieldsList = append(customFieldsList, fmt.Sprintf("Number=%d", *customData.Number))
	}
	if customData.GasLimit != nil {
		customFieldsList = append(customFieldsList, fmt.Sprintf("GasLimit=%d", *customData.GasLimit))
	}
	if customData.GasUsed != nil {
		customFieldsList = append(customFieldsList, fmt.Sprintf("GasUsed=%d", *customData.GasUsed))
	}
	if customData.Timestamp != nil {
		customFieldsList = append(customFieldsList, fmt.Sprintf("Timestamp=%d", *customData.Timestamp))
	}
	if customData.ExtraData != nil {
		customFieldsList = append(customFieldsList, fmt.Sprintf("ExtraData=%v", *customData.ExtraData))
	}
	if customData.BaseFeePerGas != nil {
		customFieldsList = append(customFieldsList, fmt.Sprintf("BaseFeePerGas=%s", customData.BaseFeePerGas.String()))
	}
	if customData.Transactions != nil {
		customFieldsList = append(customFieldsList, fmt.Sprintf("Transactions=%v", customData.Transactions))
	}
	if customData.Withdrawals != nil {
		customFieldsList = append(customFieldsList, fmt.Sprintf("Withdrawals=%v", customData.Withdrawals))
	}
	return strings.Join(customFieldsList, ", ")
}

// This function generates an invalid payload by taking a base payload and modifying the specified field such that it ends up being invalid.
// One small consideration is that the payload needs to contain transactions and specially transactions using the PREVRANDAO opcode for all the fields to be compatible with this function.
func GenerateInvalidPayload(basePayload *typ.ExecutableData, baseBeaconRoot *common.Hash, payloadField InvalidPayloadBlockField) (*typ.ExecutableData, *common.Hash, error) {

	var customPayloadMod *CustomPayloadData
	switch payloadField {
	case InvalidParentHash:
		modParentHash := basePayload.ParentHash
		modParentHash[common.HashLength-1] = byte(255 - modParentHash[common.HashLength-1])
		customPayloadMod = &CustomPayloadData{
			ParentHash: &modParentHash,
		}
	case InvalidStateRoot:
		modStateRoot := basePayload.StateRoot
		modStateRoot[common.HashLength-1] = byte(255 - modStateRoot[common.HashLength-1])
		customPayloadMod = &CustomPayloadData{
			StateRoot: &modStateRoot,
		}
	case InvalidReceiptsRoot:
		modReceiptsRoot := basePayload.ReceiptsRoot
		modReceiptsRoot[common.HashLength-1] = byte(255 - modReceiptsRoot[common.HashLength-1])
		customPayloadMod = &CustomPayloadData{
			ReceiptsRoot: &modReceiptsRoot,
		}
	case InvalidNumber:
		modNumber := basePayload.Number - 1
		customPayloadMod = &CustomPayloadData{
			Number: &modNumber,
		}
	case InvalidGasLimit:
		modGasLimit := basePayload.GasLimit * 2
		customPayloadMod = &CustomPayloadData{
			GasLimit: &modGasLimit,
		}
	case InvalidGasUsed:
		modGasUsed := basePayload.GasUsed - 1
		customPayloadMod = &CustomPayloadData{
			GasUsed: &modGasUsed,
		}
	case InvalidTimestamp:
		modTimestamp := basePayload.Timestamp - 1
		customPayloadMod = &CustomPayloadData{
			Timestamp: &modTimestamp,
		}
	case InvalidPrevRandao:
		// This option potentially requires a transaction that uses the PREVRANDAO opcode.
		// Otherwise the payload will still be valid.
		modPrevRandao := common.Hash{}
		rand.Read(modPrevRandao[:])
		customPayloadMod = &CustomPayloadData{
			PrevRandao: &modPrevRandao,
		}
	case RemoveTransaction:
		emptyTxs := make([][]byte, 0)
		customPayloadMod = &CustomPayloadData{
			Transactions: &emptyTxs,
		}
	case InvalidTransactionSignature,
		InvalidTransactionNonce,
		InvalidTransactionGas,
		InvalidTransactionGasPrice,
		InvalidTransactionGasTipPrice,
		InvalidTransactionValue,
		InvalidTransactionChainID:

		if len(basePayload.Transactions) == 0 {
			return nil, nil, fmt.Errorf("no transactions available for modification")
		}
		var baseTx types.Transaction
		if err := baseTx.UnmarshalBinary(basePayload.Transactions[0]); err != nil {
			return nil, nil, err
		}
		var customTxData CustomTransactionData
		switch payloadField {
		case InvalidTransactionSignature:
			modifiedSignature := SignatureValuesFromRaw(baseTx.RawSignatureValues())
			modifiedSignature.R = modifiedSignature.R.Sub(modifiedSignature.R, common.Big1)
			customTxData.Signature = &modifiedSignature
		case InvalidTransactionNonce:
			customNonce := baseTx.Nonce() - 1
			customTxData.Nonce = &customNonce
		case InvalidTransactionGas:
			customGas := uint64(0)
			customTxData.Gas = &customGas
		case InvalidTransactionGasPrice:
			customTxData.GasPriceOrGasFeeCap = common.Big0
		case InvalidTransactionGasTipPrice:
			invalidGasTip := new(big.Int).Set(globals.GasTipPrice)
			invalidGasTip.Mul(invalidGasTip, big.NewInt(2))
			customTxData.GasTipCap = invalidGasTip
		case InvalidTransactionValue:
			// Vault account initially has 0x123450000000000000000, so this value should overflow
			customTxData.Value, _ = hexutil.DecodeBig("0x123450000000000000001")
		case InvalidTransactionChainID:
			customChainID := new(big.Int).Set(globals.ChainID)
			customChainID.Add(customChainID, common.Big1)
			customTxData.ChainID = customChainID
		}

		modifiedTx, err := customizeTransaction(&baseTx, globals.VaultKey, &customTxData)
		if err != nil {
			return nil, nil, err
		}

		modifiedTxBytes, err := modifiedTx.MarshalBinary()
		if err != nil {
			return nil, nil, err
		}
		modifiedTransactions := [][]byte{
			modifiedTxBytes,
		}
		customPayloadMod = &CustomPayloadData{
			Transactions: &modifiedTransactions,
		}
	}

	if customPayloadMod == nil {
		copyPayload := *basePayload
		return &copyPayload, nil, nil
	}

	return customPayloadMod.CustomizePayload(basePayload, baseBeaconRoot)
}

/*
	 Generates an alternative withdrawals list that contains the same
		amounts and accounts, but the order in the list is different, so
		stateRoot of the resulting payload should be the same.
*/
func RandomizeWithdrawalsOrder(src types.Withdrawals) types.Withdrawals {
	dest := make(types.Withdrawals, len(src))
	perm := rand.Perm(len(src))
	for i, v := range perm {
		dest[v] = src[i]
	}
	return dest
}
