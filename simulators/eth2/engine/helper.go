package main

import (
	"context"
	"crypto/ecdsa"
	"crypto/rand"
	"fmt"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/ethereum/go-ethereum/trie"
	"github.com/ethereum/hive/hivesim"
	"github.com/ethereum/hive/simulators/eth2/engine/setup"
	blsu "github.com/protolambda/bls12-381-util"
	beacon "github.com/protolambda/zrnt/eth2/beacon/common"
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

type Logging interface {
	Logf(format string, values ...interface{})
}

type node struct {
	ExecutionClient      string
	ConsensusClient      string
	ValidatorShares      uint64
	ExecutionClientTTD   *big.Int
	BeaconNodeTTD        *big.Int
	TestVerificationNode bool
	DisableStartup       bool
	ChainGenerator       ChainGenerator
	Chain                []*types.Block
	ExecutionSubnet      string
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
	AltairForkEpoch                 *big.Int
	MergeForkEpoch                  *big.Int
	CapellaForkEpoch                *big.Int
	ValidatorCount                  *big.Int
	KeyTranches                     *big.Int
	SlotTime                        *big.Int
	TerminalTotalDifficulty         *big.Int
	SafeSlotsToImportOptimistically *big.Int
	ExtraShares                     *big.Int

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
	c.SafeSlotsToImportOptimistically = choose(a.SafeSlotsToImportOptimistically, b.SafeSlotsToImportOptimistically)
	c.ExtraShares = choose(a.ExtraShares, b.ExtraShares)

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
		var randomParentHash common.Hash
		rand.Read(randomParentHash[:])
		customPayloadMod = &CustomPayloadData{
			ParentHash: &randomParentHash,
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

type EngineResponse struct {
	Status          PayloadStatus
	LatestValidHash *common.Hash
}

type EngineResponseHash struct {
	Response *EngineResponse
	Hash     common.Hash
}

//
type EngineResponseMocker struct {
	Lock                    sync.Mutex
	DefaultResponse         *EngineResponse
	HashToResponse          map[common.Hash]*EngineResponse
	HashPassthrough         map[common.Hash]bool
	NewPayloadCalled        chan EngineResponseHash
	ForkchoiceUpdatedCalled chan EngineResponseHash
	Mocking                 bool
}

func NewEngineResponseMocker(defaultResponse *EngineResponse, perHashResponses ...*EngineResponseHash) *EngineResponseMocker {
	e := &EngineResponseMocker{
		DefaultResponse:         defaultResponse,
		HashToResponse:          make(map[common.Hash]*EngineResponse),
		HashPassthrough:         make(map[common.Hash]bool),
		NewPayloadCalled:        make(chan EngineResponseHash),
		ForkchoiceUpdatedCalled: make(chan EngineResponseHash),
		Mocking:                 true,
	}
	for _, r := range perHashResponses {
		e.AddResponse(r.Hash, r.Response)
	}
	return e
}

func (e *EngineResponseMocker) AddResponse(h common.Hash, r *EngineResponse) {
	e.Lock.Lock()
	defer e.Lock.Unlock()
	if e.HashToResponse == nil {
		e.HashToResponse = make(map[common.Hash]*EngineResponse)
	}
	e.HashToResponse[h] = r
}

func (e *EngineResponseMocker) AddPassthrough(h common.Hash, pass bool) {
	e.Lock.Lock()
	defer e.Lock.Unlock()
	e.HashPassthrough[h] = pass
}

func (e *EngineResponseMocker) CanPassthrough(h common.Hash) bool {
	e.Lock.Lock()
	defer e.Lock.Unlock()
	if pass, ok := e.HashPassthrough[h]; ok && pass {
		return true
	}
	return false
}

func (e *EngineResponseMocker) GetResponse(h common.Hash) *EngineResponse {
	e.Lock.Lock()
	defer e.Lock.Unlock()
	if e.HashToResponse != nil {
		if r, ok := e.HashToResponse[h]; ok {
			return r
		}
	}
	return e.DefaultResponse
}

func (e *EngineResponseMocker) SetDefaultResponse(r *EngineResponse) {
	e.Lock.Lock()
	defer e.Lock.Unlock()
	e.DefaultResponse = r
}

func (e *EngineResponseMocker) AddGetPayloadPassthroughToProxy(p *Proxy) {
	p.AddResponseCallback(EngineGetPayloadV1, func(res []byte, req []byte) *proxy.Spoof {
		// Hash of the payload built is being added to the passthrough list
		var (
			payload ExecutableDataV1
		)
		err := UnmarshalFromJsonRPCResponse(res, &payload)
		if err != nil {
			panic(err)
		}
		e.AddPassthrough(payload.BlockHash, true)
		return nil
	})
}

func (e *EngineResponseMocker) AddNewPayloadCallbackToProxy(p *Proxy) {
	p.AddResponseCallback(EngineNewPayloadV1, func(res []byte, req []byte) *proxy.Spoof {
		var (
			payload ExecutableDataV1
			status  PayloadStatusV1
			spoof   *proxy.Spoof
			err     error
		)
		err = UnmarshalFromJsonRPCRequest(req, &payload)
		if err != nil {
			panic(err)
		}
		err = UnmarshalFromJsonRPCResponse(res, &status)
		if err != nil {
			panic(err)
		}
		if r := e.GetResponse(payload.BlockHash); e.Mocking && !e.CanPassthrough(payload.BlockHash) && r != nil {
			// We are mocking this specific response, either with a hash specific response, or the default response
			spoof, err = payloadStatusSpoof(EngineNewPayloadV1, &PayloadStatusV1{
				Status:          r.Status,
				LatestValidHash: r.LatestValidHash,
				ValidationError: nil,
			})
			if err != nil {
				panic(err)
			}
			select {
			case e.NewPayloadCalled <- EngineResponseHash{
				Response: r,
				Hash:     payload.BlockHash,
			}:
			default:
			}
			return spoof
		} else {
			select {
			case e.NewPayloadCalled <- EngineResponseHash{
				Response: &EngineResponse{
					Status:          status.Status,
					LatestValidHash: status.LatestValidHash,
				},
				Hash: payload.BlockHash,
			}:
			default:
			}
		}
		return nil
	})
}

func (e *EngineResponseMocker) AddForkchoiceUpdatedCallbackToProxy(p *Proxy) {
	p.AddResponseCallback(EngineForkchoiceUpdatedV1, func(res []byte, req []byte) *proxy.Spoof {
		var (
			fcState ForkchoiceStateV1
			pAttr   PayloadAttributesV1
			fResp   ForkChoiceResponse
			spoof   *proxy.Spoof
			err     error
		)
		err = UnmarshalFromJsonRPCRequest(req, &fcState, &pAttr)
		if err != nil {
			panic(err)
		}
		err = UnmarshalFromJsonRPCResponse(res, &fResp)
		if err != nil {
			panic(err)
		}

		if r := e.GetResponse(fcState.HeadBlockHash); e.Mocking && !e.CanPassthrough(fcState.HeadBlockHash) && r != nil {
			// We are mocking this specific response, either with a hash specific response, or the default response
			spoof, err = forkchoiceResponseSpoof(EngineForkchoiceUpdatedV1, PayloadStatusV1{
				Status:          r.Status,
				LatestValidHash: r.LatestValidHash,
				ValidationError: nil,
			}, nil)
			if err != nil {
				panic(err)
			}

			select {
			case e.ForkchoiceUpdatedCalled <- EngineResponseHash{
				Response: r,
				Hash:     fcState.HeadBlockHash,
			}:
			default:
			}
			return spoof
		} else {
			// Let the original response pass through
			select {
			case e.ForkchoiceUpdatedCalled <- EngineResponseHash{
				Response: &EngineResponse{
					Status:          fResp.PayloadStatus.Status,
					LatestValidHash: fResp.PayloadStatus.LatestValidHash,
				},
				Hash: fcState.HeadBlockHash,
			}:
			default:
			}
		}
		return nil
	})
}

func (e *EngineResponseMocker) AddCallbacksToProxy(p *Proxy) {
	e.AddForkchoiceUpdatedCallbackToProxy(p)
	e.AddNewPayloadCallbackToProxy(p)
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

func ContextWithSlotsTimeout(parent context.Context, t *Testnet, slots beacon.Slot) (context.Context, context.CancelFunc) {
	timeout := time.Duration(uint64(slots)*uint64(t.spec.SECONDS_PER_SLOT)) * time.Second
	return context.WithTimeout(parent, timeout)
}

// Try to approximate how much time until the merge based on current time, bellatrix fork epoch,
// TTD, execution clients' consensus mechanism, current total difficulty.
// This function is used to calculate timeouts, so it will always return a pessimistic value.
func SlotsUntilMerge(t *Testnet, c *Config) beacon.Slot {
	l := make([]beacon.Slot, 0)
	l = append(l, SlotsUntilBellatrix(t.genesisTime, t.spec))

	for i, e := range t.ExecutionClients().Running() {
		l = append(l, beacon.Slot(TimeUntilTerminalBlock(e, c.Eth1Consensus, c.TerminalTotalDifficulty, c.Nodes[i])/uint64(t.spec.SECONDS_PER_SLOT)))
	}

	// Return the worst case
	var max = beacon.Slot(0)
	for _, s := range l {
		if s > max {
			max = s
		}
	}

	fmt.Printf("INFO: Estimated slots until merge %d\n", max)

	// Add more slots give it some wiggle room
	return max + 5
}

func SlotsUntilBellatrix(genesisTime beacon.Timestamp, spec *beacon.Spec) beacon.Slot {
	currentTimestamp := beacon.Timestamp(time.Now().Unix())
	bellatrixTime, err := spec.TimeAtSlot(beacon.Slot(spec.BELLATRIX_FORK_EPOCH*beacon.Epoch(spec.SLOTS_PER_EPOCH)), genesisTime)
	if err != nil {
		panic(err)
	}
	if currentTimestamp >= bellatrixTime {
		return beacon.Slot(0)
	}
	s := beacon.Slot((bellatrixTime-currentTimestamp)/spec.SECONDS_PER_SLOT) + 1
	fmt.Printf("INFO: bellatrixTime:%d, currentTimestamp:%d, slots=%d\n", bellatrixTime, currentTimestamp, s)
	return s
}

func TimeUntilTerminalBlock(e *ExecutionClient, c setup.Eth1Consensus, defaultTTD *big.Int, n node) uint64 {
	var ttd = defaultTTD
	if n.ExecutionClientTTD != nil {
		ttd = n.ExecutionClientTTD
	}
	// Get the current total difficulty for node
	userRPCAddress, err := e.UserRPCAddress()
	if err != nil {
		panic(err)
	}
	client := &http.Client{}
	rpcClient, _ := rpc.DialHTTPWithClient(userRPCAddress, client)
	var tdh *TotalDifficultyHeader
	if err := rpcClient.CallContext(context.Background(), &tdh, "eth_getBlockByNumber", "latest", false); err != nil {
		panic(err)
	}
	td := tdh.TotalDifficulty.ToInt()
	if td.Cmp(ttd) >= 0 {
		// TTD already reached
		return 0
	}
	fmt.Printf("INFO: ttd:%d, td:%d, diffPerBlock:%d, secondsPerBlock:%d\n", ttd, td, c.DifficultyPerBlock(), c.SecondsPerBlock())
	td.Sub(ttd, td).Div(td, c.DifficultyPerBlock()).Mul(td, big.NewInt(int64(c.SecondsPerBlock())))
	return td.Uint64()
}

func SlotsToDuration(slots beacon.Slot, spec *beacon.Spec) time.Duration {
	return time.Duration(slots) * time.Duration(spec.SECONDS_PER_SLOT) * time.Second
}

// Gets the head of the execution client
func GetHead(ctx context.Context, en *ExecutionClient) (ethcommon.Hash, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	userRPCAddress, err := en.UserRPCAddress()
	if err != nil {
		panic(err)
	}
	client := &http.Client{}
	rpcClient, _ := rpc.DialHTTPWithClient(userRPCAddress, client)
	eth := ethclient.NewClient(rpcClient)
	b, err := eth.BlockByNumber(ctx, nil)
	if err != nil {
		return ethcommon.Hash{}, err
	}
	return b.Hash(), nil
}

// Returns true if all head hashes match
func CheckHeads(ctx context.Context, all ExecutionClients) (bool, error) {
	if len(all) <= 1 {
		return true, nil
	}
	var hash ethcommon.Hash
	for i, en := range all {
		h, err := GetHead(ctx, en)
		if err != nil {
			return false, err
		}
		if i == 0 {
			hash = h
		} else {
			if h != hash {
				fmt.Printf("Hash mismatch between heads: %s != %s\n", h, hash)
				return false, nil
			}
			fmt.Printf("Hash match between heads: %s == %s\n", h, hash)
		}
	}
	return true, nil
}
