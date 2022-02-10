package main

import (
	"fmt"
	"time"

	"github.com/ethereum/go-ethereum/core/types"
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

	// TODO: Expected FcU outcome, could be "SYNCING", "VALID", etc..
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

/*
	Chains Included:
		- blocks_1_td_196608.rlp
			Block 1, ab5d6faf8d4460e2f58cff55055411bd66c2bbce7436d54bbc962576ce84605f, difficulty: 196608
			Chain Total Difficulty 196608

		- blocks_1_td_196704.rlp
			Block 1, f37dca584c0f6abde6e9c0bb486ac42a8ac091156ab97aeb26136ee37a3aaef7, difficulty: 196704
			Chain Total Difficulty 196704

		- blocks_2_td_393120.rlp
			Block 1, ab5d6faf8d4460e2f58cff55055411bd66c2bbce7436d54bbc962576ce84605f, difficulty: 196608
			Block 2, 6e98e14794478cbfe8feaa24d4463b1a4e5d3743a66a49a851a3029d9ff3de60, difficulty: 196512
			Chain Total Difficulty 393120

		- blocks_2_td_393504.rlp
			Block 1, f37dca584c0f6abde6e9c0bb486ac42a8ac091156ab97aeb26136ee37a3aaef7, difficulty: 196704
			Block 2, ee5dbba4ccfdc75800bf98804be90d2ffe7fa9b2dce37b6fe31c891ca5fb87b8, difficulty: 196800
			Chain Total Difficulty 393504

	Chains were generated using the following command:
		`hivechain generate -genesis ./init/genesis.json -mine -length 2|1`.
*/
var mergeTestSpecs = []MergeTestSpec{
	{
		Name:          "Single Block Re-org to Higher-Total-Difficulty Chain, Equal Height",
		TTD:           196608,
		MainChainFile: "blocks_1_td_196608.rlp",
		SecondaryClients: []SecondaryClient{
			{
				ChainFile:        "blocks_1_td_196704.rlp",
				ForkchoiceUpdate: true,
			},
		},
	},
	{
		Name:          "Single Block Re-org to Lower-Total-Difficulty Chain, Equal Height",
		TTD:           196608,
		MainChainFile: "blocks_1_td_196704.rlp",
		SecondaryClients: []SecondaryClient{
			{
				ChainFile:        "blocks_1_td_196608.rlp",
				ForkchoiceUpdate: true,
			},
		},
	},
	{
		Name:          "Two Block Re-org to Higher-Total-Difficulty Chain, Equal Height",
		TTD:           393120,
		MainChainFile: "blocks_2_td_393120.rlp",
		SecondaryClients: []SecondaryClient{
			{
				ChainFile:        "blocks_2_td_393504.rlp",
				ForkchoiceUpdate: true,
			},
		},
	},
	{
		Name:          "Two Block Re-org to Lower-Total-Difficulty Chain, Equal Height",
		TTD:           393120,
		MainChainFile: "blocks_2_td_393504.rlp",
		SecondaryClients: []SecondaryClient{
			{
				ChainFile:        "blocks_2_td_393120.rlp",
				ForkchoiceUpdate: true,
			},
		},
	},
	{
		Name:          "Two Block Re-org to Higher-Height Chain",
		TTD:           196704,
		MainChainFile: "blocks_1_td_196704.rlp",
		SecondaryClients: []SecondaryClient{
			{
				ChainFile:        "blocks_2_td_393120.rlp",
				ForkchoiceUpdate: true,
			},
		},
	},
	{
		Name:          "Two Block Re-org to Lower-Height Chain",
		TTD:           196704,
		MainChainFile: "blocks_2_td_393120.rlp",
		SecondaryClients: []SecondaryClient{
			{
				ChainFile:        "blocks_1_td_196704.rlp",
				ForkchoiceUpdate: true,
			},
		},
	},

	/* TODOs:
	- reorg to a block with uncles
	- reorg to invalid block (bad state root or similar)
	- reorg multiple times to multiple client chains (5+)
	-
	*/
}

var mergeTests = func() []TestSpec {
	testSpecs := make([]TestSpec, 0)
	for _, mergeTest := range mergeTestSpecs {
		testSpecs = append(testSpecs, GenerateMergeTestSpec(mergeTest))
	}
	return testSpecs
}()

func GenerateMergeTestSpec(mergeTestSpec MergeTestSpec) TestSpec {
	runFunc := func(t *TestEnv) {
		// These tests start with whichever chain was included in TestSpec.ChainFile

		// The first client waits for TTD, which ideally should be reached immediately using loaded chain
		t.CLMock.waitForTTD()

		// At this point, Head must be main client's HeadBlockHash, but this can change depending on the
		// secondary clients
		mustHeadHash := t.CLMock.LatestForkchoice.HeadBlockHash

		enode, err := t.Engine.EnodeURL()
		if err != nil {
			t.Fatalf("FAIL (%s): Unable to obtain main client enode", err)
		}

		// Start a secondary clients with alternative PoW chains
		// Exact same TotalDifficulty, different hashes, so reorg is necessary.
		for _, secondaryClient := range mergeTestSpec.SecondaryClients {
			// TODO: Check if the secondary has a configured TTD, use that instead.
			altChainClient := secondaryClient.StartAlternativeChainClient(t, enode)
			t.CLMock.AddEngineClient(t.T, altChainClient.Client)
			if secondaryClient.ForkchoiceUpdate {
				// Broadcast ForkchoiceUpdated to the HeadBlockHash of this client
				altChainHead := altChainClient.GetHeadHeader()

				// Re-Org latest cl.LatestForkchoice
				t.CLMock.LatestForkchoice.HeadBlockHash = altChainHead.Hash()
				t.CLMock.LatestForkchoice.SafeBlockHash = altChainHead.Hash()
				t.CLMock.LatestForkchoice.FinalizedBlockHash = altChainHead.Hash()

				// Send new ForkchoiceUpdated to all clients
				t.CLMock.broadcastLatestForkchoice()

				mustHeadHash = altChainHead.Hash()
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
	Client *hivesim.Client
	Engine *EngineClient
}

func (spec SecondaryClient) StartAlternativeChainClient(t *TestEnv, enode string) *AlternativeChainClient {
	clientTypes, err := t.Sim.ClientTypes()
	if err != nil || len(clientTypes) == 0 {
		t.Fatalf("FAIL (%s): Unable to obtain client types: %v", t.TestName, err)
	}
	// Let's use the first just for now
	altClient := clientTypes[0]

	altClientFiles := t.ClientFiles.Set("/chain.rlp", "./chains/"+spec.ChainFile)

	altClientParams := t.ClientParams.Set("HIVE_BOOTNODE", enode)

	if spec.TTD != 0 {
		ttd := calcRealTTD(t.ClientFiles["/genesis.json"], spec.TTD)
		altClientParams = altClientParams.Set("HIVE_TERMINAL_TOTAL_DIFFICULTY", fmt.Sprintf("%d", ttd))
	}

	c := t.T.StartClient(altClient.Name, altClientParams, hivesim.WithStaticFiles(altClientFiles))
	ec := NewEngineClient(t.T, c)

	altChainClient := &AlternativeChainClient{
		TestEnv: t,
		Client:  c,
		Engine:  ec,
	}
	return altChainClient
}

func (c *AlternativeChainClient) GetHeadHeader() *types.Header {
	latestHeader, err := c.Engine.Eth.HeaderByNumber(c.Engine.Ctx(), nil)
	if err != nil {
		c.TestEnv.Fatalf("FAIL (%s): Unable to get alternative client head block hash: %v", c.TestEnv.TestName, err)
	}
	return latestHeader
}

func (c *AlternativeChainClient) Close() {
	c.Engine.Close()
}
