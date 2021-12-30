package setup

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"time"

	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/hive/hivesim"
	"github.com/protolambda/zrnt/eth2/beacon/common"
	"github.com/protolambda/ztyp/codec"
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

func StateBundle(templ common.BeaconState, genesisTime time.Time) (hivesim.StartOption, error) {
	state, err := templ.CopyState()
	if err != nil {
		return nil, fmt.Errorf("failed to copy state: %v", err)
	}
	// if err := state.SetGenesisTime(common.Timestamp(genesisTime.Unix())); err != nil {
	//         return nil, fmt.Errorf("failed to set genesis time: %v", err)
	// }
	var stateBytes bytes.Buffer
	if err := state.Serialize(codec.NewEncodingWriter(&stateBytes)); err != nil {
		return nil, fmt.Errorf("failed to serialize genesis state: %v", err)
	}
	return hivesim.WithDynamicFile("/hive/input/genesis.ssz", bytesSource(stateBytes.Bytes())), nil
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
