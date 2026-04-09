# Lean Hive Simulator

This simulator contains the initial Rust-based Hive coverage for lean clients.

Today it runs an RPC compatibility suite that:

- launches each selected `lean` client,
- checks the shared Lean HTTP endpoints on fresh nodes, and
- uses the `lean-spec-client` helper to drive post-genesis
  justification/finalization cases for the shared RPC endpoints.

The simulator currently labels all Lean runs as `devnet3` and passes that
profile metadata through the client environment:

- `HIVE_LEAN_DEVNET_LABEL=devnet3`
- `HIVE_LEAN_SPEC_CUTOFF_HASH=be853180d21aa36d6401b8c1541aa6fcaad5008d`
- `HIVE_LEAN_REAM_IMAGE=ghcr.io/reamlabs/ream:latest-devnet3`

That profile selection is only a placeholder for now. The client launch scripts
do not switch behavior on it yet, but the label is in place so we can wire real
devnet-specific flags later without reshaping the simulator.

The helper-backed post-genesis cases are part of the default suite. The
LeanSpec helper image generates and caches a small fixed set of validator keys
at image-build time so the tests do not need to regenerate them on every
container start.

This gives us a simple onboarding path for new lean clients while leaving a clear place to grow:

- p2p behavior,
- syncing,
- req/resp exchanges,
- RPC compatibility, and
- other protocol-focused scenarios.
