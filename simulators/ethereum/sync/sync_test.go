package main

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/log"
	rpc "github.com/ethereum/go-ethereum/rpc"
	"github.com/ethereum/hive/simulators/common"
	"github.com/ethereum/hive/simulators/common/providers/hive"
)

var (
	host        common.TestSuiteHost
	syncTimeout = 30 * time.Second //the number of seconds before a sync is considered stalled or failed
	testSuite   common.TestSuiteID
	params      = map[string]string{
		"HIVE_NETWORK_ID":     "19763",
		"HIVE_CHAIN_ID":       "19763",
		"HIVE_FORK_HOMESTEAD": "0",
		"HIVE_FORK_TANGERINE": "0",
		"HIVE_FORK_SPURIOUS":  "0",
		"HIVE_FORK_BYZANTIUM": "0",
		"HIVE_NODETYPE":       "full",
	}
	sourceFiles = map[string]string{
		"genesis.json": "/simplechain/genesis.json",
		"chain.rlp":    "/simplechain/chain.rlp",
	}
	sinkFiles = map[string]string{
		"genesis.json": "/simplechain/genesis.json",
	}
)

var (
	testchainHeadNumber = uint64(3000)
	testchainHeadHash   = ethcommon.HexToHash("0xc95596f4707fb382554b660b4847c599eb5f8fdcf99be2c5654aaadd4ec97840")
)

func TestMain(m *testing.M) {
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

	// start the test suite
	testSuite, err := host.StartTestSuite("Sync test suite",
		`This suite of tests verifies that clients can sync from each other in different modes.
For each client, we test if it can serve as a sync source for all other clients (including itself)
`, logFile)
	if err != nil {
		t.Fatalf("Simulator error. Failed to start test suite. %v ", err)
	}
	defer host.EndTestSuite(testSuite)

	availableClients, err := host.GetClientTypes()
	if err != nil {
		t.Fatalf("Simulator error. Cannot get client types. %v", err)
	}

	for _, sourceType := range availableClients {
		sourceNode, endSourceTest := startSourceNode(t, testSuite, sourceType)
		if sourceNode == nil {
			continue
		}

		for _, sinkType := range availableClients {
			testName := fmt.Sprintf("sync %v -> %v", sourceType, sinkType)
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
				var bootnode = sourceNode.enode
				sinkNode, err := getClient(sinkType, testSuite, testID, params, sinkFiles, bootnode)
				if err != nil {
					msg := fmt.Sprintf("Unable to instantiate sink-node %v: %s", sinkType, err.Error())
					log.Error(msg)
					summaryResult.AddDetail(msg)
					// If the sink doesn't even start, that doesn't affect the test for the source-client
					clientResults.AddResult(sinkNode.id, false, msg)
					return
				}

				if err := syncNodes(sourceNode, sinkNode, testchainHeadNumber, testchainHeadHash); err != nil {
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

func startSourceNode(t *testing.T, testSuite common.TestSuiteID, sourceType string) (*node, func(error)) {
	desc := fmt.Sprintf("Tests that `%v` can do a block import and serve as sync source", sourceType)
	sourceTestID, _ := host.StartTest(testSuite, fmt.Sprintf("`%v` as sync-source", sourceType), desc)
	sourceNode, err := getClient(sourceType, testSuite, sourceTestID, params, sourceFiles, "")
	endSourceTest := func(err error) {
		if err == nil {
			host.EndTest(testSuite, sourceTestID, &common.TestResult{Pass: true}, nil)
		} else {
			t.Error("Test failed:", err)
			host.EndTest(testSuite, sourceTestID,
				&common.TestResult{Pass: false},
				common.TestClientResults{
					sourceType: {Pass: false, Details: err.Error()},
				},
			)
		}
	}
	if err != nil {
		endSourceTest(fmt.Errorf("unable to instantiate source-node `%v`: %s", sourceType, err.Error()))
		return nil, nil
	}
	if err := sourceNode.checkHead(testchainHeadNumber, testchainHeadHash); err != nil {
		endSourceTest(err)
		return nil, nil
	}
	return sourceNode, endSourceTest
}

// node represents an instantiated client, which has successfully spun up RPC,
// and delivered enode enodeId
type node struct {
	name      string
	enode     string
	rpcClient *rpc.Client
	id        string
}

// getClient instantiates a client, and fetches ip, enode etc.
// Any error during this process is returned.
func getClient(clientType string, testSuite common.TestSuiteID, testID common.TestID, params, files map[string]string, bootnode string) (*node, error) {
	n := &node{name: clientType}
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
	enodePtr, err := host.GetClientEnode(testSuite, testID, id)
	if err != nil {
		return n, fmt.Errorf("Client (`%v`) enode could not be obtained: %v", clientType, err)
	}
	if enodePtr == nil {
		return n, fmt.Errorf("Client (`%v`) enode could not be obtained: (nil)", clientType)
	}
	n.enode = *enodePtr
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
func syncNodes(source, sink *node, wantNumber uint64, wantHash ethcommon.Hash) error {
	var (
		timeout = time.After(syncTimeout)
		current = uint64(0)
	)
	for {
		log.Debug("Checking sync progress", "current", current)
		select {
		case <-timeout:
			return fmt.Errorf("Sync timeout (%v elapsed)", syncTimeout)
		default:
			block, err := sink.head()
			if err != nil {
				log.Error("Error getting block", "sink", sink.name, "error", err)
				return err
			}
			blockNumber := block.Number.Uint64()
			if blockNumber != current {
				log.Info("Block progressed", "sink", sink.name, "at", blockNumber)
			}
			if current == wantNumber {
				if block.Hash() != wantHash {
					return fmt.Errorf("wrong head hash %x, want %x", block.Hash(), wantHash)
				}
				return nil // success
			}
			// check in a little while....
			current = blockNumber
			time.Sleep(1000 * time.Millisecond)
		}
	}
}

// head returns the node's chain head.
func (n *node) head() (*types.Header, error) {
	ctx, _ := context.WithTimeout(context.Background(), 5*time.Second)
	return ethclient.NewClient(n.rpcClient).HeaderByNumber(ctx, nil)
}

// checkHead checks whether the remote chain head matches the given values.
func (n *node) checkHead(num uint64, hash ethcommon.Hash) error {
	head, err := n.head()
	if err != nil {
		return fmt.Errorf("can't query chain head: %v", err)
	}
	if head.Hash() != hash {
		return fmt.Errorf("wrong chain head %d (%x), want %d (%x)", head.Number, head.Hash(), num, hash)
	}
	return nil
}
