package clients

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	api "github.com/ethereum/go-ethereum/core/beacon"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/ethereum/hive/hivesim"
	"github.com/ethereum/hive/simulators/eth2/common/spoofing/proxy"
	"github.com/ethereum/hive/simulators/eth2/common/utils"
	"github.com/golang-jwt/jwt/v4"
	spoof "github.com/rauljordan/engine-proxy/proxy"
)

const (
	PortUserRPC   = 8545
	PortEngineRPC = 8551
)

var AllEngineCallsLog = []string{
	"engine_forkchoiceUpdatedV1",
	"engine_forkchoiceUpdatedV2",
	"engine_getPayloadV1",
	"engine_getPayloadV2",
	"engine_newPayloadV1",
	"engine_newPayloadV2",
}

type ExecutionClient struct {
	T                *hivesim.T
	HiveClient       *hivesim.Client
	ClientType       string
	OptionsGenerator func() ([]hivesim.StartOption, error)
	proxy            **proxy.Proxy
	proxyPort        int
	subnet           string
	ttd              *big.Int
	clientIndex      int
	logEngineCalls   bool

	engineRpcClient *rpc.Client
	ethRpcClient    *rpc.Client
	eth             *ethclient.Client
}

func NewExecutionClient(
	t *hivesim.T,
	eth1Def *hivesim.ClientDefinition,
	optionsGenerator func() ([]hivesim.StartOption, error),
	clientIndex int,
	proxyPort int,
	subnet string,
	ttd *big.Int,
	logEngineCalls bool,
) *ExecutionClient {
	return &ExecutionClient{
		T:                t,
		ClientType:       eth1Def.Name,
		OptionsGenerator: optionsGenerator,
		proxyPort:        proxyPort,
		proxy:            new(*proxy.Proxy),
		subnet:           subnet,
		ttd:              ttd,
		clientIndex:      clientIndex,
		logEngineCalls:   logEngineCalls,
	}
}

func (en *ExecutionClient) UserRPCAddress() (string, error) {
	return fmt.Sprintf("http://%v:%d", en.HiveClient.IP, PortUserRPC), nil
}

func (en *ExecutionClient) EngineRPCAddress() (string, error) {
	// TODO what will the default port be?
	return fmt.Sprintf("http://%v:%d", en.HiveClient.IP, PortEngineRPC), nil
}

func (en *ExecutionClient) MustGetEnode() string {
	addr, err := en.HiveClient.EnodeURL()
	if err != nil {
		panic(err)
	}
	return addr
}

func (en *ExecutionClient) ConfiguredTTD() *big.Int {
	return en.ttd
}

func (en *ExecutionClient) Start(extraOptions ...hivesim.StartOption) error {
	if en.HiveClient != nil {
		return fmt.Errorf("client already started")
	}
	en.T.Logf("Starting client %s", en.ClientType)
	opts, err := en.OptionsGenerator()
	if err != nil {
		return fmt.Errorf("unable to get start options: %v", err)
	}
	opts = append(opts, extraOptions...)

	en.HiveClient = en.T.StartClient(en.ClientType, opts...)

	// Prepare Eth/Engine RPCs
	engineRPCAddress, err := en.EngineRPCAddress()
	if err != nil {
		return err
	}
	client := &http.Client{}
	// Prepare HTTP Client
	en.engineRpcClient, err = rpc.DialHTTPWithClient(engineRPCAddress, client)
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

	secret, err := hex.DecodeString(
		"7365637265747365637265747365637265747365637265747365637265747365",
	)
	if err != nil {
		panic(err)
	}
	simIP, err := en.T.Sim.ContainerNetworkIP(
		en.T.SuiteID,
		"bridge",
		"simulation",
	)
	if err != nil {
		panic(err)
	}

	proxy := proxy.NewProxy(
		net.ParseIP(simIP),
		en.proxyPort,
		dest,
		secret,
	)
	if en.logEngineCalls {
		logCallback := func(res []byte, req []byte) *spoof.Spoof {
			en.T.Logf(
				"DEBUG: execution client %d, request: %s",
				en.clientIndex,
				req,
			)
			en.T.Logf(
				"DEBUG: execution client %d, response: %s",
				en.clientIndex,
				res,
			)
			return nil
		}
		for _, c := range AllEngineCallsLog {
			proxy.AddResponseCallback(c, logCallback)
		}
	}

	*en.proxy = proxy

	return nil
}

func (en *ExecutionClient) Shutdown() error {
	if err := en.T.Sim.StopClient(en.T.SuiteID, en.T.TestID, en.HiveClient.Container); err != nil {
		return err
	}
	en.HiveClient = nil
	return nil
}

func (en *ExecutionClient) IsRunning() bool {
	return en.HiveClient != nil
}

func (en *ExecutionClient) Proxy() *proxy.Proxy {
	if en.proxy != nil && *en.proxy != nil {
		return *en.proxy
	}
	return nil
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
	secret, _ := hex.DecodeString(
		"7365637265747365637265747365637265747365637265747365637265747365",
	)
	en.PrepareAuthCallToken(secret, time.Now())
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
) (*api.ExecutableData, error) {
	var result api.ExecutableData
	if err := en.PrepareDefaultAuthCallToken(); err != nil {
		return nil, err
	}
	request := fmt.Sprintf("engine_getPayload%d", version)
	ctx, cancel := context.WithTimeout(parentCtx, time.Second*10)
	defer cancel()
	err := en.engineRpcClient.CallContext(ctx, &result, request, payloadID)
	return &result, err
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
	request := fmt.Sprintf("engine_newPayload%d", version)
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
	if td.Cmp(ec.ttd) >= 0 {
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
		if ec.subnet == subnet {
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
			enode, err := en.HiveClient.EnodeURL()
			if err != nil {
				return "", err
			}
			enodes = append(enodes, enode)
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

type Proxies []**proxy.Proxy

func (all Proxies) Running() []*proxy.Proxy {
	res := make([]*proxy.Proxy, 0)
	for _, p := range all {
		if p != nil && *p != nil {
			res = append(res, *p)
		}
	}
	return res
}
