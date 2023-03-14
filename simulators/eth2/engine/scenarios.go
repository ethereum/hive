package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"strings"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	api "github.com/ethereum/go-ethereum/beacon/engine"
	"github.com/ethereum/hive/hivesim"
	"github.com/ethereum/hive/simulators/eth2/common/chain_generators/pow"
	"github.com/ethereum/hive/simulators/eth2/common/clients"
	el "github.com/ethereum/hive/simulators/eth2/common/config/execution"
	"github.com/ethereum/hive/simulators/eth2/common/debug"
	payload_spoof "github.com/ethereum/hive/simulators/eth2/common/spoofing/payload"
	"github.com/ethereum/hive/simulators/eth2/common/spoofing/proxy"
	tn "github.com/ethereum/hive/simulators/eth2/common/testnet"
	"github.com/protolambda/eth2api"
	beacon "github.com/protolambda/zrnt/eth2/beacon/common"
	spoof "github.com/rauljordan/engine-proxy/proxy"
)

var (
	DEFAULT_VALIDATOR_COUNT           uint64 = 60
	DEFAULT_SLOT_TIME                 uint64 = 6
	DEFAULT_TERMINAL_TOTAL_DIFFICULTY uint64 = 100

	EPOCHS_TO_FINALITY beacon.Epoch = 4

	// Default config used for all tests unless a client specific config exists
	DEFAULT_CONFIG = &tn.Config{
		ValidatorCount: big.NewInt(int64(DEFAULT_VALIDATOR_COUNT)),
		SlotTime:       big.NewInt(int64(DEFAULT_SLOT_TIME)),
		TerminalTotalDifficulty: big.NewInt(
			int64(DEFAULT_TERMINAL_TOTAL_DIFFICULTY),
		),
		AltairForkEpoch:    common.Big0,
		BellatrixForkEpoch: common.Big0,
		Eth1Consensus:      &el.ExecutionCliqueConsensus{},
	}

	// Clients that do not support starting on epoch 0 with all forks enabled.
	// Tests take longer for these clients.
	INCREMENTAL_FORKS_CONFIG = &tn.Config{
		TerminalTotalDifficulty: big.NewInt(
			int64(DEFAULT_TERMINAL_TOTAL_DIFFICULTY) * 5,
		),
		AltairForkEpoch:    common.Big1,
		BellatrixForkEpoch: common.Big2,
	}
	INCREMENTAL_FORKS_CLIENTS = map[string]bool{
		"nimbus": true,
		"prysm":  true,
	}

	SAFE_SLOTS_TO_IMPORT_OPTIMISTICALLY_CLIENT_OVERRIDE = map[string]*big.Int{}
)

func getClientConfig(n clients.NodeDefinition) *tn.Config {
	config := DEFAULT_CONFIG
	if INCREMENTAL_FORKS_CLIENTS[n.ConsensusClient] {
		config = config.Join(INCREMENTAL_FORKS_CONFIG)
	}
	return config
}

func TransitionTestnet(t *hivesim.T, env *tn.Environment,
	n clients.NodeDefinition,
) {
	ctx := context.Background()
	config := getClientConfig(n).Join(&tn.Config{
		NodeDefinitions: []clients.NodeDefinition{
			n,
			n,
		},
	})

	testnet := tn.StartTestnet(ctx, t, env, config)
	defer testnet.Stop()

	finalityCtx, cancel := testnet.Spec().EpochTimeoutContext(
		ctx,
		EPOCHS_TO_FINALITY+1,
	)
	defer cancel()

	finalized, err := testnet.WaitForFinality(finalityCtx)
	if err != nil {
		t.Fatalf("FAIL: Waiting for finality: %v", err)
	}

	if err := testnet.VerifyParticipation(
		ctx,
		tn.FirstSlotAfterCheckpoint{Checkpoint: &finalized},
		0.95,
	); err != nil {
		t.Fatalf("FAIL: Verifying participation: %v", err)
	}

	if err := testnet.VerifyExecutionPayloadIsCanonical(
		ctx,
		tn.LastSlotAtCheckpoint{Checkpoint: &finalized},
	); err != nil {
		t.Fatalf("FAIL: Verifying execution payload is canonical: %v", err)
	}

	if err := testnet.VerifyProposers(
		ctx,
		tn.LastSlotAtCheckpoint{Checkpoint: &finalized},
		false,
	); err != nil {
		t.Fatalf("FAIL: Verifying proposers: %v", err)
	}
}

func TestRPCError(t *hivesim.T, env *tn.Environment,
	n clients.NodeDefinition,
) {
	ctx := context.Background()
	config := getClientConfig(n).Join(&tn.Config{
		NodeDefinitions: []clients.NodeDefinition{
			n,
			n,
		},
	})

	testnet := tn.StartTestnet(ctx, t, env, config)
	defer testnet.Stop()

	finalityCtx, cancel := testnet.Spec().EpochTimeoutContext(
		ctx,
		EPOCHS_TO_FINALITY+2,
	)
	defer cancel()

	finalized, err := testnet.WaitForExecutionFinality(finalityCtx)
	if err != nil {
		t.Fatalf("FAIL: Waiting for finality: %v", err)
	}

	if err := testnet.VerifyParticipation(
		ctx,
		tn.FirstSlotAfterCheckpoint{Checkpoint: &finalized},
		0.95,
	); err != nil {
		t.Fatalf("FAIL: Verifying participation: %v", err)
	}

	if err := testnet.VerifyExecutionPayloadIsCanonical(
		ctx,
		tn.LastSlotAtCheckpoint{Checkpoint: &finalized},
	); err != nil {
		t.Fatalf("FAIL: Verifying execution payload is canonical: %v", err)
	}

	if err := testnet.VerifyProposers(
		ctx,
		tn.LastSlotAtCheckpoint{Checkpoint: &finalized},
		true,
	); err != nil {
		t.Fatalf("FAIL: Verifying proposers: %v", err)
	}

	if err := testnet.VerifyELHeads(ctx); err != nil {
		t.Fatalf("FAIL: Verifying EL Heads: %v", err)
	}

	fields := make(map[string]interface{})
	fields["headBlockHash"] = "weird error"
	spoof := &spoof.Spoof{
		Method: EngineForkchoiceUpdatedV1,
		Fields: fields,
	}

	testnet.Proxies().Running()[0].AddRequest(spoof)

	time.Sleep(24 * time.Second)

	if err := testnet.VerifyParticipation(
		ctx,
		tn.FirstSlotAfterCheckpoint{Checkpoint: &finalized},
		0.95,
	); err != nil {
		t.Fatalf("FAIL: %v", err)
	}

	if err := testnet.VerifyELHeads(ctx); err == nil {
		t.Fatalf("FAIL: Expected different heads after spoof %v", err)
	}
}

// Test `latest`, `safe`, `finalized` block labels on the post-merge testnet.
func BlockLatestSafeFinalized(t *hivesim.T, env *tn.Environment,
	n clients.NodeDefinition,
) {
	ctx := context.Background()
	config := getClientConfig(n).Join(&tn.Config{
		NodeDefinitions: []clients.NodeDefinition{
			n,
			n,
		},
	})

	testnet := tn.StartTestnet(ctx, t, env, config)
	defer testnet.Stop()

	finalityCtx, cancel := testnet.Spec().EpochTimeoutContext(
		ctx,
		EPOCHS_TO_FINALITY+2,
	)
	defer cancel()

	_, err := testnet.WaitForExecutionFinality(finalityCtx)
	if err != nil {
		t.Fatalf("FAIL: Waiting for execution finality: %v", err)
	}

	if err := testnet.VerifyELBlockLabels(ctx); err != nil {
		t.Fatalf("FAIL: Verifying EL block labels: %v", err)
	}
}

/*
Generate a testnet where the transition payload contains an unknown PoW parent.
Verify that the testnet can finalize after this.
*/
func UnknownPoWParent(t *hivesim.T, env *tn.Environment,
	n clients.NodeDefinition,
) {
	ctx := context.Background()
	config := getClientConfig(n).Join(&tn.Config{
		NodeDefinitions: clients.NodeDefinitions{
			n,
			n,
			n,
		},
	})
	testnet := tn.StartTestnet(ctx, t, env, config)
	defer testnet.Stop()

	var (
		getPayloadLock     sync.Mutex
		getPayloadCount    int
		invalidPayloadHash common.Hash
		/*
			invalidPayloadNewParent common.Hash
			invalidPayloadOldParent common.Hash
		*/
		invalidPayloadNodeID int
	)

	// The EL mock will intercept an engine_getPayloadV1 call and set a random
	// parent block in the response
	getPayloadCallbackGen := func(node int) func([]byte, []byte) *spoof.Spoof {
		return func(res []byte, req []byte) *spoof.Spoof {
			getPayloadLock.Lock()
			defer getPayloadLock.Unlock()
			getPayloadCount++
			// Invalidate the transition payload
			if getPayloadCount == 1 {
				var (
					payload api.ExecutableData
					spoof   *spoof.Spoof
					err     error
				)
				err = proxy.UnmarshalFromJsonRPCResponse(res, &payload)
				if err != nil {
					panic(err)
				}
				t.Logf(
					"INFO (%v): Generating payload with unknown PoW parent: %s",
					t.TestID,
					res,
				)
				invalidPayloadHash, spoof, err = payload_spoof.GenerateInvalidPayloadSpoof(
					EngineGetPayloadV1,
					&payload,
					payload_spoof.InvalidParentHash,
					VaultSigner,
				)
				if err != nil {
					panic(err)
				}
				t.Logf(
					"INFO (%v): Invalidated payload hash: %v",
					t.TestID,
					invalidPayloadHash,
				)
				invalidPayloadNodeID = node
				return spoof
			}
			return nil
		}
	}
	// The EL mock will intercept an engine_newPayloadV1 on the node that
	// generated the invalid hash in order to validate it and broadcast it.
	newPayloadCallbackGen := func(node int) func([]byte, []byte) *spoof.Spoof {
		return func(res []byte, req []byte) *spoof.Spoof {
			var (
				payload api.ExecutableData
				spoof   *spoof.Spoof
				err     error
			)
			err = proxy.UnmarshalFromJsonRPCRequest(req, &payload)
			if err != nil {
				panic(err)
			}

			// Validate the new payload in the node that produced it
			if invalidPayloadNodeID == node &&
				payload.BlockHash == invalidPayloadHash {
				t.Logf(
					"INFO (%v): Validating new payload: %s",
					t.TestID,
					payload.BlockHash,
				)

				spoof, err = payload_spoof.PayloadStatusSpoof(
					EngineNewPayloadV1,
					&api.PayloadStatusV1{
						Status:          Valid,
						LatestValidHash: &payload.BlockHash,
						ValidationError: nil,
					},
				)
				if err != nil {
					panic(err)
				}
				return spoof
			}
			return nil
		}
	}
	// The EL mock will intercept an engine_forkchoiceUpdatedV1 on the node
	// that generated the invalid hash in order to validate and broadcast it.
	fcUCallbackGen := func(node int) func([]byte, []byte) *spoof.Spoof {
		return func(res []byte, req []byte) *spoof.Spoof {
			var (
				fcState api.ForkchoiceStateV1
				pAttr   api.PayloadAttributes
				spoof   *spoof.Spoof
				err     error
			)
			err = proxy.UnmarshalFromJsonRPCRequest(req, &fcState, &pAttr)
			if err != nil {
				panic(err)
			}

			// Validate the new payload in the node that produced it
			if invalidPayloadNodeID == node &&
				fcState.HeadBlockHash == invalidPayloadHash {
				t.Logf(
					"INFO (%v): Validating forkchoiceUpdated: %s",
					t.TestID,
					fcState.HeadBlockHash,
				)

				spoof, err = payload_spoof.ForkchoiceResponseSpoof(
					EngineForkchoiceUpdatedV1,
					api.PayloadStatusV1{
						Status:          Valid,
						LatestValidHash: &fcState.HeadBlockHash,
						ValidationError: nil,
					},
					nil,
				)
				if err != nil {
					panic(err)
				}
				return spoof
			}
			return nil
		}
	}
	for n, p := range testnet.Proxies().Running() {
		p.AddResponseCallback(EngineGetPayloadV1, getPayloadCallbackGen(n))
		p.AddResponseCallback(EngineNewPayloadV1, newPayloadCallbackGen(n))
		p.AddResponseCallback(EngineForkchoiceUpdatedV1, fcUCallbackGen(n))
	}

	// Network should recover from this
	finalityCtx, cancel := testnet.Spec().EpochTimeoutContext(
		ctx,
		EPOCHS_TO_FINALITY+1,
	)
	defer cancel()

	finalized, err := testnet.WaitForFinality(finalityCtx)
	if err != nil {
		t.Fatalf("FAIL: Waiting for finality: %v", err)
	}

	// Lowering participation requirement since we invalidated one block
	// temporarily
	if err := testnet.VerifyParticipation(
		ctx,
		tn.FirstSlotAfterCheckpoint{Checkpoint: &finalized},
		0.80,
	); err != nil {
		t.Fatalf("FAIL: Verifying participation: %v", err)
	}

	if err := testnet.VerifyExecutionPayloadIsCanonical(
		ctx,
		tn.LastSlotAtCheckpoint{Checkpoint: &finalized},
	); err != nil {
		t.Fatalf("FAIL: Verifying execution payload is canonical: %v", err)
	}

	if err := testnet.VerifyProposers(
		ctx,
		tn.LastSlotAtCheckpoint{Checkpoint: &finalized},
		true,
	); err != nil {
		t.Fatalf("FAIL: Verifying proposers: %v", err)
	}

	if err := testnet.VerifyELHeads(ctx); err != nil {
		t.Fatalf("FAIL: Verifying EL Heads: %v", err)
	}
}

/*
Generates a testnet case where one payload is invalidated in the recipient
nodes.

invalidPayloadNumber: The number of the payload to invalidate.

	1 is transition payload, 2+ is any canonical chain payload.

invalidStatusResponse: The validation error response to inject in the

	recipient nodes.
*/
func InvalidPayloadGen(
	invalidPayloadNumber int,
	invalidStatusResponse PayloadStatus,
) func(t *hivesim.T, env *tn.Environment, n clients.NodeDefinition) {
	return func(t *hivesim.T, env *tn.Environment, n clients.NodeDefinition) {
		ctx := context.Background()
		config := getClientConfig(n).Join(&tn.Config{
			NodeDefinitions: clients.NodeDefinitions{
				n,
				n,
				n,
			},
		})

		testnet := tn.StartTestnet(ctx, t, env, config)
		defer testnet.Stop()

		// All proxies will use the same callback, therefore we need to use a lock and a counter
		var (
			getPayloadLock        sync.Mutex
			getPayloadCount       int
			invalidPayloadHash    common.Hash
			invalidPayloadProxyId int
		)

		// The EL mock will intercept the first engine_getPayloadV1 and corrupt the stateRoot in the response
		getPayloadCallbackGen := func(id int) func(res []byte, req []byte) *spoof.Spoof {
			return func(res []byte, req []byte) *spoof.Spoof {
				getPayloadLock.Lock()
				defer getPayloadLock.Unlock()
				getPayloadCount++
				if getPayloadCount == invalidPayloadNumber {
					// We are not going to spoof anything here, we just need to save the transition payload hash and the id of the validator that generated it
					// to invalidate it in other clients.
					var (
						payload api.ExecutableData
					)
					err := proxy.UnmarshalFromJsonRPCResponse(res, &payload)
					if err != nil {
						panic(err)
					}
					invalidPayloadHash = payload.BlockHash
					invalidPayloadProxyId = id
					// Remove this validator from the verification
					testnet.RemoveNodeAsVerifier(id)
				}
				return nil
			}
		}

		// Here we will intercept the response from the rest of the clients that did not generate the payload to artificially invalidate them
		newPayloadCallbackGen := func(id int) func(res []byte, req []byte) *spoof.Spoof {
			return func(res []byte, req []byte) *spoof.Spoof {
				t.Logf("INFO: newPayload callback node %d", id)
				var (
					payload api.ExecutableData
					spoof   *spoof.Spoof
					err     error
				)
				err = proxy.UnmarshalFromJsonRPCRequest(req, &payload)
				if err != nil {
					panic(err)
				}
				// Only invalidate if the payload hash matches the invalid
				// payload hash and this validator is not the one that
				// generated it
				if invalidPayloadProxyId != id &&
					payload.BlockHash == invalidPayloadHash {
					var latestValidHash common.Hash
					// Latest valid hash depends on the error we are injecting
					if invalidPayloadNumber == 1 ||
						invalidStatusResponse == InvalidBlockHash {
						/* The transition payload has a PoW parent, hash must
						be 0. If the payload has an invalid hash, we cannot
						link it to any other payload, hence 0.
						*/
						latestValidHash = common.Hash{}
					} else {
						latestValidHash = payload.ParentHash
					}
					status := api.PayloadStatusV1{
						Status:          string(invalidStatusResponse),
						LatestValidHash: &latestValidHash,
						ValidationError: nil,
					}
					spoof, err = payload_spoof.PayloadStatusSpoof(
						EngineNewPayloadV1,
						&status,
					)
					if err != nil {
						panic(err)
					}
					t.Logf(
						"INFO: Invalidating payload on validator %d: %v",
						id,
						payload.BlockHash,
					)
				}
				return spoof
			}
		}
		/* 	We pass the id of the proxy to identify which one it is within the
		callback
		*/
		for i, p := range testnet.Proxies().Running() {
			p.AddResponseCallback(EngineGetPayloadV1, getPayloadCallbackGen(i))
			p.AddResponseCallback(EngineNewPayloadV1, newPayloadCallbackGen(i))
		}

		execPayloadCtx, cancel := testnet.Spec().SlotTimeoutContext(
			ctx,
			SlotsUntilMerge(ctx, testnet, config),
		)
		defer cancel()

		_, err := testnet.WaitForExecutionPayload(execPayloadCtx)
		if err != nil {
			t.Fatalf("FAIL: Waiting for execution payload: %v", err)
		}

		// Wait 5 slots after the invalidated payload
		<-testnet.Spec().SlotsTimeout(5)

		// Verify beacon block with invalid payload is not accepted
		if b, err := testnet.VerifyExecutionPayloadHashInclusion(
			ctx,
			tn.LastestSlotByHead{},
			invalidPayloadHash,
		); err != nil {
			t.Fatalf("FAIL: Error during payload verification: %v", err)
		} else if b != nil {
			t.Fatalf(
				"FAIL: Invalid Payload %v was included in slot %d (%v)",
				invalidPayloadHash,
				b.Slot(),
				b.StateRoot(),
			)
		}
	}
}

// The produced and broadcasted payload contains an invalid prevrandao value.
// The PREVRANDAO opcode is not used in any transaction and therefore not introduced in the state changes.
func IncorrectHeaderPrevRandaoPayload(
	t *hivesim.T,
	env *tn.Environment,
	n clients.NodeDefinition,
) {
	ctx := context.Background()
	config := getClientConfig(n).Join(&tn.Config{
		NodeDefinitions: clients.NodeDefinitions{
			n,
			n,
			n,
		},
	})

	testnet := tn.StartTestnet(ctx, t, env, config)
	defer testnet.Stop()

	// All proxies will use the same callback, therefore we need to use a lock and a counter
	var (
		getPayloadLock     sync.Mutex
		getPayloadCount    int
		invalidPayloadHash common.Hash
	)

	// The EL mock will intercept an engine_getPayloadV1 call and corrupt the prevRandao in the response
	c := func(res []byte, req []byte) *spoof.Spoof {
		getPayloadLock.Lock()
		defer getPayloadLock.Unlock()
		getPayloadCount++
		// Invalidate a payload after the transition payload
		if getPayloadCount == 2 {
			var (
				payload api.ExecutableData
				spoof   *spoof.Spoof
				err     error
			)
			err = proxy.UnmarshalFromJsonRPCResponse(res, &payload)
			if err != nil {
				panic(err)
			}
			t.Logf("INFO (%v): Invalidating payload: %s", t.TestID, res)
			invalidPayloadHash, spoof, err = payload_spoof.GenerateInvalidPayloadSpoof(
				EngineGetPayloadV1,
				&payload,
				payload_spoof.InvalidPrevRandao,
				VaultSigner,
			)
			if err != nil {
				panic(err)
			}
			t.Logf(
				"INFO (%v): Invalidated payload hash: %v",
				t.TestID,
				invalidPayloadHash,
			)
			return spoof
		}
		return nil
	}
	for _, p := range testnet.Proxies().Running() {
		p.AddResponseCallback(EngineGetPayloadV1, c)
	}

	execPayloadCtx, cancel := testnet.Spec().SlotTimeoutContext(
		ctx,
		SlotsUntilMerge(ctx, testnet, config),
	)
	defer cancel()
	_, err := testnet.WaitForExecutionPayload(execPayloadCtx)
	if err != nil {
		t.Fatalf("FAIL: Waiting for execution payload: %v", err)
	}

	// Wait 5 slots
	time.Sleep(
		time.Duration(testnet.Spec().SECONDS_PER_SLOT) * time.Second * 5,
	)

	// Verify beacon block with invalid payload is not accepted
	b, err := testnet.VerifyExecutionPayloadHashInclusion(
		ctx,
		tn.LastestSlotByHead{},
		invalidPayloadHash,
	)
	if err != nil {
		t.Fatalf("FAIL: Error during payload verification: %v", err)
	} else if b != nil {
		t.Fatalf("FAIL: Invalid Payload %v was included in slot %d (%v)", invalidPayloadHash, b.StateRoot())
	}
}

// The responses for `engine_newPayloadV1` and `engine_forkchoiceUpdatedV1` are delayed by `timeout` - 1s.
func Timeouts(t *hivesim.T, env *tn.Environment, n clients.NodeDefinition) {
	var (
		ForkchoiceUpdatedTimeoutSeconds = 8
		NewPayloadTimeoutSeconds        = 8
		ToleranceSeconds                = 1
		ctx                             = context.Background()
	)

	config := getClientConfig(n).Join(&tn.Config{
		NodeDefinitions: clients.NodeDefinitions{
			n,
			n,
		},
		// Use the default mainnet slot time to allow the timeout value to make sense
		SlotTime: big.NewInt(int64(12)),
	})

	testnet := tn.StartTestnet(ctx, t, env, config)
	defer testnet.Stop()

	delayEnabled := make(chan interface{})

	// The EL mock will intercept an `engine_newPayloadV1` and `engine_forkchoiceUpdatedV1` to
	// introduce an artificial delay which should almost max out the time limit of the beacon client.
	gen := func(delaySeconds int) func([]byte, []byte) *spoof.Spoof {
		return func(res []byte, req []byte) *spoof.Spoof {
			select {
			case <-time.After(time.Duration(delaySeconds) * time.Second):
			case <-delayEnabled:
			}
			return nil
		}
	}

	var (
		p0 = testnet.Proxies().Running()[0]
		p1 = testnet.Proxies().Running()[1]
	)
	p0.AddResponseCallback(
		EngineNewPayloadV1,
		gen(NewPayloadTimeoutSeconds-ToleranceSeconds),
	)
	p1.AddResponseCallback(
		EngineForkchoiceUpdatedV1,
		gen(ForkchoiceUpdatedTimeoutSeconds-ToleranceSeconds),
	)

	// Finality should be reached anyway because the time limit is not reached on the engine calls
	finalityCtx, cancel := testnet.Spec().EpochTimeoutContext(
		ctx,
		EPOCHS_TO_FINALITY+1,
	)
	defer cancel()
	_, err := testnet.WaitForFinality(finalityCtx)
	if err != nil {
		t.Fatalf("FAIL: Waiting for finality: %v", err)
	}
}

// The payload produced by the execution client contains an invalid timestamp value.
// This test covers scenario where the value of the timestamp is so high such that
// the next validators' attempts to produce payloads could fail by invalid payload
// attributes.
func InvalidTimestampPayload(
	t *hivesim.T,
	env *tn.Environment,
	n clients.NodeDefinition,
) {
	ctx := context.Background()
	config := getClientConfig(n).Join(&tn.Config{
		NodeDefinitions: clients.NodeDefinitions{
			n,
			n,
			n,
		},
	})

	testnet := tn.StartTestnet(ctx, t, env, config)
	defer testnet.Stop()

	// All proxies will use the same callback, therefore we need to use a lock and a counter
	var (
		getPayloadLock     sync.Mutex
		getPayloadCount    int
		invalidPayloadHash common.Hash
		done               = make(chan interface{})
	)

	// The EL mock will intercept an engine_getPayloadV1 call and invalidate the timestamp in the response
	c := func(res []byte, req []byte) *spoof.Spoof {
		getPayloadLock.Lock()
		defer getPayloadLock.Unlock()
		getPayloadCount++
		var (
			payload   api.ExecutableData
			payloadID api.PayloadID
			spoof     *spoof.Spoof
			err       error
		)
		err = proxy.UnmarshalFromJsonRPCResponse(res, &payload)
		if err != nil {
			panic(err)
		}
		err = proxy.UnmarshalFromJsonRPCRequest(req, &payloadID)
		if err != nil {
			panic(err)
		}
		t.Logf(
			"INFO: Got payload %v, parent=%v, from PayloadID=%x",
			payload.BlockHash,
			payload.ParentHash,
			payloadID,
		)
		// Invalidate a payload after the transition payload
		if getPayloadCount == 2 {
			t.Logf("INFO (%v): Invalidating payload: %s", t.TestID, res)
			// We are pushing the timestamp past the slot time.
			// The beacon chain shall identify this and reject the payload.
			newTimestamp := payload.Timestamp + config.SlotTime.Uint64()
			// We add some extraData to guarantee we can identify the payload we altered
			extraData := []byte("alt")
			invalidPayloadHash, spoof, err = payload_spoof.CustomizePayloadSpoof(
				EngineGetPayloadV1,
				&payload,
				&payload_spoof.CustomPayloadData{
					Timestamp: &newTimestamp,
					ExtraData: &extraData,
				},
			)
			if err != nil {
				panic(err)
			}
			t.Logf(
				"INFO (%v): Invalidated payload hash: %v",
				t.TestID,
				invalidPayloadHash,
			)
			select {
			case done <- nil:
			default:
			}
			return spoof
		}
		return nil
	}
	// No ForkchoiceUpdated with payload attributes should fail, which could happen if the payload
	// with invalid timestamp is accepted (next payload would fail).
	var (
		fcuLock       sync.Mutex
		fcUAttrCount  int
		fcudone       = make(chan error)
		fcUCountLimit = 8
	)

	for _, p := range testnet.Proxies().Running() {
		p.AddResponseCallback(EngineGetPayloadV1, c)
		p.AddResponseCallback(
			EngineForkchoiceUpdatedV1,
			payload_spoof.CheckErrorOnForkchoiceUpdatedPayloadAttributes(
				&fcuLock,
				fcUCountLimit,
				&fcUAttrCount,
				fcudone,
			),
		)
	}

	// Wait until the invalid payload is produced
	<-done

	// Wait until we verified all subsequent forkchoiceUpdated calls
	if err := <-fcudone; err != nil {
		t.Fatalf("FAIL: ForkchoiceUpdated call failed: %v", err)
	}

	// Verify beacon block with invalid payload is not accepted
	b, err := testnet.VerifyExecutionPayloadHashInclusion(
		ctx,
		tn.LastestSlotByHead{},
		invalidPayloadHash,
	)
	if err != nil {
		t.Fatalf("FAIL: Error during payload verification: %v", err)
	} else if b != nil {
		t.Fatalf(
			"FAIL: Invalid Payload %v was included in slot %d (%v)",
			invalidPayloadHash,
			b.Slot(),
			b.StateRoot(),
		)
	}
}

func IncorrectTTDConfigEL(
	t *hivesim.T,
	env *tn.Environment,
	n clients.NodeDefinition,
) {
	ctx := context.Background()
	config := getClientConfig(n)
	elTTD := config.TerminalTotalDifficulty.Int64() - 2
	config = config.Join(&tn.Config{
		NodeDefinitions: clients.NodeDefinitions{
			clients.NodeDefinition{
				// Add a node with an incorrect TTD to reject the invalid payload
				ExecutionClient:    n.ExecutionClient,
				ConsensusClient:    n.ConsensusClient,
				ExecutionClientTTD: big.NewInt(elTTD),
			},
		},
	})

	testnet := tn.StartTestnet(ctx, t, env, config)
	defer testnet.Stop()

	var (
		builder = testnet.BeaconClients().Running()[0]
		ec      = testnet.ExecutionClients().Running()[0]
	)
	ttdCtx, cancel := context.WithTimeout(
		ctx,
		time.Duration(el.CLIQUE_PERIOD_DEFAULT*uint64(elTTD)*2)*time.Second,
	)
	defer cancel()
	if err := ec.WaitForTerminalTotalDifficulty(ttdCtx); err != nil {
		t.Fatalf("FAIL: %v", err)
	}
	// Wait a couple of slots
	time.Sleep(time.Duration(config.SlotTime.Uint64()*5) * time.Second)

	// Try to get the latest execution payload, must be nil
	if b, err := builder.GetLatestExecutionBeaconBlock(ctx); err != nil {
		t.Fatalf(
			"FAIL: Unable to query for the latest execution payload: %v",
			err,
		)
	} else if b != nil {
		t.Fatalf(
			"FAIL: Execution payload was included in the beacon chain with a misconfigured TTD on the EL: %v",
			b.StateRoot(),
		)
	}
}

// The produced and broadcasted transition payload has parent with an invalid total difficulty.
func IncorrectTerminalBlockGen(
	ttdDelta int64,
) func(t *hivesim.T, env *tn.Environment, n clients.NodeDefinition) {
	return func(t *hivesim.T, env *tn.Environment, n clients.NodeDefinition) {
		ctx := context.Background()
		config := getClientConfig(n)
		BadTTD := big.NewInt(config.TerminalTotalDifficulty.Int64() + ttdDelta)
		config = config.Join(&tn.Config{
			NodeDefinitions: clients.NodeDefinitions{
				clients.NodeDefinition{
					ExecutionClient:      n.ExecutionClient,
					ConsensusClient:      n.ConsensusClient,
					ValidatorShares:      1,
					TestVerificationNode: true,
				},
				clients.NodeDefinition{
					ExecutionClient:      n.ExecutionClient,
					ConsensusClient:      n.ConsensusClient,
					ValidatorShares:      1,
					TestVerificationNode: true,
				},
				clients.NodeDefinition{
					ExecutionClient:      n.ExecutionClient,
					ConsensusClient:      n.ConsensusClient,
					ValidatorShares:      1,
					TestVerificationNode: true,
				},
				clients.NodeDefinition{
					// Add a node with an incorrect TTD to reject the invalid payload
					ExecutionClient:    n.ExecutionClient,
					ConsensusClient:    n.ConsensusClient,
					ValidatorShares:    0,
					ExecutionClientTTD: BadTTD,
					BeaconNodeTTD:      BadTTD,
				},
			},
		})

		testnet := tn.StartTestnet(ctx, t, env, config)
		defer testnet.Stop()

		badTTDImporter := testnet.BeaconClients().Running()[3]

		// Wait for all execution clients with the correct TTD reach the merge
		execPayloadCtx, cancel := testnet.Spec().SlotTimeoutContext(
			ctx,
			SlotsUntilMerge(ctx, testnet, config),
		)
		defer cancel()
		transitionPayloadHash, err := testnet.WaitForExecutionPayload(
			execPayloadCtx,
		)
		if err != nil {
			t.Fatalf("FAIL: Waiting for execution payload: %v", err)
		}

		ec := testnet.ExecutionClients().Running()[0]

		transitionHeader, err := ec.HeaderByHash(ctx, transitionPayloadHash)
		if err != nil {
			t.Fatalf(
				"FAIL: Unable to get transition payload header from execution client: %v",
				err,
			)
		}

		terminalHeader, err := ec.HeaderByHash(
			ctx,
			transitionHeader.ParentHash,
		)
		if err != nil {
			t.Fatalf(
				"FAIL: Unable to get transition payload header from execution client: %v",
				err,
			)
		}
		terminalBlockTD, err := ec.TotalDifficultyByHash(
			ctx,
			terminalHeader.Hash(),
		)
		if err != nil {
			t.Fatalf(
				"FAIL: Unable to get terminal block total difficulty from execution client: %v",
				err,
			)
		}

		terminalParentHeader, err := ec.HeaderByHash(
			ctx,
			terminalHeader.ParentHash,
		)
		if err != nil {
			t.Fatalf(
				"FAIL: Unable to get transition payload header from execution client: %v",
				err,
			)
		}
		terminalBlockParentTD, err := ec.TotalDifficultyByHash(
			ctx,
			terminalParentHeader.Hash(),
		)
		if err != nil {
			t.Fatalf(
				"FAIL: Unable to get terminal block total difficulty from execution client: %v",
				err,
			)
		}

		t.Logf(
			"INFO: CorrectTTD=%d, BadTTD=%d, TerminalBlockTotalDifficulty=%d, TerminalBlockParentTotalDifficulty=%d",
			config.TerminalTotalDifficulty,
			BadTTD,
			terminalBlockTD,
			terminalBlockParentTD,
		)

		// Wait a couple of slots
		time.Sleep(time.Duration(5*config.SlotTime.Uint64()) * time.Second)

		// Transition payload should not be part of the beacon node with bad TTD
		b, err := testnet.VerifyExecutionPayloadHashInclusionNode(
			ctx,
			tn.LastestSlotByHead{},
			badTTDImporter,
			transitionPayloadHash,
		)
		if err != nil {
			t.Fatalf("FAIL: Error during payload verification: %v", err)
		} else if b != nil {
			t.Fatalf("FAIL: Node with bad TTD included beacon block with correct TTD: %v", b)
		}
	}
}

func SyncingWithInvalidChain(
	t *hivesim.T,
	env *tn.Environment,
	n clients.NodeDefinition,
) {
	ctx := context.Background()
	config := getClientConfig(n).Join(&tn.Config{
		NodeDefinitions: clients.NodeDefinitions{
			// Builder 1
			clients.NodeDefinition{
				ExecutionClient:      n.ExecutionClient,
				ConsensusClient:      n.ConsensusClient,
				ValidatorShares:      1,
				TestVerificationNode: false,
			},
			// Builder 2
			clients.NodeDefinition{
				ExecutionClient:      n.ExecutionClient,
				ConsensusClient:      n.ConsensusClient,
				ValidatorShares:      1,
				TestVerificationNode: false,
			},
			// Importer
			clients.NodeDefinition{
				ExecutionClient:      n.ExecutionClient,
				ConsensusClient:      n.ConsensusClient,
				ValidatorShares:      0,
				TestVerificationNode: true,
			},
		},
	})

	testnet := tn.StartTestnet(ctx, t, env, config)
	defer testnet.Stop()

	var (
		transitionPayloadHeight uint64
		lastValidHash           common.Hash
		invalidPayloadHashes    = make([]common.Hash, 0)
		payloadMap              = make(map[common.Hash]api.ExecutableData)
		done                    = make(chan interface{})
	)

	// Payloads will be intercepted here and spoofed to simulate sync.
	// Then after 4 payloads (p1, p2, p3, p4), the last one will be marked `INVALID` with
	// `latestValidHash==p1.Hash`
	newPayloadCallback := func(res []byte, req []byte) *spoof.Spoof {
		var (
			payload api.ExecutableData
			spoof   *spoof.Spoof
			err     error
		)
		err = proxy.UnmarshalFromJsonRPCRequest(req, &payload)
		if err != nil {
			panic(err)
		}
		payloadMap[payload.BlockHash] = payload
		if lastValidHash == (common.Hash{}) {
			// This is the transition payload (P1) because it's the first time this callback is called
			transitionPayloadHeight = payload.Number
			lastValidHash = payload.BlockHash
			t.Logf(
				"INFO: Last VALID hash %d: %v",
				payload.Number-transitionPayloadHeight,
				payload.BlockHash,
			)
		} else {
			if payload.Number >= transitionPayloadHeight+3 {
				// This is P4
				invalidPayloadHashes = append(invalidPayloadHashes, payload.BlockHash)
				status := api.PayloadStatusV1{
					Status:          Invalid,
					LatestValidHash: &lastValidHash,
					ValidationError: nil,
				}
				spoof, err = payload_spoof.PayloadStatusSpoof(EngineNewPayloadV1, &status)
				if err != nil {
					panic(err)
				}
				t.Logf("INFO: Returning INVALID payload %d: %v", payload.Number-transitionPayloadHeight, payload.BlockHash)
				select {
				case done <- nil:
				default:
				}
			} else if payload.Number > transitionPayloadHeight {
				// For all other payloads, including P2/P3, return SYNCING
				invalidPayloadHashes = append(invalidPayloadHashes, payload.BlockHash)
				status := api.PayloadStatusV1{
					Status:          Syncing,
					LatestValidHash: nil,
					ValidationError: nil,
				}
				spoof, err = payload_spoof.PayloadStatusSpoof(EngineNewPayloadV1, &status)
				if err != nil {
					panic(err)
				}
				t.Logf("INFO: Returning SYNCING payload %d: %v", payload.Number-transitionPayloadHeight, payload.BlockHash)
			}
		}
		return spoof
	}

	forkchoiceUpdatedCallback := func(res []byte, req []byte) *spoof.Spoof {
		var (
			fcState api.ForkchoiceStateV1
			pAttr   api.PayloadAttributes
			spoof   *spoof.Spoof
			err     error
		)
		err = proxy.UnmarshalFromJsonRPCRequest(req, &fcState, &pAttr)
		if err != nil {
			panic(err)
		}
		if lastValidHash == (common.Hash{}) {
			panic(
				fmt.Errorf(
					"NewPayload was not called before ForkchoiceUpdated",
				),
			)
		}
		payload, ok := payloadMap[fcState.HeadBlockHash]
		if !ok {
			panic(fmt.Errorf("payload not found: %v", fcState.HeadBlockHash))
		}

		if payload.Number != transitionPayloadHeight {
			if payload.Number == transitionPayloadHeight+3 {
				// This is P4, but we probably won't receive this since NewPayload(P4) already returned INVALID
				status := api.PayloadStatusV1{
					Status:          Invalid,
					LatestValidHash: &lastValidHash,
					ValidationError: nil,
				}
				spoof, err = payload_spoof.ForkchoiceResponseSpoof(
					EngineForkchoiceUpdatedV1,
					status,
					nil,
				)
				if err != nil {
					panic(err)
				}
				t.Logf(
					"INFO: Returning INVALID payload %d (ForkchoiceUpdated): %v",
					payload.Number-transitionPayloadHeight,
					payload.BlockHash,
				)
			} else {
				// For all other payloads, including P2/P3, return SYNCING
				status := api.PayloadStatusV1{
					Status:          Syncing,
					LatestValidHash: nil,
					ValidationError: nil,
				}
				spoof, err = payload_spoof.ForkchoiceResponseSpoof(EngineForkchoiceUpdatedV1, status, nil)
				if err != nil {
					panic(err)
				}
				t.Logf("INFO: Returning SYNCING payload %d (ForkchoiceUpdated): %v", payload.Number-transitionPayloadHeight, payload.BlockHash)
			}
		}
		return spoof
	}

	importerProxy := testnet.Proxies().Running()[2]

	// Add the callback to the last proxy which will not produce blocks
	importerProxy.AddResponseCallback(EngineNewPayloadV1, newPayloadCallback)
	importerProxy.AddResponseCallback(
		EngineForkchoiceUpdatedV1,
		forkchoiceUpdatedCallback,
	)

	<-done

	// Wait a few slots for re-org to happen
	time.Sleep(
		time.Duration(testnet.Spec().SECONDS_PER_SLOT) * time.Second * 5,
	)

	// Verify the head of the chain, it should be a block with the latestValidHash included
	for _, bn := range testnet.VerificationNodes().BeaconClients().Running() {
		versionedBlock, err := bn.BlockV2(ctx, eth2api.BlockHead)
		if err != nil {
			t.Fatalf("FAIL: Unable to poll beacon chain head: %v", err)
		}
		if versionedBlock.Version != "bellatrix" {
			// Block can't contain an executable payload
			t.Fatalf("FAIL: Head of the chain is not a bellatrix fork block")
		}
		if payload, err := versionedBlock.ExecutionPayload(); err == nil {
			t.Fatalf(
				"FAIL: error getting execution payload: %v",
				err,
			)
		} else if !bytes.Equal(payload.BlockHash[:], lastValidHash[:]) {
			t.Fatalf(
				"FAIL: Head does not contain the expected execution payload: %v != %v",
				payload.BlockHash.String(),
				lastValidHash.Hex(),
			)
		}
	}

	// Verify payloads
	if b, err := testnet.VerifyExecutionPayloadHashInclusion(ctx, tn.LastestSlotByHead{}, lastValidHash); b == nil ||
		err != nil {
		if err != nil {
			t.Fatalf(
				"FAIL: Valid Payload %v could not be found: %v",
				lastValidHash,
				err,
			)
		}
		t.Fatalf("FAIL: Valid Payload %v could not be found", lastValidHash)
	}
	for i, p := range invalidPayloadHashes {
		b, err := testnet.VerifyExecutionPayloadHashInclusion(
			ctx,
			tn.LastestSlotByHead{},
			p,
		)
		if err != nil {
			t.Fatalf("FAIL: Error during payload verification: %v", err)
		} else if b != nil {
			t.Fatalf(
				"FAIL: Invalid Payload (%d) %v was included in slot %d (%v)",
				i+1, p, b.Slot(), b.StateRoot(),
			)
		}
	}
}

func BaseFeeEncodingCheck(
	t *hivesim.T,
	env *tn.Environment,
	n clients.NodeDefinition,
) {
	ctx := context.Background()
	config := getClientConfig(n).Join(&tn.Config{
		InitialBaseFeePerGas: big.NewInt(9223372036854775807), // 2**63 - 1
		NodeDefinitions: []clients.NodeDefinition{
			n,
			n,
		},
	})

	testnet := tn.StartTestnet(ctx, t, env, config)
	defer testnet.Stop()

	execPayloadCtx, cancel := testnet.Spec().SlotTimeoutContext(
		ctx,
		SlotsUntilMerge(ctx, testnet, config),
	)
	defer cancel()
	transitionPayloadHash, err := testnet.WaitForExecutionPayload(
		execPayloadCtx,
	)
	if err != nil {
		t.Fatalf("FAIL: Waiting for execution payload: %v", err)
	}

	// Check the base fee value in the transition payload.
	// Must be at least 256 to guarantee that the endianess encoding is correct.
	ec := testnet.ExecutionClients().Running()[0]
	h, err := ec.HeaderByHash(ctx, transitionPayloadHash)
	if err != nil {
		t.Fatalf(
			"FAIL: Unable to get transition payload header from execution client: %v",
			err,
		)
	}
	if h.Difficulty.Cmp(common.Big0) != 0 {
		t.Fatalf(
			"FAIL: Transition header obtained is not PoS header: difficulty=%x",
			h.Difficulty,
		)
	}
	if h.BaseFee.Cmp(common.Big256) < 0 {
		t.Fatalf("FAIL: Basefee insufficient for test: %x", h.BaseFee)
	}

	t.Logf(
		"INFO: Transition Payload created with sufficient baseFee: %x",
		h.BaseFee,
	)
}

func EqualTimestampTerminalTransitionBlock(
	t *hivesim.T,
	env *tn.Environment,
	n clients.NodeDefinition,
) {
	ctx := context.Background()
	config := getClientConfig(n)
	config = config.Join(&tn.Config{
		// We are increasing the clique period, therefore we can reduce the TTD
		TerminalTotalDifficulty: big.NewInt(
			config.TerminalTotalDifficulty.Int64() / 3,
		),
		NodeDefinitions: []clients.NodeDefinition{
			n,
			n,
		},

		// The clique period needs to be equal to the slot time to try to get the CL client to attempt to produce
		// a payload with the same timestamp as the terminal block
		Eth1Consensus: &el.ExecutionCliqueConsensus{
			CliquePeriod: config.SlotTime.Uint64(),
		},
	})

	testnet := tn.StartTestnet(ctx, t, env, config)
	defer testnet.Stop()

	// No ForkchoiceUpdated with payload attributes should fail, which could happen if CL tries to create
	// the payload with `timestamp==terminalBlock.timestamp`.
	var (
		fcuLock       sync.Mutex
		fcUAttrCount  int
		fcudone       = make(chan error)
		fcUCountLimit = 5
	)

	for _, p := range testnet.Proxies().Running() {
		p.AddResponseCallback(
			EngineForkchoiceUpdatedV1,
			payload_spoof.CheckErrorOnForkchoiceUpdatedPayloadAttributes(
				&fcuLock,
				fcUCountLimit,
				&fcUAttrCount,
				fcudone,
			),
		)
	}

	// Wait until we verified all subsequent forkchoiceUpdated calls
	if err := <-fcudone; err != nil {
		t.Fatalf("FAIL: ForkchoiceUpdated call failed: %v", err)
	}
}

func TTDBeforeBellatrix(
	t *hivesim.T,
	env *tn.Environment,
	n clients.NodeDefinition,
) {
	ctx := context.Background()
	config := getClientConfig(n)
	config = config.Join(&tn.Config{
		AltairForkEpoch:         common.Big1,
		BellatrixForkEpoch:      common.Big2,
		TerminalTotalDifficulty: big.NewInt(150),
		NodeDefinitions: []clients.NodeDefinition{
			n,
			n,
		},
	})

	testnet := tn.StartTestnet(ctx, t, env, config)
	defer testnet.Stop()

	execPayloadCtx, cancel := testnet.Spec().SlotTimeoutContext(
		ctx,
		SlotsUntilMerge(ctx, testnet, config),
	)
	defer cancel()
	_, err := testnet.WaitForExecutionPayload(execPayloadCtx)
	if err != nil {
		for i, ec := range testnet.ExecutionClients().Running() {
			if b, err := ec.BlockByNumber(ctx, nil); err == nil {
				t.Logf(
					"INFO: Last block on execution client %d: number=%d, hash=%s",
					i,
					b.NumberU64(),
					b.Hash(),
				)
			}
		}
		t.Fatalf("FAIL: Waiting for execution payload: %v", err)
	}
	if err := testnet.VerifyELHeads(ctx); err != nil {
		t.Fatalf("FAIL: Verifying EL Heads: %v", err)
	}
}

func InvalidQuantityPayloadFields(
	t *hivesim.T,
	env *tn.Environment,
	n clients.NodeDefinition,
) {
	ctx := context.Background()
	config := getClientConfig(n).Join(&tn.Config{
		NodeDefinitions: clients.NodeDefinitions{
			n,
			n,
			n,
		},
	})

	testnet := tn.StartTestnet(ctx, t, env, config)
	defer testnet.Stop()

	// First we are going to wait for the transition to happen
	execPayloadCtx, cancel := testnet.Spec().SlotTimeoutContext(
		ctx,
		SlotsUntilMerge(ctx, testnet, config),
	)
	defer cancel()
	_, err := testnet.WaitForExecutionPayload(execPayloadCtx)
	if err != nil {
		t.Fatalf("FAIL: Waiting for execution payload: %v", err)
	}

	// https://github.com/ethereum/execution-apis/blob/main/src/engine/specification.md
	// Values of a field of QUANTITY type MUST be encoded as a hexadecimal string with a 0x prefix
	// and the leading 0s stripped (except for the case of encoding the value 0) matching the
	// regular expression ^0x(?:0|(?:[a-fA-F1-9][a-fA-F0-9]*))$.
	// Note: Byte order of encoded value having QUANTITY type is big-endian.
	type QuantityType struct {
		Name    string
		BitSize uint64
	}
	type InvalidationType int
	const (
		Overflow InvalidationType = iota
		LeadingZero
		Empty
		NoPrefix
		InvalidationTypeCount
	)

	invalidateQuantityType := func(id int, method string, response []byte, q QuantityType, invType InvalidationType) *spoof.Spoof {
		responseFields := make(map[string]json.RawMessage)
		if err := proxy.UnmarshalFromJsonRPCResponse(response, &responseFields); err != nil {
			panic(
				fmt.Errorf("unable to unmarshal: %v. json: %s", err, response),
			)
		}
		var fieldOriginalValue string
		if err := json.Unmarshal(responseFields[q.Name], &fieldOriginalValue); err != nil {
			panic(
				fmt.Errorf(
					"unable to unmarshal: %v. json: %s",
					err,
					responseFields[q.Name],
				),
			)
		}
		fields := make(map[string]interface{})
		switch invType {
		case Overflow:
			overflowingStringSize := int((q.BitSize / 4) + 1)
			fields[q.Name] = "0x" + strings.Repeat(
				"0",
				overflowingStringSize-(len(fieldOriginalValue)-2),
			) + fieldOriginalValue[2:]
		case LeadingZero:
			if !strings.HasPrefix(fieldOriginalValue, "0x") {
				panic(
					fmt.Errorf(
						"invalid original value: %v",
						fieldOriginalValue,
					),
				)
			}
			fields[q.Name] = "0x0" + fieldOriginalValue[2:]
		case Empty:
			fields[q.Name] = "0x"
		case NoPrefix:
			// Remove the "0x" prefix
			fields[q.Name] = fieldOriginalValue[2:]
		default:
			panic("Invalid QUANTITY invalidation type")
		}

		t.Logf(
			"INFO: Spoofing (node %d) %s, %s -> %s",
			id,
			q.Name,
			fieldOriginalValue,
			fields[q.Name],
		)
		// Return the new payload status spoof
		return &spoof.Spoof{
			Method: method,
			Fields: fields,
		}
	}

	allQuantityFields := []QuantityType{
		{
			Name:    "blockNumber",
			BitSize: 64,
		},
		{
			Name:    "gasLimit",
			BitSize: 64,
		},
		{
			Name:    "gasUsed",
			BitSize: 64,
		},
		{
			Name:    "timestamp",
			BitSize: 64,
		},
		{
			Name:    "baseFeePerGas",
			BitSize: 256,
		},
	}

	// All proxies will use the same callback, therefore we need to use a lock and a counter
	var (
		getPayloadLock       sync.Mutex
		getPayloadCount      int
		invalidPayloadHashes = make([]common.Hash, 0)
		done                 = make(chan interface{})
	)

	// The EL mock will intercept the first engine_getPayloadV1 and corrupt the stateRoot in the response
	getPayloadCallbackGen := func(id int) func([]byte, []byte) *spoof.Spoof {
		return func(res, req []byte) *spoof.Spoof {
			getPayloadLock.Lock()
			defer getPayloadLock.Unlock()
			defer func() {
				getPayloadCount++
			}()
			var payload api.ExecutableData
			err := proxy.UnmarshalFromJsonRPCResponse(res, &payload)
			if err != nil {
				panic(err)
			}

			field := getPayloadCount / int(InvalidationTypeCount)
			invType := InvalidationType(
				getPayloadCount % int(InvalidationTypeCount),
			)

			if field >= len(allQuantityFields) {
				select {
				case done <- nil:
				default:
				}
				return nil
			}
			// Customize to get a different hash in order to properly check that the payload is actually not included
			customExtraData := []byte(
				fmt.Sprintf(
					"invalid %s %d",
					allQuantityFields[field].Name,
					invType,
				),
			)
			newHash, spoof, _ := payload_spoof.CustomizePayloadSpoof(
				EngineGetPayloadV1,
				&payload,
				&payload_spoof.CustomPayloadData{
					ExtraData: &customExtraData,
				},
			)
			invalidPayloadHashes = append(invalidPayloadHashes, newHash)
			return proxy.Combine(
				spoof,
				invalidateQuantityType(
					id,
					EngineGetPayloadV1,
					res,
					allQuantityFields[field],
					invType,
				),
			)
		}
	}

	// We pass the id of the proxy to identify which one it is within the callback
	for i, p := range testnet.Proxies().Running() {
		p.AddResponseCallback(EngineGetPayloadV1, getPayloadCallbackGen(i))
	}

	// Wait until we are done
	var testFailed bool
	timeoutCtx, cancel := testnet.Spec().SlotTimeoutContext(
		ctx,
		beacon.Slot(len(allQuantityFields)*int(InvalidationTypeCount)*2),
	)
	defer cancel()
	select {
	case <-done:
	case <-timeoutCtx.Done():
		t.Logf(
			"FAIL: Timeout while waiting for CL requesting all payloads, test is invalid.",
		)
		testFailed = true
	}

	// Check that none of the invalidated payloads made it into the beacon chain
	for i, p := range invalidPayloadHashes {
		b, err := testnet.VerifyExecutionPayloadHashInclusion(
			ctx,
			tn.LastestSlotByTime{},
			p,
		)
		if err != nil {
			t.Fatalf("FAIL: Error during payload verification: %v", err)
		} else if b != nil {

			if execPayload, err := b.ExecutionPayload(); err != nil {
				t.Logf(
					"FAIL: Beacon block does not contain a payload, slot %d (%v)",
					b.Slot(), b.StateRoot(),
				)
			} else {
				t.Logf(
					"FAIL: Invalid Payload #%d, %v (%x), was included in slot %d (%v)",
					i+1, p, execPayload.ExtraData, b.Slot(), b.StateRoot(),
				)
			}

			// Mark test as failure, but continue checking all variations
			testFailed = true
		}
	}
	if testFailed {
		t.Fail()
	} else {
		t.Logf("INFO: Success, none of the hashes were included")
	}
}

func SyncingWithChainHavingValidTransitionBlock(
	t *hivesim.T,
	env *tn.Environment,
	n clients.NodeDefinition,
) {
	var (
		safeSlotsToImportOptimistically = big.NewInt(8)
		safeSlotsImportThreshold        = uint64(4)
		ctx                             = context.Background()
	)
	if clientSafeSlots, ok := SAFE_SLOTS_TO_IMPORT_OPTIMISTICALLY_CLIENT_OVERRIDE[n.ConsensusClient]; ok {
		safeSlotsToImportOptimistically = clientSafeSlots
	}

	config := getClientConfig(n).Join(&tn.Config{
		NodeDefinitions: clients.NodeDefinitions{
			// Builder
			clients.NodeDefinition{
				ExecutionClient: n.ExecutionClient,
				ConsensusClient: n.ConsensusClient,
				ValidatorShares: 1,
				ChainGenerator: &pow.ChainGenerator{
					BlockCount: 1,
					Config:     pow.Defaults,
				},
			},
			// Importer
			clients.NodeDefinition{
				ExecutionClient: n.ExecutionClient,
				ConsensusClient: n.ConsensusClient,
				ValidatorShares: 0,
				ChainGenerator: &pow.ChainGenerator{
					BlockCount: 1,
					Config:     pow.Defaults,
				},
			},
		},
		Eth1Consensus:                   el.ExecutionPreChain{},
		SafeSlotsToImportOptimistically: safeSlotsToImportOptimistically,
	})

	testnet := tn.StartTestnet(ctx, t, env, config)
	defer testnet.Stop()

	var (
		builder       = testnet.BeaconClients()[0]
		importer      = testnet.BeaconClients()[1]
		importerProxy = testnet.Proxies().Running()[1]
	)

	importerResponseMocker := payload_spoof.NewEngineResponseMocker(
		&api.PayloadStatusV1{
			// By default we respond SYNCING to any payload
			Status:          Syncing,
			LatestValidHash: nil,
		},
	)
	importerResponseMocker.AddCallbacksToProxy(importerProxy)

	execPayloadCtx, cancel := testnet.Spec().SlotTimeoutContext(
		ctx,
		SlotsUntilMerge(ctx, testnet, config),
	)
	defer cancel()
	// Wait until the builder creates the first block with an execution payload
	_, err := testnet.WaitForExecutionPayload(execPayloadCtx)
	if err != nil {
		t.Fatalf("FAIL: Waiting for execution payload on builder: %v", err)
	}

	builderExecutionBlock, err := builder.GetFirstExecutionBeaconBlock(ctx)
	if err != nil || builderExecutionBlock == nil {
		t.Fatalf("FAIL: Could not find first execution block")
	}
	t.Logf(
		"Builder Execution block found on slot %d",
		builderExecutionBlock.Slot(),
	)

	// We wait until the importer reaches optimistic sync
	optimisticStateCtx, cancel := testnet.Spec().SlotTimeoutContext(
		ctx,
		beacon.Slot(
			safeSlotsToImportOptimistically.Uint64()+
				safeSlotsImportThreshold),
	)
	defer cancel()
	_, err = importer.WaitForOptimisticState(
		optimisticStateCtx,
		eth2api.BlockIdSlot(builderExecutionBlock.Slot()),
		true,
	)
	if err != nil {
		t.Fatalf(
			"FAIL: Timeout waiting for beacon node to become optimistic: %v",
			err,
		)
	}

	// Mocked responses are disabled, so the EL can finally validate payloads
	importerResponseMocker.Mocking = false

	// Wait until newPayload or forkchoiceUpdated are called at least once
	ctxTimeout, cancel := context.WithTimeout(ctx, time.Second*20)
	defer cancel()
	select {
	case <-importerResponseMocker.NewPayloadCalled:
	case <-importerResponseMocker.ForkchoiceUpdatedCalled:
	case <-ctxTimeout.Done():
		t.Fatalf(
			"FAIL: Timeout waiting for beacon node to send engine directive: %v",
			err,
		)
	}

	// Wait a couple of slots here to make sure syncing does not produce a false positive
	time.Sleep(time.Duration(config.SlotTime.Uint64()*10) * time.Second)

	// Wait for the importer to get an execution payload
	execPayloadCtx, cancel = testnet.Spec().SlotTimeoutContext(
		ctx,
		beacon.Slot(safeSlotsToImportOptimistically.Uint64()+safeSlotsImportThreshold),
	)
	defer cancel()
	_, err = importer.WaitForExecutionPayload(execPayloadCtx)
	if err != nil {
		t.Fatalf("FAIL: Waiting for execution payload on importer: %v", err)
	}

	// Compare heads, the importer must have the same head as the builder,
	// and `execution_optimistic==false`.
	var (
		importerHeadInfo *eth2api.BeaconBlockHeaderAndInfo
		builderHeadInfo  *eth2api.BeaconBlockHeaderAndInfo
	)
	importerHeadInfo, err = importer.BlockHeader(ctx, eth2api.BlockHead)
	if err != nil {
		t.Fatalf("FAIL: Failed to poll head importer head: %v", err)
	}

	builderHeadInfo, err = builder.BlockHeader(ctx, eth2api.BlockHead)
	if err != nil {
		t.Fatalf("FAIL: Failed to poll head builder head: %v", err)
	}

	if importerHeadInfo.Root != builderHeadInfo.Root {
		t.Fatalf(
			"FAIL: importer and builder heads are not equal: %v != %v",
			importerHeadInfo.Root,
			builderHeadInfo.Root,
		)
	}

	var headOptStatus clients.BlockV2OptimisticResponse
	ctxTimeout, cancel = context.WithTimeout(ctx, time.Second*5)
	defer cancel()
	if exists, err := eth2api.SimpleRequest(ctxTimeout, importer.API, eth2api.FmtGET("/eth/v2/beacon/blocks/%s", eth2api.BlockHead.BlockId()), &headOptStatus); err != nil {
		t.Fatalf("FAIL: Failed to poll head importer head: %v", err)
	} else if !exists {
		t.Fatalf("FAIL: Failed to poll head importer head: !exists")
	}
	if headOptStatus.ExecutionOptimistic {
		t.Fatalf(
			"FAIL: importer still optimistic: execution_optimistic==%t",
			headOptStatus.ExecutionOptimistic,
		)
	}
}

func SyncingWithChainHavingInvalidTransitionBlock(
	t *hivesim.T,
	env *tn.Environment,
	n clients.NodeDefinition,
) {
	var (
		safeSlotsToImportOptimistically = big.NewInt(8)
		safeSlotsImportThreshold        = uint64(4)
		ctx                             = context.Background()
	)
	if clientSafeSlots, ok := SAFE_SLOTS_TO_IMPORT_OPTIMISTICALLY_CLIENT_OVERRIDE[n.ConsensusClient]; ok {
		safeSlotsToImportOptimistically = clientSafeSlots
	}

	config := getClientConfig(n).Join(&tn.Config{
		NodeDefinitions: clients.NodeDefinitions{
			// Builder
			clients.NodeDefinition{
				ExecutionClient: n.ExecutionClient,
				ConsensusClient: n.ConsensusClient,
				ValidatorShares: 1,
				ChainGenerator: &pow.ChainGenerator{
					BlockCount: 1,
					Config:     pow.Defaults,
				},
			},
			// Importer
			clients.NodeDefinition{
				ExecutionClient: n.ExecutionClient,
				ConsensusClient: n.ConsensusClient,
				ValidatorShares: 0,
				ChainGenerator: &pow.ChainGenerator{
					BlockCount: 1,
					Config:     pow.Defaults,
				},
			},
		},
		AltairForkEpoch:                 common.Big1,
		BellatrixForkEpoch:              common.Big2,
		Eth1Consensus:                   el.ExecutionPreChain{},
		SafeSlotsToImportOptimistically: safeSlotsToImportOptimistically,
	})

	testnet := tn.StartTestnet(ctx, t, env, config)
	defer testnet.Stop()

	var (
		builder       = testnet.BeaconClients()[0]
		importer      = testnet.BeaconClients()[1]
		importerProxy = testnet.Proxies().Running()[1]
	)

	importerResponseMocker := payload_spoof.NewEngineResponseMocker(
		&api.PayloadStatusV1{
			// By default we respond SYNCING to any payload
			Status:          Syncing,
			LatestValidHash: nil,
		},
	)
	importerResponseMocker.AddCallbacksToProxy(importerProxy)

	// Wait until the builder creates the first block with an execution payload
	execPayloadCtx, cancel := testnet.Spec().SlotTimeoutContext(
		ctx,
		SlotsUntilMerge(ctx, testnet, config),
	)
	defer cancel()
	_, err := testnet.WaitForExecutionPayload(execPayloadCtx)
	if err != nil {
		t.Fatalf(
			"FAIL: Timeout waiting for execution payload on builder: %v",
			err,
		)
	}

	// Fetch the first execution block which will be used for verification
	builderExecutionBlock, err := builder.GetFirstExecutionBeaconBlock(ctx)
	if err != nil || builderExecutionBlock == nil {
		t.Fatalf("FAIL: Could not find first execution block")
	}

	t.Logf(
		"INFO: First execution block: %d, %v",
		builderExecutionBlock.Slot(), builderExecutionBlock.StateRoot(),
	)

	// We wait until the importer reaches optimistic sync
	optimisticStateCtx, cancel := testnet.Spec().SlotTimeoutContext(
		ctx,
		beacon.Slot(
			safeSlotsToImportOptimistically.Uint64()+
				safeSlotsImportThreshold),
	)
	defer cancel()
	_, err = importer.WaitForOptimisticState(
		optimisticStateCtx,
		eth2api.BlockIdSlot(builderExecutionBlock.Slot()),
		true,
	)
	if err != nil {
		t.Fatalf(
			"FAIL: Timeout waiting for beacon node to become optimistic: %v",
			err,
		)
	}

	// We invalidate the entire proof-of-stake chain
	t.Logf("INFO: Changing default response to INVALID")
	importerResponseMocker.SetDefaultResponse(&api.PayloadStatusV1{
		// The default is now that the execution client returns INVALID + LVH==0x00..00
		Status:          Invalid,
		LatestValidHash: &(common.Hash{}),
	})

	// Wait until newPayload or forkchoiceUpdated are called at least once
	ctxTimeout, cancel := context.WithTimeout(ctx, time.Second*20)
	defer cancel()
	select {
	case <-importerResponseMocker.NewPayloadCalled:
	case <-importerResponseMocker.ForkchoiceUpdatedCalled:
	case <-ctxTimeout.Done():
		t.Fatalf(
			"FAIL: Timeout waiting for beacon node to send engine directive: %v",
			err,
		)
	}

	// Wait a couple of slots here to make sure syncing does not produce a false positive
	time.Sleep(time.Duration(config.SlotTime.Uint64()*10) * time.Second)

	// Query the beacon chain head of the importer node, it should still
	// point to a pre-merge block.
	headInfo, err := importer.BlockHeader(ctx, eth2api.BlockHead)
	if err != nil {
		t.Fatalf("FAIL: Failed to poll head importer head: %v", err)
	}

	if headInfo.Header.Message.Slot != (builderExecutionBlock.Slot() - 1) {
		ctxTimeout, cancel := context.WithTimeout(ctx, time.Second*5)
		defer cancel()
		var headOptStatus clients.BlockV2OptimisticResponse
		if exists, err := eth2api.SimpleRequest(
			ctxTimeout, importer.API,
			eth2api.FmtGET(
				"/eth/v2/beacon/blocks/%s",
				eth2api.BlockHead.BlockId(),
			),
			&headOptStatus,
		); err != nil {
			// Block still not synced
			fmt.Printf(
				"DEBUG: Queried block %s: %v\n",
				eth2api.BlockHead.BlockId(),
				err,
			)
		} else if !exists {
			// Block still not synced
			fmt.Printf("DEBUG: Queried block %s: %v\n", eth2api.BlockHead.BlockId(), err)
		}

		t.Fatalf(
			"FAIL: Importer head is beyond the invalid execution payload block: importer=%v:%d, builder=%v:%d, execution_optimistic=%t",
			headInfo.Root,
			headInfo.Header.Message.Slot,
			builderExecutionBlock.StateRoot(),
			builderExecutionBlock.Slot(),
			headOptStatus.ExecutionOptimistic,
		)
	}
}

func SyncingWithChainHavingInvalidPostTransitionBlock(
	t *hivesim.T,
	env *tn.Environment,
	n clients.NodeDefinition,
) {
	var (
		safeSlotsToImportOptimistically = big.NewInt(8)
		safeSlotsImportThreshold        = uint64(4)
		ctx                             = context.Background()
	)
	if clientSafeSlots, ok := SAFE_SLOTS_TO_IMPORT_OPTIMISTICALLY_CLIENT_OVERRIDE[n.ConsensusClient]; ok {
		safeSlotsToImportOptimistically = clientSafeSlots
	}

	config := getClientConfig(n).Join(&tn.Config{
		NodeDefinitions: clients.NodeDefinitions{
			// Builder
			clients.NodeDefinition{
				ExecutionClient: n.ExecutionClient,
				ConsensusClient: n.ConsensusClient,
				ValidatorShares: 1,
				ChainGenerator: &pow.ChainGenerator{
					BlockCount: 1,
					Config:     pow.Defaults,
				},
			},
			// Importer
			clients.NodeDefinition{
				ExecutionClient: n.ExecutionClient,
				ConsensusClient: n.ConsensusClient,
				ValidatorShares: 0,
				ChainGenerator: &pow.ChainGenerator{
					BlockCount: 1,
					Config:     pow.Defaults,
				},
			},
		},
		AltairForkEpoch:                 common.Big1,
		BellatrixForkEpoch:              common.Big2,
		Eth1Consensus:                   el.ExecutionPreChain{},
		SafeSlotsToImportOptimistically: safeSlotsToImportOptimistically,
	})

	testnet := tn.StartTestnet(ctx, t, env, config)
	defer testnet.Stop()

	var (
		builder       = testnet.BeaconClients()[0]
		importer      = testnet.BeaconClients()[1]
		importerProxy = testnet.Proxies().Running()[1]
	)

	importerResponseMocker := payload_spoof.NewEngineResponseMocker(
		&api.PayloadStatusV1{
			// By default we respond SYNCING to any payload
			Status:          Syncing,
			LatestValidHash: nil,
		},
	)
	importerResponseMocker.AddCallbacksToProxy(importerProxy)

	// Wait until the builder creates the first block with an execution payload
	execPayloadCtx, cancel := testnet.Spec().SlotTimeoutContext(
		ctx,
		SlotsUntilMerge(ctx, testnet, config),
	)
	defer cancel()
	_, err := testnet.WaitForExecutionPayload(execPayloadCtx)
	if err != nil {
		t.Fatalf("FAIL: Waiting for execution payload on builder: %v", err)
	}

	// Fetch the first execution block which will be used for verification
	builderExecutionBlock, err := builder.GetFirstExecutionBeaconBlock(ctx)
	if err != nil || builderExecutionBlock == nil {
		t.Fatalf("FAIL: Could not find first execution block")
	}
	execPayload, _ := builderExecutionBlock.ExecutionPayload()
	transitionPayloadHash := execPayload.BlockHash
	t.Logf(
		"Builder Execution block found on slot %d, hash=%s",
		builderExecutionBlock.Slot(),
		transitionPayloadHash,
	)

	// We wait until the importer reaches optimistic sync
	optimisticStateCtx, cancel := testnet.Spec().SlotTimeoutContext(
		ctx,
		beacon.Slot(safeSlotsToImportOptimistically.Uint64()+
			safeSlotsImportThreshold),
	)
	defer cancel()
	_, err = importer.WaitForOptimisticState(
		optimisticStateCtx,
		eth2api.BlockIdSlot(builderExecutionBlock.Slot()),
		true,
	)
	if err != nil {
		t.Fatalf(
			"FAIL: Timeout waiting for beacon node to become optimistic: %v",
			err,
		)
	}

	// We invalidate the chain after the transition payload
	importerResponseMocker.AddResponse(
		transitionPayloadHash,
		&api.PayloadStatusV1{
			// Transition payload is valid
			Status:          Valid,
			LatestValidHash: &transitionPayloadHash,
		},
	)
	importerResponseMocker.SetDefaultResponse(&api.PayloadStatusV1{
		// The default is now that the execution client returns INVALID
		// with latest valid hash equal to the transition payload
		Status:          Invalid,
		LatestValidHash: &transitionPayloadHash,
	})

	// Wait until newPayload or forkchoiceUpdated are called at least once
	ctxTimeout, cancel := context.WithTimeout(ctx, time.Second*20)
	defer cancel()
	select {
	case <-importerResponseMocker.NewPayloadCalled:
	case <-importerResponseMocker.ForkchoiceUpdatedCalled:
	case <-ctxTimeout.Done():
		t.Fatalf(
			"FAIL: Timeout waiting for beacon node to send engine directive: %v",
			err,
		)
	}

	// Wait a couple of slots here to make sure syncing does not produce a false positive
	<-testnet.Spec().SlotsTimeout(5)

	// Query the beacon chain head of the importer node,
	// it should point to transition payload block.
	block, err := importer.GetFirstExecutionBeaconBlock(ctx)
	if err != nil || block == nil {
		t.Fatalf("FAIL: Block not found: %v", err)
	}
	payload, _ := block.ExecutionPayload()
	if common.BytesToHash(payload.BlockHash[:]) != transitionPayloadHash {
		t.Fatalf(
			"FAIL: Latest payload in the importer is not the transition payload: %v",
			common.BytesToHash(payload.BlockHash[:]),
		)
	}
}

func ReOrgSyncWithChainHavingInvalidTerminalBlock(
	t *hivesim.T,
	env *tn.Environment,
	n clients.NodeDefinition,
) {
	var (
		safeSlotsToImportOptimistically = big.NewInt(8)
		ctx                             = context.Background()
	)
	if clientSafeSlots, ok := SAFE_SLOTS_TO_IMPORT_OPTIMISTICALLY_CLIENT_OVERRIDE[n.ConsensusClient]; ok {
		safeSlotsToImportOptimistically = clientSafeSlots
	}

	// We are going to produce two PoW chains for three different clients
	// EL_A:
	EL_A := &pow.ChainGenerator{ // TD = 0x40000
		BlockCount: 2,
		Config:     pow.Defaults,
	}
	// EL_B:
	EL_B := &pow.ChainGenerator{ // TD = 0x40000
		BlockCount: 2,
		Config:     pow.Defaults,
	}

	// Network is partitioned from the start, execution client subnets A and B will never be able to communicate with
	// each other.
	config := getClientConfig(n).Join(&tn.Config{
		NodeDefinitions: clients.NodeDefinitions{
			// Builder 1
			clients.NodeDefinition{
				ExecutionClient: n.ExecutionClient,
				ConsensusClient: n.ConsensusClient,
				ValidatorShares: 10,
				ChainGenerator:  EL_A,
				ExecutionSubnet: "A",
			},
			// Importer 1
			clients.NodeDefinition{
				ExecutionClient: n.ExecutionClient,
				ConsensusClient: n.ConsensusClient,
				ValidatorShares: 0,
				ChainGenerator:  EL_A,
				ExecutionSubnet: "A",
			},
			// Builder 2
			clients.NodeDefinition{
				ExecutionClient: n.ExecutionClient,
				ConsensusClient: n.ConsensusClient,
				ValidatorShares: 10,
				ChainGenerator:  EL_B,
				ExecutionSubnet: "B",
			},
			// Importer 2
			clients.NodeDefinition{
				ExecutionClient: n.ExecutionClient,
				ConsensusClient: n.ConsensusClient,
				ValidatorShares: 0,
				ChainGenerator:  EL_B,
				ExecutionSubnet: "B",
			},
		},
		Eth1Consensus:                   el.ExecutionPreChain{},
		TerminalTotalDifficulty:         big.NewInt(0x40000),
		SafeSlotsToImportOptimistically: safeSlotsToImportOptimistically,
	})

	testnet := tn.StartTestnet(ctx, t, env, config)
	defer testnet.Stop()

	// One pair of clients will produce the first execution payload, to which the other pair won't be able to sync,
	// because they are not interconnected.
	// Therefore only one client pair will end up in optimistic sync mode.
	type BuilderImporterInfo struct {
		Builder        *clients.Node
		Importer       *clients.Node
		ChainGenerator *pow.ChainGenerator
	}
	builderImporterPairs := []BuilderImporterInfo{
		{
			Builder:        testnet.Nodes[0],
			Importer:       testnet.Nodes[1],
			ChainGenerator: EL_A,
		},
		{
			Builder:        testnet.Nodes[2],
			Importer:       testnet.Nodes[3],
			ChainGenerator: EL_B,
		},
	}
	optimisticPairChan := make(chan *BuilderImporterInfo)
	for i, p := range builderImporterPairs {
		p := p
		// Only one pair will reach optimistic sync
		go func(i int, p *BuilderImporterInfo) {
			optimisticStateCtx, cancel := testnet.Spec().SlotTimeoutContext(
				ctx,
				SlotsUntilMerge(ctx, testnet, config)+
					beacon.Slot(safeSlotsToImportOptimistically.Uint64()+4),
			)
			defer cancel()
			_, err := p.Builder.BeaconClient.WaitForOptimisticState(
				optimisticStateCtx,
				eth2api.BlockHead,
				true,
			)
			if err != nil {
				return
			}
			t.Logf("INFO: Detected optimistic sync on pair %d", i)
			select {
			case optimisticPairChan <- p:
			default:
			}
		}(i, &p)
	}

	var optimisticPair *BuilderImporterInfo
	select {
	case optimisticPair = <-optimisticPairChan:
	case <-testnet.Spec().SlotsTimeout(SlotsUntilMerge(ctx, testnet, config) +
		beacon.Slot(safeSlotsToImportOptimistically.Uint64()+4)):
		t.Fatalf("FAIL: Timeout waiting for pair to become optimistic")
	}

	t.Logf(
		"INFO: Reached optimistic sync on nodes %d + %d",
		optimisticPair.Builder.Index,
		optimisticPair.Importer.Index,
	)

	// After the client pair reaches optimistic sync, invalidate the execution payload to trigger a
	responseMocker := payload_spoof.NewEngineResponseMocker(
		&api.PayloadStatusV1{
			Status:          Invalid,
			LatestValidHash: &(common.Hash{}),
		},
	)
	// Every payload generated by this same pair is not invalidated
	responseMocker.AddGetPayloadPassthroughToProxy(
		optimisticPair.Builder.ExecutionClient.Proxy(),
	)
	// The original head of the PoW chain needs to passthrough too
	responseMocker.AddPassthrough(
		optimisticPair.ChainGenerator.Head().Hash(),
		true,
	)
	// Add the callbacks to the optimistic sync pair
	responseMocker.AddCallbacksToProxy(
		optimisticPair.Builder.ExecutionClient.Proxy(),
	)
	responseMocker.AddCallbacksToProxy(
		optimisticPair.Importer.ExecutionClient.Proxy(),
	)

	// Wait until the optimistic builder creates its first block with an execution payload.
	// At this point the builder is no longer optimistic
	execPayloadCtx, cancel := testnet.Spec().SlotTimeoutContext(
		ctx,
		12,
	)
	defer cancel()
	_, err := optimisticPair.Builder.BeaconClient.WaitForExecutionPayload(
		execPayloadCtx)
	if err != nil {
		if err := debug.PrintAllTestnetBeaconBlocks(ctx, t, testnet.BeaconClients().Running()); err != nil {
			t.Logf("FAIL: Error while printing all blocks: %v", err)
		}
		t.Fatalf(
			"FAIL: Waiting for execution payload on optimistic builder: %v",
			err,
		)
	}

	// Wait until the optimistic importer fetches the first execution payload
	// from the optimistic builder
	execPayloadCtx, cancel = testnet.Spec().SlotTimeoutContext(
		ctx,
		12,
	)
	defer cancel()
	_, err = optimisticPair.Importer.BeaconClient.WaitForExecutionPayload(
		execPayloadCtx)
	if err != nil {
		// Print all heads for debugging
		if err := debug.PrintAllTestnetBeaconBlocks(ctx, t, testnet.BeaconClients().Running()); err != nil {
			t.Logf("FAIL: Error while printing all blocks: %v", err)
		}
		t.Fatalf(
			"FAIL: Waiting for execution payload on optimistic importer: %v",
			err,
		)
	}

	// Verify the heads match
	optimisticClients := clients.ExecutionClients{
		optimisticPair.Builder.ExecutionClient,
		optimisticPair.Importer.ExecutionClient,
	}
	if match, err := optimisticClients.CheckHeads(t, ctx); err != nil {
		t.Fatalf("FAIL: Error getting head of optimistic clients: %v", err)
	} else if !match {
		// Print all heads for debugging
		if err := debug.PrintAllTestnetBeaconBlocks(ctx, t, testnet.BeaconClients().Running()); err != nil {
			t.Logf("FAIL: Error while printing all blocks: %v", err)
		}
		t.Fatalf("FAIL: Heads of the optimistic clients don't match")
	}

	// Verify heads of the two client pairs are different
	forkedClients := clients.ExecutionClients{
		testnet.ExecutionClients().Running()[0],
		testnet.ExecutionClients().Running()[2],
	}
	if match, err := forkedClients.CheckHeads(t, ctx); err != nil {
		t.Fatalf("FAIL: Error getting head of clients: %v", err)
	} else if match {
		// Print all heads for debugging
		if err := debug.PrintAllTestnetBeaconBlocks(ctx, t, testnet.BeaconClients().Running()); err != nil {
			t.Logf("FAIL: Error while printing all blocks: %v", err)
		}
		t.Fatalf("FAIL: Heads of the clients match")
	}
	time.Sleep(
		time.Duration(testnet.Spec().SECONDS_PER_SLOT) * time.Second * 6,
	)
	if err := debug.PrintAllTestnetBeaconBlocks(ctx, t, testnet.BeaconClients().Running()); err != nil {
		t.Fatalf("PrintAllTestnetBeaconBlocks failed: %v", err)
	}
}

func NoViableHeadDueToOptimisticSync(
	t *hivesim.T,
	env *tn.Environment,
	n clients.NodeDefinition,
) {
	var (
		safeSlotsToImportOptimistically = big.NewInt(8)
		// safeSlotsImportThreshold        = uint64(4)
		ctx = context.Background()
	)
	if clientSafeSlots, ok := SAFE_SLOTS_TO_IMPORT_OPTIMISTICALLY_CLIENT_OVERRIDE[n.ConsensusClient]; ok {
		safeSlotsToImportOptimistically = clientSafeSlots
	}

	config := getClientConfig(n).Join(&tn.Config{
		NodeDefinitions: clients.NodeDefinitions{
			// Builder 1
			clients.NodeDefinition{
				ExecutionClient: n.ExecutionClient,
				ConsensusClient: n.ConsensusClient,
				ValidatorShares: 1,
			},
			// Importer
			clients.NodeDefinition{
				ExecutionClient:      n.ExecutionClient,
				ConsensusClient:      n.ConsensusClient,
				ValidatorShares:      0,
				TestVerificationNode: true,
			},
			// Builder 2
			clients.NodeDefinition{
				ExecutionClient: n.ExecutionClient,
				ConsensusClient: n.ConsensusClient,
				// We are going to duplicate keys from builder 1
				ValidatorShares: 0,
				// Don't start until later in the run
				DisableStartup: true,
			},
		},
		AltairForkEpoch:    common.Big1,
		BellatrixForkEpoch: big.NewInt(4), // Slot 128
		Eth1Consensus: el.ExecutionEthashConsensus{
			MiningNodes: 2,
		},
		SafeSlotsToImportOptimistically: safeSlotsToImportOptimistically,
	})

	testnet := tn.StartTestnet(ctx, t, env, config)
	defer testnet.Stop()

	var (
		builder1      = testnet.BeaconClients()[0]
		importer      = testnet.BeaconClients()[1]
		builder1Proxy = testnet.Proxies().Running()[0]
		importerProxy = testnet.Proxies().Running()[1]
		// Not yet started
		builder2      = testnet.BeaconClients()[2]
		builder2Proxy *proxy.Proxy
	)

	importerNewPayloadResponseMocker := payload_spoof.NewEngineResponseMocker(
		nil,
	)
	importerFcUResponseMocker := payload_spoof.NewEngineResponseMocker(nil)
	importerNewPayloadResponseMocker.AddNewPayloadCallbackToProxy(
		importerProxy,
	)
	importerFcUResponseMocker.AddForkchoiceUpdatedCallbackToProxy(
		importerProxy,
	)

	// We will count the number of payloads produced by builder 1.
	// On Payload 33, the test continues.
	var (
		getPayloadCount int
		latestValidHash common.Hash
		invalidHashes   = make([]common.Hash, 0)
		invalidPayload  common.Hash
	)
	getPayloadCallback := func(res []byte, req []byte) *spoof.Spoof {
		getPayloadCount++
		var (
			payload api.ExecutableData
			err     error
		)
		err = proxy.UnmarshalFromJsonRPCResponse(res, &payload)
		if err != nil {
			panic(err)
		}
		if getPayloadCount >= 10 && getPayloadCount <= 32 {
			if getPayloadCount == 10 {
				latestValidHash = payload.BlockHash
			} else {
				invalidHashes = append(invalidHashes, payload.BlockHash)
			}

			// Return syncing for these payloads
			importerNewPayloadResponseMocker.AddResponse(
				payload.BlockHash,
				&api.PayloadStatusV1{
					Status:          Syncing,
					LatestValidHash: nil,
				},
			)
			importerFcUResponseMocker.AddResponse(
				payload.BlockHash,
				&api.PayloadStatusV1{
					Status:          Syncing,
					LatestValidHash: nil,
				},
			)
		} else if getPayloadCount >= 33 {
			// Invalidate these payloads
			if getPayloadCount == 33 {
				invalidPayload = payload.BlockHash
				for _, h := range invalidHashes {
					importerNewPayloadResponseMocker.AddResponse(h, &api.PayloadStatusV1{
						Status:          Invalid,
						LatestValidHash: &latestValidHash,
					})
					importerFcUResponseMocker.AddResponse(h, &api.PayloadStatusV1{
						Status:          Invalid,
						LatestValidHash: &latestValidHash,
					})
				}

				// Validate latest valid hash too
				importerNewPayloadResponseMocker.AddResponse(latestValidHash, &api.PayloadStatusV1{
					Status:          Valid,
					LatestValidHash: &latestValidHash,
				})
				importerFcUResponseMocker.AddResponse(latestValidHash, &api.PayloadStatusV1{
					Status:          Valid,
					LatestValidHash: &latestValidHash,
				})

			}

			invalidHashes = append(invalidHashes, payload.BlockHash)
			importerNewPayloadResponseMocker.AddResponse(payload.BlockHash, &api.PayloadStatusV1{
				Status:          Invalid,
				LatestValidHash: &latestValidHash,
			})
			importerFcUResponseMocker.AddResponse(payload.BlockHash, &api.PayloadStatusV1{
				Status:          Invalid,
				LatestValidHash: &latestValidHash,
			})

		}
		return nil
	}
	builder1Proxy.AddResponseCallback(EngineGetPayloadV1, getPayloadCallback)

	execPayloadCtx, cancel := testnet.Spec().SlotTimeoutContext(
		ctx,
		32,
	)
	defer cancel()
	if _, err := testnet.WaitForExecutionPayload(execPayloadCtx); err != nil {
		t.Fatalf("Error waiting for execution payload")
	}

forloop:
	for {
		select {
		case eR := <-importerNewPayloadResponseMocker.NewPayloadCalled:
			if invalidPayload != (common.Hash{}) && eR.Hash == invalidPayload && eR.Response.Status == Invalid {
				// The payload has been invalidated
				t.Logf("INFO: Payload %s was correctly invalidated (NewPayload)", invalidPayload)
				break forloop
			}
		case eR := <-importerFcUResponseMocker.ForkchoiceUpdatedCalled:
			if invalidPayload != (common.Hash{}) && eR.Hash == invalidPayload {
				if eR.Response.Status == Invalid {
					// The payload has been invalidated
					t.Logf("INFO: Payload %s was correctly invalidated (ForkchoiceUpdated)", invalidPayload)
					break forloop
				} else {
					t.Fatalf("FAIL: Payload was not invalidated")
				}
			}
		case <-testnet.Spec().SlotsTimeout(beacon.Slot(12)):
			t.Fatalf("FAIL: Context done while waiting for invalidated payload")
		}
	}

	// Sleep a few seconds so the invalid payload is incorporated into the chain
	time.Sleep(time.Duration(config.SlotTime.Int64()/2) * time.Second)

	// We need to check that the latestValidHash Block is indeed optimistic
	// First look for the block on the builder
	lvhBeaconBlock, err := builder1.GetBeaconBlockByExecutionHash(
		ctx,
		latestValidHash,
	)
	if err != nil {
		t.Fatalf(
			"FAIL: Error querying latest valid hash from builder 1: %v",
			err,
		)
	}
	t.Logf(
		"INFO: latest valid hash from builder 1: slot %d, root %v",
		lvhBeaconBlock.Slot(),
		lvhBeaconBlock.StateRoot(),
	)

	lastInvalidBeaconBlock, err := builder1.GetBeaconBlockByExecutionHash(
		ctx,
		invalidPayload,
	)
	if err != nil {
		t.Fatalf(
			"FAIL: Error querying latest invalid hash from builder 1: %v",
			err,
		)
	}
	t.Logf(
		"INFO: latest invalid hash from builder 1: slot %d, root %v",
		lastInvalidBeaconBlock.Slot(),
		lastInvalidBeaconBlock.StateRoot(),
	)

	// Check whether the importer is still optimistic for these blocks

	retriesLeft := 20

	for {
		// Retry several times in order to give the node some time to re-org if necessary
		if retriesLeft--; retriesLeft == 0 {
			debug.PrintAllBeaconBlocks(ctx, t, importer)
			t.Fatalf("FAIL: Unable to get latestValidHash block: %v", err)
		}
		time.Sleep(time.Second)
		t.Logf(
			"INFO: retry %d to obtain beacon block at height %d",
			20-retriesLeft,
			lvhBeaconBlock.Slot(),
		)

		if opt, err := importer.BlockIsOptimistic(ctx, eth2api.BlockHead); err != nil {
			continue
		} else if opt {
			t.Logf("INFO: Head is optimistic from the importer's perspective")
			break
		} else {
			t.Fatalf("FAIL: Head is NOT optimistic from the importer's perspective")
		}
	}

	// Shutdown the builder 1
	builder1.Shutdown()

	// Start builder 2
	// First start the execution node to set the proxy
	if err := testnet.ExecutionClients()[2].Start(); err != nil {
		t.Fatalf("FAIL: Unable to start execution client: %v", err)
	}

	builder2Proxy = testnet.ExecutionClients()[2].Proxy()

	if builder2Proxy == nil {
		t.Fatalf("FAIL: Proxy failed to start")
	}

	importerNewPayloadResponseMocker.AddNewPayloadCallbackToProxy(
		builder2Proxy,
	)
	importerFcUResponseMocker.AddForkchoiceUpdatedCallbackToProxy(
		builder2Proxy,
	)

	// Then start the beacon node
	if err := testnet.BeaconClients()[2].Start(); err != nil {
		t.Fatalf("FAIL: Unable to start beacon client: %v", err)
	}
	// Finally start the validator client reusing the keys of the first builder
	testnet.ValidatorClients()[2].Keys = testnet.ValidatorClients()[0].Keys
	if err := testnet.ValidatorClients()[2].Start(); err != nil {
		t.Fatalf("FAIL: Unable to start validator client: %v", err)
	}

	finalizationContext, cancel := testnet.Spec().SlotTimeoutContext(
		ctx,
		testnet.Spec().SLOTS_PER_EPOCH*3,
	)
	defer cancel()
	c, err := testnet.WaitForCurrentEpochFinalization(finalizationContext)
	if err != nil {
		debug.PrintAllBeaconBlocks(ctx, t, importer)
		debug.PrintAllBeaconBlocks(ctx, t, builder2)
		t.Fatalf(
			"FAIL: Error waiting for finality after builder 2 started: %v",
			err,
		)
	}
	t.Logf("INFO: Finality reached after builder 2 started: epoch %v", c.Epoch)

	// Check that importer is no longer optimistic
	if opt, err := importer.BlockIsOptimistic(ctx, eth2api.BlockHead); err != nil {
		t.Fatalf(
			"FAIL: Error querying optimistic status after finalization on importer: %v",
			err,
		)
	} else if opt {
		t.Fatalf("FAIL: Importer is still optimistic after finalization: execution_optimistic=%t", opt)
	}

	// Check that neither the first invalid payload nor the last invalid payload are included in the importer

	if b, err := importer.GetBeaconBlockByExecutionHash(ctx, invalidHashes[0]); err != nil {
		t.Fatalf("FAIL: Error querying invalid payload: %v", err)
	} else if b != nil {
		t.Fatalf(
			"FAIL: Invalid payload found in importer chain: %d, %v",
			b.Slot(), b.StateRoot(),
		)
	}
	if b, err := importer.GetBeaconBlockByExecutionHash(ctx, invalidPayload); err != nil {
		t.Fatalf("FAIL: Error querying invalid payload: %v", err)
	} else if b != nil {
		t.Fatalf(
			"FAIL: Invalid payload found in importer chain: %d, %v",
			b.Slot(), b.StateRoot(),
		)
	}
}
