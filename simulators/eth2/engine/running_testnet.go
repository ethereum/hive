package main

import (
	"bytes"
	"context"
	"fmt"
	"math/big"
	"sync"
	"time"

	ethcommon "github.com/ethereum/go-ethereum/common"
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

	beacons      BeaconNodes
	validators   []*ValidatorClient
	eth1         []*Eth1Node
	proxies      []*Proxy
	verification []bool
}

func (t *Testnet) verificationExecution() []*Eth1Node {
	verificationExecution := make([]*Eth1Node, 0)
	for i, v := range t.verification {
		if v {
			verificationExecution = append(verificationExecution, t.eth1[i])
		}
	}
	if len(verificationExecution) == 0 {
		return t.eth1
	}
	return verificationExecution
}

func (t *Testnet) verificationBeacons() []*BeaconNode {
	verificationBeacons := make([]*BeaconNode, 0)
	for i, v := range t.verification {
		if v {
			verificationBeacons = append(verificationBeacons, t.beacons[i])
		}
	}
	if len(verificationBeacons) == 0 {
		return t.beacons
	}
	return verificationBeacons
}

func (t *Testnet) verificationProxies() []*Proxy {
	verificationProxies := make([]*Proxy, 0)
	for i, v := range t.verification {
		if v {
			verificationProxies = append(verificationProxies, t.proxies[i])
		}
	}
	if len(verificationProxies) == 0 {
		return t.proxies
	}
	return verificationProxies
}

func (t *Testnet) removeNodeAsVerifier(id int) error {
	if id >= len(t.verification) {
		return fmt.Errorf("Node %d does not exist", id)
	}
	var any bool
	for _, v := range t.verification {
		if v {
			any = true
			break
		}
	}
	if any {
		t.verification[id] = false
	} else {
		// If no node is set as verifier, we will set all other nodes as verifiers then
		for i := range t.verification {
			t.verification[i] = (i != id)
		}
	}
	return nil
}

func startTestnet(t *hivesim.T, env *testEnv, config *Config) *Testnet {
	prep := prepareTestnet(t, env, config)
	testnet := prep.createTestnet(t)

	genesisTime := testnet.GenesisTime()
	countdown := genesisTime.Sub(time.Now())
	t.Logf("Created new testnet, genesis at %s (%s from now)", genesisTime, countdown)

	// for each key partition, we start a validator client with its own beacon node and eth1 node
	for i, node := range config.Nodes {
		prep.startEth1Node(testnet, env.Clients.ClientByNameAndRole(node.ExecutionClient, "eth1"), config.Eth1Consensus, node.ExecutionClientTTD)
		if node.ConsensusClient != "" {
			prep.startBeaconNode(testnet, env.Clients.ClientByNameAndRole(fmt.Sprintf("%s-bn", node.ConsensusClient), "beacon"), node.BeaconNodeTTD, []int{i})
			prep.startValidatorClient(testnet, env.Clients.ClientByNameAndRole(fmt.Sprintf("%s-vc", node.ConsensusClient), "validator"), i, i)
		}
		testnet.verification = append(testnet.verification, node.TestVerificationNode)
	}

	return testnet
}

func (t *Testnet) stopTestnet() {
	for _, p := range t.proxies {
		p.cancel()
	}
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

// WaitForFinality blocks until a beacon client reaches finality.
func (t *Testnet) WaitForFinality(ctx context.Context) (common.Checkpoint, error) {
	genesis := t.GenesisTime()
	slotDuration := time.Duration(t.spec.SECONDS_PER_SLOT) * time.Second
	timer := time.NewTicker(slotDuration)
	done := make(chan common.Checkpoint, len(t.verificationBeacons()))

	for {
		select {
		case <-ctx.Done():
			return common.Checkpoint{}, fmt.Errorf("context called")
		case finalized := <-done:
			return finalized, nil
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
				err error
			}
			var (
				wg sync.WaitGroup
				ch = make(chan res, len(t.verificationBeacons()))
			)
			for i, b := range t.verificationBeacons() {
				wg.Add(1)
				go func(ctx context.Context, i int, b *BeaconNode, ch chan res) {
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
			sorted := make([]string, len(t.verificationBeacons()))
			for out := range ch {
				if out.err != nil {
					return common.Checkpoint{}, out.err
				}
				sorted[out.idx] = out.msg
			}
			for _, msg := range sorted {
				t.t.Logf(msg)
			}
		}
	}
}

// Waits for any execution payload to be available included in a beacon block (merge)
func (t *Testnet) WaitForExecutionPayload(ctx context.Context) (ethcommon.Hash, error) {
	genesis := t.GenesisTime()
	slotDuration := time.Duration(t.spec.SECONDS_PER_SLOT) * time.Second
	timer := time.NewTicker(slotDuration)
	done := make(chan ethcommon.Hash, len(t.verificationBeacons()))

	for {
		select {
		case <-ctx.Done():
			return ethcommon.Hash{}, fmt.Errorf("context called")
		case execHash := <-done:
			return execHash, nil
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
				err error
			}

			var (
				wg sync.WaitGroup
				ch = make(chan res, len(t.verificationBeacons()))
			)
			for i, b := range t.verificationBeacons() {
				wg.Add(1)
				go func(ctx context.Context, i int, b *BeaconNode, ch chan res) {
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
			sorted := make([]string, len(t.verificationBeacons()))
			for out := range ch {
				if out.err != nil {
					return ethcommon.Hash{}, out.err
				}
				sorted[out.idx] = out.msg
			}
			for _, msg := range sorted {
				t.t.Logf(msg)
			}

		}
	}
}

func (t *Testnet) GetBeaconBlockByExecutionHash(ctx context.Context, hash ethcommon.Hash) *bellatrix.SignedBeaconBlock {
	lastSlot := t.spec.TimeToSlot(common.Timestamp(time.Now().Unix()), t.genesisTime)
	for slot := int(lastSlot); slot > 0; slot -= 1 {
		for _, bn := range t.verificationBeacons() {
			var versionedBlock eth2api.VersionedSignedBeaconBlock
			if exists, err := beaconapi.BlockV2(ctx, bn.API, eth2api.BlockIdSlot(slot), &versionedBlock); err != nil {
				continue
			} else if !exists {
				continue
			}
			if versionedBlock.Version != "bellatrix" {
				// Block can't contain an executable payload, and we are not going to find it going backwards, so return.
				return nil
			}
			block := versionedBlock.Data.(*bellatrix.SignedBeaconBlock)
			payload := block.Message.Body.ExecutionPayload
			if bytes.Compare(payload.BlockHash[:], hash[:]) == 0 {
				t.t.Logf("INFO: Execution block %v found in %d: %v", hash, block.Message.Slot, ethcommon.BytesToHash(payload.BlockHash[:]))
				return block
			}
		}

	}
	return nil
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
