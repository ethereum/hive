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

var tests = []testSpec{
	//{Name: "transition-testnet", Run: TransitionTestnet},
	{Name: "test-rpc-error", Run: TestRPCError},
	{Name: "invalid-transition-payload", Run: InvalidTransitionPayload},
	{Name: "invalid-payload-block-hash", Run: InvalidTransitionPayloadBlockHash},
	{Name: "invalid-header-prevrandao", Run: IncorrectHeaderPrevRandaoPayload},
	{Name: "invalid-timestamp", Run: InvalidTimestampPayload},
	{Name: "invalid-terminal-block-payload-lower-ttd", Run: IncorrectTerminalBlockGen(-2)},
	{Name: "invalid-terminal-block-payload-higher-ttd", Run: IncorrectTerminalBlockGen(1)},
	{Name: "syncing-with-invalid-chain", Run: SyncingWithInvalidChain},
	{Name: "basefee-encoding-check", Run: BaseFeeEncodingCheck},
	{Name: "equal-timestamp-terminal-transition-block", Run: EqualTimestampTerminalTransitionBlock},
	{Name: "ttd-before-bellatrix", Run: TTDBeforeBellatrix},
	{Name: "invalid-quantity-fields", Run: InvalidQuantityPayloadFields},
}

func main() {
	var suite = hivesim.Suite{
		Name:        "cl-test",
		Description: `Run different eth2 testnets.`,
	}
	suite.Add(hivesim.TestSpec{
		Name:        "cl-tests",
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
				Name:        fmt.Sprintf("%s-%s", test.Name, node.String()),
				Description: test.About,
				Run: func(t *hivesim.T) {
					env := &testEnv{c, keys, secrets}
					test.Run(t, env, node)
				},
			})
		}
	}
}
