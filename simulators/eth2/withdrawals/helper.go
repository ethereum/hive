package main

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/hive/simulators/eth2/common/clients"
	cl "github.com/ethereum/hive/simulators/eth2/common/config/consensus"
	"github.com/ethereum/hive/simulators/eth2/common/testnet"
	blsu "github.com/protolambda/bls12-381-util"
	beacon "github.com/protolambda/zrnt/eth2/beacon/common"
	"github.com/protolambda/ztyp/tree"
)

// API call names
const (
	EngineForkchoiceUpdatedV1 = "engine_forkchoiceUpdatedV1"
	EngineGetPayloadV1        = "engine_getPayloadV1"
	EngineNewPayloadV1        = "engine_newPayloadV1"
	EthGetBlockByHash         = "eth_getBlockByHash"
	EthGetBlockByNumber       = "eth_getBlockByNumber"
)

// Engine API Types

type PayloadStatus string

const (
	Unknown          = ""
	Valid            = "VALID"
	Invalid          = "INVALID"
	Accepted         = "ACCEPTED"
	Syncing          = "SYNCING"
	InvalidBlockHash = "INVALID_BLOCK_HASH"
)

// Signer for all txs
type Signer struct {
	ChainID    *big.Int
	PrivateKey *ecdsa.PrivateKey
}

func (vs Signer) SignTx(
	baseTx *types.Transaction,
) (*types.Transaction, error) {
	signer := types.NewEIP155Signer(vs.ChainID)
	return types.SignTx(baseTx, signer, vs.PrivateKey)
}

var VaultSigner = Signer{
	ChainID:    CHAIN_ID,
	PrivateKey: VAULT_KEY,
}

// Helper struct to keep track of current status of a validator withdrawal state
type Validator struct {
	Index                      beacon.ValidatorIndex
	WithdrawAddress            *common.Address
	Exited                     bool
	ExitCondition              string
	ExactWithdrawableBalance   *big.Int
	Keys                       *cl.KeyDetails
	BLSToExecutionChangeDomain *beacon.BLSDomain
	Verified                   bool
}

func (v *Validator) VerifyWithdrawnBalance(
	ctx context.Context,
	ec *clients.ExecutionClient,
) (bool, error) {
	if v.Verified {
		// Validator already verified on a previous iteration
		return true, nil
	}
	// Check the withdrawal address, if empty this is an error
	if v.WithdrawAddress == nil {
		return false, fmt.Errorf(
			"checked balance for validator without a withdrawal address",
		)
	}
	execAddress := *v.WithdrawAddress

	// First get the balance
	balance, err := ec.BalanceAt(ctx, execAddress, nil)
	if err != nil {
		return false, err
	}

	// If balance is zero, there have not been any withdrawals yet,
	// but this is not an error
	if balance.Cmp(common.Big0) == 0 {
		return false, nil
	}

	// If we have an exact withdrawal expected balance, then verify it
	if v.ExactWithdrawableBalance != nil {
		if v.ExactWithdrawableBalance.Cmp(balance) == 0 {
			fmt.Printf(
				"INFO: %s validator %d fully withdrawn: %d\n",
				v.ExitCondition,
				v.Index,
				v.ExactWithdrawableBalance,
			)
			v.Verified = true
			return true, nil
		} else {
			return true, fmt.Errorf("unexepected balance: want=%d, got=%d", v.ExactWithdrawableBalance, balance)
		}
	}

	// Otherwise simply return true, as a signal that a - potentially partial - withdrawal has taken place
	fmt.Printf(
		"INFO: Validator %d partially withdrawn: %d\n",
		v.Index,
		balance,
	)
	v.Verified = true
	return true, nil
}

// Signs the BLS-to-execution-change for the given address
func (v *Validator) SignBLSToExecutionChange(
	executionAddress common.Address,
) (*beacon.SignedBLSToExecutionChange, error) {
	if v.Keys == nil {
		return nil, fmt.Errorf("no key to sign")
	}
	if v.BLSToExecutionChangeDomain == nil {
		return nil, fmt.Errorf("no domain to sign")
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
		*v.BLSToExecutionChangeDomain,
	)
	sk := new(blsu.SecretKey)
	sk.Deserialize(&v.Keys.WithdrawalSecretKey)
	signature := blsu.Sign(sk, sigRoot[:]).Serialize()
	return &beacon.SignedBLSToExecutionChange{
		BLSToExecutionChange: blsToExecChange,
		Signature:            beacon.BLSSignature(signature),
	}, nil
}

// Sign and send the BLS-to-execution-change.
// Also internally update the withdraw address.
func (v *Validator) SignSendBLSToExecutionChange(
	ctx context.Context,
	bc *clients.BeaconClient,
	executionAddress common.Address,
) error {
	signedBLS, err := v.SignBLSToExecutionChange(executionAddress)
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

type Validators []*Validator

// Verify all validators have withdrawn
func (vs Validators) VerifyWithdrawnBalance(
	ctx context.Context,
	ec *clients.ExecutionClient,
) (bool, error) {
	for i, v := range vs {
		if withdrawn, err := v.VerifyWithdrawnBalance(ctx, ec); err != nil {
			return withdrawn, fmt.Errorf(
				"error verifying validator %d balance: %v",
				i,
				err,
			)
		} else if !withdrawn {
			return false, nil
		}
	}
	return true, nil
}

func (vs Validators) NonWithdrawable() Validators {
	ret := make(Validators, 0)
	for _, v := range vs {
		v := v
		if v.WithdrawAddress == nil {
			ret = append(ret, v)
		}
	}
	return ret
}

func (vs Validators) Withdrawable() Validators {
	ret := make(Validators, 0)
	for _, v := range vs {
		v := v
		if v.WithdrawAddress != nil {
			ret = append(ret, v)
		}
	}
	return ret
}

func (vs Validators) FullyWithdrawable() Validators {
	ret := make(Validators, 0)
	for _, v := range vs {
		v := v
		if v.WithdrawAddress != nil && v.Exited {
			ret = append(ret, v)
		}
	}
	return ret
}

func (vs Validators) Exited() Validators {
	ret := make(Validators, 0)
	for _, v := range vs {
		v := v
		if v.Exited {
			ret = append(ret, v)
		}
	}
	return ret
}

func (vs Validators) Chunks(totalShares int) []Validators {
	ret := make([]Validators, totalShares)
	countPerChunk := len(vs) / totalShares
	for i := range ret {
		ret[i] = vs[i*countPerChunk : (i*countPerChunk)+countPerChunk]
	}
	return ret
}

func ValidatorFromBeaconValidator(
	index beacon.ValidatorIndex,
	source beacon.Validator,
	balance beacon.Gwei,
	keys *cl.KeyDetails,
	domain *beacon.BLSDomain,
) (*Validator, error) {
	// Assume genesis state
	currentEpoch := beacon.Epoch(0)

	v := new(Validator)

	v.Index = index
	v.Keys = keys
	v.BLSToExecutionChangeDomain = domain

	wc, err := source.WithdrawalCredentials()
	if err != nil {
		return nil, err
	}
	if wc[0] == beacon.ETH1_ADDRESS_WITHDRAWAL_PREFIX {
		withdrawAddress := common.Address{}
		copy(withdrawAddress[:], wc[12:])
		v.WithdrawAddress = &withdrawAddress
	}

	exitEpoch, err := source.ExitEpoch()
	if err != nil {
		return nil, err
	}

	slashed, err := source.Slashed()
	if err != nil {
		return nil, err
	}

	// Assuming this is the genesis beacon state
	if exitEpoch <= currentEpoch || slashed {
		v.Exited = true
		if slashed {
			v.ExitCondition = "Slashed"
		} else {
			v.ExitCondition = "Voluntary Exited"
		}
		v.ExactWithdrawableBalance = big.NewInt(int64(balance))
		v.ExactWithdrawableBalance.Mul(
			v.ExactWithdrawableBalance,
			big.NewInt(1e9),
		)
	}
	return v, nil
}

func ValidatorFromBeaconState(
	state beacon.BeaconState,
	index beacon.ValidatorIndex,
	keys *cl.KeyDetails,
	domain *beacon.BLSDomain,
) (*Validator, error) {
	stateVals, err := state.Validators()
	if err != nil {
		return nil, err
	}
	balances, err := state.Balances()
	if err != nil {
		return nil, err
	}
	beaconVal, err := stateVals.Validator(index)
	if err != nil {
		return nil, err
	}
	balance, err := balances.GetBalance(index)
	if err != nil {
		return nil, err
	}
	return ValidatorFromBeaconValidator(
		index,
		beaconVal,
		balance,
		keys,
		domain,
	)
}

func ValidatorsFromBeaconState(
	state beacon.BeaconState,
	keys []*cl.KeyDetails,
	domain *beacon.BLSDomain,
) (Validators, error) {
	stateVals, err := state.Validators()
	if err != nil {
		return nil, err
	}
	balances, err := state.Balances()
	if err != nil {
		return nil, err
	}
	validatorCount, err := stateVals.ValidatorCount()
	if err != nil {
		return nil, err
	} else if validatorCount == 0 {
		return nil, fmt.Errorf("got zero validators")
	} else if validatorCount != uint64(len(keys)) {
		return nil, fmt.Errorf("incorrect amount of keys: want=%d, got=%d", validatorCount, len(keys))
	}
	validators := make(Validators, 0)
	for i := beacon.ValidatorIndex(0); i < beacon.ValidatorIndex(validatorCount); i++ {
		beaconVal, err := stateVals.Validator(beacon.ValidatorIndex(i))
		if err != nil {
			return nil, err
		}
		balance, err := balances.GetBalance(i)
		if err != nil {
			return nil, err
		}
		validator, err := ValidatorFromBeaconValidator(
			i,
			beaconVal,
			balance,
			keys[i],
			domain,
		)
		if err != nil {
			return nil, err
		}
		validators = append(validators, validator)

	}
	return validators, nil
}

func ComputeBLSToExecutionDomain(
	t *testnet.Testnet,
) beacon.BLSDomain {
	return beacon.ComputeDomain(
		beacon.DOMAIN_BLS_TO_EXECUTION_CHANGE,
		t.Spec().GENESIS_FORK_VERSION,
		t.GenesisValidatorsRoot(),
	)
}
