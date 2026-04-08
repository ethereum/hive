# Lean Hive Simulator

This simulator contains the initial Rust-based Hive coverage for lean clients.

Today it runs a small API smoke suite that:

- launches each selected `lean` client,
- verifies the health endpoint is reachable, and
- verifies the justified checkpoint endpoint responds with the expected shape.

This gives us a simple onboarding path for new lean clients while leaving a clear place to grow:

- p2p behavior,
- syncing,
- req/resp exchanges,
- RPC compatibility, and
- other protocol-focused scenarios.
