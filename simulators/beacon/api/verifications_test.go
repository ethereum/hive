package main

import (
	"testing"

	"github.com/protolambda/ztyp/tree"
	"gopkg.in/yaml.v3"
)

// Test verifications yaml file unmarshaller
func TestBeaconAPITestSteps_UnmarshalYAML(t *testing.T) {
	yamlString := `- !EthV2DebugBeaconStates
  id: head
  fields: {state_root: a100000000000000000000000000000000000000000000000000000000000000}
`

	var verifications BeaconAPITestSteps
	if err := yaml.Unmarshal([]byte(yamlString), &verifications); err != nil {
		t.Fatal(err)
	}
	if len(verifications.Verifications) != 1 {
		t.Fatal("expected 1 verification")
	}
	// Verify type is correct
	if v, ok := verifications.Verifications[0].(*EthV2DebugBeaconStates); !ok {
		t.Fatal("expected EthV2DebugBeaconStates")
	} else {
		if *v.Fields.StateRoot != (tree.Root{0xa1}) {
			t.Fatal("expected state root to be 0xa1")
		}
	}

}
