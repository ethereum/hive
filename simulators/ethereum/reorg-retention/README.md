# reorg-retention — EIP-8252 Execution-Layer Reorg State Retention Window

This simulator checks that an execution-layer (EL) client honours
[EIP-8252](https://eips.ethereum.org/EIPS/eip-8252): the **Reorg State Retention
Window**.

## The property under test

`REORG_RETENTION_WINDOW = 262144` blocks (8192 epochs × 32 slots, ≈ 36.4 days at
12 s slots). EIP-8252 requires that for every canonical block `n` in
`[head − REORG_RETENTION_WINDOW, head]`, the EL can **reconstruct that block's
post-state without re-syncing**, so it can validate a reorg that forks inside the
window. The window is measured **from the head, not from the finalized block**.
Reorgs that fork *older* than the window are explicitly out of scope and may
require a fresh sync.

## Why stalled finality is mandatory

Under normal operation finality advances and every client prunes state down to
the finalized block, so the retention window never binds — pruning to finalized
is always the tighter bound. The window only engages during an **inactivity
leak**, when finality stalls and the head keeps advancing. The test reproduces
exactly that:

- `finalizedBlockHash` is kept **low and non-advancing**. During the chain build
  the CL mocker is configured so safe/finalized never move (they stay the zero
  hash — "no finalized block"); during the reorg phase finalized is pinned at the
  **genesis** block hash. In neither case does finalized advance toward the head.
- Only `headBlockHash` is advanced, via `engine_forkchoiceUpdated`.

With finalized pinned low, the per-client retention window (set below) is the
only thing bounding reconstructable state, which is what makes the test
meaningful. Without this pinning the test would prove nothing.

## Per-client window and the `HIVE_REORG_RETENTION_WINDOW` env var

Each EL entrypoint maps the `HIVE_REORG_RETENTION_WINDOW` env var (a block count)
to that client's retention flag. The simulator sets it via
`hivesim.Params` when it starts the client. **There is no single small window
that works for all clients** — each enforces a different floor, verified against
the client's EIP-8252 source:

| Client | Flag | Floor (verified) |
|---|---|---|
| go-ethereum | `--history.state=N` (path scheme) | No CLI floor. Default `params.ReorgRetentionWindow` (262144). Reorgs within the 128 in-memory diff layers are served from memory, so a reorg **deeper than 128** is needed to exercise the on-disk reverse-diff path. `cmd/utils/flags.go:298`, `triedb/pathdb/config.go:70,75`, `params/network_params.go:35`. |
| besu | `--bonsai-historical-block-limit=N` | Floor **512** (`MINIMUM_TRIE_LOG_RETENTION_LIMIT`), CLI-rejected below it. Trie-log limiting on by default; default pruning window (262144 + 30000) stays above N. `cli/options/storage/ExtraStorageOptions.java:216-221`, `ethereum/worldstate/ExtraStorageConfiguration.java:51,60`. |
| nethermind | `--FlatDb.LongFinalityMaxReorgDepth=N` (`EnableLongFinality=true` default) | Effective backstop `max(N, MinReorgDepth + CompactSize) = max(N, 128+32=160)`. `Nethermind.State.Flat/PersistenceManager.cs:55-57`, `Nethermind.Db/FlatDbConfig.cs:58,61,62,72,73`. |
| reth | `--prune.account-history.distance=N --prune.storage-history.distance=N` (+ `--prune.minimum-distance=N`, `--prune.block-interval=1`) | The per-segment serde (TOML) floor `MINIMUM_UNWIND_SAFE_DISTANCE = 10064` is **bypassed by the CLI**, but the pruner enforces the *same value* as a runtime minimum via `PruneSegment::min_blocks()` and errors below it **unless `--prune.minimum-distance` is also lowered**. `crates/node/core/src/args/pruning.rs` (`account_history_prune_mode`), `crates/prune/types/src/target.rs:11`, `crates/prune/types/src/mode.rs:48-71`, `crates/prune/types/src/segment.rs:65-72`. |
| erigon | `--prune.distance=N` | No CLI floor (`db/kv/prune/distance.go` `parseDistanceNumber`). Default (full mode) `DefaultPruneDistance = 262144`. `node/cli/flags.go:87`, `db/config3/config3.go:49`. |

The entrypoints leave the var unset-by-default, so absent it they are a no-op.

## Test geometry

For a client with window `W`, the simulator builds a canonical chain to height
`H = beyondDepth + 64` (the trailing `64` keeps the deepest fork point newer than
the pinned genesis), stashing two sidechain blocks while their fork-point state is
still available. The beyond-window block can only be *built* while its parent's
state is present, so it is built early and submitted only after the head has
advanced past the window and that state has been pruned.

Both cases below build their own fresh chain and client, so they are fully
independent.

## The two cases

### Case A — `reconstruct-in-window-state` (EIP-8252's literal MUST)

Checks that the post-state of a block forking inside the window is reconstructable
from retained history **without a network sync**. The reconstruction is driven
explicitly: `engine_forkchoiceUpdated` rewinds head to the in-window *canonical*
ancestor, which forces the client to materialize that state. With **no peers
attached**, reaching **VALID** can only come from retained local state, which is
the property under test. The case then completes the reorg on the reconstructed
state (newPayload + forkchoiceUpdated to the sidechain head, head switches, an
extension is VALID). It also checks the **beyond-window** block degrades
gracefully: SYNCING / ACCEPTED (needs resync — allowed) or VALID (over-retained —
fine) pass; **INVALID** or a crash fail (the client must not reject a structurally
valid out-of-window block it cannot disprove).

This rewind-first sequence is **not** how a consensus client drives a reorg (no CL
forkchoices to an ancestor first); it isolates the "is the state retained and
reconstructable" question. That is why it is a separate case from B.

### Case B — `engine-reorg-to-sibling` (the engine-API intent)

Drives the reorg the way a consensus client actually does: `engine_newPayload`
for the in-window sidechain block, then `engine_forkchoiceUpdated` to it, with
**no** prior rewind. EIP-8252's motivation is that such a branch, handed in over
the engine API, is validated from retained state without a network sync. Passes
if head reaches the sidechain block within the timeout, with no peers attached.

`newPayload` answering **SYNCING / ACCEPTED** is legal here (the fork-point state
is not the live head state), so the assertion is on the *outcome* (head reaches
the sibling), reached by retrying newPayload + forkchoiceUpdated until VALID or a
timeout. A client whose engine path routes a deep reorg to network sync instead
of reconstructing locally never gets there and **fails** — that failure is the
finding this case exists to surface, not a test defect.

## Retry-on-SYNCING and pacing

`SYNCING` is a legal "come back later" engine response, and clients that
materialize their retention tier **asynchronously per block** (e.g. Nethermind's
persisted-snapshot tier, built by background persistence) need it to catch up.
Two mechanisms keep the test deterministic across persistence models:

- The reorg forkchoiceUpdated **retries on SYNCING/ACCEPTED** up to a timeout; with
  no peers, convergence to VALID is proof of local reconstruction.
- `clientConfig.pace` delays each block during the build for async-persistence
  clients, so the retained tier stays near head and an in-window fork point is
  reconstructable at reorg time. Clients that write retention synchronously at
  block commit (geth's reverse diffs) need no pacing.

## Results observed against live clients

Run on this framework against the EIP-8252 branches of each client:

| Client | Case A (reconstruct) | Case B (engine reorg to sibling) |
|---|---|---|
| **go-ethereum** | ✅ | ❌ — `newPayload` drops the in-window sidechain block (`eth/catalyst/api.go`, "State not available, ignoring new payload"), and the subsequent forkchoiceUpdated routes to `BeaconSync` → `SYNCING` with no peers. geth reconstructs on an explicit rewind (Case A) but does not use the retained state to validate a reorg handed in the way a CL does. |
| **nethermind** | ✅ | timing-sensitive — its persisted-snapshot tier is materialized by *asynchronous* per-block persistence, so the cold `newPayload` finds the fork-point state only if the tier has caught up to that depth. Under hive's compressed timeline it usually has not, so this returns `SYNCING`; in production (12s slots) the tier keeps up and the reorg completes. Hive cannot deterministically assert it. |
| **besu** | ✅ | ✅ — reconstructs+adopts the in-window reorg over the engine API (window 1024, an 896-deep roll). Bonsai writes trie logs synchronously, so this is deterministic here. |
| reth, erigon | *not yet run* (gated) | *not yet run* (gated) |

Case B failing for a client is a conformance result on the hive dashboard, not a
CI gate (no CI job runs simulators against clients).

Per-client depths (window / withinDepth / beyondDepth):
go-ethereum 192 / 160 / 320, besu 1024 / 896 / 1152, nethermind 160 / 144 / 256,
reth 10064 / 9000 / 10192, erigon 1024 / 896 / 1152.

go-ethereum additionally gets `HIVE_EXPECT_DEEP_REORGS=1` (raises its Engine-API
reorg-depth limit from 32 to 512), and erigon gets `HIVE_ENGINE_MAX_REORG_DEPTH`.

## Long / gated clients

`reth` and `erigon` build a long chain and/or rely on behaviour not reproducible
offline, so they are gated. hive forwards only a fixed set of env vars to the
simulator container (not `HIVE_REORG_RETENTION_LONG`), so the gate is baked in at
build time — enable it with **`--sim.buildarg reorg_retention_long=1`** (a
Dockerfile `ARG`/`ENV`). When off, the gated clients are **not registered** (they
do not appear as false passes, which a bare early-return would be since hivesim
has no skipped status):

- **reth** — pinned at the documented-flag floor (10064), i.e. a ~10k-block chain.
  A smaller window is reachable with the `--prune.minimum-distance` override that
  the entrypoint also sets, but that path is not verified here.
- **erigon** — the flag accepts a small window, but erigon's deep-reorg-from-head
  behaviour under stalled finality is a known work-in-progress
  (erigontech/erigon#17070) and is not verified here.

## Reused code

Post-merge payload construction and all Engine-API calls reuse the **engine**
simulator's packages — `client`, `client/hive_rpc`, `clmock`, `helper`,
`globals`, `config`, `types` — via a `replace => ../engine` directive. The build
context is the parent directory (`hive_context.txt` contains `..`) so those
packages are present during the Docker build. The chain is a Shanghai-at-genesis
chain (Engine API V2): the retention property is a state-storage behaviour
independent of the active EL fork, and Shanghai keeps the payload plumbing simple.

## Verification status

Executed against the **go-ethereum**, **nethermind** and **besu** EIP-8252 branches
(see the results table above): all three pass Case A. On Case B, besu passes
(synchronous trie logs), geth fails (defers the cold reorg to sync), and
nethermind is timing-sensitive under the compressed timeline (asynchronous
persisted tier) — it reconstructs in production but hive cannot deterministically
assert it.
**reth and erigon have not yet been run** (gated, long chains); their per-client depths, engine
reorg-depth limits, and pacing needs may require tuning once exercised. The reth
and erigon cases are gated behind `HIVE_REORG_RETENTION_LONG=1`.
