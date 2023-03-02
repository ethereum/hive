package clients

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	api "github.com/ethereum/go-ethereum/beacon/engine"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/ethereum/hive/simulators/eth2/common/spoofing/proxy"
	"github.com/ethereum/hive/simulators/eth2/common/utils"
	"github.com/golang-jwt/jwt/v4"
	spoof "github.com/rauljordan/engine-proxy/proxy"
)

const (
	PortUserRPC   = 8545
	PortEngineRPC = 8551
)

var AllForkchoiceUpdatedCalls = []string{
	"engine_forkchoiceUpdatedV1",
	"engine_forkchoiceUpdatedV2",
}

var AllEngineCallsLog = []string{
	"engine_forkchoiceUpdatedV1",
	"engine_forkchoiceUpdatedV2",
	"engine_getPayloadV1",
	"engine_getPayloadV2",
	"engine_newPayloadV1",
	"engine_newPayloadV2",
}

type ExecutionProxyConfig struct {
	Host                   net.IP
	Port                   int
	LogEngineCalls         bool
	TrackForkchoiceUpdated bool
}

type ExecutionClientConfig struct {
	ClientIndex             int
	ProxyConfig             *ExecutionProxyConfig
	TerminalTotalDifficulty int64
	EngineAPIPort           int
	RPCPort                 int
	Subnet                  string
	JWTSecret               []byte
}

type ExecutionClient struct {
	Client
	Logger utils.Logging
	Config ExecutionClientConfig

	proxy     *proxy.Proxy
	latestfcu *api.ForkchoiceStateV1

	engineRpcClient *rpc.Client
	ethRpcClient    *rpc.Client
	eth             *ethclient.Client

	startupComplete bool
}

func (en *ExecutionClient) Logf(format string, values ...interface{}) {
	if l := en.Logger; l != nil {
		l.Logf(format, values...)
	}
}

func (en *ExecutionClient) UserRPCAddress() (string, error) {
	if !en.Client.IsRunning() {
		return "", fmt.Errorf("execution client not yet launched")
	}
	var port = PortUserRPC
	if en.Config.RPCPort != 0 {
		port = en.Config.RPCPort
	}
	return fmt.Sprintf(
		"http://%v:%d",
		en.Client.GetIP(),
		port,
	), nil
}

func (en *ExecutionClient) EngineRPCAddress() (string, error) {
	var port = PortEngineRPC
	if en.Config.EngineAPIPort != 0 {
		port = en.Config.EngineAPIPort
	}
	return fmt.Sprintf(
		"http://%v:%d",
		en.Client.GetIP(),
		port,
	), nil
}

func (en *ExecutionClient) MustGetEnode() string {
	if enodeClient, ok := en.Client.(EnodeClient); ok {
		addr, err := enodeClient.GetEnodeURL()
		if err == nil {
			return addr
		}
		panic(err)
	}
	panic(fmt.Errorf("invalid client type"))
}

func (en *ExecutionClient) ConfiguredTTD() *big.Int {
	return big.NewInt(en.Config.TerminalTotalDifficulty)
}

func (en *ExecutionClient) Start() error {
	if !en.Client.IsRunning() {
		if managedClient, ok := en.Client.(ManagedClient); !ok {
			return fmt.Errorf("attempted to start an unmanaged client")
		} else {
			if err := managedClient.Start(); err != nil {
				return err
			}
		}
	}

	return en.Init(context.Background())
}

func (en *ExecutionClient) Init(ctx context.Context) error {
	if !en.startupComplete {
		defer func() {
			en.startupComplete = true
		}()

		// Prepare Eth/Engine RPCs
		engineRPCAddress, err := en.EngineRPCAddress()
		if err != nil {
			return err
		}
		client := &http.Client{}
		// Prepare HTTP Client
		en.engineRpcClient, err = rpc.DialHTTPWithClient(
			engineRPCAddress,
			client,
		)
		if err != nil {
			return err
		}

		// Prepare ETH Client
		client = &http.Client{}

		userRPCAddress, err := en.UserRPCAddress()
		if err != nil {
			return err
		}
		en.ethRpcClient, err = rpc.DialHTTPWithClient(userRPCAddress, client)
		if err != nil {
			return err
		}
		en.eth = ethclient.NewClient(en.ethRpcClient)

		// Prepare proxy
		dest, err := en.EngineRPCAddress()
		if err != nil {
			return err
		}

		if en.Config.ProxyConfig != nil {
			p := proxy.NewProxy(
				en.Config.ProxyConfig.Host,
				en.Config.ProxyConfig.Port,
				dest,
				en.Config.JWTSecret,
			)

			if en.Config.ProxyConfig.TrackForkchoiceUpdated {
				logCallback := func(req []byte) *spoof.Spoof {
					var (
						fcState api.ForkchoiceStateV1
						pAttr   api.PayloadAttributes
						err     error
					)
					err = proxy.UnmarshalFromJsonRPCRequest(
						req,
						&fcState,
						&pAttr,
					)
					if err == nil {
						en.latestfcu = &fcState
					}
					return nil
				}
				for _, c := range AllForkchoiceUpdatedCalls {
					p.AddRequestCallback(c, logCallback)
				}
			}

			if en.Config.ProxyConfig.LogEngineCalls {
				logCallback := func(res []byte, req []byte) *spoof.Spoof {
					en.Logf(
						"DEBUG: execution client %d, request: %s",
						en.Config.ClientIndex,
						req,
					)
					en.Logf(
						"DEBUG: execution client %d, response: %s",
						en.Config.ClientIndex,
						res,
					)
					return nil
				}
				for _, c := range AllEngineCallsLog {
					p.AddResponseCallback(c, logCallback)
				}
			}

			en.proxy = p
		}

	}
	return nil
}

func (en *ExecutionClient) Shutdown() error {
	if managedClient, ok := en.Client.(ManagedClient); !ok {
		return fmt.Errorf("attempted to shutdown an unmanaged client")
	} else {
		return managedClient.Shutdown()
	}
}

func (en *ExecutionClient) IsRunning() bool {
	return en.Client.IsRunning()
}

func (en *ExecutionClient) Proxy() *proxy.Proxy {
	return en.proxy
}

func (en *ExecutionClient) GetLatestForkchoiceUpdated(
	ctx context.Context,
) (*api.ForkchoiceStateV1, error) {
	if en.latestfcu != nil {
		return en.latestfcu, nil
	}
	// Try to reconstruct by querying it from the client
	forkchoiceState := &api.ForkchoiceStateV1{}
	errs := make(chan error, 3)
	var wg sync.WaitGroup

	type labelBlockHashTask struct {
		label string
		dest  *common.Hash
	}

	for _, t := range []*labelBlockHashTask{
		{
			label: "latest",
			dest:  &forkchoiceState.HeadBlockHash,
		},
		{
			label: "safe",
			dest:  &forkchoiceState.SafeBlockHash,
		},
		{
			label: "finalized",
			dest:  &forkchoiceState.FinalizedBlockHash,
		},
	} {
		wg.Add(1)
		t := t
		go func(t *labelBlockHashTask) {
			defer wg.Done()
			if res, err := en.HeaderByLabel(
				ctx,
				t.label,
			); err != nil {
				en.Logf(
					"Error trying to fetch label %s from client: %v",
					t.label,
					err,
				)
			} else if err == nil && res != nil && res.Number != nil {
				*t.dest = res.Hash()
			}
		}(t)
	}
	wg.Wait()

	select {
	case err := <-errs:
		return nil, err
	default:
	}

	return forkchoiceState, nil
}

// Engine API

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

func (en *ExecutionClient) PrepareAuthCallToken(
	jwtSecretBytes []byte,
	iat time.Time,
) error {
	newTokenString, err := GetNewToken(jwtSecretBytes, iat)
	if err != nil {
		return err
	}
	en.engineRpcClient.SetHeader(
		"Authorization",
		fmt.Sprintf("Bearer %s", newTokenString),
	)
	return nil
}

func (en *ExecutionClient) PrepareDefaultAuthCallToken() error {
	en.PrepareAuthCallToken(en.Config.JWTSecret, time.Now())
	return nil
}

func (en *ExecutionClient) EngineForkchoiceUpdated(
	parentCtx context.Context,
	fcState *api.ForkchoiceStateV1,
	pAttributes *api.PayloadAttributes,
	version int,
) (*api.ForkChoiceResponse, error) {
	var result api.ForkChoiceResponse
	if err := en.PrepareDefaultAuthCallToken(); err != nil {
		return nil, err
	}
	request := fmt.Sprintf("engine_forkchoiceUpdatedV%d", version)
	ctx, cancel := context.WithTimeout(parentCtx, time.Second*10)
	defer cancel()
	err := en.engineRpcClient.CallContext(
		ctx,
		&result,
		request,
		fcState,
		pAttributes,
	)
	return &result, err
}

func (en *ExecutionClient) EngineGetPayload(
	parentCtx context.Context,
	payloadID *api.PayloadID,
	version int,
) (*api.ExecutableData, *big.Int, error) {

	var (
		rpcString = fmt.Sprintf("engine_getPayloadV%d", version)
	)

	if err := en.PrepareDefaultAuthCallToken(); err != nil {
		return nil, nil, err
	}

	ctx, cancel := context.WithTimeout(parentCtx, time.Second*10)
	defer cancel()
	if version == 2 {
		type ExecutionPayloadEnvelope struct {
			ExecutionPayload *api.ExecutableData `json:"executionPayload" gencodec:"required"`
			BlockValue       *hexutil.Big        `json:"blockValue"       gencodec:"required"`
		}
		var response ExecutionPayloadEnvelope
		err := en.engineRpcClient.CallContext(
			ctx,
			&response,
			rpcString,
			payloadID,
		)
		return response.ExecutionPayload, (*big.Int)(response.BlockValue), err
	} else {
		var executableData api.ExecutableData
		err := en.engineRpcClient.CallContext(ctx, &executableData, rpcString, payloadID)
		return &executableData, common.Big0, err
	}
}

func (en *ExecutionClient) EngineNewPayload(
	parentCtx context.Context,
	payload *api.ExecutableData,
	version int,
) (*api.PayloadStatusV1, error) {
	var result api.PayloadStatusV1
	if err := en.PrepareDefaultAuthCallToken(); err != nil {
		return nil, err
	}
	request := fmt.Sprintf("engine_newPayloadV%d", version)
	ctx, cancel := context.WithTimeout(parentCtx, time.Second*10)
	defer cancel()
	err := en.engineRpcClient.CallContext(ctx, &result, request, payload)
	return &result, err
}

// Eth RPC
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

func (en *ExecutionClient) TotalDifficultyByNumber(
	parentCtx context.Context,
	blockNumber *big.Int,
) (*big.Int, error) {
	var td *TotalDifficultyHeader
	ctx, cancel := utils.ContextTimeoutRPC(parentCtx)
	defer cancel()
	var blockId string
	if blockNumber == nil {
		blockId = "latest"
	} else {
		blockId = fmt.Sprintf("%d", blockNumber)
	}
	if err := en.ethRpcClient.CallContext(ctx, &td, "eth_getBlockByNumber", blockId, false); err == nil {
		return td.TotalDifficulty.ToInt(), nil
	} else {
		return nil, err
	}
}

func (en *ExecutionClient) TotalDifficultyByHash(
	parentCtx context.Context,
	blockHash common.Hash,
) (*big.Int, error) {
	var td *TotalDifficultyHeader
	ctx, cancel := utils.ContextTimeoutRPC(parentCtx)
	defer cancel()
	if err := en.ethRpcClient.CallContext(ctx, &td, "eth_getBlockByHash", fmt.Sprintf("%s", blockHash), false); err == nil {
		return td.TotalDifficulty.ToInt(), nil
	} else {
		return nil, err
	}
}

func (ec *ExecutionClient) CheckTTD(parentCtx context.Context) (bool, error) {
	td, err := ec.TotalDifficultyByNumber(parentCtx, nil)
	if err != nil {
		return false, err
	}
	if td.Cmp(big.NewInt(ec.Config.TerminalTotalDifficulty)) >= 0 {
		return true, nil
	}
	return false, nil
}

func (ec *ExecutionClient) WaitForTerminalTotalDifficulty(
	parentCtx context.Context,
) error {
	for {
		select {
		case <-time.After(time.Second):
			reached, err := ec.CheckTTD(parentCtx)
			if err != nil {
				return err
			}
			if reached {
				return nil
			}
		case <-parentCtx.Done():
			return parentCtx.Err()
		}
	}
}

func (ec *ExecutionClient) HeaderByHash(
	parentCtx context.Context,
	h common.Hash,
) (*types.Header, error) {
	ctx, cancel := utils.ContextTimeoutRPC(parentCtx)
	defer cancel()
	return ec.eth.HeaderByHash(ctx, h)
}

func (ec *ExecutionClient) HeaderByNumber(
	parentCtx context.Context,
	n *big.Int,
) (*types.Header, error) {
	ctx, cancel := utils.ContextTimeoutRPC(parentCtx)
	defer cancel()
	return ec.eth.HeaderByNumber(ctx, n)
}

func (ec *ExecutionClient) HeaderByLabel(
	parentCtx context.Context,
	l string,
) (*types.Header, error) {
	ctx, cancel := utils.ContextTimeoutRPC(parentCtx)
	defer cancel()
	h := new(types.Header)
	err := ec.ethRpcClient.CallContext(
		ctx,
		h,
		"eth_getBlockByNumber",
		l,
		false,
	)
	return h, err
}

func (ec *ExecutionClient) BlockByHash(
	parentCtx context.Context,
	h common.Hash,
) (*types.Block, error) {
	ctx, cancel := utils.ContextTimeoutRPC(parentCtx)
	defer cancel()
	return ec.eth.BlockByHash(ctx, h)
}

func (ec *ExecutionClient) BlockByNumber(
	parentCtx context.Context,
	n *big.Int,
) (*types.Block, error) {
	ctx, cancel := utils.ContextTimeoutRPC(parentCtx)
	defer cancel()
	return ec.eth.BlockByNumber(ctx, n)
}

func (ec *ExecutionClient) BalanceAt(
	parentCtx context.Context,
	account common.Address,
	n *big.Int,
) (*big.Int, error) {
	ctx, cancel := utils.ContextTimeoutRPC(parentCtx)
	defer cancel()
	return ec.eth.BalanceAt(ctx, account, n)
}

func (ec *ExecutionClient) SendTransaction(
	parentCtx context.Context,
	tx *types.Transaction,
) error {
	ctx, cancel := utils.ContextTimeoutRPC(parentCtx)
	defer cancel()
	return ec.eth.SendTransaction(ctx, tx)
}

type ExecutionClients []*ExecutionClient

// Return subset of clients that are currently running
func (all ExecutionClients) Running() ExecutionClients {
	res := make(ExecutionClients, 0)
	for _, ec := range all {
		if ec.IsRunning() {
			res = append(res, ec)
		}
	}
	return res
}

// Return subset of clients that are part of an specific subnet
func (all ExecutionClients) Subnet(subnet string) ExecutionClients {
	if subnet == "" {
		return all
	}
	res := make(ExecutionClients, 0)
	for _, ec := range all {
		if ec.Config.Subnet == subnet {
			res = append(res, ec)
		}
	}
	return res
}

// Returns comma-separated Bootnodes of all running execution nodes
func (all ExecutionClients) Enodes() (string, error) {
	if len(all) == 0 {
		return "", nil
	}
	enodes := make([]string, 0)
	for _, en := range all {
		if en.IsRunning() {
			if enodeClient, ok := en.Client.(EnodeClient); ok {
				enode, err := enodeClient.GetEnodeURL()
				if err != nil {
					return "", err
				}
				enodes = append(enodes, enode)
			} else {
				return "", fmt.Errorf("invalid client type")
			}
		}
	}
	return strings.Join(enodes, ","), nil
}

// Returns true if all head hashes match
func (all ExecutionClients) CheckHeads(
	l utils.Logging,
	parentCtx context.Context,
) (bool, error) {
	if len(all) <= 1 {
		return false, fmt.Errorf(
			"attempted to check the heads of a single or zero clients matched",
		)
	}

	header, err := all[0].HeaderByNumber(parentCtx, nil)
	if err != nil || header == nil {
		return false, err
	}
	baseHash := header.Hash()

	for _, en := range all[1:] {
		header, err = en.HeaderByNumber(parentCtx, nil)
		if err != nil || header == nil {
			return false, err
		}
		h := header.Hash()
		if h != baseHash {
			if l != nil {
				l.Logf("Hash mismatch between heads: %s != %s\n", h, baseHash)
			}
			return false, nil
		} else if l != nil {
			l.Logf("Hash match between heads: %s == %s\n", h, baseHash)
		}
	}
	return true, nil
}

type ProxyProvider interface {
	Proxy() *proxy.Proxy
}
type Proxies []ProxyProvider

func (all Proxies) Running() []*proxy.Proxy {
	res := make([]*proxy.Proxy, 0)
	for _, pp := range all {
		if p := pp.Proxy(); p != nil {
			res = append(res, p)
		}
	}
	return res
}
