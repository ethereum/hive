package suites

import (
	"context"
	"fmt"
	"strings"

	"github.com/ethereum/hive/hivesim"
	"github.com/ethereum/hive/simulators/eth2/common/clients"
	consensus_config "github.com/ethereum/hive/simulators/eth2/common/config/consensus"
	"github.com/ethereum/hive/simulators/eth2/common/testnet"
	"github.com/ethereum/hive/simulators/eth2/common/utils"
)

var Deneb string = "deneb"

type TestSpec interface {
	GetName() string
	GetTestnetConfig(clients.NodeDefinitions) *testnet.Config
	GetDisplayName() string
	GetDescription() *utils.Description
	ExecutePreFork(*hivesim.T, context.Context, *testnet.Testnet, *testnet.Environment, *testnet.Config)
	WaitForFork(*hivesim.T, context.Context, *testnet.Testnet, *testnet.Environment, *testnet.Config)
	ExecutePostFork(*hivesim.T, context.Context, *testnet.Testnet, *testnet.Environment, *testnet.Config)
	ExecutePostForkWait(*hivesim.T, context.Context, *testnet.Testnet, *testnet.Environment, *testnet.Config)
	Verify(*hivesim.T, context.Context, *testnet.Testnet, *testnet.Environment, *testnet.Config)
	GetValidatorKeys(string) consensus_config.ValidatorsSetupDetails
}

// Add all tests to the suite
func SuiteHydrate(
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
			DisplayName: test.GetDisplayName(),
			Description: test.GetDescription().Format(),
			Run: func(t *hivesim.T) {
				t.Logf("Starting test: %s", test.GetName())
				defer t.Logf("Finished test: %s", test.GetName())
				keys := test.GetValidatorKeys(mnemonic)
				env := &testnet.Environment{
					Clients:    c,
					Validators: keys,
				}
				config := test.GetTestnetConfig(clientCombinations)

				// Create the testnet
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				testnet := testnet.StartTestnet(ctx, t, env, config)
				if testnet == nil {
					t.Fatalf("failed to start testnet")
				}
				defer testnet.Stop()

				// Execute pre-fork
				test.ExecutePreFork(t, ctx, testnet, env, config)

				// Wait for the fork
				test.WaitForFork(t, ctx, testnet, env, config)

				// Execute post-fork
				test.ExecutePostFork(t, ctx, testnet, env, config)

				// Execute post-fork wait
				test.ExecutePostForkWait(t, ctx, testnet, env, config)

				// Verify
				test.Verify(t, ctx, testnet, env, config)
			},
		},
		)
	}
}
