package main

import (
	"context"
	"fmt"
	"math/big"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/hive/hivesim"
	"github.com/ethereum/hive/simulators/eth2/testnet/setup"
	"github.com/protolambda/eth2api"
	"github.com/protolambda/eth2api/client/beaconapi"
	"github.com/protolambda/eth2api/client/debugapi"
	"github.com/protolambda/zrnt/eth2/beacon/altair"
	"github.com/protolambda/zrnt/eth2/beacon/bellatrix"
	"github.com/protolambda/zrnt/eth2/beacon/common"
	"github.com/protolambda/zrnt/eth2/beacon/phase0"
	"github.com/protolambda/zrnt/eth2/util/math"
)

var MAX_PARTICIPATION_SCORE = 7

type Testnet struct {
	t *hivesim.T

	genesisTime           common.Timestamp
	genesisValidatorsRoot common.Root

	// Consensus chain configuration
	spec *common.Spec
	// Execution chain configuration and genesis info
	eth1Genesis *setup.Eth1Genesis

	beacons    []*BeaconNode
	validators []*ValidatorClient
	eth1       []*Eth1Node
}

func (t *Testnet) GenesisTime() time.Time {
	return time.Unix(int64(t.genesisTime), 0)
}

func (t *Testnet) ValidatorClientIndex(pk [48]byte) (int, error) {
	for i, v := range t.validators {
		if v.ContainsKey(pk) {
			return i, nil
		}
	}
	return 0, fmt.Errorf("key not found in any validator client")
}

// WaitForFinality blocks until a beacon client reaches finality.
func (t *Testnet) WaitForFinality(ctx context.Context) (common.Checkpoint, error) {
	genesis := t.GenesisTime()
	slotDuration := time.Duration(t.spec.SECONDS_PER_SLOT) * time.Second
	timer := time.NewTicker(slotDuration)
	done := make(chan common.Checkpoint, len(t.beacons))

	for {
		select {
		case <-ctx.Done():
			return common.Checkpoint{}, fmt.Errorf("context called")
		case finalized := <-done:
			return finalized, nil
		case tim := <-timer.C:
			// start polling after first slot of genesis
			if tim.Before(genesis.Add(slotDuration)) {
				t.t.Logf("Time till genesis: %s", genesis.Sub(tim))
				continue
			}

			// new slot, log and check status of all beacon nodes
			type res struct {
				idx int
				msg string
				err error
			}
			var (
				wg sync.WaitGroup
				ch = make(chan res, len(t.beacons))
			)
			for i, b := range t.beacons {
				wg.Add(1)
				go func(ctx context.Context, i int, b *BeaconNode, ch chan res) {
					defer wg.Done()
					ctx, cancel := context.WithTimeout(ctx, time.Second*5)
					defer cancel()

					var (
						slot      common.Slot
						head      string
						justified string
						finalized string
						execution string
						health    float64
					)

					var headInfo eth2api.BeaconBlockHeaderAndInfo
					if exists, err := beaconapi.BlockHeader(ctx, b.API, eth2api.BlockHead, &headInfo); err != nil {
						ch <- res{err: fmt.Errorf("beacon %d: failed to poll head: %v", i, err)}
						return
					} else if !exists {
						ch <- res{err: fmt.Errorf("beacon %d: no head block", i)}
						return
					}

					var checkpoints eth2api.FinalityCheckpoints
					if exists, err := beaconapi.FinalityCheckpoints(ctx, b.API, eth2api.StateIdRoot(headInfo.Header.Message.StateRoot), &checkpoints); err != nil {
						ch <- res{err: fmt.Errorf("beacon %d: failed to poll finality checkpoint: %v", i, err)}
						return
					} else if !exists {
						ch <- res{err: fmt.Errorf("beacon %d: Expected state for head block", i)}
						return
					}

					var versionedBlock eth2api.VersionedSignedBeaconBlock
					if exists, err := beaconapi.BlockV2(ctx, b.API, eth2api.BlockIdRoot(headInfo.Root), &versionedBlock); err != nil {
						ch <- res{err: fmt.Errorf("beacon %d: failed to retrieve block: %v", i, err)}
						return
					} else if !exists {
						ch <- res{err: fmt.Errorf("beacon %d: block not found", i)}
						return
					}
					switch versionedBlock.Version {
					case "phase0":
						execution = "0x0000..0000"
					case "altair":
						execution = "0x0000..0000"
					case "bellatrix":
						block := versionedBlock.Data.(*bellatrix.SignedBeaconBlock)
						execution = shorten(block.Message.Body.ExecutionPayload.BlockHash.String())
					}

					slot = headInfo.Header.Message.Slot
					head = shorten(headInfo.Root.String())
					justified = shorten(checkpoints.CurrentJustified.String())
					finalized = shorten(checkpoints.Finalized.String())
					health, err := getHealth(ctx, b.API, t.spec, slot)
					if err != nil {
						ch <- res{err: fmt.Errorf("beacon %d: %s", i, err)}
					}

					ch <- res{i, fmt.Sprintf("beacon %d: slot=%d, head=%s, health=%.2f, exec_payload=%s, justified=%s, finalized=%s", i, slot, head, health, execution, justified, finalized), nil}

					ep := t.spec.SlotToEpoch(slot)
					if ep > 4 && ep > checkpoints.Finalized.Epoch+2 {
						ch <- res{err: fmt.Errorf("failed to finalize, head slot %d (epoch %d) is more than 2 ahead of finality checkpoint %d", slot, ep, checkpoints.Finalized.Epoch)}
					}

					if (checkpoints.Finalized != common.Checkpoint{}) {
						done <- checkpoints.Finalized
					}
				}(ctx, i, b, ch)
			}
			wg.Wait()
			close(ch)

			// print out logs in ascending idx order
			sorted := make([]string, len(t.beacons))
			for out := range ch {
				if out.err != nil {
					return common.Checkpoint{}, out.err
				}
				sorted[out.idx] = out.msg
			}
			for _, msg := range sorted {
				t.t.Logf(msg)
			}
		}
	}
}

// VerifyParticipation ensures that the participation of the finialized epoch
// of a given checkpoint is above the expected threshold.
func (t *Testnet) VerifyParticipation(ctx context.Context, checkpoint common.Checkpoint, expected float64) error {
	slot, _ := t.spec.EpochStartSlot(checkpoint.Epoch + 1)
	if t.spec.BELLATRIX_FORK_EPOCH <= checkpoint.Epoch {
		// slot-1 to target last slot in finalized epoch
		slot = slot - 1
	}
	for i, b := range t.beacons {
		health, err := getHealth(ctx, b.API, t.spec, slot)
		if err != nil {
			return err
		}
		if health < expected {
			return fmt.Errorf("beacon %d: participation not healthy (got: %.2f, expected: %.2f)", i, health, expected)
		}
		t.t.Logf("beacon %d: epoch=%d participation=%.2f", i, checkpoint.Epoch, health)
	}
	return nil
}

// VerifyExecutionPayloadIsCanonical retrieves the execution payload from the
// finalized block and verifies that is in the execution client's canonical
// chain.
func (t *Testnet) VerifyExecutionPayloadIsCanonical(ctx context.Context, checkpoint common.Checkpoint) error {
	var versionedBlock eth2api.VersionedSignedBeaconBlock
	if exists, err := beaconapi.BlockV2(ctx, t.beacons[0].API, eth2api.BlockIdRoot(checkpoint.Root), &versionedBlock); err != nil {
		return fmt.Errorf("beacon %d: failed to retrieve block: %v", 0, err)
	} else if !exists {
		return fmt.Errorf("beacon %d: block not found", 0)
	}
	if versionedBlock.Version != "bellatrix" {
		return nil
	}
	payload := versionedBlock.Data.(*bellatrix.SignedBeaconBlock).Message.Body.ExecutionPayload

	for i, e := range t.eth1 {
		client := ethclient.NewClient(e.Client.RPC())
		block, err := client.BlockByNumber(ctx, big.NewInt(int64(payload.BlockNumber)))
		if err != nil {
			return fmt.Errorf("eth1 %d: %s", 0, err)
		}
		if block.Hash() != [32]byte(payload.BlockHash) {
			return fmt.Errorf("eth1 %d: blocks don't match (got=%s, expected=%s)", i, shorten(block.Hash().String()), shorten(payload.BlockHash.String()))
		}
	}
	return nil
}

// VerifyProposers checks that all validator clients have proposed a block on
// the finalized beacon chain that includes an execution payload.
func (t *Testnet) VerifyProposers(ctx context.Context, checkpoint common.Checkpoint) error {
	proposers := make([]bool, len(t.beacons))
	for slot := 0; slot < int(t.spec.SLOTS_PER_EPOCH)*int(checkpoint.Epoch); slot += 1 {
		var versionedBlock eth2api.VersionedSignedBeaconBlock
		if exists, err := beaconapi.BlockV2(ctx, t.beacons[0].API, eth2api.BlockIdSlot(slot), &versionedBlock); err != nil {
			return fmt.Errorf("beacon %d: failed to retrieve block: %v", 0, err)
		} else if !exists {
			return fmt.Errorf("beacon %d: block not found", 0)
		}
		var proposerIndex common.ValidatorIndex
		switch versionedBlock.Version {
		case "phase0":
			block := versionedBlock.Data.(*phase0.SignedBeaconBlock)
			proposerIndex = block.Message.ProposerIndex
		case "altair":
			block := versionedBlock.Data.(*altair.SignedBeaconBlock)
			proposerIndex = block.Message.ProposerIndex
		case "bellatrix":
			block := versionedBlock.Data.(*bellatrix.SignedBeaconBlock)
			proposerIndex = block.Message.ProposerIndex
		}

		var validator eth2api.ValidatorResponse
		if exists, err := beaconapi.StateValidator(ctx, t.beacons[0].API, eth2api.StateIdSlot(slot), eth2api.ValidatorIdIndex(proposerIndex), &validator); err != nil {
			return fmt.Errorf("beacon %d: failed to retrieve validator: %v", 0, err)
		} else if !exists {
			return fmt.Errorf("beacon %d: validator not found", 0)
		}
		idx, err := t.ValidatorClientIndex([48]byte(validator.Validator.Pubkey))
		if err != nil {
			return fmt.Errorf("pub key not found on any validator client")
		}
		proposers[idx] = true
	}
	for i, proposed := range proposers {
		if !proposed {
			return fmt.Errorf("beacon %d: did not propose a block", i)
		}
	}
	return nil
}

func getHealth(ctx context.Context, api *eth2api.Eth2HttpClient, spec *common.Spec, slot common.Slot) (float64, error) {
	var (
		health    float64
		stateInfo eth2api.VersionedBeaconState
	)
	if exists, err := debugapi.BeaconStateV2(ctx, api, eth2api.StateIdSlot(slot), &stateInfo); err != nil {
		return 0, fmt.Errorf("failed to retrieve state: %v", err)
	} else if !exists {
		return 0, fmt.Errorf("block not found")
	}
	switch stateInfo.Version {
	case "phase0":
		state := stateInfo.Data.(*phase0.BeaconState)
		epoch := spec.SlotToEpoch(slot)
		validatorIds := make([]eth2api.ValidatorId, 0, len(state.Validators))
		for id, validator := range state.Validators {
			if epoch >= validator.ActivationEligibilityEpoch && epoch < validator.ExitEpoch && !validator.Slashed {
				validatorIds = append(validatorIds, eth2api.ValidatorIdIndex(id))
			}
		}
		var (
			beforeEpoch    = 0
			afterEpoch     = spec.SlotToEpoch(slot)
			balancesBefore []eth2api.ValidatorBalanceResponse
			balancesAfter  []eth2api.ValidatorBalanceResponse
		)

		// If it's genesis, keep before also set to 0.
		if afterEpoch != 0 {
			beforeEpoch = int(spec.SlotToEpoch(slot)) - 1
		}
		if exists, err := beaconapi.StateValidatorBalances(ctx, api, eth2api.StateIdSlot(beforeEpoch*int(spec.SLOTS_PER_EPOCH)), validatorIds, &balancesBefore); err != nil {
			return 0, fmt.Errorf("failed to retrieve validator balances: %v", err)
		} else if !exists {
			return 0, fmt.Errorf("validator balances not found")
		}
		if exists, err := beaconapi.StateValidatorBalances(ctx, api, eth2api.StateIdSlot(int(afterEpoch)*int(spec.SLOTS_PER_EPOCH)), validatorIds, &balancesAfter); err != nil {
			return 0, fmt.Errorf("failed to retrieve validator balances: %v", err)
		} else if !exists {
			return 0, fmt.Errorf("validator balances not found")
		}
		health = legacyCalcHealth(spec, balancesBefore, balancesAfter)
	case "altair":
		state := stateInfo.Data.(*altair.BeaconState)
		health = calcHealth(state.CurrentEpochParticipation)
	case "bellatrix":
		state := stateInfo.Data.(*bellatrix.BeaconState)
		health = calcHealth(state.CurrentEpochParticipation)
	}
	return health, nil
}

func calcHealth(p altair.ParticipationRegistry) float64 {
	sum := 0
	for _, p := range p {
		sum += int(p)
	}
	avg := float64(sum) / float64(len(p))
	return avg / float64(MAX_PARTICIPATION_SCORE)
}

// legacyCalcHealth calculates the health of the network based on balances at
// the beginning of an epoch versus the balances at the end.
//
// NOTE: this isn't strictly the most correct way of doing things, but it is
// quite accurate and doesn't require implementing the attestation processing
// logic here.
func legacyCalcHealth(spec *common.Spec, before, after []eth2api.ValidatorBalanceResponse) float64 {
	sum_before := big.NewInt(0)
	sum_after := big.NewInt(0)
	for i := range before {
		sum_before.Add(sum_before, big.NewInt(int64(before[i].Balance)))
		sum_after.Add(sum_after, big.NewInt(int64(after[i].Balance)))
	}
	count := big.NewInt(int64(len(before)))
	avg_before := big.NewInt(0).Div(sum_before, count).Uint64()
	avg_after := sum_after.Div(sum_after, count).Uint64()
	reward := avg_before * spec.BASE_REWARD_FACTOR / math.IntegerSquareRootPrysm(sum_before.Uint64()) / spec.HYSTERESIS_QUOTIENT
	return float64(avg_after-avg_before) / float64(reward*common.BASE_REWARDS_PER_EPOCH)
}
