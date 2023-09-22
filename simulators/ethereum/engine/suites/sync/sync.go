package suite_sync

import (
	"encoding/json"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/hive/hivesim"
	"github.com/ethereum/hive/simulators/ethereum/engine/helper"
)

// Specifies a single test variant for the given client:
// Contains the configuration for the main client and the configuration for
// the client that must perform the sync.
type SyncTestVariant struct {
	Name             string
	MainClientConfig hivesim.Params
	SyncClientConfig hivesim.Params
}

// The sync configurator takes input parameters and outputs all possible sync test variants under which
// a client must be tested for each sync test.
type SyncVariantGenerator interface {
	Configure(TTD *big.Int, GenesisFile string, ChainFile string) []SyncTestVariant
}

// Generates the default configuration for clients that have no special configuration
type DefaultSyncVariantGenerator struct{}

func (DefaultSyncVariantGenerator) Configure(*big.Int, string, string) []SyncTestVariant {
	return []SyncTestVariant{
		{
			Name:             "default",
			MainClientConfig: hivesim.Params{},
			SyncClientConfig: hivesim.Params{},
		},
	}
}

// Go-ethereum sync test variant generator
type GethSyncVariantGenerator struct{}

func (GethSyncVariantGenerator) Configure(*big.Int, string, string) []SyncTestVariant {
	return []SyncTestVariant{
		{
			Name: "Full",
			MainClientConfig: hivesim.Params{
				"HIVE_NODETYPE": "full",
			},
			SyncClientConfig: hivesim.Params{
				"HIVE_NODETYPE": "full",
			},
		},
		{
			Name: "Archive",
			MainClientConfig: hivesim.Params{
				"HIVE_NODETYPE": "archive",
			},
			SyncClientConfig: hivesim.Params{
				"HIVE_NODETYPE": "archive",
			},
		},
		{
			Name: "Snap",
			MainClientConfig: hivesim.Params{
				"HIVE_NODETYPE": "snap",
			},
			SyncClientConfig: hivesim.Params{
				"HIVE_NODETYPE": "snap",
			},
		},
	}
}

// Nethermind sync test variant generator
type NethermindSyncVariantGenerator struct{}

// Sync configuration for nethermind, which is marshaled into json string
// and passed as `HIVE_SYNC_CONFIG`
type NethermindSyncConfig struct {
	FastSync             bool         `json:"FastSync"`
	SnapSync             bool         `json:"SnapSync"`
	PivotNumber          *big.Int     `json:"PivotNumber"`
	PivotHash            *common.Hash `json:"PivotHash"`
	PivotTotalDifficulty string       `json:"PivotTotalDifficulty"`
	FastBlocks           bool         `json:"FastBlocks"`
}

func (c NethermindSyncConfig) String() string {
	b, err := json.Marshal(c)
	if err != nil {
		panic(err)
	}
	return fmt.Sprintf("%s", b)
}

func (NethermindSyncVariantGenerator) Configure(_ *big.Int, GenesisFile string, ChainFile string) []SyncTestVariant {
	result := make([]SyncTestVariant, 0)

	var (
		genesis = helper.LoadGenesis(GenesisFile)
		chain   types.Blocks
	)

	if ChainFile != "" {
		chain = helper.LoadChain("./chains/" + ChainFile)
	}

	// ArchiveSync
	archiveSyncConfig := NethermindSyncConfig{
		FastSync:   false,
		SnapSync:   false,
		FastBlocks: false,
	}
	result = append(result,
		SyncTestVariant{
			Name:             "archive sync",
			MainClientConfig: hivesim.Params{},
			SyncClientConfig: hivesim.Params{
				"HIVE_SYNC_CONFIG": archiveSyncConfig.String(),
			},
		})

	// FastSync, pivot == TTD
	pivot := chain[len(chain)-1]
	pivotHash := pivot.Hash()
	fastSyncConfigPivotOnTTD := NethermindSyncConfig{
		FastSync:             true,
		SnapSync:             false,
		FastBlocks:           true,
		PivotNumber:          pivot.Number(),
		PivotHash:            &pivotHash,
		PivotTotalDifficulty: helper.CalculateTotalDifficulty(genesis, chain, pivot.NumberU64()).String(),
	}
	result = append(result,
		SyncTestVariant{
			Name: "fast sync/pivot.Difficulty>=TTD",
			MainClientConfig: hivesim.Params{
				"HIVE_SYNC_CONFIG": NethermindSyncConfig{
					FastSync:   true,
					SnapSync:   false,
					FastBlocks: true,
				}.String(),
			},
			SyncClientConfig: hivesim.Params{
				"HIVE_SYNC_CONFIG": fastSyncConfigPivotOnTTD.String(),
			},
		})

	// FastSync, pivot < TTD
	pivot = chain[len(chain)-2]
	pivotHash = pivot.Hash()
	fastSyncConfigPivotLessThanTTD := NethermindSyncConfig{
		FastSync:             true,
		SnapSync:             false,
		FastBlocks:           true,
		PivotNumber:          pivot.Number(),
		PivotHash:            &pivotHash,
		PivotTotalDifficulty: helper.CalculateTotalDifficulty(genesis, chain, pivot.NumberU64()).String(),
	}
	result = append(result,
		SyncTestVariant{
			Name: "fast sync/pivot.Difficulty<TTD",
			MainClientConfig: hivesim.Params{
				"HIVE_SYNC_CONFIG": NethermindSyncConfig{
					FastSync:   true,
					SnapSync:   false,
					FastBlocks: true,
				}.String(),
			},
			SyncClientConfig: hivesim.Params{
				"HIVE_SYNC_CONFIG": fastSyncConfigPivotLessThanTTD.String(),
			},
		})
	return result
}

// Lists the types of sync supported by each client.
var ClientToSyncVariantGenerator = map[string]SyncVariantGenerator{
	"go-ethereum": GethSyncVariantGenerator{},
	"nethermind":  NethermindSyncVariantGenerator{},
}
