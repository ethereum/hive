package main

import (
	"context"
	"fmt"
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
	serverFiles = map[string]string{
		"genesis.json": "./simplechain/genesis.json",
		"chain.rlp":    "./simplechain/chain.rlp",
	}
	clientFiles = map[string]string{
		"genesis.json": "./simplechain/genesis.json",
	}
)

var (
	testchainHeadNumber = uint64(3000)
	testchainHeadHash   = common.HexToHash("0xc95596f4707fb382554b660b4847c599eb5f8fdcf99be2c5654aaadd4ec97840")
)

func main() {
	var suite = hivesim.Suite{
		Name:        "sync",
		Description: `This test suite verifies that the light clients can sync from the les servers`,
	}
	serverParams := params.Copy()
	serverParams.Set("HIVE_NODETYPE", "full")
	serverParams.Set("HIVE_LIGHTSERVE", "100")

	suite.Add(hivesim.ClientTestSpec{
		Name:        "CLIENT as sync source",
		Description: "This loads the test chain into the les server and verifies whether it was imported correctly.",
		Parameters:  serverParams,
		Files:       serverFiles,
		Run:         runServerTest,
	})
	hivesim.MustRunSuite(hivesim.New(), suite)
}

func runServerTest(t *hivesim.T, c *hivesim.Client) {
	// Check whether the source has imported its chain.rlp correctly.
	source := &node{c}
	if err := source.checkHead(testchainHeadNumber, testchainHeadHash); err != nil {
		t.Fatal(err)
	}

	// Configure sink to connect to the source node.
	clientParams := params.Copy()
	clientParams.Set("HIVE_NODETYPE", "light")
	enode, err := source.EnodeURL()
	if err != nil {
		t.Fatal("can't get node peer-to-peer endpoint:", enode)
	}
	clientParams.Set("HIVE_BOOTNODE", enode)

	// Sync all sink nodes against the source.
	t.RunAllClients(hivesim.ClientTestSpec{
		Name:        fmt.Sprintf("sync %s -> CLIENT", source.Type),
		Description: fmt.Sprintf("This test attempts to sync the chain from a %s node.", source.Type),
		Parameters:  clientParams,
		Files:       clientFiles,
		Run:         runSyncTest,
	})
}

func runSyncTest(t *hivesim.T, c *hivesim.Client) {
	node := &node{c}
	err := node.checkSync(t, testchainHeadNumber, testchainHeadHash)
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
			header, err := n.head()
			if err != nil {
				t.Logf("error getting header from %s (%s): %v", n.Type, n.Container, err)
				return err
			}
			headerNumber := header.Number.Uint64()
			if headerNumber != current {
				t.Logf("%s has new head %d", n.Type, headerNumber)
			}
			if current == wantNumber {
				if header.Hash() != wantHash {
					return fmt.Errorf("wrong head hash %x, want %x", header.Hash(), wantHash)
				}
				return nil // success
			}
			// check in a little while....
			current = headerNumber
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
