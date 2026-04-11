# Lean Hive Simulator

This simulator contains the initial Rust-based Hive coverage for lean clients.

Today it runs two small suites:

`api-smoke`

- launches each selected `lean` client,
- verifies the health endpoint is reachable, and
- verifies the justified checkpoint endpoint responds with the expected shape.

`api-compat`

- verifies `/lean/v0/states/finalized` returns a non-empty SSZ payload,
- verifies `/lean/v0/fork_choice` matches the shared lean JSON shape, and
- polls `/lean/v0/fork_choice` plus `/lean/v0/checkpoints/justified` after startup to check for early endpoint stability.

This gives us a simple onboarding path for new lean clients while establishing a small shared compatibility suite. Follow-on work can grow this into:

- p2p behavior,
- syncing,
- checkpoint sync coverage,
- req/resp exchanges,
- compatibility matrix output,
- readiness scoring, and
- other protocol-focused scenarios.
