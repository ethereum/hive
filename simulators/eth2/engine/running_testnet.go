package main

import (
	"bytes"
	"context"
	"fmt"
	"math/big"
	"sync"
	"time"

	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
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

// VerifyParticipation ensures that the participation of the finialized epoch
// of a given checkpoint is above the expected threshold.
func (t *Testnet) VerifyParticipation(ctx context.Context, checkpoint *common.Checkpoint, expected float64) error {
	var (
		slot common.Slot
	)
	if checkpoint != nil {
		slot, _ = t.spec.EpochStartSlot(checkpoint.Epoch + 1)
	} else {
		var headInfo eth2api.BeaconBlockHeaderAndInfo
		if exists, err := beaconapi.BlockHeader(ctx, t.verificationBeacons()[0].API, eth2api.BlockHead, &headInfo); err != nil {
			return fmt.Errorf("failed to poll head: %v", err)
		} else if !exists {
			return fmt.Errorf("no head block")
		}
		slot = headInfo.Header.Message.Slot
	}
	if t.spec.BELLATRIX_FORK_EPOCH <= t.spec.SlotToEpoch(slot) {
		// slot-1 to target last slot in finalized epoch
		slot = slot - 1
	}
	for i, b := range t.verificationBeacons() {
		health, err := getHealth(ctx, b.API, t.spec, slot)
		if err != nil {
			return err
		}
		if health < expected {
			return fmt.Errorf("beacon %d: participation not healthy (got: %.2f, expected: %.2f)", i, health, expected)
		}
		t.t.Logf("beacon %d: epoch=%d participation=%.2f", i, t.spec.SlotToEpoch(slot), health)
	}
	return nil
}

// VerifyExecutionPayloadIsCanonical retrieves the execution payload from the
// finalized block and verifies that is in the execution client's canonical
// chain.
func (t *Testnet) VerifyExecutionPayloadIsCanonical(ctx context.Context, checkpoint *common.Checkpoint) error {
	var blockId eth2api.BlockIdSlot
	if checkpoint != nil {
		slot, _ := t.spec.EpochStartSlot(checkpoint.Epoch + 1)
		blockId = eth2api.BlockIdSlot(slot - 1)
	} else {
		var headInfo eth2api.BeaconBlockHeaderAndInfo
		if exists, err := beaconapi.BlockHeader(ctx, t.verificationBeacons()[0].API, eth2api.BlockHead, &headInfo); err != nil {
			return fmt.Errorf("failed to poll head: %v", err)
		} else if !exists {
			return fmt.Errorf("no head block")
		}
		blockId = eth2api.BlockIdSlot(headInfo.Header.Message.Slot)
	}
	var versionedBlock eth2api.VersionedSignedBeaconBlock
	if exists, err := beaconapi.BlockV2(ctx, t.verificationBeacons()[0].API, blockId, &versionedBlock); err != nil {
		return fmt.Errorf("beacon %d: failed to retrieve block: %v", 0, err)
	} else if !exists {
		return fmt.Errorf("beacon %d: block not found", 0)
	}
	if versionedBlock.Version != "bellatrix" {
		return nil
	}
	payload := versionedBlock.Data.(*bellatrix.SignedBeaconBlock).Message.Body.ExecutionPayload

	for i, proxy := range t.verificationProxies() {
		client := ethclient.NewClient(proxy.RPC())
		block, err := client.BlockByNumber(ctx, big.NewInt(int64(payload.BlockNumber)))
		if err != nil {
			return fmt.Errorf("eth1 %d: %s", 0, err)
		}
		if block.Hash() != [32]byte(payload.BlockHash) {
			return fmt.Errorf("eth1 %d: blocks don't match (got=%s, expected=%s)", i, shorten(block.Hash().String()), shorten(payload.BlockHash.String()))
		}
	}
	return nil
}

// VerifyExecutionPayloadIsCanonical retrieves the execution payload from the
// finalized block and verifies that is in the execution client's canonical
// chain.
func (t *Testnet) VerifyExecutionPayloadHashInclusion(ctx context.Context, checkpoint *common.Checkpoint, hash ethcommon.Hash) *bellatrix.SignedBeaconBlock {
	bn := t.verificationBeacons()[0]
	return t.VerifyExecutionPayloadHashInclusionNode(ctx, checkpoint, bn, hash)
}

func (t *Testnet) VerifyExecutionPayloadHashInclusionNode(ctx context.Context, checkpoint *common.Checkpoint, bn *BeaconNode, hash ethcommon.Hash) *bellatrix.SignedBeaconBlock {
	var (
		lastSlot common.Slot
	)
	if checkpoint != nil {
		lastSlot = t.spec.SLOTS_PER_EPOCH * common.Slot(checkpoint.Epoch)
	} else {
		lastSlot = t.spec.TimeToSlot(common.Timestamp(time.Now().Unix()), t.genesisTime)
	}
	enr, _ := bn.ENR()
	t.t.Logf("INFO: Looking for block %v in node %v", hash, enr)
	for slot := int(lastSlot); slot > 0; slot -= 1 {
		var versionedBlock eth2api.VersionedSignedBeaconBlock
		if exists, err := beaconapi.BlockV2(ctx, bn.API, eth2api.BlockIdSlot(slot), &versionedBlock); err != nil {
			continue
		} else if !exists {
			continue
		}
		if versionedBlock.Version != "bellatrix" {
			// Block can't contain an executable payload
			continue
		}
		block := versionedBlock.Data.(*bellatrix.SignedBeaconBlock)
		payload := block.Message.Body.ExecutionPayload
		if bytes.Compare(payload.BlockHash[:], hash[:]) == 0 {
			t.t.Logf("INFO: Execution block %v found in %d: %v", hash, block.Message.Slot, ethcommon.BytesToHash(payload.BlockHash[:]))
			return block
		}
	}
	return nil
}

// VerifyProposers checks that all validator clients have proposed a block on
// the finalized beacon chain that includes an execution payload.
func (t *Testnet) VerifyProposers(ctx context.Context, checkpoint *common.Checkpoint, allow_empty_blocks bool) error {
	var (
		lastSlot common.Slot
	)
	if checkpoint != nil {
		lastSlot = t.spec.SLOTS_PER_EPOCH * common.Slot(checkpoint.Epoch)
	} else {
		lastSlot = t.spec.TimeToSlot(common.Timestamp(time.Now().Unix()), t.genesisTime)
	}

	proposers := make([]bool, len(t.verificationBeacons()))
	for slot := 0; slot < int(lastSlot); slot += 1 {
		var versionedBlock eth2api.VersionedSignedBeaconBlock
		if exists, err := beaconapi.BlockV2(ctx, t.verificationBeacons()[0].API, eth2api.BlockIdSlot(slot), &versionedBlock); err != nil {
			if allow_empty_blocks {
				continue
			}
			return fmt.Errorf("beacon %d: failed to retrieve block: %v", 0, err)
		} else if !exists {
			if allow_empty_blocks {
				continue
			}
			return fmt.Errorf("beacon %d: block not found", 0)
		}
		var proposerIndex common.ValidatorIndex
		switch versionedBlock.Version {
		case "phase0":
			block := versionedBlock.Data.(*phase0.SignedBeaconBlock)
			proposerIndex = block.Message.ProposerIndex
		case "altair":
			block := versionedBlock.Data.(*altair.SignedBeaconBlock)
			proposerIndex = block.Message.ProposerIndex
		case "bellatrix":
			block := versionedBlock.Data.(*bellatrix.SignedBeaconBlock)
			proposerIndex = block.Message.ProposerIndex
		}

		var validator eth2api.ValidatorResponse
		if exists, err := beaconapi.StateValidator(ctx, t.verificationBeacons()[0].API, eth2api.StateIdSlot(slot), eth2api.ValidatorIdIndex(proposerIndex), &validator); err != nil {
			return fmt.Errorf("beacon %d: failed to retrieve validator: %v", 0, err)
		} else if !exists {
			return fmt.Errorf("beacon %d: validator not found", 0)
		}
		idx, err := t.ValidatorClientIndex([48]byte(validator.Validator.Pubkey))
		if err != nil {
			return fmt.Errorf("pub key not found on any validator client")
		}
		proposers[idx] = true
	}
	for i, proposed := range proposers {
		if !proposed {
			return fmt.Errorf("beacon %d: did not propose a block", i)
		}
	}
	return nil
}

func (t *Testnet) VerifyELHeads(ctx context.Context) error {
	client := ethclient.NewClient(t.verificationExecution()[0].RPC())
	head, err := client.HeaderByNumber(ctx, nil)
	if err != nil {
		return err
	}

	t.t.Logf("Verifying EL heads at %v", head.Hash())
	for i, node := range t.verificationExecution() {
		client := ethclient.NewClient(node.RPC())
		head2, err := client.HeaderByNumber(ctx, nil)
		if err != nil {
			return err
		}
		if head.Hash() != head2.Hash() {
			return fmt.Errorf("different heads: %v: %v %v: %v", 0, head, i, head2)
		}
	}
	return nil
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
