package main

import (
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/hive/hivesim"
	"github.com/ethereum/hive/simulators/eth2/engine/setup"
	"github.com/protolambda/eth2api"
	"github.com/protolambda/eth2api/client/beaconapi"
	"github.com/protolambda/eth2api/client/nodeapi"
	"github.com/protolambda/zrnt/eth2/beacon/bellatrix"
	"github.com/protolambda/zrnt/eth2/beacon/common"
)

const (
	PortUserRPC      = 8545
	PortEngineRPC    = 8551
	PortBeaconTCP    = 9000
	PortBeaconUDP    = 9000
	PortBeaconAPI    = 4000
	PortBeaconGRPC   = 4001
	PortMetrics      = 8080
	PortValidatorAPI = 5000
)

// TODO: we assume the clients were configured with default ports.
// Would be cleaner to run a script in the client to get the address without assumptions

type ExecutionClient struct {
	T                *hivesim.T
	HiveClient       *hivesim.Client
	ClientType       string
	OptionsGenerator func() ([]hivesim.StartOption, error)
	proxy            **Proxy
	proxyPort        int
}

func NewExecutionClient(t *hivesim.T, eth1Def *hivesim.ClientDefinition, optionsGenerator func() ([]hivesim.StartOption, error), proxyPort int) *ExecutionClient {
	return &ExecutionClient{
		T:                t,
		ClientType:       eth1Def.Name,
		OptionsGenerator: optionsGenerator,
		proxyPort:        proxyPort,
		proxy:            new(*Proxy),
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

func (en *ExecutionClient) Start(extraOptions ...hivesim.StartOption) error {
	if en.HiveClient != nil {
		return fmt.Errorf("Client already started")
	}
	en.T.Logf("Starting client %s", en.ClientType)
	opts, err := en.OptionsGenerator()
	if err != nil {
		return fmt.Errorf("Unable to get start options: %v", err)
	}
	for _, opt := range extraOptions {
		opts = append(opts, opt)
	}
	en.HiveClient = en.T.StartClient(en.ClientType, opts...)

	// Prepare proxy
	dest, _ := en.EngineRPCAddress()

	secret, err := hex.DecodeString("7365637265747365637265747365637265747365637265747365637265747365")
	if err != nil {
		panic(err)
	}
	simIP, err := en.T.Sim.ContainerNetworkIP(en.T.SuiteID, "bridge", "simulation")
	if err != nil {
		panic(err)
	}

	*en.proxy = NewProxy(net.ParseIP(simIP), en.proxyPort, dest, secret)
	return nil
}

func (en *ExecutionClient) Shutdown() error {
	_, err := en.HiveClient.Exec("shutdown.sh")
	if err != nil {
		return err
	}
	en.HiveClient = nil
	return nil
}

func (en *ExecutionClient) IsRunning() bool {
	return en.HiveClient != nil
}

func (en *ExecutionClient) Proxy() *Proxy {
	if en.proxy != nil && *en.proxy != nil {
		return *en.proxy
	}
	return nil
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

// Returns comma-separated Bootnodes of all running execution nodes
func (ens ExecutionClients) Enodes() (string, error) {
	if len(ens) == 0 {
		return "", nil
	}
	enodes := make([]string, 0)
	for _, en := range ens {
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

type Proxies []**Proxy

func (all Proxies) Running() []*Proxy {
	res := make([]*Proxy, 0)
	for _, p := range all {
		if p != nil && *p != nil {
			res = append(res, *p)
		}
	}
	return res
}

type BeaconClient struct {
	T                *hivesim.T
	HiveClient       *hivesim.Client
	ClientType       string
	OptionsGenerator func() ([]hivesim.StartOption, error)
	API              *eth2api.Eth2HttpClient
	genesisTime      common.Timestamp
	spec             *common.Spec
	index            int
}

func NewBeaconClient(t *hivesim.T, beaconDef *hivesim.ClientDefinition, optionsGenerator func() ([]hivesim.StartOption, error), genesisTime common.Timestamp, spec *common.Spec, index int) *BeaconClient {
	return &BeaconClient{
		T:                t,
		ClientType:       beaconDef.Name,
		OptionsGenerator: optionsGenerator,
		genesisTime:      genesisTime,
		spec:             spec,
		index:            index,
	}
}

func (bn *BeaconClient) Start(extraOptions ...hivesim.StartOption) error {
	if bn.HiveClient != nil {
		return fmt.Errorf("Client already started")
	}
	bn.T.Logf("Starting client %s", bn.ClientType)
	opts, err := bn.OptionsGenerator()
	if err != nil {
		return fmt.Errorf("Unable to get start options: %v", err)
	}
	for _, opt := range extraOptions {
		opts = append(opts, opt)
	}
	bn.HiveClient = bn.T.StartClient(bn.ClientType, opts...)
	bn.API = &eth2api.Eth2HttpClient{
		Addr:  fmt.Sprintf("http://%s:%d", bn.HiveClient.IP, PortBeaconAPI),
		Cli:   &http.Client{},
		Codec: eth2api.JSONCodec{},
	}
	return nil
}

func (bn *BeaconClient) Shutdown() error {
	_, err := bn.HiveClient.Exec("shutdown.sh")
	if err != nil {
		return err
	}
	bn.HiveClient = nil
	return nil
}

func (bn *BeaconClient) IsRunning() bool {
	return bn.HiveClient != nil
}

func (bn *BeaconClient) ENR() (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()
	var out eth2api.NetworkIdentity
	if err := nodeapi.Identity(ctx, bn.API, &out); err != nil {
		return "", err
	}
	fmt.Printf("p2p addrs: %v\n", out.P2PAddresses)
	fmt.Printf("peer id: %s\n", out.PeerID)
	return out.ENR, nil
}

func (bn *BeaconClient) EnodeURL() (string, error) {
	return "", errors.New("beacon node does not have an discv4 Enode URL, use ENR or multi-address instead")
}

type BeaconClients []*BeaconClient

// Return subset of clients that are currently running
func (all BeaconClients) Running() BeaconClients {
	res := make(BeaconClients, 0)
	for _, bc := range all {
		if bc.IsRunning() {
			res = append(res, bc)
		}
	}
	return res
}

// Returns comma-separated ENRs of all running beacon nodes
func (beacons BeaconClients) ENRs() (string, error) {
	if len(beacons) == 0 {
		return "", nil
	}
	enrs := make([]string, 0)
	for _, bn := range beacons {
		if bn.IsRunning() {
			enr, err := bn.ENR()
			if err != nil {
				return "", err
			}
			enrs = append(enrs, enr)
		}
	}
	return strings.Join(enrs, ","), nil
}

func (b *BeaconClient) PrintAllBeaconBlocks(ctx context.Context) error {
	var headInfo eth2api.BeaconBlockHeaderAndInfo
	if exists, err := beaconapi.BlockHeader(ctx, b.API, eth2api.BlockHead, &headInfo); err != nil {
		return fmt.Errorf("PrintAllBeaconBlocks: failed to poll head: %v", err)
	} else if !exists {
		return fmt.Errorf("PrintAllBeaconBlocks: failed to poll head: !exists")
	}
	fmt.Printf("PrintAllBeaconBlocks: Head, slot %d, root %v\n", headInfo.Header.Message.Slot, headInfo.Root)
	for i := 1; i <= int(headInfo.Header.Message.Slot); i++ {
		var bHeader eth2api.BeaconBlockHeaderAndInfo
		if exists, err := beaconapi.BlockHeader(ctx, b.API, eth2api.BlockIdSlot(i), &bHeader); err != nil {
			fmt.Printf("PrintAllBeaconBlocks: Slot %d, not found\n", i)
			continue
		} else if !exists {
			fmt.Printf("PrintAllBeaconBlocks: Slot %d, not found\n", i)
			continue
		}
		fmt.Printf("PrintAllBeaconBlocks: Slot %d, root %v\n", i, bHeader.Root)
	}
	return nil
}

func (b *BeaconClient) WaitForExecutionPayload(ctx context.Context, timeoutSlots common.Slot) (ethcommon.Hash, error) {
	fmt.Printf("Waiting for execution payload on beacon %d\n", b.index)
	slotDuration := time.Duration(b.spec.SECONDS_PER_SLOT) * time.Second
	timer := time.NewTicker(slotDuration)
	var timeout <-chan time.Time
	if timeoutSlots > 0 {
		timeout = time.After(time.Second * time.Duration(uint64(timeoutSlots)*uint64(b.spec.SECONDS_PER_SLOT)))
	} else {
		timeout = make(<-chan time.Time)
	}

	for {
		select {
		case <-ctx.Done():
			return ethcommon.Hash{}, fmt.Errorf("context called")
		case <-timeout:
			return ethcommon.Hash{}, fmt.Errorf("Timeout")
		case <-timer.C:
			realTimeSlot := b.spec.TimeToSlot(common.Timestamp(time.Now().Unix()), b.genesisTime)
			var headInfo eth2api.BeaconBlockHeaderAndInfo
			if exists, err := beaconapi.BlockHeader(ctx, b.API, eth2api.BlockHead, &headInfo); err != nil {
				return ethcommon.Hash{}, fmt.Errorf("WaitForExecutionPayload: failed to poll head: %v", err)
			} else if !exists {
				return ethcommon.Hash{}, fmt.Errorf("WaitForExecutionPayload: failed to poll head: !exists")
			}

			var versionedBlock eth2api.VersionedSignedBeaconBlock
			if exists, err := beaconapi.BlockV2(ctx, b.API, eth2api.BlockIdRoot(headInfo.Root), &versionedBlock); err != nil {
				return ethcommon.Hash{}, fmt.Errorf("WaitForExecutionPayload: failed to retrieve block: %v", err)
			} else if !exists {
				return ethcommon.Hash{}, fmt.Errorf("WaitForExecutionPayload: block not found")
			}
			var execution ethcommon.Hash
			if versionedBlock.Version == "bellatrix" {
				block := versionedBlock.Data.(*bellatrix.SignedBeaconBlock)
				copy(execution[:], block.Message.Body.ExecutionPayload.BlockHash[:])
			}
			zero := ethcommon.Hash{}
			fmt.Printf("beacon %d: slot=%d, realTimeSlot=%d, head=%s, exec=%s\n", b.index, headInfo.Header.Message.Slot, realTimeSlot, shorten(headInfo.Root.String()), shorten(execution.Hex()))
			if bytes.Compare(execution[:], zero[:]) != 0 {
				return execution, nil
			}
		}
	}
}

type BlockV2OptimisticResponse struct {
	Version             string `json:"version"`
	ExecutionOptimistic bool   `json:"execution_optimistic"`
}

func (b *BeaconClient) CheckBlockIsOptimistic(ctx context.Context, blockID eth2api.BlockId) (bool, error) {
	var headOptStatus BlockV2OptimisticResponse
	if exists, err := eth2api.SimpleRequest(ctx, b.API, eth2api.FmtGET("/eth/v2/beacon/blocks/%s", blockID.BlockId()), &headOptStatus); err != nil {
		return false, err
	} else if !exists {
		// Block still not synced
		return false, fmt.Errorf("Block not found (!exists)")
	}
	return headOptStatus.ExecutionOptimistic, nil
}

func (b *BeaconClient) WaitForOptimisticState(ctx context.Context, timeoutSlots common.Slot, blockID eth2api.BlockId, optimistic bool) (*eth2api.BeaconBlockHeaderAndInfo, error) {
	fmt.Printf("Waiting for optimistic sync on beacon %d\n", b.index)
	slotDuration := time.Duration(b.spec.SECONDS_PER_SLOT) * time.Second
	timer := time.NewTicker(slotDuration)
	var timeout <-chan time.Time
	if timeoutSlots > 0 {
		timeout = time.After(time.Second * time.Duration(uint64(timeoutSlots)*uint64(b.spec.SECONDS_PER_SLOT)))
	} else {
		timeout = make(<-chan time.Time)
	}

	for {
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("context done")
		case <-timeout:
			return nil, fmt.Errorf("Timeout")
		case <-timer.C:
			var headOptStatus BlockV2OptimisticResponse
			if exists, err := eth2api.SimpleRequest(ctx, b.API, eth2api.FmtGET("/eth/v2/beacon/blocks/%s", blockID.BlockId()), &headOptStatus); err != nil {
				// Block still not synced
				continue
			} else if !exists {
				// Block still not synced
				continue
			}
			if headOptStatus.ExecutionOptimistic != optimistic {
				continue
			}
			// Return the block
			var blockInfo eth2api.BeaconBlockHeaderAndInfo
			if exists, err := beaconapi.BlockHeader(ctx, b.API, blockID, &blockInfo); err != nil {
				return nil, fmt.Errorf("WaitForExecutionPayload: failed to poll block: %v", err)
			} else if !exists {
				return nil, fmt.Errorf("WaitForExecutionPayload: failed to poll block: !exists")
			}
			return &blockInfo, nil
		}
	}
}

//
func (bn *BeaconClient) GetLatestExecutionBeaconBlock(ctx context.Context) (*bellatrix.SignedBeaconBlock, error) {
	var headInfo eth2api.BeaconBlockHeaderAndInfo
	if exists, err := beaconapi.BlockHeader(ctx, bn.API, eth2api.BlockHead, &headInfo); err != nil {
		return nil, fmt.Errorf("failed to poll head: %v", err)
	} else if !exists {
		return nil, fmt.Errorf("no head block")
	}
	for slot := headInfo.Header.Message.Slot; slot > 0; slot-- {
		var versionedBlock eth2api.VersionedSignedBeaconBlock
		if exists, err := beaconapi.BlockV2(ctx, bn.API, eth2api.BlockIdSlot(slot), &versionedBlock); err != nil {
			return nil, fmt.Errorf("failed to retrieve block: %v", err)
		} else if !exists {
			return nil, fmt.Errorf("block not found")
		}
		if versionedBlock.Version != "bellatrix" {
			return nil, nil
		}
		block := versionedBlock.Data.(*bellatrix.SignedBeaconBlock)
		payload := block.Message.Body.ExecutionPayload
		if ethcommon.BytesToHash(payload.BlockHash[:]) != (ethcommon.Hash{}) {
			return block, nil
		}
	}
	return nil, nil
}

func (bn *BeaconClient) GetFirstExecutionBeaconBlock(ctx context.Context) (*bellatrix.SignedBeaconBlock, error) {
	lastSlot := bn.spec.TimeToSlot(common.Timestamp(time.Now().Unix()), bn.genesisTime)
	for slot := common.Slot(0); slot <= lastSlot; slot++ {
		var versionedBlock eth2api.VersionedSignedBeaconBlock
		if exists, err := beaconapi.BlockV2(ctx, bn.API, eth2api.BlockIdSlot(slot), &versionedBlock); err != nil {
			continue
		} else if !exists {
			continue
		}
		if versionedBlock.Version != "bellatrix" {
			continue
		}
		block := versionedBlock.Data.(*bellatrix.SignedBeaconBlock)
		payload := block.Message.Body.ExecutionPayload
		if ethcommon.BytesToHash(payload.BlockHash[:]) != (ethcommon.Hash{}) {
			return block, nil
		}
	}
	return nil, nil
}

func (bn *BeaconClient) GetBeaconBlockByExecutionHash(ctx context.Context, hash ethcommon.Hash) (*bellatrix.SignedBeaconBlock, error) {
	var headInfo eth2api.BeaconBlockHeaderAndInfo
	if exists, err := beaconapi.BlockHeader(ctx, bn.API, eth2api.BlockHead, &headInfo); err != nil {
		return nil, fmt.Errorf("failed to poll head: %v", err)
	} else if !exists {
		return nil, fmt.Errorf("no head block")
	}

	for slot := int(headInfo.Header.Message.Slot); slot > 0; slot -= 1 {
		var versionedBlock eth2api.VersionedSignedBeaconBlock
		if exists, err := beaconapi.BlockV2(ctx, bn.API, eth2api.BlockIdSlot(slot), &versionedBlock); err != nil {
			continue
		} else if !exists {
			continue
		}
		if versionedBlock.Version != "bellatrix" {
			// Block can't contain an executable payload, and we are not going to find it going backwards, so return.
			return nil, nil
		}
		block := versionedBlock.Data.(*bellatrix.SignedBeaconBlock)
		payload := block.Message.Body.ExecutionPayload
		if bytes.Compare(payload.BlockHash[:], hash[:]) == 0 {
			return block, nil
		}
	}
	return nil, nil
}

func (b BeaconClients) GetBeaconBlockByExecutionHash(ctx context.Context, hash ethcommon.Hash) (*bellatrix.SignedBeaconBlock, error) {
	for _, bn := range b {
		block, err := bn.GetBeaconBlockByExecutionHash(ctx, hash)
		if err != nil || block != nil {
			return block, err
		}
	}
	return nil, nil
}

type ValidatorClient struct {
	T                *hivesim.T
	HiveClient       *hivesim.Client
	ClientType       string
	OptionsGenerator func([]*setup.KeyDetails) ([]hivesim.StartOption, error)
	Keys             []*setup.KeyDetails
}

func NewValidatorClient(t *hivesim.T, validatorDef *hivesim.ClientDefinition, optionsGenerator func([]*setup.KeyDetails) ([]hivesim.StartOption, error), keys []*setup.KeyDetails) *ValidatorClient {
	return &ValidatorClient{
		T:                t,
		ClientType:       validatorDef.Name,
		OptionsGenerator: optionsGenerator,
		Keys:             keys,
	}
}

func (vc *ValidatorClient) Start(extraOptions ...hivesim.StartOption) error {
	if vc.HiveClient != nil {
		return fmt.Errorf("Client already started")
	}
	if len(vc.Keys) == 0 {
		vc.T.Logf("Skipping validator because it has 0 validator keys")
		return nil
	}
	vc.T.Logf("Starting client %s", vc.ClientType)
	opts, err := vc.OptionsGenerator(vc.Keys)
	if err != nil {
		return fmt.Errorf("Unable to get start options: %v", err)
	}
	for _, opt := range extraOptions {
		opts = append(opts, opt)
	}
	vc.HiveClient = vc.T.StartClient(vc.ClientType, opts...)
	return nil
}

func (vc *ValidatorClient) IsRunning() bool {
	return vc.HiveClient != nil
}

func (v *ValidatorClient) ContainsKey(pk [48]byte) bool {
	for _, k := range v.Keys {
		if k.ValidatorPubkey == pk {
			return true
		}
	}
	return false
}

type ValidatorClients []*ValidatorClient

// Return subset of clients that are currently running
func (all ValidatorClients) Running() ValidatorClients {
	res := make(ValidatorClients, 0)
	for _, vc := range all {
		if vc.IsRunning() {
			res = append(res, vc)
		}
	}
	return res
}

// A validator client bundle consists of:
// - Execution client
// - Beacon client
// - Validator client
type NodeClientBundle struct {
	T               *hivesim.T
	Index           int
	ExecutionClient *ExecutionClient
	BeaconClient    *BeaconClient
	ValidatorClient *ValidatorClient
	Verification    bool
}

// Starts all clients included in the bundle
func (cb *NodeClientBundle) Start(extraOptions ...hivesim.StartOption) error {
	cb.T.Logf("Starting validator client bundle %d", cb.Index)
	if cb.ExecutionClient != nil {
		if err := cb.ExecutionClient.Start(extraOptions...); err != nil {
			return err
		}
	} else {
		cb.T.Logf("No execution client started")
	}
	if cb.BeaconClient != nil {
		if err := cb.BeaconClient.Start(extraOptions...); err != nil {
			return err
		}
	} else {
		cb.T.Logf("No beacon client started")
	}
	if cb.ValidatorClient != nil {
		if err := cb.ValidatorClient.Start(extraOptions...); err != nil {
			return err
		}
	} else {
		cb.T.Logf("No validator client started")
	}
	return nil
}

type NodeClientBundles []NodeClientBundle

// Return all execution clients, even the ones not currently running
func (cbs NodeClientBundles) ExecutionClients() ExecutionClients {
	en := make(ExecutionClients, 0)
	for _, cb := range cbs {
		if cb.ExecutionClient != nil {
			en = append(en, cb.ExecutionClient)
		}
	}
	return en
}

// Return all proxy pointers, even the ones not currently running
func (cbs NodeClientBundles) Proxies() Proxies {
	ps := make(Proxies, 0)
	for _, cb := range cbs {
		if cb.ExecutionClient != nil {
			ps = append(ps, cb.ExecutionClient.proxy)
		}
	}
	return ps
}

// Return all beacon clients, even the ones not currently running
func (cbs NodeClientBundles) BeaconClients() BeaconClients {
	bn := make(BeaconClients, 0)
	for _, cb := range cbs {
		if cb.BeaconClient != nil {
			bn = append(bn, cb.BeaconClient)
		}
	}
	return bn
}

// Return all validator clients, even the ones not currently running
func (cbs NodeClientBundles) ValidatorClients() ValidatorClients {
	vc := make(ValidatorClients, 0)
	for _, cb := range cbs {
		if cb.ValidatorClient != nil {
			vc = append(vc, cb.ValidatorClient)
		}
	}
	return vc
}

// Return subset of nodes which are marked as verification nodes
func (all NodeClientBundles) VerificationNodes() NodeClientBundles {
	// If none is set as verification, then all are verification nodes
	var any bool
	for _, cb := range all {
		if cb.Verification {
			any = true
			break
		}
	}
	if !any {
		return all
	}

	res := make(NodeClientBundles, 0)
	for _, cb := range all {
		if cb.Verification {
			res = append(res, cb)
		}
	}
	return res
}

func (cbs NodeClientBundles) RemoveNodeAsVerifier(id int) error {
	if id >= len(cbs) {
		return fmt.Errorf("Node %d does not exist", id)
	}
	var any bool
	for _, cb := range cbs {
		if cb.Verification {
			any = true
			break
		}
	}
	if any {
		cbs[id].Verification = false
	} else {
		// If no node is set as verifier, we will set all other nodes as verifiers then
		for i := range cbs {
			cbs[i].Verification = (i != id)
		}
	}
	return nil
}
