package main

import (
	"fmt"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/beacon"
	"github.com/ethereum/hive/hivesim"
)

type SecondaryClient struct {
	// Chain file to initialize the secondary client
	ChainFile string

	// Terminal Total Difficulty of this client.
	// Default: Main Client's TTD
	TTD int64

	// Whether to broadcast a ForkchoiceUpdated directive to all clients that
	// points to this client's HeadBlockHash
	ForkchoiceUpdate bool
}

type MergeTestSpec struct {
	// Name of the test
	Name string

	// Brief description of the test
	About string

	// TerminalTotalDifficulty delta value.
	// Actual TTD is genesis.Difficulty + this value
	// Default: 0
	TTD int64

	// Test maximum execution time until a timeout is raised.
	// Default: 60 seconds
	TimeoutSeconds int

	// Genesis file to be used for all clients launched during test
	// Default: genesis.json (init/genesis.json)
	GenesisFile string

	// Chain file to initialize the main client.
	MainChainFile string

	// All secondary clients to be started during the tests with their respective chain files
	SecondaryClients []SecondaryClient
}

var mergeTestSpecs = []MergeTestSpec{
	{
		Name:          "Re-org to Higher Total Difficulty Chain",
		TTD:           393120,
		MainChainFile: "blocks_2_td_393120.rlp",
		SecondaryClients: []SecondaryClient{
			{
				ChainFile:        "blocks_2_td_393504.rlp",
				ForkchoiceUpdate: true,
			},
		},
	},
	/* TODOs:
	- reorg to higher block number, still with a valid TTD
	- reorg to lower block number, still with a valid TTD
	- reorg to a block with uncles
	- reorg to invalid block (bad state root or similar)
	- reorg multiple times to multiple client chains (5+)
	-
	*/
}

var mergeTests = GenerateMergeTests()

func GenerateMergeTests() []TestSpec {
	testSpecs := make([]TestSpec, 0)
	for _, mergeTest := range mergeTestSpecs {
		testSpecs = append(testSpecs, MergeTestGenerator(mergeTest))
	}
	return testSpecs
}

func MergeTestGenerator(mergeTestSpec MergeTestSpec) TestSpec {
	runFunc := func(t *TestEnv) {
		// These tests start with whichever chain was included in TestSpec.ChainFile

		// The first client waits for TTD, which ideally should be reached immediately using loaded chain
		t.CLMock.waitForTTD()

		// At this point, Head must be main client's HeadBlockHash, but this can change depending on the
		// secondary clients
		mustHeadHash := t.CLMock.LatestForkchoice.HeadBlockHash

		// Start a secondary clients with alternative PoW chains
		// Exact same TotalDifficulty, different hashes, so reorg is necessary.
		for _, secondaryClient := range mergeTestSpec.SecondaryClients {
			altChainClient := t.StartAlternativeChainClient(secondaryClient.ChainFile)
			if secondaryClient.ForkchoiceUpdate {
				// Broadcast ForkchoiceUpdated to the HeadBlockHash of this client
				altChainHeadHash := altChainClient.GetHeadBlockHash()
				altChainFcU := beacon.ForkchoiceStateV1{
					HeadBlockHash:      altChainHeadHash,
					SafeBlockHash:      altChainHeadHash,
					FinalizedBlockHash: altChainHeadHash,
				}

				// Send new ForkchoiceUpdated to the original client and wait for it to change to it
				resp, err := t.Engine.EngineForkchoiceUpdatedV1(t.Engine.Ctx(), &altChainFcU, nil)
				if err != nil {
					t.Fatalf("FAIL (%s): Re-org to alternative PoW chain resulted in error", t.TestName)
				}
				t.Logf("INFO (%s): Response from the client when attempting to reorg to alt PoW chain: %v", t.TestName, resp)

				mustHeadHash = altChainHeadHash
			}
		}

		for {
			select {
			case <-time.After(time.Second):
				// Check whether we have sync'd
				header, err := t.Eth.HeaderByNumber(t.Ctx(), nil)
				if err != nil {
					t.Fatalf("FAIL (%s): Unable to get latest header: %v", t.TestName, err)
				}
				if header.Hash() == mustHeadHash {
					t.Logf("INFO (%s): We are sync'd to the alternative PoW chain", t.TestName)
					return
				}
			case <-t.Timeout:
				t.Fatalf("FAIL (%s): Timeout while waiting for sync on the alternative PoW chain", t.TestName)
			}
		}
	}

	return TestSpec{
		Name:           mergeTestSpec.Name,
		About:          mergeTestSpec.About,
		Run:            runFunc,
		TTD:            mergeTestSpec.TTD,
		TimeoutSeconds: mergeTestSpec.TimeoutSeconds,
		GenesisFile:    mergeTestSpec.GenesisFile,
		ChainFile:      mergeTestSpec.MainChainFile,
	}
}

type AlternativeChainClient struct {
	*TestEnv
	Engine *EngineClient
}

func (t *TestEnv) StartAlternativeChainClient(chainPath string) *AlternativeChainClient {
	clientTypes, err := t.Sim.ClientTypes()
	if err != nil || len(clientTypes) == 0 {
		t.Fatalf("FAIL (%s): Unable to obtain client types: %v", t.TestName, err)
	}
	// Let's use the first just for now
	altClient := clientTypes[0]

	altClientFiles := t.ClientFiles.Set("/chain.rlp", "./chains/"+chainPath)

	enode, err := t.Engine.EnodeURL()
	if err != nil {
		t.Fatalf("FAIL (%s): Unable to obtain bootnode", err)
	}
	altClientParams := t.ClientParams.Set("HIVE_BOOTNODE", fmt.Sprintf("%s", enode))

	c := t.T.StartClient(altClient.Name, altClientParams, hivesim.WithStaticFiles(altClientFiles))
	ec := NewEngineClient(t.T, c)

	altChainClient := &AlternativeChainClient{
		TestEnv: t,
		Engine:  ec,
	}
	return altChainClient
}

func (c *AlternativeChainClient) GetHeadBlockHash() common.Hash {
	latestHeader, err := c.Engine.Eth.HeaderByNumber(c.Engine.Ctx(), nil)
	if err != nil {
		c.TestEnv.Fatalf("FAIL (%s): Unable to get alternative client head block hash: %v", c.TestEnv.TestName, err)
	}
	return latestHeader.Hash()
}

func (c *AlternativeChainClient) Close() {
	c.Engine.Close()
}
