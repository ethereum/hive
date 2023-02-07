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

	"github.com/protolambda/eth2api"
	"github.com/protolambda/zrnt/eth2/beacon/altair"
	"github.com/protolambda/zrnt/eth2/beacon/common"
	"github.com/protolambda/zrnt/eth2/beacon/phase0"
	"github.com/protolambda/zrnt/eth2/util/math"

	"github.com/ethereum/hive/hivesim"
	"github.com/ethereum/hive/simulators/eth2/common/clients"
	execution_config "github.com/ethereum/hive/simulators/eth2/common/config/execution"
	"github.com/ethereum/hive/simulators/eth2/common/utils"
)

var MAX_PARTICIPATION_SCORE = 7

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

	return testnet
}

func (t *Testnet) Stop() {
	for _, p := range t.Proxies().Running() {
		p.Cancel()
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
	genesis := t.GenesisTimeUnix()
	slotDuration := time.Duration(t.spec.SECONDS_PER_SLOT) * time.Second
	timer := time.NewTicker(slotDuration)
	runningNodes := t.VerificationNodes().Running()
	done := make(chan error, len(runningNodes))
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case err := <-done:
			return err
		case tim := <-timer.C:
			// start polling after first slot of genesis
			if tim.Before(genesis.Add(slotDuration)) {
				t.Logf("Time till genesis: %s", genesis.Sub(tim))
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
				ch = make(chan res, len(runningNodes))
			)
			for i, n := range runningNodes {
				wg.Add(1)
				go func(
					ctx context.Context,
					i int,
					n *clients.Node,
					ch chan res,
				) {
					defer wg.Done()

					var (
						b         = n.BeaconClient
						slot      common.Slot
						head      string
						justified string
						finalized string
						execution = "0x0000..0000"
					)

					headInfo, err := b.BlockHeader(ctx, eth2api.BlockHead)
					if err != nil {
						ch <- res{
							err: fmt.Errorf(
								"node %d (%s): failed to poll head: %v",
								i,
								n.ClientNames(),
								err,
							),
						}
						return
					}

					checkpoints, err := b.BlockFinalityCheckpoints(
						ctx,
						eth2api.BlockHead,
					)
					if err != nil {
						ch <- res{err: fmt.Errorf("node %d (%s) failed to poll finality checkpoint: %v", i, n.ClientNames(), err)}
						return
					}

					versionedBlock, err := b.BlockV2(
						ctx,
						eth2api.BlockIdRoot(headInfo.Root),
					)
					if err != nil {
						ch <- res{
							err: fmt.Errorf(
								"node %d (%s) failed to retrieve block: %v",
								i,
								n.ClientNames(),
								err,
							),
						}
						return
					}
					if executionPayload, err := versionedBlock.ExecutionPayload(); err == nil {
						execution = utils.Shorten(
							executionPayload.BlockHash.String(),
						)
					}

					slot = headInfo.Header.Message.Slot
					head = utils.Shorten(headInfo.Root.String())
					justified = utils.Shorten(
						checkpoints.CurrentJustified.String(),
					)
					finalized = utils.Shorten(checkpoints.Finalized.String())

					ch <- res{
						i,
						fmt.Sprintf(
							"node %d (%s) fork=%s, slot=%d, head=%s, exec_payload=%s, justified=%s, finalized=%s",
							i,
							n.ClientNames(),
							versionedBlock.Version,
							slot,
							head,
							execution,
							justified,
							finalized,
						),
						nil,
					}

					if versionedBlock.Version == fork {
						done <- nil
					}
				}(ctx, i, n, ch)
			}
			wg.Wait()
			close(ch)

			// print out logs in ascending idx order
			sorted := make([]string, len(runningNodes))
			for out := range ch {
				if out.err != nil {
					return out.err
				}
				sorted[out.idx] = out.msg
			}
			for _, msg := range sorted {
				t.Logf(msg)
			}
		}
	}
}

// WaitForFinality blocks until a beacon client reaches finality,
// or timeoutSlots have passed, whichever happens first.
func (t *Testnet) WaitForFinality(ctx context.Context) (
	common.Checkpoint, error,
) {
	genesis := t.GenesisTimeUnix()
	slotDuration := time.Duration(t.spec.SECONDS_PER_SLOT) * time.Second
	timer := time.NewTicker(slotDuration)
	runningNodes := t.VerificationNodes().Running()
	done := make(chan common.Checkpoint, len(runningNodes))
	for {
		select {
		case <-ctx.Done():
			return common.Checkpoint{}, ctx.Err()
		case finalized := <-done:
			return finalized, nil
		case tim := <-timer.C:
			// start polling after first slot of genesis
			if tim.Before(genesis.Add(slotDuration)) {
				t.Logf("Time till genesis: %s", genesis.Sub(tim))
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
				ch = make(chan res, len(runningNodes))
			)
			for i, n := range runningNodes {
				wg.Add(1)
				go func(
					ctx context.Context,
					i int,
					n *clients.Node,
					ch chan res,
				) {
					defer wg.Done()

					var (
						b         = n.BeaconClient
						slot      common.Slot
						head      string
						justified string
						finalized string
						health    float64
						execution = "0x0000..0000"
					)

					headInfo, err := b.BlockHeader(ctx, eth2api.BlockHead)
					if err != nil {
						ch <- res{
							err: fmt.Errorf(
								"node %d (%s) failed to poll head: %v",
								i,
								n.ClientNames(),
								err,
							),
						}
						return
					}

					checkpoints, err := b.BlockFinalityCheckpoints(
						ctx,
						eth2api.BlockHead,
					)
					if err != nil {
						ch <- res{
							err: fmt.Errorf(
								"node %d (%s) failed to poll finality checkpoint: %v",
								i,
								n.ClientNames(),
								err,
							),
						}
						return
					}

					versionedBlock, err := b.BlockV2(
						ctx,
						eth2api.BlockIdRoot(headInfo.Root),
					)
					if err != nil {
						ch <- res{
							err: fmt.Errorf(
								"node %d (%s) failed to retrieve block: %v",
								i,
								n.ClientNames(),
								err,
							),
						}
						return
					}
					if executionPayload, err := versionedBlock.ExecutionPayload(); err == nil {
						execution = utils.Shorten(
							executionPayload.BlockHash.String(),
						)
					}

					slot = headInfo.Header.Message.Slot
					head = utils.Shorten(headInfo.Root.String())
					justified = utils.Shorten(
						checkpoints.CurrentJustified.String(),
					)
					finalized = utils.Shorten(checkpoints.Finalized.String())
					health, err = GetHealth(ctx, b, t.spec, slot)
					if err != nil {
						// warning is printed here instead because some clients
						// don't support the required REST endpoint.
						fmt.Printf(
							"WARN: node %d (%s) %s\n",
							i,
							n.ClientNames(),
							err,
						)
					}

					ep := t.spec.SlotToEpoch(slot)
					if ep > 4 && ep > checkpoints.Finalized.Epoch+2 {
						ch <- res{
							err: fmt.Errorf(
								"failed to finalize, head slot %d (epoch %d) "+
									"is more than 2 ahead of finality checkpoint %d",
								slot,
								ep,
								checkpoints.Finalized.Epoch,
							),
						}
					} else {
						ch <- res{
							i,
							fmt.Sprintf(
								"node %d (%s) fork=%s, slot=%d, head=%s, "+
									"health=%.2f, exec_payload=%s, justified=%s, "+
									"finalized=%s",
								i,
								n.ClientNames(),
								versionedBlock.Version,
								slot,
								head,
								health,
								execution,
								justified,
								finalized,
							),
							nil,
						}
					}

					if (checkpoints.Finalized != common.Checkpoint{}) {
						done <- checkpoints.Finalized
					}
				}(ctx, i, n, ch)
			}
			wg.Wait()
			close(ch)

			// print out logs in ascending idx order
			sorted := make([]string, len(runningNodes))
			for out := range ch {
				if out.err != nil {
					return common.Checkpoint{}, out.err
				}
				sorted[out.idx] = out.msg
			}
			for _, msg := range sorted {
				t.Logf(msg)
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
	genesis := t.GenesisTimeUnix()
	slotDuration := time.Duration(t.spec.SECONDS_PER_SLOT) * time.Second
	timer := time.NewTicker(slotDuration)
	runningNodes := t.VerificationNodes().Running()
	done := make(chan common.Checkpoint, len(runningNodes))
	for {
		select {
		case <-ctx.Done():
			return common.Checkpoint{}, ctx.Err()
		case finalized := <-done:
			return finalized, nil
		case tim := <-timer.C:
			// start polling after first slot of genesis
			if tim.Before(genesis.Add(slotDuration)) {
				t.Logf("Time till genesis: %s", genesis.Sub(tim))
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
				ch = make(chan res, len(runningNodes))
			)
			for i, n := range runningNodes {
				wg.Add(1)
				go func(ctx context.Context, i int, n *clients.Node, ch chan res) {
					defer wg.Done()
					var (
						b         = n.BeaconClient
						slot      common.Slot
						version   string
						head      string
						justified string
						finalized string
					)

					headInfo, err := b.BlockHeader(ctx, eth2api.BlockHead)
					if err != nil {
						ch <- res{
							err: fmt.Errorf(
								"node %d (%s) failed to poll head: %v",
								i,
								n.ClientNames(),
								err,
							),
						}
						return
					}
					slot = headInfo.Header.Message.Slot
					head = utils.Shorten(headInfo.Root.String())

					checkpoints, err := b.BlockFinalityCheckpoints(
						ctx,
						eth2api.BlockHead,
					)
					if err != nil {
						ch <- res{
							err: fmt.Errorf(
								"node %d (%s) failed to poll finality checkpoint: %v",
								i,
								n.ClientNames(),
								err,
							),
						}
						return
					}
					justified = utils.Shorten(
						checkpoints.CurrentJustified.String(),
					)
					finalized = utils.Shorten(checkpoints.Finalized.String())

					var (
						execution    ethcommon.Hash
						executionStr = "0x0000..0000"
					)

					if (checkpoints.Finalized != common.Checkpoint{}) {
						if versionedBlock, err := b.BlockV2(
							ctx,
							eth2api.BlockIdRoot(checkpoints.Finalized.Root),
						); err != nil {
							ch <- res{
								err: fmt.Errorf(
									"node %d (%s) failed to retrieve block: %v",
									i,
									n.ClientNames(),
									err,
								),
							}
							return
						} else {
							version = versionedBlock.Version
							if exeuctionPayload, err := versionedBlock.ExecutionPayload(); err == nil {
								execution = exeuctionPayload.BlockHash
								executionStr = utils.Shorten(execution.Hex())
							}
						}
					}

					ch <- res{
						i,
						fmt.Sprintf("node %d (%s) fork=%s, slot=%d, head=%s, "+"finalized_exec_payload=%s, justified=%s, finalized=%s",
							i,
							n.ClientNames(),
							version,
							slot,
							head,
							executionStr,
							justified,
							finalized,
						),
						nil,
					}
					emptyHash := ethcommon.Hash{}
					if !bytes.Equal(execution[:], emptyHash[:]) {
						done <- checkpoints.Finalized
					}
				}(
					ctx,
					i,
					n,
					ch,
				)
			}
			wg.Wait()
			close(ch)

			// print out logs in ascending idx order
			sorted := make([]string, len(runningNodes))
			for out := range ch {
				if out.err != nil {
					return common.Checkpoint{}, out.err
				}
				sorted[out.idx] = out.msg
			}
			for _, msg := range sorted {
				t.Logf(msg)
			}
		}
	}
}

// Waits for the current epoch to be finalized, or timeoutSlots have passed, whichever happens first.
func (t *Testnet) WaitForCurrentEpochFinalization(
	ctx context.Context,
) (common.Checkpoint, error) {
	genesis := t.GenesisTimeUnix()
	slotDuration := time.Duration(t.spec.SECONDS_PER_SLOT) * time.Second
	timer := time.NewTicker(slotDuration)
	runningNodes := t.VerificationNodes().Running()
	done := make(chan common.Checkpoint, len(runningNodes))

	// Get the current head root which must be finalized
	headInfo, err := runningNodes[0].BeaconClient.BlockHeader(
		ctx,
		eth2api.BlockHead,
	)
	if err != nil {
		return common.Checkpoint{}, fmt.Errorf("failed to poll head: %v", err)
	}
	epochToBeFinalized := t.spec.SlotToEpoch(headInfo.Header.Message.Slot)

	for {
		select {
		case <-ctx.Done():
			return common.Checkpoint{}, ctx.Err()
		case finalized := <-done:
			return finalized, nil
		case tim := <-timer.C:
			// start polling after first slot of genesis
			if tim.Before(genesis.Add(slotDuration)) {
				t.Logf("Time till genesis: %s", genesis.Sub(tim))
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
				ch = make(chan res, len(runningNodes))
			)
			for i, n := range runningNodes {
				wg.Add(1)
				go func(ctx context.Context, i int, n *clients.Node, ch chan res) {
					defer wg.Done()

					var (
						b         = n.BeaconClient
						slot      common.Slot
						head      string
						justified string
						finalized string
					)

					headInfo, err := b.BlockHeader(ctx, eth2api.BlockHead)
					if err != nil {
						ch <- res{
							err: fmt.Errorf(
								"node %d (%s) failed to poll head: %v",
								i,
								n.ClientNames(),
								err,
							),
						}
						return
					}
					slot = headInfo.Header.Message.Slot
					head = utils.Shorten(headInfo.Root.String())

					checkpoints, err := b.BlockFinalityCheckpoints(
						ctx,
						eth2api.BlockHead,
					)
					if err != nil {
						ch <- res{
							err: fmt.Errorf(
								"node %d (%s) failed to poll finality checkpoint: %v",
								i,
								n.ClientNames(),
								err,
							),
						}
						return
					}
					justified = utils.Shorten(
						checkpoints.CurrentJustified.String(),
					)
					finalized = utils.Shorten(checkpoints.Finalized.String())

					ch <- res{
						i,
						fmt.Sprintf(
							"node %d (%s) slot=%d, head=%s justified=%s, "+
								"finalized=%s, epoch_to_finalize=%d",
							i,
							n.ClientNames(),
							slot,
							head,
							justified,
							finalized,
							epochToBeFinalized,
						),
						nil,
					}

					if checkpoints.Finalized != (common.Checkpoint{}) &&
						checkpoints.Finalized.Epoch >= epochToBeFinalized {
						done <- checkpoints.Finalized
					}
				}(
					ctx,
					i,
					n,
					ch,
				)
			}
			wg.Wait()
			close(ch)

			// print out logs in ascending idx order
			sorted := make([]string, len(runningNodes))
			for out := range ch {
				if out.err != nil {
					return common.Checkpoint{}, out.err
				}
				sorted[out.idx] = out.msg
			}
			for _, msg := range sorted {
				t.Logf(msg)
			}
		}
	}
}

// Waits for any execution payload to be available included in a beacon block (merge),
// or timeoutSlots have passed, whichever happens first.
func (t *Testnet) WaitForExecutionPayload(
	ctx context.Context,
) (ethcommon.Hash, error) {
	genesis := t.GenesisTimeUnix()
	slotDuration := time.Duration(t.spec.SECONDS_PER_SLOT) * time.Second
	timer := time.NewTicker(slotDuration)
	runningNodes := t.VerificationNodes().Running()
	done := make(chan ethcommon.Hash, len(runningNodes))
	executionClient := runningNodes[0].ExecutionClient
	ttdReached := false

	for {
		select {
		case <-ctx.Done():
			return ethcommon.Hash{}, ctx.Err()
		case result := <-done:
			return result, nil
		case tim := <-timer.C:
			// start polling after first slot of genesis
			if tim.Before(genesis.Add(slotDuration)) {
				t.Logf("Time till genesis: %s", genesis.Sub(tim))
				continue
			}

			if !ttdReached {
				// Check if TTD has been reached
				if td, err := executionClient.TotalDifficultyByNumber(ctx, nil); err == nil &&
					td.Cmp(t.eth1Genesis.Genesis.Config.TerminalTotalDifficulty) >= 0 {
					ttdReached = true
				} else {
					t.Logf("Error querying eth1 for TTD: %v", err)
				}
			}

			// new slot, log and check status of all beacon nodes
			type res struct {
				idx int
				msg string
				err error
			}

			var (
				wg sync.WaitGroup
				ch = make(chan res, len(runningNodes))
			)
			for i, n := range runningNodes {
				wg.Add(1)
				go func(ctx context.Context, i int, n *clients.Node, ch chan res) {
					defer wg.Done()

					var (
						b       = n.BeaconClient
						slot    common.Slot
						version string
						head    string
						health  float64
					)

					headInfo, err := b.BlockHeader(ctx, eth2api.BlockHead)
					if err != nil {
						ch <- res{
							err: fmt.Errorf(
								"node %d (%s): failed to poll head: %v",
								i,
								n.ClientNames(),
								err,
							),
						}
						return
					}
					slot = headInfo.Header.Message.Slot
					head = utils.Shorten(headInfo.Root.String())

					if versionedBlock, err := b.BlockV2(
						ctx,
						eth2api.BlockIdRoot(headInfo.Root),
					); err != nil {
						ch <- res{
							err: fmt.Errorf(
								"node %d (%s): failed to retrieve block: %v",
								i,
								n.ClientNames(),
								err,
							),
						}
						return
					} else {
						version = versionedBlock.Version
						if executionPayload, err := versionedBlock.ExecutionPayload(); err == nil {
							emptyHash := ethcommon.Hash{}
							if !bytes.Equal(executionPayload.BlockHash[:], emptyHash[:]) {
								ch <- res{
									i,
									fmt.Sprintf(
										"node %d (%s): fork=%s, slot=%d, "+
											"head=%s, health=%.2f, exec_payload=%s",
										i,
										n.ClientNames(),
										version,
										slot,
										head,
										health,
										utils.Shorten(executionPayload.BlockHash.Hex()),
									),
									nil,
								}
								done <- executionPayload.BlockHash
							}
						}
					}

					health, err = GetHealth(ctx, b, t.spec, slot)
					if err != nil {
						// warning is printed here instead because some clients
						// don't support the required REST endpoint.
						fmt.Printf(
							"WARN: node %d (%s): %s\n",
							i,
							n.ClientNames(),
							err,
						)
					}

					ch <- res{
						i,
						fmt.Sprintf(
							"node %d (%s): fork=%s, slot=%d, head=%s, "+
								"health=%.2f, exec_payload=0x000..000",
							i,
							n.ClientNames(),
							version,
							slot,
							head,
							health,
						),
						nil,
					}
				}(
					ctx,
					i,
					n,
					ch,
				)
			}
			wg.Wait()
			close(ch)

			// print out logs in ascending idx order
			sorted := make([]string, len(runningNodes))
			for out := range ch {
				if out.err != nil {
					return ethcommon.Hash{}, out.err
				}
				sorted[out.idx] = out.msg
			}
			for _, msg := range sorted {
				t.Logf(msg)
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
