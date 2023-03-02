package consensus_config

import (
	"fmt"

	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/holiman/uint256"
	"github.com/protolambda/zrnt/eth2/beacon/altair"
	"github.com/protolambda/zrnt/eth2/beacon/bellatrix"
	"github.com/protolambda/zrnt/eth2/beacon/capella"
	"github.com/protolambda/zrnt/eth2/beacon/common"
	"github.com/protolambda/zrnt/eth2/beacon/phase0"
	"github.com/protolambda/zrnt/eth2/configs"
	"github.com/protolambda/ztyp/tree"
	"github.com/protolambda/ztyp/view"
)

func genesisPayloadHeader(
	eth1GenesisBlock *types.Block,
	spec *common.Spec,
) (*bellatrix.ExecutionPayloadHeader, error) {
	extra := eth1GenesisBlock.Extra()
	if len(extra) > common.MAX_EXTRA_DATA_BYTES {
		return nil, fmt.Errorf(
			"extra data is %d bytes, max is %d",
			len(extra),
			common.MAX_EXTRA_DATA_BYTES,
		)
	}
	if len(eth1GenesisBlock.Transactions()) != 0 {
		return nil, fmt.Errorf(
			"expected no transactions in genesis execution payload",
		)
	}

	baseFee, overflow := uint256.FromBig(eth1GenesisBlock.BaseFee())
	if overflow {
		return nil, fmt.Errorf("basefee larger than 2^256-1")
	}

	return &bellatrix.ExecutionPayloadHeader{
		ParentHash:    common.Root(eth1GenesisBlock.ParentHash()),
		FeeRecipient:  common.Eth1Address(eth1GenesisBlock.Coinbase()),
		StateRoot:     common.Bytes32(eth1GenesisBlock.Root()),
		ReceiptsRoot:  common.Bytes32(eth1GenesisBlock.ReceiptHash()),
		LogsBloom:     common.LogsBloom(eth1GenesisBlock.Bloom()),
		PrevRandao:    common.Bytes32{},
		BlockNumber:   view.Uint64View(eth1GenesisBlock.NumberU64()),
		GasLimit:      view.Uint64View(eth1GenesisBlock.GasLimit()),
		GasUsed:       view.Uint64View(eth1GenesisBlock.GasUsed()),
		Timestamp:     common.Timestamp(eth1GenesisBlock.Time()),
		ExtraData:     extra,
		BaseFeePerGas: view.Uint256View(*baseFee),
		BlockHash:     common.Root(eth1GenesisBlock.Hash()),
		// empty transactions root
		TransactionsRoot: common.PayloadTransactionsType(spec).
			DefaultNode().
			MerkleRoot(tree.GetHashFn()),
	}, nil
}

func genesisPayloadHeaderCapella(
	eth1GenesisBlock *types.Block,
	spec *common.Spec,
) (*capella.ExecutionPayloadHeader, error) {
	extra := eth1GenesisBlock.Extra()
	if len(extra) > common.MAX_EXTRA_DATA_BYTES {
		return nil, fmt.Errorf(
			"extra data is %d bytes, max is %d",
			len(extra),
			common.MAX_EXTRA_DATA_BYTES,
		)
	}
	if len(eth1GenesisBlock.Transactions()) != 0 {
		return nil, fmt.Errorf(
			"expected no transactions in genesis execution payload",
		)
	}
	if len(eth1GenesisBlock.Withdrawals()) != 0 {
		return nil, fmt.Errorf(
			"expected no withdrawals in genesis execution payload",
		)
	}

	baseFee, overflow := uint256.FromBig(eth1GenesisBlock.BaseFee())
	if overflow {
		return nil, fmt.Errorf("basefee larger than 2^256-1")
	}

	return &capella.ExecutionPayloadHeader{
		ParentHash:    common.Root(eth1GenesisBlock.ParentHash()),
		FeeRecipient:  common.Eth1Address(eth1GenesisBlock.Coinbase()),
		StateRoot:     common.Bytes32(eth1GenesisBlock.Root()),
		ReceiptsRoot:  common.Bytes32(eth1GenesisBlock.ReceiptHash()),
		LogsBloom:     common.LogsBloom(eth1GenesisBlock.Bloom()),
		PrevRandao:    common.Bytes32{},
		BlockNumber:   view.Uint64View(eth1GenesisBlock.NumberU64()),
		GasLimit:      view.Uint64View(eth1GenesisBlock.GasLimit()),
		GasUsed:       view.Uint64View(eth1GenesisBlock.GasUsed()),
		Timestamp:     common.Timestamp(eth1GenesisBlock.Time()),
		ExtraData:     extra,
		BaseFeePerGas: view.Uint256View(*baseFee),
		BlockHash:     common.Root(eth1GenesisBlock.Hash()),
		// empty transactions root
		TransactionsRoot: common.PayloadTransactionsType(spec).
			DefaultNode().
			MerkleRoot(tree.GetHashFn()),
		// empty withdrawals root
		WithdrawalsRoot: common.WithdrawalsType(spec).
			DefaultNode().
			MerkleRoot(tree.GetHashFn()),
	}, nil
}

func createValidators(
	spec *common.Spec,
	keys []*KeyDetails,
) []phase0.KickstartValidatorData {
	validators := make([]phase0.KickstartValidatorData, 0, len(keys))
	for _, key := range keys {
		validators = append(validators, phase0.KickstartValidatorData{
			Pubkey:                key.ValidatorPubkey,
			WithdrawalCredentials: key.WithdrawalCredentials(),
			Balance:               spec.MAX_EFFECTIVE_BALANCE + key.ExtraInitialBalance,
		})
	}
	return validators
}

// BuildBeaconState creates a beacon state, with either ExecutionFromGenesis or NoExecutionFromGenesis, the given timestamp, and validators derived from the given keys.
// The deposit contract will be recognized as an empty tree, ready for new deposits, thus skipping any transactions for pre-mined validators.
//
// TODO: instead of providing a eth1 genesis, provide an eth1 chain, so we can simulate a merge genesis state that embeds an existing eth1 chain.
func BuildBeaconState(
	spec *common.Spec,
	eth1Genesis *core.Genesis,
	eth2GenesisTime common.Timestamp,
	keys []*KeyDetails,
) (common.BeaconState, error) {
	if uint64(len(keys)) < uint64(spec.MIN_GENESIS_ACTIVE_VALIDATOR_COUNT) {
		return nil, fmt.Errorf(
			"not enough validator keys for genesis. Got %d, but need at least %d",
			len(keys),
			spec.MIN_GENESIS_ACTIVE_VALIDATOR_COUNT,
		)
	}

	eth1GenesisBlock := eth1Genesis.ToBlock()
	eth1BlockHash := common.Root(eth1GenesisBlock.Hash())

	validators := createValidators(spec, keys)

	hFn := tree.GetHashFn()

	var state common.BeaconState
	var forkVersion common.Version
	var previousForkVersion common.Version
	var emptyBodyRoot common.Root
	if spec.CAPELLA_FORK_EPOCH == 0 {
		stateView := capella.NewBeaconStateView(spec)
		forkVersion = spec.CAPELLA_FORK_VERSION
		previousForkVersion = spec.BELLATRIX_FORK_VERSION
		emptyBodyRoot = capella.BeaconBlockBodyType(configs.Mainnet).
			New().
			HashTreeRoot(hFn)
		// TODO: Check if we need to add execution payload to the state
		state = stateView
	} else if spec.BELLATRIX_FORK_EPOCH == 0 {
		stateView := bellatrix.NewBeaconStateView(spec)
		forkVersion = spec.BELLATRIX_FORK_VERSION
		previousForkVersion = spec.ALTAIR_FORK_VERSION
		emptyBodyRoot = bellatrix.BeaconBlockBodyType(configs.Mainnet).
			New().
			HashTreeRoot(hFn)
		state = stateView
	} else if spec.ALTAIR_FORK_EPOCH == 0 {
		state = bellatrix.NewBeaconStateView(spec)
		forkVersion = spec.ALTAIR_FORK_VERSION
		previousForkVersion = spec.GENESIS_FORK_VERSION
		emptyBodyRoot = altair.BeaconBlockBodyType(configs.Mainnet).
			New().
			HashTreeRoot(hFn)
	} else {
		state = phase0.NewBeaconStateView(spec)
		forkVersion = spec.GENESIS_FORK_VERSION
		previousForkVersion = spec.GENESIS_FORK_VERSION
		emptyBodyRoot = phase0.BeaconBlockBodyType(configs.Mainnet).
			New().
			HashTreeRoot(hFn)
	}

	if err := state.SetGenesisTime(eth2GenesisTime); err != nil {
		return nil, err
	}

	if err := state.SetFork(common.Fork{
		PreviousVersion: previousForkVersion,
		CurrentVersion:  forkVersion,
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
	if err := state.SetLatestBlockHeader(&common.BeaconBlockHeader{BodyRoot: emptyBodyRoot}); err != nil {
		return nil, err
	}
	// Seed RANDAO with Eth1 entropy
	if err := state.SeedRandao(spec, eth1BlockHash); err != nil {
		return nil, err
	}

	for _, v := range validators {
		if err := state.AddValidator(spec, v.Pubkey, v.WithdrawalCredentials, v.Balance); err != nil {
			return nil, err
		}
	}
	vals, err := state.Validators()
	if err != nil {
		return nil, err
	}
	// Process activations and exits
	for i := 0; i < len(validators); i++ {
		val, err := vals.Validator(common.ValidatorIndex(i))
		if err != nil {
			return nil, err
		}
		vEff, err := val.EffectiveBalance()
		if err != nil {
			return nil, err
		}
		if vEff == spec.MAX_EFFECTIVE_BALANCE {
			if err := val.SetActivationEligibilityEpoch(common.GENESIS_EPOCH); err != nil {
				return nil, err
			}
			if err := val.SetActivationEpoch(common.GENESIS_EPOCH); err != nil {
				return nil, err
			}
		}
		// Process exits/slashings
		slashings, err := state.Slashings()
		if err != nil {
			return nil, err
		}
		if keys[i].Exited || keys[i].Slashed {
			exit_epoch := common.GENESIS_EPOCH
			val.SetExitEpoch(exit_epoch)
			val.SetWithdrawableEpoch(
				exit_epoch + spec.MIN_VALIDATOR_WITHDRAWABILITY_DELAY,
			)
			if keys[i].Slashed {
				val.MakeSlashed()

				bal, err := val.EffectiveBalance()
				if err != nil {
					return nil, err
				}

				if err := slashings.AddSlashing(exit_epoch, bal); err != nil {
					return nil, err
				}
			}
		}
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

	switch st := state.(type) {
	case *bellatrix.BeaconStateView:
		tdd := uint256.Int(spec.TERMINAL_TOTAL_DIFFICULTY)
		embedExecAtGenesis := tdd.ToBig().Cmp(eth1Genesis.Difficulty) <= 0

		var execPayloadHeader *bellatrix.ExecutionPayloadHeader
		if embedExecAtGenesis {
			execPayloadHeader, err = genesisPayloadHeader(
				eth1GenesisBlock,
				spec,
			)
			if err != nil {
				return nil, err
			}
		} else {
			// we didn't build any on the eth1 chain though,
			// so we just put the genesis hash here (it could be any block from eth1 chain before TTD that is not ahead of eth2)
			execPayloadHeader = new(bellatrix.ExecutionPayloadHeader)
		}

		if err := st.SetLatestExecutionPayloadHeader(execPayloadHeader); err != nil {
			return nil, err
		}
	case *capella.BeaconStateView:
		// did we hit the TTD at genesis block?
		tdd := uint256.Int(spec.TERMINAL_TOTAL_DIFFICULTY)
		embedExecAtGenesis := tdd.ToBig().Cmp(eth1Genesis.Difficulty) <= 0

		var execPayloadHeader *capella.ExecutionPayloadHeader
		if embedExecAtGenesis {
			execPayloadHeader, err = genesisPayloadHeaderCapella(
				eth1GenesisBlock,
				spec,
			)
			if err != nil {
				return nil, err
			}
		} else {
			// we didn't build any on the eth1 chain though,
			// so we just put the genesis hash here (it could be any block from eth1 chain before TTD that is not ahead of eth2)
			execPayloadHeader = new(capella.ExecutionPayloadHeader)
		}

		if err := st.SetLatestExecutionPayloadHeader(execPayloadHeader); err != nil {
			return nil, err
		}
	}

	return state, nil
}
