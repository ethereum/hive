package main

import (
	"testing"

	"github.com/protolambda/ztyp/tree"
	"gopkg.in/yaml.v3"
)

// Test verifications yaml file unmarshaller
func TestBeaconAPIVerifications_UnmarshalYAML(t *testing.T) {
	yamlString := `- method: BeaconStateV2
  id: head
  fields:
    state_root: a100000000000000000000000000000000000000000000000000000000000000
`

	var verifications BeaconAPIVerifications
	if err := yaml.Unmarshal([]byte(yamlString), &verifications); err != nil {
		t.Fatal(err)
	}
	if len(verifications.Verifications) != 1 {
		t.Fatal("expected 1 verification")
	}
	// Verify type is correct
	if v, ok := verifications.Verifications[0].(*BeaconStateV2Verification); !ok {
		t.Fatal("expected BeaconStateV2Verification")
	} else {
		if *v.Fields.StateRoot != (tree.Root{0xa1}) {
			t.Fatal("expected state root to be 0x01")
		}
	}

}
