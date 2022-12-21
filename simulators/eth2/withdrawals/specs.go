package main

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/hive/hivesim"
	"github.com/ethereum/hive/simulators/eth2/common/clients"
	cl "github.com/ethereum/hive/simulators/eth2/common/config/consensus"
	el "github.com/ethereum/hive/simulators/eth2/common/config/execution"
	"github.com/ethereum/hive/simulators/eth2/common/testnet"
	beacon "github.com/protolambda/zrnt/eth2/beacon/common"
)

type BaseWithdrawalsTestSpec struct {
	// Spec
	Name        string
	Description string
	// Testnet Nodes
	NodeCount           int
	ValidatingNodeCount int
	// Beacon Chain
	ValidatorCount                 uint64
	ValidatorsExtraBalance         beacon.Gwei
	ExecutionWithdrawalCredentials bool
	CapellaGenesis                 bool
	// Test Function
	Run func(*hivesim.T, *testnet.Environment, *testnet.Config)
}

var (
	DEFAULT_VALIDATOR_COUNT uint64 = 64
	DEFAULT_SLOT_TIME       uint64 = 6

	EPOCHS_TO_FINALITY beacon.Epoch = 4

	// Default config used for all tests unless a client specific config exists
	DEFAULT_CONFIG = &testnet.Config{
		ValidatorCount:          big.NewInt(int64(DEFAULT_VALIDATOR_COUNT)),
		SlotTime:                big.NewInt(int64(DEFAULT_SLOT_TIME)),
		TerminalTotalDifficulty: common.Big0,
		AltairForkEpoch:         common.Big0,
		BellatrixForkEpoch:      common.Big0,
		CapellaForkEpoch:        common.Big1,
		Eth1Consensus:           &el.ExecutionCliqueConsensus{},
	}

	// Clients that do not support starting on epoch 0 with all forks enabled.
	// Tests take longer for these clients.
	/*
		INCREMENTAL_FORKS_CONFIG = &testnet.Config{
			AltairForkEpoch:    common.Big0,
			BellatrixForkEpoch: common.Big0,
			CapellaForkEpoch:   common.Big1,
		}
		INCREMENTAL_FORKS_CLIENTS = map[string]bool{
			"nimbus": true,
			"prysm":  true,
		}
	*/
)

func (ts BaseWithdrawalsTestSpec) GetTestnetConfig(
	nodeDefinition clients.NodeDefinition,
) *testnet.Config {
	config := DEFAULT_CONFIG

	/*
		if INCREMENTAL_FORKS_CLIENTS[n.ConsensusClient] {
			config = config.Join(INCREMENTAL_FORKS_CONFIG)
		}
	*/

	if ts.CapellaGenesis {
		config.CapellaForkEpoch = common.Big0
	}
	nodeCount := 2
	if ts.NodeCount > 0 {
		nodeCount = ts.NodeCount
	}
	maxValidatingNodeIndex := nodeCount - 1
	if ts.ValidatingNodeCount > 0 {
		maxValidatingNodeIndex = ts.ValidatingNodeCount - 1
	}
	nodeDefinitions := make(clients.NodeDefinitions, 0)
	for i := 0; i < nodeCount; i++ {
		n := nodeDefinition
		if i <= maxValidatingNodeIndex {
			n.ValidatorShares = 1
		} else {
			n.ValidatorShares = 0
		}
		nodeDefinitions = append(nodeDefinitions, n)
	}
	return config.Join(&testnet.Config{
		NodeDefinitions: nodeDefinitions,
	})
}

func (ts BaseWithdrawalsTestSpec) Execute(
	t *hivesim.T,
	env *testnet.Environment,
	n clients.NodeDefinition,
) {
	ts.Run(t, env, ts.GetTestnetConfig(n))
}

func (ts BaseWithdrawalsTestSpec) GetName() string {
	return ts.Name
}

func (ts BaseWithdrawalsTestSpec) GetDescription() string {
	return ts.Description
}

func (ts BaseWithdrawalsTestSpec) GetValidatorKeys(
	mnemonic string,
) []*cl.KeyDetails {
	var validatorCount uint64 = DEFAULT_VALIDATOR_COUNT
	if ts.ValidatorCount != 0 {
		validatorCount = ts.ValidatorCount
	}
	keySrc := &cl.MnemonicsKeySource{
		From:       0,
		To:         validatorCount,
		Validator:  mnemonic,
		Withdrawal: mnemonic,
	}
	keys, err := keySrc.Keys()
	if err != nil {
		panic(err)
	}

	for index, key := range keys {
		if ts.ValidatorsExtraBalance > 0 {
			key.ExtraInitialBalance = ts.ValidatorsExtraBalance
		}
		if ts.ExecutionWithdrawalCredentials {
			key.WithdrawalCredentialType = beacon.ETH1_ADDRESS_WITHDRAWAL_PREFIX
			key.WithdrawalExecAddress = beacon.Eth1Address{byte(index + 0x100)}
		}
	}

	return keys
}
