package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
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

type Eth1Node struct {
	*hivesim.Client
}

func (en *Eth1Node) UserRPCAddress() (string, error) {
	return fmt.Sprintf("http://%v:%d", en.IP, PortUserRPC), nil
}

func (en *Eth1Node) EngineRPCAddress() (string, error) {
	// TODO what will the default port be?
	return fmt.Sprintf("http://%v:%d", en.IP, PortEngineRPC), nil
}

func (en *Eth1Node) MustGetEnode() string {
	addr, err := en.EnodeURL()
	if err != nil {
		panic(err)
	}
	return addr
}

type BeaconNode struct {
	*hivesim.Client
	API         *eth2api.Eth2HttpClient
	genesisTime common.Timestamp
	spec        *common.Spec
	index       int
}

type BeaconNodes []*BeaconNode

func NewBeaconNode(cl *hivesim.Client, genesisTime common.Timestamp, spec *common.Spec, index int) *BeaconNode {
	return &BeaconNode{
		Client: cl,
		API: &eth2api.Eth2HttpClient{
			Addr:  fmt.Sprintf("http://%s:%d", cl.IP, PortBeaconAPI),
			Cli:   &http.Client{},
			Codec: eth2api.JSONCodec{},
		},
		genesisTime: genesisTime,
		spec:        spec,
		index:       index,
	}
}

func (bn *BeaconNode) ENR() (string, error) {
	ctx, _ := context.WithTimeout(context.Background(), time.Second*10)
	var out eth2api.NetworkIdentity
	if err := nodeapi.Identity(ctx, bn.API, &out); err != nil {
		return "", err
	}
	fmt.Printf("p2p addrs: %v\n", out.P2PAddresses)
	fmt.Printf("peer id: %s\n", out.PeerID)
	return out.ENR, nil
}

func (bn *BeaconNode) EnodeURL() (string, error) {
	return "", errors.New("beacon node does not have an discv4 Enode URL, use ENR or multi-address instead")
}

// Returns comma-separated ENRs of all beacon nodes
func (beacons BeaconNodes) ENRs() (string, error) {
	if len(beacons) == 0 {
		return "", nil
	}
	enrs := make([]string, 0)
	for _, bn := range beacons {
		enr, err := bn.ENR()
		if err != nil {
			return "", err
		}
		enrs = append(enrs, enr)
	}
	return strings.Join(enrs, ","), nil
}

func (b *BeaconNode) WaitForExecutionPayload(ctx context.Context, timeoutSlots common.Slot) (ethcommon.Hash, error) {
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
			fmt.Printf("beacon %d: slot=%d, head=%s, exec=%s\n", b.index, headInfo.Header.Message.Slot, shorten(headInfo.Root.String()), shorten(execution.Hex()))
			if bytes.Compare(execution[:], zero[:]) != 0 {
				return execution, nil
			}
		}
	}
}

//
func (bn *BeaconNode) GetLatestExecutionBeaconBlock(ctx context.Context) (*bellatrix.SignedBeaconBlock, error) {
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

func (bn *BeaconNode) GetFirstExecutionBeaconBlock(ctx context.Context) (*bellatrix.SignedBeaconBlock, error) {
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

func (bn *BeaconNode) GetBeaconBlockByExecutionHash(ctx context.Context, hash ethcommon.Hash) (*bellatrix.SignedBeaconBlock, error) {
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

func (b BeaconNodes) GetBeaconBlockByExecutionHash(ctx context.Context, hash ethcommon.Hash) (*bellatrix.SignedBeaconBlock, error) {
	for _, bn := range b {
		block, err := bn.GetBeaconBlockByExecutionHash(ctx, hash)
		if err != nil || block != nil {
			return block, err
		}
	}
	return nil, nil
}

type ValidatorClient struct {
	*hivesim.Client
	keys []*setup.KeyDetails
}

func (v *ValidatorClient) ContainsKey(pk [48]byte) bool {
	for _, k := range v.keys {
		if k.ValidatorPubkey == pk {
			return true
		}
	}
	return false
}
