package main

import (
	"testing"

	"github.com/ethereum/hive/internal/libhive"
)

func TestSuiteDevnet(t *testing.T) {
	tests := []struct {
		name    string
		suite   *libhive.TestSuite
		clients []string
		want    string
	}{
		{
			name: "client config nametag",
			suite: &libhive.TestSuite{
				RunMetadata: &libhive.RunMetadata{
					ClientConfig: &libhive.ClientConfigInfo{
						Content: map[string]any{
							"clients": []any{
								map[string]any{"client": "ream", "nametag": "devnet4"},
							},
						},
					},
				},
			},
			want: "devnet4",
		},
		{
			name: "client config file path",
			suite: &libhive.TestSuite{
				RunMetadata: &libhive.RunMetadata{
					ClientConfig: &libhive.ClientConfigInfo{
						FilePath: "simulators/lean/clients/devnet5.yaml",
					},
				},
			},
			want: "devnet5",
		},
		{
			name:    "client name",
			suite:   &libhive.TestSuite{},
			clients: []string{"ream_devnet3"},
			want:    "devnet3",
		},
		{
			name: "suite description",
			suite: &libhive.TestSuite{
				Description: "Runs Lean tests using the devnet6 profile.",
			},
			want: "devnet6",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := suiteDevnet(test.suite, test.clients); got != test.want {
				t.Fatalf("devnet mismatch: got %q, want %q", got, test.want)
			}
		})
	}
}
