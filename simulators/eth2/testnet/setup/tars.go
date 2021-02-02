package setup

import (
	"archive/tar"
	"bytes"
	"fmt"
	"github.com/ethereum/hive/hivesim"
	"github.com/protolambda/zrnt/eth2/beacon"
	"github.com/protolambda/ztyp/codec"
	"io"
	"io/ioutil"
	"time"
)

func virtualFileTar(data []byte, destPath string, mode int64, t time.Time, tw *tar.Writer) error {
	if err := tw.WriteHeader(&tar.Header{
		Name:       destPath,
		Size:       int64(len(data)),
		Mode:       mode,
		Uid:        1000,
		Gid:        1000,
		ModTime:    t,
		AccessTime: t,
		ChangeTime: t,
	}); err != nil {
		return fmt.Errorf("failed write tar header for %s: %v", destPath, err)
	}
	if _, err := tw.Write(data); err != nil {
		return fmt.Errorf("failed to write config for %s: %v", destPath, err)
	}
	return nil
}

func packTar(b []byte) hivesim.StartOption {
	return hivesim.WithTAR(func() io.ReadCloser {
		return ioutil.NopCloser(bytes.NewReader(b))
	})
}

func StateTar(spec *beacon.Spec, keys []*KeyDetails, genesisTime beacon.Timestamp) (hivesim.StartOption, error) {
	state, err := BuildPhase0State(spec, keys, genesisTime)
	if err != nil {
		return nil, err
	}
	var stateBytes bytes.Buffer
	if err := state.Serialize(codec.NewEncodingWriter(&stateBytes)); err != nil {
		return nil, fmt.Errorf("failed to serialize genesis state: %v", err)
	}

	var out bytes.Buffer
	tw := tar.NewWriter(&out)
	if err := virtualFileTar(stateBytes.Bytes(), "/hive/input/genesis.ssz", 0664, time.Unix(0, 0), tw); err != nil {
		return nil, err
	}
	if err := tw.Close(); err != nil {
		return nil, err
	}
	return packTar(out.Bytes()), nil
}

func KeysTar(keys []*KeyDetails) (hivesim.StartOption, error) {
	var out bytes.Buffer
	tw := tar.NewWriter(&out)
	t := time.Unix(0, 0)
	for _, k := range keys {
		p := fmt.Sprintf("/hive/input/keystores/0x%x/keystore.json", k.ValidatorPubkey[:])
		if err := virtualFileTar(k.ValidatorKeystoreJSON, p, 0600, t, tw); err != nil {
			return nil, fmt.Errorf("failed to keystore: %v", err)
		}

		p = fmt.Sprintf("/hive/input/secrets/0x%x", k.ValidatorPubkey[:])
		if err := virtualFileTar([]byte(k.ValidatorKeystorePass), p, 0600, t, tw); err != nil {
			return nil, fmt.Errorf("failed to keystore secret: %v", err)
		}
	}
	if err := tw.Close(); err != nil {
		return nil, err
	}
	return packTar(out.Bytes()), nil
}
