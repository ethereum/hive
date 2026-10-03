# Proposal signer integration scenarios

This suite tests two gaps identified in the
[2026-09-19 source and coverage audit](../../../docs/lean-signer-coverage-audit-2026-09-19.md).
It currently supports the Ream devnet5 proposal endpoint only. It does not establish
normal-duty reachability, all-role safety, persistence, concurrency, or full
cryptographic conformance.

## Run

From the Hive repository root, with Docker running and a current Hive executable:

```sh
go run . --sim '^lean$' \
  --client-file simulators/lean/clients/devnet5.yaml --client ream_devnet5 \
  --sim.limit '^signer-lifecycle/' --sim.timelimit 15m
```

This uses Hive’s existing devnet5 client configuration and the default Ream image
from `clients/ream/Dockerfile` (`ghcr.io/reamlabs/ream:latest`). No Ream build
arguments or client image definitions are changed. Record the executable revision
and image digest for each run: `latest` can move beyond the audited revision.
The saved [live evidence](../../../docs/experiments/ream-signer-hive-2026-10-03/README.md)
was collected with Ream `9cd3498b97a5b2dbc54fd5b48c28143b773ca47d`; it is not a
claim about every later image. The simulator packages a separate
LeanSpec oracle at `0b7d33ecbc9ee2435759c92de4da4d08d7faf1c8`. Its Python XMSS
dependency closure is selected from that revision's lockfile and installed from
hash-pinned CPython 3.12 Linux wheels (`tools/signer-requirements.txt`), without
installing the Rust signature/aggregation libraries. The source archive is also
checked against SHA-256
`e376ee30adb8c65239acbf8f7bbb461c93f644b4cb9c89e65e46851d3f05520a`.
Oracle pinning does not change the reference selected by other Lean suites.
The oracle and fixture pins describe the audited XMSS scheme; a client scheme change requires revalidating their compatibility.

Each case uses fresh disposable validator-0 proposal/attestation fixtures from
Ream `5d19570188b5a2b877449e63695c6214db3f7ae9`, exact hash checks and independently
decoded metadata. No private key bytes appear in the JSON evidence or reports.
Genesis is one hour in the future to isolate the signer from scheduled duties.

## Contracts

`preparation-containment-recovery` executes:

1. Position `131070`: require an independently valid signature.
2. Position `131072`: accept an independently valid signature **or a structured
   refusal without signature material**. Report which occurred. This is inside
   activation but outside the initial prepared window.
3. Position `131071`: require another independently valid signature. It remains
   in the overlap even if one preparation advancement occurred.

A panic/connection loss is a failure, even if the node later answers requests.
A structured refusal passes containment only; it does not establish continued
signing across the boundary. There is no hidden expected-failure exemption.

`rejection-state-preservation` executes:

1. Valid signing at position `3`.
2. Refusal at the actual activation end, `262144`.
3. Valid signing at a fresh position, `4`.
4. Refusal at `2^32 + 3` (truncation would alias the initial position).
5. Valid signing at a fresh position, `5`.

Standalone activation and overflow checks already exist upstream. This sequence
checks that these operations do not disrupt subsequent signing in a running
client. An always-refusing client cannot pass either scenario.

Every valid response is checked against the expected genesis/registry proposal
key, exact requested root and position. Wrong-root, wrong-position, wrong-key,
and altered-signature controls must all reject. A control failure invalidates
the oracle rather than diagnosing the client.

## Evidence and verifier

The simulator log contains `SIGNER_EVIDENCE` JSON (public requests, responses,
fixture hashes, decoded bounds, client version and reference pin) and
`SIGNER_REPORT` JSON. Hive reports failed cases normally. Read the client log
before labeling a transport failure a process crash: the RPC worker can panic
while the overall process survives.

The oracle reads one evidence JSON object on stdin. Exit codes are:

- `0`: the declared sequence contract passed;
- `1`: client response/operation violated that contract;
- `2`: evidence, pinned runtime, or independent verifier was unusable.

Replay outside Docker with a clean pinned LeanSpec checkout:

```sh
LEAN_SIGNER_REFERENCE_ROOT=/path/to/leanSpec \
  /path/to/leanSpec/.venv/bin/python simulators/lean/tools/signer_oracle.py \
  < evidence.json
```

Validate the oracle against the saved real merged-Ream signatures and deliberately
broken client responses:

```sh
LEAN_SIGNER_REFERENCE_ROOT=/path/to/leanSpec \
  /path/to/leanSpec/.venv/bin/python -m unittest discover \
  -s simulators/lean/tools -p test_signer_oracle.py -v
cargo test -p lean-sim --locked signer
```

The verifier regression package is also a standalone Docker target:

```sh
docker build --target signer-verifier -f simulators/lean/Dockerfile \
  -t hive/lean-signer-verifier .
docker run --rm hive/lean-signer-verifier
```

`requireNonReuse: true` is available for **explicit policy experiments**. The
saved A/B/A transcript violates it. It is not a registered mandatory conformance
case: leanSig establishes a caller obligation, but current sources do not promise
that Ream's diagnostic endpoint refuses conflicting requests. Byte-identical
retries are distinguished from conflicting roots. See the audit for the exact
security-model-to-implementation translation.
