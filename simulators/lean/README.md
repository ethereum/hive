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

Select the Lean devnet profile through the standard Hive client-file config.
Reusable devnet profiles live in `simulators/lean/clients/`:

`./hive --sim lean --client-file simulators/lean/clients/devnet3.yaml --client ream`

or:

`./hive --sim lean --client-file simulators/lean/clients/devnet4.yaml --client ream`

The lean simulator resolves the active devnet from the selected client name
(`ream_devnet4`, `ethlambda_devnet3`, `gean_devnet3`, and so on) and validates
it against the central support matrix in
`simulators/lean/config/lean-devnets.txt`.

This gives us a simple onboarding path for new lean clients while leaving a clear place to grow:

- p2p behavior,
- syncing,
- req/resp exchanges,
- RPC compatibility, and
- other protocol-focused scenarios.
