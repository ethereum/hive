package main

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"time"

	"github.com/ethereum/hive/hivesim"
	"github.com/ethereum/hive/simulators/eth2/testnet/mock"
	"github.com/ethereum/hive/simulators/eth2/testnet/setup"
)

func jsonStr(v interface{}) string {
	dat, err := json.MarshalIndent(v, "  ", "  ")
	if err != nil {
		panic(err)
	}
	return string(dat)
}

type testSpec struct {
	Name  string
	About string
	Run   func(*hivesim.T, *TestEnv)
}

var tests = []testSpec{
	{Name: "single-client-merge-testnet", Run: mergeTestnetTest},
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
			c := ClientsByRole(clientTypes)
			if len(c.Eth1) != 1 {
				t.Fatalf("choose 1 eth1 client type")
			}
			if len(c.Beacon) != 1 {
				t.Fatalf("choose 1 beacon client type")
			}
			if len(c.Validator) != 1 {
				t.Fatalf("choose 1 validator client type")
			}
			runAllTests(t, c)
		},
	})
	hivesim.MustRunSuite(hivesim.New(), suite)
}

func runAllTests(t *hivesim.T, c *ClientDefinitionsByRole) {
	mnemonic := "couple kiwi radio river setup fortune hunt grief buddy forward perfect empty slim wear bounce drift execute nation tobacco dutch chapter festival ice fog"

	// Generate validator keys to use for all tests.
	keySrc := &setup.MnemonicsKeySource{
		From:       0,
		To:         64,
		Validator:  mnemonic,
		Withdrawal: mnemonic,
	}
	keys, err := keySrc.Keys()
	if err != nil {
		t.Fatal(err)
	}
	secrets, err := setup.SecretKeys(keys)
	if err != nil {
		t.Fatal(err)
	}

	for _, test := range tests {
		test := test
		t.Run(hivesim.TestSpec{
			Name:        fmt.Sprintf("%s", test.Name),
			Description: test.About,
			Run: func(t *hivesim.T) {
				env := &TestEnv{c, keys, secrets}
				test.Run(t, env)
			},
		})
	}
}

func mergeTestnetTest(t *hivesim.T, env *TestEnv) {
	config := config{
		AltairForkEpoch: 0,
		MergeForkEpoch:  0,
		// ValidatorCount:          1<<14,
		ValidatorCount:          64,
		SlotTime:                2,
		TerminalTotalDifficulty: big.NewInt(150000000),
		TtdBlocksCount:          0,
		Nodes: []node{
			{
				ExecutionClient: "go-ethereum",
				ConsensusClient: "lighthouse",
			},
			{
				ExecutionClient: "go-ethereum",
				ConsensusClient: "lighthouse",
			},
		},
	}
	startTestnet(t, &config, env)
}

func startTestnet(t *hivesim.T, config *config, env *TestEnv) {
	prep := prepareTestnet(t, env, config)
	testnet := prep.createTestnet(t)

	genesisTime := testnet.GenesisTime()
	countdown := genesisTime.Sub(time.Now())
	t.Logf("created new testnet, genesis at %s (%s from now)", genesisTime, countdown)

	// for each key partition, we start a validator client with its own beacon node and eth1 node
	for i, node := range config.Nodes {
		prep.startEth1Node(testnet, env.Clients.ClientByNameAndRole(node.ExecutionClient, "eth1"), true)
		prep.startBeaconNode(testnet, env.Clients.ClientByNameAndRole(fmt.Sprintf("%s-bn", node.ConsensusClient), "beacon"), []int{i})
		prep.startValidatorClient(testnet, env.Clients.ClientByNameAndRole(fmt.Sprintf("%s-vc", node.ConsensusClient), "validator"), i, i)
	}
	t.Logf("started all nodes!")

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
