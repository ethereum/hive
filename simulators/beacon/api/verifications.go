package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/ethereum/hive/hivesim"
	beacon_client "github.com/marioevz/eth-clients/clients/beacon"
	"github.com/protolambda/eth2api"
	"github.com/protolambda/zrnt/eth2/beacon/altair"
	"github.com/protolambda/zrnt/eth2/beacon/common"
	"github.com/protolambda/zrnt/eth2/beacon/phase0"
	"github.com/protolambda/ztyp/tree"
	"github.com/protolambda/ztyp/view"
	"gopkg.in/yaml.v3"
)

type BeaconAPITestStep interface {
	Execute(t *hivesim.T, parentCtx context.Context, cl *beacon_client.BeaconClient)
}

// Verifications Interface
type BeaconAPITestSteps struct {
	Verifications []BeaconAPITestStep
}

// Beacon API Verifications will be parsed from yaml by identified the first field "method"
func (l *BeaconAPITestSteps) UnmarshalYAML(unmarshal func(interface{}) error) error {
	// Unmarshal the YAML list into YAML nodes in order to read their tags
	var rawNodes []yaml.Node
	if err := unmarshal(&rawNodes); err != nil {
		return err
	}

	// Iterate over the raw list and create the appropriate object based on the "method" field
	for _, node := range rawNodes {
		var object BeaconAPITestStep
		objectType := strings.TrimPrefix(node.Tag, "!")
		switch objectType {
		case "EthV2DebugBeaconStates":
			object = &EthV2DebugBeaconStates{}
		case "EthV1BeaconStatesFork":
			object = &EthV1BeaconStatesFork{}
		case "EthV1BeaconStatesFinalityCheckpoints":
			object = &EthV1BeaconStatesFinalityCheckpoints{}
		default:
			log.WithField("type", objectType).Warn("unknown type parsing hive.yaml")
			continue
		}

		// Decode the object to the appropriate type
		if err := node.Decode(object); err != nil {
			return err
		}

		// Add the object to the object list
		l.Verifications = append(l.Verifications, object)
	}

	return nil
}

func (l *BeaconAPITestSteps) DoVerifications(t *hivesim.T, parentCtx context.Context, cl *beacon_client.BeaconClient) {
	for _, v := range l.Verifications {
		v.Execute(t, parentCtx, cl)
	}
}

// Helpers
func StringToStateID(str string) (eth2api.StateId, error) {
	switch str {
	case "head":
		return eth2api.StateHead, nil
	case "genesis":
		return eth2api.StateGenesis, nil
	case "finalized":
		return eth2api.StateFinalized, nil
	case "justified":
		return eth2api.StateJustified, nil
	default:
		if strings.HasPrefix(str, "root:") {
			root := tree.Root{}
			root.UnmarshalText([]byte(strings.TrimPrefix(str, "root:")))
			return eth2api.StateIdRoot(root), nil
		} else if strings.HasPrefix(str, "slot:") {
			var slot view.Uint64View
			slot.UnmarshalText([]byte(strings.TrimPrefix(str, "slot:")))
			return eth2api.StateIdSlot(slot), nil
		}
	}
	return nil, fmt.Errorf("unknown state id: %s", str)
}

// eth/v2/debug/beacon/states/{state_id}
type EthV2DebugBeaconStatesFields struct {
	// Common Fields
	StateRoot             *tree.Root        `yaml:"state_root"`
	Slot                  *common.Slot      `yaml:"slot"`
	GenesisTime           *common.Timestamp `yaml:"genesis_time"`
	GenesisValidatorsRoot *common.Root      `yaml:"genesis_validators_root"`
	Fork                  *common.Fork      `yaml:"fork"`

	// History
	LatestBlockHeader *common.BeaconBlockHeader    `yaml:"latest_block_header"`
	BlockRoots        *phase0.HistoricalBatchRoots `yaml:"block_roots"`
	StateRoots        *phase0.HistoricalBatchRoots `yaml:"state_roots"`
	HistoricalRoots   *phase0.HistoricalRoots      `yaml:"historical_roots"`

	// Eth1
	Eth1Data         *common.Eth1Data      `yaml:"eth1_data"`
	Eth1DataVotes    *phase0.Eth1DataVotes `yaml:"eth1_data_votes"`
	Eth1DepositIndex *common.DepositIndex  `yaml:"eth1_deposit_index"`

	// Registry
	Validators *phase0.ValidatorRegistry `yaml:"validators"`
	Balances   *phase0.Balances          `yaml:"balances"`

	// Randao
	RandaoMixes *phase0.RandaoMixes `yaml:"randao_mixes"`

	// Slashings
	Slashings *phase0.SlashingsHistory `yaml:"slashings"`

	// Attestations / Participation
	PreviousEpochAttestations  *phase0.PendingAttestations   `yaml:"previous_epoch_attestations"`
	CurrentEpochAttestations   *phase0.PendingAttestations   `yaml:"current_epoch_attestations"`
	PreviousEpochParticipation *altair.ParticipationRegistry `yaml:"previous_epoch_participation"`
	CurrentEpochParticipation  *altair.ParticipationRegistry `yaml:"current_epoch_participation"`

	// Finality
	JustificationBits           *common.JustificationBits `yaml:"justification_bits"`
	PreviousJustifiedCheckpoint *common.Checkpoint        `yaml:"previous_justified_checkpoint"`
	CurrentJustifiedCheckpoint  *common.Checkpoint        `yaml:"current_justified_checkpoint"`
	FinalizedCheckpoint         *common.Checkpoint        `yaml:"finalized_checkpoint"`

	// Altair
	InactivityScores *altair.InactivityScores `yaml:"inactivity_scores"`
	// SyncCommittee
	CurrentSyncCommittee *common.SyncCommittee `yaml:"current_sync_committee"`
	NextSyncCommittee    *common.SyncCommittee `yaml:"next_sync_committee"`
}
type EthV2DebugBeaconStates struct {
	Id     string                       `yaml:"id"`
	Fields EthV2DebugBeaconStatesFields `yaml:"fields"`
	Error  bool                         `yaml:"error"`
}

func (v EthV2DebugBeaconStates) Execute(t *hivesim.T, parentCtx context.Context, cl *beacon_client.BeaconClient) {
	t.Logf("verifying beacon state v2")
	stateId, err := StringToStateID(v.Id)
	if err != nil {
		t.Fatalf("failed to parse state id: %v", err)
	}

	s, err := cl.BeaconStateV2(parentCtx, stateId)
	if err != nil {
		if v.Error {
			t.Logf("expected error: %v", err)
			return
		}
		t.Fatalf("failed to get state: %v", err)
	}

	if v.Fields.StateRoot != nil {
		if s.Root() != *v.Fields.StateRoot {
			t.Fatalf("state root mismatch: got %s, expected %s", s.Root(), v.Fields.StateRoot)
		}
		t.Logf("state root matches: %s", s.Root())
	}

	if v.Fields.Slot != nil {
		if s.StateSlot() != *v.Fields.Slot {
			t.Fatalf("slot mismatch: got %d, expected %d", s.StateSlot(), v.Fields.Slot)
		}
		t.Logf("slot matches: %s", s.StateSlot())
	}

	if v.Fields.GenesisTime != nil {
		if s.GenesisTime() != *v.Fields.GenesisTime {
			t.Fatalf("genesis time mismatch: got %d, expected %d", s.GenesisTime(), v.Fields.GenesisTime)
		}
		t.Logf("genesis time matches: %s", s.GenesisTime())
	}

	if v.Fields.GenesisValidatorsRoot != nil {
		if s.GenesisValidatorsRoot() != *v.Fields.GenesisValidatorsRoot {
			t.Fatalf("genesis validators root mismatch: got %d, expected %d", s.GenesisValidatorsRoot(), v.Fields.GenesisValidatorsRoot)
		}
		t.Logf("genesis validators root matches: %s", s.GenesisValidatorsRoot())
	}

	if v.Fields.Fork != nil {
		fork := s.Fork()
		if fork.CurrentVersion != v.Fields.Fork.CurrentVersion {
			t.Fatalf("fork current version mismatch: got %d, expected %d", fork.CurrentVersion, v.Fields.Fork.CurrentVersion)
		}
		if fork.PreviousVersion != v.Fields.Fork.PreviousVersion {
			t.Fatalf("fork previous version mismatch: got %d, expected %d", fork.PreviousVersion, v.Fields.Fork.PreviousVersion)
		}
		if fork.Epoch != v.Fields.Fork.Epoch {
			t.Fatalf("fork epoch mismatch: got %d, expected %d", fork.Epoch, v.Fields.Fork.Epoch)
		}
		t.Logf("fork matches: %s", fork)
	}

	// History

	if v.Fields.LatestBlockHeader != nil {
		header := s.LatestBlockHeader()
		if header.Slot != v.Fields.LatestBlockHeader.Slot {
			t.Fatalf("latest block header slot mismatch: got %d, expected %d", header.Slot, v.Fields.LatestBlockHeader.Slot)
		}
		if header.ProposerIndex != v.Fields.LatestBlockHeader.ProposerIndex {
			t.Fatalf("latest block header proposer index mismatch: got %d, expected %d", header.ProposerIndex, v.Fields.LatestBlockHeader.ProposerIndex)
		}
		if header.ParentRoot != v.Fields.LatestBlockHeader.ParentRoot {
			t.Fatalf("latest block header parent root mismatch: got %s, expected %s", header.ParentRoot, v.Fields.LatestBlockHeader.ParentRoot)
		}
		if header.StateRoot != v.Fields.LatestBlockHeader.StateRoot {
			t.Fatalf("latest block header state root mismatch: got %s, expected %s", header.StateRoot, v.Fields.LatestBlockHeader.StateRoot)
		}
		if header.BodyRoot != v.Fields.LatestBlockHeader.BodyRoot {
			t.Fatalf("latest block header body root mismatch: got %s, expected %s", header.BodyRoot, v.Fields.LatestBlockHeader.BodyRoot)
		}
		t.Logf("latest block header matches: %s", header)
	}

	if v.Fields.BlockRoots != nil {
		roots := s.BlockRoots()
		if len(roots) != len(*v.Fields.BlockRoots) {
			t.Fatalf("block roots length mismatch: got %d, expected %d", len(roots), len(*v.Fields.BlockRoots))
		}
		for i, root := range roots {
			if root != (*v.Fields.BlockRoots)[i] {
				t.Fatalf("block root mismatch: got %s, expected %s", root, (*v.Fields.BlockRoots)[i])
			}
		}
		t.Logf("block roots match")
	}

	if v.Fields.StateRoots != nil {
		roots := s.StateRoots()
		if len(roots) != len(*v.Fields.StateRoots) {
			t.Fatalf("state roots length mismatch: got %d, expected %d", len(roots), len(*v.Fields.StateRoots))
		}
		for i, root := range roots {
			if root != (*v.Fields.StateRoots)[i] {
				t.Fatalf("state root mismatch: got %s, expected %s", root, (*v.Fields.StateRoots)[i])
			}
		}
		t.Logf("state roots match")
	}

	if v.Fields.HistoricalRoots != nil {
		roots := s.HistoricalRoots()
		if len(roots) != len(*v.Fields.HistoricalRoots) {
			t.Fatalf("historical roots length mismatch: got %d, expected %d", len(roots), len(*v.Fields.HistoricalRoots))
		}
		for i, root := range roots {
			if root != (*v.Fields.HistoricalRoots)[i] {
				t.Fatalf("historical root mismatch: got %s, expected %s", root, (*v.Fields.HistoricalRoots)[i])
			}
		}
		t.Logf("historical roots match")
	}

	// Eth1Data

	if v.Fields.Eth1Data != nil {
		data := s.Eth1Data()
		if data.DepositRoot != v.Fields.Eth1Data.DepositRoot {
			t.Fatalf("eth1 data deposit root mismatch: got %s, expected %s", data.DepositRoot, v.Fields.Eth1Data.DepositRoot)
		}
		if data.DepositCount != v.Fields.Eth1Data.DepositCount {
			t.Fatalf("eth1 data deposit count mismatch: got %d, expected %d", data.DepositCount, v.Fields.Eth1Data.DepositCount)
		}
		if data.BlockHash != v.Fields.Eth1Data.BlockHash {
			t.Fatalf("eth1 data block hash mismatch: got %s, expected %s", data.BlockHash, v.Fields.Eth1Data.BlockHash)
		}
		t.Logf("eth1 data matches: %s", data)
	}

	if v.Fields.Eth1DataVotes != nil {
		votes := s.Eth1DataVotes()
		if len(votes) != len(*v.Fields.Eth1DataVotes) {
			t.Fatalf("eth1 data votes length mismatch: got %d, expected %d", len(votes), len(*v.Fields.Eth1DataVotes))
		}
		for i, vote := range votes {
			if vote != (*v.Fields.Eth1DataVotes)[i] {
				t.Fatalf("eth1 data vote mismatch: got %s, expected %s", vote, (*v.Fields.Eth1DataVotes)[i])
			}
		}
		t.Logf("eth1 data votes match")
	}

	if v.Fields.Eth1DepositIndex != nil {
		index := s.Eth1DepositIndex()
		if index != *v.Fields.Eth1DepositIndex {
			t.Fatalf("eth1 deposit index mismatch: got %d, expected %d", index, *v.Fields.Eth1DepositIndex)
		}
		t.Logf("eth1 deposit index matches: %d", index)
	}

	// Registry
	if v.Fields.Validators != nil {
		validators := s.Validators()
		if len(validators) != len(*v.Fields.Validators) {
			t.Fatalf("validators length mismatch: got %d, expected %d", len(validators), len(*v.Fields.Validators))
		}
		for i, validator := range validators {
			if validator.Pubkey != (*v.Fields.Validators)[i].Pubkey {
				t.Fatalf("validator pubkey mismatch: got %s, expected %s", validator.Pubkey, (*v.Fields.Validators)[i].Pubkey)
			}
			if validator.WithdrawalCredentials != (*v.Fields.Validators)[i].WithdrawalCredentials {
				t.Fatalf("validator withdrawal credentials mismatch: got %s, expected %s", validator.WithdrawalCredentials, (*v.Fields.Validators)[i].WithdrawalCredentials)
			}
			if validator.EffectiveBalance != (*v.Fields.Validators)[i].EffectiveBalance {
				t.Fatalf("validator effective balance mismatch: got %d, expected %d", validator.EffectiveBalance, (*v.Fields.Validators)[i].EffectiveBalance)
			}
			if validator.Slashed != (*v.Fields.Validators)[i].Slashed {
				t.Fatalf("validator slashed mismatch: got %t, expected %t", validator.Slashed, (*v.Fields.Validators)[i].Slashed)
			}
			if validator.ActivationEligibilityEpoch != (*v.Fields.Validators)[i].ActivationEligibilityEpoch {
				t.Fatalf("validator activation eligibility epoch mismatch: got %d, expected %d", validator.ActivationEligibilityEpoch, (*v.Fields.Validators)[i].ActivationEligibilityEpoch)
			}
			if validator.ActivationEpoch != (*v.Fields.Validators)[i].ActivationEpoch {
				t.Fatalf("validator activation epoch mismatch: got %d, expected %d", validator.ActivationEpoch, (*v.Fields.Validators)[i].ActivationEpoch)
			}
			if validator.ExitEpoch != (*v.Fields.Validators)[i].ExitEpoch {
				t.Fatalf("validator exit epoch mismatch: got %d, expected %d", validator.ExitEpoch, (*v.Fields.Validators)[i].ExitEpoch)
			}
			if validator.WithdrawableEpoch != (*v.Fields.Validators)[i].WithdrawableEpoch {
				t.Fatalf("validator withdrawable epoch mismatch: got %d, expected %d", validator.WithdrawableEpoch, (*v.Fields.Validators)[i].WithdrawableEpoch)
			}
		}
		t.Logf("validators match")
	}
	if v.Fields.Balances != nil {
		balances := s.Balances()
		if len(balances) != len(*v.Fields.Balances) {
			t.Fatalf("balances length mismatch: got %d, expected %d", len(balances), len(*v.Fields.Balances))
		}
		for i, balance := range balances {
			if balance != (*v.Fields.Balances)[i] {
				t.Fatalf("balance mismatch: got %d, expected %d", balance, (*v.Fields.Balances)[i])
			}
		}
		t.Logf("balances match")
	}

	// Randao
	if v.Fields.RandaoMixes != nil {
		mixes := s.RandaoMixes()
		if len(mixes) != len(*v.Fields.RandaoMixes) {
			t.Fatalf("randao mixes length mismatch: got %d, expected %d", len(mixes), len(*v.Fields.RandaoMixes))
		}
		for i, mix := range mixes {
			if mix != (*v.Fields.RandaoMixes)[i] {
				t.Fatalf("randao mix mismatch: got %s, expected %s", mix, (*v.Fields.RandaoMixes)[i])
			}
		}
		t.Logf("randao mixes match")
	}

	// Slashings
	if v.Fields.Slashings != nil {
		slashings := s.Slashings()
		if len(slashings) != len(*v.Fields.Slashings) {
			t.Fatalf("slashings length mismatch: got %d, expected %d", len(slashings), len(*v.Fields.Slashings))
		}
		for i, slashing := range slashings {
			if slashing != (*v.Fields.Slashings)[i] {
				t.Fatalf("slashing mismatch: got %d, expected %d", slashing, (*v.Fields.Slashings)[i])
			}
		}
		t.Logf("slashings match")
	}

	// Attestations / Participation
	if v.Fields.PreviousEpochAttestations != nil {
		attestations := s.PreviousEpochAttestations()
		if len(attestations) != len(*v.Fields.PreviousEpochAttestations) {
			t.Fatalf("previous epoch attestations length mismatch: got %d, expected %d", len(attestations), len(*v.Fields.PreviousEpochAttestations))
		}
		for i, attestation := range attestations {
			if attestation != (*v.Fields.PreviousEpochAttestations)[i] {
				t.Fatalf("previous epoch attestation mismatch: got %s, expected %s", attestation, (*v.Fields.PreviousEpochAttestations)[i])
			}
		}
		t.Logf("previous epoch attestations match")
	}
	if v.Fields.CurrentEpochAttestations != nil {
		attestations := s.CurrentEpochAttestations()
		if len(attestations) != len(*v.Fields.CurrentEpochAttestations) {
			t.Fatalf("current epoch attestations length mismatch: got %d, expected %d", len(attestations), len(*v.Fields.CurrentEpochAttestations))
		}
		for i, attestation := range attestations {
			if attestation != (*v.Fields.CurrentEpochAttestations)[i] {
				t.Fatalf("current epoch attestation mismatch: got %s, expected %s", attestation, (*v.Fields.CurrentEpochAttestations)[i])
			}
		}
		t.Logf("current epoch attestations match")
	}
	if v.Fields.PreviousEpochParticipation != nil {
		participation := s.PreviousEpochParticipation()
		if len(participation) != len(*v.Fields.PreviousEpochParticipation) {
			t.Fatalf("previous epoch participation length mismatch: got %d, expected %d", len(participation), len(*v.Fields.PreviousEpochParticipation))
		}
		for i, p := range participation {
			if p != (*v.Fields.PreviousEpochParticipation)[i] {
				t.Fatalf("previous epoch participation mismatch: got %d, expected %d", p, (*v.Fields.PreviousEpochParticipation)[i])
			}
		}
		t.Logf("previous epoch participation match")
	}
	if v.Fields.CurrentEpochParticipation != nil {
		participation := s.CurrentEpochParticipation()
		if len(participation) != len(*v.Fields.CurrentEpochParticipation) {
			t.Fatalf("current epoch participation length mismatch: got %d, expected %d", len(participation), len(*v.Fields.CurrentEpochParticipation))
		}
		for i, p := range participation {
			if p != (*v.Fields.CurrentEpochParticipation)[i] {
				t.Fatalf("current epoch participation mismatch: got %d, expected %d", p, (*v.Fields.CurrentEpochParticipation)[i])
			}
		}
		t.Logf("current epoch participation match")
	}

	// Finality
	if v.Fields.JustificationBits != nil {
		bits := s.JustificationBits()
		if len(bits) != len(*v.Fields.JustificationBits) {
			t.Fatalf("justification bits length mismatch: got %d, expected %d", len(bits), len(*v.Fields.JustificationBits))
		}
		for i, bit := range bits {
			if bit != (*v.Fields.JustificationBits)[i] {
				t.Fatalf("justification bit mismatch: got %d, expected %d", bit, (*v.Fields.JustificationBits)[i])
			}
		}
		t.Logf("justification bits match")
	}
	if v.Fields.PreviousJustifiedCheckpoint != nil {
		checkpoint := s.PreviousJustifiedCheckpoint()
		if checkpoint != *v.Fields.PreviousJustifiedCheckpoint {
			t.Fatalf("previous justified checkpoint mismatch: got %s, expected %s", checkpoint, *v.Fields.PreviousJustifiedCheckpoint)
		}
		t.Logf("previous justified checkpoint matches")
	}
	if v.Fields.CurrentJustifiedCheckpoint != nil {
		checkpoint := s.CurrentJustifiedCheckpoint()
		if checkpoint != *v.Fields.CurrentJustifiedCheckpoint {
			t.Fatalf("current justified checkpoint mismatch: got %s, expected %s", checkpoint, *v.Fields.CurrentJustifiedCheckpoint)
		}
		t.Logf("current justified checkpoint matches")
	}
	if v.Fields.FinalizedCheckpoint != nil {
		checkpoint := s.FinalizedCheckpoint()
		if checkpoint != *v.Fields.FinalizedCheckpoint {
			t.Fatalf("finalized checkpoint mismatch: got %s, expected %s", checkpoint, *v.Fields.FinalizedCheckpoint)
		}
		t.Logf("finalized checkpoint matches")
	}

	// Altair
	if v.Fields.InactivityScores != nil {
		scores := s.InactivityScores()
		if scores == nil {
			t.Fatalf("inactivity scores are unexpectedly nil")
		}
		if len(scores) != len(*v.Fields.InactivityScores) {
			t.Fatalf("inactivity scores length mismatch: got %d, expected %d", len(scores), len(*v.Fields.InactivityScores))
		}
		for i, score := range scores {
			if score != (*v.Fields.InactivityScores)[i] {
				t.Fatalf("inactivity score mismatch: got %d, expected %d", score, (*v.Fields.InactivityScores)[i])
			}
		}
		t.Logf("inactivity scores match")
	}
	if v.Fields.CurrentSyncCommittee != nil {
		committee := s.CurrentSyncCommittee()
		if committee == nil {
			t.Fatalf("current sync committee is unexpectedly nil")
		}
		if committee.AggregatePubkey != (*v.Fields.CurrentSyncCommittee).AggregatePubkey {
			t.Fatalf("current sync committee aggregate pubkey mismatch: got %s, expected %s", committee.AggregatePubkey, (*v.Fields.CurrentSyncCommittee).AggregatePubkey)
		}
		if len(committee.Pubkeys) != len((*v.Fields.CurrentSyncCommittee).Pubkeys) {
			t.Fatalf("current sync committee pubkeys length mismatch: got %d, expected %d", len(committee.Pubkeys), len((*v.Fields.CurrentSyncCommittee).Pubkeys))
		}
		for i, pubkey := range committee.Pubkeys {
			if pubkey != (*v.Fields.CurrentSyncCommittee).Pubkeys[i] {
				t.Fatalf("current sync committee pubkey mismatch: got %s, expected %s", pubkey, (*v.Fields.CurrentSyncCommittee).Pubkeys[i])
			}
		}
	}
	if v.Fields.NextSyncCommittee != nil {
		committee := s.NextSyncCommittee()
		if committee == nil {
			t.Fatalf("next sync committee is unexpectedly nil")
		}
		if committee.AggregatePubkey != (*v.Fields.NextSyncCommittee).AggregatePubkey {
			t.Fatalf("next sync committee aggregate pubkey mismatch: got %s, expected %s", committee.AggregatePubkey, (*v.Fields.NextSyncCommittee).AggregatePubkey)
		}
		if len(committee.Pubkeys) != len((*v.Fields.NextSyncCommittee).Pubkeys) {
			t.Fatalf("next sync committee pubkeys length mismatch: got %d, expected %d", len(committee.Pubkeys), len((*v.Fields.NextSyncCommittee).Pubkeys))
		}
		for i, pubkey := range committee.Pubkeys {
			if pubkey != (*v.Fields.NextSyncCommittee).Pubkeys[i] {
				t.Fatalf("next sync committee pubkey mismatch: got %s, expected %s", pubkey, (*v.Fields.NextSyncCommittee).Pubkeys[i])
			}
		}
	}

	// TODO: Bellatrix, Capella, Deneb fields
}

// eth/v1/beacon/states/{state_id}/finality_checkpoints
type EthV1BeaconStatesFork struct {
	Id                  string      `yaml:"id"`
	Finalized           bool        `yaml:"finalized"`
	ExecutionOptimistic *bool       `yaml:"execution_optimistic"`
	Data                common.Fork `yaml:"data"`
	Error               bool        `yaml:"error"`
}

func (v EthV1BeaconStatesFork) Execute(t *hivesim.T, parentCtx context.Context, cl *beacon_client.BeaconClient) {
	t.Logf("verifying finality checkpoints v1")
	stateId, err := StringToStateID(v.Id)
	if err != nil {
		t.Fatalf("failed to parse state id: %v", err)
	}

	f, err := cl.StateFork(parentCtx, stateId)
	if err != nil {
		if v.Error {
			t.Logf("expected error: %v", err)
			return
		}
		t.Fatalf("failed to get state finality checkpoints: %v", err)
	}

	// TODO: Check finalized and execution_optimistic

	if f.CurrentVersion != v.Data.CurrentVersion {
		t.Fatalf("current version mismatch: got %s, expected %s", f.CurrentVersion, v.Data.CurrentVersion)
	}
	t.Logf("current version matches")

	if f.PreviousVersion != v.Data.PreviousVersion {
		t.Fatalf("previous version mismatch: got %s, expected %s", f.PreviousVersion, v.Data.PreviousVersion)
	}
	t.Logf("previous version matches")

	if f.Epoch != v.Data.Epoch {
		t.Fatalf("epoch mismatch: got %s, expected %s", f.Epoch, v.Data.Epoch)
	}
	t.Logf("epoch matches")

}

// eth/v1/beacon/states/{state_id}/finality_checkpoints
type EthV1BeaconStatesFinalityCheckpoints struct {
	Id                  string                      `yaml:"id"`
	Finalized           bool                        `yaml:"finalized"`
	ExecutionOptimistic *bool                       `yaml:"execution_optimistic"`
	Data                eth2api.FinalityCheckpoints `yaml:"data"`
	Error               bool                        `yaml:"error"`
}

func (v EthV1BeaconStatesFinalityCheckpoints) Execute(t *hivesim.T, parentCtx context.Context, cl *beacon_client.BeaconClient) {
	t.Logf("verifying finality checkpoints v1")
	stateId, err := StringToStateID(v.Id)
	if err != nil {
		t.Fatalf("failed to parse state id: %v", err)
	}

	s, err := cl.StateFinalityCheckpoints(parentCtx, stateId)
	if err != nil {
		if v.Error {
			t.Logf("expected error: %v", err)
			return
		}
		t.Fatalf("failed to get state finality checkpoints: %v", err)
	}

	// TODO: Check finalized and execution_optimistic

	if s.PreviousJustified != v.Data.PreviousJustified {
		t.Fatalf("previous justified checkpoint mismatch: got %s, expected %s", s.PreviousJustified, v.Data.PreviousJustified)
	}
	t.Logf("previous justified checkpoint matches")

	if s.CurrentJustified != v.Data.CurrentJustified {
		t.Fatalf("current justified checkpoint mismatch: got %s, expected %s", s.CurrentJustified, v.Data.CurrentJustified)
	}
	t.Logf("current justified checkpoint matches")

	if s.Finalized != v.Data.Finalized {
		t.Fatalf("finalized checkpoint mismatch: got %s, expected %s", s.Finalized, v.Data.Finalized)
	}
	t.Logf("finalized checkpoint matches")

}
