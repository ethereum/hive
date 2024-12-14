package suite_engine

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"time"

	api "github.com/ethereum/go-ethereum/beacon/engine"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
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
	// InvalidField is the field that will be modified to create an invalid payload
	InvalidField helper.InvalidPayloadBlockField
	// Syncing is true if the client is expected to be in SYNCING mode after receiving the invalid payload
	Syncing bool
	// EmptyTransactions is true if the payload should not contain any transactions
	EmptyTransactions bool
	// If true, the payload can be detected to be invalid even when syncing,
	// but this check is optional and both `INVALID` and `SYNCING` are valid responses.
	InvalidDetectedOnSync bool
	// If true, latest valid hash can be nil for this test.
	NilLatestValidHash bool
}

func (s InvalidPayloadTestCase) WithMainFork(fork config.Fork) test.Spec {
	specCopy := s
	specCopy.MainFork = fork
	return specCopy
}

func (i InvalidPayloadTestCase) GetName() string {
	syncStatus := "False"
	if i.Syncing {
		syncStatus = "True"
	}
	emptyTxsStatus := "False"
	if i.EmptyTransactions {
		emptyTxsStatus = "True"
	}
	dynFeeTxsStatus := "False"
	if i.BaseSpec.TestTransactionType == helper.DynamicFeeTxOnly {
		dynFeeTxsStatus = "True"
	}
	return fmt.Sprintf(
		"Invalid NewPayload, %s, Syncing=%s, EmptyTxs=%s, DynFeeTxs=%s",
		i.InvalidField,
		syncStatus,
		emptyTxsStatus,
		dynFeeTxsStatus,
	)
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

	txFunc := func() {
		if !tc.EmptyTransactions {
			// Function to send at least one transaction each block produced
			// Send the transaction to the globals.PrevRandaoContractAddr
			_, err := t.SendNextTransaction(
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
		alteredPayload        *typ.ExecutableData
		invalidDetectedOnSync bool = tc.InvalidDetectedOnSync
		nilLatestValidHash    bool = tc.NilLatestValidHash
		err                   error
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

			alteredPayload, err = helper.GenerateInvalidPayload(t.Rand, &t.CLMock.LatestPayloadBuilt, tc.InvalidField)
			if err != nil {
				t.Fatalf("FAIL (%s): Unable to modify payload (%v): %v", t.TestName, tc.InvalidField, err)
			}

			if t.CLMock.LatestPayloadBuilt.VersionedHashes != nil &&
				len(*t.CLMock.LatestPayloadBuilt.VersionedHashes) > 0 &&
				tc.InvalidField == helper.RemoveTransaction {
				// If the payload has versioned hashes, and we removed any transaction, it's highly likely the client will
				// be able to detect the invalid payload even when syncing because of the blob gas used.
				invalidDetectedOnSync = true
				nilLatestValidHash = true
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
				if invalidDetectedOnSync {
					// For some fields, the client can detect the invalid payload even when it doesn't have the parent.
					// However this behavior is up to the client, so we can't expect it to happen and syncing is also valid.
					// `VALID` response is still incorrect though.
					r.ExpectStatusEither(test.Invalid, test.Accepted, test.Syncing)
					// TODO: It seems like latestValidHash==nil should always be expected here.
				} else {
					r.ExpectStatusEither(test.Accepted, test.Syncing)
					r.ExpectLatestValidHash(nil)
				}
			} else {
				r.ExpectStatus(test.Invalid)
				if !(nilLatestValidHash && r.Status.LatestValidHash == nil) {
					r.ExpectLatestValidHash(&alteredPayload.ParentHash)
				}
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
				if invalidDetectedOnSync {
					// except if the client can detect the invalid payload even when syncing
					s.ExpectAnyPayloadStatus(test.Invalid, test.Accepted, test.Syncing)
				} else {
					s.ExpectPayloadStatus(test.Syncing)
				}

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
					if !(nilLatestValidHash && r.Status.LatestValidHash == nil) {
						q.ExpectLatestValidHash(&t.CLMock.LatestExecutedPayload.BlockHash)
					}
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
			p := t.TestEngine.TestHeaderByHash(alteredPayload.BlockHash)
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
			if t.CLMock.LatestPayloadBuilt.ParentHash == alteredPayload.BlockHash {
				// In some instances the payload is indiscernible from the altered one because the
				// difference lies in the new payload parameters, in this case skip this check.
				return
			}
			followUpAlteredPayload, err := (&helper.CustomPayloadData{
				ParentHash: &alteredPayload.BlockHash,
			}).CustomizePayload(t.Rand, &t.CLMock.LatestPayloadBuilt)
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
				if !(nilLatestValidHash && r.Status.LatestValidHash == nil) {
					r.ExpectLatestValidHash(&alteredPayload.ParentHash)
				}
			}
		},
	})
}

// Build on top of the latest valid payload after an invalid payload had been received:
// P <- INV_P, newPayload(INV_P), fcU(head: P, payloadAttributes: attrs) + getPayload(…)
type PayloadBuildAfterInvalidPayloadTest struct {
	test.BaseSpec
	InvalidField helper.InvalidPayloadBlockField
}

func (s PayloadBuildAfterInvalidPayloadTest) WithMainFork(fork config.Fork) test.Spec {
	specCopy := s
	specCopy.MainFork = fork
	return specCopy
}

func (i PayloadBuildAfterInvalidPayloadTest) GetName() string {
	name := fmt.Sprintf("Payload Build after New Invalid Payload: Invalid %s", i.InvalidField)
	return name
}

func (tc PayloadBuildAfterInvalidPayloadTest) Execute(t *test.Env) {
	// Add a second client to build the invalid payload
	secondaryEngine, err := hive_rpc.HiveRPCEngineStarter{}.StartClient(t.T, t.TestContext, t.Genesis, t.ClientParams, t.ClientFiles)

	if err != nil {
		t.Fatalf("FAIL (%s): Unable to spawn a secondary client: %v", t.TestName, err)
	}
	secondaryEngineTest := test.NewTestEngineClient(t, secondaryEngine)
	t.CLMock.AddEngineClient(secondaryEngine)

	// Produce blocks before starting the test
	t.CLMock.ProduceBlocks(5, clmock.BlockProcessCallbacks{})

	// Produce another block, but at the same time send an invalid payload from the other client
	t.CLMock.ProduceSingleBlock(clmock.BlockProcessCallbacks{
		OnPayloadAttributesGenerated: func() {
			// We are going to use the client that was not selected
			// by the CLMocker to produce the invalid payload
			invalidPayloadProducer := t.TestEngine
			if t.CLMock.NextBlockProducer == invalidPayloadProducer.Engine {
				invalidPayloadProducer = secondaryEngineTest
			}
			var inv_p *typ.ExecutableData

			{
				// Get a payload from the invalid payload producer and invalidate it
				var (
					prevRandao            = common.Hash{}
					suggestedFeeRecipient = common.Address{}
				)
				payloadAttributes, err := (&helper.BasePayloadAttributesCustomizer{
					Random:                &prevRandao,
					SuggestedFeeRecipient: &suggestedFeeRecipient,
				}).GetPayloadAttributes(&t.CLMock.LatestPayloadAttributes)
				if err != nil {
					t.Fatalf("FAIL (%s): Unable to customize payload attributes: %v", t.TestName, err)
				}

				r := invalidPayloadProducer.TestEngineForkchoiceUpdated(&t.CLMock.LatestForkchoice, payloadAttributes, t.CLMock.LatestHeader.Time)
				r.ExpectPayloadStatus(test.Valid)
				// Wait for the payload to be produced by the EL
				time.Sleep(time.Second)

				s := invalidPayloadProducer.TestEngineGetPayload(
					r.Response.PayloadID,
					payloadAttributes,
				)
				s.ExpectNoError()

				inv_p, err = helper.GenerateInvalidPayload(t.Rand, &s.Payload, helper.InvalidStateRoot)
				if err != nil {
					t.Fatalf("FAIL (%s): Unable to invalidate payload: %v", t.TestName, err)
				}
			}

			// Broadcast the invalid payload
			r := t.TestEngine.TestEngineNewPayload(inv_p)
			r.ExpectStatus(test.Invalid)
			r.ExpectLatestValidHash(&t.CLMock.LatestForkchoice.HeadBlockHash)
			s := secondaryEngineTest.TestEngineNewPayload(inv_p)
			s.ExpectStatus(test.Invalid)
			s.ExpectLatestValidHash(&t.CLMock.LatestForkchoice.HeadBlockHash)

			// Let the block production continue.
			// At this point the selected payload producer will
			// try to continue creating a valid payload.
		},
	})
}

type InvalidTxChainIDTest struct {
	test.BaseSpec
}

func (s InvalidTxChainIDTest) WithMainFork(fork config.Fork) test.Spec {
	specCopy := s
	specCopy.MainFork = fork
	return specCopy
}

func (s InvalidTxChainIDTest) GetName() string {
	name := fmt.Sprintf("Build Payload with Invalid ChainID Transaction, %s", s.TestTransactionType)
	return name
}

// Attempt to produce a payload after a transaction with an invalid Chain ID was sent to the client
// using `eth_sendRawTransaction`.
func (spec InvalidTxChainIDTest) Execute(t *test.Env) {
	// Produce blocks before starting the test
	t.CLMock.ProduceBlocks(5, clmock.BlockProcessCallbacks{})

	// Send a transaction with an incorrect ChainID.
	// Transaction must be not be included in payload creation.
	var invalidChainIDTx *types.Transaction
	t.CLMock.ProduceSingleBlock(clmock.BlockProcessCallbacks{
		// Run test after a new payload has been broadcast
		OnPayloadAttributesGenerated: func() {
			txCreator := helper.BaseTransactionCreator{
				Recipient:  &globals.PrevRandaoContractAddr,
				Amount:     big1,
				Payload:    nil,
				TxType:     t.TestTransactionType,
				GasLimit:   75000,
				ForkConfig: t.ForkConfig,
			}
			sender := globals.TestAccounts[0]

			ctx, cancel := context.WithTimeout(t.TestContext, globals.RPCTimeout)
			defer cancel()
			nonce, err := t.CLMock.NextBlockProducer.NonceAt(ctx, sender.GetAddress(), nil)
			if err != nil {
				t.Fatalf("FAIL(%s): Unable to get address nonce: %v", t.TestName, err)
			}

			tx, err := txCreator.MakeTransaction(sender, nonce, t.CLMock.LatestPayloadAttributes.Timestamp)
			if err != nil {
				t.Fatalf("FAIL(%s): Unable to create transaction: %v", t.TestName, err)
			}

			txCast, ok := tx.(*types.Transaction)
			if !ok {
				t.Fatalf("FAIL(%s): Unable to cast transaction to types.Transaction", t.TestName)
			}

			txCustomizerData := &helper.CustomTransactionData{
				ChainID: new(big.Int).Add(globals.ChainID, big1),
			}
			invalidChainIDTx, err = helper.CustomizeTransaction(txCast, sender, txCustomizerData)
			if err != nil {
				t.Fatalf("FAIL(%s): Unable to customize transaction: %v", t.TestName, err)
			}

			ctx, cancel = context.WithTimeout(t.TestContext, globals.RPCTimeout)
			defer cancel()
			err = t.Engine.SendTransaction(ctx, invalidChainIDTx)
			if err != nil {
				t.Logf("INFO (%s): Error on sending transaction with incorrect chain ID (Expected): %v", t.TestName, err)
			}
		},
	})

	// Verify that the latest payload built does NOT contain the invalid chain Tx
	if helper.TransactionInPayload(&t.CLMock.LatestPayloadBuilt, invalidChainIDTx) {
		p, _ := json.MarshalIndent(t.CLMock.LatestPayloadBuilt, "", " ")
		t.Fatalf("FAIL (%s): Invalid chain ID tx was included in payload: %s", t.TestName, p)
	}
}
