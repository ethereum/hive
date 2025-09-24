package main

import (
	"strings"
	"testing"
)

func TestCompareKeysOnly(t *testing.T) {
	tests := []struct {
		name        string
		actual      string
		expected    string
		shouldError bool
		errorMsg    string
	}{
		{
			name:        "matching keys with different values",
			actual:      `{"id":1,"result":"0xabc123","jsonrpc":"2.0"}`,
			expected:    `{"id":2,"result":"0xdef456","jsonrpc":"1.0"}`,
			shouldError: false,
		},
		{
			name:        "missing key in actual",
			actual:      `{"id":1,"jsonrpc":"2.0"}`,
			expected:    `{"id":1,"result":"0x123","jsonrpc":"2.0"}`,
			shouldError: true,
			errorMsg:    "missing key",
		},
		{
			name:        "extra key in actual is not allowed",
			actual:      `{"id":1,"result":"0x123","jsonrpc":"2.0","extra":"field"}`,
			expected:    `{"id":1,"result":"0x456","jsonrpc":"2.0"}`,
			shouldError: true,
			errorMsg:    "unexpected key",
		},
		{
			name:        "nested objects - matching structure",
			actual:      `{"id":1,"result":{"block":"0x1","hash":"0xabc"},"jsonrpc":"2.0"}`,
			expected:    `{"id":1,"result":{"block":"0x2","hash":"0xdef"},"jsonrpc":"2.0"}`,
			shouldError: false,
		},
		{
			name:        "nested objects - missing nested key",
			actual:      `{"id":1,"result":{"block":"0x1"},"jsonrpc":"2.0"}`,
			expected:    `{"id":1,"result":{"block":"0x2","hash":"0xdef"},"jsonrpc":"2.0"}`,
			shouldError: true,
			errorMsg:    "missing key",
		},
		{
			name:        "nested objects - extra nested key",
			actual:      `{"id":1,"result":{"block":"0x1","hash":"0xabc","extra":"key"},"jsonrpc":"2.0"}`,
			expected:    `{"id":1,"result":{"block":"0x2","hash":"0xdef"},"jsonrpc":"2.0"}`,
			shouldError: true,
			errorMsg:    "unexpected key",
		},
		{
			name:        "arrays - only check structure exists",
			actual:      `{"id":1,"result":[1,2,3],"jsonrpc":"2.0"}`,
			expected:    `{"id":1,"result":[4,5,6,7,8],"jsonrpc":"2.0"}`,
			shouldError: false,
		},
		{
			name:        "null when string expected - type mismatch",
			actual:      `{"id":1,"result":null,"jsonrpc":"2.0"}`,
			expected:    `{"id":1,"result":"0x123","jsonrpc":"2.0"}`,
			shouldError: true,
			errorMsg:    "type mismatch",
		},
		{
			name:        "null when null expected - ok",
			actual:      `{"id":1,"result":null,"jsonrpc":"2.0"}`,
			expected:    `{"id":1,"result":null,"jsonrpc":"2.0"}`,
			shouldError: false,
		},
		{
			name:        "string when number expected - type mismatch",
			actual:      `{"id":"1","result":"0x123","jsonrpc":"2.0"}`,
			expected:    `{"id":1,"result":"0x456","jsonrpc":"2.0"}`,
			shouldError: true,
			errorMsg:    "type mismatch",
		},
		{
			name:        "object when array expected - type mismatch",
			actual:      `{"id":1,"result":{"key":"value"},"jsonrpc":"2.0"}`,
			expected:    `{"id":1,"result":[1,2,3],"jsonrpc":"2.0"}`,
			shouldError: true,
			errorMsg:    "expected array",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := compareKeysOnly(tt.actual, tt.expected)
			if tt.shouldError {
				if err == nil {
					t.Errorf("expected error but got none")
				} else if !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("expected error containing %q, got %q", tt.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

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