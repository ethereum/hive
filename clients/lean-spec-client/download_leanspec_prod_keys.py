#!/usr/bin/env python3

from __future__ import annotations

import shutil
import tarfile
import tempfile
import urllib.request
from pathlib import Path
from typing import Final

KEY_ARCHIVE_URL: Final = (
    "https://github.com/leanEthereum/leansig-test-keys/releases/download/"
    "leanSpec-ad9a3226/prod_scheme.tar.gz"
)
OUTPUT_DIR: Final = Path("/app/hive/prod_scheme")
VALIDATOR_COUNT: Final = 3


def main() -> None:
    OUTPUT_DIR.mkdir(parents=True, exist_ok=True)

    with tempfile.TemporaryDirectory() as temp_dir:
        temp_path = Path(temp_dir)
        archive_path = temp_path / "prod_scheme.tar.gz"
        urllib.request.urlretrieve(KEY_ARCHIVE_URL, archive_path)

        with tarfile.open(archive_path, "r:gz") as archive:
            for validator_index in range(VALIDATOR_COUNT):
                member_name = f"prod_scheme/{validator_index}.json"
                archive.extract(member_name, path=temp_path, filter="data")

        extracted_dir = temp_path / "prod_scheme"
        for validator_index in range(VALIDATOR_COUNT):
            source = extracted_dir / f"{validator_index}.json"
            destination = OUTPUT_DIR / f"{validator_index}.json"
            shutil.copy2(source, destination)


if __name__ == "__main__":
    main()
