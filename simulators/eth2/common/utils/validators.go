// Utilities and methods to manage validators during the test cycle
package utils

import (
	"bytes"
	"context"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	beacon_client "github.com/marioevz/eth-clients/clients/beacon"
	exec_client "github.com/marioevz/eth-clients/clients/execution"
	"github.com/marioevz/eth-clients/clients/validator"
	"github.com/pkg/errors"
	blsu "github.com/protolambda/bls12-381-util"
	beacon "github.com/protolambda/zrnt/eth2/beacon/common"
	"github.com/protolambda/zrnt/eth2/beacon/phase0"
	"github.com/protolambda/ztyp/tree"
)

// Helper struct to keep track of current status of a validator withdrawal state
type Validator struct {
	Spec                     *beacon.Spec
	Index                    beacon.ValidatorIndex
	PubKey                   *beacon.BLSPubkey
	WithdrawAddress          *common.Address
	Exited                   bool
	ExitInitiated            bool
	Slashed                  bool
	ExactWithdrawableBalance *big.Int
	Keys                     *validator.ValidatorKeys
	InitialBalance           beacon.Gwei
	Balance                  beacon.Gwei
	BlockStateCache          BeaconCache
}

// Signs the BLS-to-execution-change for the given address
func (v *Validator) SignBLSToExecutionChange(
	executionAddress common.Address,
	blsToExecutionChangeDomain beacon.BLSDomain,
) (*beacon.SignedBLSToExecutionChange, error) {
	if v.Keys == nil {
		return nil, fmt.Errorf("no key to sign")
	}
	if v.WithdrawAddress != nil {
		return nil, fmt.Errorf("execution address already set")
	}
	kdPubKey := beacon.BLSPubkey{}
	copy(kdPubKey[:], v.Keys.WithdrawalPubkey[:])
	eth1Address := beacon.Eth1Address{}
	copy(eth1Address[:], executionAddress[:])
	blsToExecChange := beacon.BLSToExecutionChange{
		ValidatorIndex:     v.Index,
		FromBLSPubKey:      kdPubKey,
		ToExecutionAddress: eth1Address,
	}
	sigRoot := beacon.ComputeSigningRoot(
		blsToExecChange.HashTreeRoot(tree.GetHashFn()),
		blsToExecutionChangeDomain,
	)
	sk := new(blsu.SecretKey)
	sk.Deserialize(&v.Keys.WithdrawalSecretKey)
	signature := blsu.Sign(sk, sigRoot[:]).Serialize()
	return &beacon.SignedBLSToExecutionChange{
		BLSToExecutionChange: blsToExecChange,
		Signature:            beacon.BLSSignature(signature),
	}, nil
}

func (v *Validator) SignVoluntaryExit(
	voluntaryExitDomain beacon.BLSDomain,
) (*phase0.SignedVoluntaryExit, error) {
	if v.Keys == nil {
		return nil, fmt.Errorf("no key to sign")
	}
	voluntaryExit := phase0.VoluntaryExit{
		ValidatorIndex: v.Index,
	}
	sigRoot := beacon.ComputeSigningRoot(
		voluntaryExit.HashTreeRoot(tree.GetHashFn()),
		voluntaryExitDomain,
	)
	sk := new(blsu.SecretKey)
	sk.Deserialize(&v.Keys.ValidatorSecretKey)
	signature := blsu.Sign(sk, sigRoot[:]).Serialize()
	return &phase0.SignedVoluntaryExit{
		Message:   voluntaryExit,
		Signature: beacon.BLSSignature(signature),
	}, nil
}

// Sign and send the BLS-to-execution-change.
// Also internally update the withdraw address.
func (v *Validator) SignSendBLSToExecutionChange(
	ctx context.Context,
	bc *beacon_client.BeaconClient,
	executionAddress common.Address,
	blsToExecutionChangeDomain beacon.BLSDomain,
) error {
	signedBLS, err := v.SignBLSToExecutionChange(executionAddress, blsToExecutionChangeDomain)
	if err != nil {
		return err
	}
	if err := bc.SubmitPoolBLSToExecutionChange(ctx, beacon.SignedBLSToExecutionChanges{
		*signedBLS,
	}); err != nil {
		return err
	}

	v.WithdrawAddress = &executionAddress
	return nil
}

// Sign and send the voluntary exit.
func (v *Validator) SignSendVoluntaryExit(
	ctx context.Context,
	bc *beacon_client.BeaconClient,
	voluntaryExitDomain beacon.BLSDomain,
) error {
	signedVoluntaryExit, err := v.SignVoluntaryExit(voluntaryExitDomain)
	if err != nil {
		return err
	}
	return bc.SubmitVoluntaryExit(ctx, signedVoluntaryExit)
}

// Update the validator status from a beacon validator
func (v *Validator) updateFromBeaconValidator(
	source beacon.Validator,
	currentEpoch beacon.Epoch,
	balance beacon.Gwei,
) error {
	wc, err := source.WithdrawalCredentials()
	if err != nil {
		return err
	}
	if wc[0] == beacon.ETH1_ADDRESS_WITHDRAWAL_PREFIX {
		withdrawAddress := common.Address{}
		copy(withdrawAddress[:], wc[12:])
		v.WithdrawAddress = &withdrawAddress
	}

	exitEpoch, err := source.ExitEpoch()
	if err != nil {
		return err
	}
	exitInitiated := exitEpoch != beacon.FAR_FUTURE_EPOCH
	exited := exitEpoch <= currentEpoch

	slashed, err := source.Slashed()
	if err != nil {
		return err
	}

	// If the validator just changed its state to exited, we can calculate the exact withdrawable balance (most of the time)
	if exited && !v.Exited {
		v.ExactWithdrawableBalance = big.NewInt(int64(balance))
		v.ExactWithdrawableBalance.Mul(
			v.ExactWithdrawableBalance,
			big.NewInt(1e9),
		)
	}

	v.Exited = exited
	v.ExitInitiated = exitInitiated
	v.Slashed = slashed
	v.Balance = balance

	return nil
}

// Verifications
func WithdrawalsContainValidator(
	ws []*types.Withdrawal,
	vId beacon.ValidatorIndex,
) bool {
	for _, w := range ws {
		if w.Validator == uint64(vId) {
			return true
		}
	}
	return false
}

func (v *Validator) VerifyWithdrawnBalance(
	ctx context.Context,
	bc *beacon_client.BeaconClient,
	ec *exec_client.ExecutionClient,
	headBlockRoot tree.Root,
) (bool, error) {
	// Check the withdrawal address, if empty this is an error
	if v.WithdrawAddress == nil {
		return false, fmt.Errorf(
			"checked balance for validator without a withdrawal address",
		)
	}
	execAddress := *v.WithdrawAddress

	// First get the head block
	headBlockState, err := v.BlockStateCache.GetBlockStateByRoot(
		ctx,
		bc,
		headBlockRoot,
	)
	if err != nil {
		return false, errors.Wrap(err, "failed to get head block state")
	}
	fmt.Printf(
		"INFO: Verifying balance validator %d on slot %d\n",
		v.Index,
		headBlockState.Slot(),
	)

	// Then get the balance
	execPayload, _, _, err := headBlockState.ExecutionPayload()
	if err != nil {
		return false, errors.Wrap(
			err,
			"failed to get execution payload from head",
		)
	}
	balance, err := ec.BalanceAt(
		ctx,
		execAddress,
		big.NewInt(int64(execPayload.Number)),
	)
	if err != nil {
		return false, errors.Wrap(err, "failed to get balance")
	}

	fmt.Printf(
		"INFO: Balance of validator %d in the execution chain (block %d): %d\n",
		v.Index,
		execPayload.Number,
		balance,
	)

	// If balance is zero, there have not been any withdrawals yet,
	// but this is not an error
	if balance.Cmp(common.Big0) == 0 {
		return false, nil
	}

	// If we have an exact withdrawal expected balance, then verify it
	if v.ExactWithdrawableBalance != nil {
		if v.ExactWithdrawableBalance.Cmp(balance) == 0 {
			exitCondition := "Exited"
			if v.Slashed {
				exitCondition = "Slashed"
			}
			fmt.Printf(
				"INFO: %s validator %d fully withdrawn: %d\n",
				exitCondition,
				v.Index,
				v.ExactWithdrawableBalance,
			)
			return true, nil
		} else {
			return true, fmt.Errorf("unexepected balance: want=%d, got=%d", v.ExactWithdrawableBalance, balance)
		}
	} else {
		// We need to traverse the beacon state history to be able to compute
		// the expected partial balance withdrawn
		previousBalance := v.InitialBalance
		expectedPartialWithdrawnBalance := beacon.Gwei(0)
		for slot := beacon.Slot(0); slot <= headBlockState.Slot(); slot++ {
			blockState, err := v.BlockStateCache.GetBlockStateBySlotFromHeadRoot(ctx, bc, headBlockRoot, slot)
			if err != nil {
				return false, errors.Wrapf(err, "failed to get block state, slot %d", slot)
			}
			if blockState == nil {
				// Probably a skipped slot
				continue
			}

			execPayload, _, _, err := blockState.ExecutionPayload()
			if err != nil {
				return false, errors.Wrapf(err, "failed to get execution payload, slot %d", slot)
			}

			if WithdrawalsContainValidator(execPayload.Withdrawals, v.Index) {
				expectedPartialWithdrawnBalance += previousBalance - (v.Spec.MAX_EFFECTIVE_BALANCE)
			}

			previousBalance = blockState.Balance(v.Index)
		}

		if expectedPartialWithdrawnBalance != 0 {
			expectedBalanceWei := new(big.Int).SetUint64(uint64(expectedPartialWithdrawnBalance))
			expectedBalanceWei.Mul(expectedBalanceWei, big.NewInt(1e9))
			if balance.Cmp(expectedBalanceWei) == 0 {
				fmt.Printf(
					"INFO: Validator %d partially withdrawn: %d\n",
					v.Index,
					balance,
				)
				return true, nil
			} else {
				return true, fmt.Errorf("unexepected balance: want=%d, got=%d", expectedBalanceWei, balance)
			}

		} else {
			fmt.Printf(
				"INFO: Validator %d expected withdraw balance is zero\n",
				v.Index,
			)
		}

	}
	return false, nil
}

type Validators struct {
	vs     map[beacon.ValidatorIndex]*Validator
	spec   *beacon.Spec
	subset bool
	BeaconCache
	Keys map[beacon.ValidatorIndex]*validator.ValidatorKeys
}

func NewValidators(spec *beacon.Spec, state beacon.BeaconState, keys map[beacon.ValidatorIndex]*validator.ValidatorKeys) (*Validators, error) {
	vs := newValidators(spec, make(BeaconCache), keys, false)
	if err := vs.updateFromBeaconState(state, true); err != nil {
		return nil, err
	}
	// Set initial balance
	for _, v := range vs.vs {
		v.InitialBalance = v.Balance
	}
	return vs, nil
}

func newValidators(spec *beacon.Spec, beaconCache BeaconCache, keys map[beacon.ValidatorIndex]*validator.ValidatorKeys, subset bool) *Validators {
	return &Validators{
		vs:          make(map[beacon.ValidatorIndex]*Validator),
		spec:        spec,
		BeaconCache: beaconCache,
		Keys:        keys,
		subset:      subset,
	}
}

func (vs *Validators) GetValidatorByIndex(i beacon.ValidatorIndex) *Validator {
	for _, v := range vs.vs {
		if v.Index == i {
			return v
		}
	}
	return nil
}

func (vs *Validators) Count() int {
	return len(vs.vs)
}

func (vs *Validators) ForEach(f func(*Validator)) {
	for _, v := range vs.vs {
		f(v)
	}
}

func (vs *Validators) Subset(incl func(*Validator) bool) *Validators {
	ret := newValidators(vs.spec, vs.BeaconCache, vs.Keys, true)
	for i, v := range vs.vs {
		if incl(v) {
			ret.vs[i] = v
		}
	}
	return ret
}

func (vs *Validators) NonWithdrawable() *Validators {
	return vs.Subset(func(v *Validator) bool {
		return v.WithdrawAddress == nil
	})
}

func (vs *Validators) Withdrawable() *Validators {
	return vs.Subset(func(v *Validator) bool {
		return v.WithdrawAddress != nil
	})
}

func (vs *Validators) FullyWithdrawable() *Validators {
	return vs.Subset(func(v *Validator) bool {
		return v.WithdrawAddress != nil && v.Exited
	})
}

func (vs *Validators) Active() *Validators {
	return vs.Subset(func(v *Validator) bool {
		return !v.Exited && !v.Slashed
	})
}

func (vs *Validators) Slashed() *Validators {
	return vs.Subset(func(v *Validator) bool {
		return v.Slashed
	})
}

func (vs *Validators) Exited() *Validators {
	return vs.Subset(func(v *Validator) bool {
		return v.Exited
	})
}

func (vs *Validators) ExitInitiated() *Validators {
	return vs.Subset(func(v *Validator) bool {
		return v.ExitInitiated
	})
}

func (vs *Validators) Chunks(totalShares int) []*Validators {
	ret := make([]*Validators, totalShares)
	for i := range ret {
		ret[i] = newValidators(vs.spec, vs.BeaconCache, vs.Keys, true)
	}
	i := 0
	for _, v := range vs.vs {
		ret[i%totalShares].vs[v.Index] = v
		i++
	}
	return ret
}

func (vs *Validators) SelectByIndex(indexes ...beacon.ValidatorIndex) *Validators {
	return vs.Subset(func(v *Validator) bool {
		for _, i := range indexes {
			if v.Index == i {
				return true
			}
		}
		return false
	})
}

func (vs *Validators) SelectByCount(count int) *Validators {
	return vs.Subset(func(v *Validator) bool {
		if count > 0 {
			count--
			return true
		}
		return false
	})
}

func (vs *Validators) UpdateFromBeaconState(
	state beacon.BeaconState,
) error {
	return vs.updateFromBeaconState(state, !vs.subset)
}

func (vs *Validators) updateFromBeaconState(
	state beacon.BeaconState,
	appendNew bool,
) error {
	slot, err := state.Slot()
	if err != nil {
		return err
	}
	currentEpoch := vs.spec.SlotToEpoch(slot)

	stateVals, err := state.Validators()
	if err != nil {
		return err
	}
	balances, err := state.Balances()
	if err != nil {
		return err
	}
	validatorCount, err := stateVals.ValidatorCount()
	if err != nil {
		return err
	} else if validatorCount == 0 {
		return fmt.Errorf("got zero validators")
	} else if validatorCount != uint64(len(vs.Keys)) {
		return fmt.Errorf("incorrect amount of keys: want=%d, got=%d", validatorCount, len(vs.Keys))
	}
	for i := beacon.ValidatorIndex(0); i < beacon.ValidatorIndex(validatorCount); i++ {
		beaconVal, err := stateVals.Validator(beacon.ValidatorIndex(i))
		if err != nil {
			return err
		}
		balance, err := balances.GetBalance(i)
		if err != nil {
			return err
		}
		// Compare pub keys against the key we have
		pubKey, err := beaconVal.Pubkey()
		if err != nil {
			return errors.Wrapf(err, "failed to get pubkey for validator %d", i)
		}

		v := vs.vs[i]
		if v == nil && appendNew {
			// Check keys
			keys := vs.Keys[i]
			if keys == nil {
				return fmt.Errorf("no key for validator %d", i)
			}

			if !bytes.Equal(keys.ValidatorPubkey[:], pubKey[:]) {
				return fmt.Errorf("pubkey mismatch for validator %d", i)
			}

			v = &Validator{
				Index:           i,
				Keys:            keys,
				BlockStateCache: vs.BeaconCache,
				PubKey:          &pubKey,
			}
			vs.vs[i] = v
		}

		if v != nil {
			if err := v.updateFromBeaconValidator(
				beaconVal,
				currentEpoch,
				balance,
			); err != nil {
				return err
			}
		}
	}
	return nil
}
