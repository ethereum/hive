package helper

import (
	"fmt"
	"math/big"
	"math/rand"
	"strings"

	api "github.com/ethereum/go-ethereum/beacon/engine"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/trie"
	"github.com/ethereum/hive/simulators/ethereum/engine/globals"
	typ "github.com/ethereum/hive/simulators/ethereum/engine/types"
)

type PayloadAttributesCustomizer interface {
	GetPayloadAttributes(basePayloadAttributes *typ.PayloadAttributes) (*typ.PayloadAttributes, error)
}

type ForkchoiceUpdatedCustomizer interface {
	PayloadAttributesCustomizer
	GetForkchoiceState(baseForkchoiceUpdate api.ForkchoiceStateV1) (api.ForkchoiceStateV1, error)
	GetVersion(forkConfig *globals.ForkConfig, headTimestamp uint64, basePayloadAttributes *typ.PayloadAttributes) (int, error)
	GetExpectedError() (*int, error)
	GetExpectInvalidStatus() bool
}

type BasePayloadAttributesCustomizer struct {
	Timestamp             *uint64
	Random                *common.Hash
	SuggestedFeeRecipient *common.Address
	Withdrawals           *[]*types.Withdrawal
	RemoveWithdrawals     bool
	BeaconRoot            *common.Hash
	RemoveBeaconRoot      bool
}

var _ PayloadAttributesCustomizer = (*BasePayloadAttributesCustomizer)(nil)

func (customData *BasePayloadAttributesCustomizer) GetPayloadAttributes(basePayloadAttributes *typ.PayloadAttributes) (*typ.PayloadAttributes, error) {
	customPayloadAttributes := &typ.PayloadAttributes{
		Timestamp:             basePayloadAttributes.Timestamp,
		Random:                basePayloadAttributes.Random,
		SuggestedFeeRecipient: basePayloadAttributes.SuggestedFeeRecipient,
		Withdrawals:           basePayloadAttributes.Withdrawals,
		BeaconRoot:            basePayloadAttributes.BeaconRoot,
	}
	if customData.Timestamp != nil {
		customPayloadAttributes.Timestamp = *customData.Timestamp
	}
	if customData.Random != nil {
		customPayloadAttributes.Random = *customData.Random
	}
	if customData.SuggestedFeeRecipient != nil {
		customPayloadAttributes.SuggestedFeeRecipient = *customData.SuggestedFeeRecipient
	}
	if customData.RemoveWithdrawals {
		customPayloadAttributes.Withdrawals = nil
	} else if customData.Withdrawals != nil {
		customPayloadAttributes.Withdrawals = *customData.Withdrawals
	}
	if customData.RemoveBeaconRoot {
		customPayloadAttributes.BeaconRoot = nil
	} else if customData.BeaconRoot != nil {
		customPayloadAttributes.BeaconRoot = customData.BeaconRoot
	}
	return customPayloadAttributes, nil
}

// Customizer that makes no modifications to the forkchoice directive call.
// Used as base to other customizers.
type BaseForkchoiceUpdatedCustomizer struct {
	PayloadAttributesCustomizer
	ExpectedError       *int
	ExpectInvalidStatus bool
}

func (customizer *BaseForkchoiceUpdatedCustomizer) GetPayloadAttributes(basePayloadAttributes *typ.PayloadAttributes) (*typ.PayloadAttributes, error) {
	if customizer.PayloadAttributesCustomizer != nil {
		return customizer.PayloadAttributesCustomizer.GetPayloadAttributes(basePayloadAttributes)
	}
	return basePayloadAttributes, nil
}

func (customizer *BaseForkchoiceUpdatedCustomizer) GetForkchoiceState(baseForkchoiceUpdate api.ForkchoiceStateV1) (api.ForkchoiceStateV1, error) {
	return baseForkchoiceUpdate, nil
}

func (customizer *BaseForkchoiceUpdatedCustomizer) GetVersion(forkConfig *globals.ForkConfig, headTimestamp uint64, basePayloadAttributes *typ.PayloadAttributes) (int, error) {
	// We use the version meant for the head block unless there are payload attributes
	// in which case we use the version meant for the new payload requested.
	var payloadAttributesTimestamp *uint64
	if basePayloadAttributes != nil {
		payloadAttributesTimestamp = &basePayloadAttributes.Timestamp
	}
	return forkConfig.ForkchoiceUpdatedVersion(headTimestamp, payloadAttributesTimestamp), nil
}

func (customizer *BaseForkchoiceUpdatedCustomizer) GetExpectedError() (*int, error) {
	return customizer.ExpectedError, nil
}

func (customizer *BaseForkchoiceUpdatedCustomizer) GetExpectInvalidStatus() bool {
	return customizer.ExpectInvalidStatus
}

var _ ForkchoiceUpdatedCustomizer = (*BaseForkchoiceUpdatedCustomizer)(nil)

// Customizer that upgrades the version of the forkchoice directive call to the next version.
type UpgradeForkchoiceUpdatedVersion struct {
	ForkchoiceUpdatedCustomizer
}

var _ ForkchoiceUpdatedCustomizer = (*UpgradeForkchoiceUpdatedVersion)(nil)

func (customizer *UpgradeForkchoiceUpdatedVersion) GetVersion(forkConfig *globals.ForkConfig, headTimestamp uint64, basePayloadAttributes *typ.PayloadAttributes) (int, error) {
	if customizer.ForkchoiceUpdatedCustomizer == nil {
		return 0, fmt.Errorf("base customizer not set")
	}
	baseVersion, err := customizer.ForkchoiceUpdatedCustomizer.GetVersion(forkConfig, headTimestamp, basePayloadAttributes)
	if err != nil {
		return 0, err
	}
	return baseVersion + 1, nil
}

// Customizer that downgrades the version of the forkchoice directive call to the previous version.
type DowngradeForkchoiceUpdatedVersion struct {
	ForkchoiceUpdatedCustomizer
}

var _ ForkchoiceUpdatedCustomizer = (*DowngradeForkchoiceUpdatedVersion)(nil)

func (customizer *DowngradeForkchoiceUpdatedVersion) GetVersion(forkConfig *globals.ForkConfig, headTimestamp uint64, basePayloadAttributes *typ.PayloadAttributes) (int, error) {
	if customizer.ForkchoiceUpdatedCustomizer == nil {
		return 0, fmt.Errorf("base customizer not set")
	}
	baseVersion, err := customizer.ForkchoiceUpdatedCustomizer.GetVersion(forkConfig, headTimestamp, basePayloadAttributes)
	if err != nil {
		return 0, err
	}
	if baseVersion == 1 {
		return 1, fmt.Errorf("cannot downgrade version 1")
	}
	return baseVersion - 1, nil
}

type PayloadCustomizer interface {
	CustomizePayload(basePayload *typ.ExecutableData, baseBeaconRoot *common.Hash) (modifiedPayload *typ.ExecutableData, modifiedBeaconRoot *common.Hash, err error)
	GetTimestamp(basePayload *typ.ExecutableData) (uint64, error)
}

type VersionedHashesCustomizer interface {
	GetVersionedHashes(baseVesionedHashes *[]common.Hash) (*[]common.Hash, error)
}

type NewPayloadCustomizer interface {
	PayloadCustomizer
	VersionedHashesCustomizer
	GetVersion(forkConfig *globals.ForkConfig, basePayload *typ.ExecutableData) (int, error)
	GetExpectedError() (*int, error)
	GetExpectInvalidStatus() bool
}

type CustomPayloadData struct {
	ParentHash             *common.Hash
	FeeRecipient           *common.Address
	StateRoot              *common.Hash
	ReceiptsRoot           *common.Hash
	LogsBloom              *[]byte
	PrevRandao             *common.Hash
	Number                 *uint64
	GasLimit               *uint64
	GasUsed                *uint64
	Timestamp              *uint64
	ExtraData              *[]byte
	BaseFeePerGas          *big.Int
	BlockHash              *common.Hash
	Transactions           *[][]byte
	Withdrawals            types.Withdrawals
	RemoveWithdrawals      bool
	BlobGasUsed            *uint64
	RemoveBlobGasUsed      bool
	ExcessBlobGas          *uint64
	RemoveExcessBlobGas    bool
	ParentBeaconRoot       *common.Hash
	RemoveParentBeaconRoot bool
}

var _ PayloadCustomizer = (*CustomPayloadData)(nil)

func (customData *CustomPayloadData) GetTimestamp(basePayload *typ.ExecutableData) (uint64, error) {
	if customData.Timestamp != nil {
		return *customData.Timestamp, nil
	}
	return basePayload.Timestamp, nil
}

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
	if customData.RemoveParentBeaconRoot {
		customPayloadHeader.ParentBeaconRoot = nil
	} else if customData.ParentBeaconRoot != nil {
		customPayloadHeader.ParentBeaconRoot = customData.ParentBeaconRoot
	} else if baseBeaconRoot != nil {
		customPayloadHeader.ParentBeaconRoot = baseBeaconRoot
	}

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
	return result, customPayloadHeader.ParentBeaconRoot, nil
}

// Base new payload directive call customizer.
// Used as base to other customizers.
type BaseNewPayloadVersionCustomizer struct {
	PayloadCustomizer
	VersionedHashesCustomizer
	ExpectedError       *int
	ExpectInvalidStatus bool
}

var _ NewPayloadCustomizer = (*BaseNewPayloadVersionCustomizer)(nil)

func (customNewPayload *BaseNewPayloadVersionCustomizer) CustomizePayload(basePayload *typ.ExecutableData, baseBeaconRoot *common.Hash) (*typ.ExecutableData, *common.Hash, error) {
	if customNewPayload.PayloadCustomizer == nil {
		return basePayload, baseBeaconRoot, nil
	}
	return customNewPayload.PayloadCustomizer.CustomizePayload(basePayload, baseBeaconRoot)
}

func (customNewPayload *BaseNewPayloadVersionCustomizer) GetVersionedHashes(baseVersionedHashes *[]common.Hash) (*[]common.Hash, error) {
	if customNewPayload.VersionedHashesCustomizer != nil {
		return customNewPayload.VersionedHashesCustomizer.GetVersionedHashes(baseVersionedHashes)
	}
	return baseVersionedHashes, nil
}

func (customNewPayload *BaseNewPayloadVersionCustomizer) GetVersion(forkConfig *globals.ForkConfig, basePayload *typ.ExecutableData) (int, error) {
	var (
		timestamp = basePayload.Timestamp
		err       error
	)
	if customNewPayload.PayloadCustomizer != nil {
		timestamp, err = customNewPayload.PayloadCustomizer.GetTimestamp(basePayload)
		if err != nil {
			return 0, err
		}
	}

	return forkConfig.NewPayloadVersion(timestamp), nil
}

func (customNewPayload *BaseNewPayloadVersionCustomizer) GetExpectedError() (*int, error) {
	return customNewPayload.ExpectedError, nil
}

func (customNewPayload *BaseNewPayloadVersionCustomizer) GetExpectInvalidStatus() bool {
	return customNewPayload.ExpectInvalidStatus
}

// Customizer that upgrades the version of the payload to the next version.
type UpgradeNewPayloadVersion struct {
	NewPayloadCustomizer
}

var _ NewPayloadCustomizer = (*UpgradeNewPayloadVersion)(nil)

func (customNewPayload *UpgradeNewPayloadVersion) GetVersion(forkConfig *globals.ForkConfig, basePayload *typ.ExecutableData) (int, error) {
	if customNewPayload.NewPayloadCustomizer == nil {
		return 0, fmt.Errorf("base customizer not set")
	}
	version, err := customNewPayload.NewPayloadCustomizer.GetVersion(forkConfig, basePayload)
	return version + 1, err
}

// Customizer that downgrades the version of the payload to the previous version.
type DowngradeNewPayloadVersion struct {
	NewPayloadCustomizer
}

var _ NewPayloadCustomizer = (*DowngradeNewPayloadVersion)(nil)

func (customNewPayload *DowngradeNewPayloadVersion) GetVersion(forkConfig *globals.ForkConfig, basePayload *typ.ExecutableData) (int, error) {
	if customNewPayload.NewPayloadCustomizer == nil {
		return 0, fmt.Errorf("base customizer not set")
	}
	version, err := customNewPayload.NewPayloadCustomizer.GetVersion(forkConfig, basePayload)
	if version == 1 {
		return 1, fmt.Errorf("cannot downgrade version 1")
	}
	return version - 1, err
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
