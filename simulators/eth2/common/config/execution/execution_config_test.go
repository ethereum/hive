package execution_config

import (
	"math/big"
	"testing"

	"github.com/ethereum/hive/simulators/eth2/common/config"
)

func TestBuildChainConfig(t *testing.T) {
	slotsPerEpoch := uint64(32)
	slotTime := uint64(12)
	beaconChainGenesisTime := uint64(1634025600)
	ttd := big.NewInt(200)
	chainConfig, err := BuildChainConfig(ttd, beaconChainGenesisTime, slotsPerEpoch, slotTime, &config.ForkConfig{
		AltairForkEpoch:    big.NewInt(0),
		BellatrixForkEpoch: big.NewInt(200),
		CapellaForkEpoch:   nil,
		DenebForkEpoch:     nil,
	})
	if err != nil {
		t.Fatalf("Error producing chainConfig: %v", err)
	}
	if chainConfig.ShanghaiTime != nil {
		t.Fatal("ShanghaiTime is not nil")
	}
	if chainConfig.CancunTime != nil {
		t.Fatal("CancunTime is not nil")
	}
	if chainConfig.TerminalTotalDifficulty.Cmp(ttd) != 0 {
		t.Fatalf("Incorrect TerminalTotalDifficulty is not %d", ttd)
	}

	// Shanghai Chain Config Test
	ttd = big.NewInt(0)
	chainConfig, err = BuildChainConfig(ttd, beaconChainGenesisTime, slotsPerEpoch, slotTime, &config.ForkConfig{
		AltairForkEpoch:    big.NewInt(0),
		BellatrixForkEpoch: big.NewInt(200),
		CapellaForkEpoch:   big.NewInt(200),
		DenebForkEpoch:     nil,
	})
	if err != nil {
		t.Fatalf("Error producing shanghaiChainConfig: %v", err)
	}
	if chainConfig.ShanghaiTime == nil {
		t.Fatal("ShanghaiTime is nil")
	}
	if *chainConfig.ShanghaiTime != beaconChainGenesisTime+(slotsPerEpoch*slotTime*200) {
		t.Fatalf("Incorrect ShanghaiTime is not %d", beaconChainGenesisTime+(slotsPerEpoch*slotTime))
	}
	if chainConfig.CancunTime != nil {
		t.Fatal("CancunTime is not nil")
	}
	if chainConfig.TerminalTotalDifficulty.Cmp(ttd) != 0 {
		t.Fatalf("Incorrect TerminalTotalDifficulty is not %d", ttd)
	}

	// Incorrectly configured epoch
	ttd = big.NewInt(0)
	chainConfig, err = BuildChainConfig(ttd, beaconChainGenesisTime, slotsPerEpoch, slotTime, &config.ForkConfig{
		AltairForkEpoch:    big.NewInt(0),
		BellatrixForkEpoch: big.NewInt(200),
		CapellaForkEpoch:   big.NewInt(199),
		DenebForkEpoch:     nil,
	})
	if err == nil {
		t.Fatalf("Expected error producing chainConfig")
	}
}
