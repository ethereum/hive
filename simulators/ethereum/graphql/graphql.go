package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync"

	"github.com/ethereum/hive/simulators/common/providers/hive"
	"github.com/ethereum/hive/simulators/common/providers/local"

	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/hive/simulators/common"
)

type envvars map[string]int

func init() {
	//support providers
	hive.Support()
	local.Support()
}

func deliverTests() chan *testcase {
	out := make(chan *testcase)
	var i = 0
	go func() {
		filepath.Walk("/testcases", func(filepath string, info os.FileInfo, err error) error {
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
		if err := runTest(t, host, suiteID, client); err != nil {
			log.Error("error", "err", err)
		}
		i++
	}
	log.Info("executor finished", "num_executed", i)
}

func runTest(t *testcase, host common.TestSuiteHost, suiteID common.TestSuiteID, client string) error {

	log.Info("Starting test", "name", t.name)

	testID, err := host.StartTest(suiteID, t.name, "")
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
		host.EndTest(suiteID, testID, &common.TestResult{success, errString}, nil)
	}
	defer done()

	env := map[string]string{
		"CLIENT":                   client,
		"HIVE_FORK_DAO_VOTE":       "1",
		"HIVE_CHAIN_ID":            "1",
		"HIVE_FORK_HOMESTEAD":      "0",
		"HIVE_FORK_TANGERINE":      "0",
		"HIVE_FORK_SPURIOUS":       "0",
		"HIVE_FORK_BYZANTIUM":      "0",
		"HIVE_FORK_CONSTANTINOPLE": "0",
		"HIVE_FORK_PETERSBURG":     "0",
	}
	files := map[string]string{
		"genesis.json": "/init/testGenesis.json",
		"chain.rlp":    "/init/testBlockchain.blocks",
	}

	_, ip, _, err := host.GetNode(suiteID, testID, env, files)
	if err != nil {
		return err
	}

	nodeQuery := fmt.Sprintf("http://%s:8547/graphql/query=%s", ip.String(), string(t.graphql))
	resp, err := http.Get(nodeQuery)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	//verify the response matches expected
	if resp.StatusCode == http.StatusOK {
		jsonbytes, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		jsonstring := string(jsonbytes)

		expected := string(t.expected)

		res, err := areEqualJSON(jsonstring, string(t.expected))
		if err != nil {
			return err
		}
		if !res {
			return fmt.Errorf("Test failed, expected %s ...got.. %s  ", expected, jsonstring)
		}

	} else {
		return fmt.Errorf("Error sending request %s", strconv.Itoa(resp.StatusCode))
	}

	return nil
}

func main() {

	log.Info("Hive simulator started.")

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

	for _, client := range availableClients {
		testSuiteID, err := host.StartTestSuite("graphql", "graphql test suite covering the graphql API surface")
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

		testCh := deliverTests()
		var wg sync.WaitGroup
		for i := 0; i < 16; i++ {
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
