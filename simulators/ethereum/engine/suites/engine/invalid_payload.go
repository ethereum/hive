package suite_engine

import (
	"fmt"

	api "github.com/ethereum/go-ethereum/beacon/engine"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/hive/simulators/ethereum/engine/client/hive_rpc"
	"github.com/ethereum/hive/simulators/ethereum/engine/clmock"
	"github.com/ethereum/hive/simulators/ethereum/engine/config"
	"github.com/ethereum/hive/simulators/ethereum/engine/globals"
	"github.com/ethereum/hive/simulators/ethereum/engine/helper"
	"github.com/ethereum/hive/simulators/ethereum/engine/test"
	typ "github.com/ethereum/hive/simulators/ethereum/engine/types"
)

// Generate test cases for each field of NewPayload, where the payload contains a single invalid field and a valid hash.
type InvalidPayloadTestCase struct {
	test.BaseSpec
	InvalidField      helper.InvalidPayloadBlockField
	Syncing           bool
	EmptyTransactions bool
}

func (s InvalidPayloadTestCase) WithMainFork(fork config.Fork) test.Spec {
	specCopy := s
	specCopy.MainFork = fork
	return specCopy
}

func (i InvalidPayloadTestCase) GetName() string {
	name := fmt.Sprintf("Invalid %s NewPayload", i.InvalidField)
	if i.Syncing {
		name += " - Syncing"
	}
	if i.EmptyTransactions {
		name += " - Empty Transactions"
	}
	if i.BaseSpec.TestTransactionType == helper.DynamicFeeTxOnly {
		name += fmt.Sprintf(" - %s", i.BaseSpec.TestTransactionType)
	}
	return name
}

func (tc InvalidPayloadTestCase) Execute(t *test.Env) {
	if tc.Syncing {
		// To allow sending the primary engine client into SYNCING state, we need a secondary client to guide the payload creation
		secondaryClient, err := hive_rpc.HiveRPCEngineStarter{}.StartClient(t.T, t.TestContext, t.Genesis, t.ClientParams, t.ClientFiles)
		if err != nil {
			t.Fatalf("FAIL (%s): Unable to spawn a secondary client: %v", t.TestName, err)
		}
		t.CLMock.AddEngineClient(secondaryClient)
	}

	// Wait until TTD is reached by all clients
	t.CLMock.WaitForTTD()

	txFunc := func() {
		if !tc.EmptyTransactions {
			// Function to send at least one transaction each block produced
			// Send the transaction to the globals.PrevRandaoContractAddr
			_, err := helper.SendNextTransaction(
				t.TestContext,
				t.CLMock.NextBlockProducer,
				&helper.BaseTransactionCreator{
					Recipient:  &globals.PrevRandaoContractAddr,
					Amount:     big1,
					Payload:    nil,
					TxType:     t.TestTransactionType,
					GasLimit:   75000,
					ForkConfig: t.ForkConfig,
				},
			)
			if err != nil {
				t.Fatalf("FAIL (%s): Error trying to send transaction: %v", t.TestName, err)
			}
		}
	}

	// Produce blocks before starting the test
	t.CLMock.ProduceBlocks(5, clmock.BlockProcessCallbacks{
		// Make sure at least one transaction is included in each block
		OnPayloadProducerSelected: txFunc,
	})

	if tc.Syncing {
		// Disconnect the main engine client from the CL Mocker and produce a block
		t.CLMock.RemoveEngineClient(t.Engine)
		t.CLMock.ProduceSingleBlock(clmock.BlockProcessCallbacks{
			OnPayloadProducerSelected: txFunc,
		})

		// This block is now unknown to the main client, sending an fcU will set it to tc.Syncing mode
		r := t.TestEngine.TestEngineForkchoiceUpdated(&t.CLMock.LatestForkchoice, nil, t.CLMock.LatestPayloadBuilt.Timestamp)
		r.ExpectPayloadStatus(test.Syncing)
	}

	var (
		alteredPayload *typ.ExecutableData
		err            error
	)

	t.CLMock.ProduceSingleBlock(clmock.BlockProcessCallbacks{
		// Make sure at least one transaction is included in the payload
		OnPayloadProducerSelected: txFunc,
		// Run test after the new payload has been obtained
		OnGetPayload: func() {
			// Alter the payload while maintaining a valid hash and send it to the client, should produce an error

			// We need at least one transaction for most test cases to work
			if !tc.EmptyTransactions && len(t.CLMock.LatestPayloadBuilt.Transactions) == 0 {
				// But if the payload has no transactions, the test is invalid
				t.Fatalf("FAIL (%s): No transactions in the base payload", t.TestName)
			}

			alteredPayload, err = helper.GenerateInvalidPayload(&t.CLMock.LatestPayloadBuilt, tc.InvalidField)
			if err != nil {
				t.Fatalf("FAIL (%s): Unable to modify payload (%v): %v", t.TestName, tc.InvalidField, err)
			}

			// Depending on the field we modified, we expect a different status
			r := t.TestEngine.TestEngineNewPayload(alteredPayload)
			if tc.Syncing || tc.InvalidField == helper.InvalidParentHash {
				// Execution specification::
				// {status: ACCEPTED, latestValidHash: null, validationError: null} if the following conditions are met:
				//  - the blockHash of the payload is valid
				//  - the payload doesn't extend the canonical chain
				//  - the payload hasn't been fully validated
				// {status: SYNCING, latestValidHash: null, validationError: null}
				// if the payload extends the canonical chain and requisite data for its validation is missing
				// (the client can assume the payload extends the canonical because the linking payload could be missing)
				r.ExpectStatusEither(test.Accepted, test.Syncing)
				r.ExpectLatestValidHash(nil)
			} else {
				r.ExpectStatus(test.Invalid)
				r.ExpectLatestValidHash(&alteredPayload.ParentHash)
			}

			// Send the forkchoiceUpdated with a reference to the invalid payload.
			fcState := api.ForkchoiceStateV1{
				HeadBlockHash:      alteredPayload.BlockHash,
				SafeBlockHash:      alteredPayload.BlockHash,
				FinalizedBlockHash: alteredPayload.BlockHash,
			}
			payloadAttrbutes := t.CLMock.LatestPayloadAttributes
			payloadAttrbutes.Timestamp = alteredPayload.Timestamp + 1
			payloadAttrbutes.Random = common.Hash{}
			payloadAttrbutes.SuggestedFeeRecipient = common.Address{}

			// Execution specification:
			//  {payloadStatus: {status: INVALID, latestValidHash: null, validationError: errorMessage | null}, payloadId: null}
			//  obtained from the Payload validation process if the payload is deemed INVALID
			s := t.TestEngine.TestEngineForkchoiceUpdated(&fcState, &payloadAttrbutes, alteredPayload.Timestamp)
			if !tc.Syncing {
				// Execution specification:
				//  {payloadStatus: {status: INVALID, latestValidHash: null, validationError: errorMessage | null}, payloadId: null}
				//  obtained from the Payload validation process if the payload is deemed INVALID
				// Note: SYNCING/ACCEPTED is acceptable here as long as the block produced after this test is produced successfully
				s.ExpectAnyPayloadStatus(test.Syncing, test.Accepted, test.Invalid)
			} else {
				// At this moment the response should be SYNCING
				s.ExpectPayloadStatus(test.Syncing)

				// When we send the previous payload, the client must now be capable of determining that the invalid payload is actually invalid
				p := t.TestEngine.TestEngineNewPayload(&t.CLMock.LatestExecutedPayload)
				p.ExpectStatus(test.Valid)
				p.ExpectLatestValidHash(&t.CLMock.LatestExecutedPayload.BlockHash)

				/*
					// Another option here could be to send an fcU to the previous payload,
					// but this does not seem like something the CL would do.
					s := t.TestEngine.TestEngineForkchoiceUpdated(&api.ForkchoiceStateV1{
						HeadBlockHash:      previousPayload.BlockHash,
						SafeBlockHash:      previousPayload.BlockHash,
						FinalizedBlockHash: previousPayload.BlockHash,
					}, nil)
					s.ExpectPayloadStatus(Valid)
				*/

				q := t.TestEngine.TestEngineNewPayload(alteredPayload)
				if tc.InvalidField == helper.InvalidParentHash {
					// There is no invalid parentHash, if this value is incorrect,
					// it is assumed that the block is missing and we need to sync.
					// ACCEPTED also valid since the CLs normally use these interchangeably
					q.ExpectStatusEither(test.Syncing, test.Accepted)
					q.ExpectLatestValidHash(nil)
				} else if tc.InvalidField == helper.InvalidNumber {
					// A payload with an invalid number can force us to start a sync cycle
					// as we don't know if that block might be a valid future block.
					q.ExpectStatusEither(test.Invalid, test.Syncing)
					if q.Status.Status == test.Invalid {
						q.ExpectLatestValidHash(&t.CLMock.LatestExecutedPayload.BlockHash)
					} else {
						q.ExpectLatestValidHash(nil)
					}
				} else {
					// Otherwise the response should be INVALID.
					q.ExpectStatus(test.Invalid)
					q.ExpectLatestValidHash(&t.CLMock.LatestExecutedPayload.BlockHash)
				}

				// Try sending the fcU again, this time we should get the proper invalid response.
				// At this moment the response should be INVALID
				if tc.InvalidField != helper.InvalidParentHash {
					s := t.TestEngine.TestEngineForkchoiceUpdated(&fcState, nil, alteredPayload.Timestamp)
					// Note: SYNCING is acceptable here as long as the block produced after this test is produced successfully
					s.ExpectAnyPayloadStatus(test.Syncing, test.Invalid)
				}
			}

			// Finally, attempt to fetch the invalid payload using the JSON-RPC endpoint
			p := t.TestEngine.TestBlockByHash(alteredPayload.BlockHash)
			p.ExpectError()
		},
	})

	if tc.Syncing {
		// Send the valid payload and its corresponding forkchoiceUpdated
		r := t.TestEngine.TestEngineNewPayload(&t.CLMock.LatestExecutedPayload)
		r.ExpectStatus(test.Valid)
		r.ExpectLatestValidHash(&t.CLMock.LatestExecutedPayload.BlockHash)

		s := t.TestEngine.TestEngineForkchoiceUpdated(&t.CLMock.LatestForkchoice, nil, t.CLMock.LatestExecutedPayload.Timestamp)
		s.ExpectPayloadStatus(test.Valid)
		s.ExpectLatestValidHash(&t.CLMock.LatestExecutedPayload.BlockHash)

		// Add main client again to the CL Mocker
		t.CLMock.AddEngineClient(t.Engine)
	}

	// Lastly, attempt to build on top of the invalid payload
	t.CLMock.ProduceSingleBlock(clmock.BlockProcessCallbacks{
		// Run test after the new payload has been obtained
		OnGetPayload: func() {
			followUpAlteredPayload, err := (&helper.CustomPayloadData{
				ParentHash: &alteredPayload.BlockHash,
			}).CustomizePayload(&t.CLMock.LatestPayloadBuilt)
			if err != nil {
				t.Fatalf("FAIL (%s): Unable to modify payload: %v", t.TestName, err)
			}

			t.Logf("INFO (%s): Sending customized NewPayload: ParentHash %v -> %v", t.TestName, t.CLMock.LatestPayloadBuilt.ParentHash, alteredPayload.BlockHash)
			// Response status can be ACCEPTED (since parent payload could have been thrown out by the client)
			// or SYNCING (parent payload is thrown out and also client assumes that the parent is part of canonical chain)
			// or INVALID (client still has the payload and can verify that this payload is incorrectly building on top of it),
			// but a VALID response is incorrect.
			r := t.TestEngine.TestEngineNewPayload(followUpAlteredPayload)
			r.ExpectStatusEither(test.Accepted, test.Invalid, test.Syncing)
			if r.Status.Status == test.Accepted || r.Status.Status == test.Syncing {
				r.ExpectLatestValidHash(nil)
			} else if r.Status.Status == test.Invalid {
				r.ExpectLatestValidHash(&alteredPayload.ParentHash)
			}
		},
	})
}
