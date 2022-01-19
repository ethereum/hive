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
	"github.com/protolambda/zrnt/eth2/beacon/common"
	"github.com/protolambda/zrnt/eth2/beacon/merge"
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

func (t *Testnet) VerifyFinality(ctx context.Context) {
	genesis := t.GenesisTime()
	slotDuration := time.Duration(t.spec.SECONDS_PER_SLOT) * time.Second
	timer := time.NewTicker(slotDuration)
	done := make(chan struct{}, len(t.beacons))

	for {
		select {
		case <-ctx.Done():
			return
		case <-done:
			return
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

					var headInfo eth2api.BeaconBlockHeaderAndInfo
					if exists, err := beaconapi.BlockHeader(ctx, b.API, eth2api.BlockHead, &headInfo); err != nil {
						t.t.Errorf("beacon %d: failed to poll head: %v", i, err)
					} else if !exists {
						t.t.Fatalf("beacon %d: no head block", i)
					}

					var checkpoints eth2api.FinalityCheckpoints
					if exists, err := beaconapi.FinalityCheckpoints(ctx, b.API, eth2api.StateIdRoot(headInfo.Header.Message.StateRoot), &checkpoints); err != nil {
						t.t.Errorf("beacon %d: failed to poll finality checkpoint: %v", i, err)
						return
					} else if !exists {
						t.t.Fatalf("beacon %d: Expected state for head block", i)
					}

					var versionedBlock eth2api.VersionedSignedBeaconBlock
					if exists, err := beaconapi.BlockV2(ctx, b.API, eth2api.BlockIdRoot(headInfo.Root), &versionedBlock); err != nil {
						t.t.Errorf("beacon %d: failed to retrieve block: %v", i, err)
						return
					} else if !exists {
						t.t.Fatalf("beacon %d: block not found", i)
					}
					block := versionedBlock.Data.(*merge.SignedBeaconBlock)

					var stateInfo eth2api.VersionedBeaconState
					if exists, err := debugapi.BeaconStateV2(ctx, b.API, eth2api.StateIdRoot(headInfo.Header.Message.StateRoot), &stateInfo); err != nil {
						t.t.Errorf("beacon %d: failed to retrieve state: %v", i, err)
						return
					} else if !exists {
						t.t.Fatalf("beacon %d: block not found", i)
					}
					state := stateInfo.Data.(*merge.BeaconState)

					slot := headInfo.Header.Message.Slot
					head := shorten(headInfo.Root.String())
					justified := shorten(checkpoints.CurrentJustified.String())
					finalized := shorten(checkpoints.Finalized.String())
					execution := shorten(block.Message.Body.ExecutionPayload.BlockHash.String())
					health := calcHealth(state.CurrentEpochParticipation)

					ch <- res{i, fmt.Sprintf("beacon %d: slot=%d, head=%s, health=%.2f, exec_payload=%s, justified=%s, finalized=%s", i, slot, head, health, execution, justified, finalized)}

					ep := t.spec.SlotToEpoch(slot)
					if ep > 4 && ep > checkpoints.Finalized.Epoch+2 {
						t.t.Fatalf("failed to finalize, head slot %d (epoch %d) is more than 2 ahead of finality checkpoint %d", slot, ep, checkpoints.Finalized.Epoch)
					}

					if (checkpoints.Finalized != common.Checkpoint{}) {
						done <- struct{}{}
					}
				}(ctx, i, b, ch)
			}
			wg.Wait()
			close(ch)

			// print out logs in ascending idx order
			sorted := make([]string, len(t.beacons))
			for out := range ch {
				sorted[out.idx] = out.msg
			}
			for _, msg := range sorted {
				t.t.Logf(msg)
			}
		}
	}
}

func (t *Testnet) VerifyParticipation(ctx context.Context, epoch int, expected int) {
	slot := (epoch + 1) * int(t.spec.SLOTS_PER_EPOCH)
	for i, b := range t.beacons {
		var versionedState eth2api.VersionedBeaconState
		if exists, err := debugapi.BeaconStateV2(ctx, b.API, eth2api.StateIdSlot(slot), &versionedState); err != nil {
			t.t.Errorf("beacon %d: failed to retrieve state: %v", i, err)
			return
		} else if !exists {
			t.t.Fatalf("beacon %d: block not found", i)
		}
		state := versionedState.Data.(*merge.BeaconState)
		health := int(calcHealth(state.PreviousEpochParticipation) * 100)
		if health < expected {
			t.t.Fatalf("beacon %d: participation not healthy (got: %d, expected: %d)", i, health, expected)
		}
		t.t.Logf("beacon %d: epoch=%d participation=%d", i, epoch, health)
	}
}

func (t *Testnet) VerifyExecutionPayloadIsCanonical(ctx context.Context) {
	var versionedBlock eth2api.VersionedSignedBeaconBlock
	if exists, err := beaconapi.BlockV2(ctx, t.beacons[0].API, eth2api.BlockFinalized, &versionedBlock); err != nil {
		t.t.Errorf("beacon %d: failed to retrieve block: %v", 0, err)
		return
	} else if !exists {
		t.t.Fatalf("beacon %d: block not found", 0)
	}
	payload := versionedBlock.Data.(*merge.SignedBeaconBlock).Message.Body.ExecutionPayload

	for i, e := range t.eth1 {
		client := ethclient.NewClient(e.Client.RPC())
		block, err := client.BlockByNumber(ctx, big.NewInt(int64(payload.BlockNumber)))
		if err != nil {
			t.t.Fatalf("eth1 %d: %s", err)
		}
		if block.Hash() != [32]byte(payload.BlockHash) {
			t.t.Fatalf("eth1 %d: blocks don't match (got=%s, expected=%s)", i, shorten(block.Hash().String()), shorten(payload.BlockHash.String()))
		}
	}

}

func (t *Testnet) VerifyProposers(ctx context.Context) {
	proposers := make([]bool, len(t.beacons))
	var checkpoints eth2api.FinalityCheckpoints
	if exists, err := beaconapi.FinalityCheckpoints(ctx, t.beacons[0].API, eth2api.StateHead, &checkpoints); err != nil {
		t.t.Errorf("beacon %d: failed to poll finality checkpoint: %v", 0, err)
		return
	} else if !exists {
		t.t.Fatalf("beacon %d: Expected state for head block")
	}
	finalized, _ := t.spec.EpochStartSlot(checkpoints.Finalized.Epoch)
	for slot := 0; slot < int(finalized+t.spec.SLOTS_PER_EPOCH); slot += 1 {
		var versionedBlock eth2api.VersionedSignedBeaconBlock
		if exists, err := beaconapi.BlockV2(ctx, t.beacons[0].API, eth2api.BlockIdSlot(slot), &versionedBlock); err != nil {
			t.t.Errorf("beacon %d: failed to retrieve block: %v", 0, err)
			return
		} else if !exists {
			t.t.Fatalf("beacon %d: block not found", 0)
		}
		block := versionedBlock.Data.(*merge.SignedBeaconBlock)
		proposerIndex := block.Message.ProposerIndex
		var validator eth2api.ValidatorResponse
		if exists, err := beaconapi.StateValidator(ctx, t.beacons[0].API, eth2api.StateIdSlot(slot), eth2api.ValidatorIdIndex(proposerIndex), &validator); err != nil {
			t.t.Errorf("beacon %d: failed to retrieve validator: %v", 0, err)
			return
		} else if !exists {
			t.t.Fatalf("beacon %d: validator not found", 0)
		}
		idx, err := t.ValidatorClientIndex([48]byte(validator.Validator.Pubkey))
		if err != nil {
			t.t.Fatalf("pub key not found on any validator client")
		}
		proposers[idx] = true
	}
	for i, proposed := range proposers {
		if !proposed {
			t.t.Fatalf("beacon %d: did not propose a block with a valid execution payload", i)
		}
	}
}

func calcHealth(p altair.ParticipationRegistry) float64 {
	sum := 0
	for _, p := range p {
		sum += int(p)
	}
	avg := float64(sum) / float64(len(p))
	return avg / float64(MAX_PARTICIPATION_SCORE)
}
