package main

import (
	"fmt"

	"github.com/ethereum/hive/hivesim"
	"github.com/ethereum/hive/simulators/eth2/testnet/setup"
)

var tests = []testSpec{
	// {Name: "single-client-testnet", Run: Phase0Testnet},
	{Name: "transition-testnet", Run: TransitionTestnet},
}

func main() {
	var suite = hivesim.Suite{
		Name:        "eth2-testnet",
		Description: `Run different eth2 testnets.`,
	}
	suite.Add(hivesim.TestSpec{
		Name:        "eth2-testnets",
		Description: "Collection of different eth2 testnet compositions and assertions.",
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
	for _, node := range c.Combinations() {
		for _, test := range tests {
			test := test
			t.Run(hivesim.TestSpec{
				Name:        fmt.Sprintf("%s-%s", test.Name, node),
				Description: test.About,
				Run: func(t *hivesim.T) {
					env := &testEnv{c, keys, secrets}
					test.Run(t, env, node)
				},
			})
		}
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
