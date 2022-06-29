package main

import (
	"bytes"
	"context"
	"fmt"
	"math/big"
	"strings"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/hive/hivesim"
	"github.com/ethereum/hive/simulators/eth2/engine/setup"
	"github.com/protolambda/eth2api"
	"github.com/protolambda/eth2api/client/beaconapi"
	"github.com/protolambda/zrnt/eth2/beacon/bellatrix"
	"github.com/rauljordan/engine-proxy/proxy"
)

var (
	DEFAULT_VALIDATOR_COUNT           uint64 = 60
	DEFAULT_SLOT_TIME                 uint64 = 6
	DEFAULT_TERMINAL_TOTAL_DIFFICULTY uint64 = 100

	// Default config used for all tests unless a client specific config exists
	DEFAULT_CONFIG = &Config{
		ValidatorCount:          big.NewInt(int64(DEFAULT_VALIDATOR_COUNT)),
		SlotTime:                big.NewInt(int64(DEFAULT_SLOT_TIME)),
		TerminalTotalDifficulty: big.NewInt(int64(DEFAULT_TERMINAL_TOTAL_DIFFICULTY)),
		AltairForkEpoch:         common.Big0,
		MergeForkEpoch:          common.Big0,
		Eth1Consensus:           &setup.Eth1CliqueConsensus{},
	}

	// Clients that do not support starting on epoch 0 with all forks enabled.
	// Tests take longer for these clients.
	INCREMENTAL_FORKS_CONFIG = &Config{
		TerminalTotalDifficulty: big.NewInt(int64(DEFAULT_TERMINAL_TOTAL_DIFFICULTY) * 3),
		AltairForkEpoch:         common.Big1,
		MergeForkEpoch:          common.Big2,
	}
	INCREMENTAL_FORKS_CLIENTS = map[string]bool{
		"nimbus": true,
		"prysm":  true,
	}
)

func getClientConfig(n node) *Config {
	config := DEFAULT_CONFIG
	if INCREMENTAL_FORKS_CLIENTS[n.ConsensusClient] == true {
		config = config.join(INCREMENTAL_FORKS_CONFIG)
	}
	return config
}

func TransitionTestnet(t *hivesim.T, env *testEnv, n node) {
	config := getClientConfig(n).join(&Config{
		Nodes: []node{
			n,
			n,
		},
	})

	testnet := startTestnet(t, env, config)
	defer testnet.stopTestnet()

	ctx := context.Background()
	finalized, err := testnet.WaitForFinality(ctx)
	if err != nil {
		t.Fatalf("FAIL: Waiting for finality: %v", err)
	}
	if err := testnet.VerifyParticipation(ctx, &finalized, 0.95); err != nil {
		t.Fatalf("FAIL: Verifying participation: %v", err)
	}
	if err := testnet.VerifyExecutionPayloadIsCanonical(ctx, &finalized); err != nil {
		t.Fatalf("FAIL: Verifying execution payload is canonical: %v", err)
	}
	if err := testnet.VerifyProposers(ctx, &finalized, false); err != nil {
		t.Fatalf("FAIL: Verifying proposers: %v", err)
	}
}

func TestRPCError(t *hivesim.T, env *testEnv, n node) {
	config := getClientConfig(n).join(&Config{
		Nodes: []node{
			n,
			n,
		},
	})

	testnet := startTestnet(t, env, config)
	defer testnet.stopTestnet()

	ctx := context.Background()
	finalized, err := testnet.WaitForFinality(ctx)
	if err != nil {
		t.Fatalf("FAIL: Waiting for finality: %v", err)
	}
	if err := testnet.VerifyParticipation(ctx, &finalized, 0.95); err != nil {
		t.Fatalf("FAIL: Verifying participation: %v", err)
	}
	if err := testnet.VerifyExecutionPayloadIsCanonical(ctx, &finalized); err != nil {
		t.Fatalf("FAIL: Verifying execution payload is canonical: %v", err)
	}
	if err := testnet.VerifyProposers(ctx, &finalized, true); err != nil {
		t.Fatalf("FAIL: Verifying proposers: %v", err)
	}
	if err := testnet.VerifyELHeads(ctx); err != nil {
		t.Fatalf("FAIL: Verifying EL Heads: %v", err)
	}
	fields := make(map[string]interface{})
	fields["headBlockHash"] = "weird error"
	spoof := &proxy.Spoof{
		Method: EngineForkchoiceUpdatedV1,
		Fields: fields,
	}
	testnet.proxies[0].AddRequest(spoof)
	time.Sleep(24 * time.Second)
	if err := testnet.VerifyParticipation(ctx, &finalized, 0.95); err != nil {
		t.Fatalf("FAIL: %v", err)
	}
	if err := testnet.VerifyELHeads(ctx); err == nil {
		t.Fatalf("FAIL: Expected different heads after spoof %v", err)
	}
}

func InvalidTransitionPayload(t *hivesim.T, env *testEnv, n node) {
	config := getClientConfig(n).join(&Config{
		Nodes: Nodes{
			n,
			n,
			n,
		},
	})

	testnet := startTestnet(t, env, config)
	defer testnet.stopTestnet()

	// All proxies will use the same callback, therefore we need to use a lock and a counter
	var (
		getPayloadLock        sync.Mutex
		getPayloadCount       int
		invalidPayloadHash    common.Hash
		invalidPayloadProxyId int
	)

	// The EL mock will intercept the first engine_getPayloadV1 and corrupt the stateRoot in the response
	getPayloadCallbackGen := func(id int) func(res []byte, req []byte) *proxy.Spoof {
		return func(res []byte, req []byte) *proxy.Spoof {
			getPayloadLock.Lock()
			defer getPayloadLock.Unlock()
			getPayloadCount++
			if getPayloadCount == 1 {
				// We are not going to spoof anything here, we just need to save the transition payload hash and the id of the validator that generated it
				// to invalidate it in other clients.
				var (
					payload ExecutableDataV1
				)
				err := UnmarshalFromJsonRPCResponse(res, &payload)
				if err != nil {
					panic(err)
				}
				invalidPayloadHash = payload.BlockHash
				invalidPayloadProxyId = id
				// Remove this validator from the verification
				testnet.removeNodeAsVerifier(id)
			}
			return nil
		}
	}

	// Here we will intercept the response from the rest of the clients that did not generate the payload to artificially invalidate them
	newPayloadCallbackGen := func(id int) func(res []byte, req []byte) *proxy.Spoof {
		return func(res []byte, req []byte) *proxy.Spoof {
			var (
				payload ExecutableDataV1
				spoof   *proxy.Spoof
				err     error
			)
			err = UnmarshalFromJsonRPCRequest(req, &payload)
			if err != nil {
				panic(err)
			}
			// Only invalidate if the payload hash matches the invalid payload hash and this validator is not the one that generated it
			if invalidPayloadProxyId != id && payload.BlockHash == invalidPayloadHash {
				status := PayloadStatusV1{
					Status:          Invalid,
					LatestValidHash: &(common.Hash{}),
					ValidationError: nil,
				}
				spoof, err = payloadStatusSpoof(EngineNewPayloadV1, &status)
				if err != nil {
					panic(err)
				}
				t.Logf("INFO: Invalidating payload on validator %d: %v", id, payload.BlockHash)
			}
			return spoof
		}
	}
	// We pass the id of the proxy to identify which one it is within the callback
	for i, p := range testnet.proxies {
		p.AddResponseCallback(EngineGetPayloadV1, getPayloadCallbackGen(i))
		p.AddResponseCallback(EngineNewPayloadV1, newPayloadCallbackGen(i))
	}

	ctx := context.Background()
	_, err := testnet.WaitForExecutionPayload(ctx)
	if err != nil {
		t.Fatalf("FAIL: Waiting for execution payload: %v", err)
	}

	// Wait 5 slots
	time.Sleep(time.Duration(testnet.spec.SECONDS_PER_SLOT) * time.Second * 5)

	// Verify beacon block with invalid payload is not accepted
	if b := testnet.VerifyExecutionPayloadHashInclusion(ctx, nil, invalidPayloadHash); b != nil {
		t.Fatalf("FAIL: Invalid Payload %v was included in slot %d (%v)", invalidPayloadHash, b.Message.Slot, b.Message.StateRoot)
	}

}

func InvalidTransitionPayloadBlockHash(t *hivesim.T, env *testEnv, n node) {
	config := getClientConfig(n).join(&Config{
		Nodes: Nodes{
			n,
			n,
			n,
		},
	})

	testnet := startTestnet(t, env, config)
	defer testnet.stopTestnet()

	// All proxies will use the same callback, therefore we need to use a lock and a counter
	var (
		getPayloadLock        sync.Mutex
		getPayloadCount       int
		invalidPayloadHash    common.Hash
		invalidPayloadProxyId int
	)

	// The EL mock will intercept the first engine_getPayloadV1 and corrupt the stateRoot in the response
	getPayloadCallbackGen := func(id int) func(res []byte, req []byte) *proxy.Spoof {
		return func(res []byte, req []byte) *proxy.Spoof {
			getPayloadLock.Lock()
			defer getPayloadLock.Unlock()
			getPayloadCount++
			t.Logf("INFO: getPayload callback %d", getPayloadCount)
			if getPayloadCount == 1 {
				// We are not going to spoof anything here, we just need to save the transition payload hash and the id of the validator that generated it
				// to invalidate it in other clients.
				var (
					payload ExecutableDataV1
				)
				err := UnmarshalFromJsonRPCResponse(res, &payload)
				if err != nil {
					panic(err)
				}
				invalidPayloadHash = payload.BlockHash
				invalidPayloadProxyId = id
				// Remove this validator from the verification
				testnet.removeNodeAsVerifier(id)
			}
			return nil
		}
	}

	// Here we will intercept the response from the rest of the clients that did not generate the payload to artificially invalidate them
	newPayloadCallbackGen := func(id int) func(res []byte, req []byte) *proxy.Spoof {
		return func(res []byte, req []byte) *proxy.Spoof {
			t.Logf("INFO: newPayload callback node %d", id)
			var (
				payload ExecutableDataV1
				spoof   *proxy.Spoof
				err     error
			)
			err = UnmarshalFromJsonRPCRequest(req, &payload)
			if err != nil {
				panic(err)
			}
			// Only invalidate if the payload hash matches the invalid payload hash and this validator is not the one that generated it
			if invalidPayloadProxyId != id && payload.BlockHash == invalidPayloadHash {
				status := PayloadStatusV1{
					Status:          InvalidBlockHash,
					LatestValidHash: &(common.Hash{}),
					ValidationError: nil,
				}
				spoof, err = payloadStatusSpoof(EngineNewPayloadV1, &status)
				if err != nil {
					panic(err)
				}
				t.Logf("INFO: Invalidating payload on validator %d: %v", id, payload.BlockHash)
			}
			return spoof
		}
	}
	// We pass the id of the proxy to identify which one it is within the callback
	t.Logf("INFO: proxies = %v", testnet.proxies)
	for i, p := range testnet.proxies {
		t.Logf("INFO: adding callback to proxy %d", i)
		p.AddResponseCallback(EngineGetPayloadV1, getPayloadCallbackGen(i))
		p.AddResponseCallback(EngineNewPayloadV1, newPayloadCallbackGen(i))
	}

	ctx := context.Background()
	_, err := testnet.WaitForExecutionPayload(ctx)
	if err != nil {
		t.Fatalf("FAIL: Waiting for execution payload: %v", err)
	}

	// Wait 5 slots
	time.Sleep(time.Duration(testnet.spec.SECONDS_PER_SLOT) * time.Second * 5)

	// Verify beacon block with invalid payload is not accepted
	if b := testnet.VerifyExecutionPayloadHashInclusion(ctx, nil, invalidPayloadHash); b != nil {
		t.Fatalf("FAIL: Invalid Payload %v was included in slot %d (%v)", invalidPayloadHash, b.Message.Slot, b.Message.StateRoot)
	}

}

// The produced and broadcasted payload contains an invalid prevrandao value.
// The PREVRANDAO opcode is not used in any transaction and therefore not introduced in the state changes.
func IncorrectHeaderPrevRandaoPayload(t *hivesim.T, env *testEnv, n node) {
	config := getClientConfig(n).join(&Config{
		Nodes: Nodes{
			n,
			n,
			n,
		},
	})

	testnet := startTestnet(t, env, config)
	defer testnet.stopTestnet()

	// All proxies will use the same callback, therefore we need to use a lock and a counter
	var (
		getPayloadLock     sync.Mutex
		getPayloadCount    int
		invalidPayloadHash common.Hash
	)

	// The EL mock will intercept an engine_getPayloadV1 call and corrupt the prevRandao in the response
	c := func(res []byte, req []byte) *proxy.Spoof {
		getPayloadLock.Lock()
		defer getPayloadLock.Unlock()
		getPayloadCount++
		// Invalidate a payload after the transition payload
		if getPayloadCount == 2 {
			var (
				payload ExecutableDataV1
				spoof   *proxy.Spoof
				err     error
			)
			err = UnmarshalFromJsonRPCResponse(res, &payload)
			if err != nil {
				panic(err)
			}
			t.Logf("INFO (%v): Invalidating payload: %s", t.TestID, res)
			invalidPayloadHash, spoof, err = generateInvalidPayloadSpoof(EngineGetPayloadV1, &payload, InvalidPrevRandao)
			if err != nil {
				panic(err)
			}
			t.Logf("INFO (%v): Invalidated payload hash: %v", t.TestID, invalidPayloadHash)
			return spoof
		}
		return nil
	}
	for _, p := range testnet.proxies {
		p.AddResponseCallback(EngineGetPayloadV1, c)
	}

	ctx := context.Background()
	_, err := testnet.WaitForExecutionPayload(ctx)
	if err != nil {
		t.Fatalf("FAIL: Waiting for execution payload: %v", err)
	}

	// Wait 5 slots
	time.Sleep(time.Duration(testnet.spec.SECONDS_PER_SLOT) * time.Second * 5)

	// Verify beacon block with invalid payload is not accepted
	if b := testnet.VerifyExecutionPayloadHashInclusion(ctx, nil, invalidPayloadHash); b != nil {
		t.Fatalf("FAIL: Invalid Payload %v was included in slot %d (%v)", invalidPayloadHash, b.Message.Slot, b.Message.StateRoot)
	}
}

// The payload produced by the execution client contains an invalid timestamp value.
// This test covers scenario where the value of the timestamp is so high such that
// the next validators' attempts to produce payloads could fail by invalid payload
// attributes.
func InvalidTimestampPayload(t *hivesim.T, env *testEnv, n node) {
	config := getClientConfig(n).join(&Config{
		Nodes: Nodes{
			n,
			n,
			n,
		},
	})

	testnet := startTestnet(t, env, config)
	defer testnet.stopTestnet()

	// All proxies will use the same callback, therefore we need to use a lock and a counter
	var (
		getPayloadLock     sync.Mutex
		getPayloadCount    int
		invalidPayloadHash common.Hash
		done               = make(chan interface{})
	)

	// The EL mock will intercept an engine_getPayloadV1 call and invalidate the timestamp in the response
	c := func(res []byte, req []byte) *proxy.Spoof {
		getPayloadLock.Lock()
		defer getPayloadLock.Unlock()
		getPayloadCount++
		var (
			payload   ExecutableDataV1
			payloadID PayloadID
			spoof     *proxy.Spoof
			err       error
		)
		err = UnmarshalFromJsonRPCResponse(res, &payload)
		if err != nil {
			panic(err)
		}
		err = UnmarshalFromJsonRPCRequest(req, &payloadID)
		t.Logf("INFO: Got payload %v, parent=%v, from PayloadID=%x", payload.BlockHash, payload.ParentHash, payloadID)
		// Invalidate a payload after the transition payload
		if getPayloadCount == 2 {
			t.Logf("INFO (%v): Invalidating payload: %s", t.TestID, res)
			// We are pushing the timestamp past the slot time.
			// The beacon chain shall identify this and reject the payload.
			newTimestamp := payload.Timestamp + config.SlotTime.Uint64()
			// We add some extraData to guarantee we can identify the payload we altered
			extraData := []byte("alt")
			invalidPayloadHash, spoof, err = customizePayloadSpoof(EngineGetPayloadV1, &payload,
				&CustomPayloadData{
					Timestamp: &newTimestamp,
					ExtraData: &extraData,
				})
			if err != nil {
				panic(err)
			}
			t.Logf("INFO (%v): Invalidated payload hash: %v", t.TestID, invalidPayloadHash)
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

	for _, p := range testnet.proxies {
		p.AddResponseCallback(EngineGetPayloadV1, c)
		p.AddResponseCallback(EngineForkchoiceUpdatedV1, CheckErrorOnForkchoiceUpdatedPayloadAttr(&fcuLock, fcUCountLimit, &fcUAttrCount, fcudone))
	}

	// Wait until the invalid payload is produced
	<-done

	// Wait until we verified all subsequent forkchoiceUpdated calls
	if err := <-fcudone; err != nil {
		t.Fatalf("FAIL: ForkchoiceUpdated call failed: %v", err)
	}

	// Verify beacon block with invalid payload is not accepted
	ctx := context.Background()
	if b := testnet.VerifyExecutionPayloadHashInclusion(ctx, nil, invalidPayloadHash); b != nil {
		t.Fatalf("FAIL: Invalid Payload %v was included in slot %d (%v)", invalidPayloadHash, b.Message.Slot, b.Message.StateRoot)
	}
}

// The produced and broadcasted transition payload has parent with an invalid total difficulty.
func IncorrectTerminalBlockLowerTTD(t *hivesim.T, env *testEnv, n node) {
	config := getClientConfig(n)
	BadTTD := big.NewInt(config.TerminalTotalDifficulty.Int64() - 50)
	config = config.join(&Config{
		Nodes: Nodes{
			node{
				ExecutionClient: n.ExecutionClient,
				ConsensusClient: n.ConsensusClient,
				ValidatorShares: 1,
			},
			node{
				ExecutionClient: n.ExecutionClient,
				ConsensusClient: n.ConsensusClient,
				ValidatorShares: 1,
			},
			node{
				ExecutionClient: n.ExecutionClient,
				ConsensusClient: n.ConsensusClient,
				ValidatorShares: 1,
			},
			node{
				// Add a node with an incorrect TTD to produce the invalid payload
				ExecutionClient:         n.ExecutionClient,
				ValidatorShares:         0,
				TerminalTotalDifficulty: BadTTD,
			},
		},
	})

	testnet := startTestnet(t, env, config)
	defer testnet.stopTestnet()

	// All proxies will use the same callback, therefore we need to use a lock and a counter
	var (
		getPayloadLock     sync.Mutex
		getPayloadCount    int
		invalidPayloadHash common.Hash
		invalidPayloadChan = make(chan ExecutableDataV1)
		finish             = make(chan interface{})
	)

	// Check on the bad TTD client when their TTD is hit and create a payload, which will be available to the first payload creator
	go func() {
		ec := NewEngineClient(t, testnet.eth1[len(testnet.eth1)-1], BadTTD)

		var (
			ttdBlockHeader *types.Header
			ttdHit         bool
		)
		for {
			ttdBlockHeader, ttdHit = ec.checkTTD()
			if ttdHit {
				break
			}
			select {
			case <-time.After(time.Second):
			case <-finish:
				return
			}
		}
		r, err := ec.EngineForkchoiceUpdatedV1(&ForkchoiceStateV1{
			HeadBlockHash: ttdBlockHeader.Hash(),
		}, &PayloadAttributesV1{
			Timestamp:             ttdBlockHeader.Time + 1,
			PrevRandao:            common.Hash{},
			SuggestedFeeRecipient: common.Address{},
		})
		if err != nil {
			panic(err)
		}
		t.Logf("INFO: Forkchoice response: %v", r)
		time.Sleep(time.Second)
		p, err := ec.EngineGetPayloadV1(r.PayloadID)
		if err != nil {
			panic(err)
		}
		invalidPayloadChan <- p
	}()

	// The EL mock will intercept an engine_getPayloadV1 call and corrupt the prevRandao in the response
	c := func(res []byte, req []byte) *proxy.Spoof {
		getPayloadLock.Lock()
		defer getPayloadLock.Unlock()
		getPayloadCount++
		// Invalidate a payload after the transition payload
		if getPayloadCount == 1 {
			var (
				originalPayload ExecutableDataV1
				invalidPayload  ExecutableDataV1
				spoof           *proxy.Spoof
				err             error
			)
			err = UnmarshalFromJsonRPCResponse(res, &originalPayload)
			if err != nil {
				panic(err)
			}
			invalidPayload = <-invalidPayloadChan
			t.Logf("INFO (%v): Invalidating payload: %s", t.TestID, res)
			invalidPayloadHash, spoof, err = customizePayloadSpoof(EngineGetPayloadV1, &invalidPayload, &CustomPayloadData{
				// We need to use the timestamp/prevrandao that the CL used to request the payload, otherwise the payload will be rejected
				// by other reasons other than the incorrect parent terminal block
				Timestamp:  &originalPayload.Timestamp,
				PrevRandao: &originalPayload.PrevRandao,
			})
			if err != nil {
				panic(err)
			}
			t.Logf("INFO (%v): Invalidated payload hash: %v", t.TestID, invalidPayloadHash)
			return spoof
		}
		return nil
	}
	for _, p := range testnet.proxies {
		p.AddResponseCallback(EngineGetPayloadV1, c)
	}

	ctx := context.Background()
	finalized, err := testnet.WaitForFinality(ctx)
	if err != nil {
		t.Fatalf("FAIL: Waiting for finality: %v", err)
	}
	if err := testnet.VerifyParticipation(ctx, &finalized, 0.95); err != nil {
		t.Fatalf("FAIL: Verifying participation: %v", err)
	}
	if err := testnet.VerifyExecutionPayloadIsCanonical(ctx, &finalized); err != nil {
		t.Fatalf("FAIL: Verifying execution payload is canonical: %v", err)
	}
	if err := testnet.VerifyProposers(ctx, &finalized, true); err != nil {
		t.Fatalf("FAIL: Verifying proposers: %v", err)
	}
	if err := testnet.VerifyELHeads(ctx); err != nil {
		t.Fatalf("FAIL: Verifying EL Heads: %v", err)
	}
	if b := testnet.VerifyExecutionPayloadHashInclusion(ctx, &finalized, invalidPayloadHash); b != nil {
		t.Fatalf("FAIL: Invalid Payload %v was included in slot %d (%v)", invalidPayloadHash, b.Message.Slot, b.Message.StateRoot)
	}
}

func SyncingWithInvalidChain(t *hivesim.T, env *testEnv, n node) {
	config := getClientConfig(n).join(&Config{
		Nodes: Nodes{
			// First two nodes will do all the proposals
			node{
				ExecutionClient:      n.ExecutionClient,
				ConsensusClient:      n.ConsensusClient,
				ValidatorShares:      1,
				TestVerificationNode: false,
			},
			node{
				ExecutionClient:      n.ExecutionClient,
				ConsensusClient:      n.ConsensusClient,
				ValidatorShares:      1,
				TestVerificationNode: false,
			},
			// Last node will receive invalidated payloads and verify
			node{
				ExecutionClient:      n.ExecutionClient,
				ConsensusClient:      n.ConsensusClient,
				ValidatorShares:      0,
				TestVerificationNode: true,
			},
		},
	})

	testnet := startTestnet(t, env, config)
	defer testnet.stopTestnet()

	var (
		transitionPayloadHeight uint64
		lastValidHash           common.Hash
		invalidPayloadHashes    = make([]common.Hash, 0)
		payloadMap              = make(map[common.Hash]ExecutableDataV1)
		done                    = make(chan interface{})
	)

	// Payloads will be intercepted here and spoofed to simulate sync.
	// Then after 4 payloads (p1, p2, p3, p4), the last one will be marked `INVALID` with
	// `latestValidHash==p1.Hash`
	newPayloadCallback := func(res []byte, req []byte) *proxy.Spoof {
		var (
			payload ExecutableDataV1
			spoof   *proxy.Spoof
			err     error
		)
		err = UnmarshalFromJsonRPCRequest(req, &payload)
		if err != nil {
			panic(err)
		}
		payloadMap[payload.BlockHash] = payload
		if lastValidHash == (common.Hash{}) {
			// This is the transition payload (P1) because it's the first time this callback is called
			transitionPayloadHeight = payload.Number
			lastValidHash = payload.BlockHash
			t.Logf("INFO: Last VALID hash %d: %v", payload.Number-transitionPayloadHeight, payload.BlockHash)
		} else {
			if payload.Number == transitionPayloadHeight+3 {
				// This is P4
				invalidPayloadHashes = append(invalidPayloadHashes, payload.BlockHash)
				status := PayloadStatusV1{
					Status:          Invalid,
					LatestValidHash: &lastValidHash,
					ValidationError: nil,
				}
				spoof, err = payloadStatusSpoof(EngineNewPayloadV1, &status)
				if err != nil {
					panic(err)
				}
				t.Logf("INFO: Returning INVALID payload %d: %v", payload.Number-transitionPayloadHeight, payload.BlockHash)
				select {
				case done <- nil:
				default:
				}
			} else {
				// For all other payloads, including P2/P3, return SYNCING
				invalidPayloadHashes = append(invalidPayloadHashes, payload.BlockHash)
				status := PayloadStatusV1{
					Status:          Syncing,
					LatestValidHash: nil,
					ValidationError: nil,
				}
				spoof, err = payloadStatusSpoof(EngineNewPayloadV1, &status)
				if err != nil {
					panic(err)
				}
				t.Logf("INFO: Returning SYNCING payload %d: %v", payload.Number-transitionPayloadHeight, payload.BlockHash)
			}
		}
		return spoof
	}

	forkchoiceUpdatedCallback := func(res []byte, req []byte) *proxy.Spoof {
		var (
			fcState ForkchoiceStateV1
			pAttr   PayloadAttributesV1
			spoof   *proxy.Spoof
			err     error
		)
		err = UnmarshalFromJsonRPCRequest(req, &fcState, &pAttr)
		if err != nil {
			panic(err)
		}
		if lastValidHash == (common.Hash{}) {
			panic(fmt.Errorf("NewPayload was not called before ForkchoiceUpdated"))
		}
		payload, ok := payloadMap[fcState.HeadBlockHash]
		if !ok {
			panic(fmt.Errorf("payload not found: %v", fcState.HeadBlockHash))
		}

		if payload.Number != transitionPayloadHeight {
			if payload.Number == transitionPayloadHeight+3 {
				// This is P4, but we probably won't receive this since NewPayload(P4) already returned INVALID
				status := PayloadStatusV1{
					Status:          Invalid,
					LatestValidHash: &lastValidHash,
					ValidationError: nil,
				}
				spoof, err = forkchoiceResponseSpoof(EngineForkchoiceUpdatedV1, status, nil)
				if err != nil {
					panic(err)
				}
				t.Logf("INFO: Returning INVALID payload %d (ForkchoiceUpdated): %v", payload.Number-transitionPayloadHeight, payload.BlockHash)
			} else {
				// For all other payloads, including P2/P3, return SYNCING
				status := PayloadStatusV1{
					Status:          Syncing,
					LatestValidHash: nil,
					ValidationError: nil,
				}
				spoof, err = forkchoiceResponseSpoof(EngineForkchoiceUpdatedV1, status, nil)
				if err != nil {
					panic(err)
				}
				t.Logf("INFO: Returning SYNCING payload %d (ForkchoiceUpdated): %v", payload.Number-transitionPayloadHeight, payload.BlockHash)
			}
		}
		return spoof
	}

	// Add the callback to the last proxy which will not produce blocks
	testnet.proxies[len(testnet.proxies)-1].AddResponseCallback(EngineNewPayloadV1, newPayloadCallback)
	testnet.proxies[len(testnet.proxies)-1].AddResponseCallback(EngineForkchoiceUpdatedV1, forkchoiceUpdatedCallback)

	<-done
	ctx := context.Background()

	// Wait a few slots for re-org to happen
	time.Sleep(time.Duration(testnet.spec.SECONDS_PER_SLOT) * time.Second * 5)

	// Verify the head of the chain, it should be a block with the latestValidHash included
	for _, bn := range testnet.verificationBeacons() {
		var versionedBlock eth2api.VersionedSignedBeaconBlock
		if exists, err := beaconapi.BlockV2(ctx, bn.API, eth2api.BlockHead, &versionedBlock); err != nil {
			t.Fatalf("FAIL: Unable to poll beacon chain head")
		} else if !exists {
			t.Fatalf("FAIL: Unable to poll beacon chain head")
		}
		if versionedBlock.Version != "bellatrix" {
			// Block can't contain an executable payload
			t.Fatalf("FAIL: Head of the chain is not a bellatrix fork block")
		}
		block := versionedBlock.Data.(*bellatrix.SignedBeaconBlock)
		payload := block.Message.Body.ExecutionPayload
		if bytes.Compare(payload.BlockHash[:], lastValidHash[:]) != 0 {
			t.Fatalf("FAIL: Head does not contain the expected execution payload: %v != %v", payload.BlockHash.String(), lastValidHash.Hex())
		}
	}

	// Verify payloads
	if b := testnet.VerifyExecutionPayloadHashInclusion(ctx, nil, lastValidHash); b == nil {
		t.Fatalf("FAIL: Valid Payload %v could not be found", lastValidHash)
	}
	for i, p := range invalidPayloadHashes {
		if b := testnet.VerifyExecutionPayloadHashInclusion(ctx, nil, p); b != nil {
			t.Fatalf("FAIL: Invalid Payload (%d) %v was included in slot %d (%v)", i+1, p, b.Message.Slot, b.Message.StateRoot)
		}
	}

}

func BaseFeeEncodingCheck(t *hivesim.T, env *testEnv, n node) {
	config := getClientConfig(n).join(&Config{
		InitialBaseFeePerGas: big.NewInt(9223372036854775807), // 2**63 - 1
		Nodes: []node{
			n,
			n,
		},
	})

	testnet := startTestnet(t, env, config)
	defer testnet.stopTestnet()

	ctx := context.Background()
	transitionPayloadHash, err := testnet.WaitForExecutionPayload(ctx)
	if err != nil {
		t.Fatalf("FAIL: Waiting for execution payload: %v", err)
	}

	// Check the base fee value in the transition payload.
	// Must be at least 256 to guarantee that the endianess encoding is correct.
	ec := NewEngineClient(t, testnet.eth1[0], config.TerminalTotalDifficulty)
	h, err := ec.Eth.HeaderByHash(ec.Ctx(), transitionPayloadHash)
	if err != nil {
		t.Fatalf("FAIL: Unable to get transition payload header from execution client: %v", err)
	}
	if h.Difficulty.Cmp(common.Big0) != 0 {
		t.Fatalf("FAIL: Transition header obtained is not PoS header: difficulty=%x", h.Difficulty)
	}
	if h.BaseFee.Cmp(common.Big256) < 0 {
		t.Fatalf("FAIL: Basefee insufficient for test: %x", h.BaseFee)
	}

	t.Logf("INFO: Transition Payload created with sufficient baseFee: %x", h.BaseFee)
}

func EqualTimestampTerminalTransitionBlock(t *hivesim.T, env *testEnv, n node) {
	config := getClientConfig(n)
	config = config.join(&Config{
		// We are increasing the clique period, therefore we can reduce the TTD
		TerminalTotalDifficulty: big.NewInt(config.TerminalTotalDifficulty.Int64() / 3),
		Nodes: []node{
			n,
			n,
		},

		// The clique period needs to be equal to the slot time to try to get the CL client to attempt to produce
		// a payload with the same timestamp as the terminal block
		Eth1Consensus: &setup.Eth1CliqueConsensus{
			CliquePeriod: config.SlotTime.Uint64(),
		},
	})

	testnet := startTestnet(t, env, config)
	defer testnet.stopTestnet()

	// No ForkchoiceUpdated with payload attributes should fail, which could happen if CL tries to create
	// the payload with `timestamp==terminalBlock.timestamp`.
	var (
		fcuLock       sync.Mutex
		fcUAttrCount  int
		fcudone       = make(chan error)
		fcUCountLimit = 5
	)

	for _, p := range testnet.proxies {
		p.AddResponseCallback(EngineForkchoiceUpdatedV1, CheckErrorOnForkchoiceUpdatedPayloadAttr(&fcuLock, fcUCountLimit, &fcUAttrCount, fcudone))
	}

	// Wait until we verified all subsequent forkchoiceUpdated calls
	if err := <-fcudone; err != nil {
		t.Fatalf("FAIL: ForkchoiceUpdated call failed: %v", err)
	}
}

func TTDBeforeBellatrix(t *hivesim.T, env *testEnv, n node) {
	config := getClientConfig(n)
	config = config.join(&Config{
		AltairForkEpoch:         common.Big1,
		MergeForkEpoch:          common.Big2,
		TerminalTotalDifficulty: big.NewInt(50),
		Nodes: []node{
			n,
			n,
		},
	})

	testnet := startTestnet(t, env, config)
	defer testnet.stopTestnet()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*time.Duration((config.MergeForkEpoch.Uint64()+1)*config.SlotTime.Uint64()*32))
	defer cancel()

	_, err := testnet.WaitForExecutionPayload(ctx)
	if err != nil {
		t.Fatalf("FAIL: Waiting for execution payload: %v", err)
	}
	if err := testnet.VerifyELHeads(ctx); err != nil {
		t.Fatalf("FAIL: Verifying EL Heads: %v", err)
	}
}

func InvalidQuantityPayloadFields(t *hivesim.T, env *testEnv, n node) {
	config := getClientConfig(n).join(&Config{
		Nodes: Nodes{
			n,
			n,
			n,
		},
	})

	testnet := startTestnet(t, env, config)
	defer testnet.stopTestnet()

	// First we are going to wait for the transition to happen
	ctx := context.Background()
	_, err := testnet.WaitForExecutionPayload(ctx)
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
		InvalidationTypeCount
	)

	invalidateQuantityType := func(method string, response []byte, q QuantityType, invType InvalidationType) *proxy.Spoof {
		responseFields := make(map[string]string)
		if err := UnmarshalFromJsonRPCResponse(response, &responseFields); err != nil {
			panic(fmt.Errorf("Unable to unmarshal: %v", err))
		}
		fieldOriginalValue := responseFields[q.Name]
		fields := make(map[string]interface{})
		switch invType {
		case Overflow:
			overflowingStringSize := int((q.BitSize / 4) + 1)
			fields[q.Name] = "0x" + strings.Repeat("0", overflowingStringSize-(len(fieldOriginalValue)-2)) + fieldOriginalValue[2:]
		case LeadingZero:
			if !strings.HasPrefix(fieldOriginalValue, "0x") {
				panic(fmt.Errorf("Invalid original value: %v", fieldOriginalValue))
			}
			fields[q.Name] = "0x0" + fieldOriginalValue[2:]
		case Empty:
			fields[q.Name] = "0x"
		default:
			panic("Invalid QUANTITY invalidation type")
		}

		t.Logf("INFO: Spoofing %s, %s -> %s", q.Name, fieldOriginalValue, fields[q.Name])
		// Return the new payload status spoof
		return &proxy.Spoof{
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
	getPayloadCallback := func(res []byte, req []byte) *proxy.Spoof {
		getPayloadLock.Lock()
		defer getPayloadLock.Unlock()
		defer func() {
			getPayloadCount++
		}()
		var (
			payload ExecutableDataV1
		)
		err := UnmarshalFromJsonRPCResponse(res, &payload)
		if err != nil {
			panic(err)
		}

		field := getPayloadCount / int(InvalidationTypeCount)
		invType := InvalidationType(getPayloadCount % int(InvalidationTypeCount))

		if field >= len(allQuantityFields) {
			select {
			case done <- nil:
			default:
			}
			return nil
		}
		invalidPayloadHashes = append(invalidPayloadHashes, payload.BlockHash)
		return invalidateQuantityType(EngineGetPayloadV1, res, allQuantityFields[field], invType)
	}

	// We pass the id of the proxy to identify which one it is within the callback
	for _, p := range testnet.proxies {
		p.AddResponseCallback(EngineGetPayloadV1, getPayloadCallback)
	}

	// Wait until we are done
	<-done

	// Check that none of the invalidated payloads made it into the beacon chain
	for i, p := range invalidPayloadHashes {
		if b := testnet.VerifyExecutionPayloadHashInclusion(ctx, nil, p); b != nil {
			t.Fatalf("FAIL: Invalid Payload (%d) %v was included in slot %d (%v)", i+1, p, b.Message.Slot, b.Message.StateRoot)
		}
	}
}
