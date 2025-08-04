package utils_test

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/hive/simulators/eth2/common/config"
	cl "github.com/ethereum/hive/simulators/eth2/common/config/consensus"
	el "github.com/ethereum/hive/simulators/eth2/common/config/execution"
	"github.com/ethereum/hive/simulators/eth2/common/testnet"
	"github.com/ethereum/hive/simulators/eth2/common/utils"
)

var mnemonic = "couple kiwi radio river setup fortune hunt grief buddy forward perfect empty slim wear bounce drift execute nation tobacco dutch chapter festival ice fog"

func TestValidatorKeys(t *testing.T) {
	validatorCount := uint64(1024)
	// Prepare a testnet
	keySrc := &cl.MnemonicsKeySource{
		From:     0,
		To:       validatorCount,
		Mnemonic: mnemonic,
	}
	keys, err := keySrc.Keys()
	if err != nil {
		t.Fatalf("failed to generate keys: %v", err)
	}

	env := &testnet.Environment{
		Validators: keys,
	}
	config := &testnet.Config{
		ConsensusConfig: &cl.ConsensusConfig{
			ValidatorCount: new(big.Int).SetUint64(validatorCount),
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

	p, err := testnet.PrepareTestnet(env, config)
	if err != nil {
		t.Fatalf("failed to prepare testnet: %v", err)
	}

	validators, err := utils.NewValidators(p.Spec, p.BeaconGenesis, p.ValidatorsSetupDetails.KeysMap(0))
	if err != nil {
		t.Fatalf("failed to update validators: %v", err)
	}

	if validators.Count() != int(validatorCount) {
		t.Fatalf("invalid validator count: %d", validators.Count())
	}

	chunked := validators.Chunks(3)
	if len(chunked) != 3 {
		t.Fatalf("invalid chunk count: %d", len(chunked))
	}
	if chunked[0].Count() != 342 {
		t.Fatalf("invalid chunk count: %d", chunked[0].Count())
	}
	if chunked[1].Count() != 341 {
		t.Fatalf("invalid chunk count: %d", chunked[1].Count())
	}
	if chunked[2].Count() != 341 {
		t.Fatalf("invalid chunk count: %d", chunked[2].Count())
	}
	if validators.Exited().Count() != 0 {
		t.Fatalf("invalid exited validator count: %d", validators.Exited().Count())
	}
}
