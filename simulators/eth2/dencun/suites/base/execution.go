package suite_base

import (
	"context"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/hive/hivesim"

	beacon_verification "github.com/ethereum/hive/simulators/eth2/common/spoofing/beacon"
	tn "github.com/ethereum/hive/simulators/eth2/common/testnet"
	"github.com/ethereum/hive/simulators/eth2/common/utils"
	"github.com/ethereum/hive/simulators/ethereum/engine/globals"
	engine_helper "github.com/ethereum/hive/simulators/ethereum/engine/helper"
	"github.com/protolambda/eth2api"
	beacon "github.com/protolambda/zrnt/eth2/beacon/common"
)

var Deneb string = "deneb"

var (
	normalTxAccounts = globals.TestAccounts[:len(globals.TestAccounts)/2]
	blobTxAccounts   = globals.TestAccounts[len(globals.TestAccounts)/2:]
)

func WithdrawalAddress(vI beacon.ValidatorIndex) common.Address {
	return common.Address{byte(vI + 0x100)}
}

// Generic Deneb test routine, capable of running most of the test
// scenarios.
func (ts BaseTestSpec) ExecutePreFork(
	t *hivesim.T,
	ctx context.Context,
	testnet *tn.Testnet,
	env *tn.Environment,
	config *tn.Config,
) {
	// Setup the transaction spammers, both normal and blob transactions
	normalTxSpammer := utils.TransactionSpammer{
		T:                        t,
		Name:                     "normal",
		Recipient:                &CodeContractAddress,
		ExecutionClients:         testnet.ExecutionClients().Running(),
		Accounts:                 normalTxAccounts,
		TransactionType:          engine_helper.DynamicFeeTxOnly,
		TransactionsPerIteration: 40,
		SecondsBetweenIterations: int(testnet.Spec().SECONDS_PER_SLOT),
	}

	// Start sending normal transactions from dedicated accounts
	go normalTxSpammer.Run(ctx)

	// Add verification of Beacon->Execution Engine API calls to the proxies
	chainconfig := testnet.ExecutionGenesis().Config
	// NewPayloadV2 expires at CancunTime: if a client sends a payload with
	// a timestamp greater than CancunTime, and it's using NewPayloadV2, it
	// must result in test failure.
	newPayloadV2ExpireVerifier := beacon_verification.NewEngineMaxTimestampVerifier(
		t,
		beacon_verification.EngineNewPayloadV2,
		*chainconfig.CancunTime,
	)
	// ForkchoiceUpdatedV2 expires at CancunTime: if a client sends a payload with
	// a timestamp greater than CancunTime, and it's using ForkchoiceUpdatedV2, it
	// must result in test failure.
	forkchoiceUpdatedV2ExpireVerifier := beacon_verification.NewEngineMaxTimestampVerifier(
		t,
		beacon_verification.EngineForkchoiceUpdatedV2,
		*chainconfig.CancunTime,
	)
	for _, e := range testnet.ExecutionClients() {
		newPayloadV2ExpireVerifier.AddToProxy(e.Proxy())
		forkchoiceUpdatedV2ExpireVerifier.AddToProxy(e.Proxy())
	}

	// Wait for beacon chain genesis to happen
	testnet.WaitForGenesis(ctx)
}

// Wait for the fork to happen.
// Under normal circumstances, the fork should happen at the specified slot.
// If a test injects an error at the time of the fork, the test should wait
// for the fork by clock time, and not by the client's response.
func (ts BaseTestSpec) WaitForFork(
	t *hivesim.T,
	ctx context.Context,
	testnet *tn.Testnet,
	env *tn.Environment,
	config *tn.Config,
) {
	slotsUntilFork := beacon.Slot(
		config.DenebForkEpoch.Uint64(),
	)*testnet.Spec().SLOTS_PER_EPOCH + 4
	timeoutCtx, cancel := testnet.Spec().SlotTimeoutContext(ctx, slotsUntilFork)
	defer cancel()
	if err := testnet.WaitForFork(timeoutCtx, Deneb); err != nil {
		t.Fatalf("FAIL: error waiting for deneb: %v", err)
	}
}

func (ts BaseTestSpec) ExecutePostFork(
	t *hivesim.T,
	ctx context.Context,
	testnet *tn.Testnet,
	env *tn.Environment,
	config *tn.Config,
) {
	// Wait one more slot before continuing
	testnet.WaitSlots(ctx, 1)

	// Start sending blob transactions from dedicated accounts
	blobTxSpammer := utils.TransactionSpammer{
		T:                        t,
		Name:                     "blobs",
		Recipient:                &CodeContractAddress,
		ExecutionClients:         testnet.ExecutionClients().Running(),
		Accounts:                 blobTxAccounts,
		TransactionType:          engine_helper.BlobTxOnly,
		TransactionsPerIteration: 2,
		SecondsBetweenIterations: int(testnet.Spec().SECONDS_PER_SLOT),
	}

	go blobTxSpammer.Run(ctx)

	// Send BLSToExecutionChanges messages during Deneb for all validators on BLS credentials
	nonWithdrawableValidators := testnet.Validators.NonWithdrawable()
	testnet.ValidatorGroups["nonWithdrawable"] = nonWithdrawableValidators
	if nonWithdrawableValidators.Count() > 0 {
		beaconClients := testnet.BeaconClients().Running()
		domainType := beacon.DOMAIN_BLS_TO_EXECUTION_CHANGE
		forkVersion := testnet.Spec().GENESIS_FORK_VERSION
		validatorsRoot := testnet.GenesisValidatorsRoot()
		domain := beacon.ComputeDomain(
			domainType,
			forkVersion,
			validatorsRoot,
		)
		t.Logf("INFO: BLS to execution domain=%s, domain type = %s, fork version = %s, validators root = %s", domain.String(), domainType.String(), forkVersion.String(), validatorsRoot.String())
		i := 0
		nonWithdrawableValidators.ForEach(func(v *utils.Validator) {
			b := beaconClients[i%len(beaconClients)]
			if err := v.SignSendBLSToExecutionChange(
				ctx,
				b,
				WithdrawalAddress(v.Index),
				domain,
			); err != nil {
				t.Fatalf(
					"FAIL: Unable to submit bls-to-execution changes: %v",
					err,
				)
			}
			i++
		})
		t.Logf("INFO: sent bls-to-execution changes of %d validators", i)
	} else {
		t.Logf("INFO: no validators left on BLS credentials")
	}

	// Send exit messages for the share of validators that will exit
	// at the fork
	if ts.ExitValidatorsShare > 0 {
		// Get the validators that will exit
		exitValidators := testnet.Validators.Chunks(ts.ExitValidatorsShare)[0]
		testnet.ValidatorGroups["toExit"] = exitValidators
		if exitValidators.Count() > 0 {
			beaconClients := testnet.BeaconClients().Running()
			domain := beacon.ComputeDomain(
				beacon.DOMAIN_VOLUNTARY_EXIT,
				testnet.Spec().CAPELLA_FORK_VERSION,
				testnet.GenesisValidatorsRoot(),
			)
			i := 0
			exitValidators.ForEach(func(v *utils.Validator) {
				b := beaconClients[i%len(beaconClients)]
				if err := v.SignSendVoluntaryExit(
					ctx,
					b,
					domain,
				); err != nil {
					t.Fatalf(
						"FAIL: Unable to submit exit: %v",
						err,
					)
				}
				i++
			})

		} else {
			t.Logf("INFO: no validators to exit")
		}
	}
}

func (ts BaseTestSpec) ExecutePostForkWait(
	t *hivesim.T,
	ctx context.Context,
	testnet *tn.Testnet,
	env *tn.Environment,
	config *tn.Config,
) {
	if ts.EpochsAfterFork != 0 {
		// Wait for the specified number of epochs after fork
		if err := testnet.WaitSlots(ctx, beacon.Slot(ts.EpochsAfterFork)*testnet.Spec().SLOTS_PER_EPOCH); err != nil {
			t.Fatalf("FAIL: error waiting for %d epochs after fork: %v", ts.EpochsAfterFork, err)
		}
	}
	if ts.WaitForBlobs {
		// Check that all clients have produced at least one block with blobs
		blobProposers := make([]bool, len(testnet.VerificationNodes().Running()))

		blobsWaitCtx, cancel := testnet.Spec().EpochTimeoutContext(ctx, 1)
		defer cancel()
	out:
		for {
			select {
			case <-blobsWaitCtx.Done():
				t.Fatalf("FAIL: context expired waiting for blobs: %v", blobsWaitCtx.Err())
			case <-time.After(time.Duration(testnet.Spec().SECONDS_PER_SLOT) * time.Second):
				if blobCount, err := testnet.VerifyBlobs(ctx, tn.LastestSlotByHead{}); err != nil {
					t.Fatalf("FAIL: error verifying blobs: %v", err)
				} else if blobCount > 0 {
					proposerIdx, err := testnet.GetProposer(blobsWaitCtx, tn.LastestSlotByHead{})
					if err != nil {
						t.Fatalf("FAIL: error getting proposer: %v", err)
					}
					t.Logf("INFO: blobs have been included in the chain by proposer %d", proposerIdx)
					blobProposers[proposerIdx] = true
				}

				// Check if all clients have produced a block with blobs
				allProposers := true
				for _, p := range blobProposers {
					allProposers = allProposers && p
				}
				if allProposers {
					t.Logf("INFO: all clients have produced a block with blobs")
					break out
				}
			}
		}
	}
	if ts.WaitForFinality {
		finalityCtx, cancel := testnet.Spec().EpochTimeoutContext(ctx, 5)
		defer cancel()
		if _, err := testnet.WaitForCurrentEpochFinalization(finalityCtx); err != nil {
			t.Fatalf("FAIL: error waiting for epoch finalization: %v", err)
		}
	}
}

func (ts BaseTestSpec) Verify(
	t *hivesim.T,
	ctx context.Context,
	testnet *tn.Testnet,
	env *tn.Environment,
	config *tn.Config,
) {
	// Check all clients are on the same head
	if err := testnet.VerifyELHeads(ctx); err != nil {
		t.Fatalf("FAIL: error verifying execution layer heads: %v", err)
	}

	// Check for optimistic sync
	for i, n := range testnet.Nodes.Running() {
		bc := n.BeaconClient
		if op, err := bc.BlockIsOptimistic(ctx, eth2api.BlockHead); op {
			t.Fatalf(
				"FAIL: client %d (%s) is optimistic, it should be synced.",
				i,
				n.ClientNames(),
			)
		} else if err != nil {
			t.Fatalf("FAIL: error querying optimistic state on client %d (%s): %v", i, n.ClientNames(), err)
		}
	}

	// Set timeout for the following checks
	timeoutCtx, cancel := testnet.Spec().EpochTimeoutContext(ctx, 2)
	defer cancel()

	// Check non-withdrawable validators changed their credentials
	nonWithdrawableValidators := testnet.ValidatorGroups["nonWithdrawable"]
	if nonWithdrawableValidators != nil && nonWithdrawableValidators.Count() > 0 {
		for {
			select {
			case <-time.After(time.Duration(testnet.Spec().SECONDS_PER_SLOT) * time.Second):
			case <-timeoutCtx.Done():
				t.Fatalf("FAIL: context expired waiting for non-withdrawable validators to change credentials: %v", timeoutCtx.Err())
			}
			// Update the validator set from state
			state, err := testnet.BeaconClients().Running()[0].BeaconStateV2(ctx, eth2api.StateHead)
			if err != nil {
				t.Fatalf("FAIL: error getting beacon state: %v", err)
			}

			beaconState, err := state.Tree(testnet.Spec().Spec)
			if err != nil {
				t.Fatalf("FAIL: error getting beacon state tree: %v", err)
			}

			nonWithdrawableValidators.UpdateFromBeaconState(beaconState)

			// Check if all validators have changed their credentials
			if nonWithdrawableValidators.NonWithdrawable().Count() == 0 {
				t.Logf("INFO: all non-withdrawable validators have changed credentials")
				break
			} else {
				t.Logf(
					"INFO: %d non-withdrawable validators have not changed credentials",
					nonWithdrawableValidators.NonWithdrawable().Count(),
				)
			}
		}
	}

	// Check exit validators have an exit epoch set
	toExitValidators := testnet.ValidatorGroups["toExit"]
	if toExitValidators != nil && toExitValidators.Count() > 0 {
		toExitValidatorsCount := toExitValidators.Count()
		for {
			select {
			case <-time.After(time.Duration(testnet.Spec().SECONDS_PER_SLOT) * time.Second):
			case <-timeoutCtx.Done():
				t.Fatalf("FAIL: context expired waiting for validators to exit: %v", timeoutCtx.Err())
			}
			// Update the validator set from state
			state, err := testnet.BeaconClients().Running()[0].BeaconStateV2(ctx, eth2api.StateHead)
			if err != nil {
				t.Fatalf("FAIL: error getting beacon state: %v", err)
			}

			beaconState, err := state.Tree(testnet.Spec().Spec)
			if err != nil {
				t.Fatalf("FAIL: error getting beacon state tree: %v", err)
			}

			toExitValidators.UpdateFromBeaconState(beaconState)

			// Check if all validators have initiated exit
			exitInitiatedCount := toExitValidators.ExitInitiated().Count()
			if exitInitiatedCount == toExitValidatorsCount {
				t.Logf("INFO: all validators have initiated exit")
				break
			} else {
				t.Logf(
					"INFO: %d validators have not initiated exit",
					toExitValidatorsCount-exitInitiatedCount,
				)
			}
		}
	}

	// Verify all clients agree on blobs for each slot
	if blobCount, err := testnet.VerifyBlobs(ctx, tn.LastestSlotByHead{}); err != nil {
		t.Fatalf("FAIL: error verifying blobs: %v", err)
	} else if blobCount == 0 {
		t.Fatalf("FAIL: no blobs were included in the chain")
	} else {
		t.Logf("INFO: %d blobs were included in the chain", blobCount)
	}
}
