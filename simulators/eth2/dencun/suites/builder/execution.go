package suite_builder

import (
	"bytes"
	"context"
	"encoding/json"
	"math/big"

	"github.com/ethereum/hive/hivesim"
	tn "github.com/ethereum/hive/simulators/eth2/common/testnet"
	mock_builder "github.com/marioevz/mock-builder/mock"
	beacon "github.com/protolambda/zrnt/eth2/beacon/common"
)

const (
	MAX_MISSED_SLOTS_BEFORE_CIRCUIT_BREAKER uint64 = 10
	MAX_MISSED_SLOTS_NO_CIRCUIT_BREAKER     uint64 = 1
)

var Deneb string = "deneb"

// Wait for the fork to happen.
// Under normal circumstances, the fork should happen at the specified slot.
// If a test injects an error at the time of the fork, it's possible that we
// will miss slots until the circuit breaker is active, so we need to increase
// the timeout.
func (ts BuilderTestSpec) WaitForFork(
	t *hivesim.T,
	ctx context.Context,
	testnet *tn.Testnet,
	env *tn.Environment,
	config *tn.Config,
) {
	extraSlotsTimeout := beacon.Slot(4)
	if ts.CausesMissedSlot() {
		// The error cannot be caught by the consensus client until the payload is revealed,
		// and the beacon block had been signed. Therefore there will be missed slots
		// until the circuit breaker kicks in.
		extraSlotsTimeout += testnet.Spec().SLOTS_PER_EPOCH
	}
	slotsUntilFork := beacon.Slot(
		config.DenebForkEpoch.Uint64(),
	)*testnet.Spec().SLOTS_PER_EPOCH + extraSlotsTimeout
	timeoutCtx, cancel := testnet.Spec().SlotTimeoutContext(ctx, slotsUntilFork)
	defer cancel()
	if err := testnet.WaitForFork(timeoutCtx, Deneb); err != nil {
		t.Fatalf("FAIL: error waiting for deneb: %v", err)
	}
}

// Builder testnet.
func (ts BuilderTestSpec) ExecutePostFork(
	t *hivesim.T,
	ctx context.Context,
	testnet *tn.Testnet,
	env *tn.Environment,
	config *tn.Config,
) {
	// Run the base test spec execute function
	ts.BaseTestSpec.ExecutePostFork(t, ctx, testnet, env, config)

	// Check that the builder was working properly until now
	if !ts.DenebGenesis {
		for i, b := range testnet.BeaconClients().Running() {
			builder, ok := b.Builder.(*mock_builder.MockBuilder)
			if !ok {
				t.Fatalf(
					"FAIL: client %d (%s) is not a mock builder",
					i,
					b.ClientName(),
				)
			}
			if builder.GetBuiltPayloadsCount() == 0 {
				t.Fatalf("FAIL: builder %d did not build any payloads", i)
			}
			if builder.GetSignedBeaconBlockCount() == 0 {
				t.Fatalf(
					"FAIL: builder %d did not produce any signed beacon blocks",
					i,
				)
			}
		}
	}
}

func (ts BuilderTestSpec) BuilderProducesValidPayload() bool {
	// If true, the builder is expected to produce a valid payload that should
	// be included in the canonical chain.
	if ts.ErrorOnHeaderRequest || ts.InvalidPayloadVersion ||
		ts.ErrorOnPayloadReveal || ts.InvalidatePayload != "" ||
		ts.InvalidatePayloadAttributes != "" {
		return false
	}
	return true
}

func (ts BuilderTestSpec) CausesMissedSlot() bool {
	// If true, the test is expected to cause a missed slot.
	if ts.ErrorOnHeaderRequest || ts.InvalidPayloadVersion {
		// An error on header request to the builder should fallback to local block production,
		// hence no missed slot.
		return false
	}
	if ts.ErrorOnPayloadReveal {
		// An error on payload reveal means that, since the mock builder does _not_ relay unblinded
		// payloads to the p2p network, the consensus client will not be able to produce a block
		// for the slot, and hence a missed slot is expected.
		return true
	}
	if ts.InvalidatePayloadAttributes == "" && ts.InvalidatePayload == "" {
		// If no payload is invalidated, then no missed slot is expected.
		return false
	}
	if ts.InvalidatePayload != "" {
		// An invalid payload cannot be detected by the consensus client until the payload is
		// revealed, and the beacon block had been signed. Therefore there will be missed slots.
		return true
	}
	if ts.InvalidatePayloadAttributes == mock_builder.INVALIDATE_ATTR_BEACON_ROOT {
		// Invalid Beacon Root is special because the modified value is not in the response,
		// and hence can only be detected by the consensus client after the payload is
		// revealed and executed.
		return true
	}
	// The rest of payload attributes invalidations can be detected by the consensus client
	return false
}

func (ts BuilderTestSpec) Verify(
	t *hivesim.T,
	ctx context.Context,
	testnet *tn.Testnet,
	env *tn.Environment,
	config *tn.Config,
) {
	// Run the base test spec verify function
	ts.BaseTestSpec.Verify(t, ctx, testnet, env, config)

	// Verify any modified payloads did not make it into the
	// canonical chain
	if !ts.ErrorOnHeaderRequest && !ts.ErrorOnPayloadReveal &&
		!ts.InvalidPayloadVersion &&
		ts.InvalidatePayload == "" &&
		ts.InvalidatePayloadAttributes == "" {
		// Simply verify that builder's deneb payloads were included in the
		// canonical chain
		t.Logf("INFO: Verifying builder payloads were included in the canonical chain")
		for i, n := range testnet.Nodes.Running() {
			b, ok := n.BeaconClient.Builder.(*mock_builder.MockBuilder)
			if !ok {
				t.Fatalf(
					"FAIL: client %d (%s) is not a mock builder",
					i,
					n.BeaconClient.ClientName(),
				)
			}
			ec := n.ExecutionClient
			includedPayloads := 0
			includedPayloadsWithBlobs := 0
			for _, p := range b.GetBuiltPayloads() {
				p, _, err := p.FullPayload().ToExecutableData()
				if err != nil {
					t.Fatalf(
						"FAIL: error converting payload to executable data: %v",
						err,
					)
				}
				if testnet.ExecutionGenesis().Config.CancunTime == nil {
					t.Fatalf("FAIL: Cancun time is nil")
				}
				if p.Timestamp >= *testnet.ExecutionGenesis().Config.CancunTime {
					if h, err := ec.HeaderByNumber(ctx, big.NewInt(int64(p.Number))); err != nil {
						t.Fatalf(
							"FAIL: error getting execution header from node %d: %v",
							i,
							err,
						)
					} else if h != nil {
						hash := h.Hash()
						if bytes.Equal(hash[:], p.BlockHash[:]) {
							includedPayloads++
							// On deneb we also need to make sure at least one payload has blobs
							if p.BlobGasUsed != nil && *p.BlobGasUsed > 0 {
								includedPayloadsWithBlobs++
							}
						}
					}
				}
			}
			if includedPayloads == 0 {
				t.Fatalf(
					"FAIL: builder %d did not produce deneb payloads included in the canonical chain",
					i,
				)
			}
			if includedPayloadsWithBlobs == 0 {
				t.Fatalf(
					"FAIL: builder %d did not produce deneb payloads with blobs included in the canonical chain",
					i,
				)
			}
		}
	} else {
		// TODO: INVALIDATE_ATTR_BEACON_ROOT cannot be detected by the consensus client
		//       on a blinded payload
		t.Logf("INFO: Verifying builder payloads were NOT included in the canonical chain")
		for i, n := range testnet.VerificationNodes().Running() {
			b, ok := n.BeaconClient.Builder.(*mock_builder.MockBuilder)
			if !ok {
				t.Fatalf(
					"FAIL: client %d (%s) is not a mock builder",
					i,
					n.BeaconClient.ClientName(),
				)
			}
			builtPayloads := b.GetBuiltPayloads()
			if len(builtPayloads) == 0 {
				if ts.ErrorOnHeaderRequest {
					continue
				}
				t.Fatalf("FAIL: No payloads were modified by builder %d", i)
			}
			for s, p := range builtPayloads {
				if s >= beacon.Slot(
					config.DenebForkEpoch.Uint64(),
				)*testnet.Spec().SLOTS_PER_EPOCH {
					payload, _, err := p.FullPayload().ToExecutableData()
					if err != nil {
						t.Fatalf(
							"FAIL: error converting payload to executable data: %v",
							err,
						)
					}
					for i, ec := range testnet.ExecutionClients().Running() {
						b, err := ec.BlockByNumber(
							ctx,
							new(big.Int).SetUint64(payload.Number),
						)
						if err == nil {
							// If block is found, verify that this is different from the modified payload
							h := b.Hash()
							if bytes.Equal(h[:], payload.BlockHash[:]) {
								t.Fatalf(
									"FAIL: node %d: modified payload included in canonical chain: %d (%s)",
									i,
									payload.Number,
									payload.BlockHash,
								)
							}
						} // else: block not found, payload not included in canonical chain
					}
				}
			}
			t.Logf(
				"INFO: No modified payloads were included in canonical chain of node %d",
				i,
			)
		}
	}

	// Count, print and verify missed slots
	if ts.VerifyMissedSlotsCount {
		if count, err := testnet.BeaconClients().Running()[0].GetFilledSlotsCountPerEpoch(ctx); err != nil {
			t.Fatalf("FAIL: unable to obtain slot count per epoch: %v", err)
		} else {
			for ep, slots := range count {
				t.Logf("INFO: Epoch %d, filled slots=%d", ep, slots)
			}

			var max_missed_slots uint64 = 0
			if !ts.CausesMissedSlot() {
				// These errors should be caught by the CL client when the built blinded
				// payload is received. Hence, a low number of missed slots is expected.
				max_missed_slots = MAX_MISSED_SLOTS_NO_CIRCUIT_BREAKER
			} else {
				// All other errors cannot be caught by the CL client until the
				// payload is revealed, and the beacon block had been signed.
				// Hence, a high number of missed slots is expected because the
				// circuit breaker is a mechanism that only kicks in after many
				// missed slots.
				max_missed_slots = MAX_MISSED_SLOTS_BEFORE_CIRCUIT_BREAKER
			}

			denebEpoch := beacon.Epoch(config.DenebForkEpoch.Uint64())

			if count[denebEpoch] < uint64(testnet.Spec().SLOTS_PER_EPOCH)-max_missed_slots {
				t.Fatalf(
					"FAIL: Epoch %d should have at least %d filled slots, but has %d",
					denebEpoch,
					uint64(testnet.Spec().SLOTS_PER_EPOCH)-max_missed_slots,
					count[denebEpoch],
				)
			}

		}
	}

	// Verify all submited blinded beacon blocks have correct signatures
	for i, n := range testnet.Nodes.Running() {
		b, ok := n.BeaconClient.Builder.(*mock_builder.MockBuilder)
		if !ok {
			t.Fatalf(
				"FAIL: client %d (%s) is not a mock builder",
				i,
				n.BeaconClient.ClientName(),
			)
		}

		if b.GetValidationErrorsCount() > 0 {
			// Validation errors should never happen, this means the submited blinded
			// beacon response received from the consensus client was incorrect.
			validationErrorsMap := b.GetValidationErrors()
			for slot, validationError := range validationErrorsMap {
				signedBeaconResponse, ok := b.GetSignedBeaconBlock(slot)
				if ok {
					signedBeaconResponseJson, _ := json.MarshalIndent(signedBeaconResponse, "", "  ")
					t.Logf(
						"INFO: builder %d encountered a validation error on slot %d: %v\n%s",
						i,
						slot,
						validationError,
						signedBeaconResponseJson,
					)
				}
				t.Fatalf(
					"FAIL: builder %d encountered a validation error on slot %d: %v",
					i,
					slot,
					validationError,
				)
			}
		}
	}
	t.Logf(
		"INFO: Validated all signatures of beacon blocks received by builders",
	)
}
