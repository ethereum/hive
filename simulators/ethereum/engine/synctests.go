package main

import (
	"fmt"
	"math/big"
	"time"

	"github.com/ethereum/hive/hivesim"
)

var syncTests = []TestSpec{
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
func addSyncTestsToSuite(sim *hivesim.Simulation, suite *hivesim.Suite, tests []TestSpec) {
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
			// If the TestSpec specified a custom genesis file, use that instead.
			if currentTest.GenesisFile != "" {
				genesisPath = "./init/" + currentTest.GenesisFile
			}
			testFiles := hivesim.Params{"/genesis.json": genesisPath}
			// Calculate and set the TTD for this test
			ttd := calcRealTTD(genesisPath, currentTest.TTD)
			newParams := clientEnv.Set("HIVE_TERMINAL_TOTAL_DIFFICULTY", fmt.Sprintf("%d", ttd))
			if currentTest.ChainFile != "" {
				// We are using a Proof of Work chain file, remove all clique-related settings
				// TODO: Nethermind still requires HIVE_MINER for the Engine API
				// delete(newParams, "HIVE_MINER")
				delete(newParams, "HIVE_CLIQUE_PRIVATEKEY")
				delete(newParams, "HIVE_CLIQUE_PERIOD")
				// Add the new file to be loaded as chain.rlp
			}
			for _, variant := range clientSyncVariantGenerator.Configure(big.NewInt(ttd), genesisPath, currentTest.ChainFile) {
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

						timeout := DefaultTestCaseTimeout
						// If a TestSpec specifies a timeout, use that instead
						if currentTest.TimeoutSeconds != 0 {
							timeout = time.Second * time.Duration(currentTest.TimeoutSeconds)
						}

						// Prepare sync client parameters
						syncClientParams := newParams.Copy()
						for k, v := range variant.SyncClientConfig {
							syncClientParams = syncClientParams.Set(k, v)
						}

						// Run the test case
						RunTest(currentTest.Name, big.NewInt(ttd), currentTest.SlotsToSafe, currentTest.SlotsToFinalized, timeout, t, c, currentTest.Run, syncClientParams, testFiles.Copy())
					},
				})
			}
		}
	}

}

// Client Sync tests
func postMergeSync(t *TestEnv) {
	// Launch another client after the PoS transition has happened in the main client.
	// Sync should eventually happen without issues.
	t.CLMock.waitForTTD()

	// Speed up block production
	t.CLMock.PayloadProductionClientDelay = time.Millisecond * 100

	// Also set the transition payload timestamp to 500 seconds before now
	// This is done in order to try to not create payloads too old, nor into the future.
	t.CLMock.TransitionPayloadTimestamp = big.NewInt(time.Now().Unix() - 500)

	// Produce some blocks
	t.CLMock.produceBlocks(500, BlockProcessCallbacks{})

	// Set the Bootnode
	enode, err := t.Engine.EnodeURL()
	if err != nil {
		t.Fatalf("FAIL (%s): Unable to obtain bootnode: %v", t.TestName, err)
	}
	newParams := t.ClientParams.Set("HIVE_BOOTNODE", fmt.Sprintf("%s", enode))
	newParams = newParams.Set("HIVE_MINER", "")

	hc, secondaryEngine, err := t.StartClient(t.Client.Type, newParams, t.MainTTD())
	if err != nil {
		t.Fatalf("FAIL (%s): Unable to spawn a secondary client: %v", t.TestName, err)
	}
	secondaryEngineTest := NewTestEngineClient(t, secondaryEngine)
	t.CLMock.AddEngineClient(t.T, hc, t.MainTTD())

	for {
		select {
		case <-t.Timeout:
			t.Fatalf("FAIL (%s): Test timeout", t.TestName)
		default:
		}

		// CL continues building blocks on top of the PoS chain
		t.CLMock.produceSingleBlock(BlockProcessCallbacks{})

		// When the main client syncs, the test passes
		latestHeader, err := secondaryEngineTest.Engine.Eth.HeaderByNumber(t.Ctx(), nil)
		if err != nil {
			t.Fatalf("FAIL (%s): Unable to obtain latest header: %v", t.TestName, err)
		}
		if t.CLMock.LatestHeader != nil && latestHeader.Hash() == t.CLMock.LatestHeader.Hash() {
			t.Logf("INFO (%v): Client (%v) is now synced to latest PoS block: %v", t.TestName, hc.Container, latestHeader.Hash())
			break
		}
	}

}

// Performs a test where sync is done incrementally by sending incremental newPayload/fcU calls
func incrementalPostMergeSync(t *TestEnv) {
	// Launch another client after the PoS transition has happened in the main client.
	// Sync should eventually happen without issues.
	t.CLMock.waitForTTD()

	var (
		N uint64 = 500 // Total number of PoS blocks
		S uint64 = 5   // Number of incremental steps to sync
	)

	// Speed up block production
	t.CLMock.PayloadProductionClientDelay = time.Millisecond * 100

	// Also set the transition payload timestamp to 500 seconds before now.
	// This is done in order to try to not create payloads too old, nor into the future.
	t.CLMock.TransitionPayloadTimestamp = big.NewInt(time.Now().Unix() - int64(N))

	// Produce some blocks
	t.CLMock.produceBlocks(int(N), BlockProcessCallbacks{})

	// Set the Bootnode
	enode, err := t.Engine.EnodeURL()
	if err != nil {
		t.Fatalf("FAIL (%s): Unable to obtain bootnode: %v", t.TestName, err)
	}
	newParams := t.ClientParams.Set("HIVE_BOOTNODE", fmt.Sprintf("%s", enode))
	newParams = newParams.Set("HIVE_MINER", "")

	_, secondaryEngine, err := t.StartClient(t.Client.Type, newParams, t.MainTTD())
	if err != nil {
		t.Fatalf("FAIL (%s): Unable to spawn a secondary client: %v", t.TestName, err)
	}
	secondaryEngineTest := NewTestEngineClient(t, secondaryEngine)
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
		secondaryEngineTest.TestEngineNewPayloadV1(&payload)
		secondaryEngineTest.TestEngineForkchoiceUpdatedV1(&ForkchoiceStateV1{
			HeadBlockHash: payload.BlockHash,
		}, nil)
		for {
			b, err := secondaryEngine.Eth.BlockByNumber(secondaryEngine.Ctx(), nil)
			if err != nil {
				t.Fatalf("FAIL (%s): Error trying to get latest block: %v", t.TestName, err)
			}
			if b.Hash() == payload.BlockHash {
				break
			}
			select {
			case <-time.After(time.Second):
			case <-t.Timeout:
				t.Fatalf("FAIL (%s): Timeout waiting for client to sync", t.TestName)
			}
		}
	}

}
