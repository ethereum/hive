package testnet

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	mock_builder "github.com/ethereum/hive/simulators/eth2/common/builder/mock"
	"github.com/ethereum/hive/simulators/eth2/common/clients"
	execution_config "github.com/ethereum/hive/simulators/eth2/common/config/execution"
)

var (
	Big0 = big.NewInt(0)
	Big1 = big.NewInt(1)
)

type Config struct {
	AltairForkEpoch                 *big.Int
	BellatrixForkEpoch              *big.Int
	CapellaForkEpoch                *big.Int
	ValidatorCount                  *big.Int
	KeyTranches                     *big.Int
	SlotTime                        *big.Int
	TerminalTotalDifficulty         *big.Int
	SafeSlotsToImportOptimistically *big.Int
	ExtraShares                     *big.Int

	// Node configurations to launch. Each node as a proportional share of
	// validators.
	NodeDefinitions clients.NodeDefinitions
	Eth1Consensus   execution_config.ExecutionConsensus

	// Execution Layer specific config
	InitialBaseFeePerGas     *big.Int
	GenesisExecutionAccounts map[common.Address]core.GenesisAccount

	// Builders
	EnableBuilders bool
	BuilderOptions []mock_builder.Option
}

// Choose a configuration value. `b` takes precedence
func choose(a, b *big.Int) *big.Int {
	if b != nil {
		return new(big.Int).Set(b)
	}
	if a != nil {
		return new(big.Int).Set(a)
	}
	return nil
}

// Join two configurations. `b` takes precedence
func (a *Config) Join(b *Config) *Config {
	c := Config{}
	// Forks
	c.AltairForkEpoch = choose(a.AltairForkEpoch, b.AltairForkEpoch)
	c.BellatrixForkEpoch = choose(a.BellatrixForkEpoch, b.BellatrixForkEpoch)
	c.CapellaForkEpoch = choose(a.CapellaForkEpoch, b.CapellaForkEpoch)

	// Testnet config
	c.ValidatorCount = choose(a.ValidatorCount, b.ValidatorCount)
	c.KeyTranches = choose(a.KeyTranches, b.KeyTranches)
	c.SlotTime = choose(a.SlotTime, b.SlotTime)
	c.TerminalTotalDifficulty = choose(
		a.TerminalTotalDifficulty,
		b.TerminalTotalDifficulty,
	)
	c.SafeSlotsToImportOptimistically = choose(
		a.SafeSlotsToImportOptimistically,
		b.SafeSlotsToImportOptimistically,
	)
	c.ExtraShares = choose(a.ExtraShares, b.ExtraShares)

	// EL config
	c.InitialBaseFeePerGas = choose(
		a.InitialBaseFeePerGas,
		b.InitialBaseFeePerGas,
	)

	if b.NodeDefinitions != nil {
		c.NodeDefinitions = b.NodeDefinitions
	} else {
		c.NodeDefinitions = a.NodeDefinitions
	}

	if b.Eth1Consensus != nil {
		c.Eth1Consensus = b.Eth1Consensus
	} else {
		c.Eth1Consensus = a.Eth1Consensus
	}

	if b.GenesisExecutionAccounts != nil {
		c.GenesisExecutionAccounts = b.GenesisExecutionAccounts
	} else {
		c.GenesisExecutionAccounts = a.GenesisExecutionAccounts
	}

	c.EnableBuilders = b.EnableBuilders || a.EnableBuilders

	return &c
}

func (c *Config) activeFork() string {
	if c.CapellaForkEpoch != nil && c.CapellaForkEpoch.Cmp(Big0) == 0 {
		return "capella"
	} else if c.BellatrixForkEpoch != nil && c.BellatrixForkEpoch.Cmp(Big0) == 0 {
		return "bellatrix"
	} else if c.AltairForkEpoch != nil && c.AltairForkEpoch.Cmp(Big0) == 0 {
		return "altair"
	} else {
		return "phase0"
	}
}
