#!/usr/bin/env python3

from __future__ import annotations

import json
from pathlib import Path
from typing import Final

from lean_spec.subspecs.containers.slot import Slot
from lean_spec.subspecs.xmss import TARGET_SIGNATURE_SCHEME
from lean_spec.subspecs.xmss.containers import ValidatorKeyPair
from lean_spec.types import Uint64

OUTPUT_DIR: Final = Path("/app/hive/test-keys")
VALIDATOR_COUNT: Final = 3
NUM_ACTIVE_SLOTS: Final = Uint64(16)


def main() -> None:
    OUTPUT_DIR.mkdir(parents=True, exist_ok=True)

    for validator_index in range(VALIDATOR_COUNT):
        attestation_key_pair = TARGET_SIGNATURE_SCHEME.key_gen(Slot(0), NUM_ACTIVE_SLOTS)
        proposal_key_pair = TARGET_SIGNATURE_SCHEME.key_gen(Slot(0), NUM_ACTIVE_SLOTS)
        validator_keys = ValidatorKeyPair(
            attestation_public=attestation_key_pair.public,
            attestation_secret=attestation_key_pair.secret,
            proposal_public=proposal_key_pair.public,
            proposal_secret=proposal_key_pair.secret,
        )

        output_path = OUTPUT_DIR / f"{validator_index}.json"
        output_path.write_text(
            json.dumps(validator_keys.to_dict(), sort_keys=True),
            encoding="utf-8",
        )


if __name__ == "__main__":
    main()
