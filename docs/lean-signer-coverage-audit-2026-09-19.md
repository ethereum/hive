# Signer coverage and source audit — 19 September 2026

## Decision

Implement two **live-client integration sequences**, not a second collection of
primitive boundary tests:

1. `preparation-containment-recovery`: verify signing just before the prepared
   window ends, request an active but unprepared position, then independently
   verify another signature in the overlap of the old and next windows.
2. `rejection-state-preservation`: verify signing, send an inactive request,
   verify fresh signing, send an oversized position that would alias a previously
   signed position if truncated, then verify fresh signing again.

The gap is process/HTTP containment and preservation of actual signing ability
across those operations. The primitive inequalities and ordinary sign/verify
checks already have coverage. Positive and negative crypto controls are necessary
measurement checks, not claimed as new cryptographic test cases.

The preparation case permits either a valid signature or a structured refusal
at the unprepared position. It does **not** assert an undocumented automatic
preparation capability. A refusal is recorded as a limitation, not evidence of
successful advancement. Connection loss, timeout, internal error, invalid
signature, and failure of the subsequent eligible request all fail this contract.

## Revisions checked

Live `git ls-remote` checks on 2026-09-19 returned:

| Source | Revision |
| --- | --- |
| Ream default HEAD | `b003b250f51c038cd5e16b8da02694ee0db1997e` |
| LeanSpec default HEAD | `0b7d33ecbc9ee2435759c92de4da4d08d7faf1c8` |
| leanSig default HEAD | `c08a3bae74b0d85379cab72dcbefa4091546ecbb` |
| leanSig `devnet4` branch and Ream's Cargo.lock dependency | `15cbdd43ec8525aa43fea2f42cafc5ed366084ae` |
| Hive upstream HEAD | `43ea47bef5761351e3da7b726050ea80ab362c52` |

At the audit date, Ream was cloned into a temporary audit directory. Its proposal signer is
byte-identical to the previously tested merge. At that date, LeanSpec matched the local
clean reference. The default leanSig branch is **not** the implementation selected
by Ream: both the current default branch and the selected dependency were read.
The cached Ream Docker executable reports `ream b003b25-b003b25 linux-aarch64 rustc1.98.0`.

## Existing coverage compared

| Proposed assertion | Existing test/source | Action |
| --- | --- | --- |
| Ordinary proposal sign/verify and correct key | Ream `signer.rs::signs_with_the_requested_proposal_key`; RPC `signs_and_returns_structured_refusals`; LeanSpec real block/attestation integration tests | No standalone addition; use as scenario precondition |
| Unknown validator | Ream `rejects_an_unknown_validator` and RPC refusal test | Exclude |
| `2^32` overflow | Ream `rejects_a_slot_that_cannot_be_an_xmss_epoch` | No standalone addition; use an overflow-alias input inside a recovery sequence |
| Activation-end rejection | Ream `rejects_a_slot_outside_the_key_activation_interval`; LeanSpec `test_sign_rejects_slot_outside_activation` | No standalone addition; use as recovery trigger |
| Preparation bounds and advancement | LeanSpec `test_get_prepared_interval`, `test_advance_preparation`, `test_sign_requires_prepared_interval`, `test_advance_preparation_is_a_noop_at_the_end`; leanSig correctness helper explicitly advances the key | Do not duplicate primitive assertions |
| Validator service preparation loop | LeanSpec `TestSignWithKey` tests no/one/multiple advancement and per-role registry updates, with the scheme patched | Adjacent coverage; does not exercise real Ream HTTP failure containment or real production-parameter recovery |
| Same-message deterministic retry | LeanSpec `test_deterministic_signing` | No standalone addition |
| Duplicate scheduled attestation | LeanSpec `test_duplicate_prevention_same_slot_not_attested_twice` | Existing scheduling coverage; distinct from arbitrary shared proposal-signer calls |
| Role-key alias rejection | LeanSpec validator registry rejects a shared proposal/attestation public key | Exclude from this milestone |
| Invalid signatures, roots, registries, component ordering | LeanSpec `tests/consensus/lstar/verify_signatures`, also consumed by Ream and Hive fixture runners | Exclude; mutation controls only validate the independent oracle |
| Active-unprepared request through running Ream, followed by verified overlap signing | No matching sequence found in inspected tests | Add containment/recovery case |
| Inactive and oversized requests followed by independently verified fresh signatures on the same running signer | No matching sequence found in inspected tests | Add state-preservation case |

Primary code locations:

- [Ream signer and its four tests](https://github.com/ReamLabs/ream/blob/b003b250f51c038cd5e16b8da02694ee0db1997e/crates/common/validator/lean/src/signer.rs).
- [Ream RPC tests](https://github.com/ReamLabs/ream/blob/b003b250f51c038cd5e16b8da02694ee0db1997e/crates/rpc/lean/src/handlers/test_driver.rs).
- [Ream actual production call](https://github.com/ReamLabs/ream/blob/b003b250f51c038cd5e16b8da02694ee0db1997e/crates/common/validator/lean/src/service.rs).
- [LeanSpec primitive tests](https://github.com/leanEthereum/leanSpec/blob/0b7d33ecbc9ee2435759c92de4da4d08d7faf1c8/tests/spec/crypto/xmss/test_interface.py).
- [LeanSpec service tests](https://github.com/leanEthereum/leanSpec/blob/0b7d33ecbc9ee2435759c92de4da4d08d7faf1c8/tests/node/validator/test_service.py).
- [LeanSpec registry tests](https://github.com/leanEthereum/leanSpec/blob/0b7d33ecbc9ee2435759c92de4da4d08d7faf1c8/tests/node/validator/test_registry.py).

Search scope: tracked Ream Rust source/testing and LeanSpec Python tests, with
`sign_proposal`, `prepare_signature`, `prepared.interval`, `advance.preparation`,
`131.?072`, `overflow`, `conflict`, `reuse`, `double.sign`, and `sign.*twice`;
then full reads of the matching signer, RPC, primitive and service tests. Also
read the selected leanSig correctness helper and the Hive fixture registration.
This is a bounded overlap audit, not global novelty or proof about all CI jobs.
The same investigator performed the search and source comparison. Unmerged PRs,
private tests, and all other clients' latest branches are outside this claim.

## Non-reuse: verified requirement and its limits

**Source authentication.** The IACR pages for Drake, Khovratovich, Kudinov and
Wagner's *Hash-Based Multi-Signatures for Post-Quantum Ethereum*, ePrint 2025/055,
and *Technical Note: LeanSig for Post-Quantum Ethereum*, 2025/1332, both show
2025-09-12 as the latest revision when checked. Their current public PDFs were
downloaded and text extracted with pypdf. Temporary-source SHA-256:

- 2025/055: `4963cb3e48d6e445aee58de1abc5989d992fb2b1b4c5eeb1b97f02c4b2bddd47`
- 2025/1332: `5d75397d4345ee11f84b980a611716b489140a8b730317be279f9cad0b9fdb7e`

The newer *Aborting Random Oracles: How to Build them, How to Use them*, by
Herold, Khovratovich, Kudinov, Tessaro and Wagner,
[ePrint 2026/016](https://eprint.iacr.org/2026/016), was also checked at its
latest listed revision, **2026-08-11** (CRYPTO 2026). Its section 7.1, printed
page 29, defines the incomparable-encoding oracle to refuse another query
with the same epoch; Lemma 11 links the encoding property back to synchronized
signatures. Thus this newer aborting-encoding analysis does not remove the
per-epoch query restriction. This was a check of the relevant definition and
its stated linkage, not a full proof or parameter audit. PDF SHA-256:
`790181a02564db166a064c3b9c649d027efdb2044f82680c820667b84c0f31cf`.

**Exact model.** Definition 8, section 3.2, printed page 12 of
[2025/055](https://eprint.iacr.org/2025/055) uses a signing oracle that refuses a
second query for an already used epoch. Remark 2 specifies strong unforgeability.
This is a security-model restriction, not a requirement that a verifier reject
the second honestly generated signature, and not an RPC specification.
[2025/1332](https://eprint.iacr.org/2025/1332) changes the instantiation within that
framework; it does not supply a Ream endpoint refusal policy.

**Translation to this implementation.** Ream maps a checked `u64` proposal slot
to the `u32` leanSig epoch and signs the 32-byte block root with the proposal key.
The operational identity is therefore `(actual public key, signing position)`;
different validator labels must not be treated as independent keys.

The [selected leanSig `SignatureScheme::sign` documentation](https://github.com/leanEthereum/leanSig/blob/15cbdd43ec8525aa43fea2f42cafc5ed366084ae/src/signature.rs#L155)
puts the restriction on callers and explicitly explains deterministic identical
input retries as a hardening measure that does not compromise security. The
[current default branch](https://github.com/leanEthereum/leanSig/blob/c08a3bae74b0d85379cab72dcbefa4091546ecbb/src/signature.rs)
retains this explanation. The trait overview's blanket statement about even
identical-message reuse is less precise than this method-specific qualification.

**Application.** Two independently verified signatures for different roots under
the same key and position demonstrate an invocation outside the intended signing
discipline. Ream's shared signer does not currently enforce root history; its
test endpoint deliberately exposes that boundary. Thus the saved baseline is
valid evidence of missing protection **at that boundary**, not proof of normal
duty reachability, a consensus-spec violation, a forgery, or a current exploit.

The new oracle can evaluate `requireNonReuse: true` as an explicit safety policy.
Its regression test checks the saved A/B/A evidence and all 12 crypto controls.
This policy is **not added as a mandatory client-conformance Hive scenario**:
the latest sources establish the caller obligation but do not establish the
endpoint's obligation to refuse conflicts. A useful next step for that branch
of work is tracing an actual supported production caller/recovery path.

## Fixture validation

The cached Hive default devnet5 fixture decodes to activation `[0,131072)` and
preparation `[0,131072)`. It cannot test active-but-unprepared signing. The suite
instead downloads the two disposable validator-0 keys from pinned Ream merge
`5d195701…`, checks their exact SHA-256 values, and independently decodes the
proposal key's activation `[0,262144)` and preparation `[0,131072)`.

Expected public keys come from the pinned Ream registry; the runtime genesis and
registry receive those same keys. No response-selected verification key is used.
Each case starts a fresh process with genesis one hour ahead so normal duties
cannot sign the chosen positions during the bounded scenario. The recovery
position `131071` belongs to both the initial and next prepared windows.

## Standard-image revalidation — 3 October 2026

The source-overlap audit above is dated and bounded to its listed revisions.
A new registered live run used Hive’s unchanged Ream Dockerfile and devnet5
client file, without build-argument overrides. The standard image reports
`9cd3498b97a5b2dbc54fd5b48c28143b773ca47d`. It reproduces the preparation worker
panic, independently verified recovery, and successful rejection-state sequence.
All five signatures verify and all 20 negative controls reject. This refreshes
the live result, not the exhaustive scope of the historical coverage search.
See [current public evidence and build limitations](experiments/ream-signer-hive-2026-10-03/README.md).
