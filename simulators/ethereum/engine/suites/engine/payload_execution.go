package suite_engine

import (
	"math/big"
	"math/rand"

	api "github.com/ethereum/go-ethereum/beacon/engine"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/hive/simulators/ethereum/engine/client"
	"github.com/ethereum/hive/simulators/ethereum/engine/client/hive_rpc"
	"github.com/ethereum/hive/simulators/ethereum/engine/clmock"
	"github.com/ethereum/hive/simulators/ethereum/engine/config"
	"github.com/ethereum/hive/simulators/ethereum/engine/helper"
	"github.com/ethereum/hive/simulators/ethereum/engine/test"
	typ "github.com/ethereum/hive/simulators/ethereum/engine/types"
)

type ReExecutePayloadTest struct {
	test.BaseSpec
	PayloadCount int
}

func (s ReExecutePayloadTest) WithMainFork(fork config.Fork) test.Spec {
	specCopy := s
	specCopy.MainFork = fork
	return specCopy
}

func (s ReExecutePayloadTest) GetName() string {
	name := "Re-Execute Payload"
	return name
}

// Consecutive Payload Execution: Secondary client should be able to set the forkchoiceUpdated to payloads received consecutively
func (spec ReExecutePayloadTest) Execute(t *test.Env) {
	// Wait until this client catches up with latest PoS
	t.CLMock.WaitForTTD()

	// How many Payloads we are going to re-execute
	var payloadReExecCount = 10

	if spec.PayloadCount > 0 {
		payloadReExecCount = spec.PayloadCount
	}

	// Create those blocks
	t.CLMock.ProduceBlocks(payloadReExecCount, clmock.BlockProcessCallbacks{
		OnPayloadProducerSelected: func() {
			// Send at least one transaction per payload
			_, err := t.SendNextTransaction(
				t.TestContext,
				t.CLMock.NextBlockProducer,
				&helper.BaseTransactionCreator{
					Recipient:  nil,
					Amount:     nil,
					Payload:    nil,
					TxType:     t.TestTransactionType,
					GasLimit:   75000,
					ForkConfig: t.ForkConfig,
				},
			)
			if err != nil {
				t.Fatalf("FAIL (%s): Error trying to send transaction: %v", t.TestName, err)
			}
		},
		OnGetPayload: func() {
			// Check that the transaction was included
			if len(t.CLMock.LatestPayloadBuilt.Transactions) == 0 {
				t.Fatalf("FAIL (%s): Client failed to include the expected transaction in payload built", t.TestName)
			}
		},
	})

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

type InOrderPayloadExecutionTest struct {
	test.BaseSpec
}

func (s InOrderPayloadExecutionTest) WithMainFork(fork config.Fork) test.Spec {
	specCopy := s
	specCopy.MainFork = fork
	return specCopy
}

func (s InOrderPayloadExecutionTest) GetName() string {
	name := "In-Order Consecutive Payload Execution"
	return name
}

// Consecutive Payload Execution: Secondary client should be able to set the forkchoiceUpdated to payloads received consecutively
func (spec InOrderPayloadExecutionTest) Execute(t *test.Env) {
	// Wait until this client catches up with latest PoS
	t.CLMock.WaitForTTD()

	// Send a single block to allow sending newer transaction types on the payloads
	t.CLMock.ProduceSingleBlock(clmock.BlockProcessCallbacks{})

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

type MultiplePayloadsExtendingCanonicalChainTest struct {
	test.BaseSpec
	// How many parallel payloads to execute
	PayloadCount int
	// If set to true, the head will be set to the first payload executed by the client
	// If set to false, the head will be set to the latest payload executed by the client
	SetHeadToFirstPayloadReceived bool
}

func (s MultiplePayloadsExtendingCanonicalChainTest) WithMainFork(fork config.Fork) test.Spec {
	specCopy := s
	specCopy.MainFork = fork
	return specCopy
}

func (s MultiplePayloadsExtendingCanonicalChainTest) GetName() string {
	name := "Multiple New Payloads Extending Canonical Chain"
	if s.SetHeadToFirstPayloadReceived {
		name += " (FcU to first payload received)"
	}
	return name
}

// Consecutive Payload Execution: Secondary client should be able to set the forkchoiceUpdated to payloads received consecutively
func (spec MultiplePayloadsExtendingCanonicalChainTest) Execute(t *test.Env) {
	// Wait until this client catches up with latest PoS
	t.CLMock.WaitForTTD()

	// Produce blocks before starting the test
	t.CLMock.ProduceBlocks(5, clmock.BlockProcessCallbacks{})

	callbacks := clmock.BlockProcessCallbacks{
		// We send the transactions after we got the Payload ID, before the CLMocker gets the prepared Payload
		OnPayloadProducerSelected: func() {
			recipient := common.Address{}
			rand.Read(recipient[:])
			_, err := t.SendNextTransaction(
				t.TestContext,
				t.CLMock.NextBlockProducer,
				&helper.BaseTransactionCreator{
					Recipient:  &recipient,
					Amount:     nil,
					Payload:    nil,
					TxType:     t.TestTransactionType,
					GasLimit:   75000,
					ForkConfig: t.ForkConfig,
				},
			)
			if err != nil {
				t.Fatalf("FAIL (%s): Error trying to send transaction: %v", t.TestName, err)
			}
		},
	}

	reExecFunc := func() {
		payloadCount := 80
		if spec.PayloadCount > 0 {
			payloadCount = spec.PayloadCount
		}

		basePayload := t.CLMock.LatestPayloadBuilt

		// Check that the transaction was included
		if len(basePayload.Transactions) == 0 {
			t.Fatalf("FAIL (%s): Client failed to include the expected transaction in payload built", t.TestName)
		}

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
	}

	if spec.SetHeadToFirstPayloadReceived {
		// We are going to set the head of the chain to the first payload executed by the client
		// Therefore our re-execution function must be executed after the payload was broadcast
		callbacks.OnNewPayloadBroadcast = reExecFunc
	} else {
		// Otherwise, we execute the payloads after we get the canonical one so it's
		// executed last
		callbacks.OnGetPayload = reExecFunc
	}

	t.CLMock.ProduceSingleBlock(callbacks)
	// At the end the CLMocker continues to try to execute fcU with the original payload, which should not fail
}

type NewPayloadOnSyncingClientTest struct {
	test.BaseSpec
}

func (s NewPayloadOnSyncingClientTest) WithMainFork(fork config.Fork) test.Spec {
	specCopy := s
	specCopy.MainFork = fork
	return specCopy
}

func (s NewPayloadOnSyncingClientTest) GetName() string {
	name := "Valid NewPayload->ForkchoiceUpdated on Syncing Client"
	return name
}

// Send a valid payload on a client that is currently SYNCING
func (spec NewPayloadOnSyncingClientTest) Execute(t *test.Env) {
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

	// Set a random transaction recipient
	recipient := common.Address{}
	rand.Read(recipient[:])

	// Disconnect the first engine client from the CL Mocker and produce a block
	t.CLMock.RemoveEngineClient(t.Engine)
	t.CLMock.ProduceSingleBlock(clmock.BlockProcessCallbacks{
		OnPayloadProducerSelected: func() {
			// Send at least one transaction per payload
			_, err := t.SendNextTransaction(
				t.TestContext,
				t.CLMock.NextBlockProducer,
				&helper.BaseTransactionCreator{
					Recipient:  &recipient,
					Amount:     nil,
					Payload:    nil,
					TxType:     t.TestTransactionType,
					GasLimit:   75000,
					ForkConfig: t.ForkConfig,
				},
			)
			if err != nil {
				t.Fatalf("FAIL (%s): Error trying to send transaction: %v", t.TestName, err)
			}
		},
		OnGetPayload: func() {
			// Check that the transaction was included
			if len(t.CLMock.LatestPayloadBuilt.Transactions) == 0 {
				t.Fatalf("FAIL (%s): Client failed to include the expected transaction in payload built", t.TestName)
			}
		},
	})

	previousPayload = t.CLMock.LatestPayloadBuilt

	// Send the fcU to set it to syncing mode
	r := t.TestEngine.TestEngineForkchoiceUpdated(&t.CLMock.LatestForkchoice, nil, t.CLMock.LatestHeader.Time)
	r.ExpectPayloadStatus(test.Syncing)

	t.CLMock.ProduceSingleBlock(clmock.BlockProcessCallbacks{
		OnPayloadProducerSelected: func() {
			// Send at least one transaction per payload
			_, err := t.SendNextTransaction(
				t.TestContext,
				t.CLMock.NextBlockProducer,
				&helper.BaseTransactionCreator{
					Recipient:  &recipient,
					Amount:     nil,
					Payload:    nil,
					TxType:     t.TestTransactionType,
					GasLimit:   75000,
					ForkConfig: t.ForkConfig,
				},
			)
			if err != nil {
				t.Fatalf("FAIL (%s): Error trying to send transaction: %v", t.TestName, err)
			}
		},
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

type NewPayloadWithMissingFcUTest struct {
	test.BaseSpec
}

func (s NewPayloadWithMissingFcUTest) WithMainFork(fork config.Fork) test.Spec {
	specCopy := s
	specCopy.MainFork = fork
	return specCopy
}

func (s NewPayloadWithMissingFcUTest) GetName() string {
	name := "NewPayload with Missing ForkchoiceUpdated"
	return name
}

// Send a valid `newPayload` in correct order but skip `forkchoiceUpdated` until the last payload
func (spec NewPayloadWithMissingFcUTest) Execute(t *test.Env) {
	// Wait until TTD is reached by this client
	t.CLMock.WaitForTTD()

	// Get last genesis block hash
	genesisHash := t.TestEngine.TestHeaderByNumber(Head).Header.Hash()

	// Produce blocks on the main client, these payloads will be replayed on the secondary client.
	t.CLMock.ProduceBlocks(5, clmock.BlockProcessCallbacks{
		OnPayloadProducerSelected: func() {
			var recipient common.Address
			rand.Read(recipient[:])
			// Send at least one transaction per payload
			_, err := t.SendNextTransaction(
				t.TestContext,
				t.CLMock.NextBlockProducer,
				&helper.BaseTransactionCreator{
					Recipient:  &recipient,
					Amount:     nil,
					Payload:    nil,
					TxType:     t.TestTransactionType,
					GasLimit:   75000,
					ForkConfig: t.ForkConfig,
				},
			)
			if err != nil {
				t.Fatalf("FAIL (%s): Error trying to send transaction: %v", t.TestName, err)
			}
		},
		OnGetPayload: func() {
			// Check that the transaction was included
			if len(t.CLMock.LatestPayloadBuilt.Transactions) == 0 {
				t.Fatalf("FAIL (%s): Client failed to include the expected transaction in payload built", t.TestName)
			}
		},
	})

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
	r.ExpectHash(genesisHash)

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
