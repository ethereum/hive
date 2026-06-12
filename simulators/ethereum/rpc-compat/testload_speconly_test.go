package main

import (
	"strings"
	"testing"
)

func TestSpecOnlyParsing(t *testing.T) {
	testContent := `// This is a test
// speconly: true
>> {"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}
<< {"jsonrpc":"2.0","id":1,"result":"0x1"}
`

	test, err := loadTestFile("test", strings.NewReader(testContent))
	if err != nil {
		t.Fatalf("failed to load test: %v", err)
	}

	if !test.speconly {
		t.Errorf("expected speconly to be true")
	}

	if test.name != "test" {
		t.Errorf("expected name to be 'test', got %q", test.name)
	}

	if len(test.messages) != 2 {
		t.Errorf("expected 2 messages, got %d", len(test.messages))
	}
}
