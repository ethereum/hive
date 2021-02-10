package main

import (
	"github.com/ethereum/hive/hivesim"
	"github.com/ethereum/hive/simulators/eth2/testnet/setup"
	"github.com/protolambda/zrnt/eth2/beacon"
	"github.com/protolambda/zrnt/eth2/configs"
	"time"
)

func main() {
	var suite = hivesim.Suite{
		Name: "testnet",
		Description: `This suite of tests starts a small eth2 testnet until it finalizes a 2 epochs.`,
	}
	suite.Add(hivesim.TestSpec{
		Name:        "Testnet runner",
		Description: "This runs a small quick testnet with a set of test assertions.",
		Run:         runTests,
	})
	hivesim.MustRunSuite(hivesim.New(), suite)
}

func runTests(t *hivesim.T) {
	// TODO: copy and modify with env vars
	spec := configs.Mainnet
	valCount := uint64(4096)

	clientTypes, err := t.Sim.ClientTypes()
	if err != nil {
		t.Fatal(err)
	}

	var beaconNodes, validatorClients, eth1nodes, other []string
	for _, client := range clientTypes {
		switch client.Meta.Role {
		case "beacon":
			beaconNodes = append(beaconNodes, client.Name)
		case "validator":
			validatorClients = append(validatorClients, client.Name)
		case "eth1":
			eth1nodes = append(eth1nodes, client.Name)
		default:
			other = append(other, client.Name)
		}
	}

	// TODO: build different combinations of above clients, and run tiny testnets


	// TODO: override system time of nodes to make genesis nice and predictable, or keep it dynamic?
	// (useful for caching docker containers too?)
	now := time.Now()
	// TODO: tweak this. 2 min genesis time?
	genesisTime := beacon.Timestamp((now.Add(2 * time.Minute)).Unix())

	eth2Config := hivesim.WithParams(map[string]string{
		"HIVE_ETH2_FOOBAR": "1234",
	})

	mnemonic := "couple kiwi radio river setup fortune hunt grief buddy forward perfect empty slim wear bounce drift execute nation tobacco dutch chapter festival ice fog"
	keySrc := &setup.MnemonicsKeySource{
		From:       0,
		To:         valCount,
		Validator:  mnemonic,
		Withdrawal: mnemonic,
	}
	keys, err := keySrc.Keys()
	if err != nil {
		t.Fatal(err)
	}

	// prepare genesis beacon state, with all of the validators in it.
	stateOpt, err := setup.StateTar(spec, keys, genesisTime)
	if err != nil {
		t.Fatal(err)
	}

	nodeCount := uint64(4)

	// Start beacon nodes
	for i := uint64(0); i < nodeCount; i++ {
		t.StartClientWithOptions("lighthouse_beacon", stateOpt, eth2Config)
	}

	// Start validator clients
	for i := uint64(0); i < nodeCount; i++ {
		// Give each validator client an equal subset of the genesis validator keys
		startIndex := valCount *i/nodeCount
		endIndex := valCount *(i+1)/nodeCount
		keysOpt, err := setup.KeysTar(keys[startIndex:endIndex])
		if err != nil {
			t.Fatal(err)
		}
		// Hook up validator to beacon node
		bnAPIOpt := hivesim.WithParams(map[string]string{
			"HIVE_ETH2_BN_API_ADDR": "", // TODO: get addrs from beacon nodes
		})

		t.StartClientWithOptions("lighthouse_validator", keysOpt, eth2Config, bnAPIOpt)
	}

	// TODO: not all clients have a beacon-node or validator part.
	// But they are both necessary, and need to be combined.
	// More cli options, or combined client name?
	// Prysm does not combine the validator and beacon docker images, so cannot share container.

	// TODO: faster slot times, shorter epochs, for fast testing.
	time.Sleep(15 * time.Minute)
}
