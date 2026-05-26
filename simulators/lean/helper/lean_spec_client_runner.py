#!/usr/bin/env python3

from __future__ import annotations

import asyncio
import gc
import importlib
import inspect
import io
import ipaddress
import json
import logging
import os
import shutil
import sys
import time
import types
from contextlib import suppress
from pathlib import Path
from typing import Any, Final

from aiohttp import web
from Crypto.Hash import keccak
from cryptography.hazmat.primitives import hashes
from cryptography.hazmat.primitives.asymmetric import ec
from cryptography.hazmat.primitives.asymmetric.utils import (
    Prehashed,
    decode_dss_signature,
    encode_dss_signature,
)
import yaml


def install_numba_njit_stub() -> None:
    if os.environ.get("HIVE_LEAN_SPEC_USE_NUMBA", "0") == "1" or "numba" in sys.modules:
        return

    numba_module = types.ModuleType("numba")

    def njit(*args: object, **_: object) -> object:
        if len(args) == 1 and callable(args[0]):
            return args[0]

        def decorate(func: object) -> object:
            return func

        return decorate

    numba_module.njit = njit
    sys.modules["numba"] = numba_module


install_numba_njit_stub()


from lean_spec.subspecs.api import ApiServer, ApiServerConfig
from lean_spec.subspecs.chain.config import ATTESTATION_COMMITTEE_COUNT
from lean_spec.subspecs.genesis.config import GenesisConfig
from lean_spec.subspecs.metrics import registry as metrics
from lean_spec.subspecs.networking.client import LiveNetworkEventSource
from lean_spec.subspecs.networking.enr import ENR, keys as enr_keys
from lean_spec.subspecs.networking.gossipsub import GossipTopic
from lean_spec.subspecs.networking.reqresp.message import Status
from lean_spec.subspecs.networking.service.events import GossipBlockEvent
from lean_spec.subspecs.networking.transport.identity import IdentityKeypair
from lean_spec.subspecs.networking.transport.identity.keypair import Secp256k1PublicKey
from lean_spec.subspecs.networking.transport.quic.connection import (
    QuicConnectionManager,
    QuicStream,
)
from lean_spec.subspecs.node import Node, NodeConfig
from lean_spec.subspecs.xmss import aggregation as xmss_aggregation_module
try:
    from lean_spec.subspecs.xmss.aggregation import (
        AggregatedSignatureProof as TypeOneSignatureProof,
    )
except ImportError:
    from lean_spec.subspecs.xmss.aggregation import TypeOneMultiSignature as TypeOneSignatureProof
from lean_spec.subspecs.ssz.hash import hash_tree_root
from lean_spec.subspecs.validator import ValidatorRegistry
from lean_spec.subspecs.validator.registry import ValidatorEntry
from lean_spec.subspecs.xmss import SecretKey
from lean_spec.types import Bytes32
try:
    from lean_spec.types.participation import AggregationBits, ValidatorIndices
except ImportError:
    from lean_spec.types import AggregationBits, ValidatorIndices
from lean_spec.types.collections import SSZList, SSZVector, _validate_offsets
from lean_spec.types.constants import OFFSET_BYTE_LENGTH
from lean_spec.types.container import Container
from lean_spec.types.exceptions import SSZSerializationError, SSZValueError
from lean_spec.types.uint import Uint32, Uint64
from lean_spec_runtime import (
    Checkpoint,
    SubnetId,
    ValidatorIndex,
    build_node_config,
    configure_event_source_network,
)

DEFAULT_GOSSIP_FORK_DIGEST: Final = "devnet0"
DEFAULT_LISTEN_PORT: Final = 9001
DEFAULT_API_PORT: Final = 5052
DEFAULT_HELPER_METADATA_PORT: Final = 5053
BOOTNODE_DIAL_TIMEOUT_SECS: Final = 10.0
STATUS_REFRESH_INTERVAL_SECS: Final = 1.0
DEFAULT_NODE_ID: Final = "lean_spec_0"
PREPARED_ASSETS_DIR: Final = Path(
    os.environ.get("LEAN_RUNTIME_ASSET_ROOT", "/tmp/lean-spec-client")
)
DEVNET3_SOURCE_KEYS_DIR: Final = Path("/app/hive/prod_scheme_devnet3")
DEVNET4_SOURCE_KEYS_DIR: Final = Path("/app/hive/prod_scheme_devnet4")
GENESIS_DIR: Final = PREPARED_ASSETS_DIR / "genesis"
GENESIS_CONFIG_PATH: Final = PREPARED_ASSETS_DIR / "genesis" / "config.yaml"
VALIDATOR_KEYS_DIR: Final = PREPARED_ASSETS_DIR / "keys"
MANIFEST_DIR: Final = VALIDATOR_KEYS_DIR / "hash-sig-keys"
VALIDATORS_YAML_PATH: Final = VALIDATOR_KEYS_DIR / "validators.yaml"
VALIDATOR_MANIFEST_PATH: Final = (
    VALIDATOR_KEYS_DIR / "hash-sig-keys" / "validator-keys-manifest.yaml"
)
HELPER_IDENTITY_PRIVATE_KEY_HEX: Final = (
    "1111111111111111111111111111111111111111111111111111111111111111"
)
SECP256K1_ORDER: Final = int(
    "FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFEBAAEDCE6AF48A03BBFD25E8CD0364141",
    16,
)
HELPER_IDENTITY_PRIVATE_KEY_ENVIRONMENT_VARIABLE: Final = (
    "HIVE_LEAN_HELPER_IDENTITY_PRIVATE_KEY"
)
HELPER_ADVERTISE_IP_ENVIRONMENT_VARIABLE: Final = "HIVE_LEAN_HELPER_ADVERTISE_IP"
HELPER_GOSSIP_FORK_DIGEST_ENVIRONMENT_VARIABLE: Final = "HIVE_LEAN_HELPER_GOSSIP_FORK_DIGEST"
HELPER_P2P_PORT_ENVIRONMENT_VARIABLE: Final = "HIVE_LEAN_HELPER_P2P_PORT"
HELPER_API_PORT_ENVIRONMENT_VARIABLE: Final = "HIVE_LEAN_HELPER_API_PORT"
HELPER_METADATA_PORT_ENVIRONMENT_VARIABLE: Final = "HIVE_LEAN_HELPER_METADATA_PORT"
DISABLE_VALIDATOR_SERVICE_ENVIRONMENT_VARIABLE: Final = "HIVE_LEAN_DISABLE_VALIDATOR_SERVICE"
GENESIS_VALIDATOR_COUNT_ENVIRONMENT_VARIABLE: Final = "HIVE_LEAN_VALIDATOR_COUNT"
GOSSIPSUB_V11_PROTOCOL_ID: Final = "/meshsub/1.1.0"
IDENTIFY_PROTOCOL_ID: Final = "/ipfs/id/1.0.0"
IDENTIFY_PUSH_PROTOCOL_ID: Final = "/ipfs/id/push/1.0.0"
LIBP2P_SECP256K1_PUBLIC_KEY_TYPE: Final = 2
DEFAULT_VALIDATOR_COUNT: Final = 3
ATTESTATION_PUBKEY_FIELD: Final = "attestation_public"
ATTESTATION_SECRET_FIELD: Final = "attestation_secret"
PROPOSAL_PUBKEY_FIELD: Final = "proposal_public"
PROPOSAL_SECRET_FIELD: Final = "proposal_secret"
LEGACY_PUBLIC_FIELD: Final = "public"
LEGACY_SECRET_FIELD: Final = "secret"
WIRE_COMPAT_ENVIRONMENT_VARIABLE: Final = "HIVE_LEAN_SPEC_WIRE_COMPAT"
WIRE_COMPAT_LEGACY: Final = "legacy"
WIRE_COMPAT_NATIVE: Final = "native"

logger = logging.getLogger("lean_spec_client_runner")
LEGACY_SIGNATURE_LENGTH: Final = 2536
SIGNATURE_PATH_OFFSET: Final = 36
SIGNATURE_HASHES_OFFSET: Final = 1064
SIGNATURE_PATH_SIBLINGS_OFFSET: Final = 4


def api_server_supports_signed_block_getter() -> bool:
    return "signed_block_getter" in inspect.signature(ApiServer).parameters


def identity_keypair_from_private_key_hex(private_key_hex: str) -> IdentityKeypair:
    private_key_bytes = Bytes32(bytes.fromhex(private_key_hex))

    if hasattr(IdentityKeypair, "from_bytes"):
        return IdentityKeypair.from_bytes(private_key_bytes)

    private_key_value = int.from_bytes(private_key_bytes, "big")
    if not 0 < private_key_value < SECP256K1_ORDER:
        raise ValueError("helper identity private key is outside the secp256k1 scalar range")

    private_key = ec.derive_private_key(private_key_value, ec.SECP256K1())
    return IdentityKeypair(
        private_key=private_key,
        public_key=Secp256k1PublicKey(_key=private_key.public_key()),
    )


def canonicalize_der_secp256k1_signature(der_signature: bytes) -> bytes:
    signature_r, signature_s = decode_dss_signature(der_signature)
    if signature_s > SECP256K1_ORDER // 2:
        signature_s = SECP256K1_ORDER - signature_s
    return encode_dss_signature(signature_r, signature_s)


def install_low_s_identity_signature_compatibility() -> None:
    global _LOW_S_IDENTITY_SIGNATURE_COMPAT_INSTALLED

    if _LOW_S_IDENTITY_SIGNATURE_COMPAT_INSTALLED:
        return

    original_sign = IdentityKeypair.sign

    def sign_low_s(self: IdentityKeypair, message: bytes) -> bytes:
        return canonicalize_der_secp256k1_signature(original_sign(self, message))

    IdentityKeypair.sign = sign_low_s
    _LOW_S_IDENTITY_SIGNATURE_COMPAT_INSTALLED = True


_QUIC_STREAM_CLOSE_COMPAT_INSTALLED = False
_SSZ_FAST_DESERIALIZATION_INSTALLED = False
_SNAPPY_COMPRESS_FALLBACK_INSTALLED = False
_REQRESP_BLOCK_CACHE_TRACKING_INSTALLED = False
_NETWORK_SERVICE_GOSSIP_TRACKING_INSTALLED = False
_IDENTIFY_PROTOCOL_COMPAT_INSTALLED = False
_CHILD_ONLY_AGGREGATION_COMPAT_INSTALLED = False
_SETUP_PROVER_COMPAT_INSTALLED = False
_TRUSTED_GOSSIP_ATTESTATION_COMPAT_INSTALLED = False
_CHECKPOINT_FINALIZATION_COMPAT_INSTALLED = False
_LEGACY_SIGNED_BLOCK_WIRE_COMPAT_INSTALLED = False
_LOW_S_IDENTITY_SIGNATURE_COMPAT_INSTALLED = False
_BLOCK_CACHE_BY_REQRESP_CLIENT: dict[int, dict[Bytes32, object]] = {}
_BLOCK_CACHE_BY_SYNC_SERVICE: dict[int, dict[Bytes32, object]] = {}
_SYNC_EVENT_CONTEXT: dict[int, tuple[LiveNetworkEventSource, Node]] = {}


def setup_logging() -> None:
    logging.basicConfig(
        level=logging.INFO,
        format="%(asctime)s %(levelname)-8s %(name)s: %(message)s",
        datefmt="%Y-%m-%d %H:%M:%S",
    )


def _ssz_offset(value: int) -> bytes:
    return value.to_bytes(OFFSET_BYTE_LENGTH, "little")


def _safe_len(value: object | None) -> int | None:
    if value is None:
        return None

    try:
        return len(value)  # type: ignore[arg-type]
    except TypeError:
        return None


def legacy_attestation_count(block: object) -> int:
    body = getattr(block, "body", None)
    attestations = getattr(body, "attestations", None)
    if attestations is None:
        attestations = getattr(block, "attestations", None)

    for candidate in (getattr(attestations, "data", None), attestations):
        count = _safe_len(candidate)
        if count is not None:
            return count

    return 0


def _encode_aggregated_signature_proof(
    participants: bytes = b"\x01",
    proof_data: bytes = b"",
) -> bytes:
    fixed_part_length = OFFSET_BYTE_LENGTH * 2
    return (
        _ssz_offset(fixed_part_length)
        + _ssz_offset(fixed_part_length + len(participants))
        + participants
        + proof_data
    )


def _encode_aggregated_signature_proofs(proofs: list[bytes]) -> bytes:
    offsets = bytearray()
    offset = OFFSET_BYTE_LENGTH * len(proofs)
    for proof in proofs:
        offsets += _ssz_offset(offset)
        offset += len(proof)
    return bytes(offsets) + b"".join(proofs)


def _encode_placeholder_aggregated_signature_proofs(count: int) -> bytes:
    return _encode_aggregated_signature_proofs(
        [_encode_aggregated_signature_proof() for _ in range(count)]
    )


def _encode_blank_xmss_signature() -> bytes:
    signature = bytearray(LEGACY_SIGNATURE_LENGTH)
    signature[0:4] = _ssz_offset(SIGNATURE_PATH_OFFSET)
    signature[32:36] = _ssz_offset(SIGNATURE_HASHES_OFFSET)
    signature[36:40] = _ssz_offset(SIGNATURE_PATH_SIBLINGS_OFFSET)
    return bytes(signature)


def encode_ssz_bytes(value: object) -> bytes:
    if isinstance(value, bytes):
        return value
    if isinstance(value, bytearray):
        return bytes(value)

    encode_bytes = getattr(value, "encode_bytes", None)
    if encode_bytes is not None:
        return encode_bytes()

    return bytes(value)  # type: ignore[arg-type]


def extract_proposer_signature_bytes(signed_block: object) -> bytes:
    signature = getattr(signed_block, "signature", None)
    proposer_signature = getattr(signature, "proposer_signature", None)
    if proposer_signature is None:
        return _encode_blank_xmss_signature()

    try:
        encoded = encode_ssz_bytes(proposer_signature)
    except Exception:
        logger.debug("Unable to encode proposer signature", exc_info=True)
        return _encode_blank_xmss_signature()

    if len(encoded) != LEGACY_SIGNATURE_LENGTH:
        logger.debug("Unexpected proposer signature length %d", len(encoded))
        return _encode_blank_xmss_signature()

    return encoded


def extract_attestation_signature_proofs(signed_block: object) -> list[bytes]:
    signature = getattr(signed_block, "signature", None)
    attestation_signatures = getattr(signature, "attestation_signatures", None)
    proof_values = getattr(attestation_signatures, "data", attestation_signatures)
    if proof_values is None:
        return []

    proofs = []
    for proof in proof_values:
        try:
            participants = encode_ssz_bytes(getattr(proof, "participants"))
            proof_data = encode_ssz_bytes(getattr(proof, "proof_data"))
            proofs.append(_encode_aggregated_signature_proof(participants, proof_data))
        except Exception:
            logger.debug("Unable to encode attestation signature proof", exc_info=True)
            proofs.append(_encode_aggregated_signature_proof())

    return proofs


def _encode_legacy_block_signatures(
    proposer_signature: bytes,
    attestation_signature_proofs: bytes,
) -> bytes:
    # Checkpoint clients validate that this list has one placeholder proof per
    # attestation, but they do not need the proofs to be valid for bootstrapping.
    fixed_part_length = OFFSET_BYTE_LENGTH + LEGACY_SIGNATURE_LENGTH
    return (
        _ssz_offset(fixed_part_length)
        + proposer_signature
        + attestation_signature_proofs
    )


def _encode_legacy_signed_block(
    block_bytes: bytes,
    proposer_signature: bytes,
    attestation_signature_proofs: bytes,
) -> bytes:
    signatures_bytes = _encode_legacy_block_signatures(
        proposer_signature,
        attestation_signature_proofs,
    )
    fixed_part_length = OFFSET_BYTE_LENGTH * 2
    return (
        _ssz_offset(fixed_part_length)
        + _ssz_offset(fixed_part_length + len(block_bytes))
        + block_bytes
        + signatures_bytes
    )


def encode_legacy_signed_block(signed_block: object) -> bytes:
    block = extract_inner_block(signed_block)
    proofs = extract_attestation_signature_proofs(signed_block)
    if not proofs:
        proof_bytes = _encode_placeholder_aggregated_signature_proofs(
            legacy_attestation_count(block)
        )
    else:
        proof_bytes = _encode_aggregated_signature_proofs(proofs)
    return _encode_legacy_signed_block(
        block.encode_bytes(),
        extract_proposer_signature_bytes(signed_block),
        proof_bytes,
    )

def install_legacy_signed_block_wire_compatibility() -> None:
    global _LEGACY_SIGNED_BLOCK_WIRE_COMPAT_INSTALLED

    if _LEGACY_SIGNED_BLOCK_WIRE_COMPAT_INSTALLED:
        return

    if not legacy_signed_block_compatibility_enabled():
        _LEGACY_SIGNED_BLOCK_WIRE_COMPAT_INSTALLED = True
        return

    networking_service_module = importlib.import_module(
        "lean_spec.subspecs.networking.service.service"
    )
    reqresp_handler_module = importlib.import_module(
        "lean_spec.subspecs.networking.reqresp.handler"
    )

    network_service_cls = networking_service_module.NetworkService
    request_handler_cls = reqresp_handler_module.RequestHandler
    response_code_cls = reqresp_handler_module.ResponseCode
    slot_cls = reqresp_handler_module.Slot
    uint64_cls = reqresp_handler_module.Uint64

    async def publish_block_with_legacy_signed_block(self: object, block: object) -> None:
        topic = networking_service_module.GossipTopic.block(self.network_name)
        compressed = networking_service_module.compress(encode_legacy_signed_block(block))
        await self.event_source.publish(topic.to_topic_id(), compressed)
        logger.debug("Published legacy signed block at slot %s", extract_inner_block(block).slot)

    async def handle_blocks_by_root_with_legacy_signed_block(
        self: object,
        request: object,
        response: object,
    ) -> None:
        if self.block_lookup is None:
            logger.warning("BlocksByRoot request received but no block_lookup configured")
            await response.send_error(response_code_cls.SERVER_ERROR, "Block lookup not available")
            return

        for root in request.roots.data:
            try:
                block = await self.block_lookup(root)
                if block is not None:
                    await response.send_success(encode_legacy_signed_block(block))
            except Exception as err:
                logger.warning("Error looking up block %s: %s", root.hex()[:8], err)

    async def handle_blocks_by_range_with_legacy_signed_block(
        self: object,
        request: object,
        response: object,
    ) -> None:
        if self.block_by_slot_lookup is None:
            logger.warning("BlocksByRange request received but no block_by_slot_lookup configured")
            await response.send_error(response_code_cls.SERVER_ERROR, "Block lookup not available")
            return

        if request.count == uint64_cls(0) or request.count > uint64_cls(
            reqresp_handler_module.MAX_REQUEST_BLOCKS
        ):
            await response.send_error(response_code_cls.INVALID_REQUEST, "Invalid count")
            return

        if self.current_slot_lookup is None:
            logger.warning("BlocksByRange request received but no current_slot_lookup configured")
            await response.send_error(response_code_cls.SERVER_ERROR, "Current slot not available")
            return

        current_slot = self.current_slot_lookup()
        window_floor = (
            current_slot - slot_cls(reqresp_handler_module.MIN_SLOTS_FOR_BLOCK_REQUESTS)
            if current_slot >= slot_cls(reqresp_handler_module.MIN_SLOTS_FOR_BLOCK_REQUESTS)
            else slot_cls(0)
        )
        if request.start_slot < window_floor:
            await response.send_error(
                response_code_cls.RESOURCE_UNAVAILABLE,
                "Requested slot predates history window",
            )
            return

        for index in range(int(request.count)):
            slot = request.start_slot + slot_cls(index)
            try:
                block = await self.block_by_slot_lookup(slot)
                if block is not None:
                    await response.send_success(encode_legacy_signed_block(block))
            except Exception as err:
                logger.warning("Error looking up block at slot %s: %s", slot, err)

    network_service_cls.publish_block = publish_block_with_legacy_signed_block
    request_handler_cls.handle_blocks_by_root = handle_blocks_by_root_with_legacy_signed_block
    request_handler_cls.handle_blocks_by_range = handle_blocks_by_range_with_legacy_signed_block
    logger.info("Installed LeanSpec signed-block gossip/reqresp compatibility")
    _LEGACY_SIGNED_BLOCK_WIRE_COMPAT_INSTALLED = True


def install_quic_stream_close_reset_tolerance() -> None:
    global _QUIC_STREAM_CLOSE_COMPAT_INSTALLED

    if _QUIC_STREAM_CLOSE_COMPAT_INSTALLED:
        return

    original_close = QuicStream.close

    async def close_ignoring_reset(self: QuicStream) -> None:
        try:
            await original_close(self)
        except Exception as err:
            # grandine_lean opens an identify stream that LeanSpec does not
            # implement. The peer may reset that stream before LeanSpec can
            # close it, and aioquic then raises from close(). Swallow the
            # close error so the helper keeps accepting subsequent req/resp
            # streams on the same connection.
            logger.debug(
                "Ignoring QUIC close failure for stream %d after peer reset: %s",
                self.stream_id,
                err,
            )

    QuicStream.close = close_ignoring_reset
    _QUIC_STREAM_CLOSE_COMPAT_INSTALLED = True


def install_fast_ssz_deserialization() -> None:
    global _SSZ_FAST_DESERIALIZATION_INSTALLED

    if _SSZ_FAST_DESERIALIZATION_INSTALLED:
        return

    @classmethod
    def vector_deserialize_without_validation(
        cls: type[SSZVector], stream: io.BytesIO, scope: int
    ) -> SSZVector:
        elements = []
        if cls.is_fixed_size():
            elem_byte_length = cls.get_byte_length() // cls.LENGTH
            if scope != cls.get_byte_length():
                raise SSZSerializationError(
                    f"{cls.__name__}: expected {cls.get_byte_length()} bytes, got {scope}"
                )
            elements = [
                cls.ELEMENT_TYPE.deserialize(stream, elem_byte_length)
                for _ in range(cls.LENGTH)
            ]
        else:
            first_offset = int(Uint32.deserialize(stream, OFFSET_BYTE_LENGTH))
            expected = cls.LENGTH * OFFSET_BYTE_LENGTH
            if first_offset != expected:
                raise SSZSerializationError(
                    f"{cls.__name__}: invalid offset {first_offset}, expected {expected}"
                )
            offsets = [first_offset] + [
                int(Uint32.deserialize(stream, OFFSET_BYTE_LENGTH))
                for _ in range(cls.LENGTH - 1)
            ]
            offsets.append(scope)
            _validate_offsets(offsets, scope, cls.__name__)
            for i in range(cls.LENGTH):
                start, end = offsets[i], offsets[i + 1]
                elements.append(cls.ELEMENT_TYPE.deserialize(stream, end - start))

        return cls.model_construct(data=tuple(elements))

    @classmethod
    def list_deserialize_without_validation(
        cls: type[SSZList], stream: io.BytesIO, scope: int
    ) -> SSZList:
        if cls.ELEMENT_TYPE.is_fixed_size():
            element_size = cls.ELEMENT_TYPE.get_byte_length()
            if scope % element_size != 0:
                raise SSZSerializationError(
                    f"{cls.__name__}: scope {scope} not divisible by element size {element_size}"
                )

            num_elements = scope // element_size
            if num_elements > cls.LIMIT:
                raise SSZValueError(
                    f"{cls.__name__} exceeds limit of {cls.LIMIT}, got {num_elements}"
                )

            elements = [
                cls.ELEMENT_TYPE.deserialize(stream, element_size)
                for _ in range(num_elements)
            ]
            return cls.model_construct(data=tuple(elements))

        if scope == 0:
            return cls.model_construct(data=tuple())
        if scope < OFFSET_BYTE_LENGTH:
            raise SSZSerializationError(
                f"{cls.__name__}: scope {scope} too small for variable-size list"
            )

        first_offset = int(Uint32.deserialize(stream, OFFSET_BYTE_LENGTH))
        if first_offset > scope or first_offset % OFFSET_BYTE_LENGTH != 0:
            raise SSZSerializationError(f"{cls.__name__}: invalid offset {first_offset}")

        count = first_offset // OFFSET_BYTE_LENGTH
        if count > cls.LIMIT:
            raise SSZValueError(f"{cls.__name__} exceeds limit of {cls.LIMIT}, got {count}")

        offsets = [first_offset] + [
            int(Uint32.deserialize(stream, OFFSET_BYTE_LENGTH)) for _ in range(count - 1)
        ]
        offsets.append(scope)
        _validate_offsets(offsets, scope, cls.__name__)

        elements = []
        for i in range(count):
            start, end = offsets[i], offsets[i + 1]
            elements.append(cls.ELEMENT_TYPE.deserialize(stream, end - start))

        return cls.model_construct(data=tuple(elements))

    @classmethod
    def container_deserialize_without_validation(
        cls: type[Container], stream: io.BytesIO, scope: int
    ) -> Container:
        fields: dict[str, object] = {}
        var_fields = []
        bytes_read = 0

        for field_name, field_info in cls.model_fields.items():
            field_type = cls._get_ssz_field_type(field_info.annotation)

            if field_type.is_fixed_size():
                size = field_type.get_byte_length()
                data = stream.read(size)
                if len(data) != size:
                    raise SSZSerializationError(
                        f"{cls.__name__}.{field_name}: expected {size} bytes, got {len(data)}"
                    )
                fields[field_name] = field_type.decode_bytes(data)
                bytes_read += size
            else:
                offset_bytes = stream.read(OFFSET_BYTE_LENGTH)
                if len(offset_bytes) != OFFSET_BYTE_LENGTH:
                    raise SSZSerializationError(
                        f"{cls.__name__}.{field_name}: "
                        f"expected {OFFSET_BYTE_LENGTH} offset bytes, got {len(offset_bytes)}"
                    )
                offset = int(Uint32.decode_bytes(offset_bytes))
                var_fields.append((field_name, field_type, offset))
                bytes_read += OFFSET_BYTE_LENGTH

        if var_fields:
            var_section_size = scope - bytes_read
            var_section = stream.read(var_section_size)
            if len(var_section) != var_section_size:
                raise SSZSerializationError(
                    f"{cls.__name__}: "
                    f"expected {var_section_size} variable bytes, got {len(var_section)}"
                )

            offsets = [offset for _, _, offset in var_fields] + [scope]
            for i, (name, field_type, start) in enumerate(var_fields):
                end = offsets[i + 1]
                rel_start = start - bytes_read
                rel_end = end - bytes_read
                if rel_start < 0 or rel_start > rel_end:
                    raise SSZSerializationError(
                        f"{cls.__name__}.{name}: invalid offsets start={start}, end={end}"
                    )
                fields[name] = field_type.decode_bytes(var_section[rel_start:rel_end])

        return cls.model_construct(**fields)

    SSZVector.deserialize = vector_deserialize_without_validation
    SSZList.deserialize = list_deserialize_without_validation
    Container.deserialize = container_deserialize_without_validation
    _SSZ_FAST_DESERIALIZATION_INSTALLED = True


def install_snappy_compress_fallback() -> None:
    global _SNAPPY_COMPRESS_FALLBACK_INSTALLED

    if _SNAPPY_COMPRESS_FALLBACK_INSTALLED:
        return

    snappy_package_module = importlib.import_module("lean_spec.snappy")
    snappy_compress_module = importlib.import_module("lean_spec.snappy.compress")
    snappy_framing_module = importlib.import_module("lean_spec.snappy.framing")
    reqresp_codec_module = importlib.import_module("lean_spec.subspecs.networking.reqresp.codec")
    networking_service_module = importlib.import_module(
        "lean_spec.subspecs.networking.service.service"
    )

    def literal_only_compress(data: bytes) -> bytes:
        if not data:
            return snappy_compress_module.encode_varint32(0)

        output = bytearray(snappy_compress_module.encode_varint32(len(data)))
        offset = 0
        while offset < len(data):
            block_end = min(offset + snappy_compress_module.BLOCK_SIZE, len(data))
            output.extend(snappy_compress_module._emit_literal(data[offset:block_end]))
            offset = block_end

        return bytes(output)

    def compress_with_fallback(data: bytes) -> bytes:
        return literal_only_compress(data)

    def frame_compress_with_fallback(data: bytes) -> bytes:
        output = bytearray(snappy_framing_module.STREAM_IDENTIFIER)
        offset = 0
        while offset < len(data):
            chunk_end = min(offset + snappy_framing_module.MAX_UNCOMPRESSED_CHUNK_SIZE, len(data))
            chunk = data[offset:chunk_end]
            offset = chunk_end
            crc = snappy_framing_module._mask_crc(snappy_framing_module._crc32c(chunk))
            output.append(snappy_framing_module.CHUNK_TYPE_UNCOMPRESSED)
            output.extend((4 + len(chunk)).to_bytes(3, "little"))
            output.extend(crc.to_bytes(4, "little"))
            output.extend(chunk)
        return bytes(output)

    snappy_compress_module.compress = compress_with_fallback
    snappy_framing_module.frame_compress = frame_compress_with_fallback
    snappy_package_module.compress = compress_with_fallback
    snappy_package_module.frame_compress = frame_compress_with_fallback
    reqresp_codec_module.frame_compress = frame_compress_with_fallback
    networking_service_module.compress = compress_with_fallback
    _SNAPPY_COMPRESS_FALLBACK_INSTALLED = True


def install_reqresp_block_cache_tracking() -> None:
    global _REQRESP_BLOCK_CACHE_TRACKING_INSTALLED

    if _REQRESP_BLOCK_CACHE_TRACKING_INSTALLED:
        return

    reqresp_client_module = importlib.import_module(
        "lean_spec.subspecs.networking.client.reqresp_client"
    )
    reqresp_client_cls = reqresp_client_module.ReqRespClient
    original_request_blocks_by_root = reqresp_client_cls.request_blocks_by_root

    async def request_blocks_by_root_with_cache(
        self: object,
        peer_id: object,
        roots: list[Bytes32],
    ) -> list[object]:
        signed_blocks = await original_request_blocks_by_root(self, peer_id, roots)
        block_cache = _BLOCK_CACHE_BY_REQRESP_CLIENT.get(id(self))
        if block_cache is not None:
            for signed_block in signed_blocks:
                cache_signed_block(block_cache, signed_block)
        return signed_blocks

    reqresp_client_cls.request_blocks_by_root = request_blocks_by_root_with_cache
    _REQRESP_BLOCK_CACHE_TRACKING_INSTALLED = True


def install_network_service_gossip_tracking() -> None:
    global _NETWORK_SERVICE_GOSSIP_TRACKING_INSTALLED

    if _NETWORK_SERVICE_GOSSIP_TRACKING_INSTALLED:
        return

    networking_service_module = importlib.import_module(
        "lean_spec.subspecs.networking.service.service"
    )
    network_service_cls = networking_service_module.NetworkService
    original_handle_event = network_service_cls._handle_event

    async def handle_event_with_helper_backfill(self: object, event: object) -> None:
        if isinstance(event, GossipBlockEvent):
            block_cache = _BLOCK_CACHE_BY_SYNC_SERVICE.get(id(self.sync_service))
            if block_cache is not None:
                cache_signed_block(block_cache, event.block)

        await original_handle_event(self, event)

        if not isinstance(event, GossipBlockEvent):
            return

        event_context = _SYNC_EVENT_CONTEXT.get(id(self.sync_service))
        if event_context is None:
            return

        event_source, node = event_context
        process_pending_blocks = getattr(self.sync_service, "process_pending_blocks", None)
        if process_pending_blocks is not None:
            processed_count = await process_pending_blocks()
            if processed_count > 0:
                logger.info(
                    "Processed %d cached backfill blocks after slot=%s gossip",
                    processed_count,
                    extract_inner_block(event.block).slot,
                )

        refresh_status(event_source, node)

    network_service_cls._handle_event = handle_event_with_helper_backfill
    _NETWORK_SERVICE_GOSSIP_TRACKING_INSTALLED = True


def encode_unsigned_varint(value: int) -> bytes:
    output = bytearray()
    while value >= 0x80:
        output.append((value & 0x7F) | 0x80)
        value >>= 7
    output.append(value)
    return bytes(output)


def protobuf_key(field_number: int, wire_type: int) -> bytes:
    return encode_unsigned_varint((field_number << 3) | wire_type)


def protobuf_varint_field(field_number: int, value: int) -> bytes:
    return protobuf_key(field_number, 0) + encode_unsigned_varint(value)


def protobuf_bytes_field(field_number: int, value: bytes) -> bytes:
    return protobuf_key(field_number, 2) + encode_unsigned_varint(len(value)) + value


def protobuf_string_field(field_number: int, value: str) -> bytes:
    return protobuf_bytes_field(field_number, value.encode("utf-8"))


def build_libp2p_public_key(public_key: object) -> bytes:
    raw_public_key = public_key.to_bytes()
    return (
        protobuf_varint_field(1, LIBP2P_SECP256K1_PUBLIC_KEY_TYPE)
        + protobuf_bytes_field(2, raw_public_key)
    )


def build_identify_response(identity_key: IdentityKeypair) -> bytes:
    public_key = build_libp2p_public_key(identity_key.public_key)
    protocols = (
        IDENTIFY_PROTOCOL_ID,
        IDENTIFY_PUSH_PROTOCOL_ID,
        "/leanconsensus/req/status/1/ssz_snappy",
        "/leanconsensus/req/blocks_by_root/1/ssz_snappy",
        "/leanconsensus/req/blocks_by_range/1/ssz_snappy",
        "/meshsub/1.2.0",
        GOSSIPSUB_V11_PROTOCOL_ID,
    )

    response = bytearray()
    response.extend(protobuf_bytes_field(1, public_key))
    for protocol in protocols:
        response.extend(protobuf_string_field(3, protocol))
    response.extend(protobuf_string_field(5, "ipfs/0.1.0"))
    response.extend(protobuf_string_field(6, "hive-lean-spec-helper/0.1.0"))
    return bytes(response)


async def read_delimited_identify_message(wrapper: object) -> bytes:
    length = 0
    shift = 0
    for _ in range(10):
        chunk = await wrapper.read(1)
        if not chunk:
            raise EOFError("identify stream closed before length prefix")
        byte = chunk[0]
        length |= (byte & 0x7F) << shift
        if byte & 0x80 == 0:
            break
        shift += 7
    else:
        raise ValueError("identify length prefix is too long")

    if length == 0:
        return b""
    if length > 1_048_576:
        raise ValueError(f"identify message too large: {length} bytes")
    return await wrapper.readexactly(length)


async def handle_identify_stream(
    event_source: object,
    peer_id: object,
    protocol_id: str,
    wrapper: object,
) -> None:
    if protocol_id == IDENTIFY_PROTOCOL_ID:
        response = getattr(event_source, "_hive_identify_response", b"")
        wrapper.write(encode_unsigned_varint(len(response)) + response)
        await wrapper.drain()
        logger.info("Served identify response to %s", peer_id)
    elif protocol_id == IDENTIFY_PUSH_PROTOCOL_ID:
        try:
            await asyncio.wait_for(read_delimited_identify_message(wrapper), timeout=2.0)
        except (asyncio.TimeoutError, EOFError):
            logger.debug("Identify push stream from %s closed without a full payload", peer_id)
        except Exception as err:
            logger.debug("Ignoring invalid identify push payload from %s: %s", peer_id, err)
        else:
            logger.info("Accepted identify push from %s", peer_id)

    with suppress(Exception):
        await wrapper.finish_write()


def install_identify_protocol_compatibility() -> None:
    global _IDENTIFY_PROTOCOL_COMPAT_INSTALLED

    if _IDENTIFY_PROTOCOL_COMPAT_INSTALLED:
        return

    live_module = importlib.import_module(
        "lean_spec.subspecs.networking.client.event_source.live"
    )
    protocol_module = importlib.import_module(
        "lean_spec.subspecs.networking.client.event_source.protocol"
    )

    extra_protocols = {
        GOSSIPSUB_V11_PROTOCOL_ID,
        IDENTIFY_PROTOCOL_ID,
        IDENTIFY_PUSH_PROTOCOL_ID,
    }
    live_module.SUPPORTED_PROTOCOLS = frozenset(
        set(live_module.SUPPORTED_PROTOCOLS) | extra_protocols
    )
    protocol_module.SUPPORTED_PROTOCOLS = frozenset(
        set(protocol_module.SUPPORTED_PROTOCOLS) | extra_protocols
    )

    event_source_cls = live_module.LiveNetworkEventSource

    async def accept_streams_with_identify(
        self: object,
        peer_id: object,
        conn: object,
    ) -> None:
        def is_running() -> bool:
            stop_event = getattr(self, "_stop_event", None)
            if stop_event is not None:
                return not stop_event.is_set()
            return bool(getattr(self, "_running", False))

        try:
            logger.info("Stream acceptor started for peer %s", peer_id)
            while is_running() and peer_id in self._connections:
                try:
                    stream = await conn.accept_stream()
                except Exception as err:
                    logger.debug("Stream accept failed for %s: %s", peer_id, err)
                    break

                negotiated = await self._negotiate_inbound_stream(peer_id, stream)
                if negotiated is None:
                    continue
                protocol_id, wrapper = negotiated

                if protocol_id in (
                    live_module.GOSSIPSUB_DEFAULT_PROTOCOL_ID,
                    live_module.GOSSIPSUB_PROTOCOL_ID_V12,
                    GOSSIPSUB_V11_PROTOCOL_ID,
                ):
                    await self._handle_gossipsub_inbound_stream(
                        peer_id,
                        conn,
                        protocol_id,
                        wrapper,
                    )
                elif protocol_id in live_module.REQRESP_PROTOCOL_IDS:
                    self._handle_reqresp_inbound_stream(peer_id, protocol_id, wrapper)
                elif protocol_id in (IDENTIFY_PROTOCOL_ID, IDENTIFY_PUSH_PROTOCOL_ID):
                    await handle_identify_stream(self, peer_id, protocol_id, wrapper)
                else:
                    logger.debug(
                        "Unknown protocol %s from %s, closing stream", protocol_id, peer_id
                    )
                    await stream.close()
        except asyncio.CancelledError:
            logger.debug("Stream acceptor cancelled for %s", peer_id)
        except Exception as err:
            logger.warning("Stream acceptor error for %s: %s", peer_id, err)

    event_source_cls._accept_streams = accept_streams_with_identify
    _IDENTIFY_PROTOCOL_COMPAT_INSTALLED = True


def install_child_only_aggregation_compatibility() -> None:
    global _CHILD_ONLY_AGGREGATION_COMPAT_INSTALLED

    if _CHILD_ONLY_AGGREGATION_COMPAT_INSTALLED:
        return

    original_to_aggregation_bits = ValidatorIndices.to_aggregation_bits
    original_aggregate = TypeOneSignatureProof.aggregate
    aggregate_parameters = inspect.signature(original_aggregate).parameters

    def to_aggregation_bits_allowing_empty(
        self: ValidatorIndices,
    ) -> AggregationBits:
        if not self.data:
            return AggregationBits(data=[])
        return original_to_aggregation_bits(self)

    ValidatorIndices.to_aggregation_bits = to_aggregation_bits_allowing_empty

    if {"children", "raw_xmss", "xmss_participants"}.issubset(aggregate_parameters):

        def aggregate_allowing_child_only_merge(
            children: object,
            raw_xmss: object,
            xmss_participants: AggregationBits | None,
            message: Bytes32,
            slot: object,
            mode: object | None = None,
        ) -> Any:
            if (
                not raw_xmss
                and xmss_participants is not None
                and not any(bool(bit) for bit in xmss_participants.data)
            ):
                xmss_participants = None

            return original_aggregate(
                children=children,
                raw_xmss=raw_xmss,
                xmss_participants=xmss_participants,
                message=message,
                slot=slot,
                mode=mode,
            )

        TypeOneSignatureProof.aggregate = staticmethod(aggregate_allowing_child_only_merge)

    _CHILD_ONLY_AGGREGATION_COMPAT_INSTALLED = True


def install_setup_prover_compatibility() -> None:
    global _SETUP_PROVER_COMPAT_INSTALLED

    if _SETUP_PROVER_COMPAT_INSTALLED:
        return

    original_setup_prover = getattr(xmss_aggregation_module, "setup_prover", None)
    original_setup_verifier = getattr(xmss_aggregation_module, "setup_verifier", None)
    if original_setup_prover is None and original_setup_verifier is None:
        _SETUP_PROVER_COMPAT_INSTALLED = True
        return

    initialized_prover_modes: set[str] = set()
    initialized_verifier_modes: set[str] = set()
    default_mode = getattr(xmss_aggregation_module, "LEAN_ENV", os.environ.get("LEAN_ENV"))

    def setup_once(
        original_setup: object | None,
        initialized_modes: set[str],
        mode: object | None,
    ) -> None:
        if original_setup is None:
            return
        key = str(mode)
        if key in initialized_modes:
            return
        if mode is None:
            original_setup()
        else:
            original_setup(mode=mode)
        initialized_modes.add(key)

    def setup_prover_once(*, mode: object | None = None) -> None:
        setup_once(original_setup_prover, initialized_prover_modes, mode or default_mode)

    def setup_verifier_once(*, mode: object | None = None) -> None:
        setup_once(original_setup_verifier, initialized_verifier_modes, mode or default_mode)

    if original_setup_prover is not None:
        xmss_aggregation_module.setup_prover = setup_prover_once
    if original_setup_verifier is not None:
        xmss_aggregation_module.setup_verifier = setup_verifier_once
    _SETUP_PROVER_COMPAT_INSTALLED = True


def install_trusted_gossip_attestation_compatibility() -> None:
    global _TRUSTED_GOSSIP_ATTESTATION_COMPAT_INSTALLED

    if _TRUSTED_GOSSIP_ATTESTATION_COMPAT_INSTALLED:
        return

    try:
        from lean_spec.forks.lstar.spec import LstarSpec
        from lean_spec.forks.lstar.store import AttestationSignatureEntry
    except ImportError as err:
        logger.debug("LeanSpec trusted attestation compatibility unavailable: %s", err)
        return

    def on_gossip_attestation_without_signature_verification(
        self: Any,
        store: Any,
        signed_attestation: Any,
        is_aggregator: bool = False,
    ) -> Any:
        validator_id = signed_attestation.validator_id
        attestation_data = signed_attestation.data
        signature = signed_attestation.signature

        self.validate_attestation(store, attestation_data)

        key_state = store.states.get(attestation_data.target.root)
        assert key_state is not None, (
            f"No state available to validate attestation for target block "
            f"{attestation_data.target.root.hex()}"
        )
        assert validator_id.is_valid(Uint64(len(key_state.validators))), (
            f"Validator {validator_id} not found in state {attestation_data.target.root.hex()}"
        )

        if not is_aggregator:
            return store

        new_committee_sigs = {k: set(v) for k, v in store.attestation_signatures.items()}
        new_committee_sigs.setdefault(attestation_data, set()).add(
            AttestationSignatureEntry(validator_id, signature)
        )
        return store.model_copy(update={"attestation_signatures": new_committee_sigs})

    LstarSpec.on_gossip_attestation = on_gossip_attestation_without_signature_verification
    _TRUSTED_GOSSIP_ATTESTATION_COMPAT_INSTALLED = True
    logger.info("Installed LeanSpec trusted gossip attestation compatibility")


def install_checkpoint_finalization_compatibility() -> None:
    global _CHECKPOINT_FINALIZATION_COMPAT_INSTALLED

    if _CHECKPOINT_FINALIZATION_COMPAT_INSTALLED or not uses_latest_leanspec_format():
        return

    try:
        from lean_spec.forks.lstar.spec import LstarSpec
    except ImportError as err:
        logger.debug("LeanSpec checkpoint finalization compatibility unavailable: %s", err)
        return

    try:
        from lean_spec.subspecs.validator.service import ValidatorService
    except ImportError:
        ValidatorService = None

    original_on_block = LstarSpec.on_block

    def checkpoint_from_head(store: Any) -> Any | None:
        head_block = store.blocks.get(store.head)
        if head_block is None:
            return None

        return Checkpoint(root=store.head, slot=head_block.slot)

    def advance_checkpoints_to_head(store: Any) -> Any:
        head_checkpoint = checkpoint_from_head(store)
        if head_checkpoint is None:
            return store

        updates = {}
        if head_checkpoint.slot > store.latest_justified.slot:
            updates["latest_justified"] = head_checkpoint
        if head_checkpoint.slot > store.latest_finalized.slot:
            updates["latest_finalized"] = head_checkpoint

        if updates:
            return store.model_copy(update=updates)
        return store

    def pin_head_to_block(store: Any, block: Any) -> Any:
        block_root = hash_tree_root(block)
        if block_root not in store.blocks:
            return store

        return store.model_copy(
            update={
                "head": block_root,
                "safe_target": block_root,
            }
        )

    def aggregate_without_local_multisig_prover(self: Any, store: Any) -> Any:
        return store.model_copy(
            update={
                "attestation_signatures": {},
                "latest_new_aggregated_payloads": {},
            }
        ), []

    def on_gossip_aggregated_attestation_without_verification(
        self: Any,
        store: Any,
        signed_attestation: Any,
    ) -> Any:
        data = signed_attestation.data
        proof = signed_attestation.proof

        self.validate_attestation(store, data)

        new_aggregated_payloads = {
            k: set(v) for k, v in store.latest_new_aggregated_payloads.items()
        }
        new_aggregated_payloads.setdefault(data, set()).add(proof)
        return store.model_copy(
            update={"latest_new_aggregated_payloads": new_aggregated_payloads}
        )

    def on_block_with_justified_checkpoint_finalized(
        self: Any,
        store: Any,
        signed_block: Any,
    ) -> Any:
        updated_store = original_on_block(self, store, signed_block)
        return advance_checkpoints_to_head(
            pin_head_to_block(updated_store, extract_inner_block(signed_block))
        )

    def produce_block_with_justified_checkpoint_finalized(
        self: Any,
        store: Any,
        slot: Any = None,
        validator_index: Any = None,
        **kwargs: Any,
    ) -> Any:
        if kwargs:
            slot = kwargs.get("slot", slot)
            validator_index = kwargs.get("validator_index", validator_index)
        if slot is None or validator_index is None:
            raise TypeError("slot and validator_index are required")

        parent_root = store.head
        head_state = store.states[parent_root]
        block = self.block_class(
            slot=slot,
            proposer_index=validator_index,
            parent_root=parent_root,
            state_root=Bytes32.zero(),
            body=self.block_body_class(
                attestations=self.aggregated_attestations_class(data=[]),
            ),
        )
        post_state = self.process_block(self.process_slots(head_state, slot), block)
        block = block.model_copy(update={"state_root": hash_tree_root(post_state)})
        block_root = hash_tree_root(block)
        updated_store = store.model_copy(
            update={
                "blocks": store.blocks | {block_root: block},
                "states": store.states | {block_root: post_state},
                "head": block_root,
                "safe_target": block_root,
            }
        )
        return advance_checkpoints_to_head(updated_store), block, []

    async def skip_local_attestation_signing(self: Any, slot: Any) -> None:
        logger.debug(
            "Skipping helper-local attestation signing at slot %s; "
            "checkpoint compatibility finalizes helper-produced blocks directly",
            slot,
        )

    LstarSpec.aggregate = aggregate_without_local_multisig_prover
    LstarSpec.on_gossip_aggregated_attestation = (
        on_gossip_aggregated_attestation_without_verification
    )
    LstarSpec.on_block = on_block_with_justified_checkpoint_finalized
    LstarSpec.produce_block_with_signatures = produce_block_with_justified_checkpoint_finalized
    if ValidatorService is not None:
        ValidatorService._produce_attestations = skip_local_attestation_signing
    _CHECKPOINT_FINALIZATION_COMPAT_INSTALLED = True
    logger.info("Installed LeanSpec checkpoint finalization compatibility")


def parse_bootnodes() -> list[str]:
    raw_bootnodes = os.environ.get("HIVE_BOOTNODES", "")
    if not raw_bootnodes:
        return []

    return [bootnode for bootnode in raw_bootnodes.split(",") if bootnode]


def parse_validator_indices() -> list[int]:
    raw_indices = os.environ.get("HIVE_LEAN_VALIDATOR_INDICES", "")
    if not raw_indices:
        return []

    return [int(index) for index in raw_indices.split(",") if index]


def genesis_validator_count() -> int:
    raw_count = os.environ.get(GENESIS_VALIDATOR_COUNT_ENVIRONMENT_VARIABLE, "")
    if not raw_count:
        return DEFAULT_VALIDATOR_COUNT

    count = int(raw_count)
    if count < 0:
        raise ValueError(f"{GENESIS_VALIDATOR_COUNT_ENVIRONMENT_VARIABLE} must be non-negative")
    return count


def devnet_label() -> str:
    return os.environ.get("HIVE_LEAN_DEVNET_LABEL", "devnet3")


def helper_gossip_fork_digest() -> str:
    return os.environ.get(
        HELPER_GOSSIP_FORK_DIGEST_ENVIRONMENT_VARIABLE,
        DEFAULT_GOSSIP_FORK_DIGEST,
    )


def helper_p2p_port() -> int:
    return int(os.environ.get(HELPER_P2P_PORT_ENVIRONMENT_VARIABLE, str(DEFAULT_LISTEN_PORT)))


def helper_api_port() -> int:
    return int(os.environ.get(HELPER_API_PORT_ENVIRONMENT_VARIABLE, str(DEFAULT_API_PORT)))


def helper_metadata_port() -> int:
    return int(
        os.environ.get(
            HELPER_METADATA_PORT_ENVIRONMENT_VARIABLE,
            str(DEFAULT_HELPER_METADATA_PORT),
        )
    )


def listen_addr() -> str:
    return f"/ip4/0.0.0.0/udp/{helper_p2p_port()}/quic-v1"


def uses_latest_leanspec_format() -> bool:
    return devnet_label() == "devnet4"


def wire_compat_mode() -> str:
    mode = os.environ.get(WIRE_COMPAT_ENVIRONMENT_VARIABLE, WIRE_COMPAT_NATIVE).strip().lower()
    if mode not in {WIRE_COMPAT_LEGACY, WIRE_COMPAT_NATIVE}:
        logger.warning(
            "Unknown %s=%r; falling back to %s",
            WIRE_COMPAT_ENVIRONMENT_VARIABLE,
            mode,
            WIRE_COMPAT_NATIVE,
        )
        return WIRE_COMPAT_NATIVE
    return mode


def legacy_signed_block_compatibility_enabled() -> bool:
    return uses_latest_leanspec_format() and wire_compat_mode() == WIRE_COMPAT_LEGACY


def load_validator(index: int) -> dict[str, str]:
    with (source_keys_dir() / f"{index}.json").open(encoding="utf-8") as key_file:
        validator = json.load(key_file)

    if {"attestation_keypair", "proposal_keypair"}.issubset(validator):
        return {
            ATTESTATION_PUBKEY_FIELD: validator["attestation_keypair"]["public_key"],
            ATTESTATION_SECRET_FIELD: validator["attestation_keypair"]["secret_key"],
            PROPOSAL_PUBKEY_FIELD: validator["proposal_keypair"]["public_key"],
            PROPOSAL_SECRET_FIELD: validator["proposal_keypair"]["secret_key"],
        }

    if {
        ATTESTATION_PUBKEY_FIELD,
        ATTESTATION_SECRET_FIELD,
        PROPOSAL_PUBKEY_FIELD,
        PROPOSAL_SECRET_FIELD,
    }.issubset(validator):
        return validator

    if {LEGACY_PUBLIC_FIELD, LEGACY_SECRET_FIELD}.issubset(validator):
        # Latest published devnet4 helper bundles currently expose a single
        # XMSS keypair per validator. LeanSpec still expects separate
        # attestation/proposal entries, so we mirror the same key material
        # into both roles for test-only helper startup compatibility.
        return {
            ATTESTATION_PUBKEY_FIELD: validator[LEGACY_PUBLIC_FIELD],
            ATTESTATION_SECRET_FIELD: validator[LEGACY_SECRET_FIELD],
            PROPOSAL_PUBKEY_FIELD: validator[LEGACY_PUBLIC_FIELD],
            PROPOSAL_SECRET_FIELD: validator[LEGACY_SECRET_FIELD],
        }

    raise ValueError(
        f"Unsupported validator key format for validator {index}: "
        f"expected nested keypairs, split attestation/proposal fields, or "
        f"{LEGACY_PUBLIC_FIELD!r}/{LEGACY_SECRET_FIELD!r}"
    )


def decode_secret_key(secret_key_hex: str) -> SecretKey:
    # Current devnet4 prod keys are large enough that LeanSpec's pydantic-heavy
    # decode path can trip GC while nested objects are still being built.
    was_gc_enabled = gc.isenabled()
    if was_gc_enabled:
        gc.disable()
    try:
        return SecretKey.decode_bytes(bytes.fromhex(secret_key_hex))
    finally:
        if was_gc_enabled:
            gc.enable()


def source_keys_dir() -> Path:
    if uses_latest_leanspec_format():
        return DEVNET4_SOURCE_KEYS_DIR

    return DEVNET3_SOURCE_KEYS_DIR


def prepare_output_dirs() -> None:
    if PREPARED_ASSETS_DIR.exists():
        shutil.rmtree(PREPARED_ASSETS_DIR)

    GENESIS_DIR.mkdir(parents=True, exist_ok=True)
    MANIFEST_DIR.mkdir(parents=True, exist_ok=True)


def write_genesis(validators: list[dict[str, str]], genesis_time: int) -> None:
    genesis = {
        "GENESIS_TIME": genesis_time,
        "ATTESTATION_COMMITTEE_COUNT": 1,
        "MAX_ATTESTATIONS_DATA": 16,
        "NUM_VALIDATORS": len(validators),
        "VALIDATOR_COUNT": len(validators),
        "ACTIVE_EPOCH": 18,
    }

    if uses_latest_leanspec_format():
        genesis["GENESIS_VALIDATORS"] = [
            {
                "attestation_pubkey": f"0x{validator[ATTESTATION_PUBKEY_FIELD]}",
                "proposal_pubkey": f"0x{validator[PROPOSAL_PUBKEY_FIELD]}",
            }
            for validator in validators
        ]
    else:
        genesis["GENESIS_VALIDATORS"] = [
            f"0x{validator[ATTESTATION_PUBKEY_FIELD]}" for validator in validators
        ]

    with GENESIS_CONFIG_PATH.open("w", encoding="utf-8") as genesis_file:
        yaml.safe_dump(genesis, genesis_file, sort_keys=False)


def write_validator_keys(
    validators: list[dict[str, str]], node_id: str, validator_indices: list[int]
) -> None:
    with VALIDATORS_YAML_PATH.open("w", encoding="utf-8") as validators_file:
        yaml.safe_dump({node_id: validator_indices}, validators_file, sort_keys=False)

    manifest = {
        "key_scheme": "SIGTopLevelTargetSumLifetime32Dim64Base8",
        "hash_function": "Poseidon2",
        "encoding": "TargetSum",
        "lifetime": 32,
        "log_num_active_epochs": 5,
        "num_active_epochs": 32,
        "num_validators": len(validators),
        "validators": [],
    }

    for index, validator in enumerate(validators):
        if uses_latest_leanspec_format():
            attestation_secret_key_file = f"validator_{index}_att_sk.ssz"
            proposal_secret_key_file = f"validator_{index}_prop_sk.ssz"

            (MANIFEST_DIR / attestation_secret_key_file).write_bytes(
                bytes.fromhex(validator[ATTESTATION_SECRET_FIELD])
            )
            (MANIFEST_DIR / proposal_secret_key_file).write_bytes(
                bytes.fromhex(validator[PROPOSAL_SECRET_FIELD])
            )

            manifest["validators"].append(
                {
                    "index": index,
                    "attestation_pubkey_hex": f"0x{validator[ATTESTATION_PUBKEY_FIELD]}",
                    "proposal_pubkey_hex": f"0x{validator[PROPOSAL_PUBKEY_FIELD]}",
                    "attestation_privkey_file": attestation_secret_key_file,
                    "proposal_privkey_file": proposal_secret_key_file,
                }
            )
        else:
            secret_key_file = f"validator_{index}_sk.ssz"

            (MANIFEST_DIR / secret_key_file).write_bytes(
                bytes.fromhex(validator[ATTESTATION_SECRET_FIELD])
            )

            manifest["validators"].append(
                {
                    "index": index,
                    "pubkey_hex": f"0x{validator[ATTESTATION_PUBKEY_FIELD]}",
                    "privkey_file": secret_key_file,
                }
            )

    with VALIDATOR_MANIFEST_PATH.open("w", encoding="utf-8") as manifest_file:
        yaml.safe_dump(manifest, manifest_file, sort_keys=False)


def prepare_runtime_assets(node_id: str) -> None:
    genesis_time = int(os.environ.get("HIVE_LEAN_GENESIS_TIME", int(time.time()) + 30))
    validator_indices = parse_validator_indices()
    validator_count = genesis_validator_count()
    if validator_indices and max(validator_indices) >= validator_count:
        raise ValueError(
            f"{GENESIS_VALIDATOR_COUNT_ENVIRONMENT_VARIABLE}={validator_count} does not include "
            f"assigned validator index {max(validator_indices)}"
        )
    validators = [load_validator(index) for index in range(validator_count)]

    prepare_output_dirs()
    write_genesis(validators, genesis_time)
    write_validator_keys(validators, node_id, validator_indices)


def helper_genesis_metadata() -> dict[str, object]:
    with GENESIS_CONFIG_PATH.open(encoding="utf-8") as genesis_file:
        genesis_config = yaml.safe_load(genesis_file)

    validators = genesis_config.get("GENESIS_VALIDATORS", [])
    if uses_latest_leanspec_format():
        genesis_validator_entries = [
            {
                "attestation_public_key": validator["attestation_pubkey"],
                "proposal_public_key": validator["proposal_pubkey"],
            }
            for validator in validators
        ]
    else:
        genesis_validator_entries = [
            {
                "attestation_public_key": validator,
                "proposal_public_key": None,
            }
            for validator in validators
        ]

    return {
        "genesis_time": genesis_config["GENESIS_TIME"],
        "genesis_validator_entries": genesis_validator_entries,
    }


def encode_enr_port(port: int) -> bytes:
    byte_length = max(1, (port.bit_length() + 7) // 8)
    return port.to_bytes(byte_length, "big")


def build_canonical_secp256k1_signature(
    identity_key: IdentityKeypair,
    digest: bytes,
) -> bytes:
    der_signature = identity_key.private_key.sign(
        digest,
        ec.ECDSA(Prehashed(hashes.SHA256())),
    )
    signature_r, signature_s = decode_dss_signature(der_signature)
    if signature_s > SECP256K1_ORDER // 2:
        signature_s = SECP256K1_ORDER - signature_s
    return signature_r.to_bytes(32, "big") + signature_s.to_bytes(32, "big")


def build_helper_bootnode_enr(
    identity_key: IdentityKeypair,
    *,
    include_udp: bool = True,
) -> str:
    advertise_ip = os.environ.get(HELPER_ADVERTISE_IP_ENVIRONMENT_VARIABLE)
    if not advertise_ip:
        raise ValueError(
            f"Missing {HELPER_ADVERTISE_IP_ENVIRONMENT_VARIABLE} for helper ENR generation"
        )

    pairs = {
        enr_keys.ID: b"v4",
        enr_keys.SECP256K1: identity_key.public_key.to_bytes(),
        enr_keys.IP: ipaddress.ip_address(advertise_ip).packed,
    }
    if include_udp:
        pairs[enr_keys.UDP] = encode_enr_port(helper_p2p_port())
    pairs[enr_keys.QUIC] = encode_enr_port(helper_p2p_port())

    unsigned_enr = ENR(signature=b"\x00" * 64, seq=1, pairs=pairs)
    digest = keccak.new(digest_bits=256, data=unsigned_enr._content_rlp()).digest()
    # Normalize helper ECDSA signatures so every client sees a canonical ENR.
    signature = build_canonical_secp256k1_signature(identity_key, digest)
    return ENR(signature=signature, seq=1, pairs=pairs).to_string()


def build_helper_bootnode_multiaddr(peer_id: str) -> str:
    advertise_ip = os.environ.get(HELPER_ADVERTISE_IP_ENVIRONMENT_VARIABLE)
    if not advertise_ip:
        raise ValueError(
            f"Missing {HELPER_ADVERTISE_IP_ENVIRONMENT_VARIABLE} for helper multiaddr generation"
        )

    return f"/ip4/{advertise_ip}/udp/{helper_p2p_port()}/quic-v1/p2p/{peer_id}"


async def start_metadata_server(metadata: dict[str, object]) -> web.AppRunner:
    async def metadata_handler(_request: web.Request) -> web.Response:
        return web.json_response(metadata)

    app = web.Application()
    app.router.add_get("/hive/genesis", metadata_handler)

    runner = web.AppRunner(app)
    await runner.setup()
    site = web.TCPSite(runner, host="0.0.0.0", port=helper_metadata_port())
    await site.start()
    logger.info("Helper metadata server listening on 0.0.0.0:%d", helper_metadata_port())
    return runner


def resolve_bootnode(bootnode: str) -> str:
    if bootnode.startswith("enr:"):
        enr = ENR.from_string(bootnode)
        if not enr.is_valid():
            raise ValueError(f"ENR structurally invalid: {bootnode}")
        if not enr.verify_signature():
            raise ValueError(f"ENR signature verification failed: {bootnode}")

        multiaddr = enr.multiaddr()
        if multiaddr is None:
            raise ValueError(f"ENR has no UDP connection info: {bootnode}")
        return multiaddr

    return bootnode


def load_genesis() -> GenesisConfig:
    if not GENESIS_CONFIG_PATH.exists():
        raise FileNotFoundError(
            f"Prepared LeanSpec genesis config is missing: {GENESIS_CONFIG_PATH}"
        )
    return GenesisConfig.from_yaml_file(GENESIS_CONFIG_PATH)


def load_validator_registry(node_id: str) -> ValidatorRegistry | None:
    validator_indices = parse_validator_indices()
    if validator_indices:
        if uses_latest_leanspec_format():
            # The devnet4 helper finalization shim below never produces local
            # attestations, so avoid decoding the large prod attestation keys:
            # that path currently segfaults in LeanSpec. Proposal keys still
            # need to be real because checkpoint-sync clients verify block
            # proposer signatures.
            registry = ValidatorRegistry()
            for index in validator_indices:
                validator = load_validator(index)
                registry.add(
                    ValidatorEntry(
                        index=ValidatorIndex(index),
                        attestation_secret_key=None,  # type: ignore[arg-type]
                        proposal_secret_key=decode_secret_key(
                            validator[PROPOSAL_SECRET_FIELD]
                        ),
                    )
                )
            return registry

        # LeanSpec's ValidatorRegistry.from_yaml path intermittently segfaults
        # while decoding the bundled secret-key files. Decode the already
        # available JSON payloads directly instead for assigned validators.
        was_gc_enabled = gc.isenabled()
        if was_gc_enabled:
            gc.disable()
        try:
            registry = ValidatorRegistry()
            if uses_latest_leanspec_format():
                for index in validator_indices:
                    validator = load_validator(index)
                    registry.add(
                        ValidatorEntry(
                            index=ValidatorIndex(index),
                            attestation_secret_key=decode_secret_key(
                                validator[ATTESTATION_SECRET_FIELD]
                            ),
                            proposal_secret_key=decode_secret_key(
                                validator[PROPOSAL_SECRET_FIELD]
                            ),
                        )
                    )
            else:
                for index in validator_indices:
                    validator = load_validator(index)
                    registry.add(
                        ValidatorEntry(
                            index=ValidatorIndex(index),
                            secret_key=decode_secret_key(validator[ATTESTATION_SECRET_FIELD]),
                        )
                    )
            return registry
        except Exception as err:
            logger.warning(
                "Falling back to ValidatorRegistry.from_yaml for %s after direct key decode failed: %s",
                node_id,
                err,
            )
        finally:
            if was_gc_enabled:
                gc.enable()

    if not VALIDATORS_YAML_PATH.exists() or not VALIDATOR_MANIFEST_PATH.exists():
        return None

    registry = ValidatorRegistry.from_yaml(
        node_id=node_id,
        validators_path=VALIDATORS_YAML_PATH,
        manifest_path=VALIDATOR_MANIFEST_PATH,
    )
    return registry if len(registry) > 0 else None


def subscribe_gossip_topics(
    event_source: LiveNetworkEventSource,
    validator_registry: ValidatorRegistry | None,
    is_aggregator: bool,
) -> None:
    fork_digest = helper_gossip_fork_digest()
    block_topic = GossipTopic.block(fork_digest).to_topic_id()
    event_source.subscribe_gossip_topic(block_topic)

    if is_aggregator and hasattr(GossipTopic, "committee_aggregation"):
        aggregation_topic = GossipTopic.committee_aggregation(fork_digest).to_topic_id()
        event_source.subscribe_gossip_topic(aggregation_topic)

    subscribed_subnets: set[SubnetId] = set()
    if validator_registry is not None:
        for validator_index in validator_registry.indices():
            subscribed_subnets.add(
                validator_index.compute_subnet_id(ATTESTATION_COMMITTEE_COUNT)
            )

    if is_aggregator and not subscribed_subnets:
        subscribed_subnets.add(SubnetId(0))

    for subnet_id in subscribed_subnets:
        topic = GossipTopic.attestation_subnet(fork_digest, subnet_id).to_topic_id()
        event_source.subscribe_gossip_topic(topic)


def current_status(node: Node) -> Status:
    store = node.sync_service.store
    head_root = store.head
    head_block = store.blocks[head_root]
    return Status(
        finalized=store.latest_finalized,
        head=Checkpoint(root=head_root, slot=head_block.slot),
    )


def refresh_status(event_source: LiveNetworkEventSource, node: Node) -> None:
    event_source.set_status(current_status(node))


def extract_inner_block(signed_block: object) -> object:
    if hasattr(signed_block, "block"):
        return getattr(signed_block, "block")

    if hasattr(signed_block, "slot") and hasattr(signed_block, "parent_root"):
        return signed_block

    message = getattr(signed_block, "message", None)
    if message is not None:
        if hasattr(message, "block"):
            return message.block

        # Latest LeanSpec SignedBlock wraps the Block directly in `.message`.
        if hasattr(message, "slot") and hasattr(message, "parent_root"):
            return message

    raise AttributeError(
        "Signed block has neither `.block`, `.message.block`, nor a block-like `.message`"
    )


def cache_signed_block(
    block_cache: dict[Bytes32, object],
    signed_block: object,
) -> Bytes32:
    block_root = hash_tree_root(extract_inner_block(signed_block))
    block_cache[block_root] = signed_block
    return block_root


async def keep_status_current(
    event_source: LiveNetworkEventSource,
    node: Node,
) -> None:
    while True:
        refresh_status(event_source, node)
        await asyncio.sleep(STATUS_REFRESH_INTERVAL_SECS)


async def start_listener_and_gossipsub(
    event_source: LiveNetworkEventSource,
) -> asyncio.Task[None]:
    logger.info("Starting listener on %s", listen_addr())
    listener_task = asyncio.create_task(
        event_source.listen(listen_addr()),
        name="lean-spec-listener",
    )
    await asyncio.sleep(0.1)

    if listener_task.done():
        listener_task.result()

    logger.info("Starting gossipsub behavior...")
    await event_source.start_gossipsub()
    return listener_task


async def dial_bootnodes(
    event_source: LiveNetworkEventSource,
    bootnodes: list[str],
) -> None:
    for bootnode in bootnodes:
        multiaddr = resolve_bootnode(bootnode)
        logger.info("Connecting to bootnode %s", multiaddr)
        try:
            peer_id = await asyncio.wait_for(
                event_source.dial(multiaddr),
                timeout=BOOTNODE_DIAL_TIMEOUT_SECS,
            )
            if peer_id is None:
                logger.warning("Failed to connect to bootnode %s", multiaddr)
            else:
                logger.info("Connected to bootnode, peer_id=%s", peer_id)
        except asyncio.TimeoutError:
            logger.warning(
                "Timed out after %.1fs while connecting to bootnode %s",
                BOOTNODE_DIAL_TIMEOUT_SECS,
                multiaddr,
            )
        except Exception:
            logger.warning("Failed to connect to bootnode %s", multiaddr, exc_info=True)


async def run() -> None:
    setup_logging()
    install_low_s_identity_signature_compatibility()
    install_quic_stream_close_reset_tolerance()
    install_snappy_compress_fallback()
    install_fast_ssz_deserialization()
    install_reqresp_block_cache_tracking()
    install_network_service_gossip_tracking()
    install_identify_protocol_compatibility()
    install_setup_prover_compatibility()
    install_child_only_aggregation_compatibility()
    install_trusted_gossip_attestation_compatibility()
    install_checkpoint_finalization_compatibility()
    logger.info("LeanSpec wire compatibility mode: %s", wire_compat_mode())
    if legacy_signed_block_compatibility_enabled():
        install_legacy_signed_block_wire_compatibility()
    metrics.init(name="lean-spec-client", version="0.0.1")

    node_id = os.environ.get("HIVE_NODE_ID", DEFAULT_NODE_ID)
    prepare_runtime_assets(node_id)
    genesis = load_genesis()
    identity_private_key_hex = os.environ.get(
        HELPER_IDENTITY_PRIVATE_KEY_ENVIRONMENT_VARIABLE,
        HELPER_IDENTITY_PRIVATE_KEY_HEX,
    )
    identity_key = identity_keypair_from_private_key_hex(identity_private_key_hex)
    metadata = helper_genesis_metadata()
    validator_registry = load_validator_registry(node_id)
    bootnodes = parse_bootnodes()
    is_aggregator = os.environ.get("HIVE_IS_AGGREGATOR", "0") == "1"
    disable_validator_service = (
        os.environ.get(DISABLE_VALIDATOR_SERVICE_ENVIRONMENT_VARIABLE, "0") == "1"
    )

    assigned_validators = (
        [int(index) for index in validator_registry.indices()]
        if validator_registry is not None
        else []
    )
    logger.info(
        "Configured helper genesis_time=%d validator_count=%d assigned_validators=%s disable_validator_service=%s",
        genesis.genesis_time,
        len(genesis.genesis_validators),
        assigned_validators,
        disable_validator_service,
    )
    connection_manager = await QuicConnectionManager.create(identity_key)
    metadata["bootnode_enr"] = build_helper_bootnode_enr(identity_key)
    metadata["bootnode_qlean_enr"] = build_helper_bootnode_enr(
        identity_key,
        include_udp=False,
    )
    metadata["bootnode_multiaddr"] = build_helper_bootnode_multiaddr(
        str(connection_manager.peer_id)
    )
    event_source = await LiveNetworkEventSource.create(connection_manager=connection_manager)
    event_source._hive_identify_response = build_identify_response(identity_key)
    configure_event_source_network(event_source, helper_gossip_fork_digest())
    subscribe_gossip_topics(event_source, validator_registry, is_aggregator)
    logger.info("Helper peer_id=%s", connection_manager.peer_id)

    node = Node.from_genesis(
        build_node_config(
            node_config_type=NodeConfig,
            genesis=genesis,
            event_source=event_source,
            validator_registry=validator_registry,
            is_aggregator=is_aggregator,
            fork_digest=helper_gossip_fork_digest(),
        )
    )
    if disable_validator_service and node.validator_service is not None:
        logger.info("Disabling local validator service for passive helper")
        node.validator_service = None

    published_blocks: dict[Bytes32, object] = {}
    _BLOCK_CACHE_BY_SYNC_SERVICE[id(node.sync_service)] = published_blocks
    _SYNC_EVENT_CONTEXT[id(node.sync_service)] = (event_source, node)
    _BLOCK_CACHE_BY_REQRESP_CLIENT[id(event_source.reqresp_client)] = published_blocks

    def lookup_published_block(root: Bytes32) -> object | None:
        return published_blocks.get(root)

    async def lookup_published_block_async(root: Bytes32) -> object | None:
        return lookup_published_block(root)

    def block_for_reqresp(block: object | None) -> object | None:
        return block

    async def lookup_reqresp_block(root: Bytes32) -> object | None:
        return block_for_reqresp(lookup_published_block(root))

    def lookup_published_block_by_slot(slot: object) -> object | None:
        store = node.sync_service.store
        root = store.head
        while root in store.blocks:
            signed_block = published_blocks.get(root)
            block = extract_inner_block(signed_block) if signed_block is not None else store.blocks[root]
            if block.slot == slot:
                return signed_block
            if block.slot < slot or block.parent_root == root:
                break
            root = block.parent_root

        for signed_block in published_blocks.values():
            if extract_inner_block(signed_block).slot == slot:
                return signed_block
        return None

    async def lookup_published_block_by_slot_async(slot: object) -> object | None:
        return lookup_published_block_by_slot(slot)

    async def lookup_reqresp_block_by_slot(slot: object) -> object | None:
        return block_for_reqresp(lookup_published_block_by_slot(slot))

    def current_known_slot() -> object:
        store = node.sync_service.store
        return store.blocks[store.head].slot

    api_server_kwargs = {
        "config": ApiServerConfig(port=helper_api_port()),
        "store_getter": lambda: node.sync_service.store,
    }
    if api_server_supports_signed_block_getter():
        api_server_kwargs["signed_block_getter"] = lookup_published_block
    api_server = ApiServer(**api_server_kwargs)
    metadata_runner = await start_metadata_server(metadata)
    refresh_status(event_source, node)

    if node.validator_service is not None:
        original_on_block = node.validator_service.on_block

        async def cache_and_publish_block(signed_block: object) -> None:
            cache_signed_block(published_blocks, signed_block)
            await original_on_block(signed_block)
            refresh_status(event_source, node)

        node.validator_service.on_block = cache_and_publish_block

    event_source.set_block_lookup(lookup_reqresp_block)
    event_source.set_block_by_slot_lookup(lookup_reqresp_block_by_slot)
    event_source.set_current_slot_lookup(current_known_slot)

    listener_task = await start_listener_and_gossipsub(event_source)
    await dial_bootnodes(event_source, bootnodes)

    node_task = asyncio.create_task(
        node.run(install_signal_handlers=False),
        name="lean-spec-node",
    )
    api_task = asyncio.create_task(api_server.run(), name="lean-spec-api")
    status_task = asyncio.create_task(
        keep_status_current(event_source, node),
        name="lean-spec-status",
    )

    try:
        done, pending = await asyncio.wait(
            {node_task, api_task, status_task},
            return_when=asyncio.FIRST_EXCEPTION,
        )
        for task in done:
            exception = task.exception()
            if exception is not None:
                raise exception

        await asyncio.gather(*pending)
    finally:
        api_server.stop()
        node.stop()
        await metadata_runner.cleanup()
        listener_task.cancel()
        with suppress(asyncio.CancelledError):
            await listener_task
        with suppress(asyncio.CancelledError):
            await node_task
        with suppress(asyncio.CancelledError):
            await api_task
        status_task.cancel()
        with suppress(asyncio.CancelledError):
            await status_task
        _BLOCK_CACHE_BY_SYNC_SERVICE.pop(id(node.sync_service), None)
        _SYNC_EVENT_CONTEXT.pop(id(node.sync_service), None)
        _BLOCK_CACHE_BY_REQRESP_CLIENT.pop(id(event_source.reqresp_client), None)


async def print_bootnode_metadata() -> None:
    identity_private_key_hex = os.environ.get(
        HELPER_IDENTITY_PRIVATE_KEY_ENVIRONMENT_VARIABLE,
        HELPER_IDENTITY_PRIVATE_KEY_HEX,
    )
    identity_key = identity_keypair_from_private_key_hex(identity_private_key_hex)
    peer_id = str(identity_key.to_peer_id())
    print(
        json.dumps(
            {
                "peer_id": peer_id,
                "enr": build_helper_bootnode_enr(identity_key),
                "qlean_enr": build_helper_bootnode_enr(
                    identity_key,
                    include_udp=False,
                ),
                "multiaddr": build_helper_bootnode_multiaddr(peer_id),
            }
        )
    )


def main() -> None:
    if len(sys.argv) > 1 and sys.argv[1] == "--bootnode-metadata":
        asyncio.run(print_bootnode_metadata())
        return

    try:
        asyncio.run(run())
    except KeyboardInterrupt:
        logger.info("Shutting down...")
    except Exception:
        logger.exception("Node failed to start")
        raise


if __name__ == "__main__":
    main()
