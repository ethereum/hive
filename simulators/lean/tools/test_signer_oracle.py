"""Regression checks against real saved signatures, not mock crypto.

LEAN_SIGNER_REFERENCE_ROOT=/path/to/pinned/spec <spec-python> -m unittest discover \
    -s simulators/lean/tools -p test_signer_oracle.py
"""

import copy
import json
import os
from pathlib import Path
import unittest
from unittest.mock import patch

import signer_oracle as oracle


class OracleTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        oracle.configure_reference(os.environ["LEAN_SIGNER_REFERENCE_ROOT"])
        directory = (
            Path(__file__).resolve().parents[3]
            / "docs/experiments/ream-signer-merged-2026-09-07"
        )
        manifest = json.loads((directory / "manifest.json").read_text())
        cls.baseline = {
            "referenceCommit": oracle.REFERENCE,
            "scheme": oracle.SCHEME,
            "expectedPublicKey": manifest["expectedProposalPublicKey"],
            "controlPublicKey": manifest["negativeControlPublicKey"],
            "steps": [
                {
                    "request": json.loads(
                        (directory / f"request-{name}.json").read_text()
                    ),
                    "response": json.loads(
                        (directory / f"response-{name}.json").read_text()
                    ),
                    "expected": "retry" if name == "a-repeat" else "signed",
                }
                for name in manifest["cases"]
            ],
        }

    def test_saved_conflict_is_verified_and_policy_failure_is_explicit(self):
        evidence = copy.deepcopy(self.baseline)
        report = oracle.evaluate(evidence)
        self.assertTrue(report["passed"])
        self.assertEqual(report["distinctVerifiedRootsAtOnePosition"], 2)
        self.assertEqual(sum(len(s["controls"]) for s in report["steps"]), 12)
        evidence["requireNonReuse"] = True
        self.assertFalse(oracle.evaluate(evidence)["passed"])

    def test_client_faults_never_become_safe_refusals(self):
        for fault in (
            "wrong-key",
            "bad-signature",
            "wrong-context",
            "transport",
            "internal-error",
            "refusal-leaks-signature",
        ):
            with self.subTest(fault=fault):
                evidence = copy.deepcopy(self.baseline)
                evidence["steps"] = evidence["steps"][:1]
                step = evidence["steps"][0]
                step["expected"] = "signed_or_refused"
                if fault == "wrong-key":
                    step["response"]["proposalPublicKey"] = evidence["controlPublicKey"]
                elif fault == "bad-signature":
                    step["response"]["signature"] = "0x00"
                elif fault == "wrong-context":
                    step["response"]["slot"] += 1
                elif fault == "transport":
                    del step["response"]
                    step["operationError"] = "connection closed"
                elif fault == "internal-error":
                    step["response"]["status"] = "error"
                else:
                    step["response"].update(status="refused", errorCode="not_prepared")
                self.assertFalse(oracle.evaluate(evidence)["passed"])

    def test_refusal_and_verified_recovery_are_both_required(self):
        evidence = copy.deepcopy(self.baseline)
        step = evidence["steps"][1]
        step["expected"] = "refused"
        step["response"].update(
            status="refused",
            signature=None,
            proposalPublicKey=None,
            errorCode="policy_refusal",
        )
        self.assertTrue(oracle.evaluate(evidence)["passed"])
        # A positive before the failure cannot substitute for recovery afterwards.
        evidence["steps"][2]["response"].update(
            status="refused", signature=None, errorCode="broken_after_failure"
        )
        evidence["steps"][2]["expected"] = "signed"
        self.assertFalse(oracle.evaluate(evidence)["passed"])

    def test_always_refusing_client_cannot_pass(self):
        evidence = copy.deepcopy(self.baseline)
        for step in evidence["steps"]:
            step["expected"] = "signed_or_refused"
            step["response"].update(
                status="refused", signature=None, errorCode="unsupported"
            )
        self.assertFalse(oracle.evaluate(evidence)["passed"])

    def test_evidence_mismatch_is_not_attributed_to_client(self):
        evidence = copy.deepcopy(self.baseline)
        evidence["referenceCommit"] = "wrong-reference"
        with self.assertRaises(ValueError):
            oracle.evaluate(evidence)

    def test_broken_negative_control_invalidates_oracle(self):
        from lean_spec.spec.crypto.xmss.interface import GeneralizedXmssScheme

        with patch.object(GeneralizedXmssScheme, "verify", return_value=True):
            with self.assertRaises(RuntimeError):
                oracle.evaluate(self.baseline)


if __name__ == "__main__":
    unittest.main()
