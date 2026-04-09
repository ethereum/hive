#!/usr/bin/env python3

from __future__ import annotations

import os
from pathlib import Path

import yaml

OUTPUT_DIR = Path("/tmp/ream-network")
CONFIG_PATH = OUTPUT_DIR / "config.yaml"

# Matches the devnet3 LeanSpec helper validator set used in Hive.
GENESIS_VALIDATORS = [
    "0x4ebeee6aa607247dadcba631f8ffb473e170c9000181261de029a149a86d825c60c3f626315c9b549d425f2f6d51d94bcbe14a02",
    "0x7017345bbe0feb6b7a926e1f0b988158b7552e81fbbaebd43a87a0cb679f247987f275ba0ca85df1131669f210b5e777a3797618",
    "0x1859df2e6b17775d054a696990aa680b5dcfdfcd5c063b9c6b7f90ed4da48d2a8a503f73574ddde2b0409e148dc42129980ba1af",
]


def main() -> None:
    genesis_time = os.environ.get("HIVE_LEAN_GENESIS_TIME")
    if not genesis_time:
        raise SystemExit("HIVE_LEAN_GENESIS_TIME must be set to generate the ream network config")

    OUTPUT_DIR.mkdir(parents=True, exist_ok=True)
    config = {
        "GENESIS_TIME": int(genesis_time),
        "NUM_VALIDATORS": len(GENESIS_VALIDATORS),
        "GENESIS_VALIDATORS": GENESIS_VALIDATORS,
    }
    with CONFIG_PATH.open("w", encoding="utf-8") as config_file:
        yaml.safe_dump(config, config_file, sort_keys=False)


if __name__ == "__main__":
    main()
