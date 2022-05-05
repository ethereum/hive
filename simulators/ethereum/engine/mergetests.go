package main

import (
	"fmt"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/hive/hivesim"
)

type SecondaryClientSpec struct {
	// Chain file to initialize the secondary client
	ChainFile string

	// Terminal Total Difficulty of this client.
	// Default: Main Client's TTD
	TTD int64

	// Whether the PoS chain should be built on top of this secondary client
	BuildPoSChainOnTop bool

	// Whether the main client shall sync to this secondary client or not.
	MainClientShallSync bool

	// TODO: Expected FcU outcome, could be "SYNCING", "VALID", etc..
}

type SecondaryClientSpecs []SecondaryClientSpec

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

	// Amount of seconds to keep checking that the main client does not switch chains.
	// Default: 0 seconds
	KeepCheckingSeconds int

	// Genesis file to be used for all clients launched during test
	// Default: genesis.json (init/genesis.json)
	GenesisFile string

	// Chain file to initialize the main client.
	MainChainFile string

	// Whether or not to send a forkchoiceUpdated directive on the main client before any attempts to re-org
	// to secondary clients happen.
	SkipMainClientFcU bool

	// Whether or not to wait for TTD to be reached by the main client
	SkipMainClientTTDWait bool

	// All secondary clients to be started during the tests with their respective chain files
	SecondaryClientSpecs SecondaryClientSpecs
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

		- blocks_1024_td_135112316.rlp
			Block 1, 2a9b7a5a209a58f672fa23a3ad9c831958d37dd74a960f69c0075b5c92be457e, difficulty: 196416
			* Details of every block use `hivechain print`
			Chain Total Difficulty 135112316

		- blocks_1_td_196416.rlp
			Block 1, 2a9b7a5a209a58f672fa23a3ad9c831958d37dd74a960f69c0075b5c92be457e, difficulty: 196416
			Chain Total Difficulty 196416

	Chains were generated using the following command:
		`hivechain generate -genesis ./init/genesis.json -mine -length 2|1`.
*/
var mergeTestSpecs = []MergeTestSpec{
	{
		Name:          "Single Block Re-org to Higher-Total-Difficulty Chain, Equal Height",
		TTD:           196608,
		MainChainFile: "blocks_1_td_196608.rlp",
		SecondaryClientSpecs: []SecondaryClientSpec{
			{
				ChainFile:           "blocks_1_td_196704.rlp",
				BuildPoSChainOnTop:  true,
				MainClientShallSync: true,
			},
		},
	},
	{
		Name:          "Single Block Re-org to Lower-Total-Difficulty Chain, Equal Height",
		TTD:           196608,
		MainChainFile: "blocks_1_td_196704.rlp",
		SecondaryClientSpecs: []SecondaryClientSpec{
			{
				ChainFile:           "blocks_1_td_196608.rlp",
				BuildPoSChainOnTop:  true,
				MainClientShallSync: true,
			},
		},
	},
	{
		Name:          "Two Block Re-org to Higher-Total-Difficulty Chain, Equal Height",
		TTD:           393120,
		MainChainFile: "blocks_2_td_393120.rlp",
		SecondaryClientSpecs: []SecondaryClientSpec{
			{
				ChainFile:           "blocks_2_td_393504.rlp",
				BuildPoSChainOnTop:  true,
				MainClientShallSync: true,
			},
		},
	},
	{
		Name:          "Two Block Re-org to Lower-Total-Difficulty Chain, Equal Height",
		TTD:           393120,
		MainChainFile: "blocks_2_td_393504.rlp",
		SecondaryClientSpecs: []SecondaryClientSpec{
			{
				ChainFile:           "blocks_2_td_393120.rlp",
				BuildPoSChainOnTop:  true,
				MainClientShallSync: true,
			},
		},
	},
	{
		Name:          "Two Block Re-org to Higher-Height Chain",
		TTD:           196704,
		MainChainFile: "blocks_1_td_196704.rlp",
		SecondaryClientSpecs: []SecondaryClientSpec{
			{
				ChainFile:           "blocks_2_td_393120.rlp",
				BuildPoSChainOnTop:  true,
				MainClientShallSync: true,
			},
		},
	},
	{
		Name:          "Two Block Re-org to Lower-Height Chain",
		TTD:           196704,
		MainChainFile: "blocks_2_td_393120.rlp",
		SecondaryClientSpecs: []SecondaryClientSpec{
			{
				ChainFile:           "blocks_1_td_196704.rlp",
				BuildPoSChainOnTop:  true,
				MainClientShallSync: true,
			},
		},
	},
	{
		Name:                "Halt following PoW chain",
		TTD:                 196608,
		MainChainFile:       "blocks_1_td_196608.rlp",
		SkipMainClientFcU:   true,
		TimeoutSeconds:      120,
		KeepCheckingSeconds: 60,
		SecondaryClientSpecs: []SecondaryClientSpec{
			{
				TTD:                 393120,
				ChainFile:           "blocks_2_td_393120.rlp",
				BuildPoSChainOnTop:  false,
				MainClientShallSync: false,
			},
		},
	},
	{
		Name:                  "Long PoW Chain Sync",
		TTD:                   135112316,
		MainChainFile:         "blocks_1_td_196416.rlp",
		SkipMainClientFcU:     true,
		SkipMainClientTTDWait: true,
		TimeoutSeconds:        300,
		SecondaryClientSpecs: []SecondaryClientSpec{
			{
				ChainFile:           "blocks_1024_td_135112316.rlp",
				BuildPoSChainOnTop:  true,
				MainClientShallSync: true,
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

func (clients SecondaryClientSpecs) AnySync() bool {
	for _, c := range clients {
		if c.MainClientShallSync {
			return true
		}
	}
	return false
}

func (clients SecondaryClientSpecs) AnyPoSChainOnTop() bool {
	for _, c := range clients {
		if c.BuildPoSChainOnTop {
			return true
		}
	}
	return false
}

func GenerateMergeTestSpec(mergeTestSpec MergeTestSpec) TestSpec {
	runFunc := func(t *TestEnv) {
		// The first client waits for TTD, which ideally should be reached immediately using loaded chain
		if !mergeTestSpec.SkipMainClientTTDWait {
			if !t.Engine.waitForTTDWithTimeout(t.Timeout) {
				t.Fatalf("FAIL (%s): Timeout waiting for EngineClient (%s) to reach TTD", t.TestName, t.Engine.Container)
			}

			if !mergeTestSpec.SkipMainClientFcU {
				// Set the head of the CLMocker to the head of the main client
				t.CLMock.setTTDBlockClient(t.Engine)
			}
		}

		// At this point, Head must be main client's HeadBlockHash, but this can change depending on the
		// secondary clients
		var mustHeadHash common.Hash
		if header, err := t.Eth.HeaderByNumber(t.Ctx(), nil); err == nil {
			mustHeadHash = header.Hash()
		} else {
			t.Fatalf("FAIL (%s): Unable to obtain main client latest header: %v", t.TestName, err)
		}

		// Get the main client's enode to pass it to secondary clients
		enode, err := t.Engine.EnodeURL()
		if err != nil {
			t.Fatalf("FAIL (%s): Unable to obtain main client enode: %v", t.TestName, err)
		}

		// Start a secondary clients with alternative PoW chains
		for _, secondaryClientSpec := range mergeTestSpec.SecondaryClientSpecs {
			// Start the secondary client with the alternative PoW chain
			t.Logf("INFO (%s): Running secondary client: %v", t.TestName, secondaryClientSpec)
			secondaryClient := secondaryClientSpec.StartClient(t, enode)

			// Add this client to the CLMocker list of Engine clients
			t.CLMock.AddEngineClient(t.T, secondaryClient.Client, big.NewInt(secondaryClientSpec.TTD))

			if secondaryClientSpec.BuildPoSChainOnTop {
				if !secondaryClient.waitForTTDWithTimeout(t.Timeout) {
					t.Fatalf("FAIL (%s): Timeout waiting for EngineClient (%s) to reach TTD", t.TestName, secondaryClient.Client.Container)
				}
				t.CLMock.setTTDBlockClient(secondaryClient)
			}

			if secondaryClientSpec.MainClientShallSync {
				// The main client shall sync to this secondary client in order for the test to succeed.
				if header, err := secondaryClient.Eth.HeaderByNumber(secondaryClient.Ctx(), nil); err == nil {
					mustHeadHash = header.Hash()
				} else {
					t.Fatalf("FAIL (%s): Unable to obtain client [%s] latest header: %v", t.TestName, secondaryClient.Container, err)
				}
			}
		}

		// Test end state of the main client
		for {
			if mergeTestSpec.SecondaryClientSpecs.AnyPoSChainOnTop() {
				// Build a block and check whether the main client switches
				t.CLMock.produceSingleBlock(BlockProcessCallbacks{})

				// If the main client should follow the PoS chain, update the mustHeadHash
				if mustHeadHash == t.CLMock.LatestFinalizedHeader.ParentHash {
					mustHeadHash = t.CLMock.LatestFinalizedHeader.Hash()
				}

				// Check for timeout
				select {
				case <-t.Timeout:
					t.Fatalf("FAIL (%s): Timeout while waiting for sync on the alternative PoW chain", t.TestName)
				default:
				}
			} else {
				// The test case does not build the PoS chain, wait here before checking the head again.
				select {
				case <-time.After(time.Second):
				case <-t.Timeout:
					t.Fatalf("FAIL (%s): Timeout while waiting for sync on the alternative PoW chain", t.TestName)
				}
			}
			if header, err := t.Eth.HeaderByNumber(t.Ctx(), nil); err == nil {
				if header.Hash() == mustHeadHash {
					t.Logf("INFO (%s): Main client is now synced to the expected head, %v", t.TestName, header.Hash())
					break
				}
			} else {
				t.Fatalf("FAIL (%s): Error getting latest header for main client: %v", t.TestName, err)
			}

		}

		// Test specified that we must keep checking the main client to stick to mustHeadHash for a certain amount of time
		for ; mergeTestSpec.KeepCheckingSeconds > 0; mergeTestSpec.KeepCheckingSeconds -= 1 {
			if mergeTestSpec.SecondaryClientSpecs.AnyPoSChainOnTop() {
				// Build a block and check whether the main client switches
				t.CLMock.produceSingleBlock(BlockProcessCallbacks{})

				// If the main client should follow the PoS chain, update the mustHeadHash
				if mustHeadHash == t.CLMock.LatestFinalizedHeader.ParentHash {
					mustHeadHash = t.CLMock.LatestFinalizedHeader.Hash()
				}

				// Check for timeout
				select {
				case <-t.Timeout:
					t.Fatalf("FAIL (%s): Timeout while waiting for sync on the alternative PoW chain", t.TestName)
				default:
				}
			} else {
				// The test case does not build the PoS chain, wait here before checking the head again.
				select {
				case <-time.After(time.Second):
				case <-t.Timeout:
					t.Fatalf("FAIL (%s): Timeout while waiting for sync on the alternative PoW chain", t.TestName)
				}
			}
			if header, err := t.Eth.HeaderByNumber(t.Ctx(), nil); err == nil {
				if header.Hash() != mustHeadHash {
					t.Logf("FAIL (%s): Main client synced to incorrect chain: %v", t.TestName, header.Hash())
					break
				}
			} else {
				t.Fatalf("FAIL (%s): Error getting latest header for main client: %v", t.TestName, err)
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

func (spec SecondaryClientSpec) ChainPath() string {
	return "./chains/" + spec.ChainFile
}

func (spec SecondaryClientSpec) StartClient(t *TestEnv, enode string) *EngineClient {
	clientTypes, err := t.Sim.ClientTypes()
	if err != nil || len(clientTypes) == 0 {
		t.Fatalf("FAIL (%s): Unable to obtain client types: %v", t.TestName, err)
	}
	// Let's use the first just for now
	secondaryClientType := clientTypes[0]

	secondaryClientFiles := t.ClientFiles.Set("/chain.rlp", spec.ChainPath())

	secondaryClientParams := t.ClientParams.Set("HIVE_BOOTNODE", enode)

	if spec.TTD != 0 {
		ttd := calcRealTTD(t.ClientFiles["/genesis.json"], spec.TTD)
		secondaryClientParams = secondaryClientParams.Set("HIVE_TERMINAL_TOTAL_DIFFICULTY", fmt.Sprintf("%d", ttd))
	}

	c := t.T.StartClient(secondaryClientType.Name, secondaryClientParams, hivesim.WithStaticFiles(secondaryClientFiles))
	return NewEngineClient(t.T, c, big.NewInt(spec.TTD))
}
