package main

import (
	"fmt"
	"math/big"

	"github.com/ethereum/hive/hivesim"
	"github.com/ethereum/hive/simulators/eth2/testnet/setup"
	blsu "github.com/protolambda/bls12-381-util"
)

type ConsensusType int

const (
	Ethash ConsensusType = iota
	Clique
)

type testSpec struct {
	Name  string
	About string
	Run   func(*hivesim.T, *testEnv, node)
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

func (n *node) String() string {
	return fmt.Sprintf("%s-%s", n.ConsensusClient, n.ExecutionClient)
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
	Nodes         []node
	Eth1Consensus ConsensusType
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
