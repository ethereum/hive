package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/hive/hivesim"
)

var (
	// the number of seconds before a sync is considered stalled or failed
	syncTimeout = 45 * time.Second
	params      = hivesim.Params{
		"HIVE_NETWORK_ID":     "19763",
		"HIVE_CHAIN_ID":       "19763",
		"HIVE_FORK_HOMESTEAD": "0",
		"HIVE_FORK_TANGERINE": "0",
		"HIVE_FORK_SPURIOUS":  "0",
		"HIVE_FORK_BYZANTIUM": "0",
	}
	sourceFiles = map[string]string{
		"genesis.json": "./simplechain/genesis.json",
		"chain.rlp":    "./simplechain/chain.rlp",
	}
	sinkFiles = map[string]string{
		"genesis.json": "./simplechain/genesis.json",
	}
	sourceURL string
)

var (
	testchainHeadNumber = uint64(3000)
	testchainHeadHash   = common.HexToHash("0xc95596f4707fb382554b660b4847c599eb5f8fdcf99be2c5654aaadd4ec97840")
)

func main() {
	var suite = hivesim.Suite{
		Name: "sync",
		Description: `This suite of tests verifies that clients can sync from each other in different modes.
For each client, we test if it can serve as a sync source for all other clients (including itself).`,
	}

	// SYNC by ETH protocol
	ethParams := params.Set("HIVE_NODETYPE", "full")
	suite.Add(hivesim.ClientTestSpec{
		Name:        "CLIENT as sync source",
		Description: "This loads the test chain into the client and verifies whether it was imported correctly.",
		Parameters:  ethParams,
		Files:       sourceFiles,
		Run:         runETHSourceTest,
	})

	// SYNC by LES protocol
	serverParams := params.Set("HIVE_NODETYPE", "full")
	serverParams = serverParams.Set("HIVE_LIGHTSERVE", "100")
	suite.Add(hivesim.ClientTestSpec{
		Name:        "CLIENT as sync source",
		Description: "This loads the test chain into the les server and verifies whether it was imported correctly.",
		Parameters:  serverParams,
		Files:       sourceFiles,
		Run:         runLESSourceTest,
	})
	hivesim.MustRunSuite(hivesim.New(), suite)
}

func runETHSourceTest(t *hivesim.T, c *hivesim.Client) {
	// Check whether the source has imported its chain.rlp correctly.
	source := &node{c}
	if err := source.checkHead(testchainHeadNumber, testchainHeadHash); err != nil {
		t.Fatal(err)
	}

	// Configure sink to connect to the source node.
	enode, err := source.EnodeURL()
	if err != nil {
		t.Fatal("can't get node peer-to-peer endpoint:", enode)
	}
	sinkParams := params.Set("HIVE_BOOTNODE", enode)

	// Sync all sink nodes against the source.
	t.RunAllClients(hivesim.ClientTestSpec{
		Name:        fmt.Sprintf("sync %s -> CLIENT", source.Type),
		Description: fmt.Sprintf("This test attempts to sync the chain from a %s node.", source.Type),
		Parameters:  sinkParams,
		Files:       sinkFiles,
		Run:         runETHSyncTest,
	})
}

func runETHSyncTest(t *hivesim.T, c *hivesim.Client) {
	node := &node{c}
	err := node.checkSync(t, testchainHeadNumber, testchainHeadHash)
	if err != nil {
		t.Fatal("sync failed:", err)
	}
}

func runLESSourceTest(t *hivesim.T, c *hivesim.Client) {
	// Short circuit if the source is not Geth client.
	if !isGeth(c.Type) {
		return
	}

	// Check whether the source has imported its chain.rlp correctly.
	source := &node{c}
	if err := source.checkHead(testchainHeadNumber, testchainHeadHash); err != nil {
		t.Fatal(err)
	}

	// Configure sink to connect to the source node.
	clientParams := params.Set("HIVE_NODETYPE", "light")
	enode, err := source.EnodeURL()
	if err != nil {
		t.Fatal("can't get node peer-to-peer endpoint:", enode)
	}
	sourceURL = enode

	// Sync all sink nodes against the source.
	t.RunAllClients(hivesim.ClientTestSpec{
		Name:        fmt.Sprintf("sync %s -> CLIENT", source.Type),
		Description: fmt.Sprintf("This test attempts to sync the chain from a %s node.", source.Type),
		Parameters:  clientParams,
		Files:       sinkFiles,
		Run:         runLESSyncTest,
	})
}

func runLESSyncTest(t *hivesim.T, c *hivesim.Client) {
	// Short circuit if the destination is not Geth client.
	if !isGeth(c.Type) {
		return
	}
	// todo(rjl493456442) Wait the initialization of light client
	time.Sleep(time.Second * 10)

	err := c.RPC().Call(nil, "admin_addPeer", sourceURL)
	if err != nil {
		t.Fatalf("connection failed:", err)
	}

	node := &node{c}
	err = node.checkSync(t, testchainHeadNumber, testchainHeadHash)
	if err != nil {
		t.Fatal("sync failed:", err)
	}
}

type node struct {
	*hivesim.Client
}

// checkSync waits for the node to reach the head of the chain.
func (n *node) checkSync(t *hivesim.T, wantNumber uint64, wantHash common.Hash) error {
	var (
		timeout = time.After(syncTimeout)
		current = uint64(0)
	)
	for {
		select {
		case <-timeout:
			return fmt.Errorf("timeout (%v elapsed, current head is %d)", syncTimeout, current)
		default:
			block, err := n.head()
			if err != nil {
				t.Logf("error getting block from %s (%s): %v", n.Type, n.Container, err)
				return err
			}
			blockNumber := block.Number.Uint64()
			if blockNumber != current {
				t.Logf("%s has new head %d", n.Type, blockNumber)
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
	return ethclient.NewClient(n.RPC()).HeaderByNumber(ctx, nil)
}

// checkHead checks whether the remote chain head matches the given values.
func (n *node) checkHead(num uint64, hash common.Hash) error {
	head, err := n.head()
	if err != nil {
		return fmt.Errorf("can't query chain head: %v", err)
	}
	if head.Hash() != hash {
		return fmt.Errorf("wrong chain head %d (%s), want %d (%s)", head.Number, head.Hash().TerminalString(), num, hash.TerminalString())
	}
	return nil
}

// isGeth reports the client is Geth-based client.
func isGeth(client string) bool {
	return strings.HasPrefix(client, "go-ethereum")
}