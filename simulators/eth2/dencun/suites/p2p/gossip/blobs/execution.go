package suite_blobs_gossip

import (
	"bytes"
	"context"

	"github.com/ethereum/hive/hivesim"

	tn "github.com/ethereum/hive/simulators/eth2/common/testnet"
	blobber_common "github.com/marioevz/blobber/common"
	"github.com/protolambda/eth2api"
	"github.com/protolambda/zrnt/eth2/beacon/common"
)

func KZGListContains(list common.KZGCommitments, kzg common.KZGCommitment) bool {
	for _, l := range list {
		if bytes.Equal(kzg[:], l[:]) {
			return true
		}
	}
	return false
}

func (ts P2PBlobsGossipTestSpec) Verify(
	t *hivesim.T,
	ctx context.Context,
	testnet *tn.Testnet,
	env *tn.Environment,
	config *tn.Config,
) {
	// Do base verifications
	ts.BaseTestSpec.Verify(t, ctx, testnet, env, config)

	// Check that the blobber produced something
	if len(testnet.Blobber().GetProducedBlockRoots()) == 0 {
		t.Fatalf("FAIL: Blobber did not produce any blocks")
	}

	// Check the must-include/reject blobs, and make sure that the must-include blobs are in the chain whereas the must-reject blobs are not
	mustIncludeBlobs := testnet.Blobber().IncludeBlobRecord()
	mustRejectBlobs := testnet.Blobber().RejectBlobRecord()
	allSlots := blobber_common.GetAllSlots(mustIncludeBlobs, mustRejectBlobs)

	for _, slot := range allSlots {
		for i, bn := range testnet.BeaconClients().Running() {
			versionedBlock, err := bn.BlockV2(ctx, eth2api.BlockIdSlot(slot))
			if err != nil {
				continue
			}
			blockKzgCommitments := versionedBlock.KZGCommitments()

			// Check that the must-include blobs are in the chain
			for _, includeKzg := range mustIncludeBlobs.Get(slot) {
				if !KZGListContains(blockKzgCommitments, includeKzg) {
					t.Fatalf("FAIL: Good blob missing from the chain: node=%d, slot=%d, kzg_commitment=%s", i, slot, includeKzg)
				}
			}

			// Check that the must-reject blobs are not in the chain
			for _, rejectKzg := range mustRejectBlobs.Get(slot) {
				if KZGListContains(blockKzgCommitments, rejectKzg) {
					t.Fatalf("FAIL: Bad blob in the chain: node=%d, slot=%d, kzg_commitment=%s", i, slot, rejectKzg)
				}
			}

		}
	}
}
