package main

import "testing"

// TestCapabilitiesSchemaValidation verifies that eth_capabilities responses
// with differing optional fields all validate against the OpenRPC result
// schema. eth_capabilities retention fields (oldestBlock, deleteStrategy) are
// client- and config-specific: e.g. an archive/hash geth omits deleteStrategy
// while a full/path geth includes it, and a client may disable a resource
// entirely. A speconly test must accept any spec-valid shape, which exact- or
// structural-matching against a single recorded example cannot do.
func TestCapabilitiesSchemaValidation(t *testing.T) {
	schemas, err := parseSpec("testdata/openrpc-capabilities.json")
	if err != nil {
		t.Fatalf("parseSpec: %v", err)
	}
	schema := schemas["eth_capabilities"]
	if schema == nil {
		t.Fatal("no schema for eth_capabilities")
	}

	head := `"head":{"number":"0x2d","hash":"0xe27a3e81bd7cfe2aec2cc9e832c73a17c93e7efcf659cf4b39883b96c48708c2"}`
	valid := map[string]string{
		// archive/hash geth: no deleteStrategy.
		"archive-mode": `{` + head + `,"state":{"disabled":false,"oldestBlock":"0x0"},"tx":{"disabled":false,"oldestBlock":"0x0"},"logs":{"disabled":false,"oldestBlock":"0x0"},"receipts":{"disabled":false,"oldestBlock":"0x0"},"blocks":{"disabled":false,"oldestBlock":"0x0"},"stateproofs":{"disabled":false,"oldestBlock":"0x0"}}`,
		// full/path geth: adds deleteStrategy on state/tx/stateproofs.
		"full-mode": `{` + head + `,"state":{"disabled":false,"oldestBlock":"0x0","deleteStrategy":{"type":"window","retentionBlocks":"0x80"}},"tx":{"disabled":false,"oldestBlock":"0x0","deleteStrategy":{"type":"window","retentionBlocks":"0x23dbb0"}},"logs":{"disabled":false,"oldestBlock":"0x0","deleteStrategy":{"type":"window","retentionBlocks":"0x23dbb0"}},"receipts":{"disabled":false,"oldestBlock":"0x0"},"blocks":{"disabled":false,"oldestBlock":"0x0"},"stateproofs":{"disabled":false,"oldestBlock":"0x0","deleteStrategy":{"type":"window","retentionBlocks":"0x80"}}}`,
		// a client with a resource entirely disabled (e.g. no state proofs).
		"disabled-resource": `{` + head + `,"state":{"disabled":false,"oldestBlock":"0x0"},"tx":{"disabled":false},"logs":{"disabled":false},"receipts":{"disabled":false},"blocks":{"disabled":false},"stateproofs":{"disabled":true}}`,
	}
	for name, resp := range valid {
		if err := validateResult(schema, []byte(resp)); err != nil {
			t.Errorf("%s: expected valid, got error: %v", name, err)
		}
	}

	// Sanity check: an actually-invalid response (unknown key) must be rejected.
	bad := `{` + head + `,"state":{"disabled":false,"bogus":true},"tx":{"disabled":false},"logs":{"disabled":false},"receipts":{"disabled":false},"blocks":{"disabled":false},"stateproofs":{"disabled":false}}`
	if err := validateResult(schema, []byte(bad)); err == nil {
		t.Error("expected invalid response (unknown key) to be rejected, got nil")
	}
}
