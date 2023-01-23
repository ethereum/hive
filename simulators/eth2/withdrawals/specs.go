package main

import (
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
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
	ValidatorCount uint64
	CapellaGenesis bool

	// Genesis Validators Configuration
	// (One every Nth validator, 1 means all validators, 2 means half, etc...)
	GenesisExecutionWithdrawalCredentialsShares int
	GenesisExitedShares                         int
	GenesisSlashedShares                        int

	// Other Testing Configuration
	SubmitBLSChangesOnBellatrix bool
}

var (
	DEFAULT_VALIDATOR_COUNT uint64 = 128
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
	allNodeDefinitions []clients.NodeDefinition,
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
	if len(allNodeDefinitions) == 0 {
		panic("incorrect number of node definitions")
	} else if len(allNodeDefinitions) > 1 {
		nodeCount = len(allNodeDefinitions)
	}
	if ts.NodeCount > 0 {
		nodeCount = ts.NodeCount
	}
	maxValidatingNodeIndex := nodeCount - 1
	if ts.ValidatingNodeCount > 0 {
		maxValidatingNodeIndex = ts.ValidatingNodeCount - 1
	}
	nodeDefinitions := make(clients.NodeDefinitions, 0)
	for i := 0; i < nodeCount; i++ {
		n := allNodeDefinitions[i%len(allNodeDefinitions)]
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

func (ts BaseWithdrawalsTestSpec) CanRun(clients.NodeDefinitions) bool {
	// Base test specs can always run
	return true
}

func (ts BaseWithdrawalsTestSpec) GetName() string {
	return ts.Name
}

func (ts BaseWithdrawalsTestSpec) GetDescription() string {
	return ts.Description
}

func (ts BaseWithdrawalsTestSpec) GetValidatorCount() uint64 {
	if ts.ValidatorCount != 0 {
		return ts.ValidatorCount
	}
	return DEFAULT_VALIDATOR_COUNT
}

func (ts BaseWithdrawalsTestSpec) GetValidatorKeys(
	mnemonic string,
) []*cl.KeyDetails {
	keySrc := &cl.MnemonicsKeySource{
		From:       0,
		To:         ts.GetValidatorCount(),
		Validator:  mnemonic,
		Withdrawal: mnemonic,
	}
	keys, err := keySrc.Keys()
	if err != nil {
		panic(err)
	}

	for index, key := range keys {
		// All validators have idiosyncratic balance amounts to identify them
		key.ExtraInitialBalance = beacon.Gwei(index + 1)

		if ts.GenesisExecutionWithdrawalCredentialsShares > 0 &&
			(index%ts.GenesisExecutionWithdrawalCredentialsShares) == 0 {
			key.WithdrawalCredentialType = beacon.ETH1_ADDRESS_WITHDRAWAL_PREFIX
			key.WithdrawalExecAddress = beacon.Eth1Address{byte(index + 0x100)}
		}
		if ts.GenesisExitedShares > 1 && (index%ts.GenesisExitedShares) == 1 {
			key.Exited = true
		}
		if ts.GenesisSlashedShares > 2 &&
			(index%ts.GenesisSlashedShares) == 2 {
			key.Slashed = true
		}
		fmt.Printf(
			"INFO: Validator %d, extra_gwei=%d, exited=%v, slashed=%v, key_type=%d\n",
			index,
			key.ExtraInitialBalance,
			key.Exited,
			key.Slashed,
			key.WithdrawalCredentialType,
		)
	}

	return keys
}
