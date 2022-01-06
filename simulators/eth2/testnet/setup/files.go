package setup

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/hive/hivesim"
	"github.com/protolambda/zrnt/eth2/beacon/common"
	"github.com/protolambda/ztyp/codec"
	"gopkg.in/yaml.v2"
	"io"
	"io/ioutil"
)

func bytesSource(data []byte) func() (io.ReadCloser, error) {
	return func() (io.ReadCloser, error) {
		return ioutil.NopCloser(bytes.NewReader(data)), nil
	}
}

func Eth1Bundle(genesis *core.Genesis) (hivesim.StartOption, error) {
	out, err := json.Marshal(genesis)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize genesis state: %v", err)
	}
	return hivesim.WithDynamicFile("genesis.json", bytesSource(out)), nil
}

func StateBundle(templ common.BeaconState, genesisTime common.Timestamp) (hivesim.StartOption, error) {
	state, err := templ.CopyState()
	if err != nil {
		return nil, fmt.Errorf("failed to copy state: %v", err)
	}
	if err := state.SetGenesisTime(genesisTime); err != nil {
		return nil, fmt.Errorf("failed to set genesis time: %v", err)
	}
	var stateBytes bytes.Buffer
	if err := state.Serialize(codec.NewEncodingWriter(&stateBytes)); err != nil {
		return nil, fmt.Errorf("failed to serialize genesis state: %v", err)
	}
	return hivesim.WithDynamicFile("/hive/input/genesis.ssz", bytesSource(stateBytes.Bytes())), nil
}

func ConsensusConfigsBundle(spec *common.Spec, genesis *core.Genesis, valCount uint64) ([]hivesim.StartOption, error) {
	config, err := yaml.Marshal(spec.Config)
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
	mergePreset, err := yaml.Marshal(spec.MergePreset)
	if err != nil {
		return nil, err
	}
	db := rawdb.NewMemoryDatabase()
	genesisHash := genesis.ToBlock(db).Hash()
	return []hivesim.StartOption{
		hivesim.WithDynamicFile("/hive/input/config.yaml", bytesSource(config)),
		hivesim.WithDynamicFile("/hive/input/preset_phase0.yaml", bytesSource(phase0Preset)),
		hivesim.WithDynamicFile("/hive/input/preset_altair.yaml", bytesSource(altairPreset)),
		hivesim.WithDynamicFile("/hive/input/preset_merge.yaml", bytesSource(mergePreset)),
		hivesim.Params{
			"HIVE_ETH2_ETH1_GENESIS_HASH": genesisHash.String(),
		},
	}, nil
}

func KeysBundle(keys []*KeyDetails) hivesim.StartOption {
	opts := make([]hivesim.StartOption, 0, len(keys)*2)
	for _, k := range keys {
		p := fmt.Sprintf("/hive/input/keystores/0x%x/keystore.json", k.ValidatorPubkey[:])
		opts = append(opts, hivesim.WithDynamicFile(p, bytesSource(k.ValidatorKeystoreJSON)))
		p = fmt.Sprintf("/hive/input/secrets/0x%x", k.ValidatorPubkey[:])
		opts = append(opts, hivesim.WithDynamicFile(p, bytesSource([]byte(k.ValidatorKeystorePass))))
	}
	return hivesim.Bundle(opts...)
}
