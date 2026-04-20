#!/usr/bin/env python3

from __future__ import annotations

import hashlib
import json
import os
import shutil
import socket
import sys
import time
import urllib.request
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
ZEAM_TEST_KEYS_BASE_URL = (
    "https://raw.githubusercontent.com/blockblaz/zeam-test-keys/main/hash-sig-keys"
)
ZEAM_TEST_KEYS_CACHE = Path(os.environ.get("ZEAM_TEST_KEYS_CACHE", "/tmp/zeam-test-keys/hash-sig-keys"))
GENESIS_TIME = int(os.environ.get("HIVE_LEAN_GENESIS_TIME", str(int(time.time()) + 30)))
BOOTNODE_ENV = os.environ.get("HIVE_BOOTNODES", "")


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


def detect_local_ip() -> str:
    try:
        address = socket.gethostbyname(socket.gethostname())
        if address and address != "127.0.0.1":
            return address
    except OSError:
        pass
    return "127.0.0.1"


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
    specs = [
        {
            "name": NODE_ID,
            "indices": [0],
            "private_key": current_key,
            "ip": detect_local_ip(),
        }
    ]
    for index in range(1, count):
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
    values = [value.strip() for value in BOOTNODE_ENV.split(",") if value.strip()]
    enrs = [value for value in values if value.startswith("enr:")]
    if values and not enrs:
        print(
            "Ignoring unsupported non-ENR HIVE_BOOTNODES values while preparing lean runtime assets",
            file=sys.stderr,
        )
    return enrs or FALLBACK_BOOTNODES


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
    if CLIENT_KIND == "lantern":
        return "".join(f"- {entry}\n" for entry in bootnodes())
    return "".join(f'- "{entry}"\n' for entry in bootnodes())


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
        lines.append(f'{spec["name"]}:')
        for index in spec["indices"]:
            lines.append(f"  - {index}")
    return "\n".join(lines) + "\n"


def ensure_zeam_test_key(index: int, kind: str) -> Path:
    if kind not in {"sk", "pk"}:
        raise ValueError(f"Unsupported zeam key kind {kind!r}")
    filename = f"validator_{index}_{kind}.ssz"
    path = ZEAM_TEST_KEYS_CACHE / filename
    if path.exists():
        return path
    path.parent.mkdir(parents=True, exist_ok=True)
    url = f"{ZEAM_TEST_KEYS_BASE_URL}/{filename}"
    try:
        with urllib.request.urlopen(url, timeout=30) as response, path.open("wb") as out:
            shutil.copyfileobj(response, out)
    except Exception as exc:  # pragma: no cover - network errors
        raise RuntimeError(f"Unable to download zeam test key {filename} from {url}: {exc}") from exc
    return path


def write_ethlambda_assignments(
    asset_root: Path,
    specs: list[dict[str, object]],
    validators: list[dict[str, str]],
) -> None:
    lines: list[str] = []
    for spec in specs:
        lines.append(f'{spec["name"]}:')
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
        lines.append(f'{spec["name"]}:')
        for index in spec["indices"]:
            # Zeam expects XMSS keypairs serialized in SSZ files. Use pre-generated
            # zeam test keys and map them into attester filenames so the runtime
            # accepts the assignment format.
            attester_base = f"validator_{index}_attester"
            sk_filename = f"{attester_base}_sk.ssz"
            pk_filename = f"{attester_base}_pk.ssz"
            shutil.copyfile(
                ensure_zeam_test_key(index, "sk"),
                asset_root / "hash-sig-keys" / sk_filename,
            )
            shutil.copyfile(
                ensure_zeam_test_key(index, "pk"),
                asset_root / "hash-sig-keys" / pk_filename,
            )
            lines.extend(
                [
                    f"  - index: {index}",
                    f'    pubkey_hex: "{validators[index]["attestation_public"]}"',
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
        lines.append(f'{spec["name"]}:')
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


def write_qlean_lean_assignments(asset_root: Path, validators: list[dict[str, str]]) -> None:
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


def write_grandine_lean_assignments(
    asset_root: Path,
    specs: list[dict[str, object]],
    validators: list[dict[str, str]],
) -> None:
    lines: list[str] = []
    for spec in specs:
        lines.append(f'{spec["name"]}:')
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
        lines.append(f'{spec["name"]}:')
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
    # nlean reads hash-sig keys from `<hash-sig-key-dir>/validator_<idx>_{attester,proposer}_key_{pk,sk}.ssz`,
    # as assigned via `--validator-config`. Write both public-key and secret-key
    # files so NodeApp's per-validator path walk succeeds, and emit a per-node
    # libp2p node.key that nlean.sh can feed to --node-key.
    validator_config_lines = [
        "# Auto-generated by prepare_lean_client_assets.py for nlean hive runs.",
        "shuffle: roundrobin",
        "deployment_mode: local",
        "config:",
        "  activeEpoch: 18",
        '  keyType: "hash-sig"',
        "validators:",
    ]
    for spec in specs:
        validator_config_lines.extend(
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
            validator_config_lines.append("    isAggregator: true")

        # nlean reads per-node libp2p keys adjacent to the validator-config at a
        # `${name}.key` path (hex-encoded secp256k1). Write one per spec so the
        # runtime can locate keys for any node in the registry.
        key_hex = trim_hex(spec["private_key"])  # type: ignore[arg-type]
        write_text(asset_root / f"{spec['name']}.key", key_hex + "\n")

        for index in spec["indices"]:
            validator = validators[index]
            if uses_dual_key_genesis():
                attester_sk = f"validator_{index}_attester_key_sk.ssz"
                attester_pk = f"validator_{index}_attester_key_pk.ssz"
                proposer_sk = f"validator_{index}_proposer_key_sk.ssz"
                proposer_pk = f"validator_{index}_proposer_key_pk.ssz"
                write_bytes(
                    asset_root / "hash-sig-keys" / attester_sk,
                    bytes.fromhex(validator["attestation_secret"]),
                )
                write_bytes(
                    asset_root / "hash-sig-keys" / attester_pk,
                    bytes.fromhex(validator["attestation_public"]),
                )
                write_bytes(
                    asset_root / "hash-sig-keys" / proposer_sk,
                    bytes.fromhex(validator["proposal_secret"]),
                )
                write_bytes(
                    asset_root / "hash-sig-keys" / proposer_pk,
                    bytes.fromhex(validator["proposal_public"]),
                )
            else:
                raise ValueError(
                    "nlean hive integration currently requires devnet4 dual-key genesis; "
                    f"got DEVNET_LABEL={DEVNET_LABEL!r}"
                )

    write_text(
        asset_root / "validator-config.yaml",
        "\n".join(validator_config_lines) + "\n",
    )


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
        write_text(asset_root / "validators.yaml", render_validator_registry(specs))
        write_ream_assignments(asset_root, validators)
        return
    if CLIENT_KIND == "nlean":
        # nlean.sh requires validator-config.yaml + hash-sig-keys/; write_nlean_assignments
        # emits both. It also writes per-node libp2p keys under <name>.key so
        # --node-key can point at them.
        write_nlean_assignments(asset_root, specs, validators)
        return
    if CLIENT_KIND == "qlean":
        write_text(asset_root / "validators.yaml", render_validator_registry(specs))
        write_qlean_lean_assignments(asset_root, validators)
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
