package main

import (
	"time"

	"github.com/ethereum/hive/hivesim"
)

func main() {
	suite := hivesim.Suite{
		Name:        "clique",
		Description: "This test suite tests clique mining support.",
	}
	suite.Add(hivesim.ClientTestSpec{
		Name:        "mine one block",
		Description: "Waits for a single block to get mined with clique.",
		Files: map[string]string{
			"/genesis.json": "genesis.json",
		},
		Parameters: hivesim.Params{
			"HIVE_CLIQUE_PERIOD":     "2",
			"HIVE_CLIQUE_PRIVATEKEY": "9c647b8b7c4e7c3490668fb6c11473619db80c93704c70893d3813af4090c39c",
			"HIVE_MINER":             "658bdf435d810c91414ec09147daa6db62406379",
			// These are required for clique.
			"HIVE_CHAIN_ID":            "1334",
			"HIVE_FORK_HOMESTEAD":      "0",
			"HIVE_FORK_TANGERINE":      "0",
			"HIVE_FORK_SPURIOUS":       "0",
			"HIVE_FORK_BYZANTIUM":      "0",
			"HIVE_FORK_CONSTANTINOPLE": "0",
			"HIVE_FORK_PETERSBURG":     "0",
		},
		Run: miningTest,
	})
	hivesim.MustRunSuite(hivesim.New(), suite)
}

type block struct {
	Number     string `json:"number"`
	Hash       string `json:"hash"`
	ParentHash string `json:"parentHash"`
}

func miningTest(t *hivesim.T, c *hivesim.Client) {
	start := time.Now()
	timeout := 10 * time.Second

	for {
		var b block
		if err := c.RPC().Call(&b, "eth_getBlockByNumber", "latest", false); err != nil {
			t.Fatal("eth_getBlockByNumber call failed:", err)
		}
		switch b.Number {
		case "0x0":
			// still at genesis block, keep waiting.
			if time.Since(start) > timeout {
				t.Fatal("no block produced within", timeout)
			}
			time.Sleep(300 * time.Millisecond)
		case "0x1":
			t.Log("block mined:", b.Hash)
			return
		default:
			t.Fatal("wrong latest block: number", b.Number, ", hash", b.Hash, ", parent", b.ParentHash)
		}
	}
}
