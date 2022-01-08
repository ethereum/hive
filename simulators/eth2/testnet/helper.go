package main

import (
	"math/big"

	"github.com/ethereum/hive/simulators/eth2/testnet/setup"
	blsu "github.com/protolambda/bls12-381-util"
)

type TestEnv struct {
	Clients *ClientDefinitionsByRole
	Keys    []*setup.KeyDetails
	Secrets *[]blsu.SecretKey
}

type node struct {
	ExecutionClient string
	ConsensusClient string
}

type config struct {
	AltairForkEpoch         uint64
	MergeForkEpoch          uint64
	ValidatorCount          uint64
	KeyTranches             uint64
	SlotTime                uint64
	TerminalTotalDifficulty *big.Int

	// Node configurations to launch. Each node as a proportional share of
	// validators.
	Nodes []node
}

func (c *config) activeFork() string {
	if c.MergeForkEpoch == 0 {
		return "merge"
	} else if c.AltairForkEpoch == 0 {
		return "altair"
	} else {
		return "phase0"
	}
}
