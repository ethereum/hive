package main

import (
	"context"
	"encoding/json"
	"math/big"
	"time"

	"github.com/ethereum/hive/hivesim"
	"github.com/ethereum/hive/simulators/eth2/testnet/mock"
)

func jsonStr(v interface{}) string {
	dat, err := json.MarshalIndent(v, "  ", "  ")
	if err != nil {
		panic(err)
	}
	return string(dat)
}

type Config struct {
	AltairForkEpoch         uint64
	MergeForkEpoch          uint64
	ValidatorCount          uint64
	KeyTranches             uint64
	SlotTime                uint64
	TotalTerminalDifficulty *big.Int
}

func main() {
	var suite = hivesim.Suite{
		Name:        "eth2-testnet",
		Description: `Run different eth2 testnets.`,
	}
	suite.Add(hivesim.TestSpec{
		Name:        "merge-testnets",
		Description: "Collection of different merge testnet compositions and assertions.",
		Run: func(t *hivesim.T) {
			clientTypes, err := t.Sim.ClientTypes()
			if err != nil {
				t.Fatal(err)
			}
			t.Log("clients by role:", jsonStr(clientTypes))
			byRole := ClientsByRole(clientTypes)
			t.Log("clients by role:", jsonStr(byRole))
			mergeTest := byRole.MergeTestnetTest()
			t.Run(mergeTest)
		},
	})
	hivesim.MustRunSuite(hivesim.New(), suite)
}

func (nc *ClientDefinitionsByRole) MergeTestnetTest() hivesim.TestSpec {
	return hivesim.TestSpec{
		Name:        "single-client-merge-testnet",
		Description: "This runs quick merge single-client testnet, with 4 nodes and 2**14 (minimum) validators",
		Run: func(t *hivesim.T) {
			config := Config{
				AltairForkEpoch: 0,
				MergeForkEpoch:  0,
				// ValidatorCount:          1<<14,
				ValidatorCount:          64,
				SlotTime:                1,
				TotalTerminalDifficulty: big.NewInt(824242424),
			}
			nc.startTestnet(t, &config)
		},
	}
}

func (nc *ClientDefinitionsByRole) startTestnet(t *hivesim.T, config *Config) {
	prep := prepareTestnet(t, config)
	testnet := prep.createTestnet(t)

	genesisTime := testnet.GenesisTime()
	countdown := genesisTime.Sub(time.Now())
	t.Logf("created new testnet, genesis at %s (%s from now)", genesisTime, countdown)

	if len(nc.Eth1) != 1 {
		t.Fatalf("choose 1 eth1 client type")
	}
	if len(nc.Beacon) != 1 {
		t.Fatalf("choose 1 beacon client type")
	}
	if len(nc.Validator) != 1 {
		t.Fatalf("choose 1 validator client type")
	}

	// for each key partition, we start a validator client with its own beacon node and eth1 node
	for i := 0; i < len(prep.keyTranches); i++ {
		prep.startEth1Node(testnet, nc.Eth1[0], true)
		prep.startBeaconNode(testnet, nc.Beacon[0], []int{i})
		prep.startValidatorClient(testnet, nc.Validator[0], i, i)
	}
	t.Logf("started all nodes!")

	t.Logf("starting eth1 miner")
	addr, err := testnet.eth1[0].EnodeURL()
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	go mock.MineChain(t, ctx, prep.eth1Genesis.Genesis, addr)

	// TODO: maybe run other assertions / tests in the background?
	testnet.TrackFinality(ctx)
}

/*
	TODO More testnet ideas:

	Name:        "two-client-testnet",
	Description: "This runs quick eth2 testnets with combinations of 2 client types, beacon nodes matched with preferred validator type, and dummy eth1 endpoint.",
	Name:        "all-client-testnet",
	Description: "This runs a quick eth2 testnet with all client types, beacon nodes matched with preferred validator type, and dummy eth1 endpoint.",
	Name:        "cross-single-client-testnet",
	Description: "This runs a quick eth2 single-client testnet, but beacon nodes are matched with all validator types",
*/
