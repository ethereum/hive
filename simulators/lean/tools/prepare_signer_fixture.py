#!/usr/bin/env python3
"""Validate disposable Ream fixtures and emit ONLY their public metadata.

Secret-key decoding is confined to fixture setup, never the evidence oracle.
Run with the pinned LeanSpec interpreter, passing a directory with both SSZ files.
"""

import hashlib
import json
from pathlib import Path
import sys

from signer_oracle import REFERENCE, SCHEME, configure_reference, require

FIXTURE_COMMIT = "5d19570188b5a2b877449e63695c6214db3f7ae9"
FILES = {
    "validator_0_attestation_sk.ssz": "24e8e7484145a115cfaddfd471794417933d861616e0bed9b89c9452ea429968",
    "validator_0_proposal_sk.ssz": "ebe36d97ee4ec988d1182d150c5f117547436530432c6f4b31b168477f050361",
}
ATTESTATION_KEY = "0x9eb14868923a923291404c6a82030e19ba0e3004b9e5b64d2419b8591657f9104298d77399350c43082a6023e812e433bfcdaa4e"
PROPOSAL_KEY = "0xd80efa199a42987324e182419b07e4758b411d5d990f83681e0c6154f8f4af2fe5a8be4a30a25414f7f504117d95a055dc66b019"
CONTROL_KEY = "0x789edd4c2806222f9ae35926d66c2f127192ec2d4e4cfb79e8141d6aae34a01a135f571815b7987b76c6ef3df4fa8a01fcc9785e"


def inspect(directory):
    from lean_spec.spec.crypto.xmss.containers import SecretKey
    from lean_spec.spec.crypto.xmss.interface import PROD_SIGNATURE_SCHEME as scheme

    for filename, digest in FILES.items():
        raw = (directory / filename).read_bytes()
        require(
            hashlib.sha256(raw).hexdigest() == digest,
            f"Fixture hash mismatch: {filename}",
        )
    key = SecretKey.decode_bytes(
        (directory / "validator_0_proposal_sk.ssz").read_bytes()
    )
    active = scheme.get_activation_interval(key)
    prepared = scheme.get_prepared_interval(key)
    require(
        (active.start, active.stop) == (0, 262144), "Unexpected activation interval"
    )
    require(
        (prepared.start, prepared.stop) == (0, 131072), "Unexpected prepared interval"
    )
    require(
        prepared.stop < active.stop,
        "Fixture cannot exercise active-but-unprepared signing",
    )
    return {
        "referenceCommit": REFERENCE,
        "scheme": SCHEME,
        "fixtureCommit": FIXTURE_COMMIT,
        "fixtureHashes": FILES,
        "expectedPublicKey": PROPOSAL_KEY,
        "controlPublicKey": CONTROL_KEY,
        "attestationPublicKey": ATTESTATION_KEY,
        "activation": [active.start, active.stop],
        "prepared": [prepared.start, prepared.stop],
    }


if __name__ == "__main__":
    import os

    configure_reference(
        os.environ.get("LEAN_SIGNER_REFERENCE_ROOT", "/app/signer-reference")
    )
    print(json.dumps(inspect(Path(sys.argv[1])), indent=2))
