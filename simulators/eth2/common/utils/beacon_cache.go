package utils

import (
	"bytes"
	"context"
	"fmt"

	beacon_client "github.com/marioevz/eth-clients/clients/beacon"
	"github.com/protolambda/eth2api"
	beacon "github.com/protolambda/zrnt/eth2/beacon/common"
	"github.com/protolambda/ztyp/tree"
)

type BeaconBlockState struct {
	*beacon_client.VersionedBeaconStateResponse
	*beacon_client.VersionedSignedBeaconBlock
}

type BeaconCache map[tree.Root]BeaconBlockState

// Clear the cache for when there was a known/expected re-org to query everything again
func (c BeaconCache) Clear() error {
	roots := make([]tree.Root, len(c))
	i := 0
	for s := range c {
		roots[i] = s
		i++
	}
	for _, s := range roots {
		delete(c, s)
	}
	return nil
}

func (c BeaconCache) GetBlockStateByRoot(
	ctx context.Context,
	bc *beacon_client.BeaconClient,
	blockroot tree.Root,
) (BeaconBlockState, error) {
	if s, ok := c[blockroot]; ok {
		return s, nil
	}
	b, err := bc.BlockV2(ctx, eth2api.BlockIdRoot(blockroot))
	if err != nil {
		return BeaconBlockState{}, err
	}
	s, err := bc.BeaconStateV2(ctx, eth2api.StateIdSlot(b.Slot()))
	if err != nil {
		return BeaconBlockState{}, err
	}
	blockStateRoot := b.StateRoot()
	stateRoot := s.Root()
	if !bytes.Equal(blockStateRoot[:], stateRoot[:]) {
		return BeaconBlockState{}, fmt.Errorf(
			"state root mismatch while fetching state",
		)
	}
	both := BeaconBlockState{
		VersionedBeaconStateResponse: s,
		VersionedSignedBeaconBlock:   b,
	}
	c[blockroot] = both
	return both, nil
}

func (c BeaconCache) GetBlockStateBySlotFromHeadRoot(
	ctx context.Context,
	bc *beacon_client.BeaconClient,
	headblockroot tree.Root,
	slot beacon.Slot,
) (*BeaconBlockState, error) {
	current, err := c.GetBlockStateByRoot(ctx, bc, headblockroot)
	if err != nil {
		return nil, err
	}
	if current.Slot() < slot {
		return nil, fmt.Errorf("requested for slot above head")
	}
	for {
		if current.Slot() == slot {
			return &current, nil
		}
		if current.Slot() < slot || current.Slot() == 0 {
			// Skipped slot probably, not a fatal error
			return nil, nil
		}
		current, err = c.GetBlockStateByRoot(ctx, bc, current.ParentRoot())
		if err != nil {
			return nil, err
		}
	}
}
