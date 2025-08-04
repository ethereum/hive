package consensus_config

import (
	"bytes"
	"fmt"
	"math/big"

	el_common "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/hive/hivesim"
	"github.com/ethereum/hive/simulators/eth2/common/config"
	execution_config "github.com/ethereum/hive/simulators/eth2/common/config/execution"
	"github.com/holiman/uint256"
	"github.com/protolambda/zrnt/eth2/beacon/common"
	"github.com/protolambda/ztyp/codec"
	"github.com/protolambda/ztyp/tree"
	"github.com/protolambda/ztyp/view"
	"gopkg.in/yaml.v2"
)

type ConsensusConfig struct {
	ValidatorCount                  *big.Int `json:"validator_count,omitempty"`
	KeyTranches                     *big.Int `json:"key_tranches,omitempty"`
	SlotsPerEpoch                   *big.Int `json:"slots_per_epoch,omitempty"`
	SlotTime                        *big.Int `json:"slot_time,omitempty"`
	SafeSlotsToImportOptimistically *big.Int `json:"safe_slots_to_import_optimistically,omitempty"`
	ExtraShares                     *big.Int `json:"extra_shares,omitempty"`
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
func (a *ConsensusConfig) Join(b *ConsensusConfig) *ConsensusConfig {
	if b == nil {
		return a
	}
	if a == nil {
		return b
	}
	c := ConsensusConfig{}
	c.ValidatorCount = choose(a.ValidatorCount, b.ValidatorCount)
	c.KeyTranches = choose(a.KeyTranches, b.KeyTranches)
	c.SlotsPerEpoch = choose(a.SlotsPerEpoch, b.SlotsPerEpoch)
	c.SlotTime = choose(a.SlotTime, b.SlotTime)
	c.SafeSlotsToImportOptimistically = choose(
		a.SafeSlotsToImportOptimistically,
		b.SafeSlotsToImportOptimistically,
	)
	c.ExtraShares = choose(a.ExtraShares, b.ExtraShares)
	return &c
}

func StateBundle(state common.BeaconState) (hivesim.StartOption, error) {
	var stateBytes bytes.Buffer
	if err := state.Serialize(codec.NewEncodingWriter(&stateBytes)); err != nil {
		return nil, fmt.Errorf("failed to serialize genesis state: %v", err)
	}
	return hivesim.WithDynamicFile(
		"/hive/input/genesis.ssz",
		config.BytesSource(stateBytes.Bytes()),
	), nil
}

func BuildSpec(
	baseSpec *common.Spec,
	forkConfig *config.ForkConfig,
	consensusConfig *ConsensusConfig,
	depositAddress common.Eth1Address,
	executionGenesis *execution_config.ExecutionGenesis,
) (*common.Spec, error) {
	// Generate beacon spec
	// TODO: specify build-target based on preset, to run clients in mainnet or minimal mode.
	// copy the default mainnet config, and make some minimal modifications for testnet usage
	specCpy := *baseSpec
	spec := &specCpy
	spec.Config.DEPOSIT_CONTRACT_ADDRESS = depositAddress
	spec.Config.DEPOSIT_CHAIN_ID = view.Uint64View(executionGenesis.ChainID())
	spec.Config.DEPOSIT_NETWORK_ID = view.Uint64View(executionGenesis.NetworkID())
	spec.Config.ETH1_FOLLOW_DISTANCE = 1

	for _, currentConfig := range []struct {
		ForkName      string
		VersionConfig *common.Version
		EpochConfig   *common.Epoch
		ForkConfig    *big.Int
	}{
		{
			ForkName:      "genesis",
			VersionConfig: &spec.Config.GENESIS_FORK_VERSION,
			EpochConfig:   nil,
			ForkConfig:    nil,
		},
		{
			ForkName:      "altair",
			VersionConfig: &spec.Config.ALTAIR_FORK_VERSION,
			EpochConfig:   &spec.Config.ALTAIR_FORK_EPOCH,
			ForkConfig:    forkConfig.AltairForkEpoch,
		},
		{
			ForkName:      "bellatrix",
			VersionConfig: &spec.Config.BELLATRIX_FORK_VERSION,
			EpochConfig:   &spec.Config.BELLATRIX_FORK_EPOCH,
			ForkConfig:    forkConfig.BellatrixForkEpoch,
		},
		{
			ForkName:      "capella",
			VersionConfig: &spec.Config.CAPELLA_FORK_VERSION,
			EpochConfig:   &spec.Config.CAPELLA_FORK_EPOCH,
			ForkConfig:    forkConfig.CapellaForkEpoch,
		},
		{
			ForkName:      "deneb",
			VersionConfig: &spec.Config.DENEB_FORK_VERSION,
			EpochConfig:   &spec.Config.DENEB_FORK_EPOCH,
			ForkConfig:    forkConfig.DenebForkEpoch,
		},
	} {
		// Modify version to avoid conflicts with base spec values
		if currentConfig.VersionConfig == nil {
			return nil, fmt.Errorf("VersionConfig was not configured for %s", currentConfig.ForkName)
		}
		currentConfig.VersionConfig[3] = 0x0a
		fmt.Printf("Fork %s version %s\n", currentConfig.ForkName, currentConfig.VersionConfig.String())

		// Adjust epoch to the fork configuration if it is set
		if currentConfig.EpochConfig != nil && currentConfig.ForkConfig != nil {
			*currentConfig.EpochConfig = common.Epoch(currentConfig.ForkConfig.Uint64())
			fmt.Printf("Fork %s at epoch %d\n", currentConfig.ForkName, currentConfig.ForkConfig.Uint64())
		}
	}

	if consensusConfig.ValidatorCount == nil {
		return nil, fmt.Errorf("ValidatorCount was not configured")
	}
	spec.Config.MIN_GENESIS_ACTIVE_VALIDATOR_COUNT = view.Uint64View(
		consensusConfig.ValidatorCount.Uint64(),
	)
	if consensusConfig.SlotTime != nil {
		spec.Config.SECONDS_PER_SLOT = common.Timestamp(
			consensusConfig.SlotTime.Uint64(),
		)
	}
	tdd, _ := uint256.FromBig(forkConfig.TerminalTotalDifficulty)
	spec.Config.TERMINAL_TOTAL_DIFFICULTY = view.Uint256View(*tdd)
	if executionGenesis.IsPostMerge() {
		spec.Config.TERMINAL_BLOCK_HASH = tree.Root(executionGenesis.Hash)
		spec.Config.TERMINAL_BLOCK_HASH_ACTIVATION_EPOCH = common.Timestamp(0)
	}

	// Validators can exit immediately
	spec.Config.SHARD_COMMITTEE_PERIOD = 0
	spec.Config.CHURN_LIMIT_QUOTIENT = 2

	// Validators can withdraw immediately
	spec.Config.MIN_VALIDATOR_WITHDRAWABILITY_DELAY = 0

	spec.Config.PROPOSER_SCORE_BOOST = 40
	return spec, nil
}

func ConsensusConfigsBundle(
	spec *common.Spec,
	executionGenesisHash el_common.Hash,
	valCount uint64,
) (hivesim.StartOption, error) {
	specConfig, err := yaml.Marshal(spec.Config)
	if err != nil {
		return nil, err
	}
	phase0Preset, err := yaml.Marshal(spec.Phase0Preset)
	if err != nil {
		return nil, err
	}
	altairPreset, err := yaml.Marshal(spec.AltairPreset)
	if err != nil {
		return nil, err
	}
	bellatrixPreset, err := yaml.Marshal(spec.BellatrixPreset)
	if err != nil {
		return nil, err
	}
	capellaPreset, err := yaml.Marshal(spec.CapellaPreset)
	if err != nil {
		return nil, err
	}
	denebPreset, err := yaml.Marshal(spec.DenebPreset)
	if err != nil {
		return nil, err
	}
	return hivesim.Bundle(
		hivesim.WithDynamicFile(
			"/hive/input/config.yaml",
			config.BytesSource(specConfig),
		),
		hivesim.WithDynamicFile(
			"/hive/input/preset_phase0.yaml",
			config.BytesSource(phase0Preset),
		),
		hivesim.WithDynamicFile(
			"/hive/input/preset_altair.yaml",
			config.BytesSource(altairPreset),
		),
		hivesim.WithDynamicFile(
			"/hive/input/preset_bellatrix.yaml",
			config.BytesSource(bellatrixPreset),
		),
		hivesim.WithDynamicFile(
			"/hive/input/preset_capella.yaml",
			config.BytesSource(capellaPreset),
		),
		hivesim.WithDynamicFile(
			"/hive/input/preset_deneb.yaml",
			config.BytesSource(denebPreset),
		),
		hivesim.Params{
			"HIVE_ETH2_ETH1_GENESIS_HASH": executionGenesisHash.String(),
		},
	), nil
}
