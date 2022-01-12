package main

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/ethereum/hive/hivesim"
	"github.com/ethereum/hive/simulators/eth2/testnet/setup"
	"github.com/protolambda/eth2api"
	"github.com/protolambda/eth2api/client/beaconapi"
	"github.com/protolambda/zrnt/eth2/beacon/common"
	"github.com/protolambda/zrnt/eth2/beacon/merge"
)

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

			type res struct {
				idx int
				msg string
			}
			// new slot, log and check status of all beacon nodes
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

					var blockInfo eth2api.VersionedSignedBeaconBlock
					if exists, err := beaconapi.BlockV2(ctx, b.API, eth2api.BlockIdRoot(headInfo.Root), &blockInfo); err != nil {
						t.t.Errorf("beacon %d: failed to retrieve block: %v", i, err)
						return
					} else if !exists {
						t.t.Fatalf("beacon %d: block not found", i)
					}
					mergeBlock := blockInfo.Data.(*merge.SignedBeaconBlock)

					var out eth2api.FinalityCheckpoints
					if exists, err := beaconapi.FinalityCheckpoints(ctx, b.API, eth2api.StateIdRoot(headInfo.Header.Message.StateRoot), &out); err != nil {
						t.t.Errorf("beacon %d: failed to poll finality checkpoint: %v", i, err)
						return
					} else if !exists {
						t.t.Fatalf("beacon %d: Expected state for head block", i)
					}

					slot := headInfo.Header.Message.Slot
					head := shorten(headInfo.Root.String())
					justified := shorten(out.CurrentJustified.String())
					finalized := shorten(out.Finalized.String())
					execution := shorten(mergeBlock.Message.Body.ExecutionPayload.BlockHash.String())
					ch <- res{i, fmt.Sprintf("beacon %d: slot=%d, head=%s, exec_payload=%s, justified=%s, finalized=%s", i, slot, head, execution, justified, finalized)}

					ep := t.spec.SlotToEpoch(slot)
					if ep > 4 && ep > out.Finalized.Epoch+2 {
						t.t.Fatalf("failed to finalize, head slot %d (epoch %d) is more than 2 ahead of finality checkpoint %d", slot, ep, out.Finalized.Epoch)
					}

					if (out.Finalized != common.Checkpoint{}) {
						done <- struct{}{}
					}
				}(ctx, i, b, ch)
			}
			wg.Wait()
			close(ch)
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
