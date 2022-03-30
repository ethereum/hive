package main

import (
	"context"
	"fmt"
	"math/big"
	"time"

	"github.com/ethereum/hive/hivesim"
)

var (
	VALIDATOR_COUNT           uint64 = 64
	SLOT_TIME                 uint64 = 3
	TERMINAL_TOTAL_DIFFICULTY        = big.NewInt(100)
)

func startTestnet(t *hivesim.T, env *testEnv, config *config) *Testnet {
	prep := prepareTestnet(t, env, config)
	testnet := prep.createTestnet(t)

	genesisTime := testnet.GenesisTime()
	countdown := genesisTime.Sub(time.Now())
	t.Logf("Created new testnet, genesis at %s (%s from now)", genesisTime, countdown)

	// for each key partition, we start a validator client with its own beacon node and eth1 node
	for i, node := range config.Nodes {
		prep.startEth1Node(testnet, env.Clients.ClientByNameAndRole(node.ExecutionClient, "eth1"), config.Eth1Consensus)
		prep.startBeaconNode(testnet, env.Clients.ClientByNameAndRole(fmt.Sprintf("%s-bn", node.ConsensusClient), "beacon"), []int{i})
		prep.startValidatorClient(testnet, env.Clients.ClientByNameAndRole(fmt.Sprintf("%s-vc", node.ConsensusClient), "validator"), i, i)
	}

	return testnet
}

func Phase0Testnet(t *hivesim.T, env *testEnv, n node) {
	config := config{
		AltairForkEpoch:         10,
		MergeForkEpoch:          20,
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
