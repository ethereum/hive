package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"time"

	api "github.com/ethereum/go-ethereum/beacon/engine"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/hive/hivesim"
	mock_builder "github.com/ethereum/hive/simulators/eth2/common/builder/mock"
	"github.com/ethereum/hive/simulators/eth2/common/clients"

	beacon_verification "github.com/ethereum/hive/simulators/eth2/common/spoofing/beacon"
	tn "github.com/ethereum/hive/simulators/eth2/common/testnet"
	"github.com/protolambda/eth2api"
	beacon "github.com/protolambda/zrnt/eth2/beacon/common"
)

var ConsensusClientsSupportingBLSChangesBeforeCapella = []string{
	"prysm",
	"lodestar",
}

// Generic withdrawals test routine, capable of running most of the test
// scenarios.
func (ts BaseWithdrawalsTestSpec) Execute(
	t *hivesim.T,
	env *tn.Environment,
	n []clients.NodeDefinition,
) {
	config := ts.GetTestnetConfig(n)
	ctx := context.Background()

	testnet := tn.StartTestnet(ctx, t, env, config)
	defer testnet.Stop()

	// Add verification of Beacon->Execution Engine API calls to the proxies

	chainconfig := testnet.ExecutionGenesis().Config
	// NewPayloadV1 expires at ShanghaiTime
	newPayloadV1ExpireVerifier := beacon_verification.NewEngineMaxTimestampVerifier(
		t,
		beacon_verification.EngineNewPayloadV1,
		*chainconfig.ShanghaiTime,
	)
	// ForkchoiceUpdatedV1 expires at ShanghaiTime
	forkchoiceUpdatedV1ExpireVerifier := beacon_verification.NewEngineMaxTimestampVerifier(
		t,
		beacon_verification.EngineForkchoiceUpdatedV1,
		*chainconfig.ShanghaiTime,
	)
	for _, e := range testnet.ExecutionClients() {
		newPayloadV1ExpireVerifier.AddToProxy(e.Proxy())
		forkchoiceUpdatedV1ExpireVerifier.AddToProxy(e.Proxy())
	}

	blsDomain := ComputeBLSToExecutionDomain(testnet)

	// Get all validators info
	allValidators, err := ValidatorsFromBeaconState(
		testnet.GenesisBeaconState(),
		*testnet.Spec().Spec,
		env.Keys,
		&blsDomain,
	)
	if err != nil {
		t.Fatalf("FAIL: Error parsing validators from beacon state")
	}
	genesisNonWithdrawable := allValidators.NonWithdrawable()

	// Wait for beacon chain genesis to happen
	testnet.WaitForGenesis(ctx)

	// Wait for 3 slots to pass
	<-time.After(
		3 * time.Second * time.Duration(testnet.Spec().SECONDS_PER_SLOT),
	)

	if beaconClients := testnet.FilterByCL(ConsensusClientsSupportingBLSChangesBeforeCapella).
		BeaconClients(); len(
		beaconClients,
	) > 0 &&
		ts.SubmitBLSChangesOnBellatrix {
		// If there are clients that support sending BLS to execution
		// changes in bellatrix, we send half of the changes here
		if len(genesisNonWithdrawable) > 0 {
			nonWithdrawableValidators := genesisNonWithdrawable.Chunks(2)[0]

			if len(nonWithdrawableValidators) > 0 {
				t.Logf(
					"INFO: Sending %d validators' BLS-to-exec-change on bellatrix",
					len(nonWithdrawableValidators),
				)
				for i := 0; i < len(nonWithdrawableValidators); i++ {
					b := beaconClients[i%len(beaconClients)]
					v := nonWithdrawableValidators[i]
					if err := v.SignSendBLSToExecutionChange(
						ctx,
						b,
						common.Address{byte(v.Index + 0x100)},
					); err != nil {
						t.Fatalf(
							"FAIL: Unable to submit bls-to-execution changes: %v",
							err,
						)
					} else {
						t.Logf("INFO: Sent validator %d BLS-To-Exec-Change on Bellatrix (%s)", v.Index, b.ClientName())
					}
				}

			}

		} else {
			t.Logf("INFO: no validators left on BLS credentials")
		}
	} else {
		t.Logf("INFO: No beacon clients support BLS-To-Execution-Changes on bellatrix, skipping")
	}

	// Wait for Capella
	if config.CapellaForkEpoch.Uint64() > 0 {
		slotsUntilCapella := beacon.Slot(
			config.CapellaForkEpoch.Uint64(),
		) * testnet.Spec().SLOTS_PER_EPOCH
		testnet.WaitSlots(ctx, slotsUntilCapella)
	}

	// If there are any remaining validators that cannot withdraw yet, send
	// them now
	nonWithdrawableValidators := allValidators.NonWithdrawable()
	if len(nonWithdrawableValidators) > 0 {
		beaconClients := testnet.BeaconClients()
		for i := 0; i < len(nonWithdrawableValidators); i++ {
			b := beaconClients[i%len(beaconClients)]
			v := nonWithdrawableValidators[i]
			if err := v.SignSendBLSToExecutionChange(
				ctx,
				b,
				common.Address{byte(v.Index + 0x100)},
			); err != nil {
				t.Fatalf(
					"FAIL: Unable to submit bls-to-execution changes: %v",
					err,
				)
			} else {
				t.Logf("INFO: Sent validator %d BLS-To-Exec-Change on Capella (%s)", v.Index, b.ClientName())
			}
		}

		// Wait for all BLS to execution to be included
		slotsForAllBlsInclusion := beacon.Slot(
			len(genesisNonWithdrawable)/int(
				testnet.Spec().MAX_BLS_TO_EXECUTION_CHANGES,
			) + 1,
		)
		testnet.WaitSlots(ctx, slotsForAllBlsInclusion)
	} else {
		t.Logf("INFO: no validators left on BLS credentials")
	}

	// Get the beacon state and verify the credentials were updated
	var versionedBeaconState *clients.VersionedBeaconStateResponse
	for _, bn := range testnet.BeaconClients().Running() {
		versionedBeaconState, err = bn.BeaconStateV2(
			ctx,
			eth2api.StateHead,
		)
		if err != nil || versionedBeaconState == nil {
			t.Logf("WARN: Unable to get latest beacon state: %v", err)
		} else {
			break
		}
	}
	if versionedBeaconState == nil {
		t.Fatalf(
			"FAIL: Unable to get latest beacon state from any client: %v",
			err,
		)
	}

	validators := versionedBeaconState.Validators()

	if len(genesisNonWithdrawable) > 0 {
		t.Logf("INFO: Checking validator updates on slot %d",
			versionedBeaconState.StateSlot())

		for _, v := range genesisNonWithdrawable {
			validator := validators[v.Index]
			credentials := validator.WithdrawalCredentials
			if !bytes.Equal(
				credentials[:1],
				[]byte{beacon.ETH1_ADDRESS_WITHDRAWAL_PREFIX},
			) {
				t.Fatalf(
					"FAIL: Withdrawal credential not updated for validator %d: %v",
					v.Index,
					credentials,
				)
			}
			if v.WithdrawAddress == nil {
				t.Fatalf(
					"FAIL: BLS-to-execution change was not sent for validator %d",
					v.Index,
				)
			}
			if !bytes.Equal(v.WithdrawAddress[:], credentials[12:]) {
				t.Fatalf(
					"FAIL: Incorrect withdrawal credential for validator %d: want=%x, got=%x",
					v.Index,
					v.WithdrawAddress,
					credentials[12:],
				)
			}
			t.Logf("INFO: Successful BLS to execution change: %s", credentials)
		}
	}

	// Wait for all validators to withdraw
	waitSlotsForAllWithdrawals := beacon.Slot(
		(len(validators)/int(testnet.Spec().MAX_WITHDRAWALS_PER_PAYLOAD) +
			5), // Wiggle room
	)
	slotCtx, cancel := testnet.Spec().
		SlotTimeoutContext(ctx, waitSlotsForAllWithdrawals)
	defer cancel()
loop:
	for {
		select {
		case <-slotCtx.Done():
			PrintWithdrawalHistory(allValidators[0].BlockStateCache)
			t.Fatalf("FAIL: Timeout waiting on all accounts to withdraw")
		case <-time.After(time.Duration(testnet.Spec().SECONDS_PER_SLOT) * time.Second):
			// Print all info
			testnet.BeaconClients().Running().PrintStatus(slotCtx, t)

			// Check all accounts
			for _, n := range testnet.Nodes.Running() {
				ec := n.ExecutionClient
				bc := n.BeaconClient
				headBlockRoot, err := bc.BlockV2Root(ctx, eth2api.BlockHead)
				if err != nil {
					t.Logf("INFO: error getting head block: %v", err)
					continue
				}
				if allAccountsWithdrawn, err := allValidators.Withdrawable().VerifyWithdrawnBalance(ctx, bc, ec, headBlockRoot); err != nil {
					t.Logf("INFO: error getting withdrawals balances: %v", err)
					continue
				} else if allAccountsWithdrawn {
					t.Logf("INFO: All accounts have successfully withdrawn")
					break loop
				}
			}
		}
	}

	PrintWithdrawalHistory(allValidators[0].BlockStateCache)

	// Lastly check all clients are on the same head
	testnet.VerifyELHeads(ctx)

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

	if ts.WaitForFinality {
		testnet.WaitForFinality(ctx)
	}
}

var (
	slotsPerEpoch             = uint64(32)
	withdrawalsPerInvalidList = uint64(16)
)

// Builder testnet.
func (ts BuilderWithdrawalsTestSpec) Execute(
	t *hivesim.T,
	env *tn.Environment,
	n []clients.NodeDefinition,
) {
	config := ts.GetTestnetConfig(n)
	ctx := context.Background()

	capellaSlot := beacon.Slot(
		config.CapellaForkEpoch.Uint64() * slotsPerEpoch,
	)

	// Configure the builder according to the error
	config.BuilderOptions = make([]mock_builder.Option, 0)

	// Bump the built payloads value
	config.BuilderOptions = append(
		config.BuilderOptions,
		mock_builder.WithPayloadWeiValueBump(big.NewInt(10000)),
	)

	// Inject test error
	switch ts.BuilderTestError {
	case INVALID_WITHDRAWALS:
		config.BuilderOptions = append(
			config.BuilderOptions,
			mock_builder.WithPayloadAttributesModifier(
				func(pa *api.PayloadAttributes, s beacon.Slot) (bool, error) {
					// Only modify once we reached capella
					if s >= capellaSlot {
						// Create a list of invalid (random) withdrawals within the length limit
						pa.Withdrawals = make(
							[]*types.Withdrawal,
							withdrawalsPerInvalidList,
						)
						for i := uint64(0); i < withdrawalsPerInvalidList; i++ {
							w := types.Withdrawal{}
							w.Index = i + (uint64(s-capellaSlot) * withdrawalsPerInvalidList)
							w.Validator = i + 1
							w.Amount = i + 1
							rand.Read(w.Address[:])
							pa.Withdrawals[i] = &w
						}
						return true, nil
					}
					return false, nil
				},
			),
		)
	case INVALIDATE_SINGLE_WITHDRAWAL_ADDRESS,
		INVALIDATE_SINGLE_WITHDRAWAL_AMOUNT,
		INVALIDATE_SINGLE_WITHDRAWAL_VALIDATOR_INDEX,
		INVALIDATE_SINGLE_WITHDRAWAL_INDEX:
		config.BuilderOptions = append(
			config.BuilderOptions,
			mock_builder.WithPayloadAttributesModifier(
				func(pa *api.PayloadAttributes, s beacon.Slot) (bool, error) {
					// Only modify once we reached capella
					if s >= capellaSlot {
						// We need to invalidate a single withdrawal
						if len(pa.Withdrawals) > 0 {
							switch ts.BuilderTestError {
							case INVALIDATE_SINGLE_WITHDRAWAL_ADDRESS:
								pa.Withdrawals[0].Address[0]++
							case INVALIDATE_SINGLE_WITHDRAWAL_AMOUNT:
								pa.Withdrawals[0].Amount++
							case INVALIDATE_SINGLE_WITHDRAWAL_VALIDATOR_INDEX:
								pa.Withdrawals[0].Validator++
							case INVALIDATE_SINGLE_WITHDRAWAL_INDEX:
								pa.Withdrawals[0].Index++
							}
							return true, nil
						}
					}
					return false, nil
				},
			),
		)
	case VALID_WITHDRAWALS_INVALID_STATE_ROOT:
		config.BuilderOptions = append(
			config.BuilderOptions,
			mock_builder.WithPayloadModifier(
				func(ed *api.ExecutableData, s beacon.Slot) (bool, error) {
					// Only modify once we reached capella
					if s >= capellaSlot {
						var (
							originalHash      = ed.BlockHash
							originalStateRoot = ed.StateRoot
							modifiedStateRoot = common.Hash{}
						)
						// We need to simulate the builder producing an invalid
						// execution payload by modifying its state root
						rand.Read(modifiedStateRoot[:])
						if b, err := api.ExecutableDataToBlock(*ed); err != nil {
							return false, err
						} else {
							header := b.Header()
							header.Root = modifiedStateRoot
							modifiedHash := header.Hash()
							copy(ed.BlockHash[:], modifiedHash[:])
							copy(ed.StateRoot[:], modifiedStateRoot[:])
						}
						t.Logf(
							"INFO: Modified payload %d: hash:%s->%s, stateRoot:%s->%s, parentHash:%s",
							ed.Number,
							originalHash,
							ed.BlockHash,
							originalStateRoot,
							ed.StateRoot,
							ed.ParentHash,
						)
						return true, nil
					}
					return false, nil
				},
			),
		)
	case ERROR_ON_HEADER_REQUEST:
		config.BuilderOptions = append(
			config.BuilderOptions,
			mock_builder.WithErrorOnHeaderRequest(
				func(s beacon.Slot) error {
					if s >= capellaSlot {
						return fmt.Errorf("error produced by test")
					}
					return nil
				},
			),
		)
	case ERROR_ON_UNBLINDED_PAYLOAD_REQUEST:
		config.BuilderOptions = append(
			config.BuilderOptions,
			mock_builder.WithErrorOnPayloadReveal(
				func(s beacon.Slot) error {
					if s >= capellaSlot {
						return fmt.Errorf("error produced by test")
					}
					return nil
				},
			),
		)
	}

	testnet := tn.StartTestnet(ctx, t, env, config)
	defer testnet.Stop()

	go func() {
		lastNonce := uint64(0)
		txPerIteration := 5
		txCreator := BaseTransactionCreator{
			GasLimit:   500000,
			Amount:     common.Big1,
			PrivateKey: VaultKey,
		}
		// Send some transactions constantly in the bg
		for {
			select {
			case <-ctx.Done():
				return
			case <-time.After(time.Second):
				for i := 0; i < txPerIteration; i++ {
					txCreator.Recipient = &CodeContractAddress
					tx, err := txCreator.MakeTransaction(lastNonce)
					if err != nil {
						panic(err)
					}
					if err := testnet.ExecutionClients().Running()[0].SendTransaction(
						ctx,
						tx,
					); err != nil {
						t.Logf("INFO: Error sending tx: %v", err)
					}
					lastNonce++
				}
			}
		}
	}()

	// Wait for capella
	forkCtx, cancel := testnet.Spec().
		EpochTimeoutContext(ctx, beacon.Epoch(config.CapellaForkEpoch.Uint64())+1)
	defer cancel()
	if err := testnet.WaitForFork(forkCtx, "capella"); err != nil {
		t.Fatalf("FAIL: error while waiting for capella: %v", err)
	}

	// Check that the builder was working properly until now
	for i, b := range testnet.BeaconClients().Running() {
		builder := b.Builder
		if builder.GetBuiltPayloadsCount() == 0 {
			t.Fatalf("FAIL: builder %d did not build any payloads", i)
		}
		if builder.GetSignedBeaconBlockCount() == 0 {
			t.Fatalf(
				"FAIL: builder %d did not produce any signed beacon blocks",
				i,
			)
		}
	}

	// Wait for finalization, to verify that builder modifications
	// did not affect the network
	finalityCtx, cancel := testnet.Spec().EpochTimeoutContext(ctx, 5)
	defer cancel()
	if _, err := testnet.WaitForCurrentEpochFinalization(finalityCtx); err != nil {
		t.Fatalf("FAIL: error waiting for epoch finalization: %v", err)
	}

	// Verify any modified payloads did not make it into the
	// canonical chain
	switch ts.BuilderTestError {
	case NO_ERROR:
		// Simply verify that builder's capella payloads were included in the
		// canonical chain
		for i, n := range testnet.Nodes.Running() {
			b := n.BeaconClient.Builder
			ec := n.ExecutionClient
			includedPayloads := 0
			for _, p := range b.GetBuiltPayloads() {
				if p.Withdrawals != nil {
					if h, err := ec.HeaderByNumber(ctx, big.NewInt(int64(p.Number))); err != nil {
						t.Fatalf(
							"FAIL: error getting execution header from node %d: %v",
							i,
							err,
						)
					} else if h != nil {
						hash := h.Hash()
						if bytes.Equal(hash[:], p.BlockHash[:]) {
							includedPayloads++
						}
					}
				}
			}
			if includedPayloads == 0 {
				t.Fatalf(
					"FAIL: builder %d did not produce capella payloads included in the canonical chain",
					i,
				)
			}
		}
	case INVALID_WITHDRAWALS,
		INVALIDATE_SINGLE_WITHDRAWAL_ADDRESS,
		INVALIDATE_SINGLE_WITHDRAWAL_AMOUNT,
		INVALIDATE_SINGLE_WITHDRAWAL_VALIDATOR_INDEX,
		INVALIDATE_SINGLE_WITHDRAWAL_INDEX:
		for i, n := range testnet.VerificationNodes().Running() {
			modifiedPayloads := n.BeaconClient.Builder.GetModifiedPayloads()
			if len(modifiedPayloads) == 0 {
				t.Fatalf("FAIL: No payloads were modified by builder %d", i)
			}
			for _, p := range modifiedPayloads {
				for _, ec := range testnet.ExecutionClients().Running() {
					b, err := ec.BlockByNumber(
						ctx,
						big.NewInt(int64(p.Number)),
					)
					if err != nil {
						t.Fatalf(
							"FAIL: Error getting execution block %d: %v",
							p.Number,
							err,
						)
					}
					h := b.Hash()
					if bytes.Equal(h[:], p.BlockHash[:]) {
						t.Fatalf(
							"FAIL: Modified payload included in canonical chain: %d (%s)",
							p.Number,
							p.BlockHash,
						)
					}
				}
			}
			t.Logf(
				"INFO: No modified payloads were included in canonical chain of node %d",
				i,
			)
		}
	}

	// Count and print missed slots
	if count, err := testnet.BeaconClients().Running()[0].GetFilledSlotsCountPerEpoch(ctx); err != nil {
		t.Fatalf("FAIL: unable to obtain slot count per epoch: %v", err)
	} else {
		for ep, slots := range count {
			t.Logf("INFO: Epoch %d, filled slots=%d", ep, slots)
		}
	}
}
