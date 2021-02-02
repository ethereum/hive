package testnet

import (
	"github.com/ethereum/hive/hivesim"
	"io"
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
	// TODO: generate genesis state
	// TODO: generate keystores
	// TODO: generate bootnodes list
	//
	genesisState := hivesim.WithTAR(func() io.ReadCloser {
		return nil
	})
	eth2Config := hivesim.WithParams(map[string]string{
		"HIVE_ETH2_FOOBAR": "1234",
	})
	// TODO: start beacon nodes and validators
	// TODO: not all clients have a beacon-node or validator part.
	// But they are both necessary, and need to be combined.
	// More cli options, or combined client name?
	// Prysm does not combine the validator and beacon docker images, so cannot share container.
	t.StartClientWithOptions("lighthouse_beacon", genesisState, eth2Config)
	t.StartClientWithOptions("lighthouse_validator", genesisState, eth2Config)
}
