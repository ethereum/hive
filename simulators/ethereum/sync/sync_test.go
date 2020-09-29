package main

import (
	"context"
	"fmt"
	"github.com/ethereum/hive/simulators/common/providers/hive"
	"os"
	//"sync"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/p2p/enode"
	rpc "github.com/ethereum/go-ethereum/rpc"
	"github.com/ethereum/hive/simulators/common"
)

var (
	host        common.TestSuiteHost
	syncTimeout = 30 * time.Second //the number of seconds before a sync is considered stalled or failed
	testSuite   common.TestSuiteID
	params      = map[string]string{
		"HIVE_NETWORK_ID":     "1",
		"HIVE_CHAIN_ID":       "1",
		"HIVE_FORK_HOMESTEAD": "0",
		"HIVE_FORK_TANGERINE": "0",
		"HIVE_FORK_SPURIOUS":  "0",
		"HIVE_FORK_BYZANTIUM": "0",
		"HIVE_NODETYPE":       "full", //fast sync
	}
	sourceFiles = map[string]string{
		"genesis.json": "/simplechain/genesis.json",
		"chain.rlp":    "/simplechain/chain.rlp",
	}
	sinkFiles = map[string]string{
		"genesis.json": "/simplechain/genesis.json",
	}
)

type testCase struct {
	Client string
}

func TestMain(m *testing.M) {
	//Max Concurrency is specified in the parallel flag, which is supplied to the simulator container
	host = hive.New()
	os.Exit(RunTestSuite(m))
}

func RunTestSuite(m *testing.M) int {
	return m.Run()
}

func copyMap(a map[string]string) map[string]string {
	var b = make(map[string]string)
	for k, v := range a {
		b[k] = v
	}
	return b
}

func TestSync(t *testing.T) {
	log.Root().SetHandler(log.StdoutHandler)
	logFile, _ := os.LookupEnv("HIVE_SIMLOG")
	//start the test suite
	testSuite, err := host.StartTestSuite("Sync test suite",
		`This suite of tests verifies that clients can sync from each other in different modes.
For each client, we test if it can serve as a sync source for all other clients (including itself)
`, logFile)
	if err != nil {
		t.Fatalf("Simulator error. Failed to start test suite. %v ", err)
	}
	defer host.EndTestSuite(testSuite)
	//get all client types required to test
	availableClients, err := host.GetClientTypes()
	if err != nil {
		t.Fatalf("Simulator error. Cannot get client types. %v", err)
	}

	for _, sourceType := range availableClients {
		// Load source with chain
		sourceTestID, _ := host.StartTest(testSuite, fmt.Sprintf("`%v` as sync-source", sourceType),
			fmt.Sprintf("Tests that `%v` can do a block import and serve as sync source", sourceType))
		sourceNode, err := getClient(sourceType, testSuite, sourceTestID, params, sourceFiles, "")
		endSourceTest := func(err error) {
			if err != nil {
				t.Errorf("Test failed %s", err.Error())
				host.EndTest(testSuite, sourceTestID, &common.TestResult{Pass: false},
					common.TestClientResults{
						sourceType: {Pass: false, Details: err.Error()},
					})
				return
			}
			host.EndTest(testSuite, sourceTestID, &common.TestResult{Pass: true}, nil)
		}
		if err != nil {
			endSourceTest(fmt.Errorf("Unable to instantiate source-node `%v`: %s", sourceType, err.Error()))
			continue
		}
		// TODO: Verify that the source-node is indeed at the expected block num.

		for _, sinkType := range availableClients {
			testName := fmt.Sprintf("fast-sync %v -> %v", sourceType, sinkType)
			desc := fmt.Sprintf("This test initialises the client-under-test (`%v`) "+
				"with a predefined chain, and attempts to sync up another "+
				"node (`%v`) against the it.", sourceType, sinkType)
			sinkType := sinkType
			t.Run(testName, func(t *testing.T) {
				var (
					summaryResult = &common.TestResult{Pass: true}
					clientResults = make(common.TestClientResults)
				)
				testID, err := host.StartTest(testSuite, testName, desc)
				if err != nil {
					t.Errorf("Unable to start test: %s", err.Error())
					return
				}
				defer func() {
					host.EndTest(testSuite, testID, summaryResult, clientResults)
					//send test failures to standard outputs too
					if !summaryResult.Pass {
						t.Errorf("Test failed %s", summaryResult.Details)
					}
				}()
				var bootnode = sourceNode.enodeId.String()
				sinkNode, err := getClient(sinkType, testSuite, testID, params, sinkFiles, bootnode)
				if err != nil {
					msg := fmt.Sprintf("Unable to instantiate sink-node %v: %s", sinkType, err.Error())
					log.Error(msg)
					summaryResult.AddDetail(msg)
					// If the sink doesn't even start, that doesn't affect the test for the source-client
					clientResults.AddResult(sinkNode.id, false, msg)
					return
				}

				if err := syncNodes(sourceNode, sinkNode, 10); err != nil {
					summaryResult.Pass = false
					clientResults.AddResult(sourceNode.id, false, err.Error())
					clientResults.AddResult(sinkNode.id, false, err.Error())
					t.Error(err.Error())
				} else {
					clientResults.AddResult(sourceNode.id, true, "synk-sink ok")
					clientResults.AddResult(sinkNode.id, true, "synk-source ok")
				}
				// This should not be needed:
				//host.KillNode(testSuite, testID, sinkNodeId)
			})
		}
		// This will signal that the source node is done for deletion
		endSourceTest(nil)
	}
}

// node represents an instantiated client, which has successfully spun up RPC,
// and delivered enode enodeId
type node struct {
	name      string
	enodeId   enode.Node
	rpcClient *rpc.Client
	id        string
}

// getClient instantiates a client, and fetches ip, enode enodeId etc.
// Any error during this process is returned
func getClient(clientType string, testSuite common.TestSuiteID, testID common.TestID, params, files map[string]string, bootnode string) (*node, error) {
	n := &node{
		name: clientType,
		id:   "",
	}
	params = copyMap(params)
	params["CLIENT"] = clientType
	if bootnode != "" {
		params["HIVE_BOOTNODE"] = bootnode
	}
	id, ip, _, err := host.GetNode(testSuite, testID, params, files)
	if err != nil {
		return n, fmt.Errorf("Unable to create node: %v", err)
	}
	n.id = id
	enodeIDPtr, err := host.GetClientEnode(testSuite, testID, id)
	if err != nil {
		return n, fmt.Errorf("Client (`%v`) enode ID could not be obtained: %v", clientType, err)
	}
	if enodeIDPtr == nil {
		return n, fmt.Errorf("Client (`%v`) enode ID could not be obtained: (nil)", clientType)
	}
	enodeId, err := enode.ParseV4(*enodeIDPtr)
	if err != nil {
		return n, fmt.Errorf("Client (`%v`) enode ID could not be parsed. Got: `%v`, error: `%v`", clientType, *enodeIDPtr, err)
	}
	tcpPort := enodeId.TCP()
	if tcpPort == 0 {
		tcpPort = 30303
	}
	//replace the ip with what the provider says it is
	if enodeId = enode.NewV4(enodeId.Pubkey(), ip, tcpPort, 30303); enodeId != nil {
		n.enodeId = *enodeId
	}
	//connect over rpc to the main node to add the peer (the mobile client does not have the admin function)
	n.rpcClient, err = rpc.Dial(fmt.Sprintf("http://%s:8545", ip))
	if err != nil {
		return n, fmt.Errorf("Client (`%v`), rpc cli could not be created: %v", clientType, err)
	}
	// yay, all goood
	return n, nil
}

// sync peers the two nodes, and returns either when
// - an error occurs
// - the sink is synced or
// - the sync times out, or
func syncNodes(source, sink *node, headblock uint64) error {

	// The code below to add peers via RPC is not needed, since we use
	// the HIVE_BOOTNODES env var to configure the peering
	//
	// Peer the with eachother (hopefully one of them has admin_addPeer)
	//var res bool
	//if err := source.rpcClient.Call(&res, "admin_addPeer", sink.enodeId.String()); err != nil {
	//	log.Info("RPC call [1] to source.addPeer(sink) failed", "error", err)
	//	// Some nodes (parity) uses another format
	//	if err := source.rpcClient.Call(&res, "parity_addReservedPeer", sink.enodeId.String()); err != nil {
	//		log.Info("RPC call [2] to source.addReservedPeer(sink) failed", "error", err)
	//	}
	//}
	//if err := sink.rpcClient.Call(&res, "admin_addPeer", source.enodeId.String()); err != nil {
	//	log.Info("RPC call [1] to sink.addPeer(source) failed", "error", err)
	//	if err := sink.rpcClient.Call(&res, "parity_addReservedPeer", source.enodeId.String()); err != nil {
	//		log.Info("RPC call [2] to sink.addReservedPeer(source) failed", "error", err)
	//	}
	//}
	//log.Info("Peering done", "source", source.enodeId.String(), "sink", sink.enodeId.String())
	var (
		timeout = time.After(syncTimeout)
		ethCli  = ethclient.NewClient(sink.rpcClient)
		current = uint64(0)
	)
	for {
		log.Debug("Checking sync progress", "current", current)
		select {
		case <-timeout:
			return fmt.Errorf("Sync timeout (%v elapsed)", syncTimeout)
		default:
			ctx, _ := context.WithTimeout(context.Background(), time.Second)
			if block, err := ethCli.BlockByNumber(ctx, nil); err != nil {
				log.Error("Error getting block", "sink", sink.name, "error", err)
				return err
			} else {
				blockNumber := block.NumberU64()
				if current != blockNumber {
					log.Info("Block progressed", "sink", sink.name, "at", blockNumber)
				}
				current := blockNumber
				if current == headblock {
					//Success
					return nil
				}
			}
			//check in a little while....
			time.Sleep(1000 * time.Millisecond)
		}
	}
}
