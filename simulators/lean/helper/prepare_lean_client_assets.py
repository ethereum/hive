#!/usr/bin/env python3

from __future__ import annotations

import hashlib
import json
import os
import shutil
import sys
import time
from pathlib import Path

SUPPORTED_CLIENTS = {
    "ethlambda",
    "gean",
    "grandine_lean",
    "lantern",
    "nlean",
    "qlean",
    "ream",
    "zeam",
}
FALLBACK_BOOTNODES = [
    "enr:-IW4QA0pljjdLfxS_EyUxNAxJSoGCwmOVNJauYWsTiYHyWG5Bky-7yCEktSvu_w-PWUrmzbc8vYL_Mx5pgsAix2OfOMBgmlkgnY0gmlwhKwUAAGEcXVpY4IfkIlzZWNwMjU2azGhA6mw8mfwe-3TpjMMSk7GHe3cURhOn9-ufyAqy40wEyui",
]

CLIENT_KIND = os.environ.get("LEAN_CLIENT_KIND", "ethlambda")
DEVNET_LABEL = os.environ.get("HIVE_LEAN_DEVNET_LABEL", "devnet3")
NODE_ID = os.environ.get("HIVE_NODE_ID", f"{CLIENT_KIND}_0")
ASSET_ROOT = Path(os.environ.get("LEAN_RUNTIME_ASSET_ROOT", f"/tmp/{CLIENT_KIND}-runtime"))
SOURCE_KEYS_DIR = Path(
    "/app/hive/prod_scheme_devnet4" if DEVNET_LABEL == "devnet4" else "/app/hive/prod_scheme_devnet3"
)
GENESIS_TIME = int(os.environ.get("HIVE_LEAN_GENESIS_TIME", str(int(time.time()) + 30)))
BOOTNODE_ENV = os.environ.get("HIVE_BOOTNODES", "")
LOCAL_IP_PLACEHOLDER = "__HIVE_LOCAL_IP__"


def validate_client_kind() -> None:
    if CLIENT_KIND not in SUPPORTED_CLIENTS:
        raise ValueError(
            f"Unsupported lean runtime asset client kind {CLIENT_KIND!r}; "
            f"expected one of {sorted(SUPPORTED_CLIENTS)!r}"
        )


def uses_dual_key_genesis() -> bool:
    return DEVNET_LABEL == "devnet4"


def trim_hex(value: str) -> str:
    value = value.strip()
    return value[2:] if value.startswith("0x") else value


def deterministic_private_key(label: str) -> str:
    return hashlib.sha256(label.encode("utf-8")).hexdigest()


def observer_only() -> bool:
    return os.environ.get("HIVE_LEAN_CLIENT_RUNTIME_ROLE", "").strip().lower() == "observer"


def observer_needs_validator_assignment() -> bool:
    # These clients can't currently boot with an empty local validator assignment, should be a standardized behavior
    return CLIENT_KIND in {"lantern", "nlean", "qlean", "zeam"}


def parse_runtime_validator_indices() -> list[int] | None:
    raw_indices = os.environ.get("HIVE_LEAN_VALIDATOR_INDICES", "").strip()
    if not raw_indices:
        return None
    return [int(value) for value in raw_indices.split(",") if value.strip()]


def load_source_validator(index: int) -> dict[str, str]:
    validator = json.loads((SOURCE_KEYS_DIR / f"{index}.json").read_text(encoding="utf-8"))
    if {
        "attestation_public",
        "attestation_secret",
        "proposal_public",
        "proposal_secret",
    }.issubset(validator):
        return validator
    if {"public", "secret"}.issubset(validator):
        return {
            "attestation_public": validator["public"],
            "attestation_secret": validator["secret"],
            "proposal_public": validator["public"],
            "proposal_secret": validator["secret"],
        }
    raise ValueError(f"Unsupported validator key format for {SOURCE_KEYS_DIR / f'{index}.json'}")


def parse_override_entries() -> list[tuple[str, str]] | None:
    raw_entries = os.environ.get("HIVE_LEAN_GENESIS_VALIDATOR_ENTRIES", "").strip()
    if raw_entries:
        parsed: list[tuple[str, str]] = []
        for entry in raw_entries.split(","):
            attestation, proposal = entry.split("|", 1)
            parsed.append((trim_hex(attestation), trim_hex(proposal)))
        return parsed

    raw_validators = os.environ.get("HIVE_LEAN_GENESIS_VALIDATORS", "").strip()
    if raw_validators:
        return [(trim_hex(value), trim_hex(value)) for value in raw_validators.split(",")]

    return None


def load_validators() -> list[dict[str, str]]:
    override_entries = parse_override_entries()
    count = len(override_entries) if override_entries is not None else 3
    validators = [load_source_validator(index) for index in range(count)]
    if override_entries is not None:
        for validator, (attestation, proposal) in zip(validators, override_entries):
            validator["attestation_public"] = attestation
            validator["proposal_public"] = proposal
    return validators


def node_specs(count: int) -> list[dict[str, object]]:
    current_key = trim_hex(
        os.environ.get("HIVE_CLIENT_PRIVATE_KEY", deterministic_private_key(f"{CLIENT_KIND}:{NODE_ID}:node"))
    )
    requested_indices = parse_runtime_validator_indices()
    if observer_only():
        current_indices = []
        if observer_needs_validator_assignment():
            current_indices = requested_indices or [0]
    else:
        current_indices = requested_indices or [0]
    specs = [
        {
            "name": NODE_ID,
            "indices": current_indices,
            "private_key": current_key,
            # The simulator prepares runtime assets before Hive creates the
            # client container, so it cannot know the client's eventual Docker
            # IP yet. The client entrypoints replace this placeholder at launch
            # time from inside the live container.
            "ip": LOCAL_IP_PLACEHOLDER,
        }
    ]

    if observer_only() and not observer_needs_validator_assignment():
        # Observer-mode clients should not receive dummy validator assignments.
        # Some runtimes, including Ethlambda, still load those extra keys and
        # start proposing locally, which forks the sync test away from the
        # helper mesh.
        return specs

    owned_indices = set(current_indices)
    for index in range(count):
        if index in owned_indices:
            continue
        specs.append(
            {
                "name": f"{CLIENT_KIND}_dummy_{index}",
                "indices": [index],
                "private_key": deterministic_private_key(f"{CLIENT_KIND}:dummy:{index}"),
                "ip": f"127.0.0.{10 + index}",
            }
        )
    return specs


def bootnodes() -> list[str]:
    if BOOTNODE_ENV.strip().lower() == "none":
        return []

    values = [value.strip() for value in BOOTNODE_ENV.split(",") if value.strip()]
    # Lean runtimes consume bootnodes either from the raw HIVE_BOOTNODES env
    # or from nodes.yaml, and those endpoints can be ENRs or libp2p multiaddrs
    # depending on the client/runtime under test. Preserve the exact values the
    # simulator passes in so helper-mesh tests exercise the real source.
    return values or FALLBACK_BOOTNODES


def write_text(path: Path, contents: str) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(contents, encoding="utf-8")


def write_bytes(path: Path, contents: bytes) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_bytes(contents)


def format_genesis_pubkey(value: str) -> str:
    if CLIENT_KIND == "zeam":
        return value
    return f"0x{value}"


def render_config(validators: list[dict[str, str]]) -> str:
    lines = [f"GENESIS_TIME: {GENESIS_TIME}"]
    if CLIENT_KIND == "ream":
        lines.extend(
            [
                f"NUM_VALIDATORS: {len(validators)}",
                "GENESIS_VALIDATORS:",
            ]
        )
    else:
        lines.extend(
            [
                "ATTESTATION_COMMITTEE_COUNT: 1",
                "MAX_ATTESTATIONS_DATA: 16",
                f"NUM_VALIDATORS: {len(validators)}",
                f"VALIDATOR_COUNT: {len(validators)}",
                "ACTIVE_EPOCH: 18",
                "GENESIS_VALIDATORS:",
            ]
        )
    if uses_dual_key_genesis():
        for validator in validators:
            if CLIENT_KIND == "ream":
                lines.append(
                    f'  - attestation_public_key: "{format_genesis_pubkey(validator["attestation_public"])}"'
                )
                lines.append(
                    f'    proposal_public_key: "{format_genesis_pubkey(validator["proposal_public"])}"'
                )
            else:
                # ethlambda / lantern / zeam / gean-devnet4 /
                # grandine_lean-devnet4 / qlean all accept the
                # attestation_pubkey + proposal_pubkey nested shape.
                lines.append(
                    f'  - attestation_pubkey: "{format_genesis_pubkey(validator["attestation_public"])}"'
                )
                lines.append(
                    f'    proposal_pubkey: "{format_genesis_pubkey(validator["proposal_public"])}"'
                )
    else:
        for validator in validators:
            lines.append(f'  - "{format_genesis_pubkey(validator["attestation_public"])}"')
    return "\n".join(lines) + "\n"


def render_nodes_yaml() -> str:
    entries = bootnodes()
    if CLIENT_KIND == "qlean" and not entries:
        return "[]\n"

    if CLIENT_KIND == "zeam" and BOOTNODE_ENV.strip().lower() == "none":
        return "".join(f"- {entry}\n" for entry in FALLBACK_BOOTNODES)

    if CLIENT_KIND == "lantern":
        return "".join(f"- {entry}\n" for entry in entries)
    return "".join(f'- "{entry}"\n' for entry in entries)


def render_validator_config(specs: list[dict[str, object]]) -> str:
    lines = ["shuffle: roundrobin", "validators:"]
    for spec in specs:
        lines.extend(
            [
                f'  - name: "{spec["name"]}"',
                f'    privkey: "{spec["private_key"]}"',
                "    enrFields:",
                f'      ip: "{spec["ip"]}"',
                "      quic: 9000",
                "      seq: 1",
                f"    count: {len(spec['indices'])}",
            ]
        )
        if spec["name"] == NODE_ID and os.environ.get("HIVE_IS_AGGREGATOR", "0") == "1":
            lines.append("    is_aggregator: true")
    return "\n".join(lines) + "\n"


def render_validator_registry(specs: list[dict[str, object]]) -> str:
    lines: list[str] = []
    for spec in specs:
        indices = spec["indices"]
        if not indices:
            lines.append(f'{spec["name"]}: []')
            continue
        lines.append(f'{spec["name"]}:')
        for index in indices:
            lines.append(f"  - {index}")
    return "\n".join(lines) + "\n"


def append_assignment_header(lines: list[str], spec: dict[str, object]) -> None:
    if spec["indices"]:
        lines.append(f'{spec["name"]}:')
    else:
        lines.append(f'{spec["name"]}: []')


def write_ethlambda_assignments(
    asset_root: Path,
    specs: list[dict[str, object]],
    validators: list[dict[str, str]],
) -> None:
    lines: list[str] = []
    for spec in specs:
        append_assignment_header(lines, spec)
        for index in spec["indices"]:
            validator = validators[index]
            if uses_dual_key_genesis():
                attestation_file = f"validator_{index}_attester_sk.ssz"
                proposal_file = f"validator_{index}_proposer_sk.ssz"
                write_bytes(
                    asset_root / "hash-sig-keys" / attestation_file,
                    bytes.fromhex(validator["attestation_secret"]),
                )
                write_bytes(
                    asset_root / "hash-sig-keys" / proposal_file,
                    bytes.fromhex(validator["proposal_secret"]),
                )
                lines.extend(
                    [
                        f"  - index: {index}",
                        f'    pubkey_hex: "{validator["attestation_public"]}"',
                        f'    privkey_file: "{attestation_file}"',
                        f"  - index: {index}",
                        f'    pubkey_hex: "{validator["proposal_public"]}"',
                        f'    privkey_file: "{proposal_file}"',
                    ]
                )
                continue

            filename = f"validator_{index}_sk.ssz"
            write_bytes(
                asset_root / "hash-sig-keys" / filename,
                bytes.fromhex(validator["attestation_secret"]),
            )
            lines.extend(
                [
                    f"  - index: {index}",
                    f'    pubkey_hex: "{validator["attestation_public"]}"',
                    f'    privkey_file: "{filename}"',
                ]
            )
    write_text(asset_root / "annotated_validators.yaml", "\n".join(lines) + "\n")


def write_zeam_assignments(
    asset_root: Path,
    specs: list[dict[str, object]],
    validators: list[dict[str, str]],
) -> None:
    lines: list[str] = []
    for spec in specs:
        append_assignment_header(lines, spec)
        for index in spec["indices"]:
            validator = validators[index]
            if uses_dual_key_genesis():
                attester_base = f"validator_{index}_attester"
                proposer_base = f"validator_{index}_proposer"
                attester_sk_filename = f"{attester_base}_sk.ssz"
                attester_pk_filename = f"{attester_base}_pk.ssz"
                proposer_sk_filename = f"{proposer_base}_sk.ssz"
                proposer_pk_filename = f"{proposer_base}_pk.ssz"
                write_bytes(
                    asset_root / "hash-sig-keys" / attester_sk_filename,
                    bytes.fromhex(validator["attestation_secret"]),
                )
                write_bytes(
                    asset_root / "hash-sig-keys" / attester_pk_filename,
                    bytes.fromhex(validator["attestation_public"]),
                )
                write_bytes(
                    asset_root / "hash-sig-keys" / proposer_sk_filename,
                    bytes.fromhex(validator["proposal_secret"]),
                )
                write_bytes(
                    asset_root / "hash-sig-keys" / proposer_pk_filename,
                    bytes.fromhex(validator["proposal_public"]),
                )
                lines.extend(
                    [
                        f"  - index: {index}",
                        f'    pubkey_hex: "{validator["attestation_public"]}"',
                        f'    privkey_file: "{attester_sk_filename}"',
                        f"  - index: {index}",
                        f'    pubkey_hex: "{validator["proposal_public"]}"',
                        f'    privkey_file: "{proposer_sk_filename}"',
                    ]
                )
                continue

            base = f"validator_{index}_attester"
            sk_filename = f"{base}_sk.ssz"
            pk_filename = f"{base}_pk.ssz"
            write_bytes(
                asset_root / "hash-sig-keys" / sk_filename,
                bytes.fromhex(validator["attestation_secret"]),
            )
            write_bytes(
                asset_root / "hash-sig-keys" / pk_filename,
                bytes.fromhex(validator["attestation_public"]),
            )
            lines.extend(
                [
                    f"  - index: {index}",
                    f'    pubkey_hex: "{validator["attestation_public"]}"',
                    f'    privkey_file: "{sk_filename}"',
                ]
            )
    write_text(asset_root / "annotated_validators.yaml", "\n".join(lines) + "\n")


def write_lantern_assignments(
    asset_root: Path,
    specs: list[dict[str, object]],
    validators: list[dict[str, str]],
) -> None:
    lines: list[str] = []
    for spec in specs:
        append_assignment_header(lines, spec)
        for index in spec["indices"]:
            validator = validators[index]
            attestation_file = f"validator_{index}_attester_key_sk.ssz"
            proposal_file = f"validator_{index}_proposer_key_sk.ssz"
            write_bytes(
                asset_root / "hash-sig-keys" / attestation_file,
                bytes.fromhex(validator["attestation_secret"]),
            )
            write_bytes(
                asset_root / "hash-sig-keys" / proposal_file,
                bytes.fromhex(validator["proposal_secret"]),
            )
            lines.extend(
                [
                    f"  - index: {index}",
                    f"    pubkey_hex: {validator['attestation_public']}",
                    f"    privkey_file: {attestation_file}",
                    f"  - index: {index}",
                    f"    pubkey_hex: {validator['proposal_public']}",
                    f"    privkey_file: {proposal_file}",
                ]
            )
    write_text(asset_root / "annotated_validators.yaml", "\n".join(lines) + "\n")


def write_qlean_lean_assignments(
    asset_root: Path,
    specs: list[dict[str, object]],
    validators: list[dict[str, str]],
) -> None:
    keys_directory = asset_root / "hash-sig-keys"
    keys_directory.mkdir(parents=True, exist_ok=True)

    is_devnet4 = uses_dual_key_genesis()
    manifest_filename = "validator-keys-manifest-devnet4.yaml" if is_devnet4 else "validator-keys-manifest.yaml"

    manifest_lines = [
        'key_scheme: "SIGTopLevelTargetSumLifetime32Dim64Base8"',
        'hash_function: "Poseidon2"',
        'encoding: "TargetSum"',
        f"num_validators: {len(validators)}",
        "validators:",
    ]
    annotated_lines: list[str] = []

    for index, validator in enumerate(validators):
        manifest_lines.append(f"  - index: {index}")

        attestation_secret_key_file = f"v{index}_att.sk"
        attestation_public_key_file = f"v{index}_att.pk"

        write_bytes(
            keys_directory / attestation_secret_key_file,
            bytes.fromhex(validator["attestation_secret"].removeprefix("0x"))
        )
        write_bytes(
            keys_directory / attestation_public_key_file,
            bytes.fromhex(validator["attestation_public"].removeprefix("0x"))
        )

        if is_devnet4:
            proposal_secret_key_file = f"v{index}_prop.sk"
            proposal_public_key_file = f"v{index}_prop.pk"

            write_bytes(
                keys_directory / proposal_secret_key_file,
                bytes.fromhex(validator["proposal_secret"].removeprefix("0x"))
            )
            write_bytes(
                keys_directory / proposal_public_key_file,
                bytes.fromhex(validator["proposal_public"].removeprefix("0x"))
            )

            manifest_lines.extend([
                f'    attestation_public_key_hex: "{validator["attestation_public"].removeprefix("0x")}"',
                f'    proposal_public_key_hex: "{validator["proposal_public"].removeprefix("0x")}"',
                f'    attestation_private_key_file: "{attestation_secret_key_file}"',
                f'    proposal_private_key_file: "{proposal_secret_key_file}"'
            ])
        else:
            manifest_lines.extend([
                f'    pubkey_hex: "{validator["attestation_public"].removeprefix("0x")}"',
                f'    privkey_file: "{attestation_secret_key_file}"'
            ])

    write_text(keys_directory / manifest_filename, "\n".join(manifest_lines) + "\n")
    if is_devnet4:
        for spec in specs:
            append_assignment_header(annotated_lines, spec)
            for index in spec["indices"]:
                validator = validators[index]
                annotated_lines.extend([
                    f"  - index: {index}",
                    f'    pubkey_hex: "{validator["attestation_public"].removeprefix("0x")}"',
                    f'    privkey_file: "v{index}_att.sk"',
                    f"  - index: {index}",
                    f'    pubkey_hex: "{validator["proposal_public"].removeprefix("0x")}"',
                    f'    privkey_file: "v{index}_prop.sk"',
                ])
        write_text(asset_root / "annotated_validators.yaml", "\n".join(annotated_lines) + "\n")


def write_ream_assignments(
    asset_root: Path,
    validators: list[dict[str, str]],
) -> None:
    manifest_lines = [
        'key_scheme: "SIGTopLevelTargetSumLifetime32Dim64Base8"',
        'hash_function: "Poseidon2"',
        'encoding: "TargetSum"',
        "lifetime: 4294967296",
        "log_num_active_epochs: 18",
        "num_active_epochs: 262144",
        f"num_validators: {len(validators)}",
        "validators:",
    ]

    for index, validator in enumerate(validators):
        manifest_lines.append(f"  - index: {index}")
        if uses_dual_key_genesis():
            attestation_file = f"validator_{index}_attestation_sk.ssz"
            proposal_file = f"validator_{index}_proposal_sk.ssz"
            write_bytes(
                asset_root / "hash-sig-keys" / attestation_file,
                bytes.fromhex(validator["attestation_secret"]),
            )
            write_bytes(
                asset_root / "hash-sig-keys" / proposal_file,
                bytes.fromhex(validator["proposal_secret"]),
            )
            manifest_lines.extend(
                [
                    f'    attestation_public_key_hex: "{format_genesis_pubkey(validator["attestation_public"])}"',
                    f'    proposal_public_key_hex: "{format_genesis_pubkey(validator["proposal_public"])}"',
                    f'    attestation_private_key_file: "{attestation_file}"',
                    f'    proposal_private_key_file: "{proposal_file}"',
                ]
            )
            continue

        filename = f"validator_{index}_sk.ssz"
        write_bytes(
            asset_root / "hash-sig-keys" / filename,
            bytes.fromhex(validator["attestation_secret"]),
        )
        manifest_lines.extend(
            [
                f'    pubkey_hex: "{format_genesis_pubkey(validator["attestation_public"])}"',
                f'    privkey_file: "{filename}"',
            ]
        )

    manifest_name = (
        "validator-keys-manifest-devnet4.yaml"
        if uses_dual_key_genesis()
        else "validator-keys-manifest.yaml"
    )
    write_text(asset_root / "hash-sig-keys" / manifest_name, "\n".join(manifest_lines) + "\n")


def write_ream_validator_registry(
    asset_root: Path,
    specs: list[dict[str, object]],
    validators: list[dict[str, str]],
) -> None:
    if not uses_dual_key_genesis():
        write_text(asset_root / "validators.yaml", render_validator_registry(specs))
        return

    lines: list[str] = []
    for spec in specs:
        append_assignment_header(lines, spec)
        for index in spec["indices"]:
            validator = validators[index]
            lines.extend(
                [
                    f"  - index: {index}",
                    f'    pubkey_hex: "{trim_hex(validator["attestation_public"])}"',
                    f'    privkey_file: "validator_{index}_attestation_sk.ssz"',
                    f"  - index: {index}",
                    f'    pubkey_hex: "{trim_hex(validator["proposal_public"])}"',
                    f'    privkey_file: "validator_{index}_proposal_sk.ssz"',
                ]
            )
    write_text(asset_root / "validators.yaml", "\n".join(lines) + "\n")


def write_grandine_lean_assignments(
    asset_root: Path,
    specs: list[dict[str, object]],
    validators: list[dict[str, str]],
) -> None:
    lines: list[str] = []
    for spec in specs:
        append_assignment_header(lines, spec)
        for index in spec["indices"]:
            validator = validators[index]
            if uses_dual_key_genesis():
                attestation_file = f"validator_{index}_attester_sk.ssz"
                proposal_file = f"validator_{index}_proposer_sk.ssz"
                write_bytes(
                    asset_root / "hash-sig-keys" / attestation_file,
                    bytes.fromhex(validator["attestation_secret"]),
                )
                write_bytes(
                    asset_root / "hash-sig-keys" / proposal_file,
                    bytes.fromhex(validator["proposal_secret"]),
                )
                lines.extend(
                    [
                        f"  - index: {index}",
                        f'    pubkey_hex: "{format_genesis_pubkey(validator["attestation_public"])}"',
                        f'    privkey_file: "{attestation_file}"',
                        f"  - index: {index}",
                        f'    pubkey_hex: "{format_genesis_pubkey(validator["proposal_public"])}"',
                        f'    privkey_file: "{proposal_file}"',
                    ]
                )
                continue

            filename = f"validator_{index}_sk.ssz"
            write_bytes(
                asset_root / "hash-sig-keys" / filename,
                bytes.fromhex(validator["attestation_secret"]),
            )
            lines.extend(
                [
                    f"  - index: {index}",
                    f'    pubkey_hex: "{format_genesis_pubkey(validator["attestation_public"])}"',
                    f'    privkey_file: "{filename}"',
                ]
            )
    write_text(asset_root / "annotated_validators.yaml", "\n".join(lines) + "\n")


def write_gean_assignments(
    asset_root: Path,
    specs: list[dict[str, object]],
    validators: list[dict[str, str]],
) -> None:
    # gean reads annotated_validators.yaml with entries of shape
    # {index, pubkey_hex, privkey_file} and expects the raw XMSS secret
    # key bytes under hash-sig-keys/<privkey_file>. On devnet4 we emit two
    # entries per validator (attestation + proposal) so both keys are
    # available; gean itself only consumes one pubkey per entry today.
    lines: list[str] = []
    for spec in specs:
        append_assignment_header(lines, spec)
        for index in spec["indices"]:
            validator = validators[index]
            if uses_dual_key_genesis():
                attestation_file = f"validator_{index}_attester_sk.ssz"
                proposal_file = f"validator_{index}_proposer_sk.ssz"
                write_bytes(
                    asset_root / "hash-sig-keys" / attestation_file,
                    bytes.fromhex(validator["attestation_secret"]),
                )
                write_bytes(
                    asset_root / "hash-sig-keys" / proposal_file,
                    bytes.fromhex(validator["proposal_secret"]),
                )
                lines.extend(
                    [
                        f"  - index: {index}",
                        f'    pubkey_hex: "{validator["attestation_public"]}"',
                        f'    privkey_file: "{attestation_file}"',
                        f"  - index: {index}",
                        f'    pubkey_hex: "{validator["proposal_public"]}"',
                        f'    privkey_file: "{proposal_file}"',
                    ]
                )
                continue

            filename = f"validator_{index}_sk.ssz"
            write_bytes(
                asset_root / "hash-sig-keys" / filename,
                bytes.fromhex(validator["attestation_secret"]),
            )
            lines.extend(
                [
                    f"  - index: {index}",
                    f'    pubkey_hex: "{validator["attestation_public"]}"',
                    f'    privkey_file: "{filename}"',
                ]
            )
    write_text(asset_root / "annotated_validators.yaml", "\n".join(lines) + "\n")


def write_nlean_assignments(
    asset_root: Path,
    specs: list[dict[str, object]],
    validators: list[dict[str, str]],
) -> None:
    # nlean consumes the shared annotated_validators.yaml (via
    # --annotated-validators) and reads each privkey_file out of
    # --hash-sig-key-dir. Use the same (attestation, proposal) entry-pair
    # layout as ethlambda / gean / grandine_lean — nlean resolves the role
    # from the `_attester` / `_proposer` substring in the filename.
    #
    # nlean also needs a per-node libp2p key file for --node-key; write one
    # alongside each spec so any node in the registry can be launched.
    if not uses_dual_key_genesis():
        raise ValueError(
            "nlean hive integration currently requires devnet4 dual-key genesis; "
            f"got DEVNET_LABEL={DEVNET_LABEL!r}"
        )

    lines: list[str] = []
    for spec in specs:
        key_hex = trim_hex(spec["private_key"])  # type: ignore[arg-type]
        write_text(asset_root / f"{spec['name']}.key", key_hex + "\n")

        append_assignment_header(lines, spec)
        for index in spec["indices"]:
            validator = validators[index]
            attester_sk = f"validator_{index}_attester_sk.ssz"
            proposer_sk = f"validator_{index}_proposer_sk.ssz"
            write_bytes(
                asset_root / "hash-sig-keys" / attester_sk,
                bytes.fromhex(validator["attestation_secret"]),
            )
            write_bytes(
                asset_root / "hash-sig-keys" / proposer_sk,
                bytes.fromhex(validator["proposal_secret"]),
            )
            lines.extend(
                [
                    f"  - index: {index}",
                    f'    pubkey_hex: "{validator["attestation_public"]}"',
                    f'    privkey_file: "{attester_sk}"',
                    f"  - index: {index}",
                    f'    pubkey_hex: "{validator["proposal_public"]}"',
                    f'    privkey_file: "{proposer_sk}"',
                ]
            )
    write_text(asset_root / "annotated_validators.yaml", "\n".join(lines) + "\n")


def write_client_specific_assets(
    asset_root: Path,
    specs: list[dict[str, object]],
    validators: list[dict[str, str]],
) -> None:
    if CLIENT_KIND == "ethlambda":
        write_ethlambda_assignments(asset_root, specs, validators)
        return
    if CLIENT_KIND == "gean":
        # gean.sh verifies /tmp/gean-runtime/validators.yaml exists; write the
        # shared registry even though gean itself reads annotated_validators.yaml.
        write_text(asset_root / "validators.yaml", render_validator_registry(specs))
        write_gean_assignments(asset_root, specs, validators)
        return
    if CLIENT_KIND == "zeam":
        write_zeam_assignments(asset_root, specs, validators)
        return
    if CLIENT_KIND == "lantern":
        write_text(asset_root / "validators.yaml", render_validator_registry(specs))
        write_lantern_assignments(asset_root, specs, validators)
        return
    if CLIENT_KIND == "grandine_lean":
        if uses_dual_key_genesis():
            write_grandine_lean_assignments(asset_root, specs, validators)
        else:
            write_text(asset_root / "validators.yaml", render_validator_registry(specs))
        return
    if CLIENT_KIND == "ream":
        write_ream_validator_registry(asset_root, specs, validators)
        write_ream_assignments(asset_root, validators)
        return
    if CLIENT_KIND == "nlean":
        # nlean consumes annotated_validators.yaml plus per-node libp2p keys
        # written under <name>.key that --node-key can point at.
        write_nlean_assignments(asset_root, specs, validators)
        return
    if CLIENT_KIND == "qlean":
        write_text(asset_root / "validators.yaml", render_validator_registry(specs))
        write_qlean_lean_assignments(asset_root, specs, validators)
        return
    raise ValueError(f"Unsupported lean runtime asset client kind {CLIENT_KIND!r}")


def prepare() -> None:
    validate_client_kind()
    validators = load_validators()
    specs = node_specs(len(validators))

    if ASSET_ROOT.exists():
        shutil.rmtree(ASSET_ROOT)
    (ASSET_ROOT / "hash-sig-keys").mkdir(parents=True, exist_ok=True)

    write_text(ASSET_ROOT / "config.yaml", render_config(validators))
    write_text(ASSET_ROOT / "nodes.yaml", render_nodes_yaml())
    write_text(ASSET_ROOT / "validator-config.yaml", render_validator_config(specs))
    write_client_specific_assets(ASSET_ROOT, specs, validators)
    write_text(ASSET_ROOT / "node.key", trim_hex(specs[0]["private_key"]))  # type: ignore[arg-type]


if __name__ == "__main__":
    prepare()
