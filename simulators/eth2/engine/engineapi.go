package main

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"net"
	"net/http"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/ethereum/hive/hivesim"
	"github.com/golang-jwt/jwt/v4"
)

// https://github.com/ethereum/execution-apis/blob/v1.0.0-alpha.7/src/engine/specification.md
var EthPortHTTP = 8545
var EnginePortHTTP = 8551

// API call names
const (
	EngineForkchoiceUpdatedV1 string = "engine_forkchoiceUpdatedV1"
	EngineGetPayloadV1               = "engine_getPayloadV1"
	EngineNewPayloadV1               = "engine_newPayloadV1"
)

// EngineClient wrapper for Ethereum Engine RPC for testing purposes.
type EngineClient struct {
	*hivesim.T
	*hivesim.Client
	c                       *rpc.Client
	Eth                     *ethclient.Client
	cEth                    *rpc.Client
	IP                      net.IP
	TerminalTotalDifficulty *big.Int

	// This holds most recent context created by the Ctx method.
	// Every time Ctx is called, it creates a new context with the default
	// timeout and cancels the previous one.
	lastCtx    context.Context
	lastCancel context.CancelFunc
}

// NewClient creates a engine client that uses the given RPC client.
func NewEngineClient(t *hivesim.T, n *Eth1Node, ttd *big.Int) *EngineClient {
	engineRPCAddress, err := n.EngineRPCAddress()
	if err != nil {
		panic(err)
	}

	client := &http.Client{}
	// Prepare HTTP Client
	rpcHttpClient, _ := rpc.DialHTTPWithClient(engineRPCAddress, client)

	// Prepare ETH Client
	client = &http.Client{}

	userRPCAddress, err := n.UserRPCAddress()
	if err != nil {
		panic(err)
	}
	rpcClient, _ := rpc.DialHTTPWithClient(userRPCAddress, client)
	eth := ethclient.NewClient(rpcClient)
	return &EngineClient{
		T:                       t,
		c:                       rpcHttpClient,
		Eth:                     eth,
		cEth:                    rpcClient,
		TerminalTotalDifficulty: ttd,
		lastCtx:                 nil,
		lastCancel:              nil,
	}
}

func (ec *EngineClient) Close() {
	ec.c.Close()
	ec.Eth.Close()
	if ec.lastCtx != nil {
		ec.lastCancel()
	}
}

func (ec *EngineClient) Ctx() context.Context {
	if ec.lastCtx != nil {
		ec.lastCancel()
	}
	ec.lastCtx, ec.lastCancel = context.WithTimeout(context.Background(), 10*time.Second)
	return ec.lastCtx
}

// Helper structs to fetch the TotalDifficulty
type TD struct {
	TotalDifficulty *hexutil.Big `json:"totalDifficulty"`
}
type TotalDifficultyHeader struct {
	types.Header
	TD
}

func (tdh *TotalDifficultyHeader) UnmarshalJSON(data []byte) error {
	if err := json.Unmarshal(data, &tdh.Header); err != nil {
		return err
	}
	if err := json.Unmarshal(data, &tdh.TD); err != nil {
		return err
	}
	return nil
}

func (ec *EngineClient) checkTTD() (*types.Header, bool) {
	var td *TotalDifficultyHeader
	if err := ec.cEth.CallContext(ec.Ctx(), &td, "eth_getBlockByNumber", "latest", false); err != nil {
		panic(err)
	}
	if td.TotalDifficulty.ToInt().Cmp(ec.TerminalTotalDifficulty) >= 0 {
		return &td.Header, true
	}
	return nil, false
}

// Engine API Types
type PayloadStatusV1 struct {
	Status          PayloadStatus `json:"status"`
	LatestValidHash *common.Hash  `json:"latestValidHash"`
	ValidationError *string       `json:"validationError"`
}
type ForkChoiceResponse struct {
	PayloadStatus PayloadStatusV1 `json:"payloadStatus"`
	PayloadID     *PayloadID      `json:"payloadId"`
}

type PayloadStatus int

const (
	Unknown PayloadStatus = iota
	Valid
	Invalid
	Accepted
	Syncing
	InvalidBlockHash
)

var PayloadStatuses = map[PayloadStatus]string{
	Valid:            "VALID",
	Invalid:          "INVALID",
	Accepted:         "ACCEPTED",
	Syncing:          "SYNCING",
	InvalidBlockHash: "INVALID_BLOCK_HASH",
}

func (b PayloadStatus) String() string {
	str, ok := PayloadStatuses[b]
	if !ok {
		return "UNKNOWN"
	}
	return str
}

func (b PayloadStatus) MarshalText() ([]byte, error) {
	str, ok := PayloadStatuses[b]
	if !ok {
		return nil, fmt.Errorf("invalid payload status")
	}
	return []byte(str), nil
}

func (b *PayloadStatus) UnmarshalText(input []byte) error {
	s := string(input)
	for p, status := range PayloadStatuses {
		if status == s {
			*b = p
			return nil
		}
	}
	return fmt.Errorf("invalid payload status: %s", s)
}

type PayloadID [8]byte

func (b PayloadID) MarshalText() ([]byte, error) {
	return hexutil.Bytes(b[:]).MarshalText()
}
func (b *PayloadID) UnmarshalText(input []byte) error {
	err := hexutil.UnmarshalFixedText("PayloadID", input, b[:])
	if err != nil {
		return fmt.Errorf("invalid payload id %q: %w", input, err)
	}
	return nil
}

type ForkchoiceStateV1 struct {
	HeadBlockHash      common.Hash `json:"headBlockHash"`
	SafeBlockHash      common.Hash `json:"safeBlockHash"`
	FinalizedBlockHash common.Hash `json:"finalizedBlockHash"`
}

//go:generate go run github.com/fjl/gencodec -type ExecutableDataV1 -field-override executableDataMarshaling -out gen_ed.go
// ExecutableDataV1 structure described at https://github.com/ethereum/execution-apis/src/engine/specification.md
type ExecutableDataV1 struct {
	ParentHash    common.Hash    `json:"parentHash"    gencodec:"required"`
	FeeRecipient  common.Address `json:"feeRecipient"  gencodec:"required"`
	StateRoot     common.Hash    `json:"stateRoot"     gencodec:"required"`
	ReceiptsRoot  common.Hash    `json:"receiptsRoot"  gencodec:"required"`
	LogsBloom     []byte         `json:"logsBloom"     gencodec:"required"`
	PrevRandao    common.Hash    `json:"prevRandao"    gencodec:"required"`
	Number        uint64         `json:"blockNumber"   gencodec:"required"`
	GasLimit      uint64         `json:"gasLimit"      gencodec:"required"`
	GasUsed       uint64         `json:"gasUsed"       gencodec:"required"`
	Timestamp     uint64         `json:"timestamp"     gencodec:"required"`
	ExtraData     []byte         `json:"extraData"     gencodec:"required"`
	BaseFeePerGas *big.Int       `json:"baseFeePerGas" gencodec:"required"`
	BlockHash     common.Hash    `json:"blockHash"     gencodec:"required"`
	Transactions  [][]byte       `json:"transactions"  gencodec:"required"`
}

// JSON type overrides for executableData.
type executableDataMarshaling struct {
	Number        hexutil.Uint64
	GasLimit      hexutil.Uint64
	GasUsed       hexutil.Uint64
	Timestamp     hexutil.Uint64
	BaseFeePerGas *hexutil.Big
	ExtraData     hexutil.Bytes
	LogsBloom     hexutil.Bytes
	Transactions  []hexutil.Bytes
}

//go:generate go run github.com/fjl/gencodec -type PayloadAttributesV1 -field-override payloadAttributesMarshaling -out gen_blockparams.go
// PayloadAttributesV1 structure described at https://github.com/ethereum/execution-apis/pull/74
type PayloadAttributesV1 struct {
	Timestamp             uint64         `json:"timestamp"              gencodec:"required"`
	PrevRandao            common.Hash    `json:"prevRandao"             gencodec:"required"`
	SuggestedFeeRecipient common.Address `json:"suggestedFeeRecipient"  gencodec:"required"`
}

// JSON type overrides for PayloadAttributesV1.
type payloadAttributesMarshaling struct {
	Timestamp hexutil.Uint64
}

// JWT Tokens
func GetNewToken(jwtSecretBytes []byte, iat time.Time) (string, error) {
	newToken := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"iat": iat.Unix(),
	})
	tokenString, err := newToken.SignedString(jwtSecretBytes)
	if err != nil {
		return "", err
	}
	return tokenString, nil
}

func (ec *EngineClient) PrepareAuthCallToken(jwtSecretBytes []byte, iat time.Time) error {
	newTokenString, err := GetNewToken(jwtSecretBytes, iat)
	if err != nil {
		return err
	}
	ec.c.SetHeader("Authorization", fmt.Sprintf("Bearer %s", newTokenString))
	return nil
}

func (ec *EngineClient) PrepareDefaultAuthCallToken() error {
	secret, _ := hex.DecodeString("7365637265747365637265747365637265747365637265747365637265747365")
	ec.PrepareAuthCallToken(secret, time.Now())
	return nil
}

// Engine API Call Methods
func (ec *EngineClient) EngineForkchoiceUpdatedV1(fcState *ForkchoiceStateV1, pAttributes *PayloadAttributesV1) (ForkChoiceResponse, error) {
	var result ForkChoiceResponse
	if err := ec.PrepareDefaultAuthCallToken(); err != nil {
		return result, err
	}
	err := ec.c.CallContext(ec.Ctx(), &result, EngineForkchoiceUpdatedV1, fcState, pAttributes)
	return result, err
}

func (ec *EngineClient) EngineGetPayloadV1(payloadId *PayloadID) (ExecutableDataV1, error) {
	var result ExecutableDataV1
	if err := ec.PrepareDefaultAuthCallToken(); err != nil {
		return result, err
	}
	err := ec.c.CallContext(ec.Ctx(), &result, EngineGetPayloadV1, payloadId)
	return result, err
}

func (ec *EngineClient) EngineNewPayloadV1(payload *ExecutableDataV1) (PayloadStatusV1, error) {
	var result PayloadStatusV1
	if err := ec.PrepareDefaultAuthCallToken(); err != nil {
		return result, err
	}
	err := ec.c.CallContext(ec.Ctx(), &result, EngineNewPayloadV1, payload)
	return result, err
}

// Json helpers
// A value of this type can a JSON-RPC request, notification, successful response or
// error response. Which one it is depends on the fields.
type jsonError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

func (err *jsonError) Error() string {
	if err.Message == "" {
		return fmt.Sprintf("json-rpc error %d", err.Code)
	}
	return err.Message
}

type jsonrpcMessage struct {
	Version string          `json:"jsonrpc,omitempty"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Error   *jsonError      `json:"error,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
}

func UnmarshalFromJsonRPCResponse(b []byte, result interface{}) error {
	var rpcMessage jsonrpcMessage
	err := json.Unmarshal(b, &rpcMessage)
	if err != nil {
		return err
	}
	if rpcMessage.Error != nil {
		return rpcMessage.Error
	}
	return json.Unmarshal(rpcMessage.Result, &result)
}

func UnmarshalFromJsonRPCRequest(b []byte, params ...interface{}) error {
	var rpcMessage jsonrpcMessage
	err := json.Unmarshal(b, &rpcMessage)
	if err != nil {
		return err
	}
	if rpcMessage.Error != nil {
		return rpcMessage.Error
	}
	return json.Unmarshal(rpcMessage.Params, &params)
}
