package main

import (
	"fmt"
	"math/big"

	"github.com/ethereum/hive/hivesim"
	"github.com/ethereum/hive/simulators/eth2/testnet/setup"
	blsu "github.com/protolambda/bls12-381-util"
)

type testCase interface {
	Run(t *hivesim.T, env *testEnv)
}

type testSpec struct {
	Name  string
	About string
	Fn    testCase
}

func (s *testSpec) Run(t *hivesim.T, env *testEnv) {
	s.Fn.Run(t, env)
}

type testEnv struct {
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

func shorten(s string) string {
	start := s[0:6]
	end := s[62:]
	return fmt.Sprintf("%s..%s", start, end)
}
