package main

import (
	"fmt"
	"os"
	"time"

	"github.com/ethereum/hive/hivesim"
	"github.com/ethereum/hive/internal/simapi"
)

var (
	validators = []map[string]string{
		{
			"privateKey": "e24d25ccef5e9d01071c1216c70393f830007090a9c40779cb23cad908d95404",
			"address":    "af96c12e5e2d69c5f721fe1430b6d22a07748be6",
		},
		{
			"privateKey": "2d4b1e6ed6e469037f5c41abfdb07e275c1cc4c0797a169a2a46dacc8750a2b4",
			"address":    "da0a817cd6ac3521ee669637fffb7c6293014840",
		},
	}
	params = hivesim.Params{
		"HIVE_FORK_CONSORTIUMV2":      "1000000000",
		"HIVE_RONIN_VALIDATOR_SET":    "0x0000000000000000000000000000000000000123",
		"HIVE_RONIN_SLASH_INDICATOR":  "0x0000000000000000000000000000000000000456",
		"HIVE_RONIN_STAKING_CONTRACT": "0x0000000000000000000000000000000000000789",
		"HIVE_CONSORTIUM_PERIOD":      "3",
		"HIVE_CONSORTIUM_EPOCH":       "30",
		"HIVE_RONIN_PRIVATEKEY":       "",
		"HIVE_MINER":                  "",
		"HIVE_CHAIN_ID":               "1334",
		"HIVE_FORK_HOMESTEAD":         "0",
		"HIVE_FORK_TANGERINE":         "0",
		"HIVE_FORK_SPURIOUS":          "0",
		"HIVE_FORK_BYZANTIUM":         "0",
		"HIVE_FORK_CONSTANTINOPLE":    "0",
		"HIVE_FORK_PETERSBURG":        "0",
		"HIVE_VALIDATOR_1_SLOT_VALUE": "0x000000000000000000000000af96c12e5e2d69c5f721fe1430b6d22a07748be6",
		"HIVE_VALIDATOR_2_SLOT_VALUE": "0x000000000000000000000000da0a817cd6ac3521ee669637fffb7c6293014840",
		"HIVE_MAIN_ACCOUNT":           "0xaf96c12e5e2d69c5f721fe1430b6d22a07748be6",
	}
)

func main() {
	suite := hivesim.Suite{
		Name:        "ronin",
		Description: "This test suite tests ronin mining support.",
	}
	suite.Add(hivesim.TestSpec{
		Name: "test file loader",
		Description: "This is a meta-test. It loads the blockchain test files and " +
			"launches the actual client tests. Any errors in test files will be reported " +
			"through this test.",
		Run:       loaderTest,
		AlwaysRun: true,
	})
	hivesim.MustRunSuite(hivesim.New(), suite)
}

type block struct {
	Number     string `json:"number"`
	Hash       string `json:"hash"`
	ParentHash string `json:"parentHash"`
}

func loaderTest(t *hivesim.T) {

	mountSource, ok := os.LookupEnv("HIVE_MOUNT_SOURCE")
	if !ok {
		t.Fatal("mount source is undefined")
	}

	params1 := params.Copy()
	params1["HIVE_RONIN_PRIVATEKEY"] = validators[0]["privateKey"]
	params1["HIVE_MINER"] = validators[0]["address"]

	params2 := params.Copy()
	params2["HIVE_RONIN_PRIVATEKEY"] = validators[1]["privateKey"]
	params2["HIVE_MINER"] = validators[1]["address"]

	// start 2 client with 2 above params
	client1 := t.StartClient("ronin", params1, hivesim.WithMounts(simapi.Mount{
		RW:          true,
		Source:      fmt.Sprintf("%s/node1", mountSource),
		Destination: "/root/.ethereum/ronin",
	}))

	enode, err := client1.EnodeURLNetwork("bridge")
	if err != nil {
		t.Error(err)
	}
	params2["HIVE_BOOTNODE"] = enode

	// start client2
	client2 := t.StartClient("ronin", params2, hivesim.WithMounts(simapi.Mount{
		RW:          true,
		Source:      fmt.Sprintf("%s/node2", mountSource),
		Destination: "/root/.ethereum/ronin",
	}))

	period := time.NewTicker(3 * time.Second)
	timeout := time.NewTicker(30 * time.Second)
	for {
		select {
		case <-period.C:
			t.Log("start getting latest block")
			var b1, b2 block
			if err = client1.RPC().Call(&b1, "eth_getBlockByNumber", "latest", false); err != nil {
				t.Fatal("eth_getBlockByNumber call failed:", err)
			}
			if err = client2.RPC().Call(&b2, "eth_getBlockByNumber", "latest", false); err != nil {
				t.Fatal("eth_getBlockByNumber call failed:", err)
			}
			t.Log("block1", b1.Number, "block2", b2.Number)
		case <-timeout.C:
			return
		}
	}
}
