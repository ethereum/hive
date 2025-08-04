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
	"github.com/ethereum/hive/simulators/ethereum/engine/client/node"
	"github.com/ethereum/hive/simulators/ethereum/engine/clmock"
	"github.com/ethereum/hive/simulators/ethereum/engine/config"
	"github.com/ethereum/hive/simulators/ethereum/engine/globals"
	"github.com/ethereum/hive/simulators/ethereum/engine/helper"
	"github.com/ethereum/hive/simulators/ethereum/engine/test"
	typ "github.com/ethereum/hive/simulators/ethereum/engine/types"
)

// Attempt to re-org to a chain which at some point contains an unknown payload which is also invalid.
// Then reveal the invalid payload and expect that the client rejects it and rejects forkchoice updated calls to this chain.
// The InvalidIndex parameter determines how many payloads apart is the common ancestor from the block that invalidates the chain,
// with a value of 1 meaning that the immediate payload after the common ancestor will be invalid.

type InvalidMissingAncestorReOrgTest struct {
	test.BaseSpec
	SidechainLength   int
	InvalidIndex      int
	InvalidField      helper.InvalidPayloadBlockField
	EmptyTransactions bool
}

func (s InvalidMissingAncestorReOrgTest) WithMainFork(fork config.Fork) test.Spec {
	specCopy := s
	specCopy.MainFork = fork
	return specCopy
}

func (tc InvalidMissingAncestorReOrgTest) GetName() string {
	emptyTxsStatus := "False"
	if tc.EmptyTransactions {
		emptyTxsStatus = "True"
	}
	return fmt.Sprintf(
		"Invalid Missing Ancestor ReOrg, %s, EmptyTxs=%s, Invalid P%d",
		tc.InvalidField,
		emptyTxsStatus,
		tc.InvalidIndex,
	)
}

func (tc InvalidMissingAncestorReOrgTest) Execute(t *test.Env) {
	// Produce blocks before starting the test
	t.CLMock.ProduceBlocks(5, clmock.BlockProcessCallbacks{})

	// Save the common ancestor
	cA := t.CLMock.LatestPayloadBuilt

	// Slice to save the side B chain
	altChainPayloads := make([]*typ.ExecutableData, 0)

	// Append the common ancestor
	altChainPayloads = append(altChainPayloads, &cA)

	// Produce blocks but at the same time create an side chain which contains an invalid payload at some point (INV_P)
	// CommonAncestor◄─▲── P1 ◄─ P2 ◄─ P3 ◄─ ... ◄─ Pn
	//                 │
	//                 └── P1' ◄─ P2' ◄─ ... ◄─ INV_P ◄─ ... ◄─ Pn'
	t.CLMock.ProduceBlocks(tc.SidechainLength, clmock.BlockProcessCallbacks{

		OnPayloadProducerSelected: func() {
			// Function to send at least one transaction each block produced.
			// Empty Txs Payload with invalid stateRoot discovered an issue in geth sync, hence this is customizable.
			if !tc.EmptyTransactions {
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
		},
		OnGetPayload: func() {
			var (
				sidePayload *typ.ExecutableData
				err         error
			)
			// Insert extraData to ensure we deviate from the main payload, which contains empty extradata
			customizer := &helper.CustomPayloadData{
				ParentHash: &altChainPayloads[len(altChainPayloads)-1].BlockHash,
				ExtraData:  &([]byte{0x01}),
			}
			sidePayload, err = customizer.CustomizePayload(t.Rand, &t.CLMock.LatestPayloadBuilt)
			if err != nil {
				t.Fatalf("FAIL (%s): Unable to customize payload: %v", t.TestName, err)
			}
			if len(altChainPayloads) == tc.InvalidIndex {
				sidePayload, err = helper.GenerateInvalidPayload(t.Rand, sidePayload, tc.InvalidField)
				if err != nil {
					t.Fatalf("FAIL (%s): Unable to customize payload: %v", t.TestName, err)
				}
			}
			altChainPayloads = append(altChainPayloads, sidePayload)
		},
	})
	t.CLMock.ProduceSingleBlock(clmock.BlockProcessCallbacks{
		// Note: We perform the test in the middle of payload creation by the CL Mock, in order to be able to
		// re-org back into this chain and use the new payload without issues.
		OnGetPayload: func() {

			// Now let's send the side chain to the client using newPayload/sync
			for i := 1; i <= tc.SidechainLength; i++ {
				// Send the payload
				payloadValidStr := "VALID"
				if i == tc.InvalidIndex {
					payloadValidStr = "INVALID"
				} else if i > tc.InvalidIndex {
					payloadValidStr = "VALID with INVALID ancestor"
				}
				t.Logf("INFO (%s): Invalid chain payload %d (%s): %v", t.TestName, i, payloadValidStr, altChainPayloads[i].BlockHash)

				r := t.TestEngine.TestEngineNewPayload(altChainPayloads[i])
				p := t.TestEngine.TestEngineForkchoiceUpdated(&api.ForkchoiceStateV1{
					HeadBlockHash: altChainPayloads[i].BlockHash,
				}, nil, altChainPayloads[i].Timestamp)
				if i == tc.InvalidIndex {
					// If this is the first payload after the common ancestor, and this is the payload we invalidated,
					// then we have all the information to determine that this payload is invalid.
					r.ExpectStatus(test.Invalid)
					r.ExpectLatestValidHash(&altChainPayloads[i-1].BlockHash)
				} else if i > tc.InvalidIndex {
					// We have already sent the invalid payload, but the client could've discarded it.
					// In reality the CL will not get to this point because it will have already received the `INVALID`
					// response from the previous payload.
					// The node might save the parent as invalid, thus returning INVALID
					r.ExpectStatusEither(test.Accepted, test.Syncing, test.Invalid)
					if r.Status.Status == test.Accepted || r.Status.Status == test.Syncing {
						r.ExpectLatestValidHash(nil)
					} else if r.Status.Status == test.Invalid {
						r.ExpectLatestValidHash(&altChainPayloads[tc.InvalidIndex-1].BlockHash)
					}
				} else {
					// This is one of the payloads before the invalid one, therefore is valid.
					r.ExpectStatus(test.Valid)
					p.ExpectPayloadStatus(test.Valid)
					p.ExpectLatestValidHash(&altChainPayloads[i].BlockHash)
				}

			}

			// Resend the latest correct fcU
			r := t.TestEngine.TestEngineForkchoiceUpdated(&t.CLMock.LatestForkchoice, nil, t.CLMock.LatestPayloadBuilt.Timestamp)
			r.ExpectNoError()
			// After this point, the CL Mock will send the next payload of the canonical chain
		},
	})
}

// Attempt to re-org to a chain which at some point contains an unknown payload which is also invalid.
// Then reveal the invalid payload and expect that the client rejects it and rejects forkchoice updated calls to this chain.
type InvalidMissingAncestorReOrgSyncTest struct {
	test.BaseSpec
	// Index of the payload to invalidate, starting with 0 being the common ancestor.
	// Value must be greater than 0.
	InvalidIndex int
	// Field of the payload to invalidate (see helper module)
	InvalidField helper.InvalidPayloadBlockField
	// Whether to create payloads with empty transactions or not:
	// Used to test scenarios where the stateRoot is invalidated but its invalidation
	// goes unnoticed by the client because of the lack of transactions.
	EmptyTransactions bool
	// Height of the common ancestor in the proof-of-stake chain.
	// Value of 0 means the common ancestor is the terminal proof-of-work block.
	CommonAncestorHeight *big.Int
	// Amount of payloads to produce between the common ancestor and the head of the
	// proof-of-stake chain.
	DeviatingPayloadCount *big.Int
	// Whether the syncing client must re-org from a canonical chain.
	// If set to true, the client is driven through a valid canonical chain first,
	// and then the client is prompted to re-org to the invalid chain.
	// If set to false, the client is prompted to sync from the genesis
	// or start chain (if specified).
	ReOrgFromCanonical bool
}

func (s InvalidMissingAncestorReOrgSyncTest) WithMainFork(fork config.Fork) test.Spec {
	specCopy := s
	specCopy.MainFork = fork
	return specCopy
}

func (tc InvalidMissingAncestorReOrgSyncTest) GetName() string {
	emptyTxsStatus := "False"
	if tc.EmptyTransactions {
		emptyTxsStatus = "True"
	}
	canonicalReOrgStatus := "False"
	if tc.ReOrgFromCanonical {
		canonicalReOrgStatus = "True"
	}
	return fmt.Sprintf(
		"Invalid Missing Ancestor Syncing ReOrg, %s, EmptyTxs=%s, CanonicalReOrg=%s, Invalid P%d",
		tc.InvalidField,
		emptyTxsStatus,
		canonicalReOrgStatus,
		tc.InvalidIndex,
	)
}

func (tc InvalidMissingAncestorReOrgSyncTest) Execute(t *test.Env) {
	var (
		err             error
		secondaryClient *node.GethNode
	)
	// To allow having the invalid payload delivered via P2P, we need a second client to serve the payload
	starter := node.GethNodeEngineStarter{
		Config: node.GethNodeTestConfiguration{},
	}
	if tc.ReOrgFromCanonical {
		// If we are doing a re-org from canonical, we can add both nodes as peers from the start
		secondaryClient, err = starter.StartGethNode(t.T, t.TestContext, t.Genesis, t.ClientParams, t.ClientFiles, t.Engine)
	} else {
		secondaryClient, err = starter.StartGethNode(t.T, t.TestContext, t.Genesis, t.ClientParams, t.ClientFiles)
	}
	if err != nil {
		t.Fatalf("FAIL (%s): Unable to spawn a secondary client: %v", t.TestName, err)
	}

	t.CLMock.AddEngineClient(secondaryClient)
	secondaryTestClient := test.NewTestEngineClient(t, secondaryClient)

	if !tc.ReOrgFromCanonical {
		// Remove the original client so that it does not receive the payloads created on the canonical chain
		t.CLMock.RemoveEngineClient(t.Engine)
	}

	// Produce blocks before starting the test
	// Default is to produce 5 PoS blocks before the common ancestor
	cAHeight := 5
	if tc.CommonAncestorHeight != nil {
		cAHeight = int(tc.CommonAncestorHeight.Int64())
	}

	// Save the common ancestor
	if cAHeight == 0 {
		t.Fatalf("FAIL (%s): Invalid common ancestor height: %d", t.TestName, cAHeight)
	} else {
		t.CLMock.ProduceBlocks(cAHeight, clmock.BlockProcessCallbacks{})
	}

	// Amount of blocks to deviate starting from the common ancestor
	// Default is to deviate 10 payloads from the common ancestor
	n := 10
	if tc.DeviatingPayloadCount != nil {
		n = int(tc.DeviatingPayloadCount.Int64())
	}

	// Slice to save the side B chain
	altChainPayloads := make([]*typ.ExecutableData, 0)

	// Append the common ancestor
	cA := t.CLMock.LatestPayloadBuilt
	altChainPayloads = append(altChainPayloads, &cA)

	// Produce blocks but at the same time create an side chain which contains an invalid payload at some point (INV_P)
	// CommonAncestor◄─▲── P1 ◄─ P2 ◄─ P3 ◄─ ... ◄─ Pn
	//                 │
	//                 └── P1' ◄─ P2' ◄─ ... ◄─ INV_P ◄─ ... ◄─ Pn'
	t.Log("INFO: Starting canonical chain production")
	t.CLMock.ProduceBlocks(n, clmock.BlockProcessCallbacks{

		OnPayloadProducerSelected: func() {
			// Function to send at least one transaction each block produced.
			// Empty Txs Payload with invalid stateRoot discovered an issue in geth sync, hence this is customizable.
			if !tc.EmptyTransactions {
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
		},
		OnGetPayload: func() {
			var (
				sidePayload *typ.ExecutableData
				err         error
			)
			// Insert extraData to ensure we deviate from the main payload, which contains empty extradata
			pHash := altChainPayloads[len(altChainPayloads)-1].BlockHash
			customizer := &helper.CustomPayloadData{
				ParentHash: &pHash,
				ExtraData:  &([]byte{0x01}),
			}
			sidePayload, err = customizer.CustomizePayload(t.Rand, &t.CLMock.LatestPayloadBuilt)
			if err != nil {
				t.Fatalf("FAIL (%s): Unable to customize payload: %v", t.TestName, err)
			}
			if len(altChainPayloads) == tc.InvalidIndex {
				sidePayload, err = helper.GenerateInvalidPayload(t.Rand, sidePayload, tc.InvalidField)
				if err != nil {
					t.Fatalf("FAIL (%s): Unable to customize payload: %v", t.TestName, err)
				}
			}
			altChainPayloads = append(altChainPayloads, sidePayload)

			// TODO: This could be useful to try to produce an invalid block that has some invalid field not included in the ExecutableData
			sideBlock, err := typ.ExecutableDataToBlock(*sidePayload)
			if err != nil {
				t.Fatalf("FAIL (%s): Error converting payload to block: %v", t.TestName, err)
			}
			if len(altChainPayloads) == tc.InvalidIndex {
				var uncle *types.Block
				if tc.InvalidField == helper.InvalidOmmers {
					if unclePayload, ok := t.CLMock.ExecutedPayloadHistory[sideBlock.NumberU64()-1]; ok && unclePayload != nil {
						// Uncle is a PoS payload
						uncle, err = typ.ExecutableDataToBlock(*unclePayload)
						if err != nil {
							t.Fatalf("FAIL (%s): Unable to get uncle block: %v", t.TestName, err)
						}
					} else {
						panic("FAIL: Unable to get uncle block")
					}
				}
				// Invalidate fields not available in the ExecutableData
				sideBlock, err = helper.GenerateInvalidPayloadBlock(sideBlock, uncle, tc.InvalidField)
				if err != nil {
					t.Fatalf("FAIL (%s): Unable to customize payload block: %v", t.TestName, err)
				}
			}
		},
	})

	if !tc.ReOrgFromCanonical {
		// Add back the original client before side chain production
		t.CLMock.AddEngineClient(t.Engine)
	}

	t.Log("INFO: Starting side chain production")
	t.CLMock.ProduceSingleBlock(clmock.BlockProcessCallbacks{
		// Note: We perform the test in the middle of payload creation by the CL Mock, in order to be able to
		// re-org back into this chain and use the new payload without issues.
		OnGetPayload: func() {

			// Now let's send the side chain to the client using newPayload/sync
			for i := 1; i < n; i++ {
				// Send the payload
				payloadValidStr := "VALID"
				if i == tc.InvalidIndex {
					payloadValidStr = "INVALID"
				} else if i > tc.InvalidIndex {
					payloadValidStr = "VALID with INVALID ancestor"
				}
				payloadJs, _ := json.MarshalIndent(altChainPayloads[i], "", " ")
				t.Logf("INFO (%s): Invalid chain payload %d (%s):\n%s", t.TestName, i, payloadValidStr, payloadJs)

				if i < tc.InvalidIndex {
					p := altChainPayloads[i]
					r := secondaryTestClient.TestEngineNewPayload(p)
					r.ExpectationDescription = "Sent modified payload to secondary client, expected to be accepted"
					r.ExpectStatusEither(test.Valid, test.Accepted)

					s := secondaryTestClient.TestEngineForkchoiceUpdated(&api.ForkchoiceStateV1{
						HeadBlockHash: p.BlockHash,
					}, nil, p.Timestamp)
					s.ExpectationDescription = "Sent modified payload forkchoice updated to secondary client, expected to be accepted"
					s.ExpectAnyPayloadStatus(test.Valid, test.Syncing)

				} else {
					invalidBlock, err := typ.ExecutableDataToBlock(*altChainPayloads[i])
					if err != nil {
						t.Fatalf("FAIL (%s): TEST ISSUE - Failed to create block from payload: %v", t.TestName, err)
					}

					if err := secondaryClient.SetBlock(invalidBlock, altChainPayloads[i-1].Number, altChainPayloads[i-1].StateRoot); err != nil {
						t.Fatalf("FAIL (%s): TEST ISSUE - Failed to set invalid block: %v", t.TestName, err)
					}
					t.Logf("INFO (%s): Invalid block successfully set %d (%s): %v", t.TestName, i, payloadValidStr, invalidBlock.Hash())
				}
			}
			// Check that the second node has the correct head
			ctx, cancel := context.WithTimeout(t.TestContext, globals.RPCTimeout)
			defer cancel()
			head, err := secondaryClient.HeaderByNumber(ctx, nil)
			if err != nil {
				t.Fatalf("FAIL (%s): TEST ISSUE - Secondary Node unable to reatrieve latest header: %v", t.TestName, err)

			}
			if head.Hash() != altChainPayloads[n-1].BlockHash {
				t.Fatalf("FAIL (%s): TEST ISSUE - Secondary Node has invalid blockhash got %v want %v gotNum %v wantNum %d", t.TestName, head.Hash(), altChainPayloads[n-1].BlockHash, head.Number, altChainPayloads[n].Number)
			} else {
				t.Logf("INFO (%s): Secondary Node has correct block", t.TestName)
			}

			if !tc.ReOrgFromCanonical {
				// Add the main client as a peer of the secondary client so it is able to sync
				secondaryClient.AddPeer(t.Engine)

				ctx, cancel := context.WithTimeout(t.TimeoutContext, globals.RPCTimeout)
				defer cancel()
				l, err := t.Eth.BlockByNumber(ctx, nil)
				if err != nil {
					t.Fatalf("FAIL (%s): Unable to query main client for latest block: %v", t.TestName, err)
				}
				t.Logf("INFO (%s): Latest block on main client before sync: hash=%v, number=%d", t.TestName, l.Hash(), l.Number())
			}
			// If we are syncing through p2p, we need to keep polling until the client syncs the missing payloads
			for {
				r := t.TestEngine.TestEngineNewPayload(altChainPayloads[n])
				t.Logf("INFO (%s): Response from main client: %v", t.TestName, r.Status)
				s := t.TestEngine.TestEngineForkchoiceUpdated(&api.ForkchoiceStateV1{
					HeadBlockHash: altChainPayloads[n].BlockHash,
				}, nil, altChainPayloads[n].Timestamp)
				t.Logf("INFO (%s): Response from main client fcu: %v", t.TestName, s.Response.PayloadStatus)

				if r.Status.Status == test.Invalid {
					// We also expect that the client properly returns the LatestValidHash of the block on the
					// side chain that is immediately prior to the invalid payload (or zero if parent is PoW)
					var lvh common.Hash
					if cAHeight != 0 || tc.InvalidIndex != 1 {
						// Parent is NOT Proof of Work
						lvh = altChainPayloads[tc.InvalidIndex-1].BlockHash
					}
					r.ExpectLatestValidHash(&lvh)
					// Response on ForkchoiceUpdated should be the same
					s.ExpectPayloadStatus(test.Invalid)
					s.ExpectLatestValidHash(&lvh)
					break
				} else if test.PayloadStatus(r.Status.Status) == test.Valid {
					ctx, cancel := context.WithTimeout(t.TestContext, globals.RPCTimeout)
					defer cancel()
					latestBlock, err := t.Eth.BlockByNumber(ctx, nil)
					if err != nil {
						t.Fatalf("FAIL (%s): Unable to get latest block: %v", t.TestName, err)
					}

					// Print last n blocks, for debugging
					k := latestBlock.Number().Int64() - int64(n)
					if k < 0 {
						k = 0
					}
					for ; k <= latestBlock.Number().Int64(); k++ {
						ctx, cancel = context.WithTimeout(t.TestContext, globals.RPCTimeout)
						defer cancel()
						latestBlock, err := t.Eth.BlockByNumber(ctx, big.NewInt(k))
						if err != nil {
							t.Fatalf("FAIL (%s): Unable to get block %d: %v", t.TestName, k, err)
						}
						js, _ := json.MarshalIndent(latestBlock.Header(), "", "  ")
						t.Logf("INFO (%s): Block %d: %s", t.TestName, k, js)
					}

					t.Fatalf("FAIL (%s): Client returned VALID on an invalid chain: %v", t.TestName, r.Status)
				}

				select {
				case <-time.After(time.Second):
					continue
				case <-t.TimeoutContext.Done():
					t.Fatalf("FAIL (%s): Timeout waiting for main client to detect invalid chain", t.TestName)
				}
			}

			if !tc.ReOrgFromCanonical {
				// We need to send the canonical chain to the main client here
				for i := t.CLMock.FirstPoSBlockNumber.Uint64(); i <= t.CLMock.LatestExecutedPayload.Number; i++ {
					if payload, ok := t.CLMock.ExecutedPayloadHistory[i]; ok {
						r := t.TestEngine.TestEngineNewPayload(payload)
						r.ExpectStatus(test.Valid)
					}
				}
			}

			// Resend the latest correct fcU
			r := t.TestEngine.TestEngineForkchoiceUpdated(&t.CLMock.LatestForkchoice, nil, t.CLMock.LatestPayloadBuilt.Timestamp)
			r.ExpectNoError()
			// After this point, the CL Mock will send the next payload of the canonical chain
		},
	})

}
