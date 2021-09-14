package main

import (
	"context"
	"github.com/ethereum/hive/hivesim"
	"time"
)

func main() {
	var suite = hivesim.Suite{
		Name:        "eth2-testnet",
		Description: `Run different eth2 testnets.`,
	}
	suite.Add(hivesim.TestSpec{
		Name:        "testnets",
		Description: "Collection of different testnet compositions and assertions.",
		Run: func(t *hivesim.T) {
			clientTypes, err := t.Sim.ClientTypes()
			if err != nil {
				t.Fatal(err)
			}
			byRole :=  ClientsByRole(clientTypes)
			simpleTest := byRole.SimpleTestnetTest()
			t.Run(simpleTest)
		},
	})
	hivesim.MustRunSuite(hivesim.New(), suite)
}


func (nc *ClientDefinitionsByRole) SimpleTestnetTest() hivesim.TestSpec {
	return hivesim.TestSpec{
		Name:        "single-client-testnet",
		Description: "This runs quick eth2 single-client testnets, beacon nodes matched with preferred validator type, and dummy eth1 endpoint.",
		Run: func(t *hivesim.T) {
			// 4096 validators, split between 4 nodes
			prep := prepareTestnet(t, 4096, 4)
			testnet := prep.createTestnet()

			genesisTime := testnet.GenesisTime()
			countdown := genesisTime.Sub(time.Now())
			t.Logf("created new testnet, genesis at %s (%s from now)", genesisTime, countdown)

			// TODO: we can mix things for a multi-client testnet
			if len(nc.eth1) != 1 {
				t.Fatalf("choose 1 eth1 client type")
			}
			if len(nc.beacon) != 1 {
				t.Fatalf("choose 1 beacon client type")
			}
			if len(nc.validator) != 1 {
				t.Fatalf("choose 1 validator client type")
			}

			// for each key partition, we start a validator client with its own beacon node and eth1 node
			for i := 0; i < len(prep.keyTranches); i++ {
				prep.startEth1Node(testnet, nc.eth1[0])
				prep.startBeaconNode(testnet, nc.beacon[0], []int{i})
				prep.startValidatorClient(testnet, nc.validator[0], i, i)
			}
			t.Logf("started all nodes!")

			ctx := context.Background()
			// TODO: maybe run other assertions / tests in the background?
			testnet.ExpectFinality(ctx)
		},
	}
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