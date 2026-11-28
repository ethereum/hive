// reorg-retention is a Hive simulator for EIP-8252, the "Execution-Layer Reorg
// State Retention Window". It checks that an execution client can reconstruct
// the post-state of any canonical block within REORG_RETENTION_WINDOW blocks of
// the head and validate a reorg forking inside that window, WITHOUT re-syncing,
// and that it degrades gracefully for reorgs forking older than the window.
//
// The property only binds under stalled finality: with finality advancing every
// client prunes to the finalized block and the window never engages. The test
// therefore keeps finality stalled (finalized pinned low, never advancing toward
// head) and drives only the head forward, which is EIP-8252's inactivity-leak
// scenario.
//
// Post-merge payload construction and all Engine-API calls reuse the engine
// simulator's packages (client, clmock, helper, globals, config, types); this
// file only adds the reorg-retention test logic on top.
package main

import (
	"context"
	"fmt"
	"math/big"
	"math/rand"
	"os"
	"strings"
	"time"

	api "github.com/ethereum/go-ethereum/beacon/engine"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/types"

	"github.com/ethereum/hive/hivesim"
	"github.com/ethereum/hive/simulators/ethereum/engine/client"
	"github.com/ethereum/hive/simulators/ethereum/engine/client/hive_rpc"
	"github.com/ethereum/hive/simulators/ethereum/engine/clmock"
	"github.com/ethereum/hive/simulators/ethereum/engine/config"
	"github.com/ethereum/hive/simulators/ethereum/engine/globals"
	"github.com/ethereum/hive/simulators/ethereum/engine/helper"
	typ "github.com/ethereum/hive/simulators/ethereum/engine/types"
)

// reconstructTimeout bounds how long a within-window reorg may take to become
// VALID. SYNCING/ACCEPTED are legal "not yet" engine responses, and clients that
// materialize their reorg-retention tier asynchronously (e.g. Nethermind's
// persisted-snapshot tier) need a moment to catch up after the chain is built.
// With no peers attached, reaching VALID within this window can only come from
// retained local state, which is the property under test.
const reconstructTimeout = 90 * time.Second

// marginTop is the number of canonical blocks kept above the beyond-window fork
// point, so that fork point sits well inside the chain (newer than the pinned
// genesis finalized block) rather than at the very bottom.
const marginTop = uint64(64)

// Fee recipients used to make the sidechain blocks diverge from the canonical
// chain built at the same heights (prevRandao differs too; this just makes the
// divergence explicit and the blocks distinguishable in logs).
var (
	feeRecipientBeyond    = common.HexToAddress("0x000000000000000000000000000000000beeeeee")
	feeRecipientWithin    = common.HexToAddress("0x00000000000000000000000000000000001a1a1a")
	feeRecipientWithinExt = common.HexToAddress("0x00000000000000000000000000000000002b2b2b")
)

// clientConfig is the per-client reorg-retention parameterisation. The window is
// the value passed as HIVE_REORG_RETENTION_WINDOW; it must be at or above the
// client's enforced floor. withinDepth and beyondDepth are reorg depths measured
// back from the final head H: withinDepth is validated as the positive MUST
// (reconstructable), beyondDepth must be out of the window (graceful degradation
// allowed). H is beyondDepth + marginTop, so the chain is (window + margin) deep.
type clientConfig struct {
	window      uint64
	withinDepth uint64
	beyondDepth uint64
	extraParams hivesim.Params
	// long marks a client whose smallest usable window is large (reth, erigon),
	// making the chain to build long; such clients run only when
	// HIVE_REORG_RETENTION_LONG=1.
	long bool
	// pace delays each block during the build. Clients that materialize their
	// reorg-retention tier asynchronously per block (e.g. Nethermind's persisted
	// snapshots) need the tier to keep up with head so an in-window fork point is
	// reconstructable; without pacing the compressed build outruns persistence and
	// the retained state, though present, is not yet usable at reorg time. Clients
	// that write retention synchronously at block commit (e.g. geth's reverse
	// diffs) need none. Zero means no delay.
	pace time.Duration
	// note records the per-client caveat surfaced in the logs.
	note string
}

// configFor maps a Hive client type name to its reorg-retention configuration.
// Floors and flags are verified against each client's EIP-8252 source; see
// README.md for the file:line citations.
func configFor(name string) (clientConfig, bool) {
	switch {
	case strings.Contains(name, "go-ethereum") || strings.Contains(name, "geth"):
		// --history.state floor: none (default = params.ReorgRetentionWindow).
		// Reorgs within the 128 in-memory diff layers are served from memory, so
		// withinDepth > 128 is used to exercise the on-disk reverse-diff path.
		return clientConfig{
			window:      192,
			withinDepth: 160,
			beyondDepth: 320,
			// Raise geth's Engine-API reorg-depth limit (default 32) above our
			// reorg depths.
			extraParams: hivesim.Params{"HIVE_EXPECT_DEEP_REORGS": "1"},
		}, true
	case strings.Contains(name, "nethermind"):
		// --FlatDb.LongFinalityMaxReorgDepth; effective backstop is
		// max(N, MinReorgDepth+CompactSize) = max(N, 160), so the window is 160.
		return clientConfig{
			window:      160,
			withinDepth: 144,
			beyondDepth: 256,
			pace:        50 * time.Millisecond,
		}, true
	case strings.Contains(name, "besu"):
		// --bonsai-historical-block-limit (= Bonsai maxLayersToLoad, the historical
		// layers a reorg/lookup may roll through). Floor and default are both
		// MINIMUM_TRIE_LOG_RETENTION_LIMIT = 512, so a window of 512 would pass even
		// with the flag unmapped; use 1024 with withinDepth 896 > 512 so the flag is
		// actually exercised (a 896-deep reorg needs the limit raised above 512).
		return clientConfig{
			window:      1024,
			withinDepth: 896,
			beyondDepth: 1152,
		}, true
	case strings.Contains(name, "reth"):
		// --prune.{account,storage}-history.distance. The per-segment serde floor
		// (MINIMUM_UNWIND_SAFE_DISTANCE = 10064) is bypassed by the CLI, but the
		// pruner enforces the same value as a runtime minimum unless
		// --prune.minimum-distance is also lowered (the entrypoint does so). Using
		// the documented-flag floor here keeps the mapping conservative; it makes a
		// ~10k-block chain, hence long/gated.
		return clientConfig{
			window:      10064,
			withinDepth: 9000,
			beyondDepth: 10192,
			long:        true,
			note:        "reth window pinned at MINIMUM_UNWIND_SAFE_DISTANCE (10064); a smaller window works via --prune.minimum-distance but is unverified here",
		}, true
	case strings.Contains(name, "erigon"):
		// --prune.distance takes a raw block count with no CLI floor (default full
		// mode = DefaultPruneDistance = 262144). A small window is accepted but
		// erigon's deep-reorg-from-head behaviour under stalled finality is not
		// verified here (see erigontech/erigon#17070), so it is gated.
		return clientConfig{
			window:      1024,
			withinDepth: 896,
			beyondDepth: 1152,
			extraParams: hivesim.Params{"HIVE_ENGINE_MAX_REORG_DEPTH": "2000"},
			long:        true,
			note:        "erigon deep-reorg-from-head under stalled finality is unverified here (erigontech/erigon#17070)",
		}, true
	}
	return clientConfig{}, false
}

var suite = hivesim.Suite{
	Name: "reorg-retention",
	Description: `EIP-8252 Execution-Layer Reorg State Retention Window: a client must be
able to reconstruct the post-state of any canonical block within
REORG_RETENTION_WINDOW (262144) blocks of the head and validate a reorg forking
inside that window without re-syncing, under stalled finality. Reorgs forking
older than the window may degrade gracefully (resync).`[1:],
}

func main() {
	suite.Add(hivesim.TestSpec{
		Name:        "reorg retention test loader",
		Description: "Runs the EIP-8252 reorg-retention test against each supported execution client.",
		Run:         runLoader,
		AlwaysRun:   true,
	})
	sim := hivesim.New()
	hivesim.MustRunSuite(sim, suite)
}

func runLoader(t *hivesim.T) {
	clientTypes, err := t.Sim.ClientTypes()
	if err != nil {
		t.Fatalf("could not get client types: %v", err)
	}
	longEnabled := os.Getenv("HIVE_REORG_RETENTION_LONG") == "1"

	for _, ct := range clientTypes {
		ct := ct
		cfg, ok := configFor(ct.Name)
		if !ok {
			t.Logf("skipping client %q: no reorg-retention configuration", ct.Name)
			continue
		}
		name := ct.Name
		// Gate long/gated clients here, by not registering their subtests at all.
		// hivesim has no "skipped" status, so a subtest that merely returns is
		// recorded as a pass — a false green. Enable them with
		// `--sim.buildarg reorg_retention_long=1` (hive does not forward
		// HIVE_REORG_RETENTION_LONG to the simulator container, so it is baked in
		// at build time; see the Dockerfile).
		if cfg.long && !longEnabled {
			t.Logf("skipping %q: window %d needs a long chain; build with --sim.buildarg reorg_retention_long=1 to run it", name, cfg.window)
			continue
		}
		// Each case builds its own stalled chain and client, so they are fully
		// independent: the reconstruct case moves head to the sidechain, which must
		// not leak into the engine-reorg case.
		run := func(body func(*reorgTest, stalledChain)) func(*hivesim.T) {
			return func(t *hivesim.T) {
				r, sc, cancel := setupStalledChain(t, name, cfg)
				defer cancel()
				defer r.ec.Close()
				body(r, sc)
			}
		}
		t.Run(hivesim.TestSpec{
			Name:        fmt.Sprintf("reorg-retention/reconstruct-in-window-state (%s)", name),
			Description: "Builds a (window+margin)-deep chain under stalled finality, then checks that the post-state of a block forking inside the window is reconstructable from retained history without a network sync (rewind head to the in-window ancestor, which forces reconstruction), while a block forking older than the window degrades gracefully. This is EIP-8252's literal MUST.",
			Run: run(func(r *reorgTest, sc stalledChain) {
				r.assertBeyondWindow(sc.sideBeyond)
				r.assertReconstructWithinWindow(sc.sideWithin, sc.genesisHash)
				r.t.Logf("%s: in-window state reconstruction verified (window=%d)", name, cfg.window)
			}),
		})
		t.Run(hivesim.TestSpec{
			Name:        fmt.Sprintf("reorg-retention/engine-reorg-to-sibling (%s)", name),
			Description: "Drives the reorg the way a consensus client actually does: newPayload for a block forking inside the window, then forkchoiceUpdated to it, with no prior rewind to the fork point. EIP-8252's motivation is that such a branch, handed in via the engine API, is validated from retained state without a network sync. A client whose engine path routes deep reorgs to network sync instead of reconstructing locally fails this (head never reaches the sidechain with no peers attached), which is the finding, not a test defect.",
			Run: run(func(r *reorgTest, sc stalledChain) {
				r.assertEngineReorgToSibling(sc.sideWithin, sc.genesisHash)
				r.t.Logf("%s: engine-API reorg to in-window sibling verified (window=%d)", name, cfg.window)
			}),
		})
	}
}

// reorgTest bundles the per-run state threaded through the helpers.
type reorgTest struct {
	t  *hivesim.T
	ec client.EngineClient
	cl *clmock.CLMocker
}

// stalledChain is the fixture both cases build on: a (window+margin)-deep
// canonical chain under stalled finality, plus the two un-adopted sidechain
// blocks forking inside and outside the window.
type stalledChain struct {
	sideBeyond  typ.ExecutableData
	sideWithin  typ.ExecutableData
	genesisHash common.Hash
	H           uint64
}

// setupStalledChain starts a client and builds the fixture. The returned
// CancelFunc owns the run context and must be deferred by the caller (together
// with ec.Close) so the context and client outlive the assertions, which run
// after this returns.
func setupStalledChain(t *hivesim.T, clientName string, cfg clientConfig) (*reorgTest, stalledChain, context.CancelFunc) {
	if cfg.note != "" {
		t.Logf("note (%s): %s", clientName, cfg.note)
	}

	// Shanghai-at-genesis fork configuration: the reorg-retention property is a
	// state-storage behaviour independent of the active EL fork, so a Shanghai
	// chain exercises it while keeping the Engine API at V2 (no blobs / beacon
	// root plumbing).
	forkConfig := &config.ForkConfig{ShanghaiTimestamp: big.NewInt(0)}

	genesis := helper.LoadGenesis("./init/genesis.json")
	for _, acc := range globals.TestAccounts {
		bal, _ := new(big.Int).SetString("123450000000000000000", 16)
		genesis.Alloc[acc.GetAddress()] = core.GenesisAccount{Balance: bal}
	}
	if err := forkConfig.ConfigGenesis(&genesis); err != nil {
		t.Fatalf("could not configure genesis: %v", err)
	}

	params := globals.DefaultClientEnv.
		Set("HIVE_TERMINAL_TOTAL_DIFFICULTY_PASSED", "1").
		Set("HIVE_TERMINAL_TOTAL_DIFFICULTY", fmt.Sprintf("%d", genesis.Difficulty)).
		Set("HIVE_MERGE_BLOCK_ID", "0").
		Set("HIVE_SHANGHAI_TIMESTAMP", fmt.Sprintf("%d", forkConfig.ShanghaiTimestamp)).
		Set("HIVE_REORG_RETENTION_WINDOW", fmt.Sprintf("%d", cfg.window))
	for k, v := range cfg.extraParams {
		params = params.Set(k, v)
	}

	// Generous overall timeout: the chain is window+margin blocks deep and each
	// block costs a few Engine-API round trips.
	timeout := 30 * time.Minute
	if cfg.long {
		timeout = 120 * time.Minute
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)

	ec, err := hive_rpc.HiveRPCEngineStarter{ClientType: clientName}.StartClient(
		t, ctx, &genesis, params, hivesim.Params{},
	)
	if err != nil {
		cancel()
		t.Fatalf("could not start client %q: %v", clientName, err)
	}

	cl := clmock.NewCLMocker(t, &genesis, forkConfig, rand.New(rand.NewSource(0x8252)))
	cl.TestContext = ctx
	// Keep finality stalled: push safe/finalized far enough out that the mocker
	// never advances them, so finalized stays the zero hash (no finalized block)
	// throughout the build. The reorg phase then pins finalized at genesis. In
	// neither case does finalized advance toward head, so the per-client
	// retention window is the only bound on reconstructable state.
	cl.SlotsToSafe = new(big.Int).Lsh(big.NewInt(1), 40)
	cl.SlotsToFinalized = new(big.Int).Lsh(big.NewInt(1), 40)
	// Build empty-ish payloads quickly; a small delay still lets clients assemble
	// the payload before GetPayload.
	cl.PayloadProductionClientDelay = 20 * time.Millisecond
	cl.AddEngineClient(ec)
	cl.InitChain(ec)

	r := &reorgTest{t: t, ec: ec, cl: cl}

	H := cfg.beyondDepth + marginTop
	forkOutHeight := H - cfg.beyondDepth // == marginTop
	forkInHeight := H - cfg.withinDepth

	t.Logf("%s: window=%d withinDepth=%d beyondDepth=%d head=%d forkIn@%d forkOut@%d",
		clientName, cfg.window, cfg.withinDepth, cfg.beyondDepth, H, forkInHeight, forkOutHeight)

	// Pace block production so clients that materialize their retention tier
	// asynchronously per block keep it near head (see clientConfig.pace).
	var pacing clmock.BlockProcessCallbacks
	if cfg.pace > 0 {
		pacing.OnForkchoiceBroadcast = func() { time.Sleep(cfg.pace) }
	}

	// Phase 1: build canonical up to the beyond-window fork point, then build (but
	// do not adopt) the beyond-window sidechain block while that parent's state is
	// still available. It can only be built now; at reorg time its parent state is
	// pruned.
	cl.ProduceBlocks(int(forkOutHeight), pacing)
	sideBeyond := r.buildForkBlock(cl.LatestHeader.Hash(), cl.LatestHeader.Time, common.Hash{}, feeRecipientBeyond)
	t.Logf("built beyond-window sidechain block #%d %x (fork at height %d)", sideBeyond.Number, sideBeyond.BlockHash, forkOutHeight)

	// Phase 2: continue canonical to the within-window fork point, stash that
	// sidechain block too.
	cl.ProduceBlocks(int(forkInHeight-forkOutHeight), pacing)
	sideWithin := r.buildForkBlock(cl.LatestHeader.Hash(), cl.LatestHeader.Time, common.Hash{}, feeRecipientWithin)
	t.Logf("built within-window sidechain block #%d %x (fork at height %d)", sideWithin.Number, sideWithin.BlockHash, forkInHeight)

	// Phase 3: extend canonical to the final head H.
	cl.ProduceBlocks(int(H-forkInHeight), pacing)

	head := cl.LatestHeader
	if head.Number.Uint64() != H {
		t.Fatalf("unexpected canonical head height %d, want %d", head.Number.Uint64(), H)
	}

	// Pin finality at genesis and set head to the canonical tip. Finalized moves
	// from "none" to genesis; it never advances toward head.
	genesisHash := cl.GenesisBlock().Hash()
	r.pinForkchoice(head.Hash(), genesisHash)
	t.Logf("canonical head %d %x, finalized pinned at genesis %x", H, head.Hash(), genesisHash)

	return r, stalledChain{
		sideBeyond:  sideBeyond,
		sideWithin:  sideWithin,
		genesisHash: genesisHash,
		H:           H,
	}, cancel
}

// buildForkBlock builds a single payload on top of the given parent using a
// distinct fee recipient and a random prevRandao, so it diverges from the
// canonical block at the same height. It returns the sealed payload without
// adopting it as head. The parent's post-state must be available when this is
// called.
func (r *reorgTest) buildForkBlock(parentHash common.Hash, parentTime uint64, finalized common.Hash, feeRecipient common.Address) typ.ExecutableData {
	ts := parentTime + r.cl.GetTimestampIncrement()
	if r.cl.IsCancun(ts) {
		// This simulator runs a Shanghai-only chain; Cancun would need blob and
		// beacon-root plumbing that is deliberately omitted.
		r.t.Fatalf("buildForkBlock: Cancun payloads are not supported by this simulator")
	}
	var rnd common.Hash
	r.cl.Rand.Read(rnd[:])
	attrs := &typ.PayloadAttributes{
		Timestamp:             ts,
		Random:                rnd,
		SuggestedFeeRecipient: feeRecipient,
	}
	if r.cl.IsShanghai(ts) {
		attrs.Withdrawals = make(types.Withdrawals, 0)
	}
	fcState := &api.ForkchoiceStateV1{
		HeadBlockHash:      parentHash,
		SafeBlockHash:      finalized,
		FinalizedBlockHash: finalized,
	}

	fcuVersion := r.cl.ForkConfig.ForkchoiceUpdatedVersion(parentTime, &ts)
	ctx, cancel := context.WithTimeout(r.cl.TestContext, globals.RPCTimeout)
	defer cancel()
	resp, err := r.ec.ForkchoiceUpdated(ctx, fcuVersion, fcState, attrs)
	if err != nil {
		r.t.Fatalf("buildForkBlock: forkchoiceUpdated on parent %x failed: %v", parentHash, err)
	}
	if resp.PayloadStatus.Status != api.VALID || resp.PayloadID == nil {
		r.t.Fatalf("buildForkBlock: forkchoiceUpdated on parent %x returned %q / payloadID=%v (want VALID + id)", parentHash, resp.PayloadStatus.Status, resp.PayloadID)
	}

	// Give the client a moment to assemble the payload.
	time.Sleep(r.cl.PayloadProductionClientDelay)

	getVersion := r.cl.ForkConfig.GetPayloadVersion(ts)
	ctx2, cancel2 := context.WithTimeout(r.cl.TestContext, globals.RPCTimeout)
	defer cancel2()
	ed, _, _, _, err := r.ec.GetPayload(ctx2, getVersion, resp.PayloadID)
	if err != nil {
		r.t.Fatalf("buildForkBlock: getPayload on parent %x failed: %v", parentHash, err)
	}
	if ed.ParentHash != parentHash {
		r.t.Fatalf("buildForkBlock: built payload has wrong parent %x, want %x", ed.ParentHash, parentHash)
	}
	return ed
}

// pinForkchoice sets head to the given block and pins safe+finalized at the
// given (low) hash. It must be VALID.
func (r *reorgTest) pinForkchoice(head, finalized common.Hash) {
	fcState := &api.ForkchoiceStateV1{
		HeadBlockHash:      head,
		SafeBlockHash:      finalized,
		FinalizedBlockHash: finalized,
	}
	version := r.cl.ForkConfig.ForkchoiceUpdatedVersion(r.cl.LatestHeader.Time, nil)
	ctx, cancel := context.WithTimeout(r.cl.TestContext, globals.RPCTimeout)
	defer cancel()
	resp, err := r.ec.ForkchoiceUpdated(ctx, version, fcState, nil)
	if err != nil {
		r.t.Fatalf("pinForkchoice: forkchoiceUpdated failed: %v", err)
	}
	if resp.PayloadStatus.Status != api.VALID {
		r.t.Fatalf("pinForkchoice: forkchoiceUpdated returned %q, want VALID", resp.PayloadStatus.Status)
	}
}

// assertBeyondWindow submits a sidechain block whose fork point is older than the
// retention window. EIP-8252 allows graceful degradation here: SYNCING/ACCEPTED
// (the client may need to resync) or VALID (the client retained more than the
// required window) are all acceptable. A hard error/crash, or INVALID (declaring
// a structurally valid block invalid without the state to prove it), is a failure.
func (r *reorgTest) assertBeyondWindow(fork typ.ExecutableData) {
	version := r.cl.ForkConfig.NewPayloadVersion(fork.Timestamp)
	ctx, cancel := context.WithTimeout(r.cl.TestContext, globals.RPCTimeout)
	defer cancel()
	status, err := r.ec.NewPayload(ctx, version, &fork)
	if err != nil {
		r.t.Fatalf("beyond-window: newPayload returned a hard error; EIP-8252 permits graceful degradation (SYNCING), not a crash/error: %v", err)
	}
	switch status.Status {
	case api.VALID:
		r.t.Logf("beyond-window: VALID (client retained more than the required window; acceptable)")
	case api.SYNCING, api.ACCEPTED:
		r.t.Logf("beyond-window: %s (graceful degradation, resync may be required; acceptable per EIP-8252)", status.Status)
	case api.INVALID:
		ve := ""
		if status.ValidationError != nil {
			ve = *status.ValidationError
		}
		r.t.Fatalf("beyond-window: INVALID for a structurally valid out-of-window payload; the client must not assert invalidity it cannot determine without the pruned state (validationError=%q)", ve)
	default:
		r.t.Fatalf("beyond-window: unexpected newPayload status %q", status.Status)
	}
}

// fcuRetryUntilValid drives forkchoiceUpdated to headHash, retrying while the
// client answers SYNCING or ACCEPTED, until it returns VALID/INVALID or the
// deadline passes. No peers are attached, so a client that reaches VALID did so
// purely from retained local state. The caller asserts on the returned status.
func (r *reorgTest) fcuRetryUntilValid(headHash, finalized common.Hash, ts uint64, deadline time.Duration) api.PayloadStatusV1 {
	fcState := &api.ForkchoiceStateV1{
		HeadBlockHash:      headHash,
		SafeBlockHash:      finalized,
		FinalizedBlockHash: finalized,
	}
	version := r.cl.ForkConfig.ForkchoiceUpdatedVersion(ts, nil)
	end := time.Now().Add(deadline)
	var last api.PayloadStatusV1
	for {
		ctx, cancel := context.WithTimeout(r.cl.TestContext, globals.RPCTimeout)
		resp, err := r.ec.ForkchoiceUpdated(ctx, version, fcState, nil)
		cancel()
		if err != nil {
			r.t.Fatalf("forkchoiceUpdated to %x error: %v", headHash, err)
		}
		last = resp.PayloadStatus
		if last.Status == api.VALID || last.Status == api.INVALID {
			return last
		}
		if time.Now().After(end) {
			return last
		}
		time.Sleep(500 * time.Millisecond)
	}
}

// assertReconstructWithinWindow is EIP-8252's literal MUST: the post-state of a
// block forking inside the retention window must be reconstructable from retained
// history without a network sync. It drives the reconstruction explicitly by
// rewinding head to the in-window canonical ancestor (which forces the client to
// materialize that state), retrying on SYNCING/ACCEPTED until VALID or a timeout.
// With no peers attached, reaching VALID can only come from retained local state.
// It then completes the reorg on the reconstructed state (newPayload + fork-choice
// to the sidechain head, both VALID; head switches; an extension is VALID). This
// is deliberately NOT how a consensus client sequences a reorg — no CL rewinds to
// an ancestor first; see assertEngineReorgToSibling for that path.
func (r *reorgTest) assertReconstructWithinWindow(fork typ.ExecutableData, finalized common.Hash) {
	npVersion := r.cl.ForkConfig.NewPayloadVersion(fork.Timestamp)
	ctx, cancel := context.WithTimeout(r.cl.TestContext, globals.RPCTimeout)
	defer cancel()
	status, err := r.ec.NewPayload(ctx, npVersion, &fork)
	if err != nil {
		r.t.Fatalf("within-window: newPayload error: %v", err)
	}
	switch status.Status {
	case api.VALID, api.ACCEPTED, api.SYNCING:
		// VALID, ACCEPTED and SYNCING are all legal newPayload responses here: the
		// fork-point state is not the live head state, so a client may validate
		// eagerly (VALID) or defer (ACCEPTED/SYNCING). Reconstruction is asserted
		// via the fork-choice rewind below, not here.
		r.t.Logf("within-window: newPayload %s (full validation deferred to the fork-choice reorg)", status.Status)
	case api.INVALID:
		ve := ""
		if status.ValidationError != nil {
			ve = *status.ValidationError
		}
		r.t.Fatalf("within-window: newPayload INVALID for a structurally valid in-window payload (validationError=%q)", ve)
	default:
		r.t.Fatalf("within-window: unexpected newPayload status %q", status.Status)
	}

	// EIP-8252's requirement is that the fork-point post-state is reconstructable
	// from retained history without a network sync. Rewind the head to the
	// in-window canonical ancestor: this forces the client to reconstruct that
	// state. With no peers configured, reaching VALID here can only come from the
	// local retained history, which is exactly the property under test.
	rewindStatus := r.fcuRetryUntilValid(fork.ParentHash, finalized, fork.Timestamp, reconstructTimeout)
	if rewindStatus.Status != api.VALID {
		r.t.Fatalf("within-window: forkchoiceUpdated rewinding to the in-window ancestor must reconstruct its state without sync and be VALID within %s, got %q (the retention window data is present but the client did not reconstruct from it)", reconstructTimeout, rewindStatus.Status)
	}
	r.t.Logf("within-window: reconstructed in-window ancestor state at %x without sync (VALID)", fork.ParentHash)

	// With the fork-point state now live, the sidechain block executes against it;
	// adopt it as head to complete the reorg.
	fcState := &api.ForkchoiceStateV1{
		HeadBlockHash:      fork.BlockHash,
		SafeBlockHash:      finalized,
		FinalizedBlockHash: finalized,
	}
	ctxNP, cancelNP := context.WithTimeout(r.cl.TestContext, globals.RPCTimeout)
	defer cancelNP()
	readopt, err := r.ec.NewPayload(ctxNP, npVersion, &fork)
	if err != nil {
		r.t.Fatalf("within-window: newPayload re-executing the sidechain block on the reconstructed state error: %v", err)
	}
	if readopt.Status != api.VALID {
		r.t.Fatalf("within-window: newPayload on the reconstructed fork-point state must be VALID, got %q", readopt.Status)
	}
	fcuVersion := r.cl.ForkConfig.ForkchoiceUpdatedVersion(fork.Timestamp, nil)
	ctx2, cancel2 := context.WithTimeout(r.cl.TestContext, globals.RPCTimeout)
	defer cancel2()
	resp, err := r.ec.ForkchoiceUpdated(ctx2, fcuVersion, fcState, nil)
	if err != nil {
		r.t.Fatalf("within-window: forkchoiceUpdated error: %v", err)
	}
	if resp.PayloadStatus.Status != api.VALID {
		r.t.Fatalf("within-window: forkchoiceUpdated to the in-window sidechain head must be VALID (not %q)", resp.PayloadStatus.Status)
	}
	if resp.PayloadStatus.LatestValidHash == nil || *resp.PayloadStatus.LatestValidHash != fork.BlockHash {
		r.t.Fatalf("within-window: forkchoiceUpdated returned wrong latestValidHash %v, want %x", resp.PayloadStatus.LatestValidHash, fork.BlockHash)
	}

	// Head must actually have switched to the sidechain block.
	ctx3, cancel3 := context.WithTimeout(r.cl.TestContext, globals.RPCTimeout)
	defer cancel3()
	headBlk, err := r.ec.BlockByNumber(ctx3, new(big.Int).SetUint64(fork.Number))
	if err != nil {
		r.t.Fatalf("within-window: could not read reorged head block at %d: %v", fork.Number, err)
	}
	if headBlk.Hash() != fork.BlockHash {
		r.t.Fatalf("within-window: head did not switch to the sidechain block at height %d: got %x, want %x", fork.Number, headBlk.Hash(), fork.BlockHash)
	}

	// A subsequent newPayload extending the now-canonical sidechain must be VALID.
	ext := r.buildForkBlock(fork.BlockHash, fork.Timestamp, finalized, feeRecipientWithinExt)
	ctx4, cancel4 := context.WithTimeout(r.cl.TestContext, globals.RPCTimeout)
	defer cancel4()
	extStatus, err := r.ec.NewPayload(ctx4, r.cl.ForkConfig.NewPayloadVersion(ext.Timestamp), &ext)
	if err != nil {
		r.t.Fatalf("within-window: newPayload on sidechain extension error: %v", err)
	}
	if extStatus.Status != api.VALID {
		r.t.Fatalf("within-window: newPayload extending the adopted sidechain must be VALID, got %q", extStatus.Status)
	}
	r.t.Logf("within-window: reconstructed fork-point state, switched head to %x, extension VALID", fork.BlockHash)
}

// assertEngineReorgToSibling drives the reorg the way a consensus client actually
// does: newPayload for the sidechain block forking inside the window, then
// forkchoiceUpdated to it, with no prior rewind to the fork point. EIP-8252's
// motivation is that such a branch, handed in over the engine API, is validated
// from retained state without a network sync. newPayload answering SYNCING or
// ACCEPTED is legal (the fork-point state is not live), so the assertion is on the
// outcome: head must reach the sidechain block within the timeout, with no peers
// attached, which is only possible from retained local state. A client whose
// engine path routes a deep reorg to network sync instead of reconstructing
// locally never gets there and fails here — that is the finding this case exists
// to surface, not a test defect.
func (r *reorgTest) assertEngineReorgToSibling(fork typ.ExecutableData, finalized common.Hash) {
	npVersion := r.cl.ForkConfig.NewPayloadVersion(fork.Timestamp)
	fcuVersion := r.cl.ForkConfig.ForkchoiceUpdatedVersion(fork.Timestamp, nil)
	fcState := &api.ForkchoiceStateV1{
		HeadBlockHash:      fork.BlockHash,
		SafeBlockHash:      finalized,
		FinalizedBlockHash: finalized,
	}
	end := time.Now().Add(reconstructTimeout)
	var lastNP, lastFCU string
	for {
		// Re-send the payload each round: a client that dropped it after a SYNCING
		// answer needs it again once its retained state becomes usable.
		ctxNP, cancelNP := context.WithTimeout(r.cl.TestContext, globals.RPCTimeout)
		npStatus, err := r.ec.NewPayload(ctxNP, npVersion, &fork)
		cancelNP()
		if err != nil {
			r.t.Fatalf("engine-reorg: newPayload error: %v", err)
		}
		lastNP = string(npStatus.Status)
		if npStatus.Status == api.INVALID {
			ve := ""
			if npStatus.ValidationError != nil {
				ve = *npStatus.ValidationError
			}
			r.t.Fatalf("engine-reorg: newPayload INVALID for a structurally valid in-window payload (validationError=%q)", ve)
		}

		ctxFC, cancelFC := context.WithTimeout(r.cl.TestContext, globals.RPCTimeout)
		fcuResp, err := r.ec.ForkchoiceUpdated(ctxFC, fcuVersion, fcState, nil)
		cancelFC()
		if err != nil {
			r.t.Fatalf("engine-reorg: forkchoiceUpdated error: %v", err)
		}
		lastFCU = string(fcuResp.PayloadStatus.Status)
		if fcuResp.PayloadStatus.Status == api.INVALID {
			r.t.Fatalf("engine-reorg: forkchoiceUpdated INVALID for the in-window sidechain head")
		}
		if fcuResp.PayloadStatus.Status == api.VALID {
			ctxH, cancelH := context.WithTimeout(r.cl.TestContext, globals.RPCTimeout)
			headBlk, err := r.ec.BlockByNumber(ctxH, new(big.Int).SetUint64(fork.Number))
			cancelH()
			if err == nil && headBlk.Hash() == fork.BlockHash {
				r.t.Logf("engine-reorg: head reached the in-window sidechain block via the engine API (no rewind, no peers)")
				return
			}
		}
		if time.Now().After(end) {
			r.t.Fatalf("engine-reorg: head did not reach the in-window sidechain block within %s (last newPayload=%s, forkchoiceUpdated=%s). EIP-8252 requires validating a branch forking inside the window from retained state without sync; the client deferred the reorg to network sync instead. See the client's reorg-retention follow-up issue.", reconstructTimeout, lastNP, lastFCU)
		}
		time.Sleep(500 * time.Millisecond)
	}
}
