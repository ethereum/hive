package setup

import (
	"crypto/sha256"
	"fmt"
	"github.com/protolambda/zrnt/eth2/beacon"
)

func BuildPhase0State(spec *beacon.Spec, keys []*KeyDetails) (*beacon.BeaconStateView, error) {
	kickstartValidators := make([]beacon.KickstartValidatorData, 0, len(keys))
	hasher := sha256.New()
	withdrawalCred := func(k beacon.BLSPubkey) (out beacon.Root) {
		hasher.Reset()
		hasher.Write(k[:])
		dat := hasher.Sum(nil)
		copy(out[:], dat)
		out[0] = spec.BLS_WITHDRAWAL_PREFIX[0]
		return
	}
	for _, key := range keys {
		kickstartValidators = append(kickstartValidators, beacon.KickstartValidatorData{
			Pubkey:                key.ValidatorPubkey,
			WithdrawalCredentials: withdrawalCred(key.WithdrawalPubkey),
			Balance:               spec.MAX_EFFECTIVE_BALANCE,
		})
	}
	// set genesis time to 0, we override it later as needed.
	state, _, err := spec.KickStartState(beacon.Root{0: 0x42}, 0, kickstartValidators)
	if err != nil {
		return nil, fmt.Errorf("failed to create genesis beacon state: %v", err)
	}
	return state, nil
}
