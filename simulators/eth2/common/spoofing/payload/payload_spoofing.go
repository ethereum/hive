package payload

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"strings"
	"sync"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	api "github.com/ethereum/go-ethereum/beacon/engine"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/trie"
	"github.com/ethereum/hive/simulators/eth2/common/spoofing/proxy"
	spoof "github.com/rauljordan/engine-proxy/proxy"
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

// Payload helper methods
type SignatureValues struct {
	V *big.Int
	R *big.Int
	S *big.Int
}

func SignatureValuesFromRaw(
	v *big.Int,
	r *big.Int,
	s *big.Int,
) SignatureValues {
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

type SignerWithKey interface {
	SignTx(*types.Transaction) (*types.Transaction, error)
}

func CustomizeTransaction(
	baseTransaction *types.Transaction,
	customData *CustomTransactionData,
	signer SignerWithKey,
) (*types.Transaction, error) {
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

	if customData.Signature != nil {
		modifiedTxBase.V = customData.Signature.V
		modifiedTxBase.R = customData.Signature.R
		modifiedTxBase.S = customData.Signature.S
		return types.NewTx(modifiedTxBase), nil
	} else {
		// If a custom signature was not specified, simply sign the transaction again
		if signer == nil {
			return nil, fmt.Errorf("custom tx needs to be signed but no signer provided")
		}
		return signer.SignTx(types.NewTx(modifiedTxBase))
	}
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
	return types.DeriveSha(
		types.Transactions(txs),
		trie.NewStackTrie(nil),
	), nil
}

// Construct a customized payload by taking an existing payload as base and mixing it CustomPayloadData
// BlockHash is calculated automatically.
func CustomizePayloadSpoof(
	method string,
	basePayload *api.ExecutableData,
	customData *CustomPayloadData,
) (common.Hash, *spoof.Spoof, error) {
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
		MixDigest:   basePayload.Random,
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
	return payloadHash, &spoof.Spoof{
		Method: method,
		Fields: fields,
	}, nil
}

func (customData *CustomPayloadData) String() string {
	customFieldsList := make([]string, 0)
	if customData.ParentHash != nil {
		customFieldsList = append(
			customFieldsList,
			fmt.Sprintf("ParentHash=%s", customData.ParentHash.String()),
		)
	}
	if customData.FeeRecipient != nil {
		customFieldsList = append(
			customFieldsList,
			fmt.Sprintf("Coinbase=%s", customData.FeeRecipient.String()),
		)
	}
	if customData.StateRoot != nil {
		customFieldsList = append(
			customFieldsList,
			fmt.Sprintf("StateRoot=%s", customData.StateRoot.String()),
		)
	}
	if customData.ReceiptsRoot != nil {
		customFieldsList = append(
			customFieldsList,
			fmt.Sprintf("ReceiptsRoot=%s", customData.ReceiptsRoot.String()),
		)
	}
	if customData.LogsBloom != nil {
		customFieldsList = append(
			customFieldsList,
			fmt.Sprintf(
				"LogsBloom=%v",
				types.BytesToBloom(*customData.LogsBloom),
			),
		)
	}
	if customData.PrevRandao != nil {
		customFieldsList = append(
			customFieldsList,
			fmt.Sprintf("PrevRandao=%s", customData.PrevRandao.String()),
		)
	}
	if customData.Number != nil {
		customFieldsList = append(
			customFieldsList,
			fmt.Sprintf("Number=%d", *customData.Number),
		)
	}
	if customData.GasLimit != nil {
		customFieldsList = append(
			customFieldsList,
			fmt.Sprintf("GasLimit=%d", *customData.GasLimit),
		)
	}
	if customData.GasUsed != nil {
		customFieldsList = append(
			customFieldsList,
			fmt.Sprintf("GasUsed=%d", *customData.GasUsed),
		)
	}
	if customData.Timestamp != nil {
		customFieldsList = append(
			customFieldsList,
			fmt.Sprintf("Timestamp=%d", *customData.Timestamp),
		)
	}
	if customData.ExtraData != nil {
		customFieldsList = append(
			customFieldsList,
			fmt.Sprintf("ExtraData=%v", *customData.ExtraData),
		)
	}
	if customData.BaseFeePerGas != nil {
		customFieldsList = append(
			customFieldsList,
			fmt.Sprintf("BaseFeePerGas=%s", customData.BaseFeePerGas.String()),
		)
	}
	if customData.Transactions != nil {
		customFieldsList = append(
			customFieldsList,
			fmt.Sprintf("Transactions=%v", customData.Transactions),
		)
	}
	return strings.Join(customFieldsList, ", ")
}

type InvalidPayloadField string

const (
	InvalidParentHash           = "ParentHash"
	InvalidStateRoot            = "StateRoot"
	InvalidReceiptsRoot         = "ReceiptsRoot"
	InvalidNumber               = "Number"
	InvalidGasLimit             = "GasLimit"
	InvalidGasUsed              = "GasUsed"
	InvalidTimestamp            = "Timestamp"
	InvalidPrevRandao           = "PrevRandao"
	RemoveTransaction           = "Incomplete Transactions"
	InvalidTransactionSignature = "Transaction Signature"
	InvalidTransactionNonce     = "Transaction Nonce"
	InvalidTransactionGas       = "Transaction Gas"
	InvalidTransactionGasPrice  = "Transaction GasPrice"
	InvalidTransactionValue     = "Transaction Value"
)

// This function generates an invalid payload by taking a base payload and modifying the specified field such that it ends up being invalid.
// One small consideration is that the payload needs to contain transactions and specially transactions using the PREVRANDAO opcode for all the fields to be compatible with this function.
func GenerateInvalidPayloadSpoof(
	method string,
	basePayload *api.ExecutableData,
	payloadField InvalidPayloadField,
	signer SignerWithKey,
) (common.Hash, *spoof.Spoof, error) {
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
		modStateRoot[common.HashLength-1] = byte(
			255 - modStateRoot[common.HashLength-1],
		)
		customPayloadMod = &CustomPayloadData{
			StateRoot: &modStateRoot,
		}
	case InvalidReceiptsRoot:
		modReceiptsRoot := basePayload.ReceiptsRoot
		modReceiptsRoot[common.HashLength-1] = byte(
			255 - modReceiptsRoot[common.HashLength-1],
		)
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
			return common.Hash{}, nil, fmt.Errorf(
				"no transactions available for modification",
			)
		}
		var baseTx types.Transaction
		if err := baseTx.UnmarshalBinary(basePayload.Transactions[0]); err != nil {
			return common.Hash{}, nil, err
		}
		var customTxData CustomTransactionData
		switch payloadField {
		case InvalidTransactionSignature:
			modifiedSignature := SignatureValuesFromRaw(
				baseTx.RawSignatureValues(),
			)
			modifiedSignature.R = modifiedSignature.R.Sub(
				modifiedSignature.R,
				big.NewInt(1),
			)
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

		modifiedTx, err := CustomizeTransaction(
			&baseTx,
			&customTxData,
			signer,
		)
		if err != nil {
			return common.Hash{}, nil, err
		}

		modifiedTxBytes, err := modifiedTx.MarshalBinary()
		if err != nil {
			return common.Hash{}, nil, err
		}
		modifiedTransactions := [][]byte{
			modifiedTxBytes,
		}
		customPayloadMod = &CustomPayloadData{
			Transactions: &modifiedTransactions,
		}
	}

	if customPayloadMod == nil {
		return common.Hash{}, nil, fmt.Errorf(
			"invalid payload field to corrupt: %s",
			payloadField,
		)
	}

	return CustomizePayloadSpoof(method, basePayload, customPayloadMod)
}

// Generate a payload status spoof
func PayloadStatusSpoof(
	method string,
	status *api.PayloadStatusV1,
) (*spoof.Spoof, error) {
	fields := make(map[string]interface{})
	fields["status"] = status.Status
	fields["latestValidHash"] = status.LatestValidHash
	fields["validationError"] = status.ValidationError

	// Return the new payload status spoof
	return &spoof.Spoof{
		Method: method,
		Fields: fields,
	}, nil
}

// Generate a payload status spoof
func ForkchoiceResponseSpoof(
	method string,
	status api.PayloadStatusV1,
	payloadID *api.PayloadID,
) (*spoof.Spoof, error) {
	fields := make(map[string]interface{})
	fields["payloadStatus"] = status
	fields["payloadId"] = payloadID

	// Return the new payload status spoof
	return &spoof.Spoof{
		Method: method,
		Fields: fields,
	}, nil
}

type EngineResponseHash struct {
	Response *api.PayloadStatusV1
	Hash     common.Hash
}

type EngineResponseMocker struct {
	Lock                    sync.Mutex
	DefaultResponse         *api.PayloadStatusV1
	HashToResponse          map[common.Hash]*api.PayloadStatusV1
	HashPassthrough         map[common.Hash]bool
	NewPayloadCalled        chan EngineResponseHash
	ForkchoiceUpdatedCalled chan EngineResponseHash
	Mocking                 bool
}

func NewEngineResponseMocker(
	defaultResponse *api.PayloadStatusV1,
	perHashResponses ...*EngineResponseHash,
) *EngineResponseMocker {
	e := &EngineResponseMocker{
		DefaultResponse:         defaultResponse,
		HashToResponse:          make(map[common.Hash]*api.PayloadStatusV1),
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

func (e *EngineResponseMocker) AddResponse(
	h common.Hash,
	r *api.PayloadStatusV1,
) {
	e.Lock.Lock()
	defer e.Lock.Unlock()
	if e.HashToResponse == nil {
		e.HashToResponse = make(map[common.Hash]*api.PayloadStatusV1)
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

func (e *EngineResponseMocker) GetResponse(
	h common.Hash,
) *api.PayloadStatusV1 {
	e.Lock.Lock()
	defer e.Lock.Unlock()
	if e.HashToResponse != nil {
		if r, ok := e.HashToResponse[h]; ok {
			return r
		}
	}
	return e.DefaultResponse
}

func (e *EngineResponseMocker) SetDefaultResponse(r *api.PayloadStatusV1) {
	e.Lock.Lock()
	defer e.Lock.Unlock()
	e.DefaultResponse = r
}

func (e *EngineResponseMocker) AddGetPayloadPassthroughToProxy(
	p *proxy.Proxy,
) {
	p.AddResponseCallback(
		EngineGetPayloadV1,
		func(res []byte, req []byte) *spoof.Spoof {
			// Hash of the payload built is being added to the passthrough list
			var (
				payload api.ExecutableData // ExecutableDataV1
			)
			err := proxy.UnmarshalFromJsonRPCResponse(res, &payload)
			if err != nil {
				panic(err)
			}
			e.AddPassthrough(payload.BlockHash, true)
			return nil
		},
	)
}

func (e *EngineResponseMocker) AddNewPayloadCallbackToProxy(p *proxy.Proxy) {
	p.AddResponseCallback(
		EngineNewPayloadV1,
		func(res []byte, req []byte) *spoof.Spoof {
			var (
				payload api.ExecutableData
				status  api.PayloadStatusV1
				spoof   *spoof.Spoof
				err     error
			)
			err = proxy.UnmarshalFromJsonRPCRequest(req, &payload)
			if err != nil {
				panic(err)
			}
			err = proxy.UnmarshalFromJsonRPCResponse(res, &status)
			if err != nil {
				panic(err)
			}
			if r := e.GetResponse(payload.BlockHash); e.Mocking &&
				!e.CanPassthrough(payload.BlockHash) &&
				r != nil {
				// We are mocking this specific response, either with a hash specific response, or the default response
				spoof, err = PayloadStatusSpoof(
					EngineNewPayloadV1,
					&api.PayloadStatusV1{
						Status:          r.Status,
						LatestValidHash: r.LatestValidHash,
						ValidationError: nil,
					},
				)
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
					Response: &status,
					Hash:     payload.BlockHash,
				}:
				default:
				}
			}
			return nil
		},
	)
}

func (e *EngineResponseMocker) AddForkchoiceUpdatedCallbackToProxy(
	p *proxy.Proxy,
) {
	p.AddResponseCallback(
		EngineForkchoiceUpdatedV1,
		func(res []byte, req []byte) *spoof.Spoof {
			var (
				fcState api.ForkchoiceStateV1
				pAttr   api.PayloadAttributes
				fResp   api.ForkChoiceResponse
				spoof   *spoof.Spoof
				err     error
			)
			err = proxy.UnmarshalFromJsonRPCRequest(req, &fcState, &pAttr)
			if err != nil {
				panic(err)
			}
			err = proxy.UnmarshalFromJsonRPCResponse(res, &fResp)
			if err != nil {
				panic(err)
			}

			if r := e.GetResponse(fcState.HeadBlockHash); e.Mocking &&
				!e.CanPassthrough(fcState.HeadBlockHash) &&
				r != nil {
				// We are mocking this specific response, either with a hash specific response, or the default response
				spoof, err = ForkchoiceResponseSpoof(
					EngineForkchoiceUpdatedV1,
					api.PayloadStatusV1{
						Status:          r.Status,
						LatestValidHash: r.LatestValidHash,
						ValidationError: nil,
					},
					nil,
				)
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
					Response: &fResp.PayloadStatus,
					Hash:     fcState.HeadBlockHash,
				}:
				default:
				}
			}
			return nil
		},
	)
}

func (e *EngineResponseMocker) AddCallbacksToProxy(p *proxy.Proxy) {
	e.AddForkchoiceUpdatedCallbackToProxy(p)
	e.AddNewPayloadCallbackToProxy(p)
}

// Generates a callback that detects when a ForkchoiceUpdated with Payload Attributes fails.
// Requires a lock in case two clients receive the fcU with payload attributes at the same time.
// Requires chan(error) to return the final outcome of the callbacks.
// Requires the maximum number of fcU+attr calls to check.
func CheckErrorOnForkchoiceUpdatedPayloadAttributes(
	fcuLock *sync.Mutex,
	fcUCountLimit int,
	fcUAttrCount *int,
	fcudone chan<- error,
) func(res []byte, req []byte) *spoof.Spoof {
	return func(res []byte, req []byte) *spoof.Spoof {
		var (
			fcS api.ForkchoiceStateV1
			pA  *api.PayloadAttributes
		)
		if err := proxy.UnmarshalFromJsonRPCRequest(req, &fcS, &pA); err != nil {
			panic(
				fmt.Errorf(
					"unable to parse ForkchoiceUpdated request: %v",
					err,
				),
			)
		}
		if pA != nil {
			fcuLock.Lock()
			defer fcuLock.Unlock()
			(*fcUAttrCount)++
			// The CL requested a payload, it should have not produced an error
			var (
				fcResponse api.ForkChoiceResponse
			)
			err := proxy.UnmarshalFromJsonRPCResponse(res, &fcResponse)
			if err == nil && fcResponse.PayloadID == nil {
				err = fmt.Errorf(
					"PayloadID null on ForkchoiceUpdated with attributes",
				)
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
