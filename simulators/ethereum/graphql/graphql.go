package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync"

	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/hive/simulators/common"
	"github.com/ethereum/hive/simulators/common/providers/hive"
	"github.com/ethereum/hive/simulators/common/providers/local"
)

func init() {
	//support providers
	hive.Support()
	local.Support()
}

func deliverTests(limit int) chan *testcase {
	out := make(chan *testcase)
	var i = 0
	go func() {
		filepath.Walk("/testcases", func(filepath string, info os.FileInfo, err error) error {
			if limit >= 0 && i >= limit {
				return nil
			}
			if info.IsDir() {
				return nil
			}
			if fname := info.Name(); !strings.HasSuffix(fname, ".graphql") {
				return nil
			}

			graphql, err := ioutil.ReadFile(filepath)
			if err != nil {
				log.Error("error", "err", err)
				return nil
			}

			testname := strings.TrimSuffix(info.Name(), path.Ext(info.Name()))
			expectedfn := strings.TrimSuffix(filepath, path.Ext(filepath)) + ".json"
			expected, err := ioutil.ReadFile(expectedfn)
			if err != nil {
				log.Error("error", "err", err)
				return nil
			}

			t := testcase{name: testname, graphql: graphql, expected: expected}

			i = i + 1
			out <- &t

			return nil
		})
		log.Info("test iterator done", "tests", i)
		close(out)
	}()
	return out
}

type testcase struct {
	name     string
	graphql  []byte
	expected []byte
}

func run(testChan chan *testcase, host common.TestSuiteHost, suiteID common.TestSuiteID, client string) {
	var i = 0
	for t := range testChan {
		if err := prepareRunTest(t, host, suiteID, client); err != nil {
			log.Error("error", "err", err)
		}
		i++
	}
	log.Info("executor finished", "num_executed", i)
}

// prepareRunTest administers the hive-specific test stuff, registering the suite and reporting back the suite results
func prepareRunTest(t *testcase, host common.TestSuiteHost, suiteID common.TestSuiteID, client string) error {

	log.Info("Starting test", "name", t.name)

	testID, err := host.StartTest(suiteID, t.name, "")
	if err != nil {
		return err
	}
	// The graphql chain comes from the Besu codebase, and is built on Frontier
	env := map[string]string{
		"CLIENT":             client,
		"HIVE_FORK_DAO_VOTE": "1",
		"HIVE_CHAIN_ID":      "1",
	}
	files := map[string]string{
		"genesis.json": "/init/testGenesis.json",
		"chain.rlp":    "/init/testBlockchain.blocks",
	}
	_, ip, _, err := host.GetNode(suiteID, testID, env, files)
	if err != nil {
		host.EndTest(suiteID, testID, &common.TestResult{false, err.Error()}, nil)
		return err
	}
	if testErr := runTest(ip, t); testErr != nil {
		host.EndTest(suiteID, testID, &common.TestResult{false, testErr.Error()}, nil)
		return testErr
	}
	host.EndTest(suiteID, testID, &common.TestResult{Pass: true}, nil)
	return nil
}

// runTest does the actual testing: querying the client-under-test. It executes after a client has been
// successfully instantiated
func runTest(ip net.IP, t *testcase) error {
	type qlQuery struct {
		Query string `json:"query"`
	}

	// Example of working queries:
	// curl 'http://127.0.0.1:8547/graphql' --data-binary '{"query":"query blockNumber {\n  block {\n    number\n  }\n}\n"}'
	// curl 'http://127.0.0.1:8547/graphql' --data-binary '{"query":"query blockNumber {\n  block {\n    number\n  }\n}\n","variables":null,"operationName":"blockNumber"}'

	postData, err := json.Marshal(qlQuery{string(t.graphql)})
	if err != nil {
		return err
	}
	resp, err := http.Post(fmt.Sprintf("http://%s:8547/graphql", ip.String()),
		"application/json",
		bytes.NewReader(postData))
	respBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	//verify the response matches expected
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Node HTTP response not 200 OK, got: %v, response: %v", resp.Status, string(respBytes))
	}
	exp, got := string(t.expected), string(respBytes)
	if res, err := areEqualJSON(got, exp); err != nil {
		return err
	} else if !res {
		return fmt.Errorf("Test failed. Query:\n```\n%v\n```\nexpected: \n```\n%s\n```\n got:\n```\n%s\n```\n", string(t.graphql), exp, got)
	}
	return nil
}

func main() {
	var (
		paralellism = 16
		testLimit   = -1
	)

	log.Root().SetHandler(log.StdoutHandler)
	if val, ok := os.LookupEnv("HIVE_PARALLELISM"); ok {
		if p, err := strconv.Atoi(val); err != nil {
			log.Warn("Hive paralellism could not be converted to int", "error", err)
		} else {
			paralellism = p
		}
	}
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

	availableClients, _ := host.GetClientTypes()
	log.Info("Got clients", "clients", availableClients)
	logFile, _ := os.LookupEnv("HIVE_SIMLOG")

	for _, client := range availableClients {
		testSuiteID, err := host.StartTestSuite("graphql", "graphql test suite covering the graphql API surface", logFile)
		if err != nil {
			log.Error(fmt.Sprintf("Unable to start test suite: %s", err.Error()), err.Error())
			os.Exit(1)
		}
		defer func() {
			if err := host.EndTestSuite(testSuiteID); err != nil {
				log.Error(fmt.Sprintf("Unable to end test suite: %s", err.Error()), err.Error())
				os.Exit(1)
			}
		}()

		testCh := deliverTests(testLimit)
		var wg sync.WaitGroup
		for i := 0; i < paralellism; i++ {
			wg.Add(1)
			go func() {
				run(testCh, host, testSuiteID, client)
				wg.Done()
			}()
		}
		log.Info("Tests started", "num threads", 16)
		wg.Wait()
	}
}

func areEqualJSON(s1, s2 string) (bool, error) {
	var o1 interface{}
	var o2 interface{}

	var err error
	err = json.Unmarshal([]byte(s1), &o1)
	if err != nil {
		return false, fmt.Errorf("Error mashalling string 1 :: %s", err.Error())
	}
	err = json.Unmarshal([]byte(s2), &o2)
	if err != nil {
		return false, fmt.Errorf("Error mashalling string 2 :: %s", err.Error())
	}

	return reflect.DeepEqual(o1, o2), nil
}
