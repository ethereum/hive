package main

import (
	"bytes"
	"context"
	"fmt"
	"math/big"
	"net/http"
	"sort"
	"time"

	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/protolambda/eth2api"
	"github.com/protolambda/eth2api/client/beaconapi"
	"github.com/protolambda/zrnt/eth2/beacon/altair"
	"github.com/protolambda/zrnt/eth2/beacon/bellatrix"
	"github.com/protolambda/zrnt/eth2/beacon/common"
	"github.com/protolambda/zrnt/eth2/beacon/phase0"
	"github.com/protolambda/ztyp/tree"
)

// Interface to specify on which slot the verification will be performed
type VerificationSlot interface {
	Slot(t *Testnet, ctx context.Context, bn *BeaconClient) (common.Slot, error)
}

// Return the slot at the start of the checkpoint's following epoch
type FirstSlotAfterCheckpoint struct {
	*common.Checkpoint
}

func (c FirstSlotAfterCheckpoint) Slot(t *Testnet, _ context.Context, _ *BeaconClient) (common.Slot, error) {
	return t.spec.EpochStartSlot(c.Checkpoint.Epoch + 1)
}

// Return the slot at the end of a checkpoint
type LastSlotAtCheckpoint struct {
	*common.Checkpoint
}

func (c LastSlotAtCheckpoint) Slot(t *Testnet, _ context.Context, _ *BeaconClient) (common.Slot, error) {
	return t.spec.SLOTS_PER_EPOCH * common.Slot(c.Checkpoint.Epoch), nil
}

// Get last slot according to current time
type LastestSlotByTime struct{}

func (l LastestSlotByTime) Slot(t *Testnet, _ context.Context, _ *BeaconClient) (common.Slot, error) {
	return t.spec.TimeToSlot(common.Timestamp(time.Now().Unix()), t.genesisTime), nil
}

// Get last slot according to current head of a beacon node
type LastestSlotByHead struct{}

func (l LastestSlotByHead) Slot(t *Testnet, ctx context.Context, bn *BeaconClient) (common.Slot, error) {
	var headInfo eth2api.BeaconBlockHeaderAndInfo
	if exists, err := beaconapi.BlockHeader(ctx, bn.API, eth2api.BlockHead, &headInfo); err != nil {
		return common.Slot(0), fmt.Errorf("failed to poll head: %v", err)
	} else if !exists {
		return common.Slot(0), fmt.Errorf("no head block")
	}
	return headInfo.Header.Message.Slot, nil
}

// VerifyParticipation ensures that the participation of the finialized epoch
// of a given checkpoint is above the expected threshold.
func VerifyParticipation(t *Testnet, ctx context.Context, vs VerificationSlot, expected float64) error {
	runningBeacons := t.VerificationNodes().BeaconClients().Running()
	slot, err := vs.Slot(t, ctx, runningBeacons[0])
	if err != nil {
		return err
	}
	if t.spec.BELLATRIX_FORK_EPOCH <= t.spec.SlotToEpoch(slot) {
		// slot-1 to target last slot in finalized epoch
		slot = slot - 1
	}
	for i, b := range runningBeacons {
		health, err := getHealth(ctx, b.API, t.spec, slot)
		if err != nil {
			return err
		}
		if health < expected {
			return fmt.Errorf("beacon %d: participation not healthy (got: %.2f, expected: %.2f)", i, health, expected)
		}
		t.Logf("beacon %d: epoch=%d participation=%.2f", i, t.spec.SlotToEpoch(slot), health)
	}
	return nil
}

// VerifyExecutionPayloadIsCanonical retrieves the execution payload from the
// finalized block and verifies that is in the execution client's canonical
// chain.
func VerifyExecutionPayloadIsCanonical(t *Testnet, ctx context.Context, vs VerificationSlot) error {
	runningBeacons := t.VerificationNodes().BeaconClients().Running()
	slot, err := vs.Slot(t, ctx, runningBeacons[0])
	if err != nil {
		return err
	}
	var blockId eth2api.BlockIdSlot
	blockId = eth2api.BlockIdSlot(slot)
	var versionedBlock eth2api.VersionedSignedBeaconBlock
	if exists, err := beaconapi.BlockV2(ctx, runningBeacons[0].API, blockId, &versionedBlock); err != nil {
		return fmt.Errorf("beacon %d: failed to retrieve block: %v", 0, err)
	} else if !exists {
		return fmt.Errorf("beacon %d: block not found", 0)
	}
	if versionedBlock.Version != "bellatrix" {
		return nil
	}
	payload := versionedBlock.Data.(*bellatrix.SignedBeaconBlock).Message.Body.ExecutionPayload

	for i, proxy := range t.VerificationNodes().Proxies().Running() {
		client := ethclient.NewClient(proxy.RPC())
		block, err := client.BlockByNumber(ctx, big.NewInt(int64(payload.BlockNumber)))
		if err != nil {
			return fmt.Errorf("eth1 %d: %s", 0, err)
		}
		if block.Hash() != [32]byte(payload.BlockHash) {
			return fmt.Errorf("eth1 %d: blocks don't match (got=%s, expected=%s)", i, shorten(block.Hash().String()), shorten(payload.BlockHash.String()))
		}
	}
	return nil
}

// VerifyExecutionPayloadIsCanonical retrieves the execution payload from the
// finalized block and verifies that is in the execution client's canonical
// chain.
func VerifyExecutionPayloadHashInclusion(t *Testnet, ctx context.Context, vs VerificationSlot, hash ethcommon.Hash) (*bellatrix.SignedBeaconBlock, error) {
	for _, bn := range t.VerificationNodes().BeaconClients().Running() {
		b, err := VerifyExecutionPayloadHashInclusionNode(t, ctx, vs, bn, hash)
		if err != nil || b != nil {
			return b, err
		}
	}
	return nil, nil
}

func VerifyExecutionPayloadHashInclusionNode(t *Testnet, ctx context.Context, vs VerificationSlot, bn *BeaconClient, hash ethcommon.Hash) (*bellatrix.SignedBeaconBlock, error) {
	lastSlot, err := vs.Slot(t, ctx, bn)
	if err != nil {
		return nil, err
	}
	for slot := lastSlot; slot > 0; slot -= 1 {
		var versionedBlock eth2api.VersionedSignedBeaconBlock
		if exists, err := beaconapi.BlockV2(ctx, bn.API, eth2api.BlockIdSlot(slot), &versionedBlock); err != nil {
			continue
		} else if !exists {
			continue
		}
		if versionedBlock.Version != "bellatrix" {
			// Block can't contain an executable payload
			break
		}
		block := versionedBlock.Data.(*bellatrix.SignedBeaconBlock)
		payload := block.Message.Body.ExecutionPayload
		if bytes.Compare(payload.BlockHash[:], hash[:]) == 0 {
			return block, nil
		}
	}
	return nil, nil
}

// VerifyProposers checks that all validator clients have proposed a block on
// the finalized beacon chain that includes an execution payload.
func VerifyProposers(t *Testnet, ctx context.Context, vs VerificationSlot, allow_empty_blocks bool) error {
	runningBeacons := t.VerificationNodes().BeaconClients().Running()
	lastSlot, err := vs.Slot(t, ctx, runningBeacons[0])
	if err != nil {
		return err
	}
	proposers := make([]bool, len(runningBeacons))
	for slot := common.Slot(0); slot <= lastSlot; slot += 1 {
		var versionedBlock eth2api.VersionedSignedBeaconBlock
		if exists, err := beaconapi.BlockV2(ctx, runningBeacons[0].API, eth2api.BlockIdSlot(slot), &versionedBlock); err != nil {
			if allow_empty_blocks {
				continue
			}
			return fmt.Errorf("beacon %d: failed to retrieve block: %v", 0, err)
		} else if !exists {
			if allow_empty_blocks {
				continue
			}
			return fmt.Errorf("beacon %d: block not found", 0)
		}
		var proposerIndex common.ValidatorIndex
		switch versionedBlock.Version {
		case "phase0":
			block := versionedBlock.Data.(*phase0.SignedBeaconBlock)
			proposerIndex = block.Message.ProposerIndex
		case "altair":
			block := versionedBlock.Data.(*altair.SignedBeaconBlock)
			proposerIndex = block.Message.ProposerIndex
		case "bellatrix":
			block := versionedBlock.Data.(*bellatrix.SignedBeaconBlock)
			proposerIndex = block.Message.ProposerIndex
		}

		var validator eth2api.ValidatorResponse
		if exists, err := beaconapi.StateValidator(ctx, runningBeacons[0].API, eth2api.StateIdSlot(slot), eth2api.ValidatorIdIndex(proposerIndex), &validator); err != nil {
			return fmt.Errorf("beacon %d: failed to retrieve validator: %v", 0, err)
		} else if !exists {
			return fmt.Errorf("beacon %d: validator not found", 0)
		}
		idx, err := t.ValidatorClientIndex([48]byte(validator.Validator.Pubkey))
		if err != nil {
			return fmt.Errorf("pub key not found on any validator client")
		}
		proposers[idx] = true
	}
	for i, proposed := range proposers {
		if !proposed {
			return fmt.Errorf("beacon %d: did not propose a block", i)
		}
	}
	return nil
}

func VerifyELBlockLabels(t *Testnet, ctx context.Context) error {
	runningExecution := t.VerificationNodes().ExecutionClients().Running()
	runningBeacons := t.VerificationNodes().BeaconClients().Running()
	for i := 0; i < len(runningExecution); i++ {
		el := runningExecution[i]
		bn := runningBeacons[i]
		// Get the head
		var headInfo eth2api.BeaconBlockHeaderAndInfo
		if exists, err := beaconapi.BlockHeader(ctx, bn.API, eth2api.BlockHead, &headInfo); err != nil {
			return err
		} else if !exists {
			return fmt.Errorf("beacon %d: head info not found", i)
		}

		// Get the checkpoints
		var checkpoints eth2api.FinalityCheckpoints
		if exists, err := beaconapi.FinalityCheckpoints(ctx, bn.API, eth2api.StateIdRoot(headInfo.Header.Message.StateRoot), &checkpoints); err != nil || !exists {
			if exists, err = beaconapi.FinalityCheckpoints(ctx, bn.API, eth2api.StateIdSlot(headInfo.Header.Message.Slot), &checkpoints); err != nil {
				return err
			} else if !exists {
				return fmt.Errorf("beacon %d: finality checkpoints not found", i)
			}
		}
		blockLabels := map[string]tree.Root{
			"latest":    headInfo.Root,
			"finalized": checkpoints.Finalized.Root,
			"safe":      checkpoints.CurrentJustified.Root,
		}

		for label, root := range blockLabels {
			// Get the beacon block
			var (
				versionedBlock eth2api.VersionedSignedBeaconBlock
				expectedExec   ethcommon.Hash
			)
			if exists, err := beaconapi.BlockV2(ctx, bn.API, eth2api.BlockIdRoot(root), &versionedBlock); err != nil {
				return err
			} else if !exists {
				return fmt.Errorf("beacon %d: beacon block to query %s not found", i, label)
			}
			switch versionedBlock.Version {
			case "phase0":
				expectedExec = ethcommon.Hash{}
			case "altair":
				expectedExec = ethcommon.Hash{}
			case "bellatrix":
				block := versionedBlock.Data.(*bellatrix.SignedBeaconBlock)
				expectedExec = ethcommon.BytesToHash(block.Message.Body.ExecutionPayload.BlockHash[:])
			}

			// Get the el block and compare
			rpcAddr, _ := el.UserRPCAddress()
			rpcClient, _ := rpc.DialHTTPWithClient(rpcAddr, &http.Client{})
			var h types.Header

			if err := rpcClient.CallContext(ctx, &h, "eth_getBlockByNumber", label, false); err != nil {
				if expectedExec != (ethcommon.Hash{}) {
					return err
				}
			} else {
				if h.Hash() != expectedExec {
					return fmt.Errorf("beacon %d: Execution hash found in checkpoint block (%s) does not match what the el returns: %v != %v", i, label, expectedExec, h.Hash())
				}
				fmt.Printf("beacon %d: Execution hash matches beacon checkpoint block (%s) information: %v\n", i, label, h.Hash())
			}

		}
	}
	return nil
}

func VerifyELHeads(t *Testnet, ctx context.Context) error {
	runningExecution := t.VerificationNodes().ExecutionClients().Running()
	client := ethclient.NewClient(runningExecution[0].HiveClient.RPC())
	head, err := client.HeaderByNumber(ctx, nil)
	if err != nil {
		return err
	}

	t.Logf("Verifying EL heads at %v", head.Hash())
	for i, node := range runningExecution {
		client := ethclient.NewClient(node.HiveClient.RPC())
		head2, err := client.HeaderByNumber(ctx, nil)
		if err != nil {
			return err
		}
		if head.Hash() != head2.Hash() {
			return fmt.Errorf("different heads: %v: %v %v: %v", 0, head, i, head2)
		}
	}
	return nil
}

// Helper debugging functions
func (b *BeaconClient) PrintAllBeaconBlocks(ctx context.Context) error {
	var headInfo eth2api.BeaconBlockHeaderAndInfo
	if exists, err := beaconapi.BlockHeader(ctx, b.API, eth2api.BlockHead, &headInfo); err != nil {
		return fmt.Errorf("PrintAllBeaconBlocks: failed to poll head: %v", err)
	} else if !exists {
		return fmt.Errorf("PrintAllBeaconBlocks: failed to poll head: !exists")
	}
	fmt.Printf("PrintAllBeaconBlocks: Printing beacon chain from %s\n", b.HiveClient.Container)
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
		var (
			root      = bHeader.Root
			execution = "0x0000..0000"
		)

		var versionedBlock eth2api.VersionedSignedBeaconBlock
		if exists, err := beaconapi.BlockV2(ctx, b.API, eth2api.BlockIdRoot(root), &versionedBlock); err == nil && exists {
			switch versionedBlock.Version {
			case "bellatrix":
				block := versionedBlock.Data.(*bellatrix.SignedBeaconBlock)
				execution = shorten(block.Message.Body.ExecutionPayload.BlockHash.String())
			}
		}

		fmt.Printf("PrintAllBeaconBlocks: Slot=%d, root=%v, exec=%s\n", i, root, execution)
	}
	return nil
}

type BeaconBlockInfo struct {
	Root      tree.Root
	Parent    tree.Root
	Execution ethcommon.Hash
	Nodes     []int
}

type BeaconBlockList []*BeaconBlockInfo

func (bl BeaconBlockList) Add(root tree.Root, parent tree.Root, execution ethcommon.Hash, nodeId int) (BeaconBlockList, error) {
	for _, b := range bl {
		if root == b.Root {
			if parent != b.Parent {
				return bl, fmt.Errorf("roots equal (%s), parent root mismatch: %s != %s", root.String(), parent.String(), b.Parent.String())
			}
			if execution != b.Execution {
				return bl, fmt.Errorf("roots equal (%s), exec hash mismatch: %s != %s", root.String(), execution.String(), b.Execution.String())
			}
			for _, n := range b.Nodes {
				if nodeId == n {
					return bl, nil
				}
			}
			b.Nodes = append(b.Nodes, nodeId)
			return bl, nil
		}
	}
	return append(bl, &BeaconBlockInfo{
		Root:      root,
		Parent:    parent,
		Execution: execution,
		Nodes:     []int{nodeId},
	}), nil
}

type BeaconBlockMap map[common.Slot]BeaconBlockList

func (bm BeaconBlockMap) Add(slot common.Slot, root tree.Root, parent tree.Root, execution ethcommon.Hash, nodeId int) error {
	if _, found := bm[slot]; !found {
		bm[slot] = make(BeaconBlockList, 0)
	}
	var err error
	bm[slot], err = bm[slot].Add(root, parent, execution, nodeId)
	if err != nil {
		return fmt.Errorf("Conflicting block on slot %d: %v", slot, err)
	}
	return nil
}
func (bm BeaconBlockMap) Print(l Logging) error {
	slots := make([]int, 0, len(bm))
	for s := range bm {
		slots = append(slots, int(s))
	}
	sort.Ints(slots)
	for _, s := range slots {
		l.Logf("- Slot=%d\n", s)
		for i, b := range bm[common.Slot(s)] {
			l.Logf("    Fork %d: root=%v, parent=%v, exec=%s, nodes=%v \n", i, shorten(b.Root.String()), shorten(b.Parent.String()), shorten(b.Execution.String()), b.Nodes)
		}
	}
	return nil
}
func PrintAllBeaconBlocks(t *Testnet, ctx context.Context) error {
	runningBeacons := t.VerificationNodes().BeaconClients().Running()

	beaconTree := make(BeaconBlockMap)

	for nodeId, beaconNode := range runningBeacons {

		var (
			nextBlock eth2api.BlockId
		)

		nextBlock = eth2api.BlockHead

		for {
			if nextBlock.BlockId() == eth2api.BlockIdRoot(tree.Root{}).BlockId() {
				break
			}
			// Get block header
			var bHeader eth2api.BeaconBlockHeaderAndInfo
			if exists, err := beaconapi.BlockHeader(ctx, beaconNode.API, nextBlock, &bHeader); err != nil {
				t.Logf("Error fetching block (%s) from beacon node %d: %v", nextBlock.BlockId(), nodeId, err)
				break
			} else if !exists {
				t.Logf("Unable to fetch block (%s) from beacon node %d: !exists", nextBlock.BlockId(), nodeId)
				break
			}

			var (
				root      = bHeader.Root
				parent    = bHeader.Header.Message.ParentRoot
				execution = ethcommon.Hash{}
			)
			var versionedBlock eth2api.VersionedSignedBeaconBlock
			if exists, err := beaconapi.BlockV2(ctx, beaconNode.API, eth2api.BlockIdRoot(root), &versionedBlock); err == nil && exists {
				switch versionedBlock.Version {
				case "bellatrix":
					block := versionedBlock.Data.(*bellatrix.SignedBeaconBlock)
					execution = ethcommon.BytesToHash(block.Message.Body.ExecutionPayload.BlockHash[:])
					t.Logf("Node %d: Execution payload: hash=%s", nodeId, shorten(execution.String()))
				}
			} else if err != nil {
				t.Logf("Error getting versioned block=%s node=%d: %v", nextBlock.BlockId(), nodeId, err)
				break
			} else if !exists {
				t.Logf("Error getting versioned block=%s node=%d: !exists", nextBlock.BlockId(), nodeId)
				break
			}
			if err := beaconTree.Add(bHeader.Header.Message.Slot, root, parent, execution, nodeId); err != nil {
				return err
			}
			nextBlock = eth2api.BlockIdRoot(parent)
		}

	}
	beaconTree.Print(t)
	return nil
}
