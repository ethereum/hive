package testnet

import (
	"bytes"
	"context"
	"fmt"
	"math/big"
	"sync"
	"time"

	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/pkg/errors"

	"github.com/protolambda/eth2api"
	"github.com/protolambda/zrnt/eth2/beacon/altair"
	"github.com/protolambda/zrnt/eth2/beacon/common"
	"github.com/protolambda/zrnt/eth2/beacon/phase0"
	"github.com/protolambda/zrnt/eth2/util/math"
	"github.com/protolambda/ztyp/tree"

	"github.com/ethereum/hive/hivesim"
	"github.com/ethereum/hive/simulators/eth2/common/clients"
	execution_config "github.com/ethereum/hive/simulators/eth2/common/config/execution"
	"github.com/ethereum/hive/simulators/eth2/common/utils"
)

const (
	MAX_PARTICIPATION_SCORE = 7
)

var (
	EMPTY_EXEC_HASH = ethcommon.Hash{}
	EMPTY_TREE_ROOT = tree.Root{}
)

type Testnet struct {
	*hivesim.T
	clients.Nodes

	genesisTime           common.Timestamp
	genesisValidatorsRoot common.Root

	// Consensus chain configuration
	spec *common.Spec
	// Execution chain configuration and genesis info
	eth1Genesis *execution_config.ExecutionGenesis
	// Consensus genesis state
	eth2GenesisState common.BeaconState

	// Test configuration
	maxConsecutiveErrorsOnWaits int
}

type ActiveSpec struct {
	*common.Spec
}

func (spec *ActiveSpec) EpochTimeoutContext(
	parent context.Context,
	epochs common.Epoch,
) (context.Context, context.CancelFunc) {
	return context.WithTimeout(
		parent,
		time.Duration(
			uint64(spec.SLOTS_PER_EPOCH*common.Slot(epochs))*
				uint64(spec.SECONDS_PER_SLOT),
		)*time.Second,
	)
}

func (spec *ActiveSpec) SlotTimeoutContext(
	parent context.Context,
	slots common.Slot,
) (context.Context, context.CancelFunc) {
	return context.WithTimeout(
		parent,
		time.Duration(
			uint64(slots)*
				uint64(spec.SECONDS_PER_SLOT))*time.Second,
	)
}

func (spec *ActiveSpec) EpochsTimeout(epochs common.Epoch) <-chan time.Time {
	return time.After(
		time.Duration(
			uint64(
				spec.SLOTS_PER_EPOCH*common.Slot(epochs),
			)*uint64(
				spec.SECONDS_PER_SLOT,
			),
		) * time.Second,
	)
}

func (spec *ActiveSpec) SlotsTimeout(slots common.Slot) <-chan time.Time {
	return time.After(
		time.Duration(
			uint64(slots)*uint64(spec.SECONDS_PER_SLOT),
		) * time.Second,
	)
}

func (t *Testnet) Spec() *ActiveSpec {
	return &ActiveSpec{
		Spec: t.spec,
	}
}

func (t *Testnet) GenesisTime() common.Timestamp {
	// return time.Unix(int64(t.genesisTime), 0)
	return t.genesisTime
}

func (t *Testnet) GenesisTimeUnix() time.Time {
	return time.Unix(int64(t.genesisTime), 0)
}

func (t *Testnet) GenesisBeaconState() common.BeaconState {
	return t.eth2GenesisState
}

func (t *Testnet) GenesisValidatorsRoot() common.Root {
	return t.genesisValidatorsRoot
}

func (t *Testnet) ExecutionGenesis() *core.Genesis {
	return t.eth1Genesis.Genesis
}

func StartTestnet(
	parentCtx context.Context,
	t *hivesim.T,
	env *Environment,
	config *Config,
) *Testnet {
	prep := prepareTestnet(t, env, config)
	testnet := prep.createTestnet(t)
	genesisTime := testnet.GenesisTimeUnix()
	countdown := time.Until(genesisTime)
	t.Logf(
		"Created new testnet, genesis at %s (%s from now)",
		genesisTime,
		countdown,
	)

	testnet.Nodes = make(clients.Nodes, len(config.NodeDefinitions))

	// Init all client bundles
	for nodeIndex := range testnet.Nodes {
		testnet.Nodes[nodeIndex] = new(clients.Node)
	}

	// For each key partition, we start a client bundle that consists of:
	// - 1 execution client
	// - 1 beacon client
	// - 1 validator client,
	for nodeIndex, node := range config.NodeDefinitions {
		// Prepare clients for this node
		var (
			nodeClient = testnet.Nodes[nodeIndex]

			executionDef = env.Clients.ClientByNameAndRole(
				node.ExecutionClientName(),
				"eth1",
			)
			beaconDef = env.Clients.ClientByNameAndRole(
				node.ConsensusClientName(),
				"beacon",
			)
			validatorDef = env.Clients.ClientByNameAndRole(
				node.ValidatorClientName(),
				"validator",
			)
		)

		if executionDef == nil || beaconDef == nil || validatorDef == nil {
			t.Fatalf("FAIL: Unable to get client")
		}

		// Prepare the client objects with all the information necessary to
		// eventually start
		nodeClient.ExecutionClient = prep.prepareExecutionNode(
			parentCtx,
			testnet,
			executionDef,
			config.Eth1Consensus,
			node.ExecutionClientTTD,
			nodeIndex,
			node.Chain,
			node.ExecutionSubnet,
			env.LogEngineCalls,
		)

		if node.ConsensusClient != "" {
			nodeClient.BeaconClient = prep.prepareBeaconNode(
				parentCtx,
				testnet,
				beaconDef,
				node.BeaconNodeTTD,
				nodeIndex,
				config.EnableBuilders,
				config.BuilderOptions,
				nodeClient.ExecutionClient,
			)

			nodeClient.ValidatorClient = prep.prepareValidatorClient(
				parentCtx,
				testnet,
				validatorDef,
				nodeClient.BeaconClient,
				nodeIndex,
			)
		}

		// Add rest of properties
		nodeClient.T = t
		nodeClient.Index = nodeIndex
		nodeClient.Verification = node.TestVerificationNode

		// Start the node clients if specified so
		if !node.DisableStartup {
			nodeClient.Start()
		}
	}

	// Default config
	testnet.maxConsecutiveErrorsOnWaits = 3

	return testnet
}

func (t *Testnet) Stop() {
	for _, p := range t.Proxies().Running() {
		p.Cancel()
	}
	for _, b := range t.BeaconClients() {
		if b.Builder != nil {
			b.Builder.Cancel()
		}
	}
}

func (t *Testnet) ValidatorClientIndex(pk [48]byte) (int, error) {
	for i, v := range t.ValidatorClients() {
		if v.ContainsKey(pk) {
			return i, nil
		}
	}
	return 0, fmt.Errorf("key not found in any validator client")
}

// Wait until the beacon chain genesis happens.
func (t *Testnet) WaitForGenesis(ctx context.Context) {
	genesis := t.GenesisTimeUnix()
	select {
	case <-ctx.Done():
	case <-time.After(time.Until(genesis)):
	}
}

// Wait a certain amount of slots while printing the current status.
func (t *Testnet) WaitSlots(ctx context.Context, slots common.Slot) error {
	for s := common.Slot(0); s < slots; s++ {
		t.BeaconClients().Running().PrintStatus(ctx, t)
		select {
		case <-time.After(time.Duration(t.spec.SECONDS_PER_SLOT) * time.Second):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return nil
}

// WaitForFork blocks until a beacon client reaches specified fork,
// or context finalizes, whichever happens first.
func (t *Testnet) WaitForFork(ctx context.Context, fork string) error {
	var (
		genesis      = t.GenesisTimeUnix()
		slotDuration = time.Duration(t.spec.SECONDS_PER_SLOT) * time.Second
		timer        = time.NewTicker(slotDuration)
		runningNodes = t.VerificationNodes().Running()
		results      = makeResults(runningNodes, t.maxConsecutiveErrorsOnWaits)
	)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case tim := <-timer.C:
			// start polling after first slot of genesis
			if tim.Before(genesis.Add(slotDuration)) {
				t.Logf("Time till genesis: %s", genesis.Sub(tim))
				continue
			}

			// new slot, log and check status of all beacon nodes
			var (
				wg        sync.WaitGroup
				clockSlot = t.spec.TimeToSlot(
					common.Timestamp(time.Now().Unix()),
					t.GenesisTime(),
				)
			)
			results.Clear()

			for i, n := range runningNodes {
				wg.Add(1)
				go func(
					ctx context.Context,
					n *clients.Node,
					r *result,
				) {
					defer wg.Done()

					b := n.BeaconClient

					headInfo, err := b.BlockHeader(ctx, eth2api.BlockHead)
					if err != nil {
						r.err = errors.Wrap(err, "failed to poll head")
						return
					}

					checkpoints, err := b.BlockFinalityCheckpoints(
						ctx,
						eth2api.BlockHead,
					)
					if err != nil {
						r.err = errors.Wrap(
							err,
							"failed to poll finality checkpoint",
						)
						return
					}

					versionedBlock, err := b.BlockV2(
						ctx,
						eth2api.BlockIdRoot(headInfo.Root),
					)
					if err != nil {
						r.err = errors.Wrap(err, "failed to retrieve block")
						return
					}

					execution := ethcommon.Hash{}
					if executionPayload, err := versionedBlock.ExecutionPayload(); err == nil {
						execution = executionPayload.BlockHash
					}

					slot := headInfo.Header.Message.Slot
					if clockSlot > slot &&
						(clockSlot-slot) >= t.spec.SLOTS_PER_EPOCH {
						r.fatal = fmt.Errorf(
							"unable to sync for an entire epoch: clockSlot=%d, slot=%d",
							clockSlot,
							slot,
						)
						return
					}

					r.msg = fmt.Sprintf(
						"fork=%s, clock_slot=%s, slot=%d, head=%s, exec_payload=%s, justified=%s, finalized=%s",
						versionedBlock.Version,
						clockSlot,
						slot,
						utils.Shorten(headInfo.Root.String()),
						utils.Shorten(execution.String()),
						utils.Shorten(checkpoints.CurrentJustified.String()),
						utils.Shorten(checkpoints.Finalized.String()),
					)

					if versionedBlock.Version == fork {
						r.done = true
					}
				}(ctx, n, results[i])
			}
			wg.Wait()

			if err := results.CheckError(); err != nil {
				return err
			}
			results.PrintMessages(t.Logf)
			if results.AllDone() {
				return nil
			}
		}
	}
}

// WaitForFinality blocks until a beacon client reaches finality,
// or timeoutSlots have passed, whichever happens first.
func (t *Testnet) WaitForFinality(ctx context.Context) (
	common.Checkpoint, error,
) {
	var (
		genesis      = t.GenesisTimeUnix()
		slotDuration = time.Duration(t.spec.SECONDS_PER_SLOT) * time.Second
		timer        = time.NewTicker(slotDuration)
		runningNodes = t.VerificationNodes().Running()
		results      = makeResults(runningNodes, t.maxConsecutiveErrorsOnWaits)
	)

	for {
		select {
		case <-ctx.Done():
			return common.Checkpoint{}, ctx.Err()
		case tim := <-timer.C:
			// start polling after first slot of genesis
			if tim.Before(genesis.Add(slotDuration)) {
				t.Logf("Time till genesis: %s", genesis.Sub(tim))
				continue
			}

			// new slot, log and check status of all beacon nodes
			var (
				wg        sync.WaitGroup
				clockSlot = t.spec.TimeToSlot(
					common.Timestamp(time.Now().Unix()),
					t.GenesisTime(),
				)
			)
			results.Clear()

			for i, n := range runningNodes {
				wg.Add(1)
				go func(ctx context.Context, n *clients.Node, r *result) {
					defer wg.Done()

					b := n.BeaconClient

					headInfo, err := b.BlockHeader(ctx, eth2api.BlockHead)
					if err != nil {
						r.err = errors.Wrap(err, "failed to poll head")
						return
					}

					checkpoints, err := b.BlockFinalityCheckpoints(
						ctx,
						eth2api.BlockHead,
					)
					if err != nil {
						r.err = errors.Wrap(
							err,
							"failed to poll finality checkpoint",
						)
						return
					}

					versionedBlock, err := b.BlockV2(
						ctx,
						eth2api.BlockIdRoot(headInfo.Root),
					)
					if err != nil {
						r.err = errors.Wrap(err, "failed to retrieve block")
						return
					}
					execution := ethcommon.Hash{}
					if executionPayload, err := versionedBlock.ExecutionPayload(); err == nil {
						execution = executionPayload.BlockHash
					}

					slot := headInfo.Header.Message.Slot
					if clockSlot > slot &&
						(clockSlot-slot) >= t.spec.SLOTS_PER_EPOCH {
						r.fatal = fmt.Errorf(
							"unable to sync for an entire epoch: clockSlot=%d, slot=%d",
							clockSlot,
							slot,
						)
						return
					}

					health, _ := GetHealth(ctx, b, t.spec, slot)

					r.msg = fmt.Sprintf(
						"fork=%s, clock_slot=%d, slot=%d, head=%s, "+
							"health=%.2f, exec_payload=%s, justified=%s, "+
							"finalized=%s",
						versionedBlock.Version,
						clockSlot,
						slot,
						utils.Shorten(headInfo.Root.String()),
						health,
						utils.Shorten(execution.String()),
						utils.Shorten(checkpoints.CurrentJustified.String()),
						utils.Shorten(checkpoints.Finalized.String()),
					)

					if (checkpoints.Finalized != common.Checkpoint{}) {
						r.done = true
						r.result = checkpoints.Finalized
					}
				}(ctx, n, results[i])
			}
			wg.Wait()

			if err := results.CheckError(); err != nil {
				return common.Checkpoint{}, err
			}
			results.PrintMessages(t.Logf)
			if results.AllDone() {
				if cp, ok := results[0].result.(common.Checkpoint); ok {
					return cp, nil
				}
			}
		}
	}
}

// WaitForExecutionFinality blocks until a beacon client reaches finality
// and the finality checkpoint contains an execution payload,
// or timeoutSlots have passed, whichever happens first.
func (t *Testnet) WaitForExecutionFinality(
	ctx context.Context,
) (common.Checkpoint, error) {
	var (
		genesis      = t.GenesisTimeUnix()
		slotDuration = time.Duration(t.spec.SECONDS_PER_SLOT) * time.Second
		timer        = time.NewTicker(slotDuration)
		runningNodes = t.VerificationNodes().Running()
		results      = makeResults(runningNodes, t.maxConsecutiveErrorsOnWaits)
	)

	for {
		select {
		case <-ctx.Done():
			return common.Checkpoint{}, ctx.Err()
		case tim := <-timer.C:
			// start polling after first slot of genesis
			if tim.Before(genesis.Add(slotDuration)) {
				t.Logf("Time till genesis: %s", genesis.Sub(tim))
				continue
			}

			// new slot, log and check status of all beacon nodes
			var (
				wg        sync.WaitGroup
				clockSlot = t.spec.TimeToSlot(
					common.Timestamp(time.Now().Unix()),
					t.GenesisTime(),
				)
			)
			results.Clear()

			for i, n := range runningNodes {
				wg.Add(1)
				go func(ctx context.Context, n *clients.Node, r *result) {
					defer wg.Done()
					var (
						b       = n.BeaconClient
						version string
					)

					headInfo, err := b.BlockHeader(ctx, eth2api.BlockHead)
					if err != nil {
						r.err = errors.Wrap(err, "failed to poll head")
						return
					}
					slot := headInfo.Header.Message.Slot
					if clockSlot > slot &&
						(clockSlot-slot) >= t.spec.SLOTS_PER_EPOCH {
						r.fatal = fmt.Errorf(
							"unable to sync for an entire epoch: clockSlot=%d, slot=%d",
							clockSlot,
							slot,
						)
						return
					}

					checkpoints, err := b.BlockFinalityCheckpoints(
						ctx,
						eth2api.BlockHead,
					)
					if err != nil {
						r.err = errors.Wrap(
							err,
							"failed to poll finality checkpoint",
						)
						return
					}

					execution := ethcommon.Hash{}
					if (checkpoints.Finalized != common.Checkpoint{}) {
						if versionedBlock, err := b.BlockV2(
							ctx,
							eth2api.BlockIdRoot(checkpoints.Finalized.Root),
						); err != nil {
							r.err = errors.Wrap(
								err,
								"failed to retrieve block",
							)
							return
						} else {
							version = versionedBlock.Version
							if exeuctionPayload, err := versionedBlock.ExecutionPayload(); err == nil {
								execution = exeuctionPayload.BlockHash
							}
						}
					}

					r.msg = fmt.Sprintf(
						"fork=%s, clock_slot=%s, slot=%d, head=%s, "+
							"exec_payload=%s, justified=%s, finalized=%s",
						version,
						clockSlot,
						slot,
						utils.Shorten(headInfo.Root.String()),
						utils.Shorten(execution.Hex()),
						utils.Shorten(checkpoints.CurrentJustified.String()),
						utils.Shorten(checkpoints.Finalized.String()),
					)

					if !bytes.Equal(execution[:], EMPTY_EXEC_HASH[:]) {
						r.done = true
						r.result = checkpoints.Finalized
					}
				}(
					ctx,
					n,
					results[i],
				)
			}
			wg.Wait()

			if err := results.CheckError(); err != nil {
				return common.Checkpoint{}, err
			}
			results.PrintMessages(t.Logf)
			if results.AllDone() {
				if cp, ok := results[0].result.(common.Checkpoint); ok {
					return cp, nil
				}
			}
		}
	}
}

// Waits for the current epoch to be finalized, or timeoutSlots have passed, whichever happens first.
func (t *Testnet) WaitForCurrentEpochFinalization(
	ctx context.Context,
) (common.Checkpoint, error) {
	var (
		genesis      = t.GenesisTimeUnix()
		slotDuration = time.Duration(
			t.spec.SECONDS_PER_SLOT,
		) * time.Second
		timer        = time.NewTicker(slotDuration)
		runningNodes = t.VerificationNodes().Running()
		results      = makeResults(
			runningNodes,
			t.maxConsecutiveErrorsOnWaits,
		)
		epochToBeFinalized = t.spec.SlotToEpoch(t.spec.TimeToSlot(
			common.Timestamp(time.Now().Unix()),
			t.GenesisTime(),
		))
	)

	for {
		select {
		case <-ctx.Done():
			return common.Checkpoint{}, ctx.Err()
		case tim := <-timer.C:
			// start polling after first slot of genesis
			if tim.Before(genesis.Add(slotDuration)) {
				t.Logf("Time till genesis: %s", genesis.Sub(tim))
				continue
			}

			// new slot, log and check status of all beacon nodes
			var (
				wg        sync.WaitGroup
				clockSlot = t.spec.TimeToSlot(
					common.Timestamp(time.Now().Unix()),
					t.GenesisTime(),
				)
			)
			results.Clear()

			for i, n := range runningNodes {
				i := i
				wg.Add(1)
				go func(ctx context.Context, n *clients.Node, r *result) {
					defer wg.Done()

					b := n.BeaconClient

					headInfo, err := b.BlockHeader(ctx, eth2api.BlockHead)
					if err != nil {
						r.err = errors.Wrap(err, "failed to poll head")
						return
					}

					slot := headInfo.Header.Message.Slot
					if clockSlot > slot &&
						(clockSlot-slot) >= t.spec.SLOTS_PER_EPOCH {
						r.fatal = fmt.Errorf(
							"unable to sync for an entire epoch: clockSlot=%d, slot=%d",
							clockSlot,
							slot,
						)
						return
					}

					checkpoints, err := b.BlockFinalityCheckpoints(
						ctx,
						eth2api.BlockHead,
					)
					if err != nil {
						r.err = errors.Wrap(
							err,
							"failed to poll finality checkpoint",
						)
						return
					}

					r.msg = fmt.Sprintf(
						"clock_slot=%d, slot=%d, head=%s justified=%s, "+
							"finalized=%s, epoch_to_finalize=%d",
						clockSlot,
						slot,
						utils.Shorten(headInfo.Root.String()),
						utils.Shorten(checkpoints.CurrentJustified.String()),
						utils.Shorten(checkpoints.Finalized.String()),
						epochToBeFinalized,
					)

					if checkpoints.Finalized != (common.Checkpoint{}) &&
						checkpoints.Finalized.Epoch >= epochToBeFinalized {
						r.done = true
						r.result = checkpoints.Finalized
					}
				}(ctx, n, results[i])

			}
			wg.Wait()

			if err := results.CheckError(); err != nil {
				return common.Checkpoint{}, err
			}
			results.PrintMessages(t.Logf)
			if results.AllDone() {
				t.Logf("INFO: Epoch %d finalized", epochToBeFinalized)
				if cp, ok := results[0].result.(common.Checkpoint); ok {
					return cp, nil
				}
			}
		}
	}
}

// Waits for any execution payload to be available included in a beacon block (merge),
// or timeoutSlots have passed, whichever happens first.
func (t *Testnet) WaitForExecutionPayload(
	ctx context.Context,
) (ethcommon.Hash, error) {
	var (
		genesis      = t.GenesisTimeUnix()
		slotDuration = time.Duration(t.spec.SECONDS_PER_SLOT) * time.Second
		timer        = time.NewTicker(slotDuration)
		runningNodes = t.VerificationNodes().Running()
		results      = makeResults(
			runningNodes,
			t.maxConsecutiveErrorsOnWaits,
		)
		executionClient = runningNodes[0].ExecutionClient
		ttdReached      = false
	)

	for {
		select {
		case <-ctx.Done():
			return ethcommon.Hash{}, ctx.Err()
		case tim := <-timer.C:
			// start polling after first slot of genesis
			if tim.Before(genesis.Add(slotDuration)) {
				t.Logf("Time till genesis: %s", genesis.Sub(tim))
				continue
			}

			if !ttdReached {
				// Check if TTD has been reached
				if td, err := executionClient.TotalDifficultyByNumber(ctx, nil); err == nil {
					if td.Cmp(
						t.eth1Genesis.Genesis.Config.TerminalTotalDifficulty,
					) >= 0 {
						ttdReached = true
					} else {
						continue
					}
				} else {
					t.Logf("Error querying eth1 for TTD: %v", err)
				}
			}

			// new slot, log and check status of all beacon nodes
			var (
				wg        sync.WaitGroup
				clockSlot = t.spec.TimeToSlot(
					common.Timestamp(time.Now().Unix()),
					t.GenesisTime(),
				)
			)
			results.Clear()

			for i, n := range runningNodes {
				wg.Add(1)
				go func(ctx context.Context, n *clients.Node, r *result) {
					defer wg.Done()

					b := n.BeaconClient

					headInfo, err := b.BlockHeader(ctx, eth2api.BlockHead)
					if err != nil {
						r.err = errors.Wrap(err, "failed to poll head")
						return
					}

					slot := headInfo.Header.Message.Slot
					if clockSlot > slot &&
						(clockSlot-slot) >= t.spec.SLOTS_PER_EPOCH {
						r.fatal = fmt.Errorf(
							"unable to sync for an entire epoch: clockSlot=%d, slot=%d",
							clockSlot,
							slot,
						)
						return
					}

					versionedBlock, err := b.BlockV2(
						ctx,
						eth2api.BlockIdRoot(headInfo.Root),
					)
					if err != nil {
						r.err = errors.Wrap(err, "failed to retrieve block")
						return
					}

					executionHash := ethcommon.Hash{}
					if executionPayload, err := versionedBlock.ExecutionPayload(); err == nil {
						executionHash = executionPayload.BlockHash
					}

					health, _ := GetHealth(ctx, b, t.spec, slot)

					r.msg = fmt.Sprintf(
						"fork=%s, clock_slot=%d, slot=%d, "+
							"head=%s, health=%.2f, exec_payload=%s",
						versionedBlock.Version,
						clockSlot,
						slot,
						utils.Shorten(headInfo.Root.String()),
						health,
						utils.Shorten(executionHash.Hex()),
					)

					if !bytes.Equal(executionHash[:], EMPTY_EXEC_HASH[:]) {
						r.done = true
						r.result = executionHash
					}
				}(ctx, n, results[i])
			}
			wg.Wait()

			if err := results.CheckError(); err != nil {
				return ethcommon.Hash{}, err
			}
			results.PrintMessages(t.Logf)
			if results.AllDone() {
				if h, ok := results[0].result.(ethcommon.Hash); ok {
					return h, nil
				}
			}

		}
	}
}

func GetHealth(
	parentCtx context.Context,
	bn *clients.BeaconClient,
	spec *common.Spec,
	slot common.Slot,
) (float64, error) {
	var health float64
	stateInfo, err := bn.BeaconStateV2(parentCtx, eth2api.StateIdSlot(slot))
	if err != nil {
		return 0, fmt.Errorf("failed to retrieve state: %v", err)
	}
	currentEpochParticipation := stateInfo.CurrentEpochParticipation()
	if currentEpochParticipation != nil {
		// Altair and after
		health = calcHealth(currentEpochParticipation)
	} else {
		if stateInfo.Version != "phase0" {
			return 0, fmt.Errorf("calculate participation")
		}
		state := stateInfo.Data.(*phase0.BeaconState)
		epoch := spec.SlotToEpoch(slot)
		validatorIds := make([]eth2api.ValidatorId, 0, len(state.Validators))
		for id, validator := range state.Validators {
			if epoch >= validator.ActivationEligibilityEpoch &&
				epoch < validator.ExitEpoch &&
				!validator.Slashed {
				validatorIds = append(
					validatorIds,
					eth2api.ValidatorIdIndex(id),
				)
			}
		}
		var (
			beforeEpoch = 0
			afterEpoch  = spec.SlotToEpoch(slot)
		)

		// If it's genesis, keep before also set to 0.
		if afterEpoch != 0 {
			beforeEpoch = int(spec.SlotToEpoch(slot)) - 1
		}
		balancesBefore, err := bn.StateValidatorBalances(
			parentCtx,
			eth2api.StateIdSlot(beforeEpoch*int(spec.SLOTS_PER_EPOCH)),
			validatorIds,
		)
		if err != nil {
			return 0, fmt.Errorf(
				"failed to retrieve validator balances: %v",
				err,
			)
		}
		balancesAfter, err := bn.StateValidatorBalances(
			parentCtx,
			eth2api.StateIdSlot(int(afterEpoch)*int(spec.SLOTS_PER_EPOCH)),
			validatorIds,
		)
		if err != nil {
			return 0, fmt.Errorf(
				"failed to retrieve validator balances: %v",
				err,
			)
		}
		health = legacyCalcHealth(spec, balancesBefore, balancesAfter)
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
func legacyCalcHealth(
	spec *common.Spec,
	before, after []eth2api.ValidatorBalanceResponse,
) float64 {
	sum_before := big.NewInt(0)
	sum_after := big.NewInt(0)
	for i := range before {
		sum_before.Add(sum_before, big.NewInt(int64(before[i].Balance)))
		sum_after.Add(sum_after, big.NewInt(int64(after[i].Balance)))
	}
	count := big.NewInt(int64(len(before)))
	avg_before := big.NewInt(0).Div(sum_before, count).Uint64()
	avg_after := sum_after.Div(sum_after, count).Uint64()
	reward := avg_before * uint64(
		spec.BASE_REWARD_FACTOR,
	) / math.IntegerSquareRootPrysm(
		sum_before.Uint64(),
	) / uint64(
		spec.HYSTERESIS_QUOTIENT,
	)
	return float64(
		avg_after-avg_before,
	) / float64(
		reward*common.BASE_REWARDS_PER_EPOCH,
	)
}
