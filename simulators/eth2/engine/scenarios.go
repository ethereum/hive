package main

import (
	"context"
	"math/big"
	"time"

	"github.com/ethereum/hive/hivesim"
	"github.com/rauljordan/engine-proxy/proxy"
)

var (
	VALIDATOR_COUNT           uint64 = 64
	SLOT_TIME                 uint64 = 3
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
		t.Fatalf("%v", err)
	}
	if err := testnet.VerifyParticipation(ctx, finalized, 0.95); err != nil {
		t.Fatalf("%v", err)
	}
	if err := testnet.VerifyExecutionPayloadIsCanonical(ctx, finalized); err != nil {
		t.Fatalf("%v", err)
	}
	if err := testnet.VerifyProposers(ctx, finalized); err != nil {
		t.Fatalf("%v", err)
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
		t.Fatalf("%v", err)
	}
	if err := testnet.VerifyParticipation(ctx, finalized, 0.95); err != nil {
		t.Fatalf("%v", err)
	}
	if err := testnet.VerifyExecutionPayloadIsCanonical(ctx, finalized); err != nil {
		t.Fatalf("%v", err)
	}
	if err := testnet.VerifyProposers(ctx, finalized); err != nil {
		t.Fatalf("%v", err)
	}
	if err := testnet.VerifyELHeads(ctx); err != nil {
		t.Fatalf("%v", err)
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
		t.Fatalf("%v", err)
	}
	if err := testnet.VerifyELHeads(ctx); err == nil {
		t.Fatalf("Expected different heads after spoof %v", err)
	}
}
