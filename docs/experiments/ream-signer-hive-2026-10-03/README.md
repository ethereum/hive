# Standard Hive Ream signer run — 3 October 2026

The preparation containment case fails; the rejection/recovery case passes.
Hive built its unchanged `clients/ream/Dockerfile` with the existing
`simulators/lean/clients/devnet5.yaml --client ream_devnet5` configuration.
No client build arguments were supplied. The standard `ghcr.io/reamlabs/ream:latest`
image was pulled before the run; its executable reports Ream `9cd3498`.
See [manifest.json](manifest.json) for exact image, source and binary identities.

| Scenario | Result |
| --- | --- |
| Preparation containment/recovery | `131070` signs; active-but-unprepared `131072` panics an Actix worker and closes the connection; overlap position `131071` signs. Containment fails, recovery succeeds. |
| Rejection state preservation | `3` signs; inactive `262144` is refused; `4` signs; oversized `2^32+3` is refused; `5` signs. Recovery passes. |

Five signatures independently verify against the configured registry proposal
key. All 20 wrong-root, wrong-position, wrong-key and altered-signature controls
reject. Each case starts a fresh process with future genesis and public disposable
fixtures, isolating these requests from scheduled duties.

The [preparation client log](preparation-client.log) records the assertion in
leanSig `generalized_xmss.rs:798`: the key has not been prepared for the requested
epoch. A subsequent real signature verifies, so this is an RPC worker failure,
not a whole-process crash. It does not establish normal-duty reachability.
A structured refusal would satisfy the containment contract; automatic
preparation is not assumed.

## Evidence

- [Raw Hive result](hive-result.json), including client configuration and case verdicts.
- [Preparation transcript](preparation-containment-recovery.json) and
  [independent report](preparation-containment-recovery-verification.json).
- [Rejection transcript](rejection-state-preservation.json) and
  [independent report](rejection-state-preservation-verification.json).
- [Preparation client log](preparation-client.log) and
  [rejection client log](rejection-client.log).

Client logs retain all lines with ANSI colors and trailing whitespace removed;
the manifest records their original byte hashes.

Only public requests, signatures, public keys, logs and fixture hashes are saved.
Fixture secret bytes are not included; they come from publicly distributed Ream
test assets at a pinned revision.

## Validation and limitations

The four HTTP adapter tests and six independent-oracle regressions pass in the
isolated PR checkout. Formatting passes. The standalone `signer-verifier` Docker
target builds. Replaying the saved live transcripts returns preparation
failure (exit 1) and rejection success (exit 0).

The complete simulator image built successfully from this PR checkout with the
original devnet4/devnet5 build commands and normal Docker layer caching. This
registered run used that complete image (`hive/simulators/lean:signer-draft`),
not a cached-runtime overlay. The controller was built from the isolated checkout
before the first commit; its recorded build base is `43ea47be`. No controller
source changes are included. Image and binary hashes are recorded in the manifest.

For normal execution and independent replay, see the
[suite instructions](../../../simulators/lean/docs/signer-lifecycle.md).
The [coverage audit](../../lean-signer-coverage-audit-2026-09-19.md) describes the
bounded source comparison and why non-reuse is an optional caller-policy check,
not a mandatory endpoint-conformance scenario.

Complete simulator image build: **pass**.
