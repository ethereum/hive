package main

import (
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/hive/hivesim"
	"github.com/ethereum/hive/simulators/eth2/engine/setup"
)

var (
	CHAIN_ID = big.NewInt(1)
	// This is the account that send transactions.
	VAULT_ACCOUNT_ADDR = common.HexToAddress("0xcf49fda3be353c69b41ed96333cd24302da4556f")
	VAULT_KEY, _       = crypto.HexToECDSA("63b508a03c3b5937ceb903af8b1b0c191012ef6eb7e9c3fb7afa94e5d214d376")
)

var engineTests = []testSpec{
	//{Name: "transition-testnet", Run: TransitionTestnet},
	{Name: "test-rpc-error", Run: TestRPCError},
	{Name: "block-latest-safe-finalized", Run: BlockLatestSafeFinalized},
	{Name: "invalid-canonical-payload", Run: InvalidPayloadGen(2, Invalid)},
	{Name: "invalid-payload-block-hash", Run: InvalidPayloadGen(2, InvalidBlockHash)},
	{Name: "invalid-header-prevrandao", Run: IncorrectHeaderPrevRandaoPayload},
	{Name: "invalid-timestamp", Run: InvalidTimestampPayload},
	{Name: "syncing-with-invalid-chain", Run: SyncingWithInvalidChain},
	{Name: "basefee-encoding-check", Run: BaseFeeEncodingCheck},
	{Name: "invalid-quantity-fields", Run: InvalidQuantityPayloadFields},
}
var transitionTests = []testSpec{
	// Transition (TERMINAL_TOTAL_DIFFICULTY) tests
	{Name: "invalid-transition-payload", Run: InvalidPayloadGen(1, Invalid)},
	{Name: "unknown-pow-parent-transition-payload", Run: UnknownPoWParent},
	{Name: "ttd-before-bellatrix", Run: TTDBeforeBellatrix},
	{Name: "equal-timestamp-terminal-transition-block", Run: EqualTimestampTerminalTransitionBlock},
	{Name: "invalid-terminal-block-payload-lower-ttd", Run: IncorrectTerminalBlockGen(-2)},
	{Name: "invalid-terminal-block-payload-higher-ttd", Run: IncorrectTerminalBlockGen(1)},
	{Name: "build-atop-invalid-terminal-block", Run: IncorrectTTDConfigEL},
	{Name: "syncing-with-chain-having-valid-transition-block", Run: SyncingWithChainHavingValidTransitionBlock},
	{Name: "syncing-with-chain-having-invalid-transition-block", Run: SyncingWithChainHavingInvalidTransitionBlock},
	{Name: "syncing-with-chain-having-invalid-post-transition-block", Run: SyncingWithChainHavingInvalidPostTransitionBlock},
	{Name: "re-org-and-sync-with-chain-having-invalid-terminal-block", Run: ReOrgSyncWithChainHavingInvalidTerminalBlock},
}

func main() {

	// Create simulator that runs all tests
	sim := hivesim.New()
	// From the simulator we can get all client types provided
	clientTypes, err := sim.ClientTypes()
	if err != nil {
		panic(err)
	}
	c := ClientsByRole(clientTypes)
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
	var (
		engineSuite = hivesim.Suite{
			Name:        "eth2-engine",
			Description: `Collection of test vectors that use a ExecutionClient+BeaconNode+ValidatorClient testnet.`,
		}
		transitionSuite = hivesim.Suite{
			Name:        "eth2-engine-transition",
			Description: `Collection of test vectors that use a ExecutionClient+BeaconNode+ValidatorClient transition testnet.`,
		}
	)
	// Add all tests to the suites
	addAllTests(&engineSuite, c, engineTests)
	addAllTests(&transitionSuite, c, transitionTests)

	// Mark suites for execution
	hivesim.MustRunSuite(sim, engineSuite)
	hivesim.MustRunSuite(sim, transitionSuite)
}

func addAllTests(suite *hivesim.Suite, c *ClientDefinitionsByRole, tests []testSpec) {
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
		panic(err)
	}
	secrets, err := setup.SecretKeys(keys)
	if err != nil {
		panic(err)
	}
	for _, node := range c.Combinations() {
		for _, test := range tests {
			test := test
			suite.Add(hivesim.TestSpec{
				Name:        fmt.Sprintf("%s-%s", test.Name, node.String()),
				Description: test.About,
				Run: func(t *hivesim.T) {
					env := &testEnv{c, keys, secrets}
					test.Run(t, env, node)
				},
			},
			)
		}
	}
}
