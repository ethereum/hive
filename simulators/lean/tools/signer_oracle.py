#!/usr/bin/env python3
"""Independent public-evidence verifier. stdin/stdout JSON; exits 0/1/2.

0: declared scenario contract met; 1: client failure; 2: unusable evidence/runtime.
This process never loads secret keys. See signer-lifecycle.md for contract scope.
"""

import importlib.metadata
import importlib.machinery
import importlib.util
import json
import os
from pathlib import Path
import subprocess
import sys

REFERENCE = "0b7d33ecbc9ee2435759c92de4da4d08d7faf1c8"
SCHEME = "SIGAbortingTargetSumLifetime32Dim46Base8"


def require(condition, message):
    if not condition:
        raise ValueError(message)


def unhex(value, length=None):
    require(isinstance(value, str), "Expected hex string")
    raw = bytes.fromhex(value.removeprefix("0x"))
    require(length is None or len(raw) == length, "Wrong encoded length")
    return raw


def configure_reference(root):
    root = Path(root).resolve()
    if (root / ".git").exists():
        revision = subprocess.check_output(
            ["git", "-C", str(root), "rev-parse", "HEAD"], text=True
        ).strip()
        require(
            not subprocess.check_output(
                [
                    "git",
                    "-C",
                    str(root),
                    "status",
                    "--porcelain",
                    "--untracked-files=no",
                ],
                text=True,
            ).strip(),
            "Modified reference checkout",
        )
    else:
        revision = (root / ".git-commit").read_text().strip()
    require(revision == REFERENCE, "Reference revision mismatch")
    require(
        importlib.metadata.version("eth-ssz-specs") == "0.1.0",
        "SSZ dependency mismatch",
    )
    os.environ["LEAN_ENV"] = "prod"
    os.environ.setdefault("NUMBA_CACHE_DIR", "/tmp/hive-signer-numba")
    sys.dont_write_bytecode = True
    sys.path.insert(0, str(root / "src"))
    # LeanSpec's package initializers eagerly import the entire consensus fork and
    # initialize the Rust aggregation prover. Load only the unchanged Python XMSS
    # primitive modules and Slot type. These namespace packages skip __init__.py;
    # they do not replace, mock, or modify any signing/verification implementation.
    for name in (
        "lean_spec.spec.forks",
        "lean_spec.spec.forks.lstar",
        "lean_spec.spec.crypto.xmss",
    ):
        require(name not in sys.modules, f"Reference package already imported: {name}")
        parent, _, leaf = name.rpartition(".")
        parent_module = importlib.import_module(parent)
        spec = importlib.machinery.ModuleSpec(name, loader=None, is_package=True)
        spec.submodule_search_locations = [str(root / "src" / name.replace(".", "/"))]
        module = importlib.util.module_from_spec(spec)
        sys.modules[name] = module
        setattr(parent_module, leaf, module)


def evaluate(evidence):
    from lean_spec.spec.crypto.xmss.containers import PublicKey, Signature
    from lean_spec.spec.crypto.xmss.interface import PROD_SIGNATURE_SCHEME as scheme
    from lean_spec.spec.forks.lstar.slot import Slot
    from lean_spec.spec.ssz_types import Bytes32

    require(evidence["referenceCommit"] == REFERENCE, "Evidence reference mismatch")
    require(evidence["scheme"] == SCHEME, "Unsupported scheme")
    key_bytes = unhex(evidence["expectedPublicKey"], 52)
    key = PublicKey.decode_bytes(key_bytes)
    control_bytes = unhex(evidence["controlPublicKey"], 52)
    require(key_bytes != control_bytes, "Control key equals expected key")
    other = PublicKey.decode_bytes(control_bytes)
    require(evidence["steps"], "Empty transcript")
    # Validate the experiment before attributing any result to the client.
    seen_ids = set()
    for step in evidence["steps"]:
        request = step["request"]
        require(
            step["expected"] in ("signed", "refused", "signed_or_refused", "retry"),
            "Unknown expectation",
        )
        require(
            isinstance(request["requestId"], str)
            and request["requestId"] not in seen_ids,
            "Invalid/repeated request ID",
        )
        seen_ids.add(request["requestId"])
        for field in ("slot", "validatorIndex"):
            require(
                type(request[field]) is int and 0 <= request[field] < 2**64,
                "Invalid request integer",
            )
        unhex(request["blockRoot"], 32)
        require(
            ("response" in step) != ("operationError" in step),
            "Missing/ambiguous operation result",
        )

    failures, results, signatures, roots = [], [], {}, {}
    positive_controls = 0
    for step in evidence["steps"]:
        request, expected = step["request"], step["expected"]
        result = {"requestId": request["requestId"], "expected": expected}
        if "operationError" in step:
            result.update(outcome="operation_error", error=step["operationError"])
            failures.append(
                f"{request['requestId']}: operation failed (never a safe refusal)"
            )
            results.append(result)
            continue
        response = step["response"]
        try:
            for field in ("requestId", "validatorIndex", "slot", "blockRoot"):
                require(
                    response.get(field) == request[field],
                    f"Response correlation mismatch: {field}",
                )
            status = response["status"]
            result["outcome"] = status
            if status == "signed":
                require(
                    response.get("errorCode") is None
                    and response.get("errorMessage") is None,
                    "Signed response has error fields",
                )
                require(
                    unhex(response["proposalPublicKey"], 52) == key_bytes,
                    "Wrong registry key",
                )
                raw = unhex(response["signature"], Signature.get_byte_length())
                sig = Signature.decode_bytes(raw)
                root = unhex(request["blockRoot"], 32)
                slot = Slot(request["slot"])
                require(
                    scheme.verify(key, slot, Bytes32(root), sig), "Invalid signature"
                )
                damaged = bytearray(raw)
                field = int.from_bytes(damaged[-4:], "little")
                damaged[-4:] = (field - 1 if field else 1).to_bytes(4, "little")
                controls = {
                    "wrongRootRejected": not scheme.verify(
                        key, slot, Bytes32(bytes([root[0] ^ 1]) + root[1:]), sig
                    ),
                    "wrongSlotRejected": not scheme.verify(
                        key, Slot(int(slot) ^ 1), Bytes32(root), sig
                    ),
                    "wrongKeyRejected": not scheme.verify(
                        other, slot, Bytes32(root), sig
                    ),
                    "alteredSignatureRejected": not scheme.verify(
                        key, slot, Bytes32(root), Signature.decode_bytes(bytes(damaged))
                    ),
                }
                # A failed control invalidates the oracle, not the client.
                if not all(controls.values()):
                    raise RuntimeError("Oracle negative control accepted")
                result.update(valid=True, controls=controls)
                position = (key_bytes, request["slot"])
                roots.setdefault(position, set()).add(root)
                prior = signatures.get((position, root))
                if expected == "retry":
                    require(
                        prior is not None, "Retry has no earlier verified signature"
                    )
                    require(prior == raw, "Retry released a different signature")
                signatures[position, root] = raw
                if expected == "refused":
                    failures.append(f"{request['requestId']}: signed forbidden request")
                else:
                    positive_controls += 1
            elif status == "refused":
                require(
                    response.get("signature") is None,
                    "Refusal leaks signature material",
                )
                require(
                    isinstance(response.get("errorCode"), str)
                    and response["errorCode"].strip(),
                    "Refusal lacks error code",
                )
                result["errorCode"] = response["errorCode"]
                require(expected != "signed", "Refused eligible request")
                if step.get("errorCode"):
                    require(
                        response["errorCode"] == step["errorCode"],
                        "Wrong refusal category",
                    )
            else:
                raise ValueError(f"Client status {status!r} is not a safe refusal")
        except (ValueError, KeyError, TypeError, AssertionError) as error:
            result["clientError"] = str(error)
            failures.append(f"{request['requestId']}: {error}")
        results.append(result)
    distinct_roots = max((len(value) for value in roots.values()), default=0)
    if evidence.get("requireNonReuse", False) and distinct_roots > 1:
        failures.append(
            "Multiple verified roots for the same actual public key and position"
        )
    if not positive_controls:
        failures.append("No independently verified positive control")
    return {
        "passed": not failures,
        "failures": failures,
        "steps": results,
        "distinctVerifiedRootsAtOnePosition": distinct_roots,
        "referenceCommit": REFERENCE,
        "scheme": SCHEME,
    }


def main():
    try:
        evidence = json.load(sys.stdin)
        configure_reference(
            os.environ.get("LEAN_SIGNER_REFERENCE_ROOT", "/app/signer-reference")
        )
        report = evaluate(evidence)
        print(json.dumps(report))
        return 0 if report["passed"] else 1
    except Exception as error:
        print(json.dumps({"oracleError": str(error)}))
        return 2


if __name__ == "__main__":
    sys.exit(main())
