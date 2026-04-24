#!/usr/bin/env python3

from __future__ import annotations

import asyncio
import importlib
import io
import ipaddress
import json
import logging
import os
import shutil
import time
from contextlib import suppress
from pathlib import Path
from typing import Final

from aiohttp import web
from Crypto.Hash import keccak
from cryptography.hazmat.primitives import hashes
from cryptography.hazmat.primitives.asymmetric import ec
from cryptography.hazmat.primitives.asymmetric.utils import (
    Prehashed,
    decode_dss_signature,
)
import yaml
from lean_spec.subspecs.api import ApiServer, ApiServerConfig
from lean_spec.subspecs.chain.config import ATTESTATION_COMMITTEE_COUNT
from lean_spec.subspecs.containers import Checkpoint
from lean_spec.subspecs.containers.validator import SubnetId, ValidatorIndex
from lean_spec.subspecs.genesis.config import GenesisConfig
from lean_spec.subspecs.metrics import registry as metrics
from lean_spec.subspecs.networking.client import LiveNetworkEventSource
from lean_spec.subspecs.networking.enr import ENR, keys as enr_keys
from lean_spec.subspecs.networking.gossipsub import GossipTopic
from lean_spec.subspecs.networking.reqresp.message import Status
from lean_spec.subspecs.networking.service.events import GossipBlockEvent
from lean_spec.subspecs.networking.transport.identity import IdentityKeypair
from lean_spec.subspecs.networking.transport.quic.connection import (
    QuicConnectionManager,
    QuicStream,
)
from lean_spec.subspecs.node import Node, NodeConfig
from lean_spec.subspecs.ssz.hash import hash_tree_root
from lean_spec.subspecs.validator import ValidatorRegistry
from lean_spec.subspecs.xmss import SecretKey
from lean_spec.types import Bytes32
from lean_spec.types.collections import SSZList, SSZVector, _validate_offsets
from lean_spec.types.constants import OFFSET_BYTE_LENGTH
from lean_spec.types.container import Container
from lean_spec.types.exceptions import SSZSerializationError, SSZValueError
from lean_spec.types.uint import Uint32

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
VALIDATOR_COUNT: Final = 3
ATTESTATION_PUBKEY_FIELD: Final = "attestation_public"
ATTESTATION_SECRET_FIELD: Final = "attestation_secret"
PROPOSAL_PUBKEY_FIELD: Final = "proposal_public"
PROPOSAL_SECRET_FIELD: Final = "proposal_secret"
LEGACY_PUBLIC_FIELD: Final = "public"
LEGACY_SECRET_FIELD: Final = "secret"

logger = logging.getLogger("lean_spec_client_runner")

_QUIC_STREAM_CLOSE_PATCHED = False
_SSZ_FAST_DESERIALIZATION_PATCHED = False
_SNAPPY_COMPRESS_PATCHED = False
_REQRESP_BLOCK_CACHE_PATCHED = False
_NETWORK_SERVICE_GOSSIP_PATCHED = False
_BLOCK_CACHE_BY_REQRESP_CLIENT: dict[int, dict[Bytes32, object]] = {}
_BLOCK_CACHE_BY_SYNC_SERVICE: dict[int, dict[Bytes32, object]] = {}
_SYNC_EVENT_CONTEXT: dict[int, tuple[LiveNetworkEventSource, Node]] = {}


def setup_logging() -> None:
    logging.basicConfig(
        level=logging.INFO,
        format="%(asctime)s %(levelname)-8s %(name)s: %(message)s",
        datefmt="%Y-%m-%d %H:%M:%S",
    )


def patch_quic_stream_close() -> None:
    global _QUIC_STREAM_CLOSE_PATCHED

    if _QUIC_STREAM_CLOSE_PATCHED:
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
    _QUIC_STREAM_CLOSE_PATCHED = True


def patch_fast_ssz_deserialization() -> None:
    global _SSZ_FAST_DESERIALIZATION_PATCHED

    if _SSZ_FAST_DESERIALIZATION_PATCHED:
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
    _SSZ_FAST_DESERIALIZATION_PATCHED = True


def patch_snappy_compress() -> None:
    global _SNAPPY_COMPRESS_PATCHED

    if _SNAPPY_COMPRESS_PATCHED:
        return

    snappy_compress_module = importlib.import_module("lean_spec.snappy.compress")
    networking_service_module = importlib.import_module(
        "lean_spec.subspecs.networking.service.service"
    )
    original_compress = snappy_compress_module.compress

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
        try:
            return original_compress(data)
        except IndexError:
            logger.warning(
                "Falling back to literal-only Snappy compression after helper compressor bug"
            )
            return literal_only_compress(data)

    snappy_compress_module.compress = compress_with_fallback
    networking_service_module.compress = compress_with_fallback
    _SNAPPY_COMPRESS_PATCHED = True


def patch_reqresp_block_cache() -> None:
    global _REQRESP_BLOCK_CACHE_PATCHED

    if _REQRESP_BLOCK_CACHE_PATCHED:
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
    _REQRESP_BLOCK_CACHE_PATCHED = True


def patch_network_service_gossip_processing() -> None:
    global _NETWORK_SERVICE_GOSSIP_PATCHED

    if _NETWORK_SERVICE_GOSSIP_PATCHED:
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
        processed_count = await self.sync_service.process_pending_blocks()
        if processed_count > 0:
            logger.info(
                "Processed %d cached backfill blocks after slot=%s gossip",
                processed_count,
                extract_inner_block(event.block).slot,
            )

        refresh_status(event_source, node)

    network_service_cls._handle_event = handle_event_with_helper_backfill
    _NETWORK_SERVICE_GOSSIP_PATCHED = True


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


def load_validator(index: int) -> dict[str, str]:
    with (source_keys_dir() / f"{index}.json").open(encoding="utf-8") as key_file:
        validator = json.load(key_file)

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
        f"expected either split attestation/proposal fields or "
        f"{LEGACY_PUBLIC_FIELD!r}/{LEGACY_SECRET_FIELD!r}"
    )


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
    validators = [load_validator(index) for index in range(VALIDATOR_COUNT)]

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


def build_helper_bootnode_enr(identity_key: IdentityKeypair) -> str:
    advertise_ip = os.environ.get(HELPER_ADVERTISE_IP_ENVIRONMENT_VARIABLE)
    if not advertise_ip:
        raise ValueError(
            f"Missing {HELPER_ADVERTISE_IP_ENVIRONMENT_VARIABLE} for helper ENR generation"
        )

    pairs = {
        enr_keys.ID: b"v4",
        enr_keys.SECP256K1: identity_key.public_key.to_bytes(),
        enr_keys.IP: ipaddress.ip_address(advertise_ip).packed,
        enr_keys.UDP: encode_enr_port(helper_p2p_port()),
        enr_keys.QUIC: encode_enr_port(helper_p2p_port()),
    }
    unsigned_enr = ENR(signature=b"\x00" * 64, seq=1, pairs=pairs)
    digest = keccak.new(digest_bits=256, data=unsigned_enr._content_rlp()).digest()
    der_signature = identity_key.private_key.sign(
        digest,
        ec.ECDSA(Prehashed(hashes.SHA256())),
    )
    signature_r, signature_s = decode_dss_signature(der_signature)
    # Zeam rejects non-canonical high-s ENR signatures during startup.
    # Normalize helper ECDSA signatures so every client sees a canonical ENR.
    if signature_s > SECP256K1_ORDER // 2:
        signature_s = SECP256K1_ORDER - signature_s
    signature = signature_r.to_bytes(32, "big") + signature_s.to_bytes(32, "big")
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
        # LeanSpec's ValidatorRegistry.from_yaml path intermittently segfaults
        # while decoding the bundled secret-key files. Decode the already
        # available JSON payloads directly instead for assigned validators.
        #
        # Devnet4's latest SecretKey decoding can also raise during this fast
        # path, so fall back to the manifest loader rather than abort helper
        # startup outright.
        try:
            if uses_latest_leanspec_format():
                validator_keys = {
                    ValidatorIndex(index): (
                        SecretKey.decode_bytes(
                            bytes.fromhex(load_validator(index)[ATTESTATION_SECRET_FIELD])
                        ),
                        SecretKey.decode_bytes(
                            bytes.fromhex(load_validator(index)[PROPOSAL_SECRET_FIELD])
                        ),
                    )
                    for index in validator_indices
                }
            else:
                validator_keys = {
                    ValidatorIndex(index): SecretKey.decode_bytes(
                        bytes.fromhex(load_validator(index)[ATTESTATION_SECRET_FIELD])
                    )
                    for index in validator_indices
                }
            return ValidatorRegistry.from_secret_keys(validator_keys)
        except Exception as err:
            logger.warning(
                "Falling back to ValidatorRegistry.from_yaml for %s after direct key decode failed: %s",
                node_id,
                err,
            )

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
    patch_quic_stream_close()
    patch_fast_ssz_deserialization()
    patch_snappy_compress()
    patch_reqresp_block_cache()
    patch_network_service_gossip_processing()
    metrics.init(name="lean-spec-client", version="0.0.1")

    node_id = os.environ.get("HIVE_NODE_ID", DEFAULT_NODE_ID)
    prepare_runtime_assets(node_id)
    genesis = load_genesis()
    identity_private_key_hex = os.environ.get(
        HELPER_IDENTITY_PRIVATE_KEY_ENVIRONMENT_VARIABLE,
        HELPER_IDENTITY_PRIVATE_KEY_HEX,
    )
    identity_key = IdentityKeypair.from_bytes(
        Bytes32(bytes.fromhex(identity_private_key_hex))
    )
    metadata = helper_genesis_metadata()
    validator_registry = load_validator_registry(node_id)
    bootnodes = parse_bootnodes()
    is_aggregator = os.environ.get("HIVE_IS_AGGREGATOR", "0") == "1"

    assigned_validators = (
        [int(index) for index in validator_registry.indices()]
        if validator_registry is not None
        else []
    )
    logger.info(
        "Configured helper genesis_time=%d validator_count=%d assigned_validators=%s",
        genesis.genesis_time,
        len(genesis.genesis_validators),
        assigned_validators,
    )
    connection_manager = await QuicConnectionManager.create(identity_key)
    metadata["bootnode_enr"] = build_helper_bootnode_enr(identity_key)
    metadata["bootnode_multiaddr"] = build_helper_bootnode_multiaddr(
        str(connection_manager.peer_id)
    )
    event_source = await LiveNetworkEventSource.create(connection_manager=connection_manager)
    event_source.set_fork_digest(helper_gossip_fork_digest())
    subscribe_gossip_topics(event_source, validator_registry, is_aggregator)
    logger.info("Helper peer_id=%s", connection_manager.peer_id)

    node = Node.from_genesis(
        NodeConfig(
            genesis_time=genesis.genesis_time,
            validators=genesis.to_validators(),
            event_source=event_source,
            network=event_source.reqresp_client,
            api_config=None,
            validator_registry=validator_registry,
            fork_digest=helper_gossip_fork_digest(),
            is_aggregator=is_aggregator,
        )
    )
    api_server = ApiServer(
        config=ApiServerConfig(port=helper_api_port()),
        store_getter=lambda: node.sync_service.store,
    )
    metadata_runner = await start_metadata_server(metadata)
    refresh_status(event_source, node)

    published_blocks: dict[Bytes32, object] = {}
    _BLOCK_CACHE_BY_SYNC_SERVICE[id(node.sync_service)] = published_blocks
    _SYNC_EVENT_CONTEXT[id(node.sync_service)] = (event_source, node)
    _BLOCK_CACHE_BY_REQRESP_CLIENT[id(event_source.reqresp_client)] = published_blocks

    if node.validator_service is not None:
        original_on_block = node.validator_service.on_block

        async def cache_and_publish_block(signed_block: object) -> None:
            cache_signed_block(published_blocks, signed_block)
            await original_on_block(signed_block)
            refresh_status(event_source, node)

        node.validator_service.on_block = cache_and_publish_block

    async def lookup_published_block(root: Bytes32) -> object | None:
        return published_blocks.get(root)

    event_source.set_block_lookup(lookup_published_block)

    await dial_bootnodes(event_source, bootnodes)
    listener_task = await start_listener_and_gossipsub(event_source)
    event_source._running = True

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


def main() -> None:
    try:
        asyncio.run(run())
    except KeyboardInterrupt:
        logger.info("Shutting down...")
    except Exception:
        logger.exception("Node failed to start")
        raise


if __name__ == "__main__":
    main()
