package suite_engine

import (
	"fmt"

	api "github.com/ethereum/go-ethereum/beacon/engine"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/hive/simulators/ethereum/engine/clmock"
	"github.com/ethereum/hive/simulators/ethereum/engine/config"
	"github.com/ethereum/hive/simulators/ethereum/engine/globals"
	"github.com/ethereum/hive/simulators/ethereum/engine/helper"
	"github.com/ethereum/hive/simulators/ethereum/engine/test"
	typ "github.com/ethereum/hive/simulators/ethereum/engine/types"
)

type ForkchoiceStateField string

const (
	HeadBlockHash      ForkchoiceStateField = "Head"
	SafeBlockHash      ForkchoiceStateField = "Safe"
	FinalizedBlockHash ForkchoiceStateField = "Finalized"
)

type InconsistentForkchoiceTest struct {
	test.BaseSpec
	Field ForkchoiceStateField
}

func (s InconsistentForkchoiceTest) WithMainFork(fork config.Fork) test.Spec {
	specCopy := s
	specCopy.MainFork = fork
	return specCopy
}

func (tc InconsistentForkchoiceTest) GetName() string {
	return fmt.Sprintf("Inconsistent %s in ForkchoiceState", tc.Field)
}

// Send an inconsistent ForkchoiceState with a known payload that belongs to a side chain as head, safe or finalized.
func (tc InconsistentForkchoiceTest) Execute(t *test.Env) {
	canonicalPayloads := make([]*typ.ExecutableData, 0)
	alternativePayloads := make([]*typ.ExecutableData, 0)
	// Produce blocks before starting the test
	t.CLMock.ProduceBlocks(3, clmock.BlockProcessCallbacks{
		OnGetPayload: func() {
			// Generate and send an alternative side chain
			customData := helper.CustomPayloadData{}
			customData.ExtraData = &([]byte{0x01})
			if len(alternativePayloads) > 0 {
				customData.ParentHash = &alternativePayloads[len(alternativePayloads)-1].BlockHash
			}
			alternativePayload, err := customData.CustomizePayload(t.Rand, &t.CLMock.LatestPayloadBuilt)
			if err != nil {
				t.Fatalf("FAIL (%s): Unable to construct alternative payload: %v", t.TestName, err)
			}
			alternativePayloads = append(alternativePayloads, alternativePayload)
			latestCanonicalPayload := t.CLMock.LatestPayloadBuilt
			canonicalPayloads = append(canonicalPayloads, &latestCanonicalPayload)

			// Send the alternative payload
			r := t.TestEngine.TestEngineNewPayload(alternativePayload)
			r.ExpectStatusEither(test.Valid, test.Accepted)
		},
	})
	// Send the invalid ForkchoiceStates
	inconsistentFcU := api.ForkchoiceStateV1{
		HeadBlockHash:      canonicalPayloads[len(alternativePayloads)-1].BlockHash,
		SafeBlockHash:      canonicalPayloads[len(alternativePayloads)-2].BlockHash,
		FinalizedBlockHash: canonicalPayloads[len(alternativePayloads)-3].BlockHash,
	}
	switch tc.Field {
	case HeadBlockHash:
		inconsistentFcU.HeadBlockHash = alternativePayloads[len(alternativePayloads)-1].BlockHash
	case SafeBlockHash:
		inconsistentFcU.SafeBlockHash = alternativePayloads[len(canonicalPayloads)-2].BlockHash
	case FinalizedBlockHash:
		inconsistentFcU.FinalizedBlockHash = alternativePayloads[len(canonicalPayloads)-3].BlockHash
	}
	r := t.TestEngine.TestEngineForkchoiceUpdated(&inconsistentFcU, nil, t.CLMock.LatestPayloadBuilt.Timestamp)
	r.ExpectErrorCode(*globals.INVALID_FORKCHOICE_STATE)

	// Return to the canonical chain
	r = t.TestEngine.TestEngineForkchoiceUpdated(&t.CLMock.LatestForkchoice, nil, t.CLMock.LatestPayloadBuilt.Timestamp)
	r.ExpectPayloadStatus(test.Valid)
}

type ForkchoiceUpdatedUnknownBlockHashTest struct {
	test.BaseSpec
	Field ForkchoiceStateField
}

func (s ForkchoiceUpdatedUnknownBlockHashTest) WithMainFork(fork config.Fork) test.Spec {
	specCopy := s
	specCopy.MainFork = fork
	return specCopy
}

func (tc ForkchoiceUpdatedUnknownBlockHashTest) GetName() string {
	return fmt.Sprintf("Unknown %sBlockHash", tc.Field)
}

// Send an inconsistent ForkchoiceState with a known payload that belongs to a side chain as head, safe or finalized.
func (tc ForkchoiceUpdatedUnknownBlockHashTest) Execute(t *test.Env) {
	// Produce blocks before starting the test
	t.CLMock.ProduceBlocks(5, clmock.BlockProcessCallbacks{})

	// Generate a random block hash
	randomBlockHash := common.Hash{}
	t.Rand.Read(randomBlockHash[:])

	if tc.Field == HeadBlockHash {

		forkchoiceStateUnknownHeadHash := api.ForkchoiceStateV1{
			HeadBlockHash:      randomBlockHash,
			SafeBlockHash:      t.CLMock.LatestForkchoice.SafeBlockHash,
			FinalizedBlockHash: t.CLMock.LatestForkchoice.FinalizedBlockHash,
		}

		t.Logf("INFO (%v) forkchoiceStateUnknownHeadHash: %v\n", t.TestName, forkchoiceStateUnknownHeadHash)

		// Execution specification::
		// - {payloadStatus: {status: SYNCING, latestValidHash: null, validationError: null}, payloadId: null}
		//   if forkchoiceState.headBlockHash references an unknown payload or a payload that can't be validated
		//   because requisite data for the validation is missing
		r := t.TestEngine.TestEngineForkchoiceUpdated(&forkchoiceStateUnknownHeadHash, nil, t.CLMock.LatestExecutedPayload.Timestamp)
		r.ExpectPayloadStatus(test.Syncing)

		payloadAttributes := t.CLMock.LatestPayloadAttributes
		payloadAttributes.Timestamp += 1

		// Test again using PayloadAttributes, should also return SYNCING and no PayloadID
		r = t.TestEngine.TestEngineForkchoiceUpdated(&forkchoiceStateUnknownHeadHash,
			&payloadAttributes, t.CLMock.LatestExecutedPayload.Timestamp)
		r.ExpectPayloadStatus(test.Syncing)
		r.ExpectPayloadID(nil)
	} else {
		t.CLMock.ProduceSingleBlock(clmock.BlockProcessCallbacks{
			// Run test after a new payload has been broadcast
			OnNewPayloadBroadcast: func() {

				forkchoiceStateRandomHash := api.ForkchoiceStateV1{
					HeadBlockHash:      t.CLMock.LatestExecutedPayload.BlockHash,
					SafeBlockHash:      t.CLMock.LatestForkchoice.SafeBlockHash,
					FinalizedBlockHash: t.CLMock.LatestForkchoice.FinalizedBlockHash,
				}

				if tc.Field == SafeBlockHash {
					forkchoiceStateRandomHash.SafeBlockHash = randomBlockHash
				} else if tc.Field == FinalizedBlockHash {
					forkchoiceStateRandomHash.FinalizedBlockHash = randomBlockHash
				}

				r := t.TestEngine.TestEngineForkchoiceUpdated(&forkchoiceStateRandomHash, nil, t.CLMock.LatestExecutedPayload.Timestamp)
				r.ExpectError()

				payloadAttributes := t.CLMock.LatestPayloadAttributes
				payloadAttributes.Random = common.Hash{}
				payloadAttributes.SuggestedFeeRecipient = common.Address{}

				// Test again using PayloadAttributes, should also return INVALID and no PayloadID
				r = t.TestEngine.TestEngineForkchoiceUpdated(&forkchoiceStateRandomHash,
					&payloadAttributes, t.CLMock.LatestExecutedPayload.Timestamp)
				r.ExpectError()

			},
		})
	}
}
