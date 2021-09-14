package main

import (
	"context"
	"github.com/ethereum/hive/hivesim"
	"github.com/ethereum/hive/simulators/eth2/testnet/setup"
	"github.com/protolambda/zrnt/eth2/beacon/common"
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

func (t *Testnet) ExpectFinality(ctx context.Context) {

	genesis := t.GenesisTime()
	slotDuration := time.Duration(t.spec.SECONDS_PER_SLOT)
	ticker := time.NewTicker(slotDuration)
	// align to the start of the slot
	ticker.Reset(time.Now().Sub(genesis) % slotDuration)

	for {
		select {
		case <- ctx.Done():
			return
		case <- ticker.C:
			// new slot, check finality

			// TODO: ask all beacon nodes if they have finalized
			for _, b := range t.beacons {
				t.t.Errorf("node %s failed to finalize", b.Container)
				// TODO use Go API bindings to request finality status, error per non-finalized node.
			}
		}
	}
}