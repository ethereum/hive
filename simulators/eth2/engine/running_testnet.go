package main

import (
	"bytes"
	"context"
	"fmt"
	"math/big"
	"net/http"
	"sync"
	"time"

	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/ethereum/hive/hivesim"
	"github.com/ethereum/hive/simulators/eth2/engine/setup"
	"github.com/protolambda/eth2api"
	"github.com/protolambda/eth2api/client/beaconapi"
	"github.com/protolambda/eth2api/client/debugapi"
	"github.com/protolambda/zrnt/eth2/beacon/altair"
	"github.com/protolambda/zrnt/eth2/beacon/bellatrix"
	"github.com/protolambda/zrnt/eth2/beacon/common"
	"github.com/protolambda/zrnt/eth2/beacon/phase0"
	"github.com/protolambda/zrnt/eth2/util/math"
	"github.com/protolambda/ztyp/tree"
)

var MAX_PARTICIPATION_SCORE = 7

type Testnet struct {
	*hivesim.T
	NodeClientBundles

	genesisTime           common.Timestamp
	genesisValidatorsRoot common.Root

	// Consensus chain configuration
	spec *common.Spec
	// Execution chain configuration and genesis info
	eth1Genesis *setup.Eth1Genesis
}

func startTestnet(t *hivesim.T, env *testEnv, config *Config) *Testnet {
	prep := prepareTestnet(t, env, config)
	testnet := prep.createTestnet(t)
	genesisTime := testnet.GenesisTime()
	countdown := genesisTime.Sub(time.Now())
	t.Logf("Created new testnet, genesis at %s (%s from now)", genesisTime, countdown)

	testnet.NodeClientBundles = make(NodeClientBundles, len(config.Nodes))

	// For each key partition, we start a client bundle that consists of:
	// - 1 execution client
	// - 1 beacon client
	// - 1 validator client,
	for nodeIndex, node := range config.Nodes {
		// Prepare clients for this node
		var (
			ec *ExecutionClient
			bc *BeaconClient
			vc *ValidatorClient

			executionDef = env.Clients.ClientByNameAndRole(node.ExecutionClient, "eth1")
			beaconDef    = env.Clients.ClientByNameAndRole(fmt.Sprintf("%s-bn", node.ConsensusClient), "beacon")
			validatorDef = env.Clients.ClientByNameAndRole(fmt.Sprintf("%s-vc", node.ConsensusClient), "validator")
		)

		if executionDef == nil || beaconDef == nil || validatorDef == nil {
			t.Fatalf("FAIL: Unable to get client")
		}

		// Prepare the client objects with all the information necessary to eventually start
		ec = prep.prepareExecutionNode(testnet, executionDef, config.Eth1Consensus, node.ExecutionClientTTD, nodeIndex, node.Chain)
		if node.ConsensusClient != "" {
			bc = prep.prepareBeaconNode(testnet, beaconDef, node.BeaconNodeTTD, nodeIndex, ec)
			vc = prep.prepareValidatorClient(testnet, validatorDef, bc, nodeIndex)
		}

		// Bundle all the clients together
		testnet.NodeClientBundles[nodeIndex] = NodeClientBundle{
			T:               t,
			Index:           nodeIndex,
			Verification:    node.TestVerificationNode,
			ExecutionClient: ec,
			BeaconClient:    bc,
			ValidatorClient: vc,
		}

		// Start the node clients if specified so
		if !node.DisableStartup {
			testnet.NodeClientBundles[nodeIndex].Start()
		}
	}

	return testnet
}

func (t *Testnet) stopTestnet() {
	for _, p := range t.Proxies().Running() {
		p.cancel()
	}
}

func (t *Testnet) GenesisTime() time.Time {
	return time.Unix(int64(t.genesisTime), 0)
}

func (t *Testnet) SlotsTimeout(slots common.Slot) <-chan time.Time {
	return time.After(time.Duration(uint64(slots)*uint64(t.spec.SECONDS_PER_SLOT)) * time.Second)
}

func (t *Testnet) ValidatorClientIndex(pk [48]byte) (int, error) {
	for i, v := range t.ValidatorClients() {
		if v.ContainsKey(pk) {
			return i, nil
		}
	}
	return 0, fmt.Errorf("key not found in any validator client")
}

// WaitForFinality blocks until a beacon client reaches finality,
// or timeoutSlots have passed, whichever happens first.
func (t *Testnet) WaitForFinality(ctx context.Context, timeoutSlots common.Slot) (common.Checkpoint, error) {
	genesis := t.GenesisTime()
	slotDuration := time.Duration(t.spec.SECONDS_PER_SLOT) * time.Second
	timer := time.NewTicker(slotDuration)
	runningBeacons := t.VerificationNodes().BeaconClients().Running()
	done := make(chan common.Checkpoint, len(runningBeacons))
	var timeout <-chan time.Time
	if timeoutSlots > 0 {
		timeout = t.SlotsTimeout(timeoutSlots)
	} else {
		timeout = make(<-chan time.Time)
	}
	for {
		select {
		case <-ctx.Done():
			return common.Checkpoint{}, fmt.Errorf("context called")
		case <-timeout:
			return common.Checkpoint{}, fmt.Errorf("Timeout")
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
				ch = make(chan res, len(runningBeacons))
			)
			for i, b := range runningBeacons {
				wg.Add(1)
				go func(ctx context.Context, i int, b *BeaconClient, ch chan res) {
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
					if exists, err := beaconapi.FinalityCheckpoints(ctx, b.API, eth2api.StateIdRoot(headInfo.Header.Message.StateRoot), &checkpoints); err != nil || !exists {
						if exists, err = beaconapi.FinalityCheckpoints(ctx, b.API, eth2api.StateIdSlot(headInfo.Header.Message.Slot), &checkpoints); err != nil {
							ch <- res{err: fmt.Errorf("beacon %d: failed to poll finality checkpoint: %v", i, err)}
							return
						} else if !exists {
							ch <- res{err: fmt.Errorf("beacon %d: Expected state for head block", i)}
							return
						}
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
						// warning is printed here instead because some clients
						// don't support the required REST endpoint.
						fmt.Printf("WARN: beacon %d: %s\n", i, err)
					}

					ep := t.spec.SlotToEpoch(slot)
					if ep > 4 && ep > checkpoints.Finalized.Epoch+2 {
						ch <- res{err: fmt.Errorf("failed to finalize, head slot %d (epoch %d) is more than 2 ahead of finality checkpoint %d", slot, ep, checkpoints.Finalized.Epoch)}
					} else {
						ch <- res{i, fmt.Sprintf("beacon %d: slot=%d, head=%s, health=%.2f, exec_payload=%s, justified=%s, finalized=%s", i, slot, head, health, execution, justified, finalized), nil}
					}

					if (checkpoints.Finalized != common.Checkpoint{}) {
						done <- checkpoints.Finalized
					}
				}(ctx, i, b, ch)
			}
			wg.Wait()
			close(ch)

			// print out logs in ascending idx order
			sorted := make([]string, len(runningBeacons))
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
func (t *Testnet) WaitForExecutionFinality(ctx context.Context, timeoutSlots common.Slot) (common.Checkpoint, error) {
	genesis := t.GenesisTime()
	slotDuration := time.Duration(t.spec.SECONDS_PER_SLOT) * time.Second
	timer := time.NewTicker(slotDuration)
	runningBeacons := t.VerificationNodes().BeaconClients().Running()
	done := make(chan common.Checkpoint, len(runningBeacons))
	var timeout <-chan time.Time
	if timeoutSlots > 0 {
		timeout = t.SlotsTimeout(timeoutSlots)
	} else {
		timeout = make(<-chan time.Time)
	}
	for {
		select {
		case <-ctx.Done():
			return common.Checkpoint{}, fmt.Errorf("context called")
		case <-timeout:
			return common.Checkpoint{}, fmt.Errorf("Timeout")
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
				ch = make(chan res, len(runningBeacons))
			)
			for i, b := range runningBeacons {
				wg.Add(1)
				go func(ctx context.Context, i int, b *BeaconClient, ch chan res) {
					defer wg.Done()
					ctx, cancel := context.WithTimeout(ctx, time.Second*5)
					defer cancel()

					var (
						slot      common.Slot
						head      string
						justified string
						finalized string
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
					if exists, err := beaconapi.FinalityCheckpoints(ctx, b.API, eth2api.StateIdRoot(headInfo.Header.Message.StateRoot), &checkpoints); err != nil || !exists {
						if exists, err = beaconapi.FinalityCheckpoints(ctx, b.API, eth2api.StateIdSlot(headInfo.Header.Message.Slot), &checkpoints); err != nil {
							ch <- res{err: fmt.Errorf("beacon %d: failed to poll finality checkpoint: %v", i, err)}
							return
						} else if !exists {
							ch <- res{err: fmt.Errorf("beacon %d: Expected state for head block", i)}
							return
						}
					}

					slot = headInfo.Header.Message.Slot
					head = shorten(headInfo.Root.String())
					justified = shorten(checkpoints.CurrentJustified.String())
					finalized = shorten(checkpoints.Finalized.String())

					var (
						execution    tree.Root
						executionStr = "0x0000..0000"
					)

					if (checkpoints.Finalized != common.Checkpoint{}) {
						var versionedBlock eth2api.VersionedSignedBeaconBlock
						if exists, err := beaconapi.BlockV2(ctx, b.API, eth2api.BlockIdRoot(checkpoints.Finalized.Root), &versionedBlock); err != nil {
							ch <- res{err: fmt.Errorf("beacon %d: failed to retrieve block: %v", i, err)}
							return
						} else if !exists {
							ch <- res{err: fmt.Errorf("beacon %d: block not found", i)}
							return
						}

						switch versionedBlock.Version {
						case "bellatrix":
							block := versionedBlock.Data.(*bellatrix.SignedBeaconBlock)
							execution = block.Message.Body.ExecutionPayload.BlockHash
							executionStr = shorten(execution.String())
						}
					}

					ch <- res{i, fmt.Sprintf("beacon %d: slot=%d, head=%s, finalized_exec_payload=%s, justified=%s, finalized=%s", i, slot, head, executionStr, justified, finalized), nil}

					if (execution != tree.Root{}) {
						done <- checkpoints.Finalized
					}
				}(ctx, i, b, ch)
			}
			wg.Wait()
			close(ch)

			// print out logs in ascending idx order
			sorted := make([]string, len(runningBeacons))
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
func (t *Testnet) WaitForCurrentEpochFinalization(ctx context.Context, timeoutSlots common.Slot) (common.Checkpoint, error) {
	genesis := t.GenesisTime()
	slotDuration := time.Duration(t.spec.SECONDS_PER_SLOT) * time.Second
	timer := time.NewTicker(slotDuration)
	runningBeacons := t.VerificationNodes().BeaconClients().Running()
	done := make(chan common.Checkpoint, len(runningBeacons))
	var timeout <-chan time.Time
	if timeoutSlots > 0 {
		timeout = t.SlotsTimeout(timeoutSlots)
	} else {
		timeout = make(<-chan time.Time)
	}

	// Get the current head root which must be finalized
	var (
		epochToBeFinalized common.Epoch
		headInfo           eth2api.BeaconBlockHeaderAndInfo
	)
	if exists, err := beaconapi.BlockHeader(ctx, runningBeacons[0].API, eth2api.BlockHead, &headInfo); err != nil {
		return common.Checkpoint{}, fmt.Errorf("failed to poll head: %v", err)
	} else if !exists {
		return common.Checkpoint{}, fmt.Errorf("no head block")
	}
	epochToBeFinalized = t.spec.SlotToEpoch(headInfo.Header.Message.Slot)

	for {
		select {
		case <-ctx.Done():
			return common.Checkpoint{}, fmt.Errorf("context called")
		case <-timeout:
			return common.Checkpoint{}, fmt.Errorf("Timeout")
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
				ch = make(chan res, len(runningBeacons))
			)
			for i, b := range runningBeacons {
				wg.Add(1)
				go func(ctx context.Context, i int, b *BeaconClient, ch chan res) {
					defer wg.Done()
					ctx, cancel := context.WithTimeout(ctx, time.Second*5)
					defer cancel()

					var (
						slot      common.Slot
						head      string
						justified string
						finalized string
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
					if exists, err := beaconapi.FinalityCheckpoints(ctx, b.API, eth2api.StateIdRoot(headInfo.Header.Message.StateRoot), &checkpoints); err != nil || !exists {
						if exists, err = beaconapi.FinalityCheckpoints(ctx, b.API, eth2api.StateIdSlot(headInfo.Header.Message.Slot), &checkpoints); err != nil {
							ch <- res{err: fmt.Errorf("beacon %d: failed to poll finality checkpoint: %v", i, err)}
							return
						} else if !exists {
							ch <- res{err: fmt.Errorf("beacon %d: Expected state for head block", i)}
							return
						}
					}

					slot = headInfo.Header.Message.Slot
					head = shorten(headInfo.Root.String())
					justified = shorten(checkpoints.CurrentJustified.String())
					finalized = shorten(checkpoints.Finalized.String())

					ch <- res{i, fmt.Sprintf("beacon %d: slot=%d, head=%s justified=%s, finalized=%s, epoch_to_finalize=%d", i, slot, head, justified, finalized, epochToBeFinalized), nil}

					if checkpoints.Finalized != (common.Checkpoint{}) && checkpoints.Finalized.Epoch >= epochToBeFinalized {
						done <- checkpoints.Finalized
					}
				}(ctx, i, b, ch)
			}
			wg.Wait()
			close(ch)

			// print out logs in ascending idx order
			sorted := make([]string, len(runningBeacons))
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
func (t *Testnet) WaitForExecutionPayload(ctx context.Context, timeoutSlots common.Slot) (ethcommon.Hash, error) {
	genesis := t.GenesisTime()
	slotDuration := time.Duration(t.spec.SECONDS_PER_SLOT) * time.Second
	timer := time.NewTicker(slotDuration)
	runningBeacons := t.VerificationNodes().BeaconClients().Running()
	done := make(chan ethcommon.Hash, len(runningBeacons))
	userRPCAddress, err := t.ExecutionClients().Running()[0].UserRPCAddress()
	if err != nil {
		panic(err)
	}
	client := &http.Client{}
	rpcClient, _ := rpc.DialHTTPWithClient(userRPCAddress, client)
	ttdReached := false
	var timeout <-chan time.Time
	if timeoutSlots > 0 {
		timeout = t.SlotsTimeout(timeoutSlots)
	} else {
		timeout = make(<-chan time.Time)
	}
	for {
		select {
		case <-ctx.Done():
			return ethcommon.Hash{}, fmt.Errorf("context called")
		case result := <-done:
			return result, nil
		case <-timeout:
			return ethcommon.Hash{}, fmt.Errorf("Timeout")
		case tim := <-timer.C:
			// start polling after first slot of genesis
			if tim.Before(genesis.Add(slotDuration)) {
				t.Logf("Time till genesis: %s", genesis.Sub(tim))
				continue
			}

			if !ttdReached {
				// Check if TTD has been reached
				var td *TotalDifficultyHeader
				if err := rpcClient.CallContext(ctx, &td, "eth_getBlockByNumber", "latest", false); err == nil {
					if td.TotalDifficulty.ToInt().Cmp(t.eth1Genesis.Genesis.Config.TerminalTotalDifficulty) >= 0 {
						t.Logf("ttd (%d) reached at execution block %d", t.eth1Genesis.Genesis.Config.TerminalTotalDifficulty, td.Number)
						ttdReached = true
					}
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
				ch = make(chan res, len(runningBeacons))
			)
			for i, b := range runningBeacons {
				wg.Add(1)
				go func(ctx context.Context, i int, b *BeaconClient, ch chan res) {
					defer wg.Done()
					ctx, cancel := context.WithTimeout(ctx, time.Second*5)
					defer cancel()

					var (
						slot      common.Slot
						head      string
						execution ethcommon.Hash
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
						execution = ethcommon.Hash{}
					case "altair":
						execution = ethcommon.Hash{}
					case "bellatrix":
						block := versionedBlock.Data.(*bellatrix.SignedBeaconBlock)
						zero := ethcommon.Hash{}

						copy(execution[:], block.Message.Body.ExecutionPayload.BlockHash[:])

						if bytes.Compare(execution[:], zero[:]) != 0 {
							done <- execution
						}
					}

					slot = headInfo.Header.Message.Slot
					head = shorten(headInfo.Root.String())
					health, err := getHealth(ctx, b.API, t.spec, slot)
					if err != nil {
						// warning is printed here instead because some clients
						// don't support the required REST endpoint.
						fmt.Printf("WARN: beacon %d: %s\n", i, err)
					}

					ch <- res{i, fmt.Sprintf("beacon %d: slot=%d, head=%s, health=%.2f, exec_payload=%s", i, slot, head, health, shorten(execution.Hex())), nil}

				}(ctx, i, b, ch)
			}
			wg.Wait()
			close(ch)

			// print out logs in ascending idx order
			sorted := make([]string, len(runningBeacons))
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
