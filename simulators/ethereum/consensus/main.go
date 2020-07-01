package main

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"math/big"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/ethereum/hive/simulators/common/providers/hive"

	common2 "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/hive/simulators/common"
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
	},
	"EIP150": {
		"HIVE_FORK_HOMESTEAD":      0,
		//"HIVE_FORK_DAO_BLOCK":      2000,
		"HIVE_FORK_TANGERINE":      0,
		"HIVE_FORK_SPURIOUS":       2000,
		"HIVE_FORK_BYZANTIUM":      2000,
		"HIVE_FORK_CONSTANTINOPLE": 2000,
		"HIVE_FORK_PETERSBURG":     2000,
		"HIVE_FORK_ISTANBUL":       2000,
	},
	"EIP158": {
		"HIVE_FORK_HOMESTEAD":      0,
		//"HIVE_FORK_DAO_BLOCK":      2000,
		"HIVE_FORK_TANGERINE":      0,
		"HIVE_FORK_SPURIOUS":       0,
		"HIVE_FORK_BYZANTIUM":      2000,
		"HIVE_FORK_CONSTANTINOPLE": 2000,
		"HIVE_FORK_PETERSBURG":     2000,
		"HIVE_FORK_ISTANBUL":       2000,
	},
	"Byzantium": {
		"HIVE_FORK_HOMESTEAD": 0,
		//"HIVE_FORK_DAO_BLOCK":      2000,
		"HIVE_FORK_TANGERINE":      0,
		"HIVE_FORK_SPURIOUS":       0,
		"HIVE_FORK_BYZANTIUM":      0,
		"HIVE_FORK_CONSTANTINOPLE": 2000,
		"HIVE_FORK_PETERSBURG":     2000,
		"HIVE_FORK_ISTANBUL":       2000,
	},
	"Constantinople": {
		"HIVE_FORK_HOMESTEAD": 0,
		//"HIVE_FORK_DAO_BLOCK":      2000,
		"HIVE_FORK_TANGERINE":      0,
		"HIVE_FORK_SPURIOUS":       0,
		"HIVE_FORK_BYZANTIUM":      0,
		"HIVE_FORK_CONSTANTINOPLE": 0,
		"HIVE_FORK_PETERSBURG":     2000,
		"HIVE_FORK_ISTANBUL":       2000,
	},
	"ConstantinopleFix": {
		"HIVE_FORK_HOMESTEAD": 0,
		//"HIVE_FORK_DAO_BLOCK":      2000,
		"HIVE_FORK_TANGERINE":      0,
		"HIVE_FORK_SPURIOUS":       0,
		"HIVE_FORK_BYZANTIUM":      0,
		"HIVE_FORK_CONSTANTINOPLE": 0,
		"HIVE_FORK_PETERSBURG":     0,
		"HIVE_FORK_ISTANBUL":       2000,
	},
	"Istanbul": {
		"HIVE_FORK_HOMESTEAD": 0,
		//"HIVE_FORK_DAO_BLOCK":      2000,
		"HIVE_FORK_TANGERINE":      0,
		"HIVE_FORK_SPURIOUS":       0,
		"HIVE_FORK_BYZANTIUM":      0,
		"HIVE_FORK_CONSTANTINOPLE": 0,
		"HIVE_FORK_PETERSBURG":     0,
		"HIVE_FORK_ISTANBUL":       0,
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
	},
	"HomesteadToEIP150At5": {
		"HIVE_FORK_HOMESTEAD":      0,
//		"HIVE_FORK_DAO_BLOCK":      2000,
		"HIVE_FORK_TANGERINE":      5,
		"HIVE_FORK_SPURIOUS":       2000,
		"HIVE_FORK_BYZANTIUM":      2000,
		"HIVE_FORK_CONSTANTINOPLE": 2000,
		"HIVE_FORK_PETERSBURG":     2000,
		"HIVE_FORK_ISTANBUL":       2000,
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
	},
}

func init() {
	//support the hive testsuite engine provider
	hive.Support()
}

func deliverTests(root string, limit int) chan *testcase {
	out := make(chan *testcase)
	var i, j = 0, 0
	go func() {
		filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if limit >= 0 && i >= limit {
				return nil
			}
			if info.IsDir() {
				return nil
			}
			if fname := info.Name(); !strings.HasSuffix(fname, ".json") {
				return nil
			}
			tests := make(map[string]BlockTest)
			data, err := ioutil.ReadFile(path)
			if err = json.Unmarshal(data, &tests); err != nil {
				fmt.Println("ERROR:" + path)
				fmt.Println(err)
				log.Error("error", "err", err)
				//return err
			} else {
				j = j + 1
				for name, blocktest := range tests {
					// t is declared explicitly here, if implicit := - declaration is used,
					// golang will reuse the underlying object, and overwrite the object while it's being tested
					// by a separate thread.
					// That is also the reason that blocktest within the struct is by-value instead of by-reference
					var t testcase
					t = testcase{blockTest: blocktest, name: name, filepath: path}
					if err := t.validate(); err != nil {
						log.Error("error", "err", err, "test", t.name)
						continue
					}
					i = i + 1
					out <- &t
				}
			}
			return nil
		})
		log.Info("file iterator done", "files", j, "tests", i)
		close(out)
	}()
	return out
}

type blocktestExecutor struct {
	api         common.TestSuiteHost
	testSuiteID common.TestSuiteID
	client      string
}

type testcase struct {
	name      string
	blockTest BlockTest

	filepath string
}

// validate returns error if the network is not defined
func (t *testcase) validate() error {
	net := t.blockTest.json.Network
	if _, exist := ruleset[net]; !exist {
		return fmt.Errorf("network `%v` not defined in ruleset", net)
	}
	return nil
}

// updateEnv sets environment variables from the test
func (t *testcase) updateEnv(env map[string]string) {
	// Environment variables for rules
	rules := ruleset[t.blockTest.json.Network]
	for k, v := range rules {
		env[k] = fmt.Sprintf("%d", v)
	}
	// Possibly disable POW
	if t.blockTest.json.SealEngine == "NoProof" {
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

func (t *testcase) artefacts() (string, string, string, []string, error) {
	var blocks []string
	key := fmt.Sprintf("%x", sha1.Sum([]byte(fmt.Sprintf("%s%s", t.filepath, t.name))))
	rootFolder := fmt.Sprintf("./%s/", key)
	blockFolder := fmt.Sprintf("%s/blocks", rootFolder)

	if err := os.Mkdir(fmt.Sprintf("./%s", key), 0700); err != nil {
		return "", "", "", nil, err
	}
	if err := os.Mkdir(blockFolder, 0700); err != nil {
		return "", "", "", nil, err
	}
	genesis := toGethGenesis(&(t.blockTest.json))
	genBytes, _ := json.Marshal(genesis)
	genesisFile := fmt.Sprintf("./%v/genesis.json", key)
	if err := ioutil.WriteFile(genesisFile, genBytes, 0777); err != nil {
		return "", "", "", nil, fmt.Errorf("Failed writing genesis: `%v`", err)
	}

	for i, block := range t.blockTest.json.Blocks {
		rlpdata := common2.FromHex(block.Rlp)
		fname := fmt.Sprintf("%s/%04d.rlp", blockFolder, i+1)
		blocks = append(blocks, fname)
		if err := ioutil.WriteFile(fname, rlpdata, 0777); err != nil {
			return "", "", "", nil, fmt.Errorf("failed writing block %d: %v", i, err)
		}
	}
	//log.Info("Test artefacts", "testname", t.name, "testfile", t.filepath, "blockfolder", blockFolder)
	return rootFolder, genesisFile, "", blocks, nil
}

func (t *testcase) verifyGenesis(got []byte) error {
	if exp := t.blockTest.json.Genesis.Hash; bytes.Compare(exp[:], got) != 0 {
		return fmt.Errorf("Genesis mismatch, expected `0x%x` got `0x%x`", exp, got)
	}
	return nil
}
func (t *testcase) verifyBestblock(got []byte) error {
	if exp := t.blockTest.json.BestBlock; bytes.Compare(exp[:], got) != 0 {
		return fmt.Errorf("Last block mismatch, expected `0x%x` got `0x%x` (`%v` `%v`)", exp, got)
	}
	return nil
}

func (be *blocktestExecutor) run(testChan chan *testcase) {
	var i = 0
	for t := range testChan {

		if err := be.runTest(t); err != nil {
			log.Error("error", "err", err)
		}
		i++

	}
	log.Info("executor finished", "num_executed", i)
}

func (be *blocktestExecutor) runTest(t *testcase) error {

	log.Info("Starting test", "name", t.name, "file", t.filepath)
	start := time.Now()
	// Convert ./tests/BlockchainTests/InvalidBlocks/bcExpectSection/lastblockhash.json
	// into
	// BlockchainTests/InvalidBlocks/bcExpectSection/lastblockhash.json
	filePath := strings.TrimPrefix(t.filepath, ".")
	filePath = strings.TrimPrefix(filePath, "/tests/")
	// Link to
	// https://github.com/ethereum/tests/blob/develop/BlockchainTests/InvalidBlocks/bcExpectSection/lastblockhash.json
	//testID, err := be.api.StartTest(be.testSuiteID, t.name, testname)
	description := fmt.Sprintf("Test source:[`%v`](https://github.com/ethereum/tests/blob/develop/%v)",
		t.name,
		filePath)

	testID, err := be.api.StartTest(be.testSuiteID, t.name, description)
	if err != nil {
		return err
	}

	var done = func() {
		var (
			errString = ""
			success   = (err == nil)
		)
		if !success {
			errString = err.Error()
		}
		be.api.EndTest(be.testSuiteID, testID, &common.TestResult{success, errString}, nil)
	}
	defer done()

	root, genesis, _, blocks, err := t.artefacts()
	genesisTarget := strings.Replace(genesis, root, "", 1)
	if err != nil {
		return err
	}
	env := map[string]string{
		"CLIENT":             be.client,
		"HIVE_FORK_DAO_VOTE": "1",
		"HIVE_CHAIN_ID":      "1",
	}
	files := map[string]string{
		genesisTarget: genesis,
	}

	for _, filename := range blocks {
		fileTarget := strings.Replace(filename, root, "", 1)
		files[fileTarget] = filename
	}
	// update the parameters with test-specific stuff
	t.updateEnv(env)
	t1 := time.Now()
	// spin up a node
	_, ip, _, err := be.api.GetNode(be.testSuiteID, testID, env, files)
	if err != nil {
		return err
	}
	t2 := time.Now()
	ctx := context.Background()
	rawClient, err := rpc.DialContext(ctx, fmt.Sprintf("http://%s:8545", ip.String()))
	if err != nil {
		err = fmt.Errorf("Failed to start client: `%v`", err)
		return err
	}
	genesisHash, err := getHash(rawClient, hexutil.EncodeBig(new(big.Int)))
	if err != nil {
		err = fmt.Errorf("Failed to check genesis: `%v`", err)
		return err
	}
	t3 := time.Now()

	if err = t.verifyGenesis(genesisHash); err != nil {
		return err
	}
	t4 := time.Now()
	// verify postconditions
	lastHash, err := getHash(rawClient, "latest")
	if err != nil {
		return err
	}
	t5 := time.Now()
	if err = t.verifyBestblock(lastHash); err != nil {
		return err
	}
	t6 := time.Now()
	log.Info("Test done", "name", t.name, "artefacts", t1.Sub(start), "newnode", t2.Sub(t1), "getGenesis", t3.Sub(t2), "verifyGenesis", t4.Sub(t3), "getLatest", t5.Sub(t4), "verifyLatest", t6.Sub(t5))
	return nil
}

func getHash(rawClient *rpc.Client, arg string) ([]byte, error) {
	blockData := make(map[string]interface{})
	if err := rawClient.Call(&blockData, "eth_getBlockByNumber", arg, false); err != nil {
		// Make one more attempt
		log.Info("Client connect failed, making one more attempt...")
		if err = rawClient.Call(&blockData, "eth_getBlockByNumber", arg, false); err != nil {
			return nil, err
		}
	}
	if hash, exist := blockData["hash"]; !exist {
		return nil, fmt.Errorf("No hash found in response")
	} else {
		if hexHash, ok := hash.(string); !ok {
			return nil, fmt.Errorf("error: string conversion failed for `%v`", hash)
		} else {
			return common2.HexToHash(hexHash).Bytes(), nil
		}
	}
}
func main() {
	paralellism := 16
	log.Root().SetHandler(log.StdoutHandler)
	if val, ok := os.LookupEnv("HIVE_PARALLELISM"); ok {
		if p, err := strconv.Atoi(val); err != nil {
			log.Warn("Hive paralellism could not be converted to int", "error", err)
		} else {
			paralellism = p
		}
	}
	testLimit := -1
	if val, ok := os.LookupEnv("HIVE_SIMLIMIT"); ok {
		if p, err := strconv.Atoi(val); err != nil {
			log.Warn("Simulator test limit could not be converted to int", "error", err)
		} else {
			testLimit = p
		}
	}
	log.Info("Hive simulator started.", "paralellism", paralellism, "testlimit", testLimit)

	// get the test suite engine provider and initialise
	simProviderType := flag.String("simProvider", "", "the simulation provider type (local|hive)")
	providerConfigFile := flag.String("providerConfig", "", "the config json file for the provider")
	flag.Parse()
	host, err := common.InitProvider(*simProviderType, *providerConfigFile)
	if err != nil {
		log.Error(fmt.Sprintf("Unable to initialise provider %s", err.Error()))
		os.Exit(1)
	}

	testpath, isset := os.LookupEnv("TESTPATH")
	if !isset {
		log.Error("Test path not set ($TESTPATH)")
		os.Exit(1)
	}

	availableClients, _ := host.GetClientTypes()

	log.Info("Got clients", "clients", availableClients)
	logFile, _ := os.LookupEnv("HIVE_SIMLOG")

	fileRoot := fmt.Sprintf("%s/BlockchainTests/", testpath)
	for _, client := range availableClients {
		testSuiteID, err := host.StartTestSuite("Consensus",
			fmt.Sprintf("The `consensus` test suite executes BlockchainTests from the "+
				"offical test repository: [here](https://github.com/ethereum/tests). For every test, it starts an instance of the client-under-test, "+
				"(in this case, `%v`), and makes it import the RLP blocks. After import phase, the node is queried about it's latest blocks, which is matched"+
				"to the expected last blockhash according to the test.", client), logFile)
		if err != nil {
			log.Error("Unable to start test suite", "error", err)
			os.Exit(1)
		}
		defer func() {
			if err := host.EndTestSuite(testSuiteID); err != nil {
				log.Error("Unable to end test suite", "error", err)
				os.Exit(1)
			}
		}()

		testCh := deliverTests(fileRoot, testLimit)
		var wg sync.WaitGroup
		for i := 0; i < paralellism; i++ {
			wg.Add(1)
			go func() {
				b := blocktestExecutor{api: host, testSuiteID: testSuiteID, client: client}
				b.run(testCh)
				wg.Done()
			}()
		}
		log.Info("Tests started", "num threads", paralellism)
		wg.Wait()
	}
}
