package main

import (
	"context"
	"math/big"

	"github.com/ethereum/hive/hivesim"
	"github.com/ethereum/hive/simulators/eth2/common/clients"
	"github.com/ethereum/hive/simulators/eth2/common/config"
	consensus_config "github.com/ethereum/hive/simulators/eth2/common/config/consensus"
	el "github.com/ethereum/hive/simulators/eth2/common/config/execution"
	tn "github.com/ethereum/hive/simulators/eth2/common/testnet"
)

var (
	VALIDATOR_COUNT           = big.NewInt(64)
	SLOT_TIME                 = big.NewInt(6)
	TERMINAL_TOTAL_DIFFICULTY = big.NewInt(100)
)

func Phase0Testnet(t *hivesim.T, env *tn.Environment, n clients.NodeDefinition) {
	config := tn.Config{
		ForkConfig: &config.ForkConfig{
			AltairForkEpoch:         big.NewInt(10),
			BellatrixForkEpoch:      big.NewInt(20),
			TerminalTotalDifficulty: TERMINAL_TOTAL_DIFFICULTY,
		},
		ConsensusConfig: &consensus_config.ConsensusConfig{
			ValidatorCount: VALIDATOR_COUNT,
			SlotTime:       SLOT_TIME,
		},
		NodeDefinitions: []clients.NodeDefinition{
			n,
			n,
		},
		Eth1Consensus: el.ExecutionCliqueConsensus{},
	}

	ctx := context.Background()
	testnet := tn.StartTestnet(ctx, t, env, &config)

	finalized, err := testnet.WaitForFinality(ctx)
	if err != nil {
		t.Fatalf("%v", err)
	}
	if err := testnet.VerifyParticipation(ctx, tn.FirstSlotAfterCheckpoint{Checkpoint: &finalized}, 0.95); err != nil {
		t.Fatalf("%v", err)
	}
	if err := testnet.VerifyExecutionPayloadIsCanonical(ctx, tn.LastSlotAtCheckpoint{Checkpoint: &finalized}); err != nil {
		t.Fatalf("%v", err)
	}
	if err := testnet.VerifyProposers(ctx, tn.LastSlotAtCheckpoint{Checkpoint: &finalized}, false); err != nil {
		t.Fatalf("%v", err)
	}
}

func TransitionTestnet(t *hivesim.T, env *tn.Environment, n clients.NodeDefinition) {
	config := tn.Config{
		ForkConfig: &config.ForkConfig{
			AltairForkEpoch:         big.NewInt(0),
			BellatrixForkEpoch:      big.NewInt(0),
			TerminalTotalDifficulty: TERMINAL_TOTAL_DIFFICULTY,
		},
		ConsensusConfig: &consensus_config.ConsensusConfig{
			ValidatorCount: VALIDATOR_COUNT,
			SlotTime:       SLOT_TIME,
		},
		NodeDefinitions: []clients.NodeDefinition{
			n,
			n,
		},
		Eth1Consensus: el.ExecutionCliqueConsensus{},
	}

	ctx := context.Background()
	testnet := tn.StartTestnet(ctx, t, env, &config)

	finalized, err := testnet.WaitForFinality(ctx)
	if err != nil {
		t.Fatalf("%v", err)
	}
	if err := testnet.VerifyParticipation(ctx, tn.FirstSlotAfterCheckpoint{Checkpoint: &finalized}, 0.95); err != nil {
		t.Fatalf("%v", err)
	}
	if err := testnet.VerifyExecutionPayloadIsCanonical(ctx, tn.LastSlotAtCheckpoint{Checkpoint: &finalized}); err != nil {
		t.Fatalf("%v", err)
	}
	if err := testnet.VerifyProposers(ctx, tn.LastSlotAtCheckpoint{Checkpoint: &finalized}, false); err != nil {
		t.Fatalf("%v", err)
	}
}
