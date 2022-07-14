package main

import (
	"bytes"
	"context"
	"fmt"
	"math/big"
	"time"

	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/protolambda/eth2api"
	"github.com/protolambda/eth2api/client/beaconapi"
	"github.com/protolambda/zrnt/eth2/beacon/altair"
	"github.com/protolambda/zrnt/eth2/beacon/bellatrix"
	"github.com/protolambda/zrnt/eth2/beacon/common"
	"github.com/protolambda/zrnt/eth2/beacon/phase0"
)

// Interface to specify on which slot the verification will be performed
type VerificationSlot interface {
	Slot(t *Testnet, ctx context.Context, bn *BeaconNode) (common.Slot, error)
}

// Return the slot at the start of the checkpoint's following epoch
type FirstSlotAfterCheckpoint struct {
	*common.Checkpoint
}

func (c FirstSlotAfterCheckpoint) Slot(t *Testnet, _ context.Context, _ *BeaconNode) (common.Slot, error) {
	return t.spec.EpochStartSlot(c.Checkpoint.Epoch + 1)
}

// Return the slot at the end of a checkpoint
type LastSlotAtCheckpoint struct {
	*common.Checkpoint
}

func (c LastSlotAtCheckpoint) Slot(t *Testnet, _ context.Context, _ *BeaconNode) (common.Slot, error) {
	return t.spec.SLOTS_PER_EPOCH * common.Slot(c.Checkpoint.Epoch), nil
}

// Get last slot according to current time
type LastestSlotByTime struct{}

func (l LastestSlotByTime) Slot(t *Testnet, _ context.Context, _ *BeaconNode) (common.Slot, error) {
	return t.spec.TimeToSlot(common.Timestamp(time.Now().Unix()), t.genesisTime), nil
}

// Get last slot according to current head of a beacon node
type LastestSlotByHead struct{}

func (l LastestSlotByHead) Slot(t *Testnet, ctx context.Context, bn *BeaconNode) (common.Slot, error) {
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
	slot, err := vs.Slot(t, ctx, t.verificationBeacons()[0])
	if err != nil {
		return err
	}
	if t.spec.BELLATRIX_FORK_EPOCH <= t.spec.SlotToEpoch(slot) {
		// slot-1 to target last slot in finalized epoch
		slot = slot - 1
	}
	for i, b := range t.verificationBeacons() {
		health, err := getHealth(ctx, b.API, t.spec, slot)
		if err != nil {
			return err
		}
		if health < expected {
			return fmt.Errorf("beacon %d: participation not healthy (got: %.2f, expected: %.2f)", i, health, expected)
		}
		t.t.Logf("beacon %d: epoch=%d participation=%.2f", i, t.spec.SlotToEpoch(slot), health)
	}
	return nil
}

// VerifyExecutionPayloadIsCanonical retrieves the execution payload from the
// finalized block and verifies that is in the execution client's canonical
// chain.
func VerifyExecutionPayloadIsCanonical(t *Testnet, ctx context.Context, vs VerificationSlot) error {
	slot, err := vs.Slot(t, ctx, t.verificationBeacons()[0])
	if err != nil {
		return err
	}
	var blockId eth2api.BlockIdSlot
	blockId = eth2api.BlockIdSlot(slot)
	var versionedBlock eth2api.VersionedSignedBeaconBlock
	if exists, err := beaconapi.BlockV2(ctx, t.verificationBeacons()[0].API, blockId, &versionedBlock); err != nil {
		return fmt.Errorf("beacon %d: failed to retrieve block: %v", 0, err)
	} else if !exists {
		return fmt.Errorf("beacon %d: block not found", 0)
	}
	if versionedBlock.Version != "bellatrix" {
		return nil
	}
	payload := versionedBlock.Data.(*bellatrix.SignedBeaconBlock).Message.Body.ExecutionPayload

	for i, proxy := range t.verificationProxies() {
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
	for _, bn := range t.verificationBeacons() {
		b, err := VerifyExecutionPayloadHashInclusionNode(t, ctx, vs, bn, hash)
		if err != nil || b != nil {
			return b, err
		}
	}
	return nil, nil
}

func VerifyExecutionPayloadHashInclusionNode(t *Testnet, ctx context.Context, vs VerificationSlot, bn *BeaconNode, hash ethcommon.Hash) (*bellatrix.SignedBeaconBlock, error) {
	lastSlot, err := vs.Slot(t, ctx, bn)
	if err != nil {
		return nil, err
	}
	/*
		enr, _ := bn.ENR()
		t.t.Logf("INFO: Looking for block %v in node %v, from slot %d", hash, enr, lastSlot)
	*/
	for slot := lastSlot; slot > 0; slot -= 1 {
		var versionedBlock eth2api.VersionedSignedBeaconBlock
		if exists, err := beaconapi.BlockV2(ctx, bn.API, eth2api.BlockIdSlot(slot), &versionedBlock); err != nil {
			// t.t.Logf("INFO: Error getting block at slot %d: %v", slot, err)
			continue
		} else if !exists {
			// t.t.Logf("INFO: Error getting block at slot %d", slot)
			continue
		}
		if versionedBlock.Version != "bellatrix" {
			// Block can't contain an executable payload
			// t.t.Logf("INFO: Breaking at slot %d", slot)
			break
		}
		block := versionedBlock.Data.(*bellatrix.SignedBeaconBlock)
		payload := block.Message.Body.ExecutionPayload
		/*
			payloadS, _ := json.MarshalIndent(payload, "", "  ")
			t.t.Logf("DEBUG: Comparing payload of slot %d: %s", slot, payloadS)
		*/
		if bytes.Compare(payload.BlockHash[:], hash[:]) == 0 {
			// t.t.Logf("INFO: Execution block %v found in %d: %v", hash, block.Message.Slot, ethcommon.BytesToHash(payload.BlockHash[:]))
			return block, nil
		}
	}
	return nil, nil
}

// VerifyProposers checks that all validator clients have proposed a block on
// the finalized beacon chain that includes an execution payload.
func VerifyProposers(t *Testnet, ctx context.Context, vs VerificationSlot, allow_empty_blocks bool) error {
	lastSlot, err := vs.Slot(t, ctx, t.verificationBeacons()[0])
	if err != nil {
		return err
	}
	proposers := make([]bool, len(t.verificationBeacons()))
	for slot := common.Slot(0); slot <= lastSlot; slot += 1 {
		var versionedBlock eth2api.VersionedSignedBeaconBlock
		if exists, err := beaconapi.BlockV2(ctx, t.verificationBeacons()[0].API, eth2api.BlockIdSlot(slot), &versionedBlock); err != nil {
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
		if exists, err := beaconapi.StateValidator(ctx, t.verificationBeacons()[0].API, eth2api.StateIdSlot(slot), eth2api.ValidatorIdIndex(proposerIndex), &validator); err != nil {
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

func VerifyELHeads(t *Testnet, ctx context.Context) error {
	client := ethclient.NewClient(t.verificationExecution()[0].RPC())
	head, err := client.HeaderByNumber(ctx, nil)
	if err != nil {
		return err
	}

	t.t.Logf("Verifying EL heads at %v", head.Hash())
	for i, node := range t.verificationExecution() {
		client := ethclient.NewClient(node.RPC())
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
