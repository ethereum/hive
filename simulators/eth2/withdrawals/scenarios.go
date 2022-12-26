package main

import (
	"bytes"
	"context"
	"encoding/json"
	"time"

	"github.com/ethereum/hive/hivesim"
	"github.com/ethereum/hive/simulators/eth2/common/clients"
	tn "github.com/ethereum/hive/simulators/eth2/common/testnet"
	"github.com/protolambda/eth2api"
	beacon "github.com/protolambda/zrnt/eth2/beacon/common"
)

func TestCapellaPartialWithdrawalsWithoutBLSChanges(
	t *hivesim.T, env *tn.Environment,
	config *tn.Config,
) {
	ctx := context.Background()

	// config.CapellaForkEpoch = common.Big0
	testnet := tn.StartTestnet(ctx, t, env, config)
	defer testnet.Stop()

	// Wait for beacon chain genesis to happen
	testnet.WaitForGenesis(ctx)

	// Wait for all validators to partially withdraw
	slotsForAllPartialWithdrawals := beacon.Slot(
		len(env.Keys) / int(testnet.Spec().MAX_WITHDRAWALS_PER_PAYLOAD),
	)
	slotCtx, cancel := testnet.Spec().
		SlotTimeoutContext(ctx, slotsForAllPartialWithdrawals+1+(beacon.Slot(config.CapellaForkEpoch.Uint64())*testnet.Spec().SLOTS_PER_EPOCH))
	defer cancel()
loop:
	for {
		select {
		case <-slotCtx.Done():
			break loop
		case <-time.After(time.Duration(testnet.Spec().SECONDS_PER_SLOT) * time.Second):
			// Print all info
			testnet.BeaconClients().Running().PrintStatus(slotCtx, t)
		}
	}

	// Query the execution chain to check the balances, all most be !=0
	ec := testnet.ExecutionClients().Running()[0]
	if err := CheckCorrectWithdrawalBalances(ctx, ec, env.Keys); err != nil {
		t.Fatalf("FAIL: balance incongruence found: %v", err)
	}
}

type BLSToExecutionChangeTestSpec struct {
	BaseWithdrawalsTestSpec
	SubmitAfterCapellaFork bool
	IgnoreRPCError         bool
}

func (ts BLSToExecutionChangeTestSpec) Execute(
	t *hivesim.T,
	env *tn.Environment,
	n clients.NodeDefinition,
) {
	config := ts.GetTestnetConfig(n)
	ctx := context.Background()

	testnet := tn.StartTestnet(ctx, t, env, config)
	defer testnet.Stop()

	// Wait for beacon chain genesis to happen
	testnet.WaitForGenesis(ctx)

	// Wait for 3 slots to pass
	<-time.After(
		3 * time.Second * time.Duration(testnet.Spec().SECONDS_PER_SLOT),
	)

	// Submit BLS-to-execution directives for all validators
	capellaBLSToExecDomain := ComputeBLSToExecutionDomain(testnet)

	blsChanges := make(beacon.SignedBLSToExecutionChanges, 0)
	for index, key := range env.Keys {
		executionAddress := beacon.Eth1Address{byte(index + 0x100)}
		if signedBlsChange, err := key.SignBLSToExecutionChange(
			capellaBLSToExecDomain,
			beacon.ValidatorIndex(index),
			executionAddress,
		); err != nil {
			t.Fatalf("FAIL: Unable to sign bls-to-execution change: %v", err)
		} else {
			blsChanges = append(blsChanges, *signedBlsChange)
		}
	}

	blsChangesJson, _ := json.MarshalIndent(blsChanges, "", " ")
	t.Logf("INFO: Prepared bls changes:\n%s", blsChangesJson)

	// Send the signed bls changes to the beacon client
	if !ts.SubmitAfterCapellaFork {
		if err := testnet.BeaconClients().Running()[0].SubmitPoolBLSToExecutionChange(ctx, blsChanges); err != nil &&
			!ts.IgnoreRPCError {
			t.Fatalf(
				"FAIL: Unable to submit bls-to-execution changes: %v",
				err,
			)
		}
	} else {
		// First wait for Capella
		if config.CapellaForkEpoch.Uint64() > 0 {
			slotsUntilCapella := beacon.Slot(
				config.CapellaForkEpoch.Uint64(),
			) * testnet.Spec().SLOTS_PER_EPOCH
			testnet.WaitSlots(ctx, slotsUntilCapella)
		}
		// Then send the bls changes
		if err := testnet.BeaconClients().Running()[0].SubmitPoolBLSToExecutionChange(ctx, blsChanges); err != nil &&
			!ts.IgnoreRPCError {
			t.Fatalf(
				"FAIL: Unable to submit bls-to-execution changes: %v",
				err,
			)
		}
	}

	// Wait for all BLS to execution to be included
	slotsForAllBlsInclusion := beacon.Slot(
		len(env.Keys) / int(testnet.Spec().MAX_BLS_TO_EXECUTION_CHANGES),
	)
	testnet.WaitSlots(ctx, slotsForAllBlsInclusion)

	// Get the beacon state and verify the credentials were updated
	bn := testnet.BeaconClients().Running()[0]
	versionedBeaconState, err := bn.BeaconStateV2ByBlock(
		ctx,
		eth2api.BlockHead,
	)
	if err != nil {
		t.Fatalf("FAIL: Unable to get latest beacon state: %v", err)
	}
	validators := versionedBeaconState.Validators()
	for index := range env.Keys {
		validator := validators[index]
		credentials := validator.WithdrawalCredentials
		if !bytes.Equal(
			credentials[:1],
			[]byte{beacon.ETH1_ADDRESS_WITHDRAWAL_PREFIX},
		) {
			t.Fatalf(
				"FAIL: Withdrawal credential not updated for validator %d: %v",
				index,
				credentials,
			)
		}
		expectedExecutionAddress := beacon.Eth1Address{byte(index + 0x100)}
		if !bytes.Equal(expectedExecutionAddress[:], credentials[12:]) {
			t.Fatalf(
				"FAIL: Incorrect withdrawal credential for validator %d: want=%x, got=%x",
				index,
				expectedExecutionAddress,
				credentials[12:],
			)
		}
		t.Logf("INFO: Successful BLS to execution change: %x", credentials)
	}
}

func TestCapellaFork(t *hivesim.T, env *tn.Environment,
	config *tn.Config,
) {
	ctx := context.Background()

	testnet := tn.StartTestnet(ctx, t, env, config)
	defer testnet.Stop()

	finalityCtx, cancel := testnet.Spec().EpochTimeoutContext(
		ctx,
		EPOCHS_TO_FINALITY+2,
	)
	defer cancel()

	finalized, err := testnet.WaitForFinality(finalityCtx)
	if err != nil {
		t.Fatalf("FAIL: Waiting for finality: %v", err)
	}

	if err := testnet.VerifyParticipation(
		ctx,
		tn.FirstSlotAfterCheckpoint{Checkpoint: &finalized},
		0.95,
	); err != nil {
		t.Fatalf("FAIL: Verifying participation: %v", err)
	}

	if err := testnet.VerifyExecutionPayloadIsCanonical(
		ctx,
		tn.LastSlotAtCheckpoint{Checkpoint: &finalized},
	); err != nil {
		t.Fatalf("FAIL: Verifying execution payload is canonical: %v", err)
	}

	if err := testnet.VerifyProposers(
		ctx,
		tn.LastSlotAtCheckpoint{Checkpoint: &finalized},
		true,
	); err != nil {
		t.Fatalf("FAIL: Verifying proposers: %v", err)
	}

	if err := testnet.VerifyELHeads(ctx); err != nil {
		t.Fatalf("FAIL: Verifying EL Heads: %v", err)
	}

	for _, el := range testnet.ExecutionClients().Running() {
		if b, err := el.BlockByNumber(ctx, nil); err != nil {
			t.Fatalf("FAIL: Unable to get execution client head: %v", err)
		} else {
			withdrawals := b.Header().WithdrawalsHash
			if withdrawals == nil {
				t.Fatalf("FAIL: Nil withdrawals on capella fork: %v", withdrawals)
			}
			t.Logf("INFO: Non-Nil withdrawals on capella fork: %v", withdrawals)
		}
	}
}
