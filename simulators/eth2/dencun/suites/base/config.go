package suite_base

import (
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/hive/simulators/eth2/common/clients"
	"github.com/ethereum/hive/simulators/eth2/common/config"
	cl "github.com/ethereum/hive/simulators/eth2/common/config/consensus"
	el "github.com/ethereum/hive/simulators/eth2/common/config/execution"
	"github.com/ethereum/hive/simulators/eth2/common/testnet"
	"github.com/ethereum/hive/simulators/eth2/common/utils"
	"github.com/ethereum/hive/simulators/ethereum/engine/globals"
	beacon "github.com/protolambda/zrnt/eth2/beacon/common"
)

type BaseTestSpec struct {
	// Spec
	Name        string
	DisplayName string
	Description string

	// Testnet Nodes
	NodeCount           int
	ValidatingNodeCount int

	// Beacon Chain
	ValidatorCount uint64
	DenebGenesis   bool

	// Genesis Validators Configuration
	// (One every Nth validator, 1 means all validators, 2 means half, etc...)
	GenesisExecutionWithdrawalCredentialsShares int
	GenesisExitedShares                         int
	GenesisSlashedShares                        int

	// Actions
	ExitValidatorsShare int

	// Verifications
	EpochsAfterFork beacon.Epoch
	WaitForBlobs    bool
	WaitForFinality bool

	// Extra Gwei
	ExtraGwei beacon.Gwei
}

var (
	DEFAULT_VALIDATOR_COUNT uint64 = 128

	EPOCHS_TO_FINALITY beacon.Epoch = 4

	// Default config used for all tests unless a client specific config exists
	DEFAULT_CONFIG = &testnet.Config{
		ConsensusConfig: &cl.ConsensusConfig{
			ValidatorCount: big.NewInt(int64(DEFAULT_VALIDATOR_COUNT)),
		},
		ForkConfig: &config.ForkConfig{
			TerminalTotalDifficulty: common.Big0,
			AltairForkEpoch:         common.Big0,
			BellatrixForkEpoch:      common.Big0,
			CapellaForkEpoch:        common.Big0,
			DenebForkEpoch:          common.Big1,
		},
		Eth1Consensus: &el.ExecutionPostMergeGenesis{},
	}

	// This is the account that sends vault funding transactions.
	VaultStartAmount, _ = new(big.Int).SetString("d3c21bcecceda1000000", 16)

	CodeContractAddress = common.HexToAddress(
		"0xcccccccccccccccccccccccccccccccccccccccc",
	)
	CodeContract = common.Hex2Bytes("0x328043558043600080a250")

	BeaconRootContractAddress = common.HexToAddress(
		"0xbEAC020008aFF7331c0A389CB2AAb67597567d7a",
	)

	GasPrice    = big.NewInt(30 * params.GWei)
	GasTipPrice = big.NewInt(1 * params.GWei)

	ChainID = big.NewInt(7)
)

func (ts BaseTestSpec) GetNodeCount() int {
	if ts.NodeCount > 0 {
		return ts.NodeCount
	}
	return 2
}

func (ts BaseTestSpec) GetValidatingNodeCount() int {
	if ts.ValidatingNodeCount > 0 {
		return ts.ValidatingNodeCount
	}
	return ts.GetNodeCount()
}

func (ts BaseTestSpec) GetTestnetConfig(
	allNodeDefinitions clients.NodeDefinitions,
) *testnet.Config {
	config := *DEFAULT_CONFIG

	if ts.DenebGenesis {
		config.DenebForkEpoch = common.Big0
	}

	nodeCount := ts.GetNodeCount()

	maxValidatingNodeIndex := ts.GetValidatingNodeCount() - 1
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

	// Fund execution layer account for transactions
	config.GenesisExecutionAccounts = map[common.Address]core.GenesisAccount{
		CodeContractAddress: {
			Balance: common.Big0,
			Code:    CodeContract,
		},
	}

	for _, acc := range globals.TestAccounts {
		config.GenesisExecutionAccounts[acc.GetAddress()] = core.GenesisAccount{
			Balance: VaultStartAmount,
		}
	}

	return config.Join(&testnet.Config{
		NodeDefinitions: nodeDefinitions,
	})
}

func (ts BaseTestSpec) CanRun(clients.NodeDefinitions) bool {
	// Base test specs can always run
	return true
}

func (ts BaseTestSpec) GetName() string {
	return ts.Name
}

func (ts BaseTestSpec) GetDisplayName() string {
	return ts.DisplayName
}

func (ts BaseTestSpec) GetDescription() *utils.Description {
	desc := utils.NewDescription(ts.Description)

	// Add the testnet config description
	desc.Add(utils.CategoryTestnetConfiguration, fmt.Sprintf(`
	  - Node Count: %d
	  - Validating Node Count: %d
	  - Validator Key Count: %d
	  - Validator Key per Node: %d`,
		ts.GetNodeCount(),
		ts.GetValidatingNodeCount(),
		ts.GetValidatorCount(),
		ts.GetValidatorCount()/uint64(ts.GetValidatingNodeCount()),
	))
	if ts.DenebGenesis {
		desc.Add(utils.CategoryTestnetConfiguration, "- Genesis Fork: Deneb")
	} else {
		desc.Add(utils.CategoryTestnetConfiguration, "- Genesis Fork: Capella")
	}
	execCredentialCount := ts.GetExecutionWithdrawalCredentialCount()
	blsCredentialCount := ts.GetValidatorCount() - execCredentialCount
	desc.Add(utils.CategoryTestnetConfiguration, fmt.Sprintf(`
	  - Execution Withdrawal Credentials Count: %d
	  - BLS Withdrawal Credentials Count: %d`,
		execCredentialCount,
		blsCredentialCount,
	))

	// Add the verifications description
	desc.Add(utils.CategoryVerificationsExecutionClient, `
	  - Blob (type-3) transactions are included in the blocks`)
	desc.Add(utils.CategoryVerificationsConsensusClient, `
	  - For each blob transaction on the execution chain, the blob sidecars are available for the beacon block at the same height
	  - The beacon block lists the correct commitments for each blob`)
	if ts.WaitForFinality {
		desc.Add(utils.CategoryVerificationsConsensusClient, `
		- After all other verifications are done, the beacon chain is able to finalize the current epoch`)
	}

	return desc
}

func (ts BaseTestSpec) GetValidatorCount() uint64 {
	if ts.ValidatorCount != 0 {
		return ts.ValidatorCount
	}
	return DEFAULT_VALIDATOR_COUNT
}

func (ts BaseTestSpec) GetExecutionWithdrawalCredentialCount() uint64 {
	if ts.GenesisExecutionWithdrawalCredentialsShares != 0 {
		return ts.GetValidatorCount() / uint64(ts.GenesisExecutionWithdrawalCredentialsShares)
	}
	return 0
}

func (ts BaseTestSpec) GetValidatorKeys(
	mnemonic string,
) cl.ValidatorsSetupDetails {
	keySrc := &cl.MnemonicsKeySource{
		From:     0,
		To:       ts.GetValidatorCount(),
		Mnemonic: mnemonic,
	}
	keys, err := keySrc.Keys()
	if err != nil {
		panic(err)
	}

	for index, key := range keys {
		// All validators have idiosyncratic balance amounts to identify them.
		// Also include a high amount in order to guarantee withdrawals.
		key.ExtraInitialBalance = beacon.Gwei((index+1)*1000000) + ts.ExtraGwei

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
