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
	beacon "github.com/protolambda/zrnt/eth2/beacon/common"
)

var (
	CHAIN_ID = big.NewInt(1)
	// This is the account that send transactions.
	VAULT_ACCOUNT_ADDR = common.HexToAddress(
		"0xcf49fda3be353c69b41ed96333cd24302da4556f",
	)
	VAULT_KEY, _ = crypto.HexToECDSA(
		"63b508a03c3b5937ceb903af8b1b0c191012ef6eb7e9c3fb7afa94e5d214d376",
	)
)

type TestSpec interface {
	GetName() string
	GetDescription() string
	Execute(*hivesim.T, *testnet.Environment, clients.NodeDefinition)
	GetValidatorKeys(string) []*consensus_config.KeyDetails
}

var engineTests = []TestSpec{
	BaseWithdrawalsTestSpec{
		Name: "test-capella-fork",
		Run:  TestCapellaFork,
		Description: `
		Sanity test to check the fork transition to capella.
		`,
		CapellaGenesis: false,
	},

	BaseWithdrawalsTestSpec{
		Name: "test-capella-genesis",
		Run:  TestCapellaFork,
		Description: `
		Sanity test to check the beacon clients can start with capella genesis.
		`,
		CapellaGenesis: true,
	},

	BLSToExecutionChangeTestSpec{
		BaseWithdrawalsTestSpec: BaseWithdrawalsTestSpec{
			Name: "test-bls-to-execution-changes-on-capella-genesis",
			Description: `
			Send BLS-To-Execution-Changes after capella fork has happened.
			`,
			CapellaGenesis: true,
		},
	},

	BLSToExecutionChangeTestSpec{
		BaseWithdrawalsTestSpec: BaseWithdrawalsTestSpec{
			Name: "test-bls-to-execution-changes-after-fork-on-bellatrix-genesis",
			Description: `
			Send BLS-To-Execution-Changes before capella fork has happened,
			and verify they are included after the fork happens.
			`,
			CapellaGenesis: false,
		},
		SubmitAfterCapellaFork: true,
	},

	BLSToExecutionChangeTestSpec{
		BaseWithdrawalsTestSpec: BaseWithdrawalsTestSpec{
			Name: "test-bls-to-execution-changes-bellatrix-genesis",
			Description: `
			Send BLS-To-Execution-Changes before capella fork has happened,
			and verify they are included after the fork happens.
			`,
			CapellaGenesis: false,
		},
		SubmitAfterCapellaFork: false,
	},

	BaseWithdrawalsTestSpec{
		Name: "test-partial-withdrawals-without-bls-to-execution-changes-bellatrix-genesis",
		Description: `
		Test partial withdrawals from accounts that already had the
		Eth1 withdrawal prefix since before the capella fork.
		`,
		Run:                            TestCapellaPartialWithdrawalsWithoutBLSChanges,
		ValidatorsExtraBalance:         beacon.Gwei(1),
		ExecutionWithdrawalCredentials: true,
		CapellaGenesis:                 false,
	},
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

	// Add all tests to the suites
	addAllTests(&engineSuite, c, engineTests)

	// Mark suites for execution
	hivesim.MustRunSuite(sim, engineSuite)
}

func addAllTests(
	suite *hivesim.Suite,
	c *clients.ClientDefinitionsByRole,
	tests []TestSpec,
) {
	mnemonic := "couple kiwi radio river setup fortune hunt grief buddy forward perfect empty slim wear bounce drift execute nation tobacco dutch chapter festival ice fog"

	for _, nodeDefinition := range c.Combinations() {
		for _, test := range tests {
			test := test
			// Generate validator keys to use for this test.

			suite.Add(hivesim.TestSpec{
				Name: fmt.Sprintf(
					"%s-%s",
					test.GetName(),
					nodeDefinition.String(),
				),
				Description: test.GetDescription(),
				Run: func(t *hivesim.T) {
					keys := test.GetValidatorKeys(mnemonic)
					secrets, err := consensus_config.SecretKeys(keys)
					if err != nil {
						panic(err)
					}
					env := &testnet.Environment{
						Clients: c,
						Keys:    keys,
						Secrets: secrets,
					}
					test.Execute(t, env, nodeDefinition)
				},
			},
			)
		}
	}
}
