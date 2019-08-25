package main

import (
	"flag"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/ethereum/hive/simulators/common/providers/local"

	"github.com/ethereum/hive/simulators/common/providers/hive"

	"os"
	"strings"
	"testing"

	geth "github.com/ethereum/go-ethereum/mobile"
	"github.com/ethereum/go-ethereum/p2p/enode"
	rpc "github.com/ethereum/go-ethereum/rpc"
	"github.com/ethereum/hive/simulators/common"
)

var (
	host        common.TestSuiteHost
	syncTimeout = 60 * time.Second //the number of seconds before a sync is considered stalled or failed
	testSuite   common.TestSuiteID
)

type testCase struct {
	Client string
}

func init() {
	hive.Support()
	local.Support() // note this can be supported because the different geth instances are identified by their HIVE_NODETYPE parameter
}

func TestMain(m *testing.M) {

	//Max Concurrency is specified in the parallel flag, which is supplied to the simulator container

	simProviderType := flag.String("simProvider", "", "the simulation provider type (local|hive)")
	providerConfigFile := flag.String("providerConfig", "", "the config json file for the provider")

	flag.Parse()
	var err error
	host, err = common.InitProvider(*simProviderType, *providerConfigFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to initialise provider %s", err.Error())
		os.Exit(1)
	}

	os.Exit(RunTestSuite(m))
}

func RunTestSuite(m *testing.M) int {
	return m.Run()
}

func TestSyncsWithGeth(t *testing.T) {

	//start the test suite
	testSuite, err := host.StartTestSuite("Sync test suite", "This suite of tests verifies that clients can sync from each other in different modes.")
	if err != nil {
		t.Fatalf("Simulator error. Failed to start test suite. %v ", err)
	}
	defer host.EndTestSuite(testSuite)

	//Test a fastmode sync by creating a geth client and one each of any other type of client
	//and ask the other client to load the chain
	t.Run("fastmodes", func(t *testing.T) {

		//get all client types required to test
		availableClients, err := host.GetClientTypes()
		if err != nil {
			t.Fatalf("Simulator error. Cannot get client types. %v", err)
		}
		//there needs to be only 2 types of client
		if len(availableClients) < 2 {
			t.Fatal("Cannot execute test with less than 2 types available. This test tries to sync from each other peer to geth.")
		}
		//start with the first geth
		var firstGeth = -1
		for i, clientType := range availableClients {
			if strings.Contains(clientType, "go-ethereum") {
				firstGeth = i
				break
			}
		}
		if firstGeth == -1 {
			t.Fatal("Missing geth instance. This test requires a geth.")
		}
		startTime := time.Now()

		testID, err := host.StartTest(testSuite, "From geth", "This test initialises an instance of geth with a predefined chain and genesis (configured to run in fast-sync) and then attempts to sync multiple clients from that one.")
		if err != nil {
			t.Fatalf("Unable to start test: %s", err.Error())
		}
		// declare empty test results
		var summaryResult common.TestResult
		summaryResult.Pass = true
		clientResults := make(common.TestClientResults)
		//make sure the test ends
		defer func() {
			host.EndTest(testSuite, testID, &summaryResult, clientResults)
			//send test failures to standard outputs too
			if !summaryResult.Pass {
				t.Errorf("Test failed %s", summaryResult.Details)
			}
		}()
		//and load the genesis without a chain
		mainParms := map[string]string{
			"CLIENT":                  availableClients[firstGeth],
			"HIVE_USE_GENESIS_CONFIG": "1",
			"HIVE_NODETYPE":           "full", //fast sync
		}
		mainFiles := map[string]string{
			"HIVE_INIT_GENESIS": "/simplechain/genesis.json",
		}
		_, mainNodeIP, _, err := host.GetNode(testSuite, testID, mainParms, mainFiles)
		if err != nil {
			summaryResult.Pass = false
			summaryResult.AddDetail(fmt.Sprintf("Unable to get main node: %s", err.Error()))
			return
		}
		mainNodeURL := fmt.Sprintf("http://%s:8545", mainNodeIP.String())

		//the main client will be asked to sync with the other client.
		for i, clientType := range availableClients {
			if i != firstGeth {

				t.Logf("Starting peer node (%s)", clientType)
				// create the other node
				parms := map[string]string{
					"CLIENT": clientType,

					"HIVE_NETWORK_ID":     "1",
					"HIVE_CHAIN_ID":       "1",
					"HIVE_FORK_HOMESTEAD": "0",
					"HIVE_FORK_TANGERINE": "0",
					"HIVE_FORK_SPURIOUS":  "0",
					"HIVE_FORK_BYZANTIUM": "0",
					//	"HIVE_FORK_DAO_BLOCK": "0",
					// "HIVE_FORK_CONSTANTINOPLE": "0",
				}
				files := map[string]string{
					"HIVE_INIT_GENESIS": "/simplechain/genesis.json",
					"HIVE_INIT_CHAIN":   "/simplechain/chain.rlp",
				}
				clientID, nodeIP, _, err := host.GetNode(testSuite, testID, parms, files)
				if err != nil {
					summaryResult.Pass = false
					summaryResult.AddDetail(fmt.Sprintf("Unable to get node: %s", err.Error()))
					return
				}

				syncClient(nil, mainNodeURL, clientID, nodeIP, t, 10, startTime, true, testID, &summaryResult, clientResults)

			}
		}

		t.Log("Terminating.")

	})

	//Test sync compatibility by creating a geth node and load the chain there,
	//then create one each of each remaining type of node and sync from geth
	t.Run("compatibility", func(t *testing.T) {

		//get all client types required to test
		availableClients, err := host.GetClientTypes()
		if err != nil {
			t.Fatalf("Simulator error. Cannot get client types. %v", err)
		}
		//there needs to be at least 2 types of client
		if len(availableClients) < 2 {
			t.Fatal("Cannot execute test. There needs to be at least 2 types of client available.")
		}
		//start with the first geth
		var firstGeth = -1
		for i, clientType := range availableClients {
			if strings.Contains(clientType, "go-ethereum") {
				firstGeth = i
				break
			}
		}
		if firstGeth == -1 {
			t.Fatal("Cannot execute test. No geth client available.")
		}

		startTime := time.Now()
		testID, err := host.StartTest(testSuite, "From geth", "This test initialises an instance of geth with a predefined chain and genesis (configured to run in fast-sync) and then attempts to sync multiple clients from that one.")
		if err != nil {
			t.Fatalf("Unable to start test: %s", err.Error())
		}
		// declare empty test results
		var summaryResult common.TestResult
		summaryResult.Pass = true
		clientResults := make(common.TestClientResults)
		//make sure the test ends
		defer func() {
			host.EndTest(testSuite, testID, &summaryResult, clientResults)
			//send test failures to standard outputs too
			if !summaryResult.Pass {
				t.Errorf("Test failed %s", summaryResult.Details)
			}
		}()
		//and load the genesis + chain
		mainParms := map[string]string{
			"CLIENT":                  availableClients[firstGeth],
			"HIVE_USE_GENESIS_CONFIG": "1",
		}

		mainFiles := map[string]string{
			"HIVE_INIT_GENESIS": "/simplechain/genesis.json",
			"HIVE_INIT_CHAIN":   "/simplechain/chain.rlp",
		}

		_, mainNodeIP, _, err := host.GetNode(testSuite, testID, mainParms, mainFiles)
		if err != nil {
			summaryResult.Pass = false
			summaryResult.AddDetail(fmt.Sprintf("Unable to get main node: %s", err.Error()))
			return
		}
		mainNodeURL := fmt.Sprintf("http://%s:8545", mainNodeIP.String())

		//the remaining clients will be asked to sync with the initial
		//client. this waitgroup is for the concurrent syncprogress checks.
		//the test will continue after all the clients have finished syncing.
		var wg sync.WaitGroup

		// for all others
		for i, clientType := range availableClients {
			if i != firstGeth {

				t.Logf("Starting peer node (%s)", clientType)
				// create the other node with genesis and no chain
				parms := map[string]string{
					"CLIENT": clientType,

					"HIVE_NETWORK_ID":     "1",
					"HIVE_CHAIN_ID":       "1",
					"HIVE_FORK_HOMESTEAD": "0",
					"HIVE_FORK_TANGERINE": "0",
					"HIVE_FORK_SPURIOUS":  "0",
					"HIVE_FORK_BYZANTIUM": "0",
					//	"HIVE_FORK_DAO_BLOCK": "0",
					// "HIVE_FORK_CONSTANTINOPLE": "0",
				}

				files := map[string]string{
					"HIVE_INIT_GENESIS": "/simplechain/genesis.json",
				}

				clientID, nodeIP, _, err := host.GetNode(testSuite, testID, parms, files)
				if err != nil {
					summaryResult.Pass = false
					summaryResult.AddDetail(fmt.Sprintf("Unable to get main node: %s", err.Error()))
					return
				}

				wg.Add(1)
				go syncClient(&wg, mainNodeURL, clientID, nodeIP, t, 10, startTime, false, testID, &summaryResult, clientResults)

			}
		}

		wg.Wait()

	})

}

func syncClient(wg *sync.WaitGroup, mainURL, clientID string, nodeIP net.IP, t *testing.T, chainLength int, startTime time.Time, checkMainForSync bool, testID common.TestID, summaryResult *common.TestResult, clientResults common.TestClientResults) {
	if wg != nil {
		defer wg.Done()
	}
	peerEnodeID, err := host.GetClientEnode(testSuite, testID, clientID)
	if err != nil || peerEnodeID == nil || *peerEnodeID == "" {
		summaryResult.Pass = false
		clientResults.AddResult(clientID, false, "Enode could not be obtained.")
		return
	}
	peerNode, err := enode.ParseV4(*peerEnodeID)
	if err != nil {
		summaryResult.Pass = false
		clientResults.AddResult(clientID, false, "Enode could not be parsed.")
		return
	}
	//replace the ip with what the provider says it is
	tcpPort := peerNode.TCP()
	if tcpPort == 0 {
		tcpPort = 30303
	}
	peerNode = enode.NewV4(peerNode.Pubkey(), nodeIP, tcpPort, 30303)
	if peerNode == nil {
		summaryResult.Pass = false
		clientResults.AddResult(clientID, false, "Enode could not be created.")
		return
	}

	*peerEnodeID = peerNode.String()

	//connect over rpc to the main node to add the peer (the mobile client does not have the admin function)
	rpcClient, err := rpc.Dial(mainURL)
	if err != nil {
		summaryResult.Pass = false
		clientResults.AddResult(clientID, false, "Enode could not be created.")
		return
	}
	var res = false
	if err := rpcClient.Call(&res, "admin_addPeer", *peerEnodeID); err != nil {
		summaryResult.Pass = false
		clientResults.AddResult(clientID, false, "Admin add peer failed.")
		return
	}

	var clientURL string
	if !checkMainForSync {
		clientURL = fmt.Sprintf("http://%s:8545", nodeIP.String())
	} else {
		clientURL = mainURL
	}

	//connect (over rpc) to the peer using the mobile interface
	ethClient, err := geth.NewEthereumClient(clientURL)
	if err != nil {
		summaryResult.Pass = false
		clientResults.AddResult(clientID, false, "Could not connect to client.")
		return
	}

	//loop until done or timeout
	for timeout := time.After(syncTimeout); ; {
		select {

		case <-timeout:
			summaryResult.Pass = false
			clientResults.AddResult(clientID, false, "Sync timeout.")
			return

		default:

			ctx := geth.NewContext().WithTimeout(int64(1 * time.Second))

			block, err := ethClient.GetBlockByNumber(ctx, -1)
			if err != nil {
				t.Errorf("Error getting block from %s ", clientURL)
			}
			if block != nil {
				blockNumber := block.GetNumber()
				t.Logf("Block number: %d", block.GetNumber())
				if blockNumber == int64(chainLength) {
					//Success
					clientResults.AddResult(clientID, true, "Sync succeeded.")
					return
				}
			}
			//check in a little while....
			time.Sleep(1000 * time.Millisecond)
		}
	}
}
