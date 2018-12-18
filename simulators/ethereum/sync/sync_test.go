package main

import (
	"flag"
	"fmt"
	"net"
	"sync"
	"time"

	"os"
	"strings"
	"testing"

	geth "github.com/ethereum/go-ethereum/mobile"
	"github.com/ethereum/go-ethereum/p2p/enode"
	rpc "github.com/ethereum/go-ethereum/rpc"
	"github.com/ethereum/hive/simulators/common"
	docker "github.com/fsouza/go-dockerclient"
)

var (
	dockerHost  *string //docker host api endpoint
	hostURI     *string //simulator host endpoint
	host        common.SimulatorAPI
	daemon      *docker.Client //docker daemon proxy
	err         error
	syncTimeout = 60 * time.Second //the number of seconds before a sync is considered stalled or failed
)

type testCase struct {
	Client string
}

func TestMain(m *testing.M) {

	//Max Concurrency is specified in the parallel flag, which is supplied to the simulator container

	hostURI = flag.String("simulatorHost", "", "url of simulator host api")
	dockerHost = flag.String("dockerHost", "", "docker host api endpoint")

	flag.Parse()

	//Try to connect to the simulator host and get the client list
	host = &common.SimulatorHost{
		HostURI: hostURI,
	}

	os.Exit(m.Run())
}

func TestSyncsWithGeth(t *testing.T) {

	t.Run("fastmodes", func(t *testing.T) {

		startTime := time.Now()

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
			t.Fatal("Cannot execute test. No geth client available.")
		}

		//and load the genesis without a chain
		mainParms := map[string]string{
			"CLIENT":                  availableClients[firstGeth],
			"HIVE_INIT_GENESIS":       "/simplechain/genesis.json",
			"HIVE_USE_GENESIS_CONFIG": "1",
			"HIVE_NODETYPE":           "full", //fast sync
		}

		t.Log("Starting main node (geth)")
		_, mainNodeIP, err := host.StartNewNode(mainParms)
		if err != nil {
			t.Fatalf("Unable to start node: %v", err)
		}
		mainNodeURL := fmt.Sprintf("http://%s:8545", mainNodeIP.String())

		//the main client will be asked to sync with the other client.
		for i, clientType := range availableClients {
			if i != firstGeth {

				t.Logf("Starting peer node (%s)", clientType)
				// create the other node
				parms := map[string]string{
					"CLIENT":                   clientType,
					"HIVE_INIT_GENESIS":        "/simplechain/genesis.json",
					"HIVE_INIT_CHAIN":          "/simplechain/chain.rlp",
					"HIVE_NETWORK_ID":          "1",
					"HIVE_CHAIN_ID":            "1",
					"HIVE_FORK_HOMESTEAD":      "0",
					"HIVE_FORK_DAO_BLOCK":      "0",
					"HIVE_FORK_DAO_VOTE":       "0",
					"HIVE_FORK_TANGERINE":      "0",
					"HIVE_FORK_TANGERINE_HASH": "0x0000000000000000000000000000000000000000000000000000000000000000",
					"HIVE_FORK_SPURIOUS":       "0",
					"HIVE_FORK_BYZANTIUM":      "0",
					"HIVE_FORK_CONSTANTINOPLE": "0",
				}
				clientID, nodeIP, err := host.StartNewNode(parms)
				if err != nil {
					t.Fatalf("Unable to start node: %v", err)
				}

				syncClient(nil, mainNodeURL, clientID, nodeIP, t, 10, startTime, true)

			}
		}

		t.Log("Terminating.")

	})

	t.Run("compatibility", func(t *testing.T) {

		startTime := time.Now()

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

		//and load the genesis + chain
		mainParms := map[string]string{
			"CLIENT":                  availableClients[firstGeth],
			"HIVE_INIT_GENESIS":       "/simplechain/genesis.json",
			"HIVE_INIT_CHAIN":         "/simplechain/chain.rlp",
			"HIVE_USE_GENESIS_CONFIG": "1",
		}

		t.Log("Starting main node (geth)")
		_, mainNodeIP, err := host.StartNewNode(mainParms)
		if err != nil {
			t.Fatalf("Unable to start node: %v", err)
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
					"CLIENT":                   clientType,
					"HIVE_INIT_GENESIS":        "/simplechain/genesis.json",
					"HIVE_NETWORK_ID":          "1",
					"HIVE_CHAIN_ID":            "1",
					"HIVE_FORK_HOMESTEAD":      "0",
					"HIVE_FORK_DAO_BLOCK":      "0",
					"HIVE_FORK_DAO_VOTE":       "0",
					"HIVE_FORK_TANGERINE":      "0",
					"HIVE_FORK_TANGERINE_HASH": "0x0000000000000000000000000000000000000000000000000000000000000000",
					"HIVE_FORK_SPURIOUS":       "0",
					"HIVE_FORK_BYZANTIUM":      "0",
					"HIVE_FORK_CONSTANTINOPLE": "0",
				}
				clientID, nodeIP, err := host.StartNewNode(parms)
				if err != nil {
					t.Fatalf("Unable to start node: %v", err)
				}

				wg.Add(1)
				go syncClient(&wg, mainNodeURL, clientID, nodeIP, t, 10, startTime, false)

			}
		}
		t.Log("Waiting for client sync.")
		wg.Wait()
		t.Log("Terminating.")

	})

}

func syncClient(wg *sync.WaitGroup, mainURL, clientID string, nodeIP net.IP, t *testing.T, chainLength int, startTime time.Time, checkMainForSync bool) {
	if wg != nil {
		defer wg.Done()
	}

	peerEnodeID, err := host.GetClientEnode(clientID)
	if err != nil || peerEnodeID == nil || *peerEnodeID == "" {
		t.Fatalf("Unable to get enode: %v", err)
	}

	peerNode, err := enode.ParseV4(*peerEnodeID)
	if err != nil {
		t.Fatalf("Unable to parse enode: %v", err)
	}
	//replace the ip with what docker says it is
	peerNode = enode.NewV4(peerNode.Pubkey(), nodeIP, peerNode.TCP(), 30303)
	if peerNode == nil {
		t.Fatalf("Unable to make enode: %v", err)
	}

	*peerEnodeID = peerNode.String()

	t.Logf("Attempting to add peer %s ", *peerEnodeID)

	//connect over rpc to the main node to add the peer (the mobile client does not have the admin function)
	rpcClient, err := rpc.Dial(mainURL)
	if err != nil {

		t.Fatalf("Unable to connect to node via rpc: %v", err)
	}
	var res = false
	if err := rpcClient.Call(&res, "admin_addPeer", *peerEnodeID); err != nil {
		t.Fatalf("Unable to add peer via rpc: %v", err)
	} else {
		t.Logf("Add peer result %v", res)
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
		t.Fatalf("Unable to connect to node: %v", err)
	}

	//loop until done or timeout
	for timeout := time.After(syncTimeout); ; {
		select {

		case <-timeout:
			host.AddResults(false, clientID, t.Name(), "Sync timed out.", time.Since(startTime))
			t.Errorf("Client sync timed out %s", clientURL)
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
					host.AddResults(true, clientID, t.Name(), "", time.Since(startTime))
					return
				}
			}

			//check in a little while....
			time.Sleep(1000 * time.Millisecond)

		}

	}

}
