# hq

`hq` (hive query) is a CLI for inspecting hive test results. It can browse
recent runs, list tests and failures, show colorized diffs, and track pass/fail
rates across runs. By default it queries the public server at
`hive.ethpandaops.io`; see below for pointing it at a local hive run.

## Install

```
go install github.com/ethereum/hive/cmd/hq@latest
```

## Usage

### List clients and aliases

```
hq clients
```

The `-client` flag accepts shorthand aliases anywhere. For example,
`-client geth` resolves to `go-ethereum`, `-client nimbus` to `nimbus-el`.

### List recent runs

```
# All recent runs
hq runs

# Runs for a specific simulator
hq runs -sim rpc-compat

# Runs involving a specific client
hq runs -sim rpc-compat -client geth
```

### Inspect tests in a run

```
# All tests (pass + fail) from the most recent rpc-compat run
hq tests -sim rpc-compat

# Only failing tests, with error details
hq tests -sim rpc-compat -client geth -failed

# Filter by test name (shell-style glob; * crosses /)
hq tests -sim rpc-compat -client geth -test "eth_getBalance*"

# Tests in a specific run file
hq tests 1710000000-rpc-compat.json
```

### View result diffs

```
# Diffs for besu failures in the most recent rpc-compat run
hq diff -sim rpc-compat -client besu

# Narrow down to a specific test
hq diff -sim rpc-compat -client besu -test "debug_getRawHeader/*"

# Show full output (including raw request/response) instead of compact diffs
hq diff -sim rpc-compat -client besu -full

# Diff a specific run file
hq diff 1710000000-xxx.json -client geth
```

### Pass/fail stats across runs

```
# Compare all clients on rpc-compat over the last 10 runs
hq stats -sim rpc-compat

# Track geth's rpc-compat pass rate over time
hq stats -sim rpc-compat -client geth

# Look at more history
hq stats -sim rpc-compat -client geth -last 20
```

### Example workflow

Find out which rpc-compat tests besu is failing and inspect the diffs:

```bash
# 1. See how each client is faring
hq stats -sim rpc-compat

# 2. List besu's failing tests
hq tests -sim rpc-compat -client besu -failed

# 3. See the diffs for all failures
hq diff -sim rpc-compat -client besu

# 4. Narrow down to a specific test
hq diff -sim rpc-compat -client besu -test "debug_getRawHeader/*"
```

## Querying a local hive run

`hq` only speaks HTTP, so to run it against a local hive output directory you
currently need `hiveview` as a proxy:

```bash
# In the hive repo, serve the workspace logs:
go run ./cmd/hiveview -serve -logdir workspace/logs

# In another shell, point hq at it:
hq runs -base-url http://localhost:8080 -no-cache
```

`-no-cache` is useful when iterating locally: the listing has a 5-minute cache
TTL, which is awkward when you're kicking off fresh runs. Direct filesystem
access (no `hiveview` required) is planned.

## Global flags

| Flag | Default | Description |
|------|---------|-------------|
| `-base-url` | `https://hive.ethpandaops.io` | Hive server base URL |
| `-suite` | `generic` | Test suite name |
| `-cache-dir` | `~/.cache/hq` | Cache directory |
| `-no-cache` | `false` | Bypass cache reads |
| `-no-color` | `false` | Disable colored output |

## License

MIT
