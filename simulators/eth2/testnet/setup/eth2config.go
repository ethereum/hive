package setup

import (
	"fmt"
	"github.com/ethereum/hive/hivesim"
	"github.com/protolambda/zrnt/eth2/beacon/common"
)

func Eth2ConfigToParams(config *common.Config) hivesim.Params {
	u64 := func(v uint64) string { return fmt.Sprintf("%d", v) }
	out := hivesim.Params{
		"PRESET_BASE": config.PRESET_BASE,

		// Genesis.
		"MIN_GENESIS_ACTIVE_VALIDATOR_COUNT": u64(config.MIN_GENESIS_ACTIVE_VALIDATOR_COUNT),
		"MIN_GENESIS_TIME":                   config.MIN_GENESIS_TIME.String(),
		"GENESIS_FORK_VERSION":               config.GENESIS_FORK_VERSION.String(),
		"GENESIS_DELAY":                      config.GENESIS_DELAY.String(),

		// Altair
		"ALTAIR_FORK_VERSION": config.ALTAIR_FORK_VERSION.String(),
		"ALTAIR_FORK_EPOCH":   config.ALTAIR_FORK_EPOCH.String(),

		// Merge
		"MERGE_FORK_VERSION": config.MERGE_FORK_VERSION.String(),
		"MERGE_FORK_EPOCH":   config.MERGE_FORK_EPOCH.String(),

		// Merge transition
		"MIN_ANCHOR_POW_BLOCK_DIFFICULTY": u64(config.MIN_ANCHOR_POW_BLOCK_DIFFICULTY),

		// Time parameters
		"SECONDS_PER_SLOT":                    config.SECONDS_PER_SLOT.String(),
		"SECONDS_PER_ETH1_BLOCK":              u64(config.SECONDS_PER_ETH1_BLOCK),
		"MIN_VALIDATOR_WITHDRAWABILITY_DELAY": config.MIN_VALIDATOR_WITHDRAWABILITY_DELAY.String(),
		"SHARD_COMMITTEE_PERIOD":              config.SHARD_COMMITTEE_PERIOD.String(),
		"ETH1_FOLLOW_DISTANCE":                u64(config.ETH1_FOLLOW_DISTANCE),

		// Validator cycle
		"INACTIVITY_SCORE_BIAS":          u64(config.INACTIVITY_SCORE_BIAS),
		"INACTIVITY_SCORE_RECOVERY_RATE": u64(config.INACTIVITY_SCORE_RECOVERY_RATE),
		"EJECTION_BALANCE":               config.EJECTION_BALANCE.String(),
		"MIN_PER_EPOCH_CHURN_LIMIT":      u64(config.MIN_PER_EPOCH_CHURN_LIMIT),
		"CHURN_LIMIT_QUOTIENT":           u64(config.CHURN_LIMIT_QUOTIENT),

		// Deposit contract
		"DEPOSIT_CHAIN_ID":         u64(config.DEPOSIT_CHAIN_ID),
		"DEPOSIT_NETWORK_ID":       u64(config.DEPOSIT_NETWORK_ID),
		"DEPOSIT_CONTRACT_ADDRESS": config.DEPOSIT_CONTRACT_ADDRESS.String(),
	}
	// prefix everything
	for k, v := range out {
		delete(out, k)
		out["HIVE_ETH2_CONFIG_"+k] = v
	}
	return out
}
