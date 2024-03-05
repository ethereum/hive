package suite_sync

import (
	"context"

	"github.com/ethereum/hive/hivesim"
	tn "github.com/ethereum/hive/simulators/eth2/common/testnet"
)

// Re-org testnet.
func (ts SyncTestSpec) Verify(
	t *hivesim.T,
	ctx context.Context,
	testnet *tn.Testnet,
	env *tn.Environment,
	config *tn.Config,
) {
	t.Logf("INFO: Starting secondary clients")
	// Start the other clients
	for _, n := range testnet.Nodes {
		if !n.IsRunning() {
			if err := n.Start(); err != nil {
				t.Fatalf("FAIL: error starting node %s: %v", n.ClientNames(), err)
			}
		}
	}

	// Wait for all other clients to sync with a timeout of 1 epoch
	syncCtx, cancel := testnet.Spec().EpochTimeoutContext(ctx, 1)
	defer cancel()
	if h, err := testnet.WaitForSync(syncCtx); err != nil {
		t.Fatalf("FAIL: error waiting for sync: %v", err)
	} else {
		t.Logf("INFO: all clients synced at head %s", h)
	}

	// Run the base test spec verifications
	ts.BaseTestSpec.Verify(t, ctx, testnet, env, config)
}
