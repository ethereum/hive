package main

import (
	"context"
	"github.com/ethereum/hive/hivesim"
	"github.com/ethereum/hive/simulators/eth2/testnet/setup"
	"github.com/protolambda/eth2api"
	"github.com/protolambda/eth2api/client/beaconapi"
	"github.com/protolambda/zrnt/eth2/beacon/common"
	"sync"
	"time"
)

type Testnet struct {
	t *hivesim.T

	genesisTime common.Timestamp
	genesisValidatorsRoot common.Root

	// Consensus chain configuration
	spec                  *common.Spec
	// Execution chain configuration and genesis info
	eth1Genesis           *setup.Eth1Genesis

	beacons    []*BeaconNode
	validators []*ValidatorClient
	eth1       []*Eth1Node
}

func (t *Testnet) GenesisTime() time.Time {
	return time.Unix(int64(t.genesisTime), 0)
}

func (t *Testnet) TrackFinality(ctx context.Context) {

	genesis := t.GenesisTime()
	slotDuration := time.Duration(t.spec.SECONDS_PER_SLOT) * time.Second
	timer := time.NewTicker(slotDuration)

	for {
		select {
		case <- ctx.Done():
			return
		case tim := <- timer.C:
			// start polling after first slot of genesis
			if tim.Before(genesis.Add(slotDuration)) {
				t.t.Logf("time till genesis: %s", genesis.Sub(tim))
				continue
			}

			// new slot, log and check status of all beacon nodes

			var wg sync.WaitGroup
			for i, b := range t.beacons {
				wg.Add(1)
				go func(ctx context.Context, i int, b *BeaconNode) {
					defer wg.Done()
					ctx, _ = context.WithTimeout(ctx, time.Second * 5)

					var headInfo eth2api.BeaconBlockHeaderAndInfo
					if exists, err := beaconapi.BlockHeader(ctx, b.API, eth2api.BlockHead, &headInfo); err != nil {
						t.t.Errorf("[beacon %d] failed to poll head: %v", i, err)
						return
					} else if !exists {
						t.t.Fatalf("[beacon %d] no head block", i)
					}

					var out eth2api.FinalityCheckpoints
					if exists, err := beaconapi.FinalityCheckpoints(ctx, b.API, eth2api.StateIdRoot(headInfo.Header.Message.StateRoot), &out); err != nil {
						t.t.Errorf("[beacon %d] failed to poll finality checkpoint: %v", i, err)
						return
					} else if !exists {
						t.t.Fatalf("[beacon %d] Expected state for head block", i)
					}

					slot := headInfo.Header.Message.Slot
					t.t.Logf("beacon %d: head block root %s, slot %d, justified %s, finalized %s",
						i, headInfo.Root, slot, &out.CurrentJustified, &out.Finalized)

					if ep := t.spec.SlotToEpoch(slot); ep > out.Finalized.Epoch+2 {
						t.t.Errorf("failing to finalize, head slot %d (epoch %d) is more than 2 ahead of finality checkpoint %d", slot, ep, out.Finalized.Epoch)
					}
				}(ctx, i, b)
			}
			wg.Wait()
		}
	}
}
