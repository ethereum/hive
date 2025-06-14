package hive_rpc

import (
	"context"
	"fmt"
	"math/big"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum"
	api "github.com/ethereum/go-ethereum/beacon/engine"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/ethereum/hive/hivesim"
	"github.com/ethereum/hive/simulators/ethereum/engine/client"
	"github.com/ethereum/hive/simulators/ethereum/engine/globals"
	"github.com/ethereum/hive/simulators/ethereum/engine/helper"
	typ "github.com/ethereum/hive/simulators/ethereum/engine/types"
	"github.com/golang-jwt/jwt/v4"
	"github.com/pkg/errors"
)

type HiveRPCEngineStarter struct {
	// Client parameters used to launch the default client
	ClientType string
	ChainFile  string
	EnginePort int
	EthPort    int
	JWTSecret  []byte
}

var _ client.EngineStarter = (*HiveRPCEngineStarter)(nil)

func (s HiveRPCEngineStarter) StartClient(T *hivesim.T, testContext context.Context, genesis *core.Genesis, ClientParams hivesim.Params, ClientFiles hivesim.Params, bootClients ...client.EngineClient) (client.EngineClient, error) {
	var (
		clientType = s.ClientType
		enginePort = s.EnginePort
		ethPort    = s.EthPort
		jwtSecret  = s.JWTSecret
	)
	if clientType == "" {
		cs, err := T.Sim.ClientTypes()
		if err != nil {
			return nil, fmt.Errorf("client type was not supplied and simulator returned error on trying to get all client types: %v", err)
		}
		if len(cs) == 0 {
			return nil, fmt.Errorf("client type was not supplied and simulator returned empty client types: %v", cs)
		}
		clientType = cs[0].Name
	}
	if enginePort == 0 {
		enginePort = globals.EnginePortHTTP
	}
	if ethPort == 0 {
		ethPort = globals.EthPortHTTP
	}
	if jwtSecret == nil {
		jwtSecret = globals.DefaultJwtTokenSecretBytes
	}
	if s.ChainFile != "" {
		ClientFiles = ClientFiles.Set("/chain.rlp", "./chains/"+s.ChainFile)
	}
	if len(bootClients) > 0 {
		var (
			enodes = make([]string, len(bootClients))
			err    error
		)
		for i, bootClient := range bootClients {
			enodes[i], err = bootClient.EnodeURL()
			if err != nil {
				return nil, fmt.Errorf("unable to obtain bootnode: %v", err)
			}
		}
		enodeString := strings.Join(enodes, ",")
		ClientParams = ClientParams.Set("HIVE_BOOTNODE", enodeString)
	}

	// Start the client and create the engine client object
	genesisStart, err := helper.GenesisStartOption(genesis)
	if err != nil {
		return nil, err
	}
	c := T.StartClient(clientType, genesisStart, ClientParams, hivesim.WithStaticFiles(ClientFiles))
	if err := CheckEthEngineLive(c); err != nil {
		return nil, fmt.Errorf("Engine/Eth ports were never open for client: %v", err)
	}
	hiveLogLevel, _ := strconv.Atoi(os.Getenv("HIVE_LOGLEVEL"))
	ec := NewHiveRPCEngineClient(c, enginePort, ethPort, jwtSecret, &helper.LoggingRoundTrip{
		Logger:   T,
		ID:       c.Container,
		Inner:    http.DefaultTransport,
		LogLevel: hiveLogLevel,
	})
	return ec, nil
}

func CheckEthEngineLive(c *hivesim.Client) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()
	var (
		ticker = time.NewTicker(100 * time.Millisecond)
		dialer net.Dialer
	)
	defer ticker.Stop()
	for _, checkport := range []int{globals.EthPortHTTP, globals.EnginePortHTTP} {
		addr := fmt.Sprintf("%s:%d", c.IP, checkport)
	portcheckloop:
		for {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-ticker.C:
				conn, err := dialer.DialContext(ctx, "tcp", addr)
				if err == nil {
					conn.Close()
					break portcheckloop
				}
			}
		}
	}
	return nil
}

type AccountTransactionInfo struct {
	PreviousBlock common.Hash
	PreviousNonce uint64
}

// Implements the EngineClient interface for a normal RPC client.
type HiveRPCEngineClient struct {
	*ethclient.Client
	h              *hivesim.Client
	c              *rpc.Client
	cEth           *rpc.Client
	JWTSecretBytes []byte

	// Engine updates info
	latestFcUStateSent *api.ForkchoiceStateV1
	latestPAttrSent    *typ.PayloadAttributes
	latestFcUResponse  *api.ForkChoiceResponse

	latestPayloadSent          *typ.ExecutableData
	latestPayloadStatusReponse *api.PayloadStatusV1

	// Test account nonces
	accTxInfoMap     map[common.Address]*AccountTransactionInfo
	accTxInfoMapLock sync.Mutex
}

var _ client.EngineClient = (*HiveRPCEngineClient)(nil)

// NewClient creates a engine client that uses the given RPC client.
func NewHiveRPCEngineClient(h *hivesim.Client, enginePort int, ethPort int, jwtSecretBytes []byte, transport http.RoundTripper) *HiveRPCEngineClient {
	// Prepare HTTP Client
	httpClient := rpc.WithHTTPClient(&http.Client{Transport: transport})

	engineRpcClient, err := rpc.DialOptions(context.Background(), fmt.Sprintf("http://%s:%d/", h.IP, enginePort), httpClient)
	if err != nil {
		panic(err)
	}

	// Prepare ETH Client
	ethRpcClient, err := rpc.DialOptions(context.Background(), fmt.Sprintf("http://%s:%d/", h.IP, ethPort), httpClient)
	if err != nil {
		panic(err)
	}
	eth := ethclient.NewClient(ethRpcClient)
	return &HiveRPCEngineClient{
		h:              h,
		c:              engineRpcClient,
		Client:         eth,
		cEth:           ethRpcClient,
		JWTSecretBytes: jwtSecretBytes,
		accTxInfoMap:   make(map[common.Address]*AccountTransactionInfo),
	}
}

func (ec *HiveRPCEngineClient) ID() string {
	return ec.h.Container
}

func (ec *HiveRPCEngineClient) EnodeURL() (string, error) {
	return ec.h.EnodeURL()
}

var (
	Head      *big.Int // Nil
	Pending   = big.NewInt(-2)
	Finalized = big.NewInt(-3)
	Safe      = big.NewInt(-4)
)

// Custom toBlockNumArg to test `safe` and `finalized`
func toBlockNumArg(number *big.Int) string {
	if number == nil {
		return "latest"
	}
	if number.Cmp(Pending) == 0 {
		return "pending"
	}
	if number.Cmp(Finalized) == 0 {
		return "finalized"
	}
	if number.Cmp(Safe) == 0 {
		return "safe"
	}
	return hexutil.EncodeBig(number)
}

func (ec *HiveRPCEngineClient) StorageAtKeys(ctx context.Context, account common.Address, keys []common.Hash, blockNumber *big.Int) (map[common.Hash]*common.Hash, error) {
	reqs := make([]rpc.BatchElem, 0, len(keys))
	results := make(map[common.Hash]*common.Hash, len(keys))
	var blockNumberString string
	if blockNumber == nil {
		blockNumberString = "latest"
	} else {
		blockNumberString = hexutil.EncodeBig(blockNumber)
	}
	for _, key := range keys {
		valueResult := &common.Hash{}
		reqs = append(reqs, rpc.BatchElem{
			Method: "eth_getStorageAt",
			Args:   []interface{}{account, key, blockNumberString},
			Result: valueResult,
		})
		results[key] = valueResult
	}

	if err := ec.cEth.BatchCallContext(ctx, reqs); err != nil {
		return nil, err
	}
	for i, req := range reqs {
		if req.Error != nil {
			return nil, errors.Wrap(req.Error, fmt.Sprintf("request for storage at index %d failed", i))
		}
	}

	return results, nil
}

func (ec *HiveRPCEngineClient) HeaderByNumber(ctx context.Context, number *big.Int) (*types.Header, error) {
	var header *types.Header
	err := ec.cEth.CallContext(ctx, &header, "eth_getBlockByNumber", toBlockNumArg(number), false)
	if err == nil && header == nil {
		err = ethereum.NotFound
	}
	return header, err
}

func (ec *HiveRPCEngineClient) Close() error {
	ec.c.Close()
	ec.Client.Close()
	return nil
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

func (ec *HiveRPCEngineClient) PrepareAuthCallToken(jwtSecretBytes []byte, iat time.Time) error {
	newTokenString, err := GetNewToken(jwtSecretBytes, iat)
	if err != nil {
		return err
	}
	ec.c.SetHeader("Authorization", fmt.Sprintf("Bearer %s", newTokenString))
	return nil
}

func (ec *HiveRPCEngineClient) PrepareDefaultAuthCallToken() error {
	ec.PrepareAuthCallToken(ec.JWTSecretBytes, time.Now())
	return nil
}

// Engine API Call Methods

// Forkchoice Updated API Calls
func (ec *HiveRPCEngineClient) ForkchoiceUpdated(ctx context.Context, version int, fcState *api.ForkchoiceStateV1, pAttributes *typ.PayloadAttributes) (api.ForkChoiceResponse, error) {
	var result api.ForkChoiceResponse
	if err := ec.PrepareDefaultAuthCallToken(); err != nil {
		return result, err
	}
	ec.latestFcUStateSent = fcState
	ec.latestPAttrSent = pAttributes
	err := ec.c.CallContext(ctx,
		&result,
		fmt.Sprintf("engine_forkchoiceUpdatedV%d", version),
		fcState,
		pAttributes)
	ec.latestFcUResponse = &result
	return result, err
}

func (ec *HiveRPCEngineClient) ForkchoiceUpdatedV1(ctx context.Context, fcState *api.ForkchoiceStateV1, pAttributes *typ.PayloadAttributes) (api.ForkChoiceResponse, error) {
	return ec.ForkchoiceUpdated(ctx, 1, fcState, pAttributes)
}

func (ec *HiveRPCEngineClient) ForkchoiceUpdatedV2(ctx context.Context, fcState *api.ForkchoiceStateV1, pAttributes *typ.PayloadAttributes) (api.ForkChoiceResponse, error) {
	return ec.ForkchoiceUpdated(ctx, 2, fcState, pAttributes)
}

func (ec *HiveRPCEngineClient) ForkchoiceUpdatedV3(ctx context.Context, fcState *api.ForkchoiceStateV1, pAttributes *typ.PayloadAttributes) (api.ForkChoiceResponse, error) {
	return ec.ForkchoiceUpdated(ctx, 3, fcState, pAttributes)
}

// Get Payload API Calls

func (ec *HiveRPCEngineClient) GetPayload(ctx context.Context, version int, payloadId *api.PayloadID) (typ.ExecutableData, *big.Int, *typ.BlobsBundle, *bool, error) {
	var (
		executableData        typ.ExecutableData
		blockValue            *big.Int
		blobsBundle           *typ.BlobsBundle
		shouldOverrideBuilder *bool
		err                   error
		rpcString             = fmt.Sprintf("engine_getPayloadV%d", version)
	)

	if err = ec.PrepareDefaultAuthCallToken(); err != nil {
		return executableData, nil, nil, nil, err
	}

	if version >= 2 {
		var response typ.ExecutionPayloadEnvelope
		err = ec.c.CallContext(ctx, &response, rpcString, payloadId)
		if response.ExecutionPayload != nil {
			executableData = *response.ExecutionPayload
		}
		blockValue = response.BlockValue
		blobsBundle = response.BlobsBundle
		shouldOverrideBuilder = response.ShouldOverrideBuilder
	} else {
		err = ec.c.CallContext(ctx, &executableData, rpcString, payloadId)
	}

	return executableData, blockValue, blobsBundle, shouldOverrideBuilder, err
}

func (ec *HiveRPCEngineClient) GetPayloadV1(ctx context.Context, payloadId *api.PayloadID) (typ.ExecutableData, error) {
	ed, _, _, _, err := ec.GetPayload(ctx, 1, payloadId)
	return ed, err
}

func (ec *HiveRPCEngineClient) GetPayloadV2(ctx context.Context, payloadId *api.PayloadID) (typ.ExecutableData, *big.Int, error) {
	ed, bv, _, _, err := ec.GetPayload(ctx, 2, payloadId)
	return ed, bv, err
}

func (ec *HiveRPCEngineClient) GetPayloadV3(ctx context.Context, payloadId *api.PayloadID) (typ.ExecutableData, *big.Int, *typ.BlobsBundle, *bool, error) {
	return ec.GetPayload(ctx, 3, payloadId)
}

// Get Payload Bodies API Calls
func (ec *HiveRPCEngineClient) GetPayloadBodiesByRangeV1(ctx context.Context, start uint64, count uint64) ([]*typ.ExecutionPayloadBodyV1, error) {
	var (
		result []*typ.ExecutionPayloadBodyV1
		err    error
	)
	if err = ec.PrepareDefaultAuthCallToken(); err != nil {
		return nil, err
	}

	err = ec.c.CallContext(ctx, &result, "engine_getPayloadBodiesByRangeV1", hexutil.Uint64(start), hexutil.Uint64(count))
	return result, err
}

func (ec *HiveRPCEngineClient) GetPayloadBodiesByHashV1(ctx context.Context, hashes []common.Hash) ([]*typ.ExecutionPayloadBodyV1, error) {
	var (
		result []*typ.ExecutionPayloadBodyV1
		err    error
	)
	if err = ec.PrepareDefaultAuthCallToken(); err != nil {
		return nil, err
	}

	err = ec.c.CallContext(ctx, &result, "engine_getPayloadBodiesByHashV1", hashes)
	return result, err
}

// Get Blob Bundle API Calls
func (ec *HiveRPCEngineClient) GetBlobsBundleV1(ctx context.Context, payloadId *api.PayloadID) (*typ.BlobsBundle, error) {
	var (
		result typ.BlobsBundle
		err    error
	)
	if err = ec.PrepareDefaultAuthCallToken(); err != nil {
		return nil, err
	}

	err = ec.c.CallContext(ctx, &result, "engine_getBlobsBundleV1", payloadId)
	return &result, err
}

// New Payload API Call Methods
func (ec *HiveRPCEngineClient) NewPayload(ctx context.Context, version int, payload *typ.ExecutableData) (result api.PayloadStatusV1, err error) {
	if err := ec.PrepareDefaultAuthCallToken(); err != nil {
		return result, err
	}

	if version >= 3 {
		err = ec.c.CallContext(ctx, &result, fmt.Sprintf("engine_newPayloadV%d", version), payload, payload.VersionedHashes, payload.ParentBeaconBlockRoot)
	} else {
		err = ec.c.CallContext(ctx, &result, fmt.Sprintf("engine_newPayloadV%d", version), payload)
	}
	ec.latestPayloadStatusReponse = &result
	return result, err
}

func (ec *HiveRPCEngineClient) NewPayloadV1(ctx context.Context, payload *typ.ExecutableData) (api.PayloadStatusV1, error) {
	ec.latestPayloadSent = payload
	return ec.NewPayload(ctx, 1, payload)
}

func (ec *HiveRPCEngineClient) NewPayloadV2(ctx context.Context, payload *typ.ExecutableData) (api.PayloadStatusV1, error) {
	ec.latestPayloadSent = payload
	return ec.NewPayload(ctx, 2, payload)
}

func (ec *HiveRPCEngineClient) NewPayloadV3(ctx context.Context, payload *typ.ExecutableData) (api.PayloadStatusV1, error) {
	ec.latestPayloadSent = payload
	return ec.NewPayload(ctx, 3, payload)
}

// Exchange Transition Configuration API Call Methods
func (ec *HiveRPCEngineClient) ExchangeTransitionConfigurationV1(ctx context.Context, tConf *api.TransitionConfigurationV1) (api.TransitionConfigurationV1, error) {
	var result api.TransitionConfigurationV1
	err := ec.c.CallContext(ctx, &result, "engine_exchangeTransitionConfigurationV1", tConf)
	return result, err
}

func (ec *HiveRPCEngineClient) ExchangeCapabilities(ctx context.Context, clCapabilities []string) ([]string, error) {
	var result []string
	if err := ec.PrepareDefaultAuthCallToken(); err != nil {
		return result, err
	}
	err := ec.c.CallContext(ctx, &result, "engine_exchangeCapabilities", clCapabilities)
	return result, err
}

// Account Nonce
func (ec *HiveRPCEngineClient) GetLastAccountNonce(testCtx context.Context, account common.Address, head *types.Header) (uint64, error) {
	// First get the current head of the client where we will send the tx
	if head == nil {
		ctx, cancel := context.WithTimeout(testCtx, globals.RPCTimeout)
		defer cancel()
		var err error
		head, err = ec.HeaderByNumber(ctx, nil)
		if err != nil {
			return 0, err
		}
	}

	// Then check if we have any info about this account, and when it was last updated
	if accTxInfo, ok := ec.accTxInfoMap[account]; ok && accTxInfo != nil && (accTxInfo.PreviousBlock == head.Hash() || accTxInfo.PreviousBlock == head.ParentHash) {
		// We have info about this account and is up to date (or up to date until the very last block).
		// Return the previous nonce
		return accTxInfo.PreviousNonce, nil
	}
	// We don't have info about this account, so there is no previous nonce
	return 0, fmt.Errorf("no previous nonce for account %s", account.String())
}

func (ec *HiveRPCEngineClient) GetNextAccountNonce(testCtx context.Context, account common.Address, head *types.Header) (uint64, error) {
	// First get the current head of the client where we will send the tx
	if head == nil {
		ctx, cancel := context.WithTimeout(testCtx, globals.RPCTimeout)
		defer cancel()
		var err error
		head, err = ec.HeaderByNumber(ctx, nil)
		if err != nil {
			return 0, err
		}
	}
	// Then check if we have any info about this account, and when it was last updated
	if accTxInfo, ok := ec.accTxInfoMap[account]; ok && accTxInfo != nil && (accTxInfo.PreviousBlock == head.Hash() || accTxInfo.PreviousBlock == head.ParentHash) {
		// We have info about this account and is up to date (or up to date until the very last block).
		// Increase the nonce and return it
		accTxInfo.PreviousBlock = head.Hash()
		accTxInfo.PreviousNonce++
		return accTxInfo.PreviousNonce, nil
	}
	// We don't have info about this account, or is outdated, or we re-org'd, we must request the nonce
	ctx, cancel := context.WithTimeout(testCtx, globals.RPCTimeout)
	defer cancel()
	nonce, err := ec.NonceAt(ctx, account, head.Number)
	if err != nil {
		return 0, err
	}
	ec.accTxInfoMapLock.Lock()
	defer ec.accTxInfoMapLock.Unlock()
	ec.accTxInfoMap[account] = &AccountTransactionInfo{
		PreviousBlock: head.Hash(),
		PreviousNonce: nonce,
	}
	return nonce, nil
}

func (ec *HiveRPCEngineClient) UpdateNonce(testCtx context.Context, account common.Address, newNonce uint64) error {
	// First get the current head of the client where we will send the tx
	ctx, cancel := context.WithTimeout(testCtx, globals.RPCTimeout)
	defer cancel()
	head, err := ec.HeaderByNumber(ctx, nil)
	if err != nil {
		return err
	}
	ec.accTxInfoMap[account] = &AccountTransactionInfo{
		PreviousBlock: head.Hash(),
		PreviousNonce: newNonce,
	}
	return nil
}

func (ec *HiveRPCEngineClient) SendTransaction(ctx context.Context, tx typ.Transaction) error {
	data, err := tx.MarshalBinary()
	if err != nil {
		return err
	}
	return ec.cEth.CallContext(ctx, nil, "eth_sendRawTransaction", hexutil.Encode(data))
}

func (ec *HiveRPCEngineClient) SendTransactions(ctx context.Context, txs ...typ.Transaction) []error {
	reqs := make([]rpc.BatchElem, len(txs))
	hashes := make([]common.Hash, len(txs))
	for i := range reqs {
		data, err := txs[i].MarshalBinary()
		if err != nil {
			return []error{err}
		}
		reqs[i] = rpc.BatchElem{
			Method: "eth_sendRawTransaction",
			Args:   []interface{}{hexutil.Encode(data)},
			Result: &hashes[i],
		}
	}
	if err := ec.cEth.BatchCallContext(ctx, reqs); err != nil {
		return []error{err}
	}

	errs := make([]error, len(txs))
	for i := range reqs {
		errs[i] = reqs[i].Error
	}
	return nil
}

func (ec *HiveRPCEngineClient) PostRunVerifications() error {
	// There are no post run verifications for RPC clients yet
	return nil
}

func (ec *HiveRPCEngineClient) LatestForkchoiceSent() (fcState *api.ForkchoiceStateV1, pAttributes *typ.PayloadAttributes) {
	return ec.latestFcUStateSent, ec.latestPAttrSent
}

func (ec *HiveRPCEngineClient) LatestNewPayloadSent() *typ.ExecutableData {
	return ec.latestPayloadSent
}

func (ec *HiveRPCEngineClient) LatestForkchoiceResponse() *api.ForkChoiceResponse {
	return ec.latestFcUResponse
}
func (ec *HiveRPCEngineClient) LatestNewPayloadResponse() *api.PayloadStatusV1 {
	return ec.latestPayloadStatusReponse
}
