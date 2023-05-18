package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/ethereum/hive/hivesim"
	beacon_client "github.com/marioevz/eth-clients/clients/beacon"
	"github.com/protolambda/eth2api"
	"github.com/protolambda/ztyp/tree"
	"github.com/protolambda/ztyp/view"
	"gopkg.in/yaml.v3"
)

type BeaconAPIVerification interface {
	Check(t *hivesim.T, parentCtx context.Context, cl *beacon_client.BeaconClient)
}

type BeaconAPIVerifications struct {
	Verifications []BeaconAPIVerification
}

// Beacon API Verifications will be parsed from yaml by identified the first field "method"
func (l *BeaconAPIVerifications) UnmarshalYAML(unmarshal func(interface{}) error) error {
	// Define an intermediate struct to unmarshal the YAML list into
	var rawList []map[string]interface{}

	// Unmarshal the YAML list into the intermediate struct
	if err := unmarshal(&rawList); err != nil {
		return err
	}

	// Iterate over the raw list and create the appropriate object based on the "method" field
	for _, rawObject := range rawList {
		objectType, ok := rawObject["method"].(string)
		if !ok {
			return fmt.Errorf("missing or invalid 'method' field")
		}

		var object BeaconAPIVerification
		switch objectType {
		case "BeaconStateV2":
			object = &BeaconStateV2Verification{}
		default:
			return fmt.Errorf("unknown method: %s", objectType)
		}

		// Unmarshal the raw object data into the appropriate struct
		data, err := yaml.Marshal(rawObject)
		if err != nil {
			return err
		}
		if err := yaml.Unmarshal(data, object); err != nil {
			return err
		}

		// Add the object to the object list
		l.Verifications = append(l.Verifications, object)
	}

	return nil
}

func (l *BeaconAPIVerifications) DoVerifications(t *hivesim.T, parentCtx context.Context, cl *beacon_client.BeaconClient) {
	for _, v := range l.Verifications {
		v.Check(t, parentCtx, cl)
	}
}

type BeaconStateV2VerificationFields struct {
	StateRoot *tree.Root `yaml:"state_root"`
}
type BeaconStateV2Verification struct {
	Id     string                          `yaml:"id"`
	Fields BeaconStateV2VerificationFields `yaml:"fields"`
	Error  bool                            `yaml:"error"`
}

type BeaconStateV2VerificationField interface {
	Verify(t *hivesim.T, parentCtx context.Context, state *beacon_client.VersionedBeaconStateResponse)
}

func (v BeaconStateV2Verification) Check(t *hivesim.T, parentCtx context.Context, cl *beacon_client.BeaconClient) {
	t.Logf("verifying beacon state v2")
	var stateId eth2api.StateId
	switch v.Id {
	case "head":
		stateId = eth2api.StateHead
	case "genesis":
		stateId = eth2api.StateGenesis
	case "finalized":
		stateId = eth2api.StateFinalized
	case "justified":
		stateId = eth2api.StateJustified
	default:
		if strings.HasPrefix(v.Id, "root:") {
			root := tree.Root{}
			root.UnmarshalText([]byte(strings.TrimPrefix(v.Id, "root:")))
			stateId = eth2api.StateIdRoot(root)
		} else if strings.HasPrefix(v.Id, "slot:") {
			var slot view.Uint64View
			slot.UnmarshalText([]byte(strings.TrimPrefix(v.Id, "slot:")))
			stateId = eth2api.StateIdSlot(slot)
		} else {
			t.Fatalf("unable to parse state id: %s", v.Id)
		}
	}

	s, err := cl.BeaconStateV2(parentCtx, stateId)
	if err != nil {
		if v.Error {
			t.Logf("expected error: %v", err)
			return
		}
		t.Fatalf("failed to get state: %v", err)
	}

	if v.Fields.StateRoot != nil {
		if s.Root() != *v.Fields.StateRoot {
			t.Fatalf("state root mismatch: got %s, expected %s", s.Root(), v.Fields.StateRoot)
		}
		t.Logf("state root matches: %s", s.Root())

	}
}
