package testnet

import (
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/hive/simulators/eth2/common/clients"
	"github.com/ethereum/hive/simulators/eth2/common/config"
	consensus_config "github.com/ethereum/hive/simulators/eth2/common/config/consensus"
	execution_config "github.com/ethereum/hive/simulators/eth2/common/config/execution"
	blobber_config "github.com/marioevz/blobber/config"
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
		"grandine",
	}
)

type Config struct {
	*config.ForkConfig                `json:"fork_config,omitempty"`
	*consensus_config.ConsensusConfig `json:"consensus_config,omitempty"`

	// Node configurations to launch. Each node as a proportional share of
	// validators.
	NodeDefinitions clients.NodeDefinitions             `json:"node_definitions,omitempty"`
	Eth1Consensus   execution_config.ExecutionConsensus `json:"eth1_consensus,omitempty"`

	// Execution Layer specific config
	InitialBaseFeePerGas     *big.Int                               `json:"initial_base_fee_per_gas,omitempty"`
	GenesisExecutionAccounts map[common.Address]core.GenesisAccount `json:"genesis_execution_accounts,omitempty"`

	// Consensus Layer specific config
	DisablePeerScoring bool `json:"disable_peer_scoring,omitempty"`

	// Builders
	EnableBuilders bool                  `json:"enable_builders,omitempty"`
	BuilderOptions []mock_builder.Option `json:"builder_options,omitempty"`

	// Blobber
	EnableBlobber  bool                    `json:"enable_blobber,omitempty"`
	BlobberOptions []blobber_config.Option `json:"blobber_options,omitempty"`
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
	// ForkConfig
	c.ForkConfig = a.ForkConfig.Join(b.ForkConfig)

	// ConsensusConfig
	c.ConsensusConfig = a.ConsensusConfig.Join(b.ConsensusConfig)

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

// Check the configuration and its support by the multiple client definitions
func (c *Config) FillDefaults() error {
	if c.SlotsPerEpoch == nil {
		c.SlotsPerEpoch = big.NewInt(32)
	}
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
