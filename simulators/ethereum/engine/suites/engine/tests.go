package suite_engine

import (
	"context"
	"encoding/json"
	"math/big"
	"math/rand"
	"time"

	api "github.com/ethereum/go-ethereum/beacon/engine"
	"github.com/ethereum/hive/simulators/ethereum/engine/client"
	"github.com/ethereum/hive/simulators/ethereum/engine/client/hive_rpc"
	"github.com/ethereum/hive/simulators/ethereum/engine/clmock"
	"github.com/ethereum/hive/simulators/ethereum/engine/config"
	"github.com/ethereum/hive/simulators/ethereum/engine/globals"
	"github.com/ethereum/hive/simulators/ethereum/engine/helper"
	"github.com/ethereum/hive/simulators/ethereum/engine/test"
	typ "github.com/ethereum/hive/simulators/ethereum/engine/types"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

// Execution specification reference:
// https://github.com/ethereum/execution-apis/blob/main/src/engine/specification.md

var (
	big0      = new(big.Int)
	big1      = big.NewInt(1)
	Head      *big.Int // Nil
	Pending   = big.NewInt(-2)
	Finalized = big.NewInt(-3)
	Safe      = big.NewInt(-4)
	ZeroAddr  = common.Address{}
)

var Tests = []test.Spec{

	&test.BaseSpec{
		Name: "latest Block after Reorg",
		Run:  blockStatusReorg,
	},

	// Payload Tests
	&test.BaseSpec{
		Name: "Re-Execute Payload",
		Run:  reExecPayloads,
	},
	&test.BaseSpec{
		Name: "Multiple New Payloads Extending Canonical Chain",
		Run:  multipleNewCanonicalPayloads,
	},
	&test.BaseSpec{
		Name: "Consecutive Payload Execution",
		Run:  inOrderPayloads,
	},
	&test.BaseSpec{
		Name: "Valid NewPayload->ForkchoiceUpdated on Syncing Client",
		Run:  validPayloadFcUSyncingClient,
	},
	&test.BaseSpec{
		Name: "NewPayload with Missing ForkchoiceUpdated",
		Run:  missingFcu,
	},
	&test.BaseSpec{
		Name:                "Build Payload with Invalid ChainID Transaction (Legacy Tx)",
		Run:                 buildPayloadWithInvalidChainIDTx,
		TestTransactionType: helper.LegacyTxOnly,
	},
	&test.BaseSpec{
		Name:                "Build Payload with Invalid ChainID Transaction (EIP-1559)",
		Run:                 buildPayloadWithInvalidChainIDTx,
		TestTransactionType: helper.DynamicFeeTxOnly,
	},

	// Re-org using Engine API

	&test.BaseSpec{
		Name: "Sidechain Reorg",
		Run:  sidechainReorg,
	},
	&test.BaseSpec{
		Name: "Re-Org Back to Canonical Chain From Syncing Chain",
		Run:  reorgBackFromSyncing,
	},
	&test.BaseSpec{
		Name:             "Import and re-org to previously validated payload on a side chain",
		SlotsToSafe:      big.NewInt(15),
		SlotsToFinalized: big.NewInt(20),
		Run:              reorgPrevValidatedPayloadOnSideChain,
	},
	&test.BaseSpec{
		Name:             "Safe Re-Org to Side Chain",
		Run:              safeReorgToSideChain,
		SlotsToSafe:      big.NewInt(1),
		SlotsToFinalized: big.NewInt(2),
	},

	// Suggested Fee Recipient in Payload creation
	&test.BaseSpec{
		Name:                "Suggested Fee Recipient Test",
		Run:                 suggestedFeeRecipient,
		TestTransactionType: helper.LegacyTxOnly,
	},
	&test.BaseSpec{
		Name:                "Suggested Fee Recipient Test (EIP-1559 Transactions)",
		Run:                 suggestedFeeRecipient,
		TestTransactionType: helper.DynamicFeeTxOnly,
	},

	// PrevRandao opcode tests
	&test.BaseSpec{
		Name:                "PrevRandao Opcode Transactions",
		Run:                 prevRandaoOpcodeTx,
		TestTransactionType: helper.LegacyTxOnly,
	},
	&test.BaseSpec{
		Name:                "PrevRandao Opcode Transactions (EIP-1559 Transactions)",
		Run:                 prevRandaoOpcodeTx,
		TestTransactionType: helper.DynamicFeeTxOnly,
	},
}

// Test to verify Block information available after a reorg using forkchoiceUpdated
func blockStatusReorg(t *test.Env) {
	// Wait until this client catches up with latest PoS Block
	t.CLMock.WaitForTTD()

	// Produce blocks before starting the test
	t.CLMock.ProduceBlocks(5, clmock.BlockProcessCallbacks{})

	t.CLMock.ProduceSingleBlock(clmock.BlockProcessCallbacks{
		OnGetPayload: func() {
			// Run using an alternative Payload, verify that the latest info is updated after re-org
			customRandom := common.Hash{}
			rand.Read(customRandom[:])
			customizer := &helper.CustomPayloadData{
				PrevRandao: &customRandom,
			}
			customizedPayload, err := customizer.CustomizePayload(&t.CLMock.LatestPayloadBuilt)
			if err != nil {
				t.Fatalf("FAIL (%s): Unable to customize payload: %v", t.TestName, err)
			}

			// Send custom payload and fcU to it
			t.CLMock.BroadcastNewPayload(customizedPayload, t.ForkConfig.NewPayloadVersion(customizedPayload.Timestamp))
			t.CLMock.BroadcastForkchoiceUpdated(&api.ForkchoiceStateV1{
				HeadBlockHash:      customizedPayload.BlockHash,
				SafeBlockHash:      t.CLMock.LatestForkchoice.SafeBlockHash,
				FinalizedBlockHash: t.CLMock.LatestForkchoice.FinalizedBlockHash,
			}, nil, t.ForkConfig.NewPayloadVersion(customizedPayload.Timestamp))

			// Verify the client is serving the latest HeadBlock
			r := t.TestEngine.TestHeaderByNumber(Head)
			r.ExpectHash(customizedPayload.BlockHash)

		},
		OnForkchoiceBroadcast: func() {
			// At this point, we have re-org'd to the payload that the CLMocker was originally planning to send,
			// verify that the client is serving the latest HeadBlock.
			r := t.TestEngine.TestHeaderByNumber(Head)
			r.ExpectHash(t.CLMock.LatestForkchoice.HeadBlockHash)

		},
	})

}

// Test that performs a re-org to a previously validated payload on a side chain.
func reorgPrevValidatedPayloadOnSideChain(t *test.Env) {
	// Wait until this client catches up with latest PoS
	t.CLMock.WaitForTTD()

	// Produce blocks before starting the test
	t.CLMock.ProduceBlocks(5, clmock.BlockProcessCallbacks{})

	var (
		sidechainPayloads     = make([]*typ.ExecutableData, 0)
		sidechainPayloadCount = 5
	)

	// Produce a canonical chain while at the same time generate a side chain to which we will re-org.
	t.CLMock.ProduceBlocks(sidechainPayloadCount, clmock.BlockProcessCallbacks{
		OnGetPayload: func() {
			// The side chain will consist simply of the same payloads with extra data appended
			extraData := []byte("side")
			customData := helper.CustomPayloadData{
				ExtraData: &extraData,
			}
			if len(sidechainPayloads) > 0 {
				customData.ParentHash = &sidechainPayloads[len(sidechainPayloads)-1].BlockHash
			}
			altPayload, err := customData.CustomizePayload(&t.CLMock.LatestPayloadBuilt)
			if err != nil {
				t.Fatalf("FAIL (%s): Unable to customize payload: %v", t.TestName, err)
			}
			sidechainPayloads = append(sidechainPayloads, altPayload)

			r := t.TestEngine.TestEngineNewPayload(altPayload)
			r.ExpectStatus(test.Valid)
			r.ExpectLatestValidHash(&altPayload.BlockHash)
		},
	})

	// Attempt to re-org to one of the sidechain payloads, but not the leaf,
	// and also build a new payload from this sidechain.
	t.CLMock.ProduceSingleBlock(clmock.BlockProcessCallbacks{
		OnGetPayload: func() {
			var (
				prevRandao            = common.Hash{}
				suggestedFeeRecipient = common.Address{0x12, 0x34}
			)
			rand.Read(prevRandao[:])
			payloadAttributesCustomizer := &helper.BasePayloadAttributesCustomizer{
				Random:                &prevRandao,
				SuggestedFeeRecipient: &suggestedFeeRecipient,
			}

			reOrgPayload := sidechainPayloads[len(sidechainPayloads)-2]
			reOrgPayloadAttributes := sidechainPayloads[len(sidechainPayloads)-1].PayloadAttributes

			payloadAttributesJson, _ := json.MarshalIndent(reOrgPayloadAttributes, "", "  ")
			t.Logf("DEBUG: reOrgPayloadAttributes: %v\n", string(payloadAttributesJson))

			newPayloadAttributes, err := payloadAttributesCustomizer.GetPayloadAttributes(&reOrgPayloadAttributes)
			if err != nil {
				t.Fatalf("FAIL (%s): Unable to customize payload attributes: %v", t.TestName, err)
			}

			r := t.TestEngine.TestEngineForkchoiceUpdated(&api.ForkchoiceStateV1{
				HeadBlockHash:      reOrgPayload.BlockHash,
				SafeBlockHash:      t.CLMock.LatestForkchoice.SafeBlockHash,
				FinalizedBlockHash: t.CLMock.LatestForkchoice.FinalizedBlockHash,
			}, newPayloadAttributes, reOrgPayload.Timestamp)
			r.ExpectPayloadStatus(test.Valid)
			r.ExpectLatestValidHash(&reOrgPayload.BlockHash)

			p := t.TestEngine.TestEngineGetPayload(r.Response.PayloadID, newPayloadAttributes)
			p.ExpectPayloadParentHash(reOrgPayload.BlockHash)

			s := t.TestEngine.TestEngineNewPayload(&p.Payload)
			s.ExpectStatus(test.Valid)
			s.ExpectLatestValidHash(&p.Payload.BlockHash)

			// After this, the CLMocker will continue and try to re-org to canonical chain once again
			// CLMocker will fail the test if this is not possible, so nothing left to do.
		},
	})
}

// Test that performs a re-org of the safe block to a side chain.
func safeReorgToSideChain(t *test.Env) {
	// Wait until this client catches up with latest PoS
	t.CLMock.WaitForTTD()

	// Produce an alternative chain
	sidechainPayloads := make([]*typ.ExecutableData, 0)

	// Produce three payloads `P1`, `P2`, `P3`, along with the side chain payloads `P2'`, `P3'`
	// First payload is finalized so no alternative payload
	t.CLMock.ProduceSingleBlock(clmock.BlockProcessCallbacks{})
	t.CLMock.ProduceBlocks(2, clmock.BlockProcessCallbacks{
		OnGetPayload: func() {
			// Generate an alternative payload by simply adding extraData to the block
			altParentHash := t.CLMock.LatestPayloadBuilt.ParentHash
			if len(sidechainPayloads) > 0 {
				altParentHash = sidechainPayloads[len(sidechainPayloads)-1].BlockHash
			}
			customizer := &helper.CustomPayloadData{
				ParentHash: &altParentHash,
				ExtraData:  &([]byte{0x01}),
			}
			altPayload, err := customizer.CustomizePayload(&t.CLMock.LatestPayloadBuilt)
			if err != nil {
				t.Fatalf("FAIL (%s): Unable to customize payload: %v", t.TestName, err)
			}
			sidechainPayloads = append(sidechainPayloads, altPayload)
		},
	})

	// Verify current state of labels
	head := t.TestEngine.TestHeaderByNumber(Head)
	head.ExpectHash(t.CLMock.LatestPayloadBuilt.BlockHash)

	safe := t.TestEngine.TestHeaderByNumber(Safe)
	safe.ExpectHash(t.CLMock.ExecutedPayloadHistory[2].BlockHash)

	finalized := t.TestEngine.TestHeaderByNumber(Finalized)
	finalized.ExpectHash(t.CLMock.ExecutedPayloadHistory[1].BlockHash)

	// Re-org the safe/head blocks to point to the alternative side chain
	t.CLMock.ProduceSingleBlock(clmock.BlockProcessCallbacks{
		OnGetPayload: func() {
			for _, p := range sidechainPayloads {
				r := t.TestEngine.TestEngineNewPayload(p)
				r.ExpectStatusEither(test.Valid, test.Accepted)
			}
			r := t.TestEngine.TestEngineForkchoiceUpdated(&api.ForkchoiceStateV1{
				HeadBlockHash:      sidechainPayloads[1].BlockHash,
				SafeBlockHash:      sidechainPayloads[0].BlockHash,
				FinalizedBlockHash: t.CLMock.ExecutedPayloadHistory[1].BlockHash,
			}, nil, sidechainPayloads[1].Timestamp)
			r.ExpectPayloadStatus(test.Valid)

			head := t.TestEngine.TestHeaderByNumber(Head)
			head.ExpectHash(sidechainPayloads[1].BlockHash)

			safe := t.TestEngine.TestHeaderByNumber(Safe)
			safe.ExpectHash(sidechainPayloads[0].BlockHash)

			finalized := t.TestEngine.TestHeaderByNumber(Finalized)
			finalized.ExpectHash(t.CLMock.ExecutedPayloadHistory[1].BlockHash)

		},
	})
}

// Test that performs a re-org back to the canonical chain after re-org to syncing/unavailable chain.
func reorgBackFromSyncing(t *test.Env) {
	// Wait until this client catches up with latest PoS
	t.CLMock.WaitForTTD()

	// Produce an alternative chain
	sidechainPayloads := make([]*typ.ExecutableData, 0)
	t.CLMock.ProduceBlocks(10, clmock.BlockProcessCallbacks{
		OnGetPayload: func() {
			// Generate an alternative payload by simply adding extraData to the block
			altParentHash := t.CLMock.LatestPayloadBuilt.ParentHash
			if len(sidechainPayloads) > 0 {
				altParentHash = sidechainPayloads[len(sidechainPayloads)-1].BlockHash
			}
			customizer := &helper.CustomPayloadData{
				ParentHash: &altParentHash,
				ExtraData:  &([]byte{0x01}),
			}
			altPayload, err := customizer.CustomizePayload(&t.CLMock.LatestPayloadBuilt)
			if err != nil {
				t.Fatalf("FAIL (%s): Unable to customize payload: %v", t.TestName, err)
			}
			sidechainPayloads = append(sidechainPayloads, altPayload)
		},
	})

	// Produce blocks before starting the test (So we don't try to reorg back to the genesis block)
	t.CLMock.ProduceSingleBlock(clmock.BlockProcessCallbacks{
		OnGetPayload: func() {
			r := t.TestEngine.TestEngineNewPayload(sidechainPayloads[len(sidechainPayloads)-1])
			r.ExpectStatusEither(test.Syncing, test.Accepted)
			r.ExpectLatestValidHash(nil)
			// We are going to send one of the alternative payloads and fcU to it
			forkchoiceUpdatedBack := api.ForkchoiceStateV1{
				HeadBlockHash:      sidechainPayloads[len(sidechainPayloads)-1].BlockHash,
				SafeBlockHash:      sidechainPayloads[len(sidechainPayloads)-2].BlockHash,
				FinalizedBlockHash: sidechainPayloads[len(sidechainPayloads)-3].BlockHash,
			}

			// It is only expected that the client does not produce an error and the CL Mocker is able to progress after the re-org
			s := t.TestEngine.TestEngineForkchoiceUpdated(&forkchoiceUpdatedBack, nil, sidechainPayloads[len(sidechainPayloads)-1].Timestamp)
			s.ExpectLatestValidHash(nil)
			s.ExpectPayloadStatus(test.Syncing)

			// After this, the CLMocker will continue and try to re-org to canonical chain once again
			// CLMocker will fail the test if this is not possible, so nothing left to do.
		},
	})
}

// Reorg to a Sidechain using ForkchoiceUpdated
func sidechainReorg(t *test.Env) {
	// Wait until this client catches up with latest PoS
	t.CLMock.WaitForTTD()

	// Produce blocks before starting the test
	t.CLMock.ProduceBlocks(5, clmock.BlockProcessCallbacks{})

	// Produce two payloads, send fcU with first payload, check transaction outcome, then reorg, check transaction outcome again

	// This single transaction will change its outcome based on the payload
	tx, err := t.SendNextTransaction(
		t.TestContext,
		t.Engine,
		&helper.BaseTransactionCreator{
			Recipient:  &globals.PrevRandaoContractAddr,
			Amount:     big0,
			Payload:    nil,
			TxType:     t.TestTransactionType,
			GasLimit:   75000,
			ForkConfig: t.ForkConfig,
		},
	)
	if err != nil {
		t.Fatalf("FAIL (%s): Error trying to send transaction: %v", t.TestName, err)
	}
	t.Logf("INFO (%s): sent tx %v", t.TestName, tx.Hash())

	t.CLMock.ProduceSingleBlock(clmock.BlockProcessCallbacks{
		OnNewPayloadBroadcast: func() {
			// At this point the CLMocker has a payload that will result in a specific outcome,
			// we can produce an alternative payload, send it, fcU to it, and verify the changes
			alternativePrevRandao := common.Hash{}
			rand.Read(alternativePrevRandao[:])
			timestamp := t.CLMock.LatestPayloadBuilt.Timestamp + 1
			payloadAttributes, err := (&helper.BasePayloadAttributesCustomizer{
				Timestamp: &timestamp,
				Random:    &alternativePrevRandao,
			}).GetPayloadAttributes(&t.CLMock.LatestPayloadAttributes)
			if err != nil {
				t.Fatalf("FAIL (%s): Unable to customize payload attributes: %v", t.TestName, err)
			}

			r := t.TestEngine.TestEngineForkchoiceUpdated(
				&t.CLMock.LatestForkchoice,
				payloadAttributes,
				t.CLMock.LatestPayloadBuilt.Timestamp,
			)
			r.ExpectNoError()

			time.Sleep(t.CLMock.PayloadProductionClientDelay)

			g := t.TestEngine.TestEngineGetPayload(r.Response.PayloadID, payloadAttributes)
			g.ExpectNoError()
			alternativePayload := g.Payload
			if len(alternativePayload.Transactions) == 0 {
				t.Fatalf("FAIL (%s): alternative payload does not contain the prevRandao opcode tx", t.TestName)
			}

			s := t.TestEngine.TestEngineNewPayload(&alternativePayload)
			s.ExpectStatus(test.Valid)
			s.ExpectLatestValidHash(&alternativePayload.BlockHash)

			// We sent the alternative payload, fcU to it
			p := t.TestEngine.TestEngineForkchoiceUpdated(&api.ForkchoiceStateV1{
				HeadBlockHash:      alternativePayload.BlockHash,
				SafeBlockHash:      t.CLMock.LatestForkchoice.SafeBlockHash,
				FinalizedBlockHash: t.CLMock.LatestForkchoice.FinalizedBlockHash,
			}, nil, alternativePayload.Timestamp)
			p.ExpectPayloadStatus(test.Valid)

			// PrevRandao should be the alternative prevRandao we sent
			checkPrevRandaoValue(t, alternativePrevRandao, alternativePayload.Number)
		},
	})
	// The reorg actually happens after the CLMocker continues,
	// verify here that the reorg was successful
	latestBlockNum := t.CLMock.LatestHeadNumber.Uint64()
	checkPrevRandaoValue(t, t.CLMock.PrevRandaoHistory[latestBlockNum], latestBlockNum)

}

// Re-Execute Previous Payloads
func reExecPayloads(t *test.Env) {
	// Wait until this client catches up with latest PoS
	t.CLMock.WaitForTTD()

	// How many Payloads we are going to re-execute
	var payloadReExecCount = 10

	// Create those blocks
	t.CLMock.ProduceBlocks(payloadReExecCount, clmock.BlockProcessCallbacks{})

	// Re-execute the payloads
	r := t.TestEngine.TestBlockNumber()
	r.ExpectNoError()
	lastBlock := r.Number
	t.Logf("INFO (%s): Started re-executing payloads at block: %v", t.TestName, lastBlock)

	for i := lastBlock - uint64(payloadReExecCount) + 1; i <= lastBlock; i++ {
		payload, found := t.CLMock.ExecutedPayloadHistory[i]
		if !found {
			t.Fatalf("FAIL (%s): (test issue) Payload with index %d does not exist", i)
		}

		r := t.TestEngine.TestEngineNewPayload(payload)
		r.ExpectStatus(test.Valid)
		r.ExpectLatestValidHash(&payload.BlockHash)
	}
}

// Multiple New Payloads Extending Canonical Chain
func multipleNewCanonicalPayloads(t *test.Env) {
	// Wait until this client catches up with latest PoS
	t.CLMock.WaitForTTD()

	// Produce blocks before starting the test
	t.CLMock.ProduceBlocks(5, clmock.BlockProcessCallbacks{})

	t.CLMock.ProduceSingleBlock(clmock.BlockProcessCallbacks{
		// Run test after a new payload has been obtained
		OnGetPayload: func() {
			payloadCount := 80
			basePayload := t.CLMock.LatestPayloadBuilt

			// Fabricate and send multiple new payloads by changing the PrevRandao field
			for i := 0; i < payloadCount; i++ {
				newPrevRandao := common.Hash{}
				rand.Read(newPrevRandao[:])
				customizer := &helper.CustomPayloadData{
					PrevRandao: &newPrevRandao,
				}
				newPayload, err := customizer.CustomizePayload(&basePayload)
				if err != nil {
					t.Fatalf("FAIL (%s): Unable to customize payload %v: %v", t.TestName, i, err)
				}

				r := t.TestEngine.TestEngineNewPayload(newPayload)
				r.ExpectStatus(test.Valid)
				r.ExpectLatestValidHash(&newPayload.BlockHash)
			}
		},
	})
	// At the end the CLMocker continues to try to execute fcU with the original payload, which should not fail
}

// Consecutive Payload Execution: Secondary client should be able to set the forkchoiceUpdated to payloads received consecutively
func inOrderPayloads(t *test.Env) {
	// Wait until this client catches up with latest PoS
	t.CLMock.WaitForTTD()

	// First prepare payloads on a first client, which will also contain multiple transactions

	// We will be also verifying that the transactions are correctly interpreted in the canonical chain,
	// prepare a random account to receive funds.
	recipient := common.Address{}
	rand.Read(recipient[:])
	amountPerTx := big.NewInt(1000)
	txPerPayload := 20
	payloadCount := 10
	txsIncluded := 0

	t.CLMock.ProduceBlocks(payloadCount, clmock.BlockProcessCallbacks{
		// We send the transactions after we got the Payload ID, before the CLMocker gets the prepared Payload
		OnPayloadProducerSelected: func() {
			_, err := t.SendNextTransactions(
				t.TestContext,
				t.CLMock.NextBlockProducer,
				&helper.BaseTransactionCreator{
					Recipient:  &recipient,
					Amount:     amountPerTx,
					Payload:    nil,
					TxType:     t.TestTransactionType,
					GasLimit:   75000,
					ForkConfig: t.ForkConfig,
				},
				uint64(txPerPayload),
			)
			if err != nil {
				t.Fatalf("FAIL (%s): Error trying to send transaction: %v", t.TestName, err)
			}
		},
		OnGetPayload: func() {
			if len(t.CLMock.LatestPayloadBuilt.Transactions) < (txPerPayload / 2) {
				t.Fatalf("FAIL (%s): Client failed to include all the expected transactions in payload built: %d < %d", t.TestName, len(t.CLMock.LatestPayloadBuilt.Transactions), (txPerPayload / 2))
			}
			txsIncluded += len(t.CLMock.LatestPayloadBuilt.Transactions)
		},
	})

	expectedBalance := amountPerTx.Mul(amountPerTx, big.NewInt(int64(txsIncluded)))

	// Check balance on this first client
	r := t.TestEngine.TestBalanceAt(recipient, nil)
	r.ExpectBalanceEqual(expectedBalance)

	// Start a second client to send newPayload consecutively without fcU
	secondaryClient, err := hive_rpc.HiveRPCEngineStarter{}.StartClient(t.T, t.TestContext, t.Genesis, t.ClientParams, t.ClientFiles)
	if err != nil {
		t.Fatalf("FAIL (%s): Unable to start secondary client: %v", t.TestName, err)
	}
	secondaryTestEngineClient := test.NewTestEngineClient(t, secondaryClient)

	// Send the forkchoiceUpdated with the LatestExecutedPayload hash, we should get SYNCING back
	fcU := api.ForkchoiceStateV1{
		HeadBlockHash:      t.CLMock.LatestExecutedPayload.BlockHash,
		SafeBlockHash:      t.CLMock.LatestExecutedPayload.BlockHash,
		FinalizedBlockHash: t.CLMock.LatestExecutedPayload.BlockHash,
	}

	s := secondaryTestEngineClient.TestEngineForkchoiceUpdated(&fcU, nil, t.CLMock.LatestExecutedPayload.Timestamp)
	s.ExpectPayloadStatus(test.Syncing)
	s.ExpectLatestValidHash(nil)
	s.ExpectNoValidationError()

	// Send all the payloads in the increasing order
	for k := t.CLMock.FirstPoSBlockNumber.Uint64(); k <= t.CLMock.LatestExecutedPayload.Number; k++ {
		payload := t.CLMock.ExecutedPayloadHistory[k]

		s := secondaryTestEngineClient.TestEngineNewPayload(payload)
		s.ExpectStatus(test.Valid)
		s.ExpectLatestValidHash(&payload.BlockHash)

	}

	s = secondaryTestEngineClient.TestEngineForkchoiceUpdated(&fcU, nil, t.CLMock.LatestExecutedPayload.Timestamp)
	s.ExpectPayloadStatus(test.Valid)
	s.ExpectLatestValidHash(&fcU.HeadBlockHash)
	s.ExpectNoValidationError()

	// At this point we should have our funded account balance equal to the expected value.
	q := secondaryTestEngineClient.TestBalanceAt(recipient, nil)
	q.ExpectBalanceEqual(expectedBalance)

	// Add the client to the CLMocker
	t.CLMock.AddEngineClient(secondaryClient)

	// Produce a single block on top of the canonical chain, all clients must accept this
	t.CLMock.ProduceSingleBlock(clmock.BlockProcessCallbacks{})

	// Head must point to the latest produced payload
	p := secondaryTestEngineClient.TestHeaderByNumber(nil)
	p.ExpectHash(t.CLMock.LatestExecutedPayload.BlockHash)
}

// Send a valid payload on a client that is currently SYNCING
func validPayloadFcUSyncingClient(t *test.Env) {
	var (
		secondaryClient client.EngineClient
		previousPayload typ.ExecutableData
	)
	{
		// To allow sending the primary engine client into SYNCING state, we need a secondary client to guide the payload creation
		var err error
		secondaryClient, err = hive_rpc.HiveRPCEngineStarter{}.StartClient(t.T, t.TestContext, t.Genesis, t.ClientParams, t.ClientFiles)

		if err != nil {
			t.Fatalf("FAIL (%s): Unable to spawn a secondary client: %v", t.TestName, err)
		}
		t.CLMock.AddEngineClient(secondaryClient)
	}

	// Wait until TTD is reached by all clients
	t.CLMock.WaitForTTD()

	// Produce blocks before starting the test
	t.CLMock.ProduceBlocks(5, clmock.BlockProcessCallbacks{})

	// Disconnect the first engine client from the CL Mocker and produce a block
	t.CLMock.RemoveEngineClient(t.Engine)
	t.CLMock.ProduceSingleBlock(clmock.BlockProcessCallbacks{})

	previousPayload = t.CLMock.LatestPayloadBuilt

	// Send the fcU to set it to syncing mode
	r := t.TestEngine.TestEngineForkchoiceUpdated(&t.CLMock.LatestForkchoice, nil, t.CLMock.LatestHeader.Time)
	r.ExpectPayloadStatus(test.Syncing)

	t.CLMock.ProduceSingleBlock(clmock.BlockProcessCallbacks{
		// Run test after the new payload has been obtained
		OnGetPayload: func() {
			// Send the new payload from the second client to the first, it won't be able to validate it
			r := t.TestEngine.TestEngineNewPayload(&t.CLMock.LatestPayloadBuilt)
			r.ExpectStatusEither(test.Accepted, test.Syncing)
			r.ExpectLatestValidHash(nil)

			// Send the forkchoiceUpdated with a reference to the valid payload on the SYNCING client.
			var (
				random                = common.Hash{}
				suggestedFeeRecipient = common.Address{}
			)
			payloadAttributesCustomizer := &helper.BasePayloadAttributesCustomizer{
				Random:                &random,
				SuggestedFeeRecipient: &suggestedFeeRecipient,
			}
			newPayloadAttributes, err := payloadAttributesCustomizer.GetPayloadAttributes(&t.CLMock.LatestPayloadAttributes)
			if err != nil {
				t.Fatalf("FAIL (%s): Unable to customize payload attributes: %v", t.TestName, err)
			}
			s := t.TestEngine.TestEngineForkchoiceUpdated(&api.ForkchoiceStateV1{
				HeadBlockHash:      t.CLMock.LatestPayloadBuilt.BlockHash,
				SafeBlockHash:      t.CLMock.LatestPayloadBuilt.BlockHash,
				FinalizedBlockHash: t.CLMock.LatestPayloadBuilt.BlockHash,
			}, newPayloadAttributes, t.CLMock.LatestPayloadBuilt.Timestamp)
			s.ExpectPayloadStatus(test.Syncing)

			// Send the previous payload to be able to continue
			p := t.TestEngine.TestEngineNewPayload(&previousPayload)
			p.ExpectStatus(test.Valid)
			p.ExpectLatestValidHash(&previousPayload.BlockHash)

			// Send the new payload again

			p = t.TestEngine.TestEngineNewPayload(&t.CLMock.LatestPayloadBuilt)
			p.ExpectStatus(test.Valid)
			p.ExpectLatestValidHash(&t.CLMock.LatestPayloadBuilt.BlockHash)

			s = t.TestEngine.TestEngineForkchoiceUpdated(&api.ForkchoiceStateV1{
				HeadBlockHash:      t.CLMock.LatestPayloadBuilt.BlockHash,
				SafeBlockHash:      t.CLMock.LatestPayloadBuilt.BlockHash,
				FinalizedBlockHash: t.CLMock.LatestPayloadBuilt.BlockHash,
			}, nil, t.CLMock.LatestPayloadBuilt.Timestamp)
			s.ExpectPayloadStatus(test.Valid)

		},
	})

	// Add the secondary client again to the CL Mocker
	t.CLMock.AddEngineClient(t.Engine)

	t.CLMock.RemoveEngineClient(secondaryClient)
}

// Send a valid `newPayload` in correct order but skip `forkchoiceUpdated` until the last payload
func missingFcu(t *test.Env) {
	// Wait until TTD is reached by this client
	t.CLMock.WaitForTTD()

	// Get last PoW block hash (genesis)
	lastPoWBlockHash := t.TestEngine.TestHeaderByNumber(Head).Header.Hash()

	// Produce blocks on the main client, these payloads will be replayed on the secondary client.
	t.CLMock.ProduceBlocks(5, clmock.BlockProcessCallbacks{})

	var secondaryEngineTest *test.TestEngineClient
	{
		secondaryEngine, err := hive_rpc.HiveRPCEngineStarter{}.StartClient(t.T, t.TestContext, t.Genesis, t.ClientParams, t.ClientFiles)

		if err != nil {
			t.Fatalf("FAIL (%s): Unable to spawn a secondary client: %v", t.TestName, err)
		}
		secondaryEngineTest = test.NewTestEngineClient(t, secondaryEngine)
		t.CLMock.AddEngineClient(secondaryEngine)
	}

	// Send each payload in the correct order but skip the ForkchoiceUpdated for each
	for i := t.CLMock.FirstPoSBlockNumber.Uint64(); i <= t.CLMock.LatestHeadNumber.Uint64(); i++ {
		payload := t.CLMock.ExecutedPayloadHistory[i]
		p := secondaryEngineTest.TestEngineNewPayload(payload)
		p.ExpectStatus(test.Valid)
		p.ExpectLatestValidHash(&payload.BlockHash)
	}

	// Verify that at this point, the client's head still points to the last non-PoS block
	r := secondaryEngineTest.TestHeaderByNumber(Head)
	r.ExpectHash(lastPoWBlockHash)

	// Verify that the head correctly changes after the last ForkchoiceUpdated
	fcU := api.ForkchoiceStateV1{
		HeadBlockHash:      t.CLMock.ExecutedPayloadHistory[t.CLMock.LatestHeadNumber.Uint64()].BlockHash,
		SafeBlockHash:      t.CLMock.ExecutedPayloadHistory[t.CLMock.LatestHeadNumber.Uint64()-1].BlockHash,
		FinalizedBlockHash: t.CLMock.ExecutedPayloadHistory[t.CLMock.LatestHeadNumber.Uint64()-2].BlockHash,
	}
	p := secondaryEngineTest.TestEngineForkchoiceUpdated(&fcU, nil, t.CLMock.LatestHeader.Time)
	p.ExpectPayloadStatus(test.Valid)
	p.ExpectLatestValidHash(&fcU.HeadBlockHash)

	// Now the head should've changed to the latest PoS block
	s := secondaryEngineTest.TestHeaderByNumber(Head)
	s.ExpectHash(fcU.HeadBlockHash)

}

// Attempt to produce a payload after a transaction with an invalid Chain ID was sent to the client
// using `eth_sendRawTransaction`.
func buildPayloadWithInvalidChainIDTx(t *test.Env) {
	// Wait until TTD is reached by this client
	t.CLMock.WaitForTTD()

	// Produce blocks before starting the test
	t.CLMock.ProduceBlocks(5, clmock.BlockProcessCallbacks{})

	// Send a transaction with an incorrect ChainID.
	// Transaction must be not be included in payload creation.
	var invalidChainIDTx *types.Transaction
	t.CLMock.ProduceSingleBlock(clmock.BlockProcessCallbacks{
		// Run test after a new payload has been broadcast
		OnPayloadProducerSelected: func() {
			testAccount := globals.TestAccounts[0]
			ctx, cancel := context.WithTimeout(t.TestContext, globals.RPCTimeout)
			defer cancel()
			nonce, err := t.CLMock.NextBlockProducer.NonceAt(ctx, testAccount.GetAddress(), nil)
			if err != nil {
				t.Fatalf("FAIL(%s): Unable to get address nonce: %v", t.TestName, err)
			}
			txData := &types.LegacyTx{
				Nonce:    nonce,
				To:       &globals.PrevRandaoContractAddr,
				Value:    big0,
				Gas:      75000,
				GasPrice: globals.GasPrice,
				Data:     nil,
			}
			invalidChainID := new(big.Int).Set(globals.ChainID)
			invalidChainID.Add(invalidChainID, big1)
			invalidChainIDTx, err := types.SignTx(types.NewTx(txData), types.NewCancunSigner(invalidChainID), testAccount.GetKey())
			if err != nil {
				t.Fatalf("FAIL(%s): Unable to sign tx with invalid chain ID: %v", t.TestName, err)
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

// Fee Recipient Tests
func suggestedFeeRecipient(t *test.Env) {
	// Wait until this client catches up with latest PoS
	t.CLMock.WaitForTTD()

	// Amount of transactions to send
	txCount := 20

	// Verify that, in a block with transactions, fees are accrued by the suggestedFeeRecipient
	feeRecipient := common.Address{}
	rand.Read(feeRecipient[:])
	txRecipient := common.Address{}
	rand.Read(txRecipient[:])

	// Send multiple transactions
	for i := 0; i < txCount; i++ {
		_, err := t.SendNextTransaction(
			t.TestContext,
			t.Engine,
			&helper.BaseTransactionCreator{
				Recipient:  &txRecipient,
				Amount:     big0,
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
	// Produce the next block with the fee recipient set
	t.CLMock.NextFeeRecipient = feeRecipient
	t.CLMock.ProduceSingleBlock(clmock.BlockProcessCallbacks{})

	// Calculate the fees and check that they match the balance of the fee recipient
	r := t.TestEngine.TestBlockByNumber(Head)
	r.ExpectTransactionCountEqual(txCount)
	r.ExpectCoinbase(feeRecipient)
	blockIncluded := r.Block

	feeRecipientFees := big.NewInt(0)
	for _, tx := range blockIncluded.Transactions() {
		effGasTip, err := tx.EffectiveGasTip(blockIncluded.BaseFee())
		if err != nil {
			t.Fatalf("FAIL (%s): unable to obtain EffectiveGasTip: %v", t.TestName, err)
		}
		ctx, cancel := context.WithTimeout(t.TestContext, globals.RPCTimeout)
		defer cancel()
		receipt, err := t.Eth.TransactionReceipt(ctx, tx.Hash())
		if err != nil {
			t.Fatalf("FAIL (%s): unable to obtain receipt: %v", t.TestName, err)
		}
		feeRecipientFees = feeRecipientFees.Add(feeRecipientFees, effGasTip.Mul(effGasTip, big.NewInt(int64(receipt.GasUsed))))
	}

	s := t.TestEngine.TestBalanceAt(feeRecipient, nil)
	s.ExpectBalanceEqual(feeRecipientFees)

	// Produce another block without txns and get the balance again
	t.CLMock.NextFeeRecipient = feeRecipient
	t.CLMock.ProduceSingleBlock(clmock.BlockProcessCallbacks{})

	s = t.TestEngine.TestBalanceAt(feeRecipient, nil)
	s.ExpectBalanceEqual(feeRecipientFees)

}

// TODO: Do a PENDING block suggestedFeeRecipient

func checkPrevRandaoValue(t *test.Env, expectedPrevRandao common.Hash, blockNumber uint64) {
	storageKey := common.Hash{}
	storageKey[31] = byte(blockNumber)
	r := t.TestEngine.TestStorageAt(globals.PrevRandaoContractAddr, storageKey, nil)
	r.ExpectStorageEqual(expectedPrevRandao)

}

// PrevRandao Opcode tests
func prevRandaoOpcodeTx(t *test.Env) {
	t.CLMock.WaitForTTD()

	// Send transactions in PoS, the value of the storage in these blocks must match the prevRandao value
	var (
		txCount        = 10
		currentTxIndex = 0
		txs            = make([]typ.Transaction, 0)
	)
	t.CLMock.ProduceBlocks(txCount, clmock.BlockProcessCallbacks{
		OnPayloadProducerSelected: func() {
			tx, err := t.SendNextTransaction(
				t.TestContext,
				t.Engine,
				&helper.BaseTransactionCreator{
					Recipient:  &globals.PrevRandaoContractAddr,
					Amount:     big0,
					Payload:    nil,
					TxType:     t.TestTransactionType,
					GasLimit:   75000,
					ForkConfig: t.ForkConfig,
				},
			)
			if err != nil {
				t.Fatalf("FAIL (%s): Error trying to send transaction: %v", t.TestName, err)
			}
			txs = append(txs, tx)
			currentTxIndex++
		},
		OnForkchoiceBroadcast: func() {
			// Check the transaction tracing, which is client specific
			expectedPrevRandao := t.CLMock.PrevRandaoHistory[t.CLMock.LatestHeader.Number.Uint64()+1]
			ctx, cancel := context.WithTimeout(t.TestContext, globals.RPCTimeout)
			defer cancel()
			if err := helper.DebugPrevRandaoTransaction(ctx, t.Client.RPC(), t.Client.Type, txs[currentTxIndex-1],
				&expectedPrevRandao); err != nil {
				t.Fatalf("FAIL (%s): Error during transaction tracing: %v", t.TestName, err)
			}
		},
	})

	ctx, cancel := context.WithTimeout(t.TestContext, globals.RPCTimeout)
	defer cancel()
	lastBlockNumber, err := t.Eth.BlockNumber(ctx)
	if err != nil {
		t.Fatalf("FAIL (%s): Unable to get latest block number: %v", t.TestName, err)
	}
	for i := uint64(1); i <= lastBlockNumber; i++ {
		checkPrevRandaoValue(t, t.CLMock.PrevRandaoHistory[i], i)
	}

}

// Engine API errors

func pUint64(v uint64) *uint64 {
	return &v
}

// Register all test combinations for Paris
func init() {
	// Register bad hash tests
	for _, syncing := range []bool{false, true} {
		for _, sidechain := range []bool{false, true} {
			Tests = append(Tests, BadHashOnNewPayload{
				Syncing:   syncing,
				Sidechain: sidechain,
			})
		}
	}

	// Parent hash == block hash tests
	Tests = append(Tests,
		ParentHashOnNewPayload{
			Syncing: false,
		},
		ParentHashOnNewPayload{
			Syncing: true,
		},
	)

	// Register RPC tests
	for _, field := range []BlockStatusRPCCheckType{
		LatestOnNewPayload,
		LatestOnHeadBlockHash,
		SafeOnSafeBlockHash,
		FinalizedOnFinalizedBlockHash,
	} {
		Tests = append(Tests, BlockStatus{CheckType: field})
	}

	// Register ForkchoiceUpdate tests
	for _, field := range []ForkchoiceStateField{
		HeadBlockHash,
		SafeBlockHash,
		FinalizedBlockHash,
	} {
		Tests = append(Tests,
			InconsistentForkchoiceTest{
				Field: field,
			},
			ForkchoiceUpdatedUnknownBlockHashTest{
				Field: field,
			},
		)
	}

	// Payload Attributes Tests
	for _, t := range []InvalidPayloadAttributesTest{
		{
			Description: "Zero timestamp",
			Customizer: &helper.BasePayloadAttributesCustomizer{
				Timestamp: pUint64(0),
			},
		},
		{
			Description: "Parent timestamp",
			Customizer: &helper.TimestampDeltaPayloadAttributesCustomizer{
				PayloadAttributesCustomizer: &helper.BasePayloadAttributesCustomizer{},
				TimestampDelta:              -1,
			},
		},
	} {
		Tests = append(Tests, t)
		t.Syncing = true
		Tests = append(Tests, t)
	}

	// Payload ID Tests
	for _, payloadAttributeFieldChange := range []PayloadAttributesFieldChange{
		PayloadAttributesIncreaseTimestamp,
		PayloadAttributesRandom,
		PayloadAttributesSuggestedFeeRecipient,
	} {
		Tests = append(Tests, UniquePayloadIDTest{
			FieldModification: payloadAttributeFieldChange,
		})
	}

	// Endpoint Versions Tests
	// Early upgrade of ForkchoiceUpdated when requesting a payload
	Tests = append(Tests,
		ForkchoiceUpdatedOnPayloadRequestTest{
			BaseSpec: test.BaseSpec{
				Name:       "Early upgrade",
				ForkHeight: 2,
			},
			ForkchoiceUpdatedCustomizer: &helper.UpgradeForkchoiceUpdatedVersion{
				ForkchoiceUpdatedCustomizer: &helper.BaseForkchoiceUpdatedCustomizer{
					ExpectedError: globals.UNSUPPORTED_FORK_ERROR,
				},
			},
		},
		/*
			TODO: This test is failing because the upgraded version of the ForkchoiceUpdated does not contain the
			      expected fields of the following version.
			ForkchoiceUpdatedOnHeadBlockUpdateTest{
				BaseSpec: test.BaseSpec{
					Name:       "Early upgrade",
					ForkHeight: 2,
				},
				ForkchoiceUpdatedCustomizer: &helper.UpgradeForkchoiceUpdatedVersion{
					ForkchoiceUpdatedCustomizer: &helper.BaseForkchoiceUpdatedCustomizer{
						ExpectedError: globals.UNSUPPORTED_FORK_ERROR,
					},
				},
			},
		*/
	)

	// Invalid Payload Tests
	for _, invalidField := range []helper.InvalidPayloadBlockField{
		helper.InvalidParentHash,
		helper.InvalidStateRoot,
		helper.InvalidReceiptsRoot,
		helper.InvalidNumber,
		helper.InvalidGasLimit,
		helper.InvalidGasUsed,
		helper.InvalidTimestamp,
		helper.InvalidPrevRandao,
		helper.RemoveTransaction,
	} {
		for _, syncing := range []bool{false, true} {
			if invalidField == helper.InvalidStateRoot {
				Tests = append(Tests, InvalidPayloadTestCase{
					InvalidField:      invalidField,
					Syncing:           syncing,
					EmptyTransactions: true,
				})
			}
			Tests = append(Tests, InvalidPayloadTestCase{
				InvalidField: invalidField,
				Syncing:      syncing,
			})
		}
	}

	Tests = append(Tests, PayloadBuildAfterInvalidPayloadTest{
		InvalidField: helper.InvalidStateRoot,
	})

	// Invalid Transaction Payload Tests
	for _, invalidField := range []helper.InvalidPayloadBlockField{
		helper.InvalidTransactionSignature,
		helper.InvalidTransactionNonce,
		helper.InvalidTransactionGasPrice,
		helper.InvalidTransactionGasTipPrice,
		helper.InvalidTransactionGas,
		helper.InvalidTransactionValue,
		helper.InvalidTransactionChainID,
	} {
		for _, syncing := range []bool{false, true} {
			if invalidField != helper.InvalidTransactionGasTipPrice {
				for _, testTxType := range []helper.TestTransactionType{helper.LegacyTxOnly, helper.DynamicFeeTxOnly} {
					Tests = append(Tests, InvalidPayloadTestCase{
						BaseSpec: test.BaseSpec{
							TestTransactionType: testTxType,
						},
						InvalidField: invalidField,
						Syncing:      syncing,
					})
				}
			} else {
				Tests = append(Tests, InvalidPayloadTestCase{
					BaseSpec: test.BaseSpec{
						TestTransactionType: helper.DynamicFeeTxOnly,
					},
					InvalidField: invalidField,
					Syncing:      syncing,
				})
			}
		}

	}

	// Invalid Ancestor Re-Org Tests (Reveal Via NewPayload)
	for _, invalidIndex := range []int{1, 9, 10} {
		for _, emptyTxs := range []bool{false, true} {
			Tests = append(Tests, InvalidMissingAncestorReOrgTest{
				SidechainLength:   10,
				InvalidIndex:      invalidIndex,
				InvalidField:      helper.InvalidStateRoot,
				EmptyTransactions: emptyTxs,
			})
		}
	}

	// Invalid Ancestor Re-Org Tests (Reveal Via Sync)
	spec := test.BaseSpec{
		TimeoutSeconds:   60,
		SlotsToFinalized: big.NewInt(20),
	}
	for _, invalidField := range []helper.InvalidPayloadBlockField{
		helper.InvalidStateRoot,
		helper.InvalidReceiptsRoot,
		// TODO: helper.InvalidNumber, Test is causing a panic on the secondary node, disabling for now.
		helper.InvalidGasLimit,
		helper.InvalidGasUsed,
		helper.InvalidTimestamp,
		// TODO: helper.InvalidPrevRandao, Test consistently fails with Failed to set invalid block: missing trie node.
		helper.RemoveTransaction,
		helper.InvalidTransactionSignature,
		helper.InvalidTransactionNonce,
		helper.InvalidTransactionGas,
		helper.InvalidTransactionGasPrice,
		helper.InvalidTransactionValue,
		// helper.InvalidOmmers, Unsupported now
	} {
		for _, reOrgFromCanonical := range []bool{false, true} {
			invalidIndex := 9
			if invalidField == helper.InvalidReceiptsRoot ||
				invalidField == helper.InvalidGasLimit ||
				invalidField == helper.InvalidGasUsed ||
				invalidField == helper.InvalidTimestamp ||
				invalidField == helper.InvalidPrevRandao {
				invalidIndex = 8
			}
			if invalidField == helper.InvalidStateRoot {
				Tests = append(Tests, InvalidMissingAncestorReOrgSyncTest{
					BaseSpec:           spec,
					InvalidField:       invalidField,
					ReOrgFromCanonical: reOrgFromCanonical,
					EmptyTransactions:  true,
					InvalidIndex:       invalidIndex,
				})
			}
			Tests = append(Tests, InvalidMissingAncestorReOrgSyncTest{
				BaseSpec:           spec,
				InvalidField:       invalidField,
				ReOrgFromCanonical: reOrgFromCanonical,
				InvalidIndex:       invalidIndex,
			})
		}
	}

	// Re-org using the Engine API tests

	// Re-org a transaction out of a block, or into a new block
	Tests = append(Tests,
		TransactionReOrgTest{
			ReorgOut: true,
		},
		TransactionReOrgTest{
			ReorgDifferentBlock: true,
		},
		TransactionReOrgTest{
			ReorgDifferentBlock: true,
			NewPayloadOnRevert:  true,
		},
	)

	// Re-Org back into the canonical chain tests
	Tests = append(Tests,
		ReOrgBackToCanonicalTest{
			BaseSpec: test.BaseSpec{
				SlotsToSafe:      big.NewInt(10),
				SlotsToFinalized: big.NewInt(20),
				TimeoutSeconds:   60,
			},
			TransactionPerPayload: 1,
			ReOrgDepth:            5,
		},
		ReOrgBackToCanonicalTest{
			BaseSpec: test.BaseSpec{
				SlotsToSafe:      big.NewInt(32),
				SlotsToFinalized: big.NewInt(64),
				TimeoutSeconds:   120,
			},
			TransactionPerPayload:     50,
			ReOrgDepth:                10,
			ExecuteSidePayloadOnReOrg: true,
		},
	)

	// Fork ID Tests
	Tests = append(Tests,
		ForkIDSpec{
			BaseSpec: test.BaseSpec{
				Name:             "Fork ID: Genesis at 0, Fork at 0",
				GenesisTimestamp: pUint64(0),
				MainFork:         config.Paris,
				ForkTime:         0,
			},
		},
		ForkIDSpec{
			BaseSpec: test.BaseSpec{
				Name:             "Fork ID: Genesis at 0, Fork at 1",
				GenesisTimestamp: pUint64(0),
				MainFork:         config.Paris,
				ForkTime:         1,
			},
		},
		ForkIDSpec{
			BaseSpec: test.BaseSpec{
				Name:             "Fork ID: Genesis at 1, Fork at 1",
				GenesisTimestamp: pUint64(0),
				MainFork:         config.Paris,
				ForkTime:         1,
			},
		},
		ForkIDSpec{
			BaseSpec: test.BaseSpec{
				Name:             "Fork ID: Genesis at 0, Fork at 1, Current Block 1",
				GenesisTimestamp: pUint64(0),
				MainFork:         config.Paris,
				ForkTime:         1,
			},
			ProduceBlocksBeforePeering: 1,
		},
		ForkIDSpec{
			BaseSpec: test.BaseSpec{
				Name:             "Fork ID: Genesis at 1, Fork at 1, Previous Fork at 1",
				GenesisTimestamp: pUint64(1),
				MainFork:         config.Paris,
				ForkTime:         1,
				PreviousForkTime: 1,
			},
		},
		ForkIDSpec{
			BaseSpec: test.BaseSpec{
				Name:             "Fork ID: Genesis at 1, Fork at 2, Previous Fork at 1",
				GenesisTimestamp: pUint64(1),
				MainFork:         config.Paris,
				ForkTime:         2,
				PreviousForkTime: 1,
			},
		},
		ForkIDSpec{
			BaseSpec: test.BaseSpec{
				Name:             "Fork ID: Genesis at 1, Fork at 2, Previous Fork at 1, Current Block 1",
				GenesisTimestamp: pUint64(1),
				MainFork:         config.Paris,
				ForkTime:         2,
				PreviousForkTime: 1,
			},
			ProduceBlocksBeforePeering: 1,
		},
	)
}
