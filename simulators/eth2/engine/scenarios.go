package main

import (
	"context"
	"crypto/rand"
	"math/big"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/hive/hivesim"
	"github.com/rauljordan/engine-proxy/proxy"
)

var (
	VALIDATOR_COUNT           uint64 = 64
	SLOT_TIME                 uint64 = 6
	TERMINAL_TOTAL_DIFFICULTY        = big.NewInt(100)
)

func TransitionTestnet(t *hivesim.T, env *testEnv, n node) {
	config := config{
		AltairForkEpoch:         0,
		MergeForkEpoch:          0,
		ValidatorCount:          VALIDATOR_COUNT,
		SlotTime:                SLOT_TIME,
		TerminalTotalDifficulty: TERMINAL_TOTAL_DIFFICULTY,
		Nodes: []node{
			n,
			n,
		},
		Eth1Consensus: Clique,
	}

	testnet := startTestnet(t, env, &config)

	ctx := context.Background()
	finalized, err := testnet.WaitForFinality(ctx)
	if err != nil {
		t.Fatalf("FAIL: Waiting for finality: %v", err)
	}
	if err := testnet.VerifyParticipation(ctx, finalized, 0.95); err != nil {
		t.Fatalf("FAIL: Verifying participation: %v", err)
	}
	if err := testnet.VerifyExecutionPayloadIsCanonical(ctx, finalized); err != nil {
		t.Fatalf("FAIL: Verifying execution payload is canonical: %v", err)
	}
	if err := testnet.VerifyProposers(ctx, finalized, false); err != nil {
		t.Fatalf("FAIL: Verifying proposers: %v", err)
	}
}

func TestRPCError(t *hivesim.T, env *testEnv, n node) {
	config := config{
		AltairForkEpoch:         0,
		MergeForkEpoch:          0,
		ValidatorCount:          VALIDATOR_COUNT,
		SlotTime:                SLOT_TIME,
		TerminalTotalDifficulty: TERMINAL_TOTAL_DIFFICULTY,
		Nodes: []node{
			n,
			n,
		},
		Eth1Consensus: Clique,
	}

	testnet := startTestnet(t, env, &config)

	ctx := context.Background()
	finalized, err := testnet.WaitForFinality(ctx)
	if err != nil {
		t.Fatalf("FAIL: Waiting for finality: %v", err)
	}
	if err := testnet.VerifyParticipation(ctx, finalized, 0.95); err != nil {
		t.Fatalf("FAIL: Verifying participation: %v", err)
	}
	if err := testnet.VerifyExecutionPayloadIsCanonical(ctx, finalized); err != nil {
		t.Fatalf("FAIL: Verifying execution payload is canonical: %v", err)
	}
	if err := testnet.VerifyProposers(ctx, finalized, false); err != nil {
		t.Fatalf("FAIL: Verifying proposers: %v", err)
	}
	if err := testnet.VerifyELHeads(ctx); err != nil {
		t.Fatalf("FAIL: Verifying EL Heads: %v", err)
	}
	fields := make(map[string]interface{})
	fields["headBlockHash"] = "weird error"
	spoof := &proxy.Spoof{
		Method: "engine_forkchoiceUpdatedV1",
		Fields: fields,
	}
	testnet.proxies[0].AddRequest(spoof)
	time.Sleep(24 * time.Second)
	if err := testnet.VerifyParticipation(ctx, finalized, 0.95); err != nil {
		t.Fatalf("FAIL: %v", err)
	}
	if err := testnet.VerifyELHeads(ctx); err == nil {
		t.Fatalf("FAIL: Expected different heads after spoof %v", err)
	}
}

func InvalidTransitionPayload(t *hivesim.T, env *testEnv, n node) {
	config := config{
		AltairForkEpoch:         0,
		MergeForkEpoch:          0,
		ValidatorCount:          VALIDATOR_COUNT,
		SlotTime:                SLOT_TIME,
		TerminalTotalDifficulty: TERMINAL_TOTAL_DIFFICULTY,
		Nodes: Nodes{
			n,
			n,
			n,
		},
		Eth1Consensus: Clique,
	}

	testnet := startTestnet(t, env, &config)

	// All proxies will use the same callback, therefore we need to use a lock and a counter
	var (
		lock               sync.Mutex
		pCount             int
		invalidPayloadHash common.Hash
	)

	// The EL mock will intercept the first engine_getPayloadV1 and corrupt the stateRoot in the response
	c := func(b []byte) *proxy.Spoof {
		lock.Lock()
		defer lock.Unlock()
		pCount++
		// Only invalidate the very first payload obtained (transition payload)
		if pCount == 1 {
			var (
				payload ExecutableDataV1
				spoof   *proxy.Spoof
				err     error
			)
			UnmarshalFromJsonRPC(b, &payload)
			t.Logf("INFO (%v): Invalidating payload: %s", t.TestID, b)
			invalidPayloadHash, spoof, err = generateInvalidPayloadSpoof("engine_getPayloadV1", &payload, InvalidStateRoot)
			if err != nil {
				panic(err)
			}
			t.Logf("INFO (%v): Invalidated payload hash: %v", t.TestID, invalidPayloadHash)
			return spoof
		}
		return nil
	}
	for _, p := range testnet.proxies {
		p.AddResponseCallback("engine_getPayloadV1", c)
	}

	ctx := context.Background()
	finalized, err := testnet.WaitForFinality(ctx)
	if err != nil {
		t.Fatalf("FAIL: Waiting for finality: %v", err)
	}
	if err := testnet.VerifyParticipation(ctx, finalized, 0.95); err != nil {
		t.Fatalf("FAIL: Verifying participation: %v", err)
	}
	if err := testnet.VerifyExecutionPayloadIsCanonical(ctx, finalized); err != nil {
		t.Fatalf("FAIL: Verifying execution payload is canonical: %v", err)
	}
	if err := testnet.VerifyProposers(ctx, finalized, true); err != nil {
		t.Fatalf("FAIL: Verifying proposers: %v", err)
	}
	if err := testnet.VerifyELHeads(ctx); err != nil {
		t.Fatalf("FAIL: Verifying EL Heads: %v", err)
	}
	if b := testnet.VerifyExecutionPayloadHashInclusion(ctx, finalized, invalidPayloadHash); b != nil {
		t.Fatalf("FAIL: Invalid Payload %v was included in slot %d (%v)", invalidPayloadHash, b.Message.Slot, b.Message.StateRoot)
	}

}

func InvalidPayloadBlockHash(t *hivesim.T, env *testEnv, n node) {
	config := config{
		AltairForkEpoch:         0,
		MergeForkEpoch:          0,
		ValidatorCount:          VALIDATOR_COUNT,
		SlotTime:                SLOT_TIME,
		TerminalTotalDifficulty: TERMINAL_TOTAL_DIFFICULTY,
		Nodes: Nodes{
			n,
			n,
			n,
		},
		Eth1Consensus: Clique,
	}

	testnet := startTestnet(t, env, &config)

	// All proxies will use the same callback, therefore we need to use a lock and a counter
	var (
		lock               sync.Mutex
		pCount             int
		invalidPayloadHash common.Hash
	)

	// The EL mock will intercept the first engine_getPayloadV1 and corrupt the hash in the response
	c := func(b []byte) *proxy.Spoof {
		lock.Lock()
		defer lock.Unlock()
		pCount++
		// Invalidate the payload by corrupting the hash
		if pCount == 1 {
			var (
				payload ExecutableDataV1
				spoof   *proxy.Spoof
				err     error
			)
			UnmarshalFromJsonRPC(b, &payload)
			t.Logf("INFO (%v): Invalidating payload: %s", t.TestID, b)
			rand.Read(invalidPayloadHash[:])
			invalidPayloadHash, spoof, err = customizePayloadSpoof("engine_getPayloadV1", &payload, &CustomPayloadData{
				BlockHash: &invalidPayloadHash,
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
		p.AddResponseCallback("engine_getPayloadV1", c)
	}

	ctx := context.Background()
	finalized, err := testnet.WaitForFinality(ctx)
	if err != nil {
		t.Fatalf("FAIL: Waiting for finality: %v", err)
	}
	if err := testnet.VerifyParticipation(ctx, finalized, 0.95); err != nil {
		t.Fatalf("FAIL: Verifying participation: %v", err)
	}
	if err := testnet.VerifyExecutionPayloadIsCanonical(ctx, finalized); err != nil {
		t.Fatalf("FAIL: Verifying execution payload is canonical: %v", err)
	}
	if err := testnet.VerifyProposers(ctx, finalized, true); err != nil {
		t.Fatalf("FAIL: Verifying proposers: %v", err)
	}
	if err := testnet.VerifyELHeads(ctx); err != nil {
		t.Fatalf("FAIL: Verifying EL Heads: %v", err)
	}
	if b := testnet.VerifyExecutionPayloadHashInclusion(ctx, finalized, invalidPayloadHash); b != nil {
		t.Fatalf("FAIL: Invalid Payload %v was included in slot %d (%v)", invalidPayloadHash, b.Message.Slot, b.Message.StateRoot)
	}

}

// The produced and broadcasted payload contains an invalid prevrandao value.
// The PREVRANDAO opcode is not used in any transaction and therefore not introduced in the state changes.
func IncorrectHeaderPrevRandaoPayload(t *hivesim.T, env *testEnv, n node) {
	config := config{
		AltairForkEpoch:         0,
		MergeForkEpoch:          0,
		ValidatorCount:          VALIDATOR_COUNT,
		SlotTime:                SLOT_TIME,
		TerminalTotalDifficulty: TERMINAL_TOTAL_DIFFICULTY,
		Nodes: Nodes{
			n,
			n,
			n,
		},
		Eth1Consensus: Clique,
	}

	testnet := startTestnet(t, env, &config)

	// All proxies will use the same callback, therefore we need to use a lock and a counter
	var (
		lock               sync.Mutex
		pCount             int
		invalidPayloadHash common.Hash
	)

	// The EL mock will intercept an engine_getPayloadV1 call and corrupt the prevRandao in the response
	c := func(b []byte) *proxy.Spoof {
		lock.Lock()
		defer lock.Unlock()
		pCount++
		// Invalidate a payload after the transition payload
		if pCount == 2 {
			var (
				payload ExecutableDataV1
				spoof   *proxy.Spoof
				err     error
			)
			UnmarshalFromJsonRPC(b, &payload)
			t.Logf("INFO (%v): Invalidating payload: %s", t.TestID, b)
			invalidPayloadHash, spoof, err = generateInvalidPayloadSpoof("engine_getPayloadV1", &payload, InvalidPrevRandao)
			if err != nil {
				panic(err)
			}
			t.Logf("INFO (%v): Invalidated payload hash: %v", t.TestID, invalidPayloadHash)
			return spoof
		}
		return nil
	}
	for _, p := range testnet.proxies {
		p.AddResponseCallback("engine_getPayloadV1", c)
	}

	ctx := context.Background()
	finalized, err := testnet.WaitForFinality(ctx)
	if err != nil {
		t.Fatalf("FAIL: Waiting for finality: %v", err)
	}
	if err := testnet.VerifyParticipation(ctx, finalized, 0.95); err != nil {
		t.Fatalf("FAIL: Verifying participation: %v", err)
	}
	if err := testnet.VerifyExecutionPayloadIsCanonical(ctx, finalized); err != nil {
		t.Fatalf("FAIL: Verifying execution payload is canonical: %v", err)
	}
	if err := testnet.VerifyProposers(ctx, finalized, true); err != nil {
		t.Fatalf("FAIL: Verifying proposers: %v", err)
	}
	if err := testnet.VerifyELHeads(ctx); err != nil {
		t.Fatalf("FAIL: Verifying EL Heads: %v", err)
	}
	if b := testnet.VerifyExecutionPayloadHashInclusion(ctx, finalized, invalidPayloadHash); b != nil {
		t.Fatalf("FAIL: Invalid Payload %v was included in slot %d (%v)", invalidPayloadHash, b.Message.Slot, b.Message.StateRoot)
	}
}
