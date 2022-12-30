package consensus_config

import (
	"bytes"
	"fmt"

	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/hive/hivesim"
	"github.com/ethereum/hive/simulators/eth2/common/config"
	"github.com/protolambda/zrnt/eth2/beacon/common"
	"github.com/protolambda/ztyp/codec"
	"gopkg.in/yaml.v2"
)

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

func ConsensusConfigsBundle(
	spec *common.Spec,
	genesis *core.Genesis,
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
	genesisHash := genesis.ToBlock().Hash()
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
		hivesim.Params{
			"HIVE_ETH2_ETH1_GENESIS_HASH": genesisHash.String(),
		},
	), nil
}
