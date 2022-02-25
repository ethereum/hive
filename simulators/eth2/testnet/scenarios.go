package main

import (
	"context"
	"fmt"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/hive/hivesim"
	"github.com/ethereum/hive/simulators/eth2/testnet/mock"
)

var (
	VALIDATOR_COUNT            uint64 = 64
	SLOT_TIME                  uint64 = 2
	TERMINCAL_TOTAL_DIFFICULTY        = big.NewInt(100000000)
)

func startTestnet(t *hivesim.T, env *testEnv, config *config) *Testnet {
	prep := prepareTestnet(t, env, config)
	testnet := prep.createTestnet(t)

	genesisTime := testnet.GenesisTime()
	countdown := genesisTime.Sub(time.Now())
	t.Logf("Created new testnet, genesis at %s (%s from now)", genesisTime, countdown)

	// for each key partition, we start a validator client with its own beacon node and eth1 node
	for i, node := range config.Nodes {
		prep.startEth1Node(testnet, env.Clients.ClientByNameAndRole(node.ExecutionClient, "eth1"), config.ShouldMine)
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
		TerminalTotalDifficulty: TERMINCAL_TOTAL_DIFFICULTY,
		Nodes: []node{
			n,
			n,
		},
		ShouldMine: true,
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
		TerminalTotalDifficulty: TERMINCAL_TOTAL_DIFFICULTY,
		Nodes: []node{
			n,
			n,
		},
		ShouldMine: false,
	}

	testnet := startTestnet(t, env, &config)

	m, err := mock.NewMockClient(t, testnet.eth1Genesis.Genesis)
	if err != nil {
		t.Fatalf("unable to initialize mock client: %s", err)
	}
	m.Peer(testnet.eth1[0].MustGetEnode())

	handler := func(m *mock.MockClient, b *types.Block) (bool, error) {
		t.Logf("mock: number=%d, head=%s, total_difficulty=%d, terminal_td=%d", b.Number().Uint64(), shorten(b.Hash().String()), m.TotalDifficulty(), config.TerminalTotalDifficulty)
		m.AnnounceBlock(b)
		if m.IsTerminalTotalDifficulty() {
			t.Logf("mock: terminal total difficulty reached")
			return true, nil
		}
		return false, nil
	}

	ctx := context.Background()
	go m.MineChain(ctx, handler)
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
	m.Close()
}
