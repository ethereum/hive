package main

import (
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/hive/hivesim"
	"github.com/ethereum/hive/simulators/eth2/common/clients"
	consensus_config "github.com/ethereum/hive/simulators/eth2/common/config/consensus"
	"github.com/ethereum/hive/simulators/eth2/common/testnet"
)

var (
	CHAIN_ID = big.NewInt(1)
	// This is the account that send transactions.
	VAULT_ACCOUNT_ADDR = common.HexToAddress("0xcf49fda3be353c69b41ed96333cd24302da4556f")
	VAULT_KEY, _       = crypto.HexToECDSA("63b508a03c3b5937ceb903af8b1b0c191012ef6eb7e9c3fb7afa94e5d214d376")
)

var engineTests = []testSpec{
	{Name: "test-rpc-error", Run: TestRPCError},
	{Name: "block-latest-safe-finalized", Run: BlockLatestSafeFinalized},
	{Name: "invalid-canonical-payload", Run: InvalidPayloadGen(2, Invalid)},
	{Name: "invalid-payload-block-hash", Run: InvalidPayloadGen(2, InvalidBlockHash)},
	{Name: "invalid-header-prevrandao", Run: IncorrectHeaderPrevRandaoPayload},
	{Name: "invalid-timestamp", Run: InvalidTimestampPayload},
	{Name: "syncing-with-invalid-chain", Run: SyncingWithInvalidChain},
	{Name: "basefee-encoding-check", Run: BaseFeeEncodingCheck},
	{Name: "invalid-quantity-fields", Run: InvalidQuantityPayloadFields},
	{Name: "timeouts", Run: Timeouts},
}

func main() {
	// Create simulator that runs all tests
	sim := hivesim.New()
	// From the simulator we can get all client types provided
	clientTypes, err := sim.ClientTypes()
	if err != nil {
		panic(err)
	}
	c := clients.ClientsByRole(clientTypes)
	if len(c.Eth1) != 1 {
		panic("choose 1 eth1 client type")
	}
	if len(c.Beacon) != 1 {
		panic("choose 1 beacon client type")
	}
	if len(c.Validator) != 1 {
		panic("choose 1 validator client type")
	}
	// Create the test suites
	engineSuite := hivesim.Suite{
		Name:        "eth2-engine",
		Description: `Collection of test vectors that use a ExecutionClient+BeaconNode+ValidatorClient testnet.`,
	}
	// Add all tests to the suite
	addAllTests(&engineSuite, c, engineTests)

	// Mark suite for execution
	hivesim.MustRunSuite(sim, engineSuite)
}

func addAllTests(suite *hivesim.Suite, c *clients.ClientDefinitionsByRole, tests []testSpec) {
	mnemonic := "couple kiwi radio river setup fortune hunt grief buddy forward perfect empty slim wear bounce drift execute nation tobacco dutch chapter festival ice fog"

	// Generate validator keys to use for all tests.
	keySrc := &consensus_config.MnemonicsKeySource{
		From:     0,
		To:       64,
		Mnemonic: mnemonic,
	}
	keys, err := keySrc.Keys()
	if err != nil {
		panic(err)
	}
	for _, nodeDefinition := range c.Combinations() {
		for _, test := range tests {
			test := test
			suite.Add(hivesim.TestSpec{
				Name:        fmt.Sprintf("%s-%s", test.Name, nodeDefinition.String()),
				Description: test.About,
				Run: func(t *hivesim.T) {
					env := &testnet.Environment{
						Clients:    c,
						Validators: keys,
					}
					test.Run(t, env, nodeDefinition)
				},
			},
			)
		}
	}
}
