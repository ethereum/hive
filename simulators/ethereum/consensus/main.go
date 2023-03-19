package main

import (
	"bytes"
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
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
	"Frontier": {
		"HIVE_FORK_HOMESTEAD":      2000,
		"HIVE_FORK_DAO_BLOCK":      2000,
		"HIVE_FORK_TANGERINE":      2000,
		"HIVE_FORK_SPURIOUS":       2000,
		"HIVE_FORK_BYZANTIUM":      2000,
		"HIVE_FORK_CONSTANTINOPLE": 2000,
		"HIVE_FORK_PETERSBURG":     2000,
		"HIVE_FORK_ISTANBUL":       2000,
		"HIVE_FORK_BERLIN":         2000,
		"HIVE_FORK_LONDON":         2000,
	},
	"Homestead": {
		"HIVE_FORK_HOMESTEAD":      0,
		"HIVE_FORK_DAO_BLOCK":      2000,
		"HIVE_FORK_TANGERINE":      2000,
		"HIVE_FORK_SPURIOUS":       2000,
		"HIVE_FORK_BYZANTIUM":      2000,
		"HIVE_FORK_CONSTANTINOPLE": 2000,
		"HIVE_FORK_PETERSBURG":     2000,
		"HIVE_FORK_ISTANBUL":       2000,
		"HIVE_FORK_BERLIN":         2000,
		"HIVE_FORK_LONDON":         2000,
	},
	"EIP150": {
		"HIVE_FORK_HOMESTEAD":      0,
		"HIVE_FORK_TANGERINE":      0,
		"HIVE_FORK_SPURIOUS":       2000,
		"HIVE_FORK_BYZANTIUM":      2000,
		"HIVE_FORK_CONSTANTINOPLE": 2000,
		"HIVE_FORK_PETERSBURG":     2000,
		"HIVE_FORK_ISTANBUL":       2000,
		"HIVE_FORK_BERLIN":         2000,
		"HIVE_FORK_LONDON":         2000,
	},
	"EIP158": {
		"HIVE_FORK_HOMESTEAD":      0,
		"HIVE_FORK_TANGERINE":      0,
		"HIVE_FORK_SPURIOUS":       0,
		"HIVE_FORK_BYZANTIUM":      2000,
		"HIVE_FORK_CONSTANTINOPLE": 2000,
		"HIVE_FORK_PETERSBURG":     2000,
		"HIVE_FORK_ISTANBUL":       2000,
		"HIVE_FORK_BERLIN":         2000,
		"HIVE_FORK_LONDON":         2000,
	},
	"Byzantium": {
		"HIVE_FORK_HOMESTEAD":      0,
		"HIVE_FORK_TANGERINE":      0,
		"HIVE_FORK_SPURIOUS":       0,
		"HIVE_FORK_BYZANTIUM":      0,
		"HIVE_FORK_CONSTANTINOPLE": 2000,
		"HIVE_FORK_PETERSBURG":     2000,
		"HIVE_FORK_ISTANBUL":       2000,
		"HIVE_FORK_BERLIN":         2000,
		"HIVE_FORK_LONDON":         2000,
	},
	"Constantinople": {
		"HIVE_FORK_HOMESTEAD":      0,
		"HIVE_FORK_TANGERINE":      0,
		"HIVE_FORK_SPURIOUS":       0,
		"HIVE_FORK_BYZANTIUM":      0,
		"HIVE_FORK_CONSTANTINOPLE": 0,
		"HIVE_FORK_PETERSBURG":     2000,
		"HIVE_FORK_ISTANBUL":       2000,
		"HIVE_FORK_BERLIN":         2000,
		"HIVE_FORK_LONDON":         2000,
	},
	"ConstantinopleFix": {
		"HIVE_FORK_HOMESTEAD":      0,
		"HIVE_FORK_TANGERINE":      0,
		"HIVE_FORK_SPURIOUS":       0,
		"HIVE_FORK_BYZANTIUM":      0,
		"HIVE_FORK_CONSTANTINOPLE": 0,
		"HIVE_FORK_PETERSBURG":     0,
		"HIVE_FORK_ISTANBUL":       2000,
		"HIVE_FORK_BERLIN":         2000,
		"HIVE_FORK_LONDON":         2000,
	},
	"Istanbul": {
		"HIVE_FORK_HOMESTEAD":      0,
		"HIVE_FORK_TANGERINE":      0,
		"HIVE_FORK_SPURIOUS":       0,
		"HIVE_FORK_BYZANTIUM":      0,
		"HIVE_FORK_CONSTANTINOPLE": 0,
		"HIVE_FORK_PETERSBURG":     0,
		"HIVE_FORK_ISTANBUL":       0,
		"HIVE_FORK_BERLIN":         2000,
		"HIVE_FORK_LONDON":         2000,
	},
	"Berlin": {
		"HIVE_FORK_HOMESTEAD":      0,
		"HIVE_FORK_TANGERINE":      0,
		"HIVE_FORK_SPURIOUS":       0,
		"HIVE_FORK_BYZANTIUM":      0,
		"HIVE_FORK_CONSTANTINOPLE": 0,
		"HIVE_FORK_PETERSBURG":     0,
		"HIVE_FORK_ISTANBUL":       0,
		"HIVE_FORK_BERLIN":         0,
		"HIVE_FORK_LONDON":         2000,
	},
	"FrontierToHomesteadAt5": {
		"HIVE_FORK_HOMESTEAD":      5,
		"HIVE_FORK_DAO_BLOCK":      2000,
		"HIVE_FORK_TANGERINE":      2000,
		"HIVE_FORK_SPURIOUS":       2000,
		"HIVE_FORK_BYZANTIUM":      2000,
		"HIVE_FORK_CONSTANTINOPLE": 2000,
		"HIVE_FORK_PETERSBURG":     2000,
		"HIVE_FORK_ISTANBUL":       2000,
		"HIVE_FORK_BERLIN":         2000,
		"HIVE_FORK_LONDON":         2000,
	},
	"HomesteadToEIP150At5": {
		"HIVE_FORK_HOMESTEAD": 0,
		//		"HIVE_FORK_DAO_BLOCK":      2000,
		"HIVE_FORK_TANGERINE":      5,
		"HIVE_FORK_SPURIOUS":       2000,
		"HIVE_FORK_BYZANTIUM":      2000,
		"HIVE_FORK_CONSTANTINOPLE": 2000,
		"HIVE_FORK_PETERSBURG":     2000,
		"HIVE_FORK_ISTANBUL":       2000,
		"HIVE_FORK_BERLIN":         2000,
		"HIVE_FORK_LONDON":         2000,
	},
	"HomesteadToDaoAt5": {
		"HIVE_FORK_HOMESTEAD":      0,
		"HIVE_FORK_DAO_BLOCK":      5,
		"HIVE_FORK_TANGERINE":      2000,
		"HIVE_FORK_SPURIOUS":       2000,
		"HIVE_FORK_BYZANTIUM":      2000,
		"HIVE_FORK_CONSTANTINOPLE": 2000,
		"HIVE_FORK_PETERSBURG":     2000,
		"HIVE_FORK_ISTANBUL":       2000,
		"HIVE_FORK_BERLIN":         2000,
		"HIVE_FORK_LONDON":         2000,
	},
	"EIP158ToByzantiumAt5": {
		"HIVE_FORK_HOMESTEAD": 0,
		//"HIVE_FORK_DAO_BLOCK":      2000,
		"HIVE_FORK_TANGERINE":      0,
		"HIVE_FORK_SPURIOUS":       0,
		"HIVE_FORK_BYZANTIUM":      5,
		"HIVE_FORK_CONSTANTINOPLE": 2000,
		"HIVE_FORK_PETERSBURG":     2000,
		"HIVE_FORK_ISTANBUL":       2000,
		"HIVE_FORK_BERLIN":         2000,
		"HIVE_FORK_LONDON":         2000,
	},
	"ByzantiumToConstantinopleAt5": {
		"HIVE_FORK_HOMESTEAD": 0,
		//"HIVE_FORK_DAO_BLOCK":      2000,
		"HIVE_FORK_TANGERINE":      0,
		"HIVE_FORK_SPURIOUS":       0,
		"HIVE_FORK_BYZANTIUM":      0,
		"HIVE_FORK_CONSTANTINOPLE": 5,
		"HIVE_FORK_PETERSBURG":     2000,
		"HIVE_FORK_ISTANBUL":       2000,
		"HIVE_FORK_BERLIN":         2000,
		"HIVE_FORK_LONDON":         2000,
	},
	"ByzantiumToConstantinopleFixAt5": {
		"HIVE_FORK_HOMESTEAD": 0,
		//"HIVE_FORK_DAO_BLOCK":      2000,
		"HIVE_FORK_TANGERINE":      0,
		"HIVE_FORK_SPURIOUS":       0,
		"HIVE_FORK_BYZANTIUM":      0,
		"HIVE_FORK_CONSTANTINOPLE": 5,
		"HIVE_FORK_PETERSBURG":     5,
		"HIVE_FORK_ISTANBUL":       2000,
		"HIVE_FORK_BERLIN":         2000,
		"HIVE_FORK_LONDON":         2000,
	},
	"ConstantinopleFixToIstanbulAt5": {
		"HIVE_FORK_HOMESTEAD": 0,
		//"HIVE_FORK_DAO_BLOCK":      2000,
		"HIVE_FORK_TANGERINE":      0,
		"HIVE_FORK_SPURIOUS":       0,
		"HIVE_FORK_BYZANTIUM":      0,
		"HIVE_FORK_CONSTANTINOPLE": 0,
		"HIVE_FORK_PETERSBURG":     0,
		"HIVE_FORK_ISTANBUL":       5,
		"HIVE_FORK_BERLIN":         2000,
		"HIVE_FORK_LONDON":         2000,
	},
	"IstanbulToBerlinAt5": {
		"HIVE_FORK_HOMESTEAD": 0,
		//"HIVE_FORK_DAO_BLOCK":      2000,
		"HIVE_FORK_TANGERINE":      0,
		"HIVE_FORK_SPURIOUS":       0,
		"HIVE_FORK_BYZANTIUM":      0,
		"HIVE_FORK_CONSTANTINOPLE": 0,
		"HIVE_FORK_PETERSBURG":     0,
		"HIVE_FORK_ISTANBUL":       0,
		"HIVE_FORK_BERLIN":         5,
		"HIVE_FORK_LONDON":         2000,
	},
	"BerlinToLondonAt5": {
		"HIVE_FORK_HOMESTEAD": 0,
		//"HIVE_FORK_DAO_BLOCK":      2000,
		"HIVE_FORK_TANGERINE":      0,
		"HIVE_FORK_SPURIOUS":       0,
		"HIVE_FORK_BYZANTIUM":      0,
		"HIVE_FORK_CONSTANTINOPLE": 0,
		"HIVE_FORK_PETERSBURG":     0,
		"HIVE_FORK_ISTANBUL":       0,
		"HIVE_FORK_BERLIN":         0,
		"HIVE_FORK_LONDON":         5,
	},
	"London": {
		"HIVE_FORK_HOMESTEAD":      0,
		"HIVE_FORK_TANGERINE":      0,
		"HIVE_FORK_SPURIOUS":       0,
		"HIVE_FORK_BYZANTIUM":      0,
		"HIVE_FORK_CONSTANTINOPLE": 0,
		"HIVE_FORK_PETERSBURG":     0,
		"HIVE_FORK_ISTANBUL":       0,
		"HIVE_FORK_BERLIN":         0,
		"HIVE_FORK_LONDON":         0,
	},
	"ArrowGlacierToMergeAtDiffC0000": {
		"HIVE_FORK_HOMESTEAD":            0,
		"HIVE_FORK_TANGERINE":            0,
		"HIVE_FORK_SPURIOUS":             0,
		"HIVE_FORK_BYZANTIUM":            0,
		"HIVE_FORK_CONSTANTINOPLE":       0,
		"HIVE_FORK_PETERSBURG":           0,
		"HIVE_FORK_ISTANBUL":             0,
		"HIVE_FORK_BERLIN":               0,
		"HIVE_FORK_LONDON":               0,
		"HIVE_TERMINAL_TOTAL_DIFFICULTY": 786432,
	},
	"Merge": {
		"HIVE_FORK_HOMESTEAD":            0,
		"HIVE_FORK_TANGERINE":            0,
		"HIVE_FORK_SPURIOUS":             0,
		"HIVE_FORK_BYZANTIUM":            0,
		"HIVE_FORK_CONSTANTINOPLE":       0,
		"HIVE_FORK_PETERSBURG":           0,
		"HIVE_FORK_ISTANBUL":             0,
		"HIVE_FORK_BERLIN":               0,
		"HIVE_FORK_LONDON":               0,
		"HIVE_FORK_MERGE":                0,
		"HIVE_TERMINAL_TOTAL_DIFFICULTY": 0,
	},
	"Shanghai": {
		"HIVE_FORK_HOMESTEAD":            0,
		"HIVE_FORK_TANGERINE":            0,
		"HIVE_FORK_SPURIOUS":             0,
		"HIVE_FORK_BYZANTIUM":            0,
		"HIVE_FORK_CONSTANTINOPLE":       0,
		"HIVE_FORK_PETERSBURG":           0,
		"HIVE_FORK_ISTANBUL":             0,
		"HIVE_FORK_BERLIN":               0,
		"HIVE_FORK_LONDON":               0,
		"HIVE_FORK_MERGE":                0,
		"HIVE_TERMINAL_TOTAL_DIFFICULTY": 0,
		"HIVE_SHANGHAI_TIMESTAMP":        0,
	},
	"MergeToShanghaiAtTime15k": {
		"HIVE_FORK_HOMESTEAD":            0,
		"HIVE_FORK_TANGERINE":            0,
		"HIVE_FORK_SPURIOUS":             0,
		"HIVE_FORK_BYZANTIUM":            0,
		"HIVE_FORK_CONSTANTINOPLE":       0,
		"HIVE_FORK_PETERSBURG":           0,
		"HIVE_FORK_ISTANBUL":             0,
		"HIVE_FORK_BERLIN":               0,
		"HIVE_FORK_LONDON":               0,
		"HIVE_FORK_MERGE":                0,
		"HIVE_TERMINAL_TOTAL_DIFFICULTY": 0,
		"HIVE_SHANGHAI_TIMESTAMP":        15000,
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
		Run:       loaderTest,
		AlwaysRun: true,
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
	t.Log("parallelism:", parallelism)

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
					// Regexp matching on Name is disabled here because it's already done
					// in loadTests. Matching in loadTests is better because it has access
					// to the full test file path.
					AlwaysRun: true,
				})
			}
		}()
	}

	_, testPattern := t.Sim.TestPattern()
	re := regexp.MustCompile(testPattern)

	// Deliver test cases.
	loadTests(t, fileRoot, re, func(tc testcase) {
		for _, client := range clientTypes {
			if !client.HasRole("eth1") {
				continue
			}
			tc := tc // shallow copy
			tc.clientType = client.Name
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

// loadTests loads the files in 'root', running the given function for each test.
func loadTests(t *hivesim.T, root string, re *regexp.Regexp, fn func(testcase)) {
	filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			t.Logf("unable to walk path: %s", err)
			return err
		}
		if info.IsDir() {
			return nil
		}
		if fname := info.Name(); !strings.HasSuffix(fname, ".json") {
			return nil
		}
		pathname := strings.TrimSuffix(strings.TrimPrefix(path, root), ".json")
		if !re.MatchString(pathname) {
			fmt.Println("skip", pathname)
			return nil // skip
		}

		var tests map[string]BlockTest
		if err := common.LoadJSON(path, &tests); err != nil {
			t.Logf("invalid test file: %v", err)
			return nil
		}

		for name, blocktest := range tests {
			tc := testcase{blockTest: blocktest, name: name, filepath: path}
			if err := tc.validate(); err != nil {
				t.Errorf("test validation failed for %s: %v", tc.name, err)
				continue
			}
			fn(tc)
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
	if err != nil {
		t.Fatal("can't prepare artefacts:", err)
	}

	// update the parameters with test-specific stuff
	env := hivesim.Params{
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
	client := t.StartClient(tc.clientType, env, hivesim.WithStaticFiles(files))

	t2 := time.Now()
	genesisHash, genesisResponse, err := getBlock(client.RPC(), "0x0")
	if err != nil {
		t.Fatalf("can't get genesis: %v", err)
	}
	wantGenesis := tc.blockTest.json.Genesis.Hash
	if !bytes.Equal(wantGenesis[:], genesisHash) {
		t.Errorf("genesis hash mismatch:\n  want 0x%x\n   got 0x%x", wantGenesis, genesisHash)
		if diffs, err := compareGenesis(genesisResponse, tc.blockTest.json.Genesis); err == nil {
			t.Logf("Found differences: %v", diffs)
		}
		return
	}

	// verify postconditions
	t3 := time.Now()
	lastHash, lastResponse, err := getBlock(client.RPC(), "latest")
	if err != nil {
		t.Fatal("can't get latest block:", err)
	}
	wantBest := tc.blockTest.json.BestBlock
	if !bytes.Equal(wantBest[:], lastHash) {
		t.Errorf("last block hash mismatch:\n  want 0x%x\n   got 0x%x", wantBest, lastHash)
		t.Log("block response:", lastResponse)
		return
	}

	t4 := time.Now()
	t.Logf(`test timing:
  artefacts    %v
  startClient  %v
  checkGenesis %v
  checkLatest  %v`, t1.Sub(start), t2.Sub(t1), t3.Sub(t2), t4.Sub(t3))
}

// updateEnv sets environment variables from the test
func (tc *testcase) updateEnv(env hivesim.Params) {
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

// toGethGenesis creates the genesis specification from a test block.
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
		BaseFee:    test.Genesis.BaseFee,
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

// getBlock fetches a block from the client under test.
func getBlock(client *rpc.Client, arg string) (blockhash []byte, responseJSON string, err error) {
	blockData := make(map[string]interface{})
	if err := client.Call(&blockData, "eth_getBlockByNumber", arg, false); err != nil {
		// Make one more attempt
		fmt.Println("Client connect failed, making one more attempt...")
		if err = client.Call(&blockData, "eth_getBlockByNumber", arg, false); err != nil {
			return nil, "", err
		}
	}

	// Capture all response data.
	resp, _ := json.MarshalIndent(blockData, "", "  ")
	responseJSON = string(resp)

	hash, ok := blockData["hash"]
	if !ok {
		return nil, responseJSON, fmt.Errorf("no block hash found in response")
	}
	hexHash, ok := hash.(string)
	if !ok {
		return nil, responseJSON, fmt.Errorf("block hash in response is not a string: `%v`", hash)
	}
	return common.HexToHash(hexHash).Bytes(), responseJSON, nil
}

// compareGenesis is a helper utility to print out diffs in the genesis returned from the client,
// and print out the differences found. This is to avoid gigantic outputs where 40K tests all
// spit out all the fields.
func compareGenesis(have string, want btHeader) (string, error) {
	var haveGenesis btHeader
	if err := json.Unmarshal([]byte(have), &haveGenesis); err != nil {
		return "", err
	}
	output := ""
	cmp := func(have, want interface{}, name string) {
		if haveStr, wantStr := fmt.Sprintf("%v", have), fmt.Sprintf("%v", want); haveStr != wantStr {
			output += fmt.Sprintf("genesis.%v - have %v, want %v \n", name, haveStr, wantStr)
		}
	}
	// No need to output the hash difference -- it's already printed before entering here
	//cmp(haveGenesis.Hash, want.Hash, "hash")
	cmp(haveGenesis.MixHash, want.MixHash, "mixHash")
	cmp(haveGenesis.ParentHash, want.ParentHash, "parentHash")
	cmp(haveGenesis.ReceiptTrie, want.ReceiptTrie, "receiptsRoot")
	cmp(haveGenesis.TransactionsTrie, want.TransactionsTrie, "transactionsRoot")
	cmp(haveGenesis.UncleHash, want.UncleHash, "sha3Uncles")
	cmp(haveGenesis.Bloom, want.Bloom, "bloom")
	cmp(haveGenesis.Number, want.Number, "number")
	cmp(haveGenesis.Coinbase, want.Coinbase, "miner")
	cmp(haveGenesis.ExtraData, want.ExtraData, "extraData")
	cmp(haveGenesis.Difficulty, want.Difficulty, "difficulty")
	cmp(haveGenesis.Timestamp, want.Timestamp, "timestamp")
	cmp(haveGenesis.BaseFee, want.BaseFee, "baseFeePerGas")
	cmp(haveGenesis.GasLimit, want.GasLimit, "gasLimit")
	cmp(haveGenesis.GasUsed, want.GasUsed, "gasused")
	return output, nil
}
