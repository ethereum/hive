package setup

import (
	"crypto/sha256"
	"fmt"
	"github.com/protolambda/zrnt/eth2/beacon/common"
	"github.com/protolambda/zrnt/eth2/beacon/phase0"
)

func BuildPhase0State(spec *common.Spec, keys []*KeyDetails) (common.BeaconState, error) {
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
	// set genesis time to 0, we override it later as needed.
	state, _, err := phase0.KickStartState(spec, common.Root{0: 0x42}, 0, kickstartValidators)
	if err != nil {
		return nil, fmt.Errorf("failed to create genesis common state: %v", err)
	}
	return state, nil
}
