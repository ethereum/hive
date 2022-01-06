package setup

import (
	"crypto/sha256"
	"fmt"
	"github.com/protolambda/zrnt/eth2/beacon/common"
	"github.com/protolambda/zrnt/eth2/beacon/phase0"
	"time"
)

// TODO: build merge state
func BuildBeaconState(eth1Genesis *Eth1Genesis, spec *common.Spec, keys []*KeyDetails) (common.BeaconState, error) {
	kickstartValidators := make([]phase0.KickstartValidatorData, 0, len(keys))
	hasher := sha256.New()
	withdrawalCred := func(k common.BLSPubkey) (out common.Root) {
		hasher.Reset()
		hasher.Write(k[:])
		dat := hasher.Sum(nil)
		copy(out[:], dat)
		out[0] = common.BLS_WITHDRAWAL_PREFIX
		return
	}
	for _, key := range keys {
		kickstartValidators = append(kickstartValidators, phase0.KickstartValidatorData{
			Pubkey:                key.ValidatorPubkey,
			WithdrawalCredentials: withdrawalCred(key.WithdrawalPubkey),
			Balance:               spec.MAX_EFFECTIVE_BALANCE,
		})
	}
	// TODO: if building a Post-Merge genesis, then initialize the latest-block header
	// in the state with the genesis block of execution layer.

	// genesis 1 min from now
	genesisTime := common.Timestamp(time.Now().Add(time.Minute).Unix())
	state, _, err := phase0.KickStartState(spec, common.Root{0: 0x42}, genesisTime, kickstartValidators)
	if err != nil {
		return nil, fmt.Errorf("failed to create genesis common state: %v", err)
	}
	return state, nil
}
