#!/usr/bin/env python3

from __future__ import annotations

import asyncio
import gc
import hashlib
import importlib
import io
import inspect
import ipaddress
import json
import logging
import os
import shutil
import sys
import time
import typing
import types
from pathlib import Path
from typing import Any, Final

from cryptography import x509
from cryptography.hazmat.primitives import hashes
from cryptography.hazmat.primitives import serialization
from cryptography.hazmat.primitives.asymmetric import ec
from cryptography.hazmat.primitives.asymmetric.utils import (
    Prehashed,
    decode_dss_signature,
    encode_dss_signature,
)
from aiohttp import web
from Crypto.Hash import keccak
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

# leanSpec restructured its packages over time (subspecs/types -> node/spec).
# The devnet5 helper builds from leanSpec main (new layout) while the devnet4
# helper stays pinned on the old layout, so every import tries the new
# location first and falls back to the old ones.
try:
    from lean_spec.spec.forks import DEFAULT_REGISTRY
except (ImportError, ModuleNotFoundError):
    from lean_spec.forks import DEFAULT_REGISTRY

try:
    from lean_spec.spec.forks.lstar.containers import Checkpoint, SubnetId, ValidatorIndex
except (ImportError, ModuleNotFoundError):
    try:
        from lean_spec.subspecs.containers import Checkpoint
        from lean_spec.subspecs.containers.validator import SubnetId, ValidatorIndex
    except (ImportError, ModuleNotFoundError):
        try:
            from lean_spec.types import Checkpoint, SubnetId, ValidatorIndex
        except (ImportError, ModuleNotFoundError):
            from lean_spec.forks.lstar.containers import Checkpoint
            from lean_spec.forks.lstar.containers.validator import SubnetId, ValidatorIndex

try:
    from lean_spec.node.genesis import GenesisConfig
    from lean_spec.node.networking.enr import ENR, keys as enr_keys
    from lean_spec.node.networking.gossipsub import GossipTopic
    from lean_spec.node.networking.reqresp.message import Status
    from lean_spec.node.networking.transport.identity import (
        IdentityKeypair,
        Secp256k1PublicKey,
    )
    from lean_spec.spec.crypto.merkleization import hash_tree_root
    from lean_spec.spec.crypto.xmss import SecretKey
    from lean_spec.spec.forks.lstar import aggregation as xmss_aggregation_module
    from lean_spec.spec.forks.lstar.config import ATTESTATION_COMMITTEE_COUNT
    from lean_spec.spec.ssz import (
        Bytes32,
        Container,
        SSZList,
        SSZSerializationError,
        SSZValueError,
        SSZVector,
        Uint32,
    )
    from lean_spec.spec.ssz.collections import _validate_offsets
    from lean_spec.spec.ssz.ssz_base import BYTES_PER_LENGTH_OFFSET as OFFSET_BYTE_LENGTH
except (ImportError, ModuleNotFoundError):
    from lean_spec.subspecs.chain.config import ATTESTATION_COMMITTEE_COUNT
    from lean_spec.subspecs.genesis.config import GenesisConfig
    from lean_spec.subspecs.networking.enr import ENR, keys as enr_keys
    from lean_spec.subspecs.networking.gossipsub import GossipTopic
    from lean_spec.subspecs.networking.reqresp.message import Status
    from lean_spec.subspecs.networking.transport.identity import IdentityKeypair
    from lean_spec.subspecs.networking.transport.identity.keypair import Secp256k1PublicKey
    from lean_spec.subspecs.ssz.hash import hash_tree_root
    from lean_spec.subspecs.xmss import SecretKey
    from lean_spec.subspecs.xmss import aggregation as xmss_aggregation_module
    from lean_spec.types import Bytes32
    from lean_spec.types.collections import SSZList, SSZVector, _validate_offsets
    from lean_spec.types.constants import OFFSET_BYTE_LENGTH
    from lean_spec.types.container import Container
    from lean_spec.types.exceptions import SSZSerializationError, SSZValueError
    from lean_spec.types.uint import Uint32

logger = logging.getLogger("lean_spec_client_runner")


def import_first_module(*module_names: str) -> types.ModuleType:
    """Import the first available module from a list of historical locations.

    Mirrors the try/except import chains above for dynamically imported
    modules: new-layout names come first, old-layout names act as fallbacks
    for the pinned devnet4 helper checkout.
    """
    for module_name in module_names[:-1]:
        try:
            return importlib.import_module(module_name)
        except (ImportError, ModuleNotFoundError):
            continue
    return importlib.import_module(module_names[-1])


BOOTNODE_DIAL_TIMEOUT_SECS: Final = 10.0
STATUS_REFRESH_INTERVAL_SECS: Final = 1.0
PREPARED_ASSETS_DIR: Final = Path(
    os.environ.get("LEAN_RUNTIME_ASSET_ROOT", "/tmp/lean-spec-client")
)
DEVNET4_SOURCE_KEYS_DIR: Final = Path("/app/hive/prod_scheme_devnet4")
DEVNET5_SOURCE_KEYS_DIR: Final = Path("/app/hive/prod_scheme_devnet5")
GENESIS_DIR: Final = PREPARED_ASSETS_DIR / "genesis"
GENESIS_CONFIG_PATH: Final = PREPARED_ASSETS_DIR / "genesis" / "config.yaml"
VALIDATOR_KEYS_DIR: Final = PREPARED_ASSETS_DIR / "keys"
MANIFEST_DIR: Final = VALIDATOR_KEYS_DIR / "hash-sig-keys"
VALIDATORS_YAML_PATH: Final = VALIDATOR_KEYS_DIR / "validators.yaml"
VALIDATOR_MANIFEST_PATH: Final = (
    VALIDATOR_KEYS_DIR / "hash-sig-keys" / "validator-keys-manifest.yaml"
)
SECP256K1_ORDER: Final = int(
    "FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFEBAAEDCE6AF48A03BBFD25E8CD0364141",
    16,
)
LIBP2P_TLS_EXTENSION_OID: Final = x509.ObjectIdentifier("1.3.6.1.4.1.53594.1.1")
LIBP2P_TLS_SIGNATURE_PREFIX: Final = b"libp2p-tls-handshake:"
LIBP2P_SECP256K1_KEY_TYPE: Final = 2
HELPER_ADVERTISE_IP_ENVIRONMENT_VARIABLE: Final = "HIVE_LEAN_HELPER_ADVERTISE_IP"
ATTESTATION_PUBKEY_FIELD: Final = "attestation_public"
ATTESTATION_SECRET_FIELD: Final = "attestation_secret"
PROPOSAL_PUBKEY_FIELD: Final = "proposal_public"
PROPOSAL_SECRET_FIELD: Final = "proposal_secret"
LEGACY_PUBLIC_FIELD: Final = "public"
LEGACY_SECRET_FIELD: Final = "secret"
_LOW_S_IDENTITY_SIGNATURE_COMPAT_INSTALLED = False
_SSZ_FAST_DESERIALIZATION_INSTALLED = False
_SNAPPY_COMPRESS_FALLBACK_INSTALLED = False
_SETUP_PROVER_COMPAT_INSTALLED = False
_INBOUND_QUIC_COMPAT_INSTALLED = False


def setup_logging() -> None:
    logging.basicConfig(
        level=logging.INFO,
        format="%(asctime)s %(levelname)-8s %(name)s: %(message)s",
        datefmt="%Y-%m-%d %H:%M:%S",
    )


def api_server_supports_signed_block_getter(api_server_type: type[Any]) -> bool:
    return "signed_block_getter" in inspect.signature(api_server_type).parameters


def install_finalized_block_api_route(signed_block_getter: Any) -> None:
    try:
        routes_module = import_first_module(
            "lean_spec.node.api.routes",
            "lean_spec.subspecs.api.routes",
        )
    except (ImportError, ModuleNotFoundError):
        return
    routes = getattr(routes_module, "ROUTES", None)
    route_type = getattr(routes_module, "Route", None)
    if routes is None or route_type is None:
        return

    if any(
        getattr(route, "method", None) == "GET"
        and getattr(route, "path", None) == "/lean/v0/blocks/finalized"
        for route in routes
    ):
        return

    async def handle_finalized_block(request: web.Request) -> web.Response:
        store_getter = request.app.get("store_getter")
        store = store_getter() if store_getter else None
        if store is None:
            raise web.HTTPServiceUnavailable(reason="Store not initialized")

        signed_block = signed_block_getter(store.latest_finalized.root)
        if inspect.isawaitable(signed_block):
            signed_block = await signed_block
        if signed_block is None:
            raise web.HTTPNotFound(reason="Finalized block not available")

        ssz_bytes = await asyncio.to_thread(signed_block.encode_bytes)
        return web.Response(body=ssz_bytes, content_type="application/octet-stream")

    routes.append(
        route_type(
            "GET",
            "/lean/v0/blocks/finalized",
            handle_finalized_block,
            is_admin=False,
        )
    )
    logger.info("Installed LeanSpec finalized block API route")


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


def _decode_der_length(payload: bytes, offset: int) -> tuple[int, int]:
    if offset >= len(payload):
        raise ValueError("DER length truncated")

    first = payload[offset]
    offset += 1
    if first < 0x80:
        return first, offset

    length_size = first & 0x7F
    if length_size == 0 or length_size > 4:
        raise ValueError("Unsupported DER length")
    if offset + length_size > len(payload):
        raise ValueError("DER long-form length truncated")

    value = int.from_bytes(payload[offset : offset + length_size], "big")
    return value, offset + length_size


def _decode_der_value(payload: bytes, offset: int, expected_tag: int) -> tuple[bytes, int]:
    if offset >= len(payload):
        raise ValueError("DER value truncated")
    if payload[offset] != expected_tag:
        raise ValueError(f"Expected DER tag 0x{expected_tag:02x}")

    length, value_offset = _decode_der_length(payload, offset + 1)
    end = value_offset + length
    if end > len(payload):
        raise ValueError("DER value truncated")
    return payload[value_offset:end], end


def _decode_libp2p_signed_key(payload: bytes) -> tuple[bytes, bytes]:
    sequence, offset = _decode_der_value(payload, 0, 0x30)
    if offset != len(payload):
        raise ValueError("Trailing bytes after libp2p TLS extension sequence")

    public_key, offset = _decode_der_value(sequence, 0, 0x04)
    signature, offset = _decode_der_value(sequence, offset, 0x04)
    if offset != len(sequence):
        raise ValueError("Trailing bytes after libp2p signed-key payload")
    return public_key, signature


def _decode_unsigned_varint(payload: bytes, offset: int) -> tuple[int, int]:
    value = 0
    shift = 0
    for _ in range(10):
        if offset >= len(payload):
            raise ValueError("protobuf varint truncated")
        byte = payload[offset]
        offset += 1
        value |= (byte & 0x7F) << shift
        if byte & 0x80 == 0:
            return value, offset
        shift += 7
    raise ValueError("protobuf varint is too long")


def _decode_libp2p_public_key_protobuf(payload: bytes) -> tuple[int | None, bytes | None]:
    offset = 0
    key_type = None
    key_data = None
    while offset < len(payload):
        key, offset = _decode_unsigned_varint(payload, offset)
        field_number = key >> 3
        wire_type = key & 0x07
        if field_number == 1 and wire_type == 0:
            key_type, offset = _decode_unsigned_varint(payload, offset)
        elif field_number == 2 and wire_type == 2:
            length, offset = _decode_unsigned_varint(payload, offset)
            end = offset + length
            if end > len(payload):
                raise ValueError("protobuf public-key data truncated")
            key_data = payload[offset:end]
            offset = end
        elif wire_type == 0:
            _, offset = _decode_unsigned_varint(payload, offset)
        elif wire_type == 2:
            length, offset = _decode_unsigned_varint(payload, offset)
            offset += length
            if offset > len(payload):
                raise ValueError("protobuf length-delimited field truncated")
        else:
            raise ValueError(f"Unsupported protobuf wire type: {wire_type}")

    return key_type, key_data


def _peer_id_type() -> type[Any]:
    try:
        peer_id_module = import_first_module(
            "lean_spec.node.networking.transport.peer_id",
            "lean_spec.node.networking.transport",
            "lean_spec.subspecs.networking.transport.peer_id",
            "lean_spec.subspecs.networking.transport",
        )
    except (ImportError, ModuleNotFoundError) as err:
        raise ImportError("Unable to import LeanSpec PeerId") from err

    return getattr(peer_id_module, "PeerId")


def _peer_id_from_public_key_protobuf(public_key_protobuf: bytes) -> Any:
    digest = (
        bytes([0x00, len(public_key_protobuf)]) + public_key_protobuf
        if len(public_key_protobuf) <= 42
        else b"\x12\x20" + hashlib.sha256(public_key_protobuf).digest()
    )
    return _peer_id_type()(multihash=digest)


def _verify_libp2p_tls_identity(
    certificate: x509.Certificate,
    public_key_protobuf: bytes,
    signature: bytes,
) -> None:
    key_type, key_data = _decode_libp2p_public_key_protobuf(public_key_protobuf)
    if key_type != LIBP2P_SECP256K1_KEY_TYPE or key_data is None:
        logger.debug("Skipping libp2p TLS signature verification for key type %s", key_type)
        return

    identity_public_key = ec.EllipticCurvePublicKey.from_encoded_point(
        ec.SECP256K1(),
        key_data,
    )
    tls_public_key_bytes = certificate.public_key().public_bytes(
        encoding=serialization.Encoding.DER,
        format=serialization.PublicFormat.SubjectPublicKeyInfo,
    )
    message = LIBP2P_TLS_SIGNATURE_PREFIX + tls_public_key_bytes

    signature_candidates = [signature]
    if len(signature) == 64:
        signature_r = int.from_bytes(signature[:32], "big")
        signature_s = int.from_bytes(signature[32:], "big")
        signature_candidates.append(encode_dss_signature(signature_r, signature_s))

    last_error = None
    for candidate in signature_candidates:
        try:
            identity_public_key.verify(candidate, message, ec.ECDSA(hashes.SHA256()))
            return
        except Exception as err:
            last_error = err

    logger.debug("Unable to verify libp2p TLS identity signature: %s", last_error)


def _quic_protocol_peer_certificate(protocol_instance: Any) -> x509.Certificate | None:
    quic = getattr(protocol_instance, "_quic", None)
    tls_context = getattr(quic, "tls", None)
    certificate = getattr(tls_context, "_peer_certificate", None)
    if isinstance(certificate, x509.Certificate):
        return certificate
    return None


def _peer_id_from_quic_protocol_certificate(protocol_instance: Any) -> Any | None:
    certificate = _quic_protocol_peer_certificate(protocol_instance)
    if certificate is None:
        return None

    try:
        extension = certificate.extensions.get_extension_for_oid(LIBP2P_TLS_EXTENSION_OID)
        public_key_protobuf, signature = _decode_libp2p_signed_key(extension.value.value)
        _verify_libp2p_tls_identity(certificate, public_key_protobuf, signature)
        return _peer_id_from_public_key_protobuf(public_key_protobuf)
    except Exception as err:
        logger.debug("Unable to derive peer ID from libp2p TLS certificate: %s", err)
        return None


def _fallback_peer_id_from_quic_protocol(protocol_instance: Any) -> Any:
    quic = getattr(protocol_instance, "_quic", None)
    seed_parts = [
        b"hive-lean-inbound-quic",
        str(id(protocol_instance)).encode(),
    ]
    for attr_name in ("_peer_cid", "_original_destination_connection_id"):
        value = getattr(quic, attr_name, None)
        cid = getattr(value, "cid", value)
        if isinstance(cid, bytes):
            seed_parts.append(cid)

    return _peer_id_type()(multihash=b"\x12\x20" + hashlib.sha256(b"|".join(seed_parts)).digest())


def install_inbound_quic_connection_compatibility() -> None:
    global _INBOUND_QUIC_COMPAT_INSTALLED

    if _INBOUND_QUIC_COMPAT_INSTALLED:
        return

    try:
        connection_module = import_first_module(
            "lean_spec.node.networking.transport.quic.connection",
            "lean_spec.subspecs.networking.transport.quic.connection",
        )
    except (ImportError, ModuleNotFoundError) as err:
        logger.debug("LeanSpec inbound QUIC compatibility unavailable: %s", err)
        return

    connection_manager_cls = connection_module.QuicConnectionManager
    original_listen = connection_manager_cls.listen
    try:
        original_listen_source = inspect.getsource(original_listen)
    except (OSError, TypeError):
        original_listen_source = ""
    if "peer identity verification from the libp2p certificate is not implemented" not in original_listen_source:
        _INBOUND_QUIC_COMPAT_INSTALLED = True
        return

    async def listen_with_certificate_peer_id(
        self: Any,
        multiaddr: str,
        on_connection: Any,
    ) -> None:
        host, port, transport, _ = connection_module.parse_multiaddr(multiaddr)
        if transport != "quic":
            raise connection_module.QuicTransportError(f"Not a QUIC multiaddr: {multiaddr}")

        server_config = connection_module.QuicConfiguration(
            alpn_protocols=[connection_module.LIBP2P_ALPN_PROTOCOL],
            is_client=False,
            verify_mode=connection_module.ssl.CERT_NONE,
        )
        if self._temp_directory:
            certificate_path = self._temp_directory / "cert.pem"
            key_path = self._temp_directory / "key.pem"
            server_config.load_cert_chain(str(certificate_path), str(key_path))

        def handle_handshake(protocol_instance: Any) -> None:
            peer_id = _peer_id_from_quic_protocol_certificate(protocol_instance)
            if peer_id is None:
                peer_id = _fallback_peer_id_from_quic_protocol(protocol_instance)
                logger.info(
                    "Accepted inbound QUIC connection without certificate peer ID; "
                    "using local scoped peer ID %s",
                    peer_id,
                )

            connection = connection_module.QuicConnection(
                _protocol=protocol_instance,
                _peer_id=peer_id,
                _remote_address=f"inbound-quic/{peer_id}",
            )
            protocol_instance.connection = connection
            protocol_instance._replay_buffered_events()
            self._connections[peer_id] = connection

            task = asyncio.create_task(on_connection(connection))

            def log_callback_error(done: asyncio.Task[Any]) -> None:
                try:
                    done.result()
                except asyncio.CancelledError:
                    pass
                except Exception as err:
                    logger.warning("Inbound QUIC connection callback failed: %s", err)

            task.add_done_callback(log_callback_error)

        def create_protocol(*args: Any, **kwargs: Any) -> Any:
            protocol = connection_module.LibP2PQuicProtocol(*args, **kwargs)
            tls_context = getattr(getattr(protocol, "_quic", None), "tls", None)
            if tls_context is not None:
                tls_context._request_client_certificate = True
            protocol._on_handshake = handle_handshake
            return protocol

        await connection_module.quic_serve(
            host,
            port,
            configuration=server_config,
            create_protocol=create_protocol,
        )
        await asyncio.Event().wait()

    connection_manager_cls.listen = listen_with_certificate_peer_id
    _INBOUND_QUIC_COMPAT_INSTALLED = True
    logger.info("Installed LeanSpec inbound QUIC certificate peer-id compatibility")


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

        # Older leanSpec layouts resolve SSZ field types through a private
        # helper; newer ones store the SSZ type directly on the annotation.
        annotation_resolver = getattr(cls, "_get_ssz_field_type", None)
        for field_name, field_info in cls.model_fields.items():
            if annotation_resolver is not None:
                field_type = annotation_resolver(field_info.annotation)
            else:
                field_type = field_info.annotation

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

    snappy_package_module = import_first_module("lean_spec.node.snappy", "lean_spec.snappy")
    snappy_compress_module = import_first_module(
        "lean_spec.node.snappy.compress", "lean_spec.snappy.compress"
    )
    snappy_framing_module = import_first_module(
        "lean_spec.node.snappy.framing", "lean_spec.snappy.framing"
    )
    reqresp_codec_module = import_first_module(
        "lean_spec.node.networking.reqresp.codec",
        "lean_spec.subspecs.networking.reqresp.codec",
    )
    networking_service_module = import_first_module(
        "lean_spec.node.networking.service.service",
        "lean_spec.subspecs.networking.service.service",
    )

    def encode_snappy_uncompressed_length(value: int) -> bytes:
        # Standard unsigned LEB128, inlined because the varint helper moved
        # modules (and changed name) across leanSpec layouts.
        output = bytearray()
        while True:
            byte = value & 0x7F
            value >>= 7
            if value:
                output.append(byte | 0x80)
            else:
                output.append(byte)
                return bytes(output)

    def literal_only_compress(data: bytes) -> bytes:
        if not data:
            return encode_snappy_uncompressed_length(0)

        output = bytearray(encode_snappy_uncompressed_length(len(data)))
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


def devnet_label() -> str:
    return os.environ.get("HIVE_LEAN_DEVNET_LABEL", "devnet4")


def uses_latest_leanspec_format() -> bool:
    return devnet_label() in {"devnet4", "devnet5"}


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
    was_gc_enabled = gc.isenabled()
    if was_gc_enabled:
        gc.disable()
    try:
        return SecretKey.decode_bytes(bytes.fromhex(secret_key_hex))
    finally:
        if was_gc_enabled:
            gc.enable()


def source_keys_dir() -> Path:
    if devnet_label() == "devnet5":
        return DEVNET5_SOURCE_KEYS_DIR
    if devnet_label() == "devnet4":
        return DEVNET4_SOURCE_KEYS_DIR

    raise ValueError(f"Unsupported Lean devnet label: {devnet_label()!r}")


def prepare_output_dirs() -> None:
    if PREPARED_ASSETS_DIR.exists():
        shutil.rmtree(PREPARED_ASSETS_DIR)

    GENESIS_DIR.mkdir(parents=True, exist_ok=True)
    MANIFEST_DIR.mkdir(parents=True, exist_ok=True)


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


def genesis_validator_pubkey_keys() -> tuple[str, str]:
    """Return the per-validator pubkey YAML keys the active GenesisConfig expects.

    Newer leanSpec checkouts spell the keys out in full
    (attestation_public_key); older ones use the abbreviated form
    (attestation_pubkey). Introspect the validator entry model so the
    emitted YAML always matches the checkout that will parse it.
    """
    entry_annotation = GenesisConfig.model_fields["genesis_validators"].annotation
    (entry_class,) = typing.get_args(entry_annotation)
    if "attestation_public_key" in entry_class.model_fields:
        return ("attestation_public_key", "proposal_public_key")
    return ("attestation_pubkey", "proposal_pubkey")


def write_genesis_config(
    validators: list[dict[str, str]],
    genesis_time: int,
) -> None:
    genesis: dict[str, object] = {"GENESIS_TIME": genesis_time}

    if uses_latest_leanspec_format():
        attestation_key, proposal_key = genesis_validator_pubkey_keys()
        genesis["GENESIS_VALIDATORS"] = [
            {
                attestation_key: f"0x{validator[ATTESTATION_PUBKEY_FIELD]}",
                proposal_key: f"0x{validator[PROPOSAL_PUBKEY_FIELD]}",
            }
            for validator in validators
        ]
    else:
        genesis["GENESIS_VALIDATORS"] = [
            f"0x{validator[ATTESTATION_PUBKEY_FIELD]}" for validator in validators
        ]

    with GENESIS_CONFIG_PATH.open("w", encoding="utf-8") as genesis_file:
        yaml.safe_dump(genesis, genesis_file, sort_keys=False)


def prepare_runtime_assets(
    node_id: str,
    validator_count: int,
    *,
    validator_count_environment_variable: str | None = None,
) -> None:
    genesis_time = int(os.environ.get("HIVE_LEAN_GENESIS_TIME", int(time.time()) + 30))
    validator_indices = parse_validator_indices()
    if validator_indices and max(validator_indices) >= validator_count:
        configured_name = validator_count_environment_variable or "validator_count"
        raise ValueError(
            f"{configured_name}={validator_count} does not include "
            f"assigned validator index {max(validator_indices)}"
        )

    validators = [load_validator(index) for index in range(validator_count)]

    prepare_output_dirs()
    write_genesis_config(validators, genesis_time)
    write_validator_keys(validators, node_id, validator_indices)


def load_validator_registry_from_manifest(
    validator_registry_type: type[Any],
    node_id: str,
) -> Any | None:
    if not VALIDATORS_YAML_PATH.exists() or not VALIDATOR_MANIFEST_PATH.exists():
        return None

    registry = validator_registry_type.from_yaml(
        node_id=node_id,
        validators_path=VALIDATORS_YAML_PATH,
        manifest_path=VALIDATOR_MANIFEST_PATH,
    )
    return registry if len(registry) > 0 else None


def helper_genesis_metadata() -> dict[str, object]:
    with GENESIS_CONFIG_PATH.open(encoding="utf-8") as genesis_file:
        genesis_config = yaml.safe_load(genesis_file)

    validators = genesis_config.get("GENESIS_VALIDATORS", [])
    if uses_latest_leanspec_format():
        attestation_key, proposal_key = genesis_validator_pubkey_keys()
        genesis_validator_entries = [
            {
                "attestation_public_key": validator[attestation_key],
                "proposal_public_key": validator[proposal_key],
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
    quic_port: int,
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
        pairs[enr_keys.UDP] = encode_enr_port(quic_port)
    pairs[enr_keys.QUIC] = encode_enr_port(quic_port)

    unsigned_enr = ENR(signature=b"\x00" * 64, seq=1, pairs=pairs)
    digest = keccak.new(digest_bits=256, data=unsigned_enr._content_rlp()).digest()
    signature = build_canonical_secp256k1_signature(identity_key, digest)
    return ENR(signature=signature, seq=1, pairs=pairs).to_string()


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


async def start_metadata_server(
    metadata: dict[str, object],
    port: int,
) -> web.AppRunner:
    async def metadata_handler(_request: web.Request) -> web.Response:
        return web.json_response(metadata)

    app = web.Application()
    app.router.add_get("/hive/genesis", metadata_handler)

    runner = web.AppRunner(app)
    await runner.setup()
    site = web.TCPSite(runner, host="0.0.0.0", port=port)
    await site.start()
    logger.info("Helper metadata server listening on 0.0.0.0:%d", port)
    return runner


def subscribe_gossip_topics(
    event_source: Any,
    validator_registry: Any,
    is_aggregator: bool,
    fork_digest: str,
    *,
    include_committee_aggregation: bool = False,
) -> None:
    block_topic = GossipTopic.block(fork_digest).to_topic_id()
    event_source.subscribe_gossip_topic(block_topic)

    if (
        include_committee_aggregation
        and is_aggregator
        and hasattr(GossipTopic, "committee_aggregation")
    ):
        aggregation_topic = GossipTopic.committee_aggregation(fork_digest).to_topic_id()
        event_source.subscribe_gossip_topic(aggregation_topic)

    subscribed_subnets: set[SubnetId] = set()
    if validator_registry is not None:
        for validator_index in validator_registry.indices():
            subscribed_subnets.add(
                validator_index.compute_subnet_id(ATTESTATION_COMMITTEE_COUNT)
            )
    else:
        for validator_index in parse_validator_indices():
            subscribed_subnets.add(
                ValidatorIndex(validator_index).compute_subnet_id(
                    ATTESTATION_COMMITTEE_COUNT
                )
            )

    if is_aggregator and not subscribed_subnets:
        subscribed_subnets.add(SubnetId(0))

    for subnet_id in subscribed_subnets:
        topic = GossipTopic.attestation_subnet(fork_digest, subnet_id).to_topic_id()
        event_source.subscribe_gossip_topic(topic)


async def start_listener_and_gossipsub(
    event_source: Any,
    listen_addr: str,
) -> asyncio.Task[None]:
    logger.info("Starting listener on %s", listen_addr)
    listener_task = asyncio.create_task(
        event_source.listen(listen_addr),
        name="lean-spec-listener",
    )
    await asyncio.sleep(0.1)

    if listener_task.done():
        listener_task.result()

    logger.info("Starting gossipsub behavior...")
    await event_source.start_gossipsub()
    return listener_task


def current_status(node: Any) -> Status:
    store = node.sync_service.store
    head_root = store.head
    head_block = store.blocks[head_root]
    return Status(
        finalized=store.latest_finalized,
        head=Checkpoint(root=head_root, slot=head_block.slot),
    )


def refresh_status(event_source: Any, node: Any) -> None:
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
    event_source: Any,
    node: Any,
) -> None:
    while True:
        refresh_status(event_source, node)
        await asyncio.sleep(STATUS_REFRESH_INTERVAL_SECS)


async def dial_bootnodes(
    event_source: Any,
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


def gossip_network_name(fork_digest: str) -> str:
    return fork_digest[2:] if fork_digest.startswith("0x") else fork_digest


def configure_event_source_network(event_source: Any, fork_digest: str) -> None:
    if hasattr(event_source, "set_fork_digest"):
        event_source.set_fork_digest(fork_digest)
        return

    if hasattr(event_source, "set_network_name"):
        event_source.set_network_name(gossip_network_name(fork_digest))
        return

    raise AttributeError("LiveNetworkEventSource cannot set fork digest or network name")


def node_config_supports(node_config_type: type[Any], field_name: str) -> bool:
    return field_name in getattr(node_config_type, "__dataclass_fields__", {})


def build_node_config(
    *,
    node_config_type: type[Any],
    genesis: Any,
    event_source: Any,
    validator_registry: Any | None,
    is_aggregator: bool,
    fork_digest: str,
) -> Any:
    config: dict[str, Any] = {
        "genesis_time": genesis.genesis_time,
        "validators": genesis.to_validators(),
        "event_source": event_source,
        "network": event_source.reqresp_client,
        "api_config": None,
        "validator_registry": validator_registry,
        "is_aggregator": is_aggregator,
    }

    if node_config_supports(node_config_type, "fork"):
        config["fork"] = DEFAULT_REGISTRY.current
        config["network_name"] = gossip_network_name(fork_digest)
    else:
        config["fork_digest"] = fork_digest

    return node_config_type(**config)
