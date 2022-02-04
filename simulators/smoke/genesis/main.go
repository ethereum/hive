package main

import (
	"github.com/ethereum/hive/hivesim"
)

func main() {
	suite := hivesim.Suite{
		Name:        "genesis",
		Description: "This test suite checks client initialization with genesis blocks.",
	}
	suite.Add(hivesim.ClientTestSpec{
		Role:        "eth1",
		Name:        "empty genesis",
		Description: "This imports an empty genesis block with no environment variables.",
		Files: map[string]string{
			"/genesis.json": "genesis-empty.json",
		},
		Run: genesisTest{"0x433d0b859a77a29753d2a6df477c971dcc6300af33f9d64d821a1d490b4148b1"}.test,
	})
	suite.Add(hivesim.ClientTestSpec{
		Role:        "eth1",
		Name:        "all forks",
		Description: "This imports an empty genesis block and sets all fork block numbers.",
		Files: map[string]string{
			"/genesis.json": "genesis-empty.json",
		},
		Parameters: map[string]string{
			"HIVE_CHAIN_ID":            "10",
			"HIVE_FORK_HOMESTEAD":      "11",
			"HIVE_FORK_DAO_BLOCK":      "12",
			"HIVE_FORK_TANGERINE":      "13",
			"HIVE_FORK_SPURIOUS":       "14",
			"HIVE_FORK_BYZANTIUM":      "15",
			"HIVE_FORK_CONSTANTINOPLE": "16",
			"HIVE_FORK_PETERSBURG":     "17",
			"HIVE_FORK_ISTANBUL":       "18",
			"HIVE_FORK_MUIR_GLACIER":   "19",
			"HIVE_FORK_BERLIN":         "20",
			"HIVE_FORK_LONDON":         "21",
		},
		Run: genesisTest{"0x433d0b859a77a29753d2a6df477c971dcc6300af33f9d64d821a1d490b4148b1"}.test,
	})
	suite.Add(hivesim.ClientTestSpec{
		Role:        "eth1",
		Name:        "non-empty",
		Description: "This imports a non-empty genesis block.",
		Files: map[string]string{
			"/genesis.json": "genesis-nonempty.json",
		},
		Parameters: map[string]string{
			"HIVE_CHAIN_ID":            "10",
			"HIVE_FORK_HOMESTEAD":      "0",
			"HIVE_FORK_TANGERINE":      "0",
			"HIVE_FORK_SPURIOUS":       "0",
			"HIVE_FORK_BYZANTIUM":      "0",
			"HIVE_FORK_CONSTANTINOPLE": "0",
			"HIVE_FORK_PETERSBURG":     "0",
			"HIVE_FORK_ISTANBUL":       "0",
		},
		Run: genesisTest{"0x5ae31c6522bd5856129f66be3d582b842e4e9faaa87f21cce547128339a9db3c"}.test,
	})
	suite.Add(hivesim.ClientTestSpec{
		Name:        "precomp-storage",
		Description: "This imports a genesis where a precompile has code/nonce/storage.",
		Files: map[string]string{
			"/genesis.json": "genesis-precomp-storage.json",
		},
		Parameters: map[string]string{
			"HIVE_CHAIN_ID":            "10",
			"HIVE_FORK_HOMESTEAD":      "0",
			"HIVE_FORK_TANGERINE":      "0",
			"HIVE_FORK_SPURIOUS":       "0",
			"HIVE_FORK_BYZANTIUM":      "0",
			"HIVE_FORK_CONSTANTINOPLE": "0",
			"HIVE_FORK_PETERSBURG":     "0",
			"HIVE_FORK_ISTANBUL":       "0",
		},
		Run: genesisTest{"0x1b5dc4bd86f9209e6261d43dd3085034d3a502c3823903a417a95320caccaebf"}.test,
	})
	suite.Add(hivesim.ClientTestSpec{
		Name:        "precomp-empty",
		Description: "This imports a genesis where a precompile is an empty account.",
		Files: map[string]string{
			"/genesis.json": "genesis-precomp-empty.json",
		},
		Parameters: map[string]string{
			"HIVE_CHAIN_ID":            "10",
			"HIVE_FORK_HOMESTEAD":      "0",
			"HIVE_FORK_TANGERINE":      "0",
			"HIVE_FORK_SPURIOUS":       "0",
			"HIVE_FORK_BYZANTIUM":      "0",
			"HIVE_FORK_CONSTANTINOPLE": "0",
			"HIVE_FORK_PETERSBURG":     "0",
			"HIVE_FORK_ISTANBUL":       "0",
			"HIVE_FORK_BERLIN":         "0",
		},
		Run: genesisTest{"0xf6df6eb772235f4f193b1a514f34bc5e4e9ce747a83732d4e8fced78ba2e939c"}.test,
	})
	suite.Add(hivesim.ClientTestSpec{
		Role:        "eth1",
		Name:        "precomp-zero-balance",
		Description: "This imports a genesis where a precompile has code/nonce/storage, but balance is zero.",
		Files: map[string]string{
			"/genesis.json": "genesis-precomp-zero-balance.json",
		},
		Parameters: map[string]string{
			"HIVE_CHAIN_ID":            "10",
			"HIVE_FORK_HOMESTEAD":      "0",
			"HIVE_FORK_TANGERINE":      "0",
			"HIVE_FORK_SPURIOUS":       "0",
			"HIVE_FORK_BYZANTIUM":      "0",
			"HIVE_FORK_CONSTANTINOPLE": "0",
			"HIVE_FORK_PETERSBURG":     "0",
			"HIVE_FORK_ISTANBUL":       "0",
			"HIVE_FORK_BERLIN":         "0",
		},
		Run: genesisTest{"0x251e6c3c84531a8b59a62535db474d0b5967f6f4ec82e47d62e70497ccdde6a7"}.test,
	})

	hivesim.MustRunSuite(hivesim.New(), suite)
}

type block struct {
	Hash string `json:"hash"`
}

type genesisTest struct {
	wantHash string
}

func (g genesisTest) test(t *hivesim.T, c *hivesim.Client) {
	var b block
	if err := c.RPC().Call(&b, "eth_getBlockByNumber", "0x0", false); err != nil {
		t.Fatal("eth_getBlockByNumber call failed:", err)
	}
	t.Log("genesis hash", b.Hash)
	if b.Hash != g.wantHash {
		t.Fatal("wrong genesis hash, want", g.wantHash)
	}
}
