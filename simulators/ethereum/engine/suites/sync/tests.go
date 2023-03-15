package suite_sync

import (
	"context"
	"fmt"
	"math/big"
	"time"

	api "github.com/ethereum/go-ethereum/beacon/engine"
	"github.com/ethereum/hive/hivesim"
	"github.com/ethereum/hive/simulators/ethereum/engine/client/hive_rpc"
	"github.com/ethereum/hive/simulators/ethereum/engine/clmock"
	"github.com/ethereum/hive/simulators/ethereum/engine/globals"
	"github.com/ethereum/hive/simulators/ethereum/engine/helper"
	"github.com/ethereum/hive/simulators/ethereum/engine/test"
)

var Tests = []test.Spec{
	{
		Name:           "Sync Client Post Merge",
		Run:            postMergeSync,
		TimeoutSeconds: 180,
		ChainFile:      "blocks_1024_td_135112316.rlp",
		TTD:            135112316,
	},
	{
		Name:           "Incremental Post Merge Sync",
		Run:            incrementalPostMergeSync,
		TimeoutSeconds: 180,
		ChainFile:      "blocks_1024_td_135112316.rlp",
		TTD:            135112316,
	},
}

// Routine that adds all sync tests to a test suite
func AddSyncTestsToSuite(sim *hivesim.Simulation, suite *hivesim.Suite, tests []test.Spec) {
	clientDefs, err := sim.ClientTypes()
	if err != nil {
		panic(err)
	}
	for _, currentTest := range tests {
		for _, clientDef := range clientDefs {
			clientSyncVariantGenerator, ok := ClientToSyncVariantGenerator[clientDef.Name]
			if !ok {
				clientSyncVariantGenerator = DefaultSyncVariantGenerator{}
			}
			currentTest := currentTest
			genesisPath := "./init/genesis.json"
			// If the test.Spec specified a custom genesis file, use that instead.
			if currentTest.GenesisFile != "" {
				genesisPath = "./init/" + currentTest.GenesisFile
			}
			testFiles := hivesim.Params{"/genesis.json": genesisPath}
			// Calculate and set the TTD for this test
			genesis := helper.LoadGenesis(genesisPath)
			ttd := helper.CalculateRealTTD(&genesis, currentTest.TTD)
			newParams := globals.DefaultClientEnv.Set("HIVE_TERMINAL_TOTAL_DIFFICULTY", fmt.Sprintf("%d", ttd))
			if currentTest.ChainFile != "" {
				// We are using a Proof of Work chain file, remove all clique-related settings
				// TODO: Nethermind still requires HIVE_MINER for the Engine API
				// delete(newParams, "HIVE_MINER")
				delete(newParams, "HIVE_CLIQUE_PRIVATEKEY")
				delete(newParams, "HIVE_CLIQUE_PERIOD")
				// Add the new file to be loaded as chain.rlp
			}
			for _, variant := range clientSyncVariantGenerator.Configure(big.NewInt(ttd), genesisPath, currentTest.ChainFile) {
				variant := variant
				clientDef := clientDef
				suite.Add(hivesim.TestSpec{
					Name:        fmt.Sprintf("%s (%s, sync/%s)", currentTest.Name, clientDef.Name, variant.Name),
					Description: currentTest.About,
					Run: func(t *hivesim.T) {

						mainClientParams := newParams.Copy()
						for k, v := range variant.MainClientConfig {
							mainClientParams = mainClientParams.Set(k, v)
						}
						mainClientFiles := testFiles.Copy()
						if currentTest.ChainFile != "" {
							mainClientFiles = mainClientFiles.Set("/chain.rlp", "./chains/"+currentTest.ChainFile)
						}
						c := t.StartClient(clientDef.Name, mainClientParams, hivesim.WithStaticFiles(mainClientFiles))

						t.Logf("Start test (%s, %s, sync/%s)", c.Type, currentTest.Name, variant.Name)
						defer func() {
							t.Logf("End test (%s, %s, sync/%s)", c.Type, currentTest.Name, variant.Name)
						}()

						timeout := globals.DefaultTestCaseTimeout
						// If a test.Spec specifies a timeout, use that instead
						if currentTest.TimeoutSeconds != 0 {
							timeout = time.Second * time.Duration(currentTest.TimeoutSeconds)
						}

						// Prepare sync client parameters
						syncClientParams := newParams.Copy()
						for k, v := range variant.SyncClientConfig {
							syncClientParams = syncClientParams.Set(k, v)
						}

						// Run the test case
						test.Run(currentTest, big.NewInt(ttd), timeout, t, c, &genesis, syncClientParams, testFiles.Copy())
					},
				})
			}
		}
	}

}

// Client Sync tests
func postMergeSync(t *test.Env) {
	// Launch another client after the PoS transition has happened in the main client.
	// Sync should eventually happen without issues.
	t.CLMock.WaitForTTD()

	// Speed up block production
	t.CLMock.PayloadProductionClientDelay = 0

	// Also set the transition payload timestamp to 500 seconds before now
	// This is done in order to try to not create payloads too old, nor into the future.
	t.CLMock.TransitionPayloadTimestamp = big.NewInt(time.Now().Unix() - 500)

	// Produce some blocks
	t.CLMock.ProduceBlocks(500, clmock.BlockProcessCallbacks{})

	// Reset block production delay
	t.CLMock.PayloadProductionClientDelay = time.Second

	secondaryEngine, err := hive_rpc.HiveRPCEngineStarter{}.StartClient(t.T, t.TestContext, t.Genesis, t.ClientParams.Set("HIVE_MINER", ""), t.ClientFiles, t.Engine)
	if err != nil {
		t.Fatalf("FAIL (%s): Unable to spawn a secondary client: %v", t.TestName, err)
	}
	secondaryEngineTest := test.NewTestEngineClient(t, secondaryEngine)
	t.CLMock.AddEngineClient(secondaryEngine)

	for {
		select {
		case <-t.TimeoutContext.Done():
			t.Fatalf("FAIL (%s): Test timeout", t.TestName)
		default:
		}

		// CL continues building blocks on top of the PoS chain
		t.CLMock.ProduceSingleBlock(clmock.BlockProcessCallbacks{})

		// When the main client syncs, the test passes
		ctx, cancel := context.WithTimeout(t.TestContext, globals.RPCTimeout)
		defer cancel()
		latestHeader, err := secondaryEngineTest.Engine.HeaderByNumber(ctx, nil)
		if err != nil {
			t.Fatalf("FAIL (%s): Unable to obtain latest header: %v", t.TestName, err)
		}
		if t.CLMock.LatestHeader != nil && latestHeader.Hash() == t.CLMock.LatestHeader.Hash() {
			t.Logf("INFO (%v): Client (%v) is now synced to latest PoS block: %v", t.TestName, secondaryEngine.ID(), latestHeader.Hash())
			break
		}
	}

}

// Performs a test where sync is done incrementally by sending incremental newPayload/fcU calls
func incrementalPostMergeSync(t *test.Env) {
	// Launch another client after the PoS transition has happened in the main client.
	// Sync should eventually happen without issues.
	t.CLMock.WaitForTTD()

	var (
		N uint64 = 500 // Total number of PoS blocks
		S uint64 = 5   // Number of incremental steps to sync
	)

	// Speed up block production
	t.CLMock.PayloadProductionClientDelay = 0

	// Also set the transition payload timestamp to 500 seconds before now.
	// This is done in order to try to not create payloads too old, nor into the future.
	t.CLMock.TransitionPayloadTimestamp = big.NewInt(time.Now().Unix() - int64(N))

	// Produce some blocks
	t.CLMock.ProduceBlocks(int(N), clmock.BlockProcessCallbacks{})

	// Reset block production delay
	t.CLMock.PayloadProductionClientDelay = time.Second

	secondaryEngine, err := hive_rpc.HiveRPCEngineStarter{}.StartClient(t.T, t.TestContext, t.Genesis, t.ClientParams.Set("HIVE_MINER", ""), t.ClientFiles, t.Engine)
	if err != nil {
		t.Fatalf("FAIL (%s): Unable to spawn a secondary client: %v", t.TestName, err)
	}
	secondaryEngineTest := test.NewTestEngineClient(t, secondaryEngine)
	// t.CLMock.AddEngineClient(t.T, hc, t.MainTTD())

	if N != uint64(len(t.CLMock.ExecutedPayloadHistory)) {
		t.Fatalf("FAIL (%s): Unexpected number of payloads produced: %d != %d", t.TestName, len(t.CLMock.ExecutedPayloadHistory), N)
	}

	firstPoSBlockNumber := t.CLMock.FirstPoSBlockNumber.Uint64()
	for i := uint64(firstPoSBlockNumber + (N / S) - 1); i <= (N + firstPoSBlockNumber); i += N / S {
		payload, ok := t.CLMock.ExecutedPayloadHistory[i]
		if !ok {
			t.Fatalf("FAIL (%s): TEST ISSUE - Payload not found: %d", t.TestName, i)
		}
		for {
			secondaryEngineTest.TestEngineNewPayloadV1(payload)
			secondaryEngineTest.TestEngineForkchoiceUpdatedV1(&api.ForkchoiceStateV1{
				HeadBlockHash: payload.BlockHash,
			}, nil)
			ctx, cancel := context.WithTimeout(t.TestContext, globals.RPCTimeout)
			defer cancel()
			b, err := secondaryEngine.BlockByNumber(ctx, nil)
			if err != nil {
				t.Fatalf("FAIL (%s): Error trying to get latest block: %v", t.TestName, err)
			}
			if b.Hash() == payload.BlockHash {
				break
			}
			select {
			case <-time.After(time.Second):
			case <-t.TimeoutContext.Done():
				t.Fatalf("FAIL (%s): Timeout waiting for client to sync", t.TestName)
			}
		}
	}

}
