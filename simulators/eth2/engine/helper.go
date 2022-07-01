package main

import (
	"crypto/ecdsa"
	"crypto/rand"
	"fmt"
	"math/big"
	"strings"
	"sync"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/trie"
	"github.com/ethereum/hive/hivesim"
	"github.com/ethereum/hive/simulators/eth2/engine/setup"
	blsu "github.com/protolambda/bls12-381-util"
	"github.com/rauljordan/engine-proxy/proxy"
)

type testSpec struct {
	Name  string
	About string
	Run   func(*hivesim.T, *testEnv, node)
}

type testEnv struct {
	Clients *ClientDefinitionsByRole
	Keys    []*setup.KeyDetails
	Secrets *[]blsu.SecretKey
}

type node struct {
	ExecutionClient      string
	ConsensusClient      string
	ValidatorShares      uint64
	ExecutionClientTTD   *big.Int
	BeaconNodeTTD        *big.Int
	TestVerificationNode bool
}

type Nodes []node

func (n *node) String() string {
	return fmt.Sprintf("%s-%s", n.ConsensusClient, n.ExecutionClient)
}

func (nodes Nodes) Shares() []uint64 {
	shares := make([]uint64, len(nodes))
	for i, n := range nodes {
		shares[i] = n.ValidatorShares
	}
	return shares
}

type Config struct {
	AltairForkEpoch         *big.Int
	MergeForkEpoch          *big.Int
	CapellaForkEpoch        *big.Int
	ValidatorCount          *big.Int
	KeyTranches             *big.Int
	SlotTime                *big.Int
	TerminalTotalDifficulty *big.Int

	// Node configurations to launch. Each node as a proportional share of
	// validators.
	Nodes         Nodes
	Eth1Consensus setup.Eth1Consensus

	// Execution Layer specific config
	InitialBaseFeePerGas *big.Int
}

// Choose a configuration value. `b` takes precedence
func choose(a, b *big.Int) *big.Int {
	if b != nil {
		return new(big.Int).Set(b)
	}
	if a != nil {
		return new(big.Int).Set(a)
	}
	return nil
}

// Join two configurations. `b` takes precedence
func (a *Config) join(b *Config) *Config {
	c := Config{}
	// Forks
	c.AltairForkEpoch = choose(a.AltairForkEpoch, b.AltairForkEpoch)
	c.MergeForkEpoch = choose(a.MergeForkEpoch, b.MergeForkEpoch)
	c.CapellaForkEpoch = choose(a.CapellaForkEpoch, b.CapellaForkEpoch)

	// Testnet config
	c.ValidatorCount = choose(a.ValidatorCount, b.ValidatorCount)
	c.KeyTranches = choose(a.KeyTranches, b.KeyTranches)
	c.SlotTime = choose(a.SlotTime, b.SlotTime)
	c.TerminalTotalDifficulty = choose(a.TerminalTotalDifficulty, b.TerminalTotalDifficulty)

	// EL config
	c.InitialBaseFeePerGas = choose(a.InitialBaseFeePerGas, b.InitialBaseFeePerGas)

	if b.Nodes != nil {
		c.Nodes = b.Nodes
	} else {
		c.Nodes = a.Nodes
	}

	if b.Eth1Consensus != nil {
		c.Eth1Consensus = b.Eth1Consensus
	} else {
		c.Eth1Consensus = a.Eth1Consensus
	}

	return &c
}

func (c *Config) activeFork() string {
	if c.MergeForkEpoch != nil && c.MergeForkEpoch.Cmp(common.Big0) == 0 {
		return "merge"
	} else if c.AltairForkEpoch != nil && c.AltairForkEpoch.Cmp(common.Big0) == 0 {
		return "altair"
	} else {
		return "phase0"
	}
}

func shorten(s string) string {
	start := s[0:6]
	end := s[62:]
	return fmt.Sprintf("%s..%s", start, end)
}

// Payload helper methods
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
	Nonce     *uint64
	GasPrice  *big.Int
	Gas       *uint64
	To        *common.Address
	Value     *big.Int
	Data      *[]byte
	Signature *SignatureValues
}

func customizeTransaction(baseTransaction *types.Transaction, pk *ecdsa.PrivateKey, customData *CustomTransactionData) (*types.Transaction, error) {
	// Create a modified transaction base, from the base transaction and customData mix
	modifiedTxBase := &types.LegacyTx{}

	if customData.Nonce != nil {
		modifiedTxBase.Nonce = *customData.Nonce
	} else {
		modifiedTxBase.Nonce = baseTransaction.Nonce()
	}
	if customData.GasPrice != nil {
		modifiedTxBase.GasPrice = customData.GasPrice
	} else {
		modifiedTxBase.GasPrice = baseTransaction.GasPrice()
	}
	if customData.Gas != nil {
		modifiedTxBase.Gas = *customData.Gas
	} else {
		modifiedTxBase.Gas = baseTransaction.Gas()
	}
	if customData.To != nil {
		modifiedTxBase.To = customData.To
	} else {
		modifiedTxBase.To = baseTransaction.To()
	}
	if customData.Value != nil {
		modifiedTxBase.Value = customData.Value
	} else {
		modifiedTxBase.Value = baseTransaction.Value()
	}
	if customData.Data != nil {
		modifiedTxBase.Data = *customData.Data
	} else {
		modifiedTxBase.Data = baseTransaction.Data()
	}
	var modifiedTx *types.Transaction
	if customData.Signature != nil {
		modifiedTxBase.V = customData.Signature.V
		modifiedTxBase.R = customData.Signature.R
		modifiedTxBase.S = customData.Signature.S
		modifiedTx = types.NewTx(modifiedTxBase)
	} else {
		// If a custom signature was not specified, simply sign the transaction again
		signer := types.NewEIP155Signer(CHAIN_ID)
		var err error
		modifiedTx, err = types.SignTx(types.NewTx(modifiedTxBase), signer, pk)
		if err != nil {
			return nil, err
		}
	}
	return modifiedTx, nil
}

type CustomPayloadData struct {
	ParentHash    *common.Hash
	FeeRecipient  *common.Address
	StateRoot     *common.Hash
	ReceiptsRoot  *common.Hash
	LogsBloom     *[]byte
	PrevRandao    *common.Hash
	Number        *uint64
	GasLimit      *uint64
	GasUsed       *uint64
	Timestamp     *uint64
	ExtraData     *[]byte
	BaseFeePerGas *big.Int
	BlockHash     *common.Hash
	Transactions  *[][]byte
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

// Construct a customized payload by taking an existing payload as base and mixing it CustomPayloadData
// BlockHash is calculated automatically.
func customizePayloadSpoof(method string, basePayload *ExecutableDataV1, customData *CustomPayloadData) (common.Hash, *proxy.Spoof, error) {
	fields := make(map[string]interface{})

	txs := basePayload.Transactions
	if customData.Transactions != nil {
		txs = *customData.Transactions
	}
	txsHash, err := calcTxsHash(txs)
	if err != nil {
		return common.Hash{}, nil, err
	}
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
		MixDigest:   basePayload.PrevRandao,
		Nonce:       types.BlockNonce{0}, // could be overwritten
		BaseFee:     basePayload.BaseFeePerGas,
	}

	// Overwrite custom information
	if customData.ParentHash != nil {
		customPayloadHeader.ParentHash = *customData.ParentHash
		fields["parentHash"] = customPayloadHeader.ParentHash
	}
	if customData.FeeRecipient != nil {
		customPayloadHeader.Coinbase = *customData.FeeRecipient
		fields["feeRecipient"] = customPayloadHeader.Coinbase
	}
	if customData.StateRoot != nil {
		customPayloadHeader.Root = *customData.StateRoot
		fields["stateRoot"] = customPayloadHeader.Root
	}
	if customData.ReceiptsRoot != nil {
		customPayloadHeader.ReceiptHash = *customData.ReceiptsRoot
		fields["receiptsRoot"] = customPayloadHeader.ReceiptHash
	}
	if customData.LogsBloom != nil {
		customPayloadHeader.Bloom = types.BytesToBloom(*customData.LogsBloom)
		fields["logsBloom"] = hexutil.Bytes(*customData.LogsBloom)
	}
	if customData.PrevRandao != nil {
		customPayloadHeader.MixDigest = *customData.PrevRandao
		fields["prevRandao"] = customPayloadHeader.MixDigest
	}
	if customData.Number != nil {
		customPayloadHeader.Number = big.NewInt(int64(*customData.Number))
		fields["blockNumber"] = hexutil.Uint64(*customData.Number)
	}
	if customData.GasLimit != nil {
		customPayloadHeader.GasLimit = *customData.GasLimit
		fields["gasLimit"] = hexutil.Uint64(customPayloadHeader.GasLimit)
	}
	if customData.GasUsed != nil {
		customPayloadHeader.GasUsed = *customData.GasUsed
		fields["gasUsed"] = hexutil.Uint64(customPayloadHeader.GasUsed)
	}
	if customData.Timestamp != nil {
		customPayloadHeader.Time = *customData.Timestamp
		fields["timestamp"] = hexutil.Uint64(customPayloadHeader.Time)
	}
	if customData.ExtraData != nil {
		customPayloadHeader.Extra = *customData.ExtraData
		fields["extraData"] = hexutil.Bytes(customPayloadHeader.Extra)
	}
	if customData.BaseFeePerGas != nil {
		customPayloadHeader.BaseFee = customData.BaseFeePerGas
		fields["baseFeePerGas"] = (*hexutil.Big)(customPayloadHeader.BaseFee)
	}

	var payloadHash common.Hash
	if customData.BlockHash != nil {
		payloadHash = *customData.BlockHash
	} else {
		payloadHash = customPayloadHeader.Hash()
	}
	fields["blockHash"] = payloadHash

	// Return the new payload
	return payloadHash, &proxy.Spoof{
		Method: method,
		Fields: fields,
	}, nil
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
	return strings.Join(customFieldsList, ", ")
}

type InvalidPayloadField string

const (
	InvalidParentHash           InvalidPayloadField = "ParentHash"
	InvalidStateRoot                                = "StateRoot"
	InvalidReceiptsRoot                             = "ReceiptsRoot"
	InvalidNumber                                   = "Number"
	InvalidGasLimit                                 = "GasLimit"
	InvalidGasUsed                                  = "GasUsed"
	InvalidTimestamp                                = "Timestamp"
	InvalidPrevRandao                               = "PrevRandao"
	RemoveTransaction                               = "Incomplete Transactions"
	InvalidTransactionSignature                     = "Transaction Signature"
	InvalidTransactionNonce                         = "Transaction Nonce"
	InvalidTransactionGas                           = "Transaction Gas"
	InvalidTransactionGasPrice                      = "Transaction GasPrice"
	InvalidTransactionValue                         = "Transaction Value"
)

// This function generates an invalid payload by taking a base payload and modifying the specified field such that it ends up being invalid.
// One small consideration is that the payload needs to contain transactions and specially transactions using the PREVRANDAO opcode for all the fields to be compatible with this function.
func generateInvalidPayloadSpoof(method string, basePayload *ExecutableDataV1, payloadField InvalidPayloadField) (common.Hash, *proxy.Spoof, error) {

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
		InvalidTransactionValue:

		if len(basePayload.Transactions) == 0 {
			return common.Hash{}, nil, fmt.Errorf("No transactions available for modification")
		}
		var baseTx types.Transaction
		if err := baseTx.UnmarshalBinary(basePayload.Transactions[0]); err != nil {
			return common.Hash{}, nil, err
		}
		var customTxData CustomTransactionData
		switch payloadField {
		case InvalidTransactionSignature:
			modifiedSignature := SignatureValuesFromRaw(baseTx.RawSignatureValues())
			modifiedSignature.R = modifiedSignature.R.Sub(modifiedSignature.R, big.NewInt(1))
			customTxData = CustomTransactionData{
				Signature: &modifiedSignature,
			}
		case InvalidTransactionNonce:
			customNonce := baseTx.Nonce() - 1
			customTxData = CustomTransactionData{
				Nonce: &customNonce,
			}
		case InvalidTransactionGas:
			customGas := uint64(0)
			customTxData = CustomTransactionData{
				Gas: &customGas,
			}
		case InvalidTransactionGasPrice:
			customTxData = CustomTransactionData{
				GasPrice: big.NewInt(0),
			}
		case InvalidTransactionValue:
			// Vault account initially has 0x123450000000000000000, so this value should overflow
			customValue, err := hexutil.DecodeBig("0x123450000000000000001")
			if err != nil {
				return common.Hash{}, nil, err
			}
			customTxData = CustomTransactionData{
				Value: customValue,
			}
		}

		modifiedTx, err := customizeTransaction(&baseTx, VAULT_KEY, &customTxData)
		if err != nil {
			return common.Hash{}, nil, err
		}

		modifiedTxBytes, err := modifiedTx.MarshalBinary()
		if err != nil {
		}
		modifiedTransactions := [][]byte{
			modifiedTxBytes,
		}
		customPayloadMod = &CustomPayloadData{
			Transactions: &modifiedTransactions,
		}
	}

	if customPayloadMod == nil {
		return common.Hash{}, nil, fmt.Errorf("Invalid payload field to corrupt: %s", payloadField)
	}

	return customizePayloadSpoof(method, basePayload, customPayloadMod)
}

// Generate a payload status spoof
func payloadStatusSpoof(method string, status *PayloadStatusV1) (*proxy.Spoof, error) {
	fields := make(map[string]interface{})
	fields["status"] = status.Status
	fields["latestValidHash"] = status.LatestValidHash
	fields["validationError"] = status.ValidationError

	// Return the new payload status spoof
	return &proxy.Spoof{
		Method: method,
		Fields: fields,
	}, nil
}

// Generate a payload status spoof
func forkchoiceResponseSpoof(method string, status PayloadStatusV1, payloadID *PayloadID) (*proxy.Spoof, error) {
	fields := make(map[string]interface{})
	fields["payloadStatus"] = status
	fields["payloadId"] = payloadID

	// Return the new payload status spoof
	return &proxy.Spoof{
		Method: method,
		Fields: fields,
	}, nil
}

// Generates a callback that detects when a ForkchoiceUpdated with Payload Attributes fails.
// Requires a lock in case two clients receive the fcU with payload attributes at the same time.
// Requires chan(error) to return the final outcome of the callbacks.
// Requires the maximum number of fcU+attr calls to check.
func CheckErrorOnForkchoiceUpdatedPayloadAttr(fcuLock *sync.Mutex, fcUCountLimit int, fcUAttrCount *int, fcudone chan<- error) func(res []byte, req []byte) *proxy.Spoof {
	return func(res []byte, req []byte) *proxy.Spoof {
		var (
			fcS ForkchoiceStateV1
			pA  *PayloadAttributesV1
		)
		if err := UnmarshalFromJsonRPCRequest(req, &fcS, &pA); err != nil {
			panic(fmt.Errorf("Unable to parse ForkchoiceUpdated request: %v", err))
		}
		if pA != nil {
			fcuLock.Lock()
			defer fcuLock.Unlock()
			(*fcUAttrCount)++
			// The CL requested a payload, it should have not produced an error
			var (
				fcResponse ForkChoiceResponse
			)
			err := UnmarshalFromJsonRPCResponse(res, &fcResponse)
			if err == nil && fcResponse.PayloadID == nil {
				err = fmt.Errorf("PayloadID null on ForkchoiceUpdated with attributes")
			}
			if err != nil || (*fcUAttrCount) > fcUCountLimit {
				select {
				case fcudone <- err:
				default:
				}
			}
		}
		return nil
	}
}

func combine(a, b *proxy.Spoof) *proxy.Spoof {
	if a == nil {
		return b
	}
	if b == nil {
		return a
	}
	if a.Method != b.Method {
		panic(fmt.Errorf("Spoof methods don't match: %s != %s", a.Method, b.Method))
	}
	for k, v := range b.Fields {
		a.Fields[k] = v
	}
	return a
}
