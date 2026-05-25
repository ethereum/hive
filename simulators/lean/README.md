# Lean Hive Simulator

This simulator contains the initial Rust-based Hive coverage for lean clients.

Today it runs RPC compatibility, sync, and client interop suites. The RPC
compatibility coverage:

- launches each selected `lean` client,
- checks the shared Lean HTTP endpoints on fresh nodes, and
- uses the `lean-spec-client` helper to drive post-genesis
  justification/finalization cases for the shared RPC endpoints.

The `client-interop` suite runs every selected client against itself once and
against every other selected client in both three-node 2:1 topologies, using one
shared genesis per run and asserting that all three nodes finalize past genesis
at the same slot.

The helper-backed post-genesis cases are part of the default suite. The
LeanSpec helper image generates and caches a small fixed set of validator keys
at image-build time so the tests do not need to regenerate them on every
container start.

## Prerequisites

The lean simulator is part of the Rust workspace, so its source is compiled
inside a Docker image when you invoke `./hive --sim lean …`. Before running
any simulation, build the workspace locally from the repo root:

```
make build
```

This wraps `cargo build --workspace --all-targets` and surfaces Rust
compilation errors in seconds, instead of waiting for them to fail inside a
slow Docker build. It is the same `build` target CI exercises through
`make lint` (see the root `Makefile`).

## Selecting a devnet profile

Select the Lean devnet profile through the standard Hive client-file config.
Reusable devnet profiles live in `simulators/lean/clients/`:

`./hive --sim lean --client-file simulators/lean/clients/devnet3.yaml --client ream`

or:

`./hive --sim lean --client-file simulators/lean/clients/devnet4.yaml --client ream`

The lean simulator resolves the active devnet from the selected client name
(`ream_devnet4`, `ethlambda_devnet3`, `gean_devnet3`, and so on) and validates
it against the central support matrix in
`simulators/lean/config/lean-devnets.txt`.

## Examples

### Sync example

Run the default lean suites (RPC + sync) against a single `ream_devnet4`
node, forcing fresh client and simulator images, and write results under
`./workspace/logs`:

```
./hive --sim lean \
    --client-file simulators/lean/clients/devnet4.yaml \
    --client ream_devnet4 \
    --docker.output \
    --docker.nocache \
    --results-root ./workspace/logs
```

### Client-interop example

Run only the `client-interop` suite between `ream_devnet4` and
`lantern_devnet4`, rebuilding only the `ream_devnet4` image so iterating on
one client does not invalidate the others. Results go to a separate
`logs-tracer` directory, and stdout/stderr are tee'd to `/tmp/hive-tracer.log`
for later inspection:

```
./hive --sim lean \
    --client-file simulators/lean/clients/devnet4.yaml \
    --client ream_devnet4,lantern_devnet4 \
    --sim.limit "client-interop" \
    --docker.output \
    --docker.nocache="hive/clients/ream_devnet4" \
    --results-root ./workspace/logs-tracer 2>&1 | tee /tmp/hive-tracer.log
```

### Flag reference

| Flag | Purpose |
| --- | --- |
| `--sim` | Selects the simulator under `simulators/<name>/` (here, `lean`). |
| `--client-file` | YAML listing `client` + `nametag` pairs to launch (e.g. `simulators/lean/clients/devnet4.yaml`). |
| `--client` | Comma-separated subset of clients from `--client-file` to actually run. |
| `--sim.limit` | Regex filter; only test names matching are run (sets `HIVE_TEST_PATTERN`). |
| `--docker.output` | Streams container stdout/stderr to the host — required to see Rust panics or lean client logs while iterating. |
| `--docker.nocache` | Regex of Docker image names to rebuild from scratch (e.g. `hive/clients/ream_devnet4`). Bare `--docker.nocache` rebuilds everything. |
| `--results-root` | Directory where Hive writes per-run logs and the `hive.json` viewable with `hiveview`. |

The full flag list lives in [`docs/commandline.md`](../../docs/commandline.md).

This gives us a simple onboarding path for new lean clients while leaving a clear place to grow:

- p2p behavior,
- syncing,
- req/resp exchanges,
- RPC compatibility, and
- other protocol-focused scenarios.
