package genesis

import (
	"fmt"

	"github.com/ethereum/go-ethereum/core/types"
	consensus_config "github.com/ethereum/hive/simulators/eth2/common/config/consensus"
	"github.com/ethereum/hive/simulators/eth2/common/config/consensus/genesis/bellatrix"
	"github.com/ethereum/hive/simulators/eth2/common/config/consensus/genesis/capella"
	"github.com/ethereum/hive/simulators/eth2/common/config/consensus/genesis/deneb"
	"github.com/ethereum/hive/simulators/eth2/common/config/consensus/genesis/interfaces"
	"github.com/protolambda/zrnt/eth2/beacon/common"
	"github.com/protolambda/zrnt/eth2/beacon/phase0"
	"github.com/protolambda/ztyp/tree"
)

// BuildBeaconState creates a beacon state, with either ExecutionFromGenesis or NoExecutionFromGenesis, the given timestamp, and validators derived from the given keys.
// The deposit contract will be recognized as an empty tree, ready for new deposits, thus skipping any transactions for pre-mined validators.
//
// TODO: instead of providing a eth1 genesis, provide an eth1 chain, so we can simulate a merge genesis state that embeds an existing eth1 chain.
func BuildBeaconState(
	spec *common.Spec,
	executionGenesis *types.Block,
	beaconGenesisTime common.Timestamp,
	keys consensus_config.ValidatorsSetupDetails,
) (common.BeaconState, error) {
	if uint64(len(keys)) < uint64(spec.MIN_GENESIS_ACTIVE_VALIDATOR_COUNT) {
		return nil, fmt.Errorf(
			"not enough validator keys for genesis. Got %d, but need at least %d",
			len(keys),
			spec.MIN_GENESIS_ACTIVE_VALIDATOR_COUNT,
		)
	}

	eth1BlockHash := common.Root(executionGenesis.Hash())

	var state interfaces.StateViewGenesis
	if spec.DENEB_FORK_EPOCH == 0 {
		state = deneb.NewBeaconStateView(spec)
	} else if spec.CAPELLA_FORK_EPOCH == 0 {
		state = capella.NewBeaconStateView(spec)
	} else if spec.BELLATRIX_FORK_EPOCH == 0 {
		state = bellatrix.NewBeaconStateView(spec)
	} else {
		return nil, fmt.Errorf("pre-merge not supported")
	}

	if err := state.SetGenesisTime(beaconGenesisTime); err != nil {
		return nil, err
	}

	if err := state.SetFork(common.Fork{
		PreviousVersion: state.PreviousForkVersion(),
		CurrentVersion:  state.ForkVersion(),
		Epoch:           common.GENESIS_EPOCH,
	}); err != nil {
		return nil, err
	}
	// Empty deposit-tree
	eth1Dat := common.Eth1Data{
		DepositRoot: phase0.NewDepositRootsView().
			HashTreeRoot(tree.GetHashFn()),
		DepositCount: 0,
		BlockHash:    eth1BlockHash,
	}
	if err := state.SetEth1Data(eth1Dat); err != nil {
		return nil, err
	}
	// sanity check: Leave the deposit index to 0. No deposits happened.
	if i, err := state.Eth1DepositIndex(); err != nil {
		return nil, err
	} else if i != 0 {
		return nil, fmt.Errorf("expected 0 deposit index in state, got %d", i)
	}
	if err := state.SetLatestBlockHeader(&common.BeaconBlockHeader{BodyRoot: state.EmptyBodyRoot()}); err != nil {
		return nil, err
	}
	// Seed RANDAO with Eth1 entropy
	if err := state.SeedRandao(spec, eth1BlockHash); err != nil {
		return nil, err
	}

	if err := keys.AddToGenesisState(spec, state); err != nil {
		return nil, err
	}

	vals, err := state.Validators()
	if err != nil {
		return nil, err
	}

	if err := state.SetGenesisValidatorsRoot(vals.HashTreeRoot(tree.GetHashFn())); err != nil {
		return nil, err
	}
	if st, ok := state.(common.SyncCommitteeBeaconState); ok {
		indicesBounded, err := common.LoadBoundedIndices(vals)
		if err != nil {
			return nil, err
		}
		active := common.ActiveIndices(indicesBounded, common.GENESIS_EPOCH)
		indices, err := common.ComputeSyncCommitteeIndices(
			spec,
			state,
			common.GENESIS_EPOCH,
			active,
		)
		if err != nil {
			return nil, fmt.Errorf(
				"failed to compute sync committee indices: %v",
				err,
			)
		}
		pubs, err := common.NewPubkeyCache(vals)
		if err != nil {
			return nil, err
		}
		// Note: A duplicate committee is assigned for the current and next committee at genesis
		syncCommittee, err := common.IndicesToSyncCommittee(indices, pubs)
		if err != nil {
			return nil, err
		}
		syncCommitteeView, err := syncCommittee.View(spec)
		if err != nil {
			return nil, err
		}
		if err := st.SetCurrentSyncCommittee(syncCommitteeView); err != nil {
			return nil, err
		}
		if err := st.SetNextSyncCommittee(syncCommitteeView); err != nil {
			return nil, err
		}
	}

	// Set execution payload header
	if err := state.SetGenesisExecutionHeader(executionGenesis); err != nil {
		return nil, err
	}

	return state, nil
}
