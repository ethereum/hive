package testnet

import (
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/hive/simulators/eth2/common/clients"
	execution_config "github.com/ethereum/hive/simulators/eth2/common/config/execution"
	mock_builder "github.com/marioevz/mock-builder/mock"
)

var (
	Big0 = big.NewInt(0)
	Big1 = big.NewInt(1)

	MAINNET_SLOT_TIME int64 = 12
	MINIMAL_SLOT_TIME int64 = 6

	// Clients that support the minimal slot time in hive
	MINIMAL_SLOT_TIME_CLIENTS = []string{
		"lighthouse",
		"teku",
		"prysm",
		"lodestar",
	}
)

type Config struct {
	AltairForkEpoch                 *big.Int `json:"altair_fork_epoch,omitempty"`
	BellatrixForkEpoch              *big.Int `json:"bellatrix_fork_epoch,omitempty"`
	CapellaForkEpoch                *big.Int `json:"capella_fork_epoch,omitempty"`
	ValidatorCount                  *big.Int `json:"validator_count,omitempty"`
	KeyTranches                     *big.Int `json:"key_tranches,omitempty"`
	SlotTime                        *big.Int `json:"slot_time,omitempty"`
	TerminalTotalDifficulty         *big.Int `json:"terminal_total_difficulty,omitempty"`
	SafeSlotsToImportOptimistically *big.Int `json:"safe_slots_to_import_optimistically,omitempty"`
	ExtraShares                     *big.Int `json:"extra_shares,omitempty"`

	// Node configurations to launch. Each node as a proportional share of
	// validators.
	NodeDefinitions clients.NodeDefinitions             `json:"node_definitions,omitempty"`
	Eth1Consensus   execution_config.ExecutionConsensus `json:"eth1_consensus,omitempty"`

	// Execution Layer specific config
	InitialBaseFeePerGas     *big.Int                               `json:"initial_base_fee_per_gas,omitempty"`
	GenesisExecutionAccounts map[common.Address]core.GenesisAccount `json:"genesis_execution_accounts,omitempty"`

	// Builders
	EnableBuilders bool                  `json:"enable_builders,omitempty"`
	BuilderOptions []mock_builder.Option `json:"builder_options,omitempty"`
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

// Check the configuration and its support by the multiple client definitions
func (c *Config) fillDefaults() error {
	if c.SlotTime == nil {
		allNodeDefinitions := c.NodeDefinitions
		if len(
			allNodeDefinitions.FilterByCL(MINIMAL_SLOT_TIME_CLIENTS),
		) == len(
			allNodeDefinitions,
		) {
			// If all clients support using minimal 6 second slot time, use it
			c.SlotTime = big.NewInt(MINIMAL_SLOT_TIME)
		} else {
			// Otherwise, use the mainnet 12 second slot time
			c.SlotTime = big.NewInt(MAINNET_SLOT_TIME)
		}
		fmt.Printf("INFO: using %d second slot time\n", c.SlotTime)
	}
	return nil
}
