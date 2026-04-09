#!/usr/bin/env python3

import json
import os
import shutil
import time
from pathlib import Path

import yaml

KEYS_DIR = Path("/app/packages/testing/src/consensus_testing/test_keys/prod_scheme")
OUTPUT_DIR = Path("/tmp/lean-spec-client")
GENESIS_DIR = OUTPUT_DIR / "genesis"
KEYS_OUT_DIR = OUTPUT_DIR / "keys"
MANIFEST_DIR = KEYS_OUT_DIR / "hash-sig-keys"
VALIDATOR_COUNT = 3


def load_validator(index: int) -> dict[str, str]:
    with (KEYS_DIR / f"{index}.json").open() as key_file:
        return json.load(key_file)


def parse_validator_indices() -> list[int]:
    raw_indices = os.environ.get("HIVE_LEAN_VALIDATOR_INDICES", "")
    if not raw_indices:
        return []

    return [int(index) for index in raw_indices.split(",") if index]


def prepare_output_dirs() -> None:
    if OUTPUT_DIR.exists():
        shutil.rmtree(OUTPUT_DIR)

    GENESIS_DIR.mkdir(parents=True, exist_ok=True)
    MANIFEST_DIR.mkdir(parents=True, exist_ok=True)


def write_genesis(validators: list[dict[str, str]], genesis_time: int) -> None:
    genesis = {
        "GENESIS_TIME": genesis_time,
        "NUM_VALIDATORS": len(validators),
        "GENESIS_VALIDATORS": [
            {
                "attestation_pubkey": f"0x{validator['attestation_public']}",
                "proposal_pubkey": f"0x{validator['proposal_public']}",
            }
            for validator in validators
        ],
    }

    with (GENESIS_DIR / "config.yaml").open("w") as genesis_file:
        yaml.safe_dump(genesis, genesis_file, sort_keys=False)


def write_validator_keys(
    validators: list[dict[str, str]], node_id: str, validator_indices: list[int]
) -> None:
    with (KEYS_OUT_DIR / "validators.yaml").open("w") as validators_file:
        yaml.safe_dump({node_id: validator_indices}, validators_file, sort_keys=False)

    manifest = {
        "key_scheme": "SIGTopLevelTargetSumLifetime32Dim64Base8",
        "hash_function": "Poseidon1",
        "encoding": "TargetSum",
        "lifetime": 32,
        "log_num_active_epochs": 5,
        "num_active_epochs": 32,
        "num_validators": len(validators),
        "validators": [],
    }

    for index, validator in enumerate(validators):
        attestation_file = f"validator_{index}_attestation.ssz"
        proposal_file = f"validator_{index}_proposal.ssz"

        (MANIFEST_DIR / attestation_file).write_bytes(
            bytes.fromhex(validator["attestation_secret"])
        )
        (MANIFEST_DIR / proposal_file).write_bytes(bytes.fromhex(validator["proposal_secret"]))

        manifest["validators"].append(
            {
                "index": index,
                "attestation_pubkey_hex": f"0x{validator['attestation_public']}",
                "proposal_pubkey_hex": f"0x{validator['proposal_public']}",
                "attestation_privkey_file": attestation_file,
                "proposal_privkey_file": proposal_file,
            }
        )

    with (MANIFEST_DIR / "validator-keys-manifest.yaml").open("w") as manifest_file:
        yaml.safe_dump(manifest, manifest_file, sort_keys=False)


def main() -> None:
    node_id = os.environ.get("HIVE_NODE_ID", "lean_spec_0")
    genesis_time = int(os.environ.get("HIVE_LEAN_GENESIS_TIME", int(time.time()) + 30))
    validator_indices = parse_validator_indices()

    validators = [load_validator(index) for index in range(VALIDATOR_COUNT)]

    prepare_output_dirs()
    write_genesis(validators, genesis_time)
    write_validator_keys(validators, node_id, validator_indices)


if __name__ == "__main__":
    main()
