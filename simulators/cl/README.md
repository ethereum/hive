# CL Spec-Tests Simulator

Runs the [`ethereum/consensus-spec-tests`](https://github.com/ethereum/consensus-spec-tests)
fixtures against each selected CL client, using the client's own native
test runner (vitest, cargo-nextest, gradle reference-tests, bazel test,
nim's `consensus_spec_tests_<preset>`). Each client is built from source
inside a `<client>-specs` image so the source ref can be picked at run
time.

The simulator does not start a daemon; it starts each client image, lets
the image's entrypoint run the native test suite, polls `/status` over
HTTP, then fetches `cl-meta.json` + JUnit XML and reports one hive test
per client.

## Supported clients

| Hive client image    | Source repo              | Native test runner                        |
|----------------------|--------------------------|-------------------------------------------|
| `grandine-specs`     | `grandinetech/grandine`  | `cargo nextest run` (workspace)           |
| `lighthouse-specs`   | `sigp/lighthouse`        | `cargo nextest run -p ef_tests`           |
| `lodestar-specs`     | `ChainSafe/lodestar`     | `pnpm exec vitest run`                    |
| `nimbus-specs`       | `status-im/nimbus-eth2`  | `consensus_spec_tests_<preset> --xml:...` |
| `prysm-specs`        | `OffchainLabs/prysm`     | `bazel test //testing/spectest/...`       |
| `teku-specs`         | `Consensys/teku`         | `:eth-reference-tests:referenceTest`      |

The source repo column above is each image's default; override at run
time via `CL_SPECS_SOURCE_REPO` (e.g. point at a fork).

## Prerequisites

The simulator is a standalone Rust crate. Cargo isn't required to invoke
it — `./hive --sim cl` will build it inside Docker — but a local build
catches compilation errors faster:

```
cd simulators/cl && cargo build
```

## Running

Default client selection (all six spec-runner images):

```
./hive --sim cl \
    --client-file simulators/cl/clients/default.yaml \
    --results-root ./workspace/logs
```

A single client (fastest smoke):

```
./hive --sim cl \
    --client lighthouse-specs \
    --client-file simulators/cl/clients/default.yaml \
    --results-root ./workspace/logs
```

Override the source ref and the consensus-spec-tests version that all
selected clients run against:

```
CL_SPECS_SOURCE_REF=glamsterdam-devnet-5 \
CL_SPECS_TESTS_REF=v1.7.0-alpha.10 \
CL_SPECS_NETWORK=glamsterdam-devnet-5 \
    ./hive --sim cl \
        --client-file simulators/cl/clients/default.yaml \
        --client lighthouse-specs,lodestar-specs
```

## Simulator environment variables

The simulator reads these once at startup and forwards them to every
`*-specs` container it launches.

| Variable               | Forwarded to                       | Default   |
|------------------------|------------------------------------|-----------|
| `CL_SPECS_SOURCE_REPO` | `HIVE_CL_SOURCE_REPO`              | empty (image default per client)  |
| `CL_SPECS_SOURCE_REF`  | `HIVE_CL_SOURCE_REF`               | empty (image default per client)  |
| `CL_SPECS_TESTS_REF`   | `HIVE_CONSENSUS_SPEC_TESTS_REF`    | empty (per-client pinned version) |
| `CL_SPECS_SCOPE`       | `HIVE_CL_SPEC_SCOPE` (`smoke`/`full`/per-client) | `smoke` |
| `CL_SPECS_NETWORK`     | `HIVE_NETWORK` (devnet label, recorded only) | `unknown` |

Per-client scope values match each runner's native vocabulary. `smoke` is
defined by every image; consult the entrypoint script in
`clients/<client>-specs/` for the full set.

## Output layout

Inside each `*-specs` container, under `/out/`:

```
/out/
├── cl-meta.json         # describes what was run and which JUnit files were produced
├── junit/<suite>.xml    # one or more JUnit XMLs from the client's test runner
├── <client>.log         # raw build + test stdout
└── status               # "ok" or "failed rc=<n>"; only written after tests finish
```

The simulator polls `/status` until it appears, then GETs `/cl-meta.json`
and each `junit/*.xml` named in the meta. The parsed pass/fail counts
land on the per-client hive test result; the JUnit XML stays retrievable
from the running container while hive owns it.

## Implementation notes

- The cl simulator is excluded from the root Cargo workspace; build it
  standalone from `simulators/cl/`.
- Each `*-specs` image is a long-lived container: after its native test
  runner completes (success or failure), it `exec`s
  `python3 -m http.server 5151` on `/out` and stays alive until hive
  stops it. This matches the precedent set by
  `clients/lean-spec-client/` (test runner exposed via HTTP).
- One hive test is emitted per client. The hive test's `details` text
  contains the JUnit aggregate counts and up to 50 failed testcase
  names; the rest stay in the JUnit XML retrievable from the container.
