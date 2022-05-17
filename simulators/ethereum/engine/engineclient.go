package main

import (
	"context"
	"fmt"
	"math/big"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/ethereum/hive/hivesim"
	"github.com/golang-jwt/jwt/v4"
)

// https://github.com/ethereum/execution-apis/blob/v1.0.0-alpha.7/src/engine/specification.md
var EthPortHTTP = 8545
var EnginePortHTTP = 8551

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

type EngineClients []*EngineClient

// NewClient creates a engine client that uses the given RPC client.
func NewEngineClient(t *hivesim.T, hc *hivesim.Client, ttd *big.Int) *EngineClient {
	client := &http.Client{
		Transport: &loggingRoundTrip{
			t:     t,
			hc:    hc,
			inner: http.DefaultTransport,
		},
	}
	// Prepare HTTP Client
	rpcHttpClient, _ := rpc.DialHTTPWithClient(fmt.Sprintf("http://%v:%v/", hc.IP, EnginePortHTTP), client)

	// Prepare ETH Client
	client = &http.Client{
		Transport: &loggingRoundTrip{
			t:     t,
			hc:    hc,
			inner: http.DefaultTransport,
		},
	}
	rpcClient, _ := rpc.DialHTTPWithClient(fmt.Sprintf("http://%v:%v/", hc.IP, EthPortHTTP), client)
	eth := ethclient.NewClient(rpcClient)
	return &EngineClient{
		T:                       t,
		Client:                  hc,
		c:                       rpcHttpClient,
		Eth:                     eth,
		cEth:                    rpcClient,
		IP:                      hc.IP,
		TerminalTotalDifficulty: ttd,
		lastCtx:                 nil,
		lastCancel:              nil,
	}
}

func (ec *EngineClient) Equals(ec2 *EngineClient) bool {
	return ec.Container == ec2.Container
}

func (ec *EngineClient) checkTTD() bool {
	var td *TotalDifficultyHeader
	if err := ec.cEth.CallContext(ec.Ctx(), &td, "eth_getBlockByNumber", "latest", false); err != nil {
		panic(err)
	}
	return td.TotalDifficulty.ToInt().Cmp(ec.TerminalTotalDifficulty) >= 0
}

// Wait until the TTD is reached by a single client.
func (ec *EngineClient) waitForTTD(wg *sync.WaitGroup, done chan<- *EngineClient, cancel <-chan interface{}) {
	defer wg.Done()
	for {
		select {
		case <-time.After(tTDCheckPeriod):
			if ec.checkTTD() {
				select {
				case done <- ec:
				case <-cancel:
				}
				return
			}
		case <-cancel:
			return
		}
	}
}

// Wait until the TTD is reached by a single client with a timeout.
// Returns true if the TTD has been reached, false when timeout occurred.
func (ec *EngineClient) waitForTTDWithTimeout(timeout <-chan time.Time) bool {
	for {
		select {
		case <-time.After(tTDCheckPeriod):
			if ec.checkTTD() {
				return true
			}
		case <-timeout:
			return false
		}
	}
}

// Wait until the TTD is reached by any of the engine clients
func (ecs EngineClients) waitForTTD(timeout <-chan time.Time) *EngineClient {
	var wg sync.WaitGroup
	defer wg.Wait()
	done := make(chan *EngineClient)
	cancel := make(chan interface{})
	defer close(cancel)
	for _, ec := range ecs {
		wg.Add(1)
		go ec.waitForTTD(&wg, done, cancel)
	}
	select {
	case ec := <-done:
		return ec
	case <-timeout:
		return nil
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
	ec.lastCtx, ec.lastCancel = context.WithTimeout(context.Background(), rpcTimeout)
	return ec.lastCtx
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
type PayloadStatusSlice []PayloadStatus

const (
	Valid PayloadStatus = iota
	Invalid
	Accepted
	Syncing
	InvalidTerminalBlock
	InvalidBlockHash
)

var PayloadStatuses = map[PayloadStatus]string{
	Valid:                "VALID",
	Invalid:              "INVALID",
	Accepted:             "ACCEPTED",
	Syncing:              "SYNCING",
	InvalidTerminalBlock: "INVALID_TERMINAL_BLOCK",
	InvalidBlockHash:     "INVALID_BLOCK_HASH",
}

func (b PayloadStatus) String() string {
	str, ok := PayloadStatuses[b]
	if !ok {
		return "UNKNOWN"
	}
	return str
}

func (b PayloadStatusSlice) String() string {
	names := make([]string, 0)
	for _, status := range b {
		names = append(names, status.String())
	}
	return strings.Join(names, "|")
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

//go:generate go run github.com/fjl/gencodec -type TransitionConfigurationV1 -field-override TransitionConfigurationV1Marshaling -out gen_transitionconf.go
type TransitionConfigurationV1 struct {
	TerminalTotalDifficulty *big.Int     `json:"terminalTotalDifficulty" gencodec:"required"`
	TerminalBlockHash       *common.Hash `json:"terminalBlockHash"       gencodec:"required"`
	TerminalBlockNumber     uint64       `json:"terminalBlockNumber"     gencodec:"required"`
}

// JSON type overrides for TransitionConfigurationV1.
type TransitionConfigurationV1Marshaling struct {
	TerminalTotalDifficulty *hexutil.Big   `json:"terminalTotalDifficulty"`
	TerminalBlockNumber     hexutil.Uint64 `json:"terminalBlockNumber"`
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
	ec.PrepareAuthCallToken(defaultJwtTokenSecretBytes, time.Now())
	return nil
}

// Engine API Call Methods
func (ec *EngineClient) EngineForkchoiceUpdatedV1(ctx context.Context, fcState *ForkchoiceStateV1, pAttributes *PayloadAttributesV1) (ForkChoiceResponse, error) {
	var result ForkChoiceResponse
	if err := ec.PrepareDefaultAuthCallToken(); err != nil {
		return result, err
	}
	err := ec.c.CallContext(ctx, &result, "engine_forkchoiceUpdatedV1", fcState, pAttributes)
	return result, err
}

func (ec *EngineClient) EngineGetPayloadV1(ctx context.Context, payloadId *PayloadID) (ExecutableDataV1, error) {
	var result ExecutableDataV1
	if err := ec.PrepareDefaultAuthCallToken(); err != nil {
		return result, err
	}
	err := ec.c.CallContext(ctx, &result, "engine_getPayloadV1", payloadId)
	return result, err
}

func (ec *EngineClient) EngineNewPayloadV1(ctx context.Context, payload *ExecutableDataV1) (PayloadStatusV1, error) {
	var result PayloadStatusV1
	if err := ec.PrepareDefaultAuthCallToken(); err != nil {
		return result, err
	}
	err := ec.c.CallContext(ctx, &result, "engine_newPayloadV1", payload)
	return result, err
}

func (ec *EngineClient) EngineExchangeTransitionConfigurationV1(ctx context.Context, tConf *TransitionConfigurationV1) (TransitionConfigurationV1, error) {
	var result TransitionConfigurationV1
	if err := ec.PrepareDefaultAuthCallToken(); err != nil {
		return result, err
	}
	err := ec.c.CallContext(ctx, &result, "engine_exchangeTransitionConfigurationV1", tConf)
	return result, err
}
