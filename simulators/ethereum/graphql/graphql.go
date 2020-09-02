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
			if fname := info.Name(); !strings.HasSuffix(fname, ".json") {
				return nil
			}

			data, err := ioutil.ReadFile(filepath)
			if err != nil {
				log.Error("error", "err", err)
				return nil
			}
			var gqlTest graphQLTest
			if err = json.Unmarshal(data, &gqlTest); err != nil {
				log.Error("failed to unmarshal", "name", info.Name(), "error", err)
				return nil
			}
			i = i + 1
			t := testcase{
				name:    strings.TrimSuffix(info.Name(), path.Ext(info.Name())),
				gqlTest: &gqlTest,
			}
			out <- &t
			return nil
		})
		log.Info("test iterator done", "tests", i)
		close(out)
	}()
	return out
}

type testcase struct {
	name    string
	gqlTest *graphQLTest
}
type graphQLTest struct {
	Request    string      `json:"request"`
	Response   interface{} `json:"response"`
	StatusCode int         `json:"statusCode"`
}

func run(testChan chan *testcase, host common.TestSuiteHost, suiteID common.TestSuiteID, client string) {

	// The graphql chain comes from the Besu codebase, and is built on Frontier
	env := map[string]string{
		"CLIENT":               client,
		"HIVE_FORK_DAO_VOTE":   "1",
		"HIVE_CHAIN_ID":        "1",
		"HIVE_GRAPHQL_ENABLED": "1",
	}
	files := map[string]string{
		"genesis.json": "/init/testGenesis.json",
		"chain.rlp":    "/init/testBlockchain.blocks",
	}
	// The graphql tests can all execute on the same client container, no need to spin up new ones for every test
	//
	// However, the API to StartTest(.., testID , ... ) wants a test-id, so we need to ensure that we can indeed reuse
	// an existing container and it doesn't get torn down by the call to host.EndTest
	// See simulator.go: 398.
	//
	// To get around the quirk above, we use a meta-test
	metaTestId, err := host.StartTest(suiteID, "1. Client Instantiation and container logs", "This is a meta-test, which only checks that "+
		"the *client-under-test* can be instantiated. \n\nIf this fails, no other tests are executed. "+
		"This test contains all docker logs pertaining to the *client-under-test*, since the graphql tests"+
		" are all executed against the same instance(s)")
	var testsOnThisMetaTest []string
	metaTestResult := &common.TestResult{Pass: false}
	defer func() {
		var info = "The following tests were made against this instance:\n"
		for i := 0; i < len(testsOnThisMetaTest); i++ {
			info = fmt.Sprintf("%s\n  * `%v`", info, testsOnThisMetaTest[i])
		}
		metaTestResult.AddDetail(info)
		host.EndTest(suiteID, metaTestId, metaTestResult, nil)
	}()
	// The hive parallelism determines how many instances are used. We should make note, in the meta-test,
	// which ones were executed against this particular instance
	if err != nil {
		log.Error("error", "err", err)
		return
	}
	_, ip, _, err := host.GetNode(suiteID, metaTestId, env, files)
	if err != nil {
		log.Error("error", "err", err)
		metaTestResult.Details = fmt.Sprintf("Error occurred: %v", err)
		return
	}

	var i = 0
	for t := range testChan {
		testsOnThisMetaTest = append(testsOnThisMetaTest, t.name)
		if err := prepareRunTest(t, host, suiteID, ip); err != nil {
			log.Error("error", "err", err)
		}
		i++
	}
	metaTestResult.Pass = true
	metaTestResult.Details = "Client instantiated OK"
	log.Info("executor finished", "num_executed", i)
}

// prepareRunTest administers the hive-specific test stuff, registering the suite and reporting back the suite results
func prepareRunTest(t *testcase, host common.TestSuiteHost, suiteID common.TestSuiteID, ip net.IP) error {

	log.Info("Starting test", "name", t.name)

	testID, err := host.StartTest(suiteID, t.name,
		fmt.Sprintf("Testcase [source](https://github.com/ethereum/hive/blob/master/simulators/ethereum/graphql/testcases/%v.json)", t.name))
	if err != nil {
		host.EndTest(suiteID, testID, &common.TestResult{false, err.Error()}, nil)
		return err
	}
	if testErr := runTest(ip, t.gqlTest); testErr != nil {
		host.EndTest(suiteID, testID, &common.TestResult{false, testErr.Error()}, nil)
		return testErr
	}
	host.EndTest(suiteID, testID, &common.TestResult{Pass: true}, nil)
	return nil
}

// runTest does the actual testing: querying the client-under-test. It executes after a client has been
// successfully instantiated
func runTest(ip net.IP, t *graphQLTest) error {
	type qlQuery struct {
		Query string `json:"query"`
	}
	// Example of working queries:
	// curl 'http://127.0.0.1:8547/graphql' --data-binary '{"query":"query blockNumber {\n  block {\n    number\n  }\n}\n"}'
	// curl 'http://127.0.0.1:8547/graphql' --data-binary '{"query":"query blockNumber {\n  block {\n    number\n  }\n}\n","variables":null,"operationName":"blockNumber"}'
	postData, err := json.Marshal(qlQuery{Query: t.Request})
	if err != nil {
		return err
	}
	resp, err := http.Post(fmt.Sprintf("http://%s:8550/graphql", ip.String()),
		"application/json",
		bytes.NewReader(postData))
	if err != nil {
		return err
	}
	respBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	resp.Body.Close()

	if resp.StatusCode != t.StatusCode {
		return fmt.Errorf("Node HTTP response was `%d`, expected `%d`", resp.StatusCode, t.StatusCode)
	}
	if resp.StatusCode != 200 {
		// We don't bother to check the exact error messages, those aren't fully specified
		return nil
	}
	var got interface{}
	if err = json.Unmarshal(respBytes, &got); err != nil {
		return fmt.Errorf("Error marshalling response :: %s", err.Error())
	}
	if !reflect.DeepEqual(t.Response, got) {
		expResponse, _ := json.MarshalIndent(t.Response, "", "  ")

		return fmt.Errorf("Test failed. Query:\n```\n%v\n```\n"+
			"Expected: \n```\n%s\n```\n"+
			"Got:\n```\n%s\n```\n"+
			"HTTP response status: `%s`", t.Request, expResponse, respBytes, resp.Status)
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
		testSuiteID, err := host.StartTestSuite("graphql", "Test suite covering the graphql API surface."+
			"The GraphQL tests were initially imported from the Besu codebase.", logFile)
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
