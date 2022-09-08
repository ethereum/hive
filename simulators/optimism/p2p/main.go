package main

import (
	"context"
	"math/big"
	"time"

	"github.com/ethereum-optimism/optimism/op-node/eth"
	"github.com/ethereum-optimism/optimism/op-node/rollup/driver"
	"github.com/ethereum-optimism/optimism/op-proposer/rollupclient"
	"github.com/stretchr/testify/require"

	"github.com/ethereum/hive/hivesim"
	"github.com/ethereum/hive/optimism"
)

func main() {
	suite := hivesim.Suite{
		Name:        "optimism p2p",
		Description: "This suite runs the P2P tests",
	}

	// Add tests for full nodes.
	suite.Add(&hivesim.TestSpec{
		Name:        "simple p2p testnet",
		Description: `This suite runs the a testnet with P2P set up`,
		Run:         func(t *hivesim.T) { runP2PTests(t) },
	})

	sim := hivesim.New()
	hivesim.MustRunSuite(sim, suite)
}

// runP2PTests runs the P2P tests between the sequencer and verifier.
func runP2PTests(t *hivesim.T) {
	d := optimism.NewDevnet(t)

	d.InitChain(10, 4, 30, nil)
	d.AddEth1() // l1 eth1 node is required for l2 config init
	d.WaitUpEth1(0, time.Second*10)

	d.AddOpL2() // l2 engine is required for rollup config init
	d.WaitUpOpL2Engine(0, time.Second*10)

	// sequencer stack, on top of first eth1 node
	d.AddOpNode(0, 0, true)
	d.AddOpBatcher(0, 0, 0)
	d.AddOpProposer(0, 0, 0)

	// TODO: pass optimism.HiveUnpackParams{flag env vars here}.Params()
	//  hivesim start option to the op nodes to configure p2p networking

	// verifier A
	d.AddOpL2()
	d.AddOpNode(0, 1, false) // we attach to the same L1 node, so we don't need to configure L1 networking.

	// verifier B
	d.AddOpL2()
	d.AddOpNode(0, 2, false)

	t.Log("waiting for nodes to get onto the network")

	seq := d.GetOpNode(0)
	verifA := d.GetOpNode(1)
	verifB := d.GetOpNode(2)

	seqEng := d.GetOpL2Engine(0)
	seqEthCl := seqEng.EthClient()

	seqCl := seq.RollupClient()
	verifACl := verifA.RollupClient()
	verifBCl := verifB.RollupClient()

	ctx, cancel := context.WithCancel(context.Background())

	checkCanon := func(name string, id eth.BlockID) {
		bl, err := seqEthCl.BlockByNumber(ctx, big.NewInt(int64(id.Number)))
		if err != nil {
			t.Fatalf("%s: sequencer does not have block at height %d", name, id.Number)
		}
		if h := bl.Hash(); h != id.Hash {
			t.Fatalf("%s: sequencer diverged, height %d does not match: sequencer: %s <> verifier: %s", name, h, id.Hash)
		}
	}

	syncStat := func(name string, cl *rollupclient.RollupClient) *driver.SyncStatus {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*4)
		seqStat, err := cl.SyncStatus(ctx)
		cancel()
		if err != nil {
			t.Errorf("failed to get sync status from %s op-node: %v", name, err)
		}
		t.Log(name,
			"currentL1", seqStat.CurrentL1.TerminalString(),
			"headL1", seqStat.HeadL1.TerminalString(),
			"finalizedL2", seqStat.FinalizedL2.TerminalString(),
			"safeL2", seqStat.SafeL2.TerminalString(),
			"unsafeL2", seqStat.UnsafeL2.TerminalString())
		return seqStat // may be nil
	}

	go func() {
		ticker := time.NewTicker(time.Second * 4)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				// Check that all clients are synced
				seqStat := syncStat("sequencer ", seqCl)
				verAStat := syncStat("verifier-A", verifACl)
				verBStat := syncStat("verifier-B", verifBCl)
				checkCanon("verifier A", verAStat.UnsafeL2.ID())
				checkCanon("verifier B", verBStat.UnsafeL2.ID())
				require.LessOrEqual(t, seqStat.CurrentL1.Number, verAStat.CurrentL1.Number+d.RollupCfg.SeqWindowSize, "verifier A is not behind sequencer by more than the sequence window")
				require.LessOrEqual(t, seqStat.CurrentL1.Number, verBStat.CurrentL1.Number+d.RollupCfg.SeqWindowSize, "verifier B is not behind sequencer by more than the sequence window")
			case <-ctx.Done():
				t.Log("exiting sync checking loop")
				return
			}
		}
	}()

	// Run testnet for duration of 3 sequence windows
	time.Sleep(time.Second * time.Duration(d.L1Cfg.Config.Clique.Period*d.RollupCfg.SeqWindowSize*3))
	cancel()

	// TODO: Add P2P tests
}
