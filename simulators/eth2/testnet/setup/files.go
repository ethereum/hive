package setup

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"

	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/hive/hivesim"
	"github.com/protolambda/zrnt/eth2/beacon/common"
	"gopkg.in/yaml.v2"
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

func StateBundle(spec *common.Spec, mnemonic string, valCount uint64, tranches uint64) ([]hivesim.StartOption, error) {
	type mnemonicInfo struct {
		Mnemonic string
		Count    uint64
	}
	m := make([]mnemonicInfo, tranches)
	for i := range m {
		m[i].Mnemonic = mnemonic
		// TODO: error somewhere if this isn't divisble exactly
		m[i].Count = valCount / tranches
	}
	mnemonics, err := yaml.Marshal(m)
	if err != nil {
		return nil, err
	}
	config, err := yaml.Marshal(spec.Config)
	if err != nil {
		return nil, err
	}
	phase0_preset, err := yaml.Marshal(spec.Phase0Preset)
	if err != nil {
		return nil, err
	}
	altair_preset, err := yaml.Marshal(spec.AltairPreset)
	if err != nil {
		return nil, err
	}
	merge_preset, err := yaml.Marshal(spec.MergePreset)
	if err != nil {
		return nil, err
	}
	return []hivesim.StartOption{
		hivesim.WithDynamicFile("/hive/input/mnemonics.yaml", bytesSource(mnemonics)),
		hivesim.WithDynamicFile("/hive/input/config.yaml", bytesSource(config)),
		hivesim.WithDynamicFile("/hive/input/preset_phase0.yaml", bytesSource(phase0_preset)),
		hivesim.WithDynamicFile("/hive/input/preset_altair.yaml", bytesSource(altair_preset)),
		hivesim.WithDynamicFile("/hive/input/preset_merge.yaml", bytesSource(merge_preset)),
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
