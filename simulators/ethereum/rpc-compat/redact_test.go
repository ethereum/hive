package main

import (
	"testing"
)

func TestRedactErrorMessages(t *testing.T) {
	tests := []struct {
		name         string
		resp         string
		expected     string
		wantResp     string
		wantExpected string
		wantRedacted bool
	}{
		{
			name:         "top-level error message is redacted",
			resp:         `{"jsonrpc":"2.0","id":1,"error":{"code":-32000,"message":"client-specific error"}}`,
			expected:     `{"jsonrpc":"2.0","id":1,"error":{"code":-32000,"message":"expected error message"}}`,
			wantResp:     `{"jsonrpc":"2.0","id":1,"error":{"code":-32000}}`,
			wantExpected: `{"jsonrpc":"2.0","id":1,"error":{"code":-32000}}`,
			wantRedacted: true,
		},
		{
			name:         "nested error message is redacted (eth_simulateV1 style)",
			resp:         `{"result":{"calls":[{"error":{"code":-32000,"message":"client error"}}]}}`,
			expected:     `{"result":{"calls":[{"error":{"code":-32000,"message":"spec error"}}]}}`,
			wantResp:     `{"result":{"calls":[{"error":{"code":-32000}}]}}`,
			wantExpected: `{"result":{"calls":[{"error":{"code":-32000}}]}}`,
			wantRedacted: true,
		},
		{
			name:         "no redaction when expected has no error message",
			resp:         `{"jsonrpc":"2.0","id":1,"error":{"code":-32000,"message":"client error"}}`,
			expected:     `{"jsonrpc":"2.0","id":1,"error":{"code":-32000}}`,
			wantResp:     `{"jsonrpc":"2.0","id":1,"error":{"code":-32000,"message":"client error"}}`,
			wantExpected: `{"jsonrpc":"2.0","id":1,"error":{"code":-32000}}`,
			wantRedacted: false,
		},
		{
			name:         "no redaction when resp has no error message",
			resp:         `{"jsonrpc":"2.0","id":1,"error":{"code":-32000}}`,
			expected:     `{"jsonrpc":"2.0","id":1,"error":{"code":-32000,"message":"spec error"}}`,
			wantResp:     `{"jsonrpc":"2.0","id":1,"error":{"code":-32000}}`,
			wantExpected: `{"jsonrpc":"2.0","id":1,"error":{"code":-32000,"message":"spec error"}}`,
			wantRedacted: false,
		},
		{
			name:         "no error object, no redaction",
			resp:         `{"jsonrpc":"2.0","id":1,"result":"0x1"}`,
			expected:     `{"jsonrpc":"2.0","id":1,"result":"0x1"}`,
			wantResp:     `{"jsonrpc":"2.0","id":1,"result":"0x1"}`,
			wantExpected: `{"jsonrpc":"2.0","id":1,"result":"0x1"}`,
			wantRedacted: false,
		},
		{
			name:         "multiple nested errors all redacted",
			resp:         `{"result":{"calls":[{"error":{"code":-1,"message":"err1"}},{"error":{"code":-2,"message":"err2"}}]}}`,
			expected:     `{"result":{"calls":[{"error":{"code":-1,"message":"exp1"}},{"error":{"code":-2,"message":"exp2"}}]}}`,
			wantResp:     `{"result":{"calls":[{"error":{"code":-1}},{"error":{"code":-2}}]}}`,
			wantExpected: `{"result":{"calls":[{"error":{"code":-1}},{"error":{"code":-2}}]}}`,
			wantRedacted: true,
		},
		{
			name:         "deeply nested error through non-error objects",
			resp:         `{"result":{"a":{"b":{"error":{"code":1,"message":"deep client"}}}}}`,
			expected:     `{"result":{"a":{"b":{"error":{"code":1,"message":"deep spec"}}}}}`,
			wantResp:     `{"result":{"a":{"b":{"error":{"code":1}}}}}`,
			wantExpected: `{"result":{"a":{"b":{"error":{"code":1}}}}}`,
			wantRedacted: true,
		},
		{
			name:         "error sibling does not block recursion into other keys",
			resp:         `{"error":{"code":1,"message":"top"},"result":{"calls":[{"error":{"code":2,"message":"inner client"}}]}}`,
			expected:     `{"error":{"code":1,"message":"top spec"},"result":{"calls":[{"error":{"code":2,"message":"inner spec"}}]}}`,
			wantResp:     `{"error":{"code":1},"result":{"calls":[{"error":{"code":2}}]}}`,
			wantExpected: `{"error":{"code":1},"result":{"calls":[{"error":{"code":2}}]}}`,
			wantRedacted: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotResp, gotExpected, gotRedacted := redactErrorMessages(tt.resp, tt.expected)
			if gotRedacted != tt.wantRedacted {
				t.Errorf("redacted = %v, want %v", gotRedacted, tt.wantRedacted)
			}
			if gotResp != tt.wantResp {
				t.Errorf("resp =\n  %s\nwant\n  %s", gotResp, tt.wantResp)
			}
			if gotExpected != tt.wantExpected {
				t.Errorf("expected =\n  %s\nwant\n  %s", gotExpected, tt.wantExpected)
			}
		})
	}
}
