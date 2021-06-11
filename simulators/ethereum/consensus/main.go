package main

import (
	"bytes"
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"gopkg.in/inconshreveable/log15.v2"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/ethereum/hive/hivesim"
)

type envvars map[string]int

var ruleset = map[string]envvars{
//	"Frontier": {
//		"HIVE_FORK_HOMESTEAD":      2000,
//		"HIVE_FORK_DAO_BLOCK":      2000,
//		"HIVE_FORK_TANGERINE":      2000,
//		"HIVE_FORK_SPURIOUS":       2000,
//		"HIVE_FORK_BYZANTIUM":      2000,
//		"HIVE_FORK_CONSTANTINOPLE": 2000,
//		"HIVE_FORK_PETERSBURG":     2000,
//		"HIVE_FORK_ISTANBUL":       2000,
//		"HIVE_FORK_BERLIN":         2000,
//	},
//	"Homestead": {
//		"HIVE_FORK_HOMESTEAD":      0,
//		"HIVE_FORK_DAO_BLOCK":      2000,
//		"HIVE_FORK_TANGERINE":      2000,
//		"HIVE_FORK_SPURIOUS":       2000,
//		"HIVE_FORK_BYZANTIUM":      2000,
//		"HIVE_FORK_CONSTANTINOPLE": 2000,
//		"HIVE_FORK_PETERSBURG":     2000,
//		"HIVE_FORK_ISTANBUL":       2000,
//		"HIVE_FORK_BERLIN":         2000,
//	},
//	"EIP150": {
//		"HIVE_FORK_HOMESTEAD": 0,
//		//"HIVE_FORK_DAO_BLOCK":      2000,
//		"HIVE_FORK_TANGERINE":      0,
//		"HIVE_FORK_SPURIOUS":       2000,
//		"HIVE_FORK_BYZANTIUM":      2000,
//		"HIVE_FORK_CONSTANTINOPLE": 2000,
//		"HIVE_FORK_PETERSBURG":     2000,
//		"HIVE_FORK_ISTANBUL":       2000,
//		"HIVE_FORK_BERLIN":         2000,
//	},
//	"EIP158": {
//		"HIVE_FORK_HOMESTEAD": 0,
//		//"HIVE_FORK_DAO_BLOCK":      2000,
//		"HIVE_FORK_TANGERINE":      0,
//		"HIVE_FORK_SPURIOUS":       0,
//		"HIVE_FORK_BYZANTIUM":      2000,
//		"HIVE_FORK_CONSTANTINOPLE": 2000,
//		"HIVE_FORK_PETERSBURG":     2000,
//		"HIVE_FORK_ISTANBUL":       2000,
//		"HIVE_FORK_BERLIN":         2000,
//	},
//	"Byzantium": {
//		"HIVE_FORK_HOMESTEAD": 0,
//		//"HIVE_FORK_DAO_BLOCK":      2000,
//		"HIVE_FORK_TANGERINE":      0,
//		"HIVE_FORK_SPURIOUS":       0,
//		"HIVE_FORK_BYZANTIUM":      0,
//		"HIVE_FORK_CONSTANTINOPLE": 2000,
//		"HIVE_FORK_PETERSBURG":     2000,
//		"HIVE_FORK_ISTANBUL":       2000,
//		"HIVE_FORK_BERLIN":         2000,
//	},
//	"Constantinople": {
//		"HIVE_FORK_HOMESTEAD": 0,
//		//"HIVE_FORK_DAO_BLOCK":      2000,
//		"HIVE_FORK_TANGERINE":      0,
//		"HIVE_FORK_SPURIOUS":       0,
//		"HIVE_FORK_BYZANTIUM":      0,
//		"HIVE_FORK_CONSTANTINOPLE": 0,
//		"HIVE_FORK_PETERSBURG":     2000,
//		"HIVE_FORK_ISTANBUL":       2000,
//		"HIVE_FORK_BERLIN":         2000,
//	},
//	"ConstantinopleFix": {
//		"HIVE_FORK_HOMESTEAD": 0,
//		//"HIVE_FORK_DAO_BLOCK":      2000,
//		"HIVE_FORK_TANGERINE":      0,
//		"HIVE_FORK_SPURIOUS":       0,
//		"HIVE_FORK_BYZANTIUM":      0,
//		"HIVE_FORK_CONSTANTINOPLE": 0,
//		"HIVE_FORK_PETERSBURG":     0,
//		"HIVE_FORK_ISTANBUL":       2000,
//		"HIVE_FORK_BERLIN":         2000,
//	},
//	"Istanbul": {
//		"HIVE_FORK_HOMESTEAD": 0,
//		//"HIVE_FORK_DAO_BLOCK":      2000,
//		"HIVE_FORK_TANGERINE":      0,
//		"HIVE_FORK_SPURIOUS":       0,
//		"HIVE_FORK_BYZANTIUM":      0,
//		"HIVE_FORK_CONSTANTINOPLE": 0,
//		"HIVE_FORK_PETERSBURG":     0,
//		"HIVE_FORK_ISTANBUL":       0,
//		"HIVE_FORK_BERLIN":         2000,
//	},
//	"Berlin": {
//		"HIVE_FORK_HOMESTEAD": 0,
//		//"HIVE_FORK_DAO_BLOCK":      2000,
//		"HIVE_FORK_TANGERINE":      0,
//		"HIVE_FORK_SPURIOUS":       0,
//		"HIVE_FORK_BYZANTIUM":      0,
//		"HIVE_FORK_CONSTANTINOPLE": 0,
//		"HIVE_FORK_PETERSBURG":     0,
//		"HIVE_FORK_ISTANBUL":       0,
//		"HIVE_FORK_BERLIN":         0,
//	},
//	"FrontierToHomesteadAt5": {
//		"HIVE_FORK_HOMESTEAD":      5,
//		"HIVE_FORK_DAO_BLOCK":      2000,
//		"HIVE_FORK_TANGERINE":      2000,
//		"HIVE_FORK_SPURIOUS":       2000,
//		"HIVE_FORK_BYZANTIUM":      2000,
//		"HIVE_FORK_CONSTANTINOPLE": 2000,
//		"HIVE_FORK_PETERSBURG":     2000,
//		"HIVE_FORK_ISTANBUL":       2000,
//		"HIVE_FORK_BERLIN":         2000,
//	},
//	"HomesteadToEIP150At5": {
//		"HIVE_FORK_HOMESTEAD": 0,
//		//		"HIVE_FORK_DAO_BLOCK":      2000,
//		"HIVE_FORK_TANGERINE":      5,
//		"HIVE_FORK_SPURIOUS":       2000,
//		"HIVE_FORK_BYZANTIUM":      2000,
//		"HIVE_FORK_CONSTANTINOPLE": 2000,
//		"HIVE_FORK_PETERSBURG":     2000,
//		"HIVE_FORK_ISTANBUL":       2000,
//		"HIVE_FORK_BERLIN":         2000,
//	},
//	"HomesteadToDaoAt5": {
//		"HIVE_FORK_HOMESTEAD":      0,
//		"HIVE_FORK_DAO_BLOCK":      5,
//		"HIVE_FORK_TANGERINE":      2000,
//		"HIVE_FORK_SPURIOUS":       2000,
//		"HIVE_FORK_BYZANTIUM":      2000,
//		"HIVE_FORK_CONSTANTINOPLE": 2000,
//		"HIVE_FORK_PETERSBURG":     2000,
//		"HIVE_FORK_ISTANBUL":       2000,
//		"HIVE_FORK_BERLIN":         2000,
//	},
//	"EIP158ToByzantiumAt5": {
//		"HIVE_FORK_HOMESTEAD": 0,
//		//"HIVE_FORK_DAO_BLOCK":      2000,
//		"HIVE_FORK_TANGERINE":      0,
//		"HIVE_FORK_SPURIOUS":       0,
//		"HIVE_FORK_BYZANTIUM":      5,
//		"HIVE_FORK_CONSTANTINOPLE": 2000,
//		"HIVE_FORK_PETERSBURG":     2000,
//		"HIVE_FORK_ISTANBUL":       2000,
//		"HIVE_FORK_BERLIN":         2000,
//	},
//	"ByzantiumToConstantinopleAt5": {
//		"HIVE_FORK_HOMESTEAD": 0,
//		//"HIVE_FORK_DAO_BLOCK":      2000,
//		"HIVE_FORK_TANGERINE":      0,
//		"HIVE_FORK_SPURIOUS":       0,
//		"HIVE_FORK_BYZANTIUM":      0,
//		"HIVE_FORK_CONSTANTINOPLE": 5,
//		"HIVE_FORK_PETERSBURG":     2000,
//		"HIVE_FORK_ISTANBUL":       2000,
//		"HIVE_FORK_BERLIN":         2000,
//	},
//	"ByzantiumToConstantinopleFixAt5": {
//		"HIVE_FORK_HOMESTEAD": 0,
//		//"HIVE_FORK_DAO_BLOCK":      2000,
//		"HIVE_FORK_TANGERINE":      0,
//		"HIVE_FORK_SPURIOUS":       0,
//		"HIVE_FORK_BYZANTIUM":      0,
//		"HIVE_FORK_CONSTANTINOPLE": 5,
//		"HIVE_FORK_PETERSBURG":     5,
//		"HIVE_FORK_ISTANBUL":       2000,
//		"HIVE_FORK_BERLIN":         2000,
//	},
//	"ConstantinopleFixToIstanbulAt5": {
//		"HIVE_FORK_HOMESTEAD": 0,
//		//"HIVE_FORK_DAO_BLOCK":      2000,
//		"HIVE_FORK_TANGERINE":      0,
//		"HIVE_FORK_SPURIOUS":       0,
//		"HIVE_FORK_BYZANTIUM":      0,
//		"HIVE_FORK_CONSTANTINOPLE": 0,
//		"HIVE_FORK_PETERSBURG":     0,
//		"HIVE_FORK_ISTANBUL":       5,
//		"HIVE_FORK_BERLIN":         2000,
//	},
//	"IstanbulToBerlinAt5": {
//		"HIVE_FORK_HOMESTEAD": 0,
//		//"HIVE_FORK_DAO_BLOCK":      2000,
//		"HIVE_FORK_TANGERINE":      0,
//		"HIVE_FORK_SPURIOUS":       0,
//		"HIVE_FORK_BYZANTIUM":      0,
//		"HIVE_FORK_CONSTANTINOPLE": 0,
//		"HIVE_FORK_PETERSBURG":     0,
//		"HIVE_FORK_ISTANBUL":       0,
//		"HIVE_FORK_BERLIN":         5,
//	},
	"London": {
		"HIVE_FORK_HOMESTEAD": 		0,
		//"HIVE_FORK_DAO_BLOCK":      2000,
		"HIVE_FORK_TANGERINE":      0,
		"HIVE_FORK_SPURIOUS":       0,
		"HIVE_FORK_BYZANTIUM":      0,
		"HIVE_FORK_CONSTANTINOPLE": 0,
		"HIVE_FORK_PETERSBURG":     0,
		"HIVE_FORK_ISTANBUL":       0,
		"HIVE_FORK_BERLIN":         0,
		"HIVE_FORK_LONDON": 		0,
	},
}

func main() {
	suite := hivesim.Suite{
		Name: "consensus",
		Description: "The 'consensus' test suite executes BlockchainTests from the " +
			"offical test repository (https://github.com/ethereum/tests). For every test, it starts an instance of the client, " +
			"and makes it import the RLP blocks. After import phase, the node is queried about it's latest blocks, which is matched " +
			"to the expected last blockhash according to the test.",
	}
	suite.Add(hivesim.TestSpec{
		Name: "test file loader",
		Description: "This is a meta-test. It loads the blockchain test files and " +
			"launches the actual client tests. Any errors in test files will be reported " +
			"through this test.",
		Run: loaderTest,
	})
	hivesim.MustRunSuite(hivesim.New(), suite)
}

// loaderTest loads the blockchain test files and spawns the client tests.
func loaderTest(t *hivesim.T) {
	clientTypes, err := t.Sim.ClientTypes()
	if err != nil {
		t.Fatal("can't get client types:", err)
	}

	parallelism := 16
	if val, ok := os.LookupEnv("HIVE_PARALLELISM"); ok {
		if p, err := strconv.Atoi(val); err != nil {
			t.Logf("Warning: invalid HIVE_PARALLELISM value %q", val)
		} else {
			parallelism = p
		}
	}
	testLimit := -1
	if val, ok := os.LookupEnv("HIVE_SIMLIMIT"); ok {
		if p, err := strconv.Atoi(val); err != nil {
			t.Logf("Warning: invalid HIVE_SIMLIMIT value %q", val)
		} else {
			testLimit = p
		}
	}
	t.Log("parallelism:", parallelism, "testlimit:", testLimit)

	// Find the tests directory.
	testPath, isset := os.LookupEnv("TESTPATH")
	if !isset {
		t.Fatal("$TESTPATH not set")
	}
	fileRoot := fmt.Sprintf("%s/BlockchainTests/", testPath)

	// Spawn workers.
	var wg sync.WaitGroup
	var testCh = make(chan *testcase)
	wg.Add(parallelism)
	for i := 0; i < parallelism; i++ {
		go func() {
			defer wg.Done()
			for test := range testCh {
				t.Run(hivesim.TestSpec{
					Name:        test.name,
					Description: "Test source: " + testLink(test.filepath),
					Run:         test.run,
				})
			}
		}()
	}

	// Deliver test cases.
	loadTests(t, fileRoot, testLimit, func(tc testcase) {
		for _, client := range clientTypes {
			tc := tc // shallow copy
			tc.clientType = client
			testCh <- &tc
		}
	})
	close(testCh)

	// Wait for workers to finish.
	wg.Wait()
}

// testLink turns a test path into a link to the tests repo.
func testLink(filepath string) string {
	// Convert ./tests/BlockchainTests/InvalidBlocks/bcExpectSection/lastblockhash.json
	// into
	// BlockchainTests/InvalidBlocks/bcExpectSection/lastblockhash.json
	path := strings.TrimPrefix(filepath, ".")
	path = strings.TrimPrefix(path, "/tests/")
	// Link to
	// https://github.com/ethereum/tests/blob/develop/BlockchainTests/InvalidBlocks/bcExpectSection/lastblockhash.json
	return fmt.Sprintf("https://github.com/ethereum/tests/blob/develop/%v", path)
}

// deliverTests loads the files in 'root', running the given function for each test.
func loadTests(t *hivesim.T, root string, limit int, fn func(testcase)) {
	var i, j = 0, 0
	filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if limit >= 0 && i >= limit {
			return filepath.SkipDir
		}
		if info.IsDir() {
			return nil
		}
		if fname := info.Name(); !strings.HasSuffix(fname, ".json") {
			return nil
		}
		var tests map[string]BlockTest
		if err := common.LoadJSON(path, &tests); err != nil {
			t.Logf("invalid test file: %v", err)
			return nil
		}

		j++
		for name, blocktest := range tests {
			tc := testcase{blockTest: blocktest, name: name, filepath: path}
			if err := tc.validate(); err != nil {
				t.Errorf("test validation failed for %s: %v", tc.name, err)
				continue
			}
			fn(tc)
			i++
		}
		return nil
	})
}

type testcase struct {
	name       string
	clientType string
	blockTest  BlockTest
	filepath   string
}

// validate returns error if the test's chain rules are not supported.
func (tc *testcase) validate() error {
	net := tc.blockTest.json.Network
	if _, exist := ruleset[net]; !exist {
		return fmt.Errorf("network `%v` not defined in ruleset", net)
	}
	return nil
}

// run launches the client and runs the test case against it.
func (tc *testcase) run(t *hivesim.T) {
	start := time.Now()
	root, genesis, blocks, err := tc.artefacts()
	log15.Crit("NAME: ", tc.name, "ROOT: ", root, "GENESIS: ", genesis)
	if err != nil {
		t.Fatal("can't prepare artefacts:", err)
	}

	// update the parameters with test-specific stuff
	env := map[string]string{
		"HIVE_FORK_DAO_VOTE": "1",
		"HIVE_CHAIN_ID":      "1",
	}
	tc.updateEnv(env)
	genesisTarget := strings.Replace(genesis, root, "", 1)
	files := map[string]string{
		genesisTarget: genesis,
	}
	for _, filename := range blocks {
		fileTarget := strings.Replace(filename, root, "", 1)
		files[fileTarget] = filename
	}

	t1 := time.Now()
	client := t.StartClient(tc.clientType, env, files)

	t2 := time.Now()
	genesisHash, err := getHash(client.RPC(), "0x0")
	if err != nil {
		t.Fatal("can't get genesis:", err)
	}
	wantGenesis := tc.blockTest.json.Genesis.Hash
	if !bytes.Equal(wantGenesis[:], genesisHash) {
		t.Fatalf("genesis mismatch:\n  want 0x%x\n   got 0x%x", wantGenesis, genesisHash)
	}

	// verify postconditions
	t3 := time.Now()
	lastHash, err := getHash(client.RPC(), "latest")
	if err != nil {
		t.Fatal("can't get latest block:", err)
	}
	wantBest := tc.blockTest.json.BestBlock
	if !bytes.Equal(wantBest[:], lastHash) {
		t.Fatalf("last block mismatch:\n  want 0x%x\n   got 0x%x", wantBest, lastHash)
	}

	t4 := time.Now()
	t.Logf(`test timing:
  artefacts    %v
  startClient  %v
  checkGenesis %v
  checkLatest  %v`, t1.Sub(start), t2.Sub(t1), t3.Sub(t2), t4.Sub(t3))
}

// updateEnv sets environment variables from the test
func (tc *testcase) updateEnv(env map[string]string) {
	// Environment variables for rules.
	rules := ruleset[tc.blockTest.json.Network]
	for k, v := range rules {
		env[k] = fmt.Sprintf("%d", v)
	}
	// Possibly disable POW.
	if tc.blockTest.json.SealEngine == "NoProof" {
		env["HIVE_SKIP_POW"] = "1"
	}
}

func toGethGenesis(test *btJSON) *core.Genesis {
	genesis := &core.Genesis{
		Nonce:      test.Genesis.Nonce.Uint64(),
		Timestamp:  test.Genesis.Timestamp.Uint64(),
		ExtraData:  test.Genesis.ExtraData,
		GasLimit:   test.Genesis.GasLimit,
		Difficulty: test.Genesis.Difficulty,
		Mixhash:    test.Genesis.MixHash,
		Coinbase:   test.Genesis.Coinbase,
		Alloc:      test.Pre,
	}
	return genesis
}

// artefacts generates the test files which are copied into the client container.
func (tc *testcase) artefacts() (string, string, []string, error) {
	key := fmt.Sprintf("%x", sha1.Sum([]byte(tc.filepath+tc.name)))
	rootDir := filepath.Join(tc.clientType, key)
	blockDir := filepath.Join(rootDir, "blocks")

	if err := os.MkdirAll(blockDir, 0700); err != nil {
		return "", "", nil, err
	}
	genesis := toGethGenesis(&tc.blockTest.json)
	genBytes, _ := json.Marshal(genesis)
	genesisFile := filepath.Join(rootDir, "genesis.json")
	if err := ioutil.WriteFile(genesisFile, genBytes, 0777); err != nil {
		return rootDir, "", nil, fmt.Errorf("failed writing genesis: %v", err)
	}

	var blocks []string
	for i, block := range tc.blockTest.json.Blocks {
		rlpdata := common.FromHex(block.Rlp)
		fname := fmt.Sprintf("%s/%04d.rlp", blockDir, i+1)
		if err := ioutil.WriteFile(fname, rlpdata, 0777); err != nil {
			return rootDir, genesisFile, blocks, fmt.Errorf("failed writing block %d: %v", i, err)
		}
		blocks = append(blocks, fname)
	}
	return rootDir, genesisFile, blocks, nil
}

func getHash(rawClient *rpc.Client, arg string) ([]byte, error) {
	blockData := make(map[string]interface{})
	if err := rawClient.Call(&blockData, "eth_getBlockByNumber", arg, false); err != nil {
		// Make one more attempt
		fmt.Println("Client connect failed, making one more attempt...")
		if err = rawClient.Call(&blockData, "eth_getBlockByNumber", arg, false); err != nil {
			return nil, err
		}
	}
	if hash, exist := blockData["hash"]; !exist {
		return nil, fmt.Errorf("no block hash found in response")
	} else {
		if hexHash, ok := hash.(string); !ok {
			return nil, fmt.Errorf("error: string conversion failed for `%v`", hash)
		} else {
			return common.HexToHash(hexHash).Bytes(), nil
		}
	}
}
