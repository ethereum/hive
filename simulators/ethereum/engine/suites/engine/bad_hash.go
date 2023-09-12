package suite_engine

import (
	"fmt"
	"math/rand"

	api "github.com/ethereum/go-ethereum/beacon/engine"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/hive/simulators/ethereum/engine/clmock"
	"github.com/ethereum/hive/simulators/ethereum/engine/config"
	"github.com/ethereum/hive/simulators/ethereum/engine/helper"
	"github.com/ethereum/hive/simulators/ethereum/engine/test"
	typ "github.com/ethereum/hive/simulators/ethereum/engine/types"
)

// Corrupt the hash of a valid payload, client should reject the payload.
// All possible scenarios:
//    (fcU)
//	┌────────┐        ┌────────────────────────┐
//	│  HEAD  │◄───────┤ Bad Hash (!Sync,!Side) │
//	└────┬───┘        └────────────────────────┘
//		 │
//		 │
//	┌────▼───┐        ┌────────────────────────┐
//	│ HEAD-1 │◄───────┤ Bad Hash (!Sync, Side) │
//	└────┬───┘        └────────────────────────┘
//		 │
//
//
//	  (fcU)
//	********************  ┌───────────────────────┐
//	*  (Unknown) HEAD  *◄─┤ Bad Hash (Sync,!Side) │
//	********************  └───────────────────────┘
//		 │
//		 │
//	┌────▼───┐            ┌───────────────────────┐
//	│ HEAD-1 │◄───────────┤ Bad Hash (Sync, Side) │
//	└────┬───┘            └───────────────────────┘
//		 │
//

type BadHashOnNewPayload struct {
	test.BaseSpec
	Syncing   bool
	Sidechain bool
}

func (s BadHashOnNewPayload) WithMainFork(fork config.Fork) test.Spec {
	specCopy := s
	specCopy.MainFork = fork
	return specCopy
}

func (b BadHashOnNewPayload) GetName() string {
	return fmt.Sprintf("Bad Hash on NewPayload (Syncing=%v, Sidechain=%v)", b.Syncing, b.Sidechain)
}

func (b BadHashOnNewPayload) Execute(t *test.Env) {
	// Wait until TTD is reached by this client
	t.CLMock.WaitForTTD()

	// Produce blocks before starting the test
	t.CLMock.ProduceBlocks(5, clmock.BlockProcessCallbacks{})

	var (
		alteredPayload     typ.ExecutableData
		invalidPayloadHash common.Hash
	)

	t.CLMock.ProduceSingleBlock(clmock.BlockProcessCallbacks{
		// Run test after the new payload has been obtained
		OnGetPayload: func() {
			// Alter hash on the payload and send it to client, should produce an error
			alteredPayload = t.CLMock.LatestPayloadBuilt
			invalidPayloadHash = alteredPayload.BlockHash
			invalidPayloadHash[common.HashLength-1] = byte(255 - invalidPayloadHash[common.HashLength-1])
			alteredPayload.BlockHash = invalidPayloadHash

			if !b.Syncing && b.Sidechain {
				// We alter the payload by setting the parent to a known past block in the
				// canonical chain, which makes this payload a side chain payload, and also an invalid block hash
				// (because we did not update the block hash appropriately)
				alteredPayload.ParentHash = t.CLMock.LatestHeader.ParentHash
			} else if b.Syncing {
				// We need to send an fcU to put the client in SYNCING state.
				randomHeadBlock := common.Hash{}
				rand.Read(randomHeadBlock[:])
				fcU := api.ForkchoiceStateV1{
					HeadBlockHash:      randomHeadBlock,
					SafeBlockHash:      t.CLMock.LatestHeader.Hash(),
					FinalizedBlockHash: t.CLMock.LatestHeader.Hash(),
				}
				r := t.TestEngine.TestEngineForkchoiceUpdated(&fcU, nil, 0)
				r.ExpectPayloadStatus(test.Syncing)

				if b.Sidechain {
					// Syncing and sidechain, the caonincal head is an unknown payload to us,
					// but this specific bad hash payload is in theory part of a side chain.
					// Therefore the parent we use is the head hash.
					alteredPayload.ParentHash = t.CLMock.LatestHeader.Hash()
				} else {
					// The invalid bad-hash payload points to the unknown head, but we know it is
					// indeed canonical because the head was set using forkchoiceUpdated.
					alteredPayload.ParentHash = randomHeadBlock
				}
			}

			// Execution specification::
			// - {status: INVALID_BLOCK_HASH, latestValidHash: null, validationError: null} if the blockHash validation has failed
			// Starting from Shanghai, INVALID should be returned instead (https://github.com/ethereum/execution-apis/pull/338)
			r := t.TestEngine.TestEngineNewPayload(&alteredPayload)
			if r.Version >= 2 {
				r.ExpectStatus(test.Invalid)
			} else {
				r.ExpectStatusEither(test.InvalidBlockHash, test.Invalid)
			}
			r.ExpectLatestValidHash(nil)
		},
	})

	// Lastly, attempt to build on top of the invalid payload
	t.CLMock.ProduceSingleBlock(clmock.BlockProcessCallbacks{
		// Run test after the new payload has been obtained
		OnGetPayload: func() {
			customizer := &helper.CustomPayloadData{
				ParentHash: &alteredPayload.BlockHash,
			}
			alteredPayload, err := customizer.CustomizePayload(&t.CLMock.LatestPayloadBuilt)
			if err != nil {
				t.Fatalf("FAIL (%s): Unable to modify payload: %v", t.TestName, err)
			}

			// Response status can be ACCEPTED (since parent payload could have been thrown out by the client)
			// or INVALID (client still has the payload and can verify that this payload is incorrectly building on top of it),
			// but a VALID response is incorrect.
			r := t.TestEngine.TestEngineNewPayload(alteredPayload)
			r.ExpectStatusEither(test.Accepted, test.Invalid, test.Syncing)

		},
	})
}

type ParentHashOnNewPayload struct {
	test.BaseSpec
	Syncing bool
}

func (s ParentHashOnNewPayload) WithMainFork(fork config.Fork) test.Spec {
	specCopy := s
	specCopy.MainFork = fork
	return specCopy
}

func (p ParentHashOnNewPayload) GetName() string {
	name := "ParentHash==BlockHash on NewPayload"
	if p.Syncing {
		name += " (Syncing)"
	}
	return name
}

// Copy the parentHash into the blockHash, client should reject the payload
// (from Kintsugi Incident Report: https://notes.ethereum.org/@ExXcnR0-SJGthjz1dwkA1A/BkkdHWXTY)
func (b ParentHashOnNewPayload) Execute(t *test.Env) {
	// Wait until TTD is reached by this client
	t.CLMock.WaitForTTD()

	// Produce blocks before starting the test
	t.CLMock.ProduceBlocks(5, clmock.BlockProcessCallbacks{})

	t.CLMock.ProduceSingleBlock(clmock.BlockProcessCallbacks{
		// Run test after the new payload has been obtained
		OnGetPayload: func() {
			// Alter hash on the payload and send it to client, should produce an error
			alteredPayload := t.CLMock.LatestPayloadBuilt
			if b.Syncing {
				// Parent hash is unknown but also (incorrectly) set as the block hash
				rand.Read(alteredPayload.ParentHash[:])
			}
			alteredPayload.BlockHash = alteredPayload.ParentHash
			// Execution specification::
			// - {status: INVALID_BLOCK_HASH, latestValidHash: null, validationError: null} if the blockHash validation has failed
			// Starting from Shanghai, INVALID should be returned instead (https://github.com/ethereum/execution-apis/pull/338)
			r := t.TestEngine.TestEngineNewPayload(&alteredPayload)
			if r.Version >= 2 {
				r.ExpectStatus(test.Invalid)
			} else {
				r.ExpectStatusEither(test.Invalid, test.InvalidBlockHash)
			}
			r.ExpectLatestValidHash(nil)
		},
	})

}
