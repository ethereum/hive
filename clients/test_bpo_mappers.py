"""Check synthetic BPO aliases and compatibility with numbered rulesets."""

import json
import os
from pathlib import Path
import subprocess
import unittest


CLIENTS = (
    "besu",
    "erigon",
    "ethrex",
    "go-ethereum",
    "nethermind",
    "nimbus-el",
    "reth",
)
PARAMETERS = {
    "TIMESTAMP": "15000",
    "BLOB_TARGET": "21",
    "BLOB_MAX": "32",
    "BLOB_BASE_FEE_UPDATE_FRACTION": "20609697",
}


class BPOMappers(unittest.TestCase):
    def map_genesis(self, client, variables):
        environment = {
            k: v for k, v in os.environ.items() if not k.startswith("HIVE_")
        }
        environment.update(variables)
        mapper = Path(__file__).parent / client / "mapper.jq"
        result = subprocess.run(
            ["jq", "-f", str(mapper)],
            input='{"alloc": {}}',
            text=True,
            capture_output=True,
            check=True,
            env=environment,
        )
        return json.loads(result.stdout)["config"]

    def test_descriptive_aliases(self):
        for client in CLIENTS:
            for descriptive, numbered in (
                ("BPO_INCREASE", "BPO3"),
                ("BPO_DECREASE", "BPO4"),
            ):
                with self.subTest(client=client, fork=descriptive):
                    common = {"HIVE_AMSTERDAM_TIMESTAMP": "0"}
                    old = {
                        f"HIVE_{numbered}_{k}": v
                        for k, v in PARAMETERS.items()
                    }
                    new = {
                        f"HIVE_{descriptive}_{k}": v
                        for k, v in PARAMETERS.items()
                    }
                    legacy = self.map_genesis(client, common | old)
                    aliased = self.map_genesis(client, common | new)
                    self.assertEqual(legacy, aliased)
                    self.assertEqual(aliased[numbered.lower() + "Time"], 15000)
                    self.assertEqual(
                        aliased["blobSchedule"][numbered.lower()],
                        {
                            "target": 21,
                            "max": 32,
                            "baseFeeUpdateFraction": 20609697,
                        },
                    )

    def test_descriptive_values_take_precedence(self):
        for client in CLIENTS:
            with self.subTest(client=client):
                old = {f"HIVE_BPO3_{k}": "1" for k in PARAMETERS}
                new = {
                    f"HIVE_BPO_INCREASE_{k}": v for k, v in PARAMETERS.items()
                }
                self.assertEqual(
                    self.map_genesis(client, new),
                    self.map_genesis(client, old | new),
                )

    def test_partial_alias_preserves_numbered_values(self):
        for client in CLIENTS:
            with self.subTest(client=client):
                old = {f"HIVE_BPO4_{k}": v for k, v in PARAMETERS.items()}
                mixed = old | {"HIVE_BPO_DECREASE_TIMESTAMP": "0"}
                expected = old | {"HIVE_BPO4_TIMESTAMP": "0"}
                self.assertEqual(
                    self.map_genesis(client, mixed),
                    self.map_genesis(client, expected),
                )


if __name__ == "__main__":
    unittest.main()
