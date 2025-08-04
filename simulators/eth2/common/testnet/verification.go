package testnet

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"math/big"
	"time"

	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/hive/simulators/eth2/common/utils"
	beacon_client "github.com/marioevz/eth-clients/clients/beacon"
	"github.com/protolambda/eth2api"
	"github.com/protolambda/zrnt/eth2/beacon/common"
	"github.com/protolambda/zrnt/eth2/beacon/deneb"
	"github.com/protolambda/ztyp/tree"
)

// Interface to specify on which slot the verification will be performed
type VerificationSlot interface {
	Slot(
		ctx context.Context,
		t *Testnet,
		bn *beacon_client.BeaconClient,
	) (common.Slot, error)
}

// Return the slot at the start of the checkpoint's following epoch
type FirstSlotAfterCheckpoint struct {
	*common.Checkpoint
}

var _ VerificationSlot = FirstSlotAfterCheckpoint{}

func (c FirstSlotAfterCheckpoint) Slot(
	ctx context.Context,
	t *Testnet,
	_ *beacon_client.BeaconClient,
) (common.Slot, error) {
	return t.Spec().EpochStartSlot(c.Checkpoint.Epoch + 1)
}

// Return the slot at the end of a checkpoint
type LastSlotAtCheckpoint struct {
	*common.Checkpoint
}

var _ VerificationSlot = LastSlotAtCheckpoint{}

func (c LastSlotAtCheckpoint) Slot(
	ctx context.Context,
	t *Testnet,
	_ *beacon_client.BeaconClient,
) (common.Slot, error) {
	return t.Spec().SLOTS_PER_EPOCH * common.Slot(c.Checkpoint.Epoch), nil
}

// Get last slot according to current time
type LastestSlotByTime struct{}

var _ VerificationSlot = LastestSlotByTime{}

func (l LastestSlotByTime) Slot(
	ctx context.Context,
	t *Testnet,
	_ *beacon_client.BeaconClient,
) (common.Slot, error) {
	return t.Spec().
			TimeToSlot(common.Timestamp(time.Now().Unix()), t.GenesisTime()),
		nil
}

// Get last slot according to current head of a beacon node
type LastestSlotByHead struct{}

var _ VerificationSlot = LastestSlotByHead{}

func (l LastestSlotByHead) Slot(
	ctx context.Context,
	t *Testnet,
	bn *beacon_client.BeaconClient,
) (common.Slot, error) {
	headInfo, err := bn.BlockHeader(ctx, eth2api.BlockHead)
	if err != nil {
		return common.Slot(0), fmt.Errorf("failed to poll head: %v", err)
	}
	return headInfo.Header.Message.Slot, nil
}

// VerifyParticipation ensures that the participation of the finialized epoch
// of a given checkpoint is above the expected threshold.
func (t *Testnet) VerifyParticipation(
	parentCtx context.Context,
	vs VerificationSlot,
	expected float64,
) error {
	runningNodes := t.VerificationNodes().Running()
	slot, err := vs.Slot(parentCtx, t, runningNodes[0].BeaconClient)
	if err != nil {
		return err
	}
	if t.Spec().BELLATRIX_FORK_EPOCH <= t.Spec().SlotToEpoch(slot) {
		// slot-1 to target last slot in finalized epoch
		slot = slot - 1
	}
	for i, n := range runningNodes {
		health, err := GetHealth(
			parentCtx,
			n.BeaconClient,
			t.Spec().Spec,
			slot,
		)
		if err != nil {
			return err
		}
		if health < expected {
			return fmt.Errorf(
				"node %d (%s): participation not healthy (got:%.2f, want:%.2f)",
				i,
				n.ClientNames(),
				health,
				expected,
			)
		}
		t.Logf(
			"node %d (%s): epoch=%d participation=%.2f",
			i,
			n.ClientNames(),
			t.Spec().SlotToEpoch(slot),
			health,
		)
	}
	return nil
}

// VerifyExecutionPayloadIsCanonical retrieves the execution payload from the
// finalized block and verifies that is in the execution client's canonical
// chain.
func (t *Testnet) VerifyExecutionPayloadIsCanonical(
	parentCtx context.Context,
	vs VerificationSlot,
) error {
	runningNodes := t.VerificationNodes().Running()
	b := runningNodes[0].BeaconClient
	slot, err := vs.Slot(parentCtx, t, b)
	if err != nil {
		return err
	}

	versionedBlock, err := b.BlockV2(
		parentCtx,
		eth2api.BlockIdSlot(slot),
	)
	if err != nil {
		return fmt.Errorf(
			"node %d (%s): failed to retrieve block: %v",
			0,
			runningNodes[0].ClientNames(),
			err,
		)
	}

	payload, _, _, err := versionedBlock.ExecutionPayload()
	if err != nil {
		return err
	}

	for i, n := range runningNodes {
		ec := n.ExecutionClient
		if block, err := ec.BlockByNumber(
			parentCtx,
			big.NewInt(int64(payload.Number)),
		); err != nil {
			return fmt.Errorf("eth1 %d: %s", 0, err)
		} else {
			blockHash := block.Hash()
			if !bytes.Equal(blockHash[:], payload.BlockHash[:]) {
				return fmt.Errorf(
					"node %d (%s): execution blocks don't match (got=%s, expected=%s)",
					i,
					n.ClientNames(),
					utils.Shorten(blockHash.String()),
					utils.Shorten(payload.BlockHash.String()),
				)
			}
		}
	}
	return nil
}

// VerifyExecutionPayloadIsCanonical retrieves the execution payload from the
// finalized block and verifies that is in the execution client's canonical
// chain.
func (t *Testnet) VerifyExecutionPayloadHashInclusion(
	parentCtx context.Context,
	vs VerificationSlot,
	hash ethcommon.Hash,
) (*beacon_client.VersionedSignedBeaconBlock, error) {
	for _, bn := range t.VerificationNodes().BeaconClients().Running() {
		b, err := t.VerifyExecutionPayloadHashInclusionNode(
			parentCtx,
			vs,
			bn,
			hash,
		)
		if err != nil || b != nil {
			return b, err
		}
	}
	return nil, nil
}

func (t *Testnet) VerifyExecutionPayloadHashInclusionNode(
	parentCtx context.Context,
	vs VerificationSlot,
	bn *beacon_client.BeaconClient,
	hash ethcommon.Hash,
) (*beacon_client.VersionedSignedBeaconBlock, error) {
	lastSlot, err := vs.Slot(parentCtx, t, bn)
	if err != nil {
		return nil, err
	}
	for slot := lastSlot; slot > 0; slot -= 1 {
		versionedBlock, err := bn.BlockV2(parentCtx, eth2api.BlockIdSlot(slot))
		if err != nil {
			continue
		}
		if executionPayload, _, _, err := versionedBlock.ExecutionPayload(); err != nil {
			// Block can't contain an executable payload
			break
		} else if bytes.Equal(executionPayload.BlockHash[:], hash[:]) {
			return versionedBlock, nil
		}

	}
	return nil, nil
}

// VerifyProposers checks that all validator clients have proposed a block on
// the finalized beacon chain that includes an execution payload.
func (t *Testnet) VerifyProposers(
	parentCtx context.Context,
	vs VerificationSlot,
	allow_empty_blocks bool,
) error {
	runningNodes := t.VerificationNodes().Running()
	bn := runningNodes[0].BeaconClient
	lastSlot, err := vs.Slot(parentCtx, t, bn)
	if err != nil {
		return err
	}
	proposers := make([]bool, len(runningNodes))
	for slot := common.Slot(0); slot <= lastSlot; slot += 1 {
		versionedBlock, err := bn.BlockV2(parentCtx, eth2api.BlockIdSlot(slot))
		if err != nil {
			if allow_empty_blocks {
				continue
			}
			return fmt.Errorf(
				"node %d (%s): failed to retrieve beacon block: %v",
				0,
				runningNodes[0].ClientNames(),
				err,
			)
		}

		validator, err := bn.StateValidator(
			parentCtx,
			eth2api.StateIdSlot(slot),
			eth2api.ValidatorIdIndex(versionedBlock.ProposerIndex()),
		)
		if err != nil {
			return fmt.Errorf(
				"node %d (%s): failed to retrieve validator: %v",
				0,
				runningNodes[0].ClientNames(),
				err,
			)
		}
		idx, err := t.ValidatorClientIndex(
			[48]byte(validator.Validator.Pubkey),
		)
		if err != nil {
			return fmt.Errorf("pub key not found on any validator client")
		}
		proposers[idx] = true
	}
	for i, proposed := range proposers {
		if !proposed {
			return fmt.Errorf(
				"node %d (%s): did not propose a block",
				i,
				runningNodes[i].ClientNames(),
			)
		}
	}
	return nil
}

// GetProposer returns the node id of the proposer for the requested slot.
func (t *Testnet) GetProposer(
	parentCtx context.Context,
	vs VerificationSlot,
) (int, error) {
	runningNodes := t.VerificationNodes().Running()
	bn := runningNodes[0].BeaconClient
	slot, err := vs.Slot(parentCtx, t, bn)
	if err != nil {
		return 0, err
	}

	versionedBlock, err := bn.BlockV2(parentCtx, eth2api.BlockIdSlot(slot))
	if err != nil {
		return 0, fmt.Errorf(
			"node %d (%s): failed to retrieve beacon block: %v",
			0,
			runningNodes[0].ClientNames(),
			err,
		)
	}

	validator, err := bn.StateValidator(
		parentCtx,
		eth2api.StateIdSlot(slot),
		eth2api.ValidatorIdIndex(versionedBlock.ProposerIndex()),
	)
	if err != nil {
		return 0, fmt.Errorf(
			"node %d (%s): failed to retrieve validator: %v",
			0,
			runningNodes[0].ClientNames(),
			err,
		)
	}

	return t.ValidatorClientIndex(
		[48]byte(validator.Validator.Pubkey),
	)
}

func (t *Testnet) VerifyELBlockLabels(parentCtx context.Context) error {
	runningNodes := t.VerificationNodes().Running()
	for i := 0; i < len(runningNodes); i++ {
		n := runningNodes[i]
		el := n.ExecutionClient
		bn := n.BeaconClient
		// Get the head
		headInfo, err := bn.BlockHeader(parentCtx, eth2api.BlockHead)
		if err != nil {
			return err
		}

		// Get the checkpoints, first try querying state root, then slot number
		checkpoints, err := bn.BlockFinalityCheckpoints(
			parentCtx,
			eth2api.BlockHead,
		)
		if err != nil {
			return err
		}
		blockLabels := map[string]tree.Root{
			"latest":    headInfo.Root,
			"finalized": checkpoints.Finalized.Root,
			"safe":      checkpoints.CurrentJustified.Root,
		}

		for label, root := range blockLabels {
			// Get the beacon block
			versionedBlock, err := bn.BlockV2(
				parentCtx,
				eth2api.BlockIdRoot(root),
			)
			if err != nil {
				return err
			}
			if executionPayload, _, _, err := versionedBlock.ExecutionPayload(); err != nil {
				// Get the el block and compare
				h, err := el.HeaderByLabel(parentCtx, label)
				if err != nil {
					if executionPayload.BlockHash != (ethcommon.Hash{}) {
						return err
					}
				} else {
					if h.Hash() != executionPayload.BlockHash {
						return fmt.Errorf(
							"node %d (%s): Execution hash found in checkpoint block "+
								"(%s) does not match what the el returns: %v != %v",
							i,
							n.ClientNames(),
							label,
							executionPayload.BlockHash,
							h.Hash(),
						)
					}
					fmt.Printf(
						"node %d (%s): Execution hash matches beacon "+
							"checkpoint block (%s) information: %v\n",
						i,
						n.ClientNames(),
						label,
						h.Hash(),
					)
				}
			}

		}
	}
	return nil
}

func (t *Testnet) VerifyELHeads(
	parentCtx context.Context,
) error {
	runningExecution := t.VerificationNodes().ExecutionClients().Running()
	head, err := runningExecution[0].HeaderByNumber(parentCtx, nil)
	if err != nil {
		return err
	}

	t.Logf("Verifying EL heads at %v", head.Hash())
	for i, node := range runningExecution {
		head2, err := node.HeaderByNumber(parentCtx, nil)
		if err != nil {
			return err
		}
		if head.Hash() != head2.Hash() && head.ParentHash != head2.Hash() && head.Hash() != head2.ParentHash {
			return fmt.Errorf(
				"different heads: %v: %v %v: %v",
				0,
				head,
				i,
				head2,
			)
		}
	}
	return nil
}

func (t *Testnet) VerifyBlobs(
	parentCtx context.Context,
	vs VerificationSlot,
) (uint64, error) {
	nodes := t.VerificationNodes().Running()
	beaconClients := nodes.BeaconClients()
	if len(nodes) == 0 {
		return 0, fmt.Errorf("no beacon clients running")
	}
	if len(nodes) == 1 {
		return 0, fmt.Errorf("only one beacon client running, can't verify blobs")
	}
	var blobCount uint64

	lastSlot, err := vs.Slot(parentCtx, t, beaconClients[0])
	if err != nil {
		return 0, err
	}
	for slot := lastSlot; slot > 0; slot -= 1 {

		versionedBlock, err := beaconClients[0].BlockV2(parentCtx, eth2api.BlockIdSlot(slot))
		if err != nil {
			continue
		}
		if !versionedBlock.ContainsKZGCommitments() {
			// Block can't contain blobs before deneb
			break
		}

		// Get the block root
		blockRoot := versionedBlock.Root()

		// Get the execution block from the execution client
		executionPayload, _, _, err := versionedBlock.ExecutionPayload()
		if err != nil {
			panic(err)
		}
		executionBlock, err := nodes[0].ExecutionClient.BlockByHash(parentCtx, executionPayload.BlockHash)
		if err != nil {
			panic(err)
		}
		blockKzgCommitments := versionedBlock.KZGCommitments()

		refSidecars, err := beaconClients[0].BlobSidecars(parentCtx, eth2api.BlockIdSlot(slot))
		if err != nil && len(blockKzgCommitments) > 0 { // Prysm returns error on empty blobs
			return 0, fmt.Errorf(
				"node %d (%s): failed to retrieve blobs for slot %d: %v",
				0,
				t.VerificationNodes().Running()[0].ClientNames(),
				slot,
				err,
			)
		}
		blobCount += uint64(len(refSidecars))

		if len(refSidecars) != len(blockKzgCommitments) {
			return 0, fmt.Errorf(
				"node %d (%s): slot %d, block kzg commitments and sidecars length differ (sidecar count=%d, block kzg commitments=%d)",
				0,
				t.VerificationNodes().Running()[0].ClientNames(),
				slot,
				len(refSidecars),
				len(blockKzgCommitments),
			)
		}

		// Verify against the execution block transactions
		executionBlockHashesCount := 0
		for _, tx := range executionBlock.Transactions() {
			blobHashes := tx.BlobHashes()
			versionedHashVersion := byte(1)
			if len(blobHashes) > 0 {
				for _, blobHash := range blobHashes {
					if executionBlockHashesCount < len(blockKzgCommitments) {
						// Sha256 the kzg commitment and modify the first byte to be the version
						hasher := sha256.New()
						hasher.Write(blockKzgCommitments[executionBlockHashesCount][:])
						kzgHash := hasher.Sum(nil)
						if !bytes.Equal(blobHash[1:], kzgHash[1:]) {
							return 0, fmt.Errorf(
								"node %d (%s): slot %d, block kzg commitments and execution block hashes differ (block kzg commitment=%x, execution block hash=%x)",
								0,
								t.VerificationNodes().Running()[0].ClientNames(),
								slot,
								blockKzgCommitments[executionBlockHashesCount][:],
								blobHash[:],
							)
						}
						if blobHash[0] != versionedHashVersion {
							return 0, fmt.Errorf(
								"node %d (%s): slot %d, execution blob hash does not contain the correct version: %d",
								0,
								t.VerificationNodes().Running()[0].ClientNames(),
								slot,
								blobHash[0],
							)
						}
					} // else: test will fail after the loop, but we need to keep counting the hashes
					executionBlockHashesCount++
				}
			}
		}
		if executionBlockHashesCount != len(blockKzgCommitments) {
			return 0, fmt.Errorf(
				"node %d (%s): slot %d, block kzg commitments and execution block hashes length differ (block kzg commitment count=%d, execution block hash count=%d)",
				0,
				t.VerificationNodes().Running()[0].ClientNames(),
				slot,
				len(blockKzgCommitments),
				executionBlockHashesCount,
			)
		}

		// Verify sidecars integrity
		for i, sidecar := range refSidecars {
			// Marshal the sidecar to json for logging
			jsonSidecar, _ := json.MarshalIndent(sidecar, "", "  ")

			// Check the blob index
			if sidecar.Index != deneb.BlobIndex(i) {
				t.Logf("INFO: sidecar: %s", jsonSidecar)
				return 0, fmt.Errorf(
					"node %d (%s): slot %d, blob index does not match (got=%d, expected=%d)",
					0,
					t.VerificationNodes().Running()[0].ClientNames(),
					slot,
					sidecar.Index,
					i,
				)
			}

			// Verify the inclusion proof
			if err := sidecar.VerifyProof(tree.GetHashFn()); err != nil {
				// json log output the sidecar
				t.Logf("INFO: sidecar: %s", jsonSidecar)
				return 0, fmt.Errorf(
					"node %d (%s): slot %d, failed to verify proof: %v",
					0,
					t.VerificationNodes().Running()[0].ClientNames(),
					slot,
					err,
				)
			}

			// Check the block root of included signed header
			sidecarBlockRoot := sidecar.SignedBlockHeader.Message.HashTreeRoot(tree.GetHashFn())
			if !bytes.Equal(sidecarBlockRoot[:], blockRoot[:]) {
				t.Logf("INFO: sidecar: %s", jsonSidecar)
				return 0, fmt.Errorf(
					"node %d (%s): block root of included signed header does not match (got=%x, expected=%x)",
					0,
					t.VerificationNodes().Running()[0].ClientNames(),
					sidecarBlockRoot[:],
					blockRoot[:],
				)
			}

			// Compare kzg commitment against the one in the block
			if !bytes.Equal(sidecar.KZGCommitment[:], blockKzgCommitments[i][:]) {
				t.Logf("INFO: sidecar: %s", jsonSidecar)
				return 0, fmt.Errorf(
					"node %d (%s): block kzg commitment and sidecar kzg commitment differ (block kzg commitment=%x, sidecar kzg commitment=%x)",
					0,
					t.VerificationNodes().Running()[0].ClientNames(),
					blockKzgCommitments[i][:],
					sidecar.KZGCommitment[:],
				)
			}

			// Compare the signature against the signed block
			blockSignature := versionedBlock.Signature()
			if !bytes.Equal(sidecar.SignedBlockHeader.Signature[:], blockSignature[:]) {
				t.Logf("INFO: sidecar: %s", jsonSidecar)
				return 0, fmt.Errorf(
					"node %d (%s): block signature and sidecar signature differ (block signature=%s, sidecar block header signature=%s)",
					0,
					t.VerificationNodes().Running()[0].ClientNames(),
					blockSignature.String(),
					sidecar.SignedBlockHeader.Signature.String(),
				)
			}

			// TODO: Verify KZG Proof
		}

		// Verify against the other beacon nodes
		for i := 1; i < len(beaconClients); i++ {
			// Check the reference client against the other clients, and verify that all clients return the same blobs
			//  Also keep count of blobs per client
			bn := beaconClients[i]
			// Get the sidecars for this client
			sidecars, err := bn.BlobSidecars(parentCtx, eth2api.BlockIdSlot(slot))
			if err != nil && len(blockKzgCommitments) > 0 { // DEBUG!!! DO NOT MERGE!!!
				// We already got some blobs from the reference client, so we should not get an error here
				return 0, err
			}
			if len(sidecars) != len(refSidecars) {
				return 0, fmt.Errorf(
					"node %d (%s): different number of blobs (got=%d, expected=%d)",
					i,
					t.VerificationNodes().Running()[i].ClientNames(),
					len(sidecars),
					len(refSidecars),
				)
			}
			for j := 0; j < len(sidecars); j++ {
				refSidecarRoot := refSidecars[j].HashTreeRoot(t.spec, tree.GetHashFn())
				sidecarRoot := sidecars[j].HashTreeRoot(t.spec, tree.GetHashFn())

				// Verify the sidecar roots match
				if !bytes.Equal(sidecarRoot[:], refSidecarRoot[:]) {
					return 0, fmt.Errorf(
						"node %d (%s): different sidecar roots\ngot=%s\nexpected=%s",
						i,
						bn.ClientName(),
						sidecarRoot.String(),
						refSidecarRoot.String(),
					)
				}
			}
		}
	}
	return blobCount, nil
}
