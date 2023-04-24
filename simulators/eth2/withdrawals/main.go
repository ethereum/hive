package main

import (
	"fmt"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/hive/hivesim"
	mock_builder "github.com/ethereum/hive/simulators/eth2/common/builder/mock"
	"github.com/ethereum/hive/simulators/eth2/common/clients"
	consensus_config "github.com/ethereum/hive/simulators/eth2/common/config/consensus"
	"github.com/ethereum/hive/simulators/eth2/common/testnet"
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
	Execute(*hivesim.T, *testnet.Environment, []clients.NodeDefinition)
	GetValidatorKeys(string) []*consensus_config.KeyDetails
}

var tests = []TestSpec{
	BaseWithdrawalsTestSpec{
		Name: "test-capella-fork",
		Description: `
		Sanity test to check the fork transition to capella.
		All validators start with Execution Withdrawal Credentials,
		therefore no BLS-To-Execution changes are tested here.
		`,
		CapellaGenesis: false,
		GenesisExecutionWithdrawalCredentialsShares: 1,
	},

	BaseWithdrawalsTestSpec{
		Name: "test-capella-genesis",
		Description: `
		Sanity test to check the beacon clients can start with capella genesis.
		All validators start with Execution Withdrawal Credentials,
		therefore no BLS-To-Execution changes are tested here.
		`,
		CapellaGenesis: true,
		GenesisExecutionWithdrawalCredentialsShares: 1,
	},

	BaseWithdrawalsTestSpec{
		Name: "test-large-withdrawal-amounts",
		Description: `
		Test withdrawing large amounts, such that:
		- The amount multiplied by 1e9 overflows uint64 (on conversion from gwei to wei)
		`,
		CapellaGenesis: true,
		GenesisExecutionWithdrawalCredentialsShares: 1,
		ExtraGwei: 68719476736, // ((2 ** 35) * (1e9)) > (2 ** 64)
	},

	BaseWithdrawalsTestSpec{
		Name: "test-bls-to-execution-changes-on-capella-bellatrix-genesis",
		Description: `
		Send BLS-To-Execution-Changes after capella fork has happened.
		Half of the validators still on BLS withdrawal credentials, and the
		BLS-To-Execution-Change directives are sent for all the remaining
		validators.
		`,
		CapellaGenesis: false,
		GenesisExecutionWithdrawalCredentialsShares: 2,
		SubmitBLSChangesOnBellatrix:                 true,
	},

	BaseWithdrawalsTestSpec{
		Name: "test-full-withdrawals-bls-to-execution-changes-bellatrix-genesis",
		Description: `
			Test BLS-To-Execution-Changes to fully exit validators.
			`,
		CapellaGenesis: false,
		GenesisExecutionWithdrawalCredentialsShares: 2,
		GenesisExitedShares:                         8,
		GenesisSlashedShares:                        8,
	},
}

var builderTests = []TestSpec{
	BuilderWithdrawalsTestSpec{
		BaseWithdrawalsTestSpec: BaseWithdrawalsTestSpec{
			Name: "test-builders-capella-invalid-withdrawals",
			Description: `
			Test canonical chain can still finalize if the builders start
			building payloads with invalid withdrawals list.
			`,
			// All validators can withdraw from the start
			GenesisExecutionWithdrawalCredentialsShares: 1,
		},
		InvalidatePayloadAttributes: mock_builder.INVALIDATE_ATTR_EXTRA_WITHDRAWAL,
	},
	BuilderWithdrawalsTestSpec{
		BaseWithdrawalsTestSpec: BaseWithdrawalsTestSpec{
			Name: "test-builders-capella-error-on-capella-header-request",
			Description: `
			Test canonical chain can still finalize if the builders start
			returning error on header request after capella transition.
			`,
			// All validators can withdraw from the start
			GenesisExecutionWithdrawalCredentialsShares: 1,
		},
		ErrorOnHeaderRequest: true,
	},
	BuilderWithdrawalsTestSpec{
		BaseWithdrawalsTestSpec: BaseWithdrawalsTestSpec{
			Name: "test-builders-capella-error-on-capella-unblind-payload-requestr",
			Description: `
			Test canonical chain can still finalize if the builders start
			returning error on unblinded payload request after capella transition.
			`,
			// All validators can withdraw from the start
			GenesisExecutionWithdrawalCredentialsShares: 1,
		},
		ErrorOnPayloadReveal: true,
	},
	BuilderWithdrawalsTestSpec{
		BaseWithdrawalsTestSpec: BaseWithdrawalsTestSpec{
			Name: "test-builders-capella-invalid-payload-version",
			Description: `
			Test consensus clients correctly reject a built payload if the
			version is outdated (bellatrix instead of capella).
			`,
			// All validators can withdraw from the start
			GenesisExecutionWithdrawalCredentialsShares: 1,
		},
		InvalidPayloadVersion: true,
	},
	BuilderWithdrawalsTestSpec{
		BaseWithdrawalsTestSpec: BaseWithdrawalsTestSpec{
			Name: "test-builders-capella-invalid-payload",
			Description: `
			Test consensus clients correctly circuit break builder after a
			period of empty blocks due to invalid unblinded blocks.
			The payloads are built using an invalid state root, which can only
			be caught after unblinding the entire payload and running it in the
			local execution client, at which point another payload cannot be
			produced locally and results in an empty slot.
			`,
			// All validators can withdraw from the start
			GenesisExecutionWithdrawalCredentialsShares: 1,
		},
		InvalidatePayload: mock_builder.INVALIDATE_PAYLOAD_STATE_ROOT,
	},
	BuilderWithdrawalsTestSpec{
		BaseWithdrawalsTestSpec: BaseWithdrawalsTestSpec{
			Name: "test-builders-capella-correct-withdrawals",
			Description: `
			Test canonical chain includes capella payloads built by the builder api.
			`,
			// All validators start with BLS withdrawal credentials
			GenesisExecutionWithdrawalCredentialsShares: 0,
		},
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

	// Create the test suites
	withdrawalsSuite := hivesim.Suite{
		Name:        "eth2-withdrawals",
		Description: `Collection of test vectors that use a ExecutionClient+BeaconNode+ValidatorClient testnet for Shanghai+Capella.`,
	}
	builderSuite := hivesim.Suite{
		Name:        "eth2-withdrawals-builder",
		Description: `Collection of test vectors that use a ExecutionClient+BeaconNode+ValidatorClient testnet and builder API for Shanghai+Capella.`,
	}

	// Add all tests to the suites
	addAllTests(&withdrawalsSuite, c, tests)
	addAllTests(&builderSuite, c, builderTests)

	// Mark suites for execution
	hivesim.MustRunSuite(sim, withdrawalsSuite)
	hivesim.MustRunSuite(sim, builderSuite)
}

func addAllTests(
	suite *hivesim.Suite,
	c *clients.ClientDefinitionsByRole,
	tests []TestSpec,
) {
	mnemonic := "couple kiwi radio river setup fortune hunt grief buddy forward perfect empty slim wear bounce drift execute nation tobacco dutch chapter festival ice fog"

	clientCombinations := c.Combinations()
	for _, test := range tests {
		test := test
		suite.Add(hivesim.TestSpec{
			Name: fmt.Sprintf(
				"%s-%s",
				test.GetName(),
				strings.Join(clientCombinations.ClientTypes(), "-"),
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
				test.Execute(t, env, clientCombinations)
			},
		},
		)
	}
}
