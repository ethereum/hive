# Lean Hive Simulator

This simulator contains the initial Rust-based Hive coverage for lean clients.

Today it runs an RPC compatibility suite that:

- launches each selected `lean` client,
- checks the shared Lean HTTP endpoints on fresh nodes, and
- uses the `lean-spec-client` helper to drive post-genesis
  justification/finalization cases for the shared RPC endpoints.

The helper-backed post-genesis cases are part of the default suite. The
LeanSpec helper image generates and caches a small fixed set of validator keys
at image-build time so the tests do not need to regenerate them on every
container start.

Select the Lean devnet profile at runtime with:

`./hive --sim lean --sim.lean-devnet devnet3 --client ream`

or:

`./hive --sim lean --sim.lean-devnet devnet4 --client ream`

This gives us a simple onboarding path for new lean clients while leaving a clear place to grow:

- p2p behavior,
- syncing,
- req/resp exchanges,
- RPC compatibility, and
- other protocol-focused scenarios.
