package main

import (
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/params"
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

	// Verifications
	WaitForFinality bool

	// Extra Gwei
	ExtraGwei beacon.Gwei
}

var (
	DEFAULT_VALIDATOR_COUNT uint64 = 128
	MAINNET_SLOT_TIME       int64  = 12
	MINIMAL_SLOT_TIME       int64  = 6

	EPOCHS_TO_FINALITY beacon.Epoch = 4

	// Default config used for all tests unless a client specific config exists
	DEFAULT_CONFIG = &testnet.Config{
		ValidatorCount:          big.NewInt(int64(DEFAULT_VALIDATOR_COUNT)),
		SlotTime:                big.NewInt(MAINNET_SLOT_TIME),
		TerminalTotalDifficulty: common.Big0,
		AltairForkEpoch:         common.Big0,
		BellatrixForkEpoch:      common.Big0,
		CapellaForkEpoch:        common.Big1,
		Eth1Consensus:           &el.ExecutionPostMergeGenesis{},
	}

	MINIMAL_SLOT_TIME_CLIENTS = []string{
		"lighthouse",
		"teku",
		"prysm",
		"lodestar",
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

	// This is the account that sends vault funding transactions.
	VaultAccountAddress = common.HexToAddress(
		"0xcf49fda3be353c69b41ed96333cd24302da4556f",
	)
	VaultKey, _ = crypto.HexToECDSA(
		"63b508a03c3b5937ceb903af8b1b0c191012ef6eb7e9c3fb7afa94e5d214d376",
	)
	VaultStartAmount, _ = new(big.Int).SetString("d3c21bcecceda1000000", 16)

	CodeContractAddress = common.HexToAddress(
		"0xcccccccccccccccccccccccccccccccccccccccc",
	)
	CodeContract = common.Hex2Bytes("0x328043558043600080a250")

	GasPrice    = big.NewInt(30 * params.GWei)
	GasTipPrice = big.NewInt(1 * params.GWei)

	ChainID = big.NewInt(7)
)

func (ts BaseWithdrawalsTestSpec) GetTestnetConfig(
	allNodeDefinitions clients.NodeDefinitions,
) *testnet.Config {
	config := *DEFAULT_CONFIG

	/*
		if INCREMENTAL_FORKS_CLIENTS[n.ConsensusClient] {
			config = config.Join(INCREMENTAL_FORKS_CONFIG)
		}
	*/

	if len(
		allNodeDefinitions.FilterByCL(MINIMAL_SLOT_TIME_CLIENTS),
	) == len(
		allNodeDefinitions,
	) {
		// If all clients support using minimal 6 second slot time, use it
		config.SlotTime = big.NewInt(MINIMAL_SLOT_TIME)
	}
	fmt.Printf("INFO: using %d second slot time\n", config.SlotTime)

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

	// Fund execution layer account for transactions

	config.GenesisExecutionAccounts = map[common.Address]core.GenesisAccount{
		VaultAccountAddress: {
			Balance: VaultStartAmount,
		},
		CodeContractAddress: {
			Balance: common.Big0,
			Code:    CodeContract,
		},
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

type BuilderTestError int

const (
	NO_ERROR BuilderTestError = iota
	ERROR_ON_HEADER_REQUEST
	ERROR_ON_UNBLINDED_PAYLOAD_REQUEST
	INVALID_WITHDRAWALS
	INVALIDATE_SINGLE_WITHDRAWAL_ADDRESS
	INVALIDATE_SINGLE_WITHDRAWAL_AMOUNT
	INVALIDATE_SINGLE_WITHDRAWAL_VALIDATOR_INDEX
	INVALIDATE_SINGLE_WITHDRAWAL_INDEX
	VALID_WITHDRAWALS_INVALID_STATE_ROOT
	TIMEOUT
)

var REQUIRES_FINALIZATION_TO_ACTIVATE_BUILDER = []string{
	"lighthouse",
	"teku",
}

type BuilderWithdrawalsTestSpec struct {
	BaseWithdrawalsTestSpec
	BuilderTestError BuilderTestError
}

func (ts BuilderWithdrawalsTestSpec) GetTestnetConfig(
	allNodeDefinitions clients.NodeDefinitions,
) *testnet.Config {
	tc := ts.BaseWithdrawalsTestSpec.GetTestnetConfig(allNodeDefinitions)

	tc.CapellaForkEpoch = big.NewInt(2)

	if len(
		allNodeDefinitions.FilterByCL(
			REQUIRES_FINALIZATION_TO_ACTIVATE_BUILDER,
		),
	) > 0 {
		// At least one of the CLs require finalization to start requesting
		// headers from the builder
		tc.CapellaForkEpoch = big.NewInt(5)
	}

	// Builders are always enabled for these tests
	tc.EnableBuilders = true

	return tc
}
