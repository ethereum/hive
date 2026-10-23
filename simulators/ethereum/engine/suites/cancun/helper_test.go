package suite_cancun

import (
	"math"
	"testing"

	"github.com/ethereum/go-ethereum/common"
)

func TestBeaconRootStorageIndexes(t *testing.T) {
	// Expected keys use the 8191-entry history buffer specified by EIP-4788.
	tests := []struct {
		name         string
		timestamp    uint64
		timestampKey string
		rootKey      string
	}{
		{
			name:         "zero_timestamp",
			timestamp:    0,
			timestampKey: "0x0",
			rootKey:      "0x1fff",
		},
		{
			name:         "small_timestamp",
			timestamp:    0xa,
			timestampKey: "0xa",
			rootKey:      "0x2009",
		},
		{
			name:         "before_wraparound",
			timestamp:    8190,
			timestampKey: "0x1ffe",
			rootKey:      "0x3ffd",
		},
		{
			name:         "at_wraparound",
			timestamp:    8191,
			timestampKey: "0x0",
			rootKey:      "0x1fff",
		},
		{
			name:         "after_wraparound",
			timestamp:    8192,
			timestampKey: "0x1",
			rootKey:      "0x2000",
		},
		{
			name:         "max_timestamp",
			timestamp:    math.MaxUint64,
			timestampKey: "0xfff",
			rootKey:      "0x2ffe",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			expectedTimestampKey := common.HexToHash(tc.timestampKey)
			expectedRootKey := common.HexToHash(tc.rootKey)
			gotTimestampKey, gotRootKey := BeaconRootStorageIndexes(tc.timestamp)
			if gotTimestampKey != expectedTimestampKey {
				t.Error("expected timestamp key to be", expectedTimestampKey.Hex(), "got", gotTimestampKey.Hex())
			}
			if gotRootKey != expectedRootKey {
				t.Error("expected root key to be", expectedRootKey.Hex(), "got", gotRootKey.Hex())
			}
		})
	}
}
