#!/usr/bin/env python3

from __future__ import annotations

import asyncio
import gc
import importlib
import inspect
import json
import logging
import os
import sys
from contextlib import suppress
from typing import Any, Final

from lean_spec_runtime import (
    ATTESTATION_SECRET_FIELD,
    Checkpoint,
    HELPER_ADVERTISE_IP_ENVIRONMENT_VARIABLE,
    PROPOSAL_SECRET_FIELD,
    ValidatorIndex,
    api_server_supports_signed_block_getter,
    build_helper_bootnode_enr,
    build_node_config,
    cache_signed_block,
    configure_event_source_network,
    decode_secret_key,
    dial_bootnodes,
    extract_inner_block,
    helper_genesis_metadata,
    identity_keypair_from_private_key_hex,
    install_fast_ssz_deserialization,
    install_legacy_signed_block_wire_compatibility,
    install_low_s_identity_signature_compatibility,
    install_setup_prover_compatibility,
    install_snappy_compress_fallback,
    keep_status_current,
    legacy_signed_block_compatibility_enabled,
    load_genesis,
    load_validator,
    load_validator_registry_from_manifest,
    parse_bootnodes,
    parse_validator_indices,
    prepare_runtime_assets,
    refresh_status,
    setup_logging,
    start_listener_and_gossipsub,
    start_metadata_server,
    subscribe_gossip_topics,
    uses_latest_leanspec_format,
    wire_compat_mode,
)

from lean_spec.subspecs.api import ApiServer, ApiServerConfig
from lean_spec.subspecs.metrics import registry as metrics
from lean_spec.subspecs.networking.client import LiveNetworkEventSource
from lean_spec.subspecs.networking.service.events import GossipBlockEvent
from lean_spec.subspecs.networking.transport.identity import IdentityKeypair
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
from lean_spec.types import Bytes32
try:
    from lean_spec.types.participation import AggregationBits, ValidatorIndices
except ImportError:
    from lean_spec.types import AggregationBits, ValidatorIndices
from lean_spec.types.uint import Uint32, Uint64

DEFAULT_GOSSIP_FORK_DIGEST: Final = "devnet0"
DEFAULT_LISTEN_PORT: Final = 9001
DEFAULT_API_PORT: Final = 5052
DEFAULT_HELPER_METADATA_PORT: Final = 5053
DEFAULT_NODE_ID: Final = "lean_spec_0"
HELPER_IDENTITY_PRIVATE_KEY_HEX: Final = (
    "1111111111111111111111111111111111111111111111111111111111111111"
)
HELPER_IDENTITY_PRIVATE_KEY_ENVIRONMENT_VARIABLE: Final = (
    "HIVE_LEAN_HELPER_IDENTITY_PRIVATE_KEY"
)
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

logger = logging.getLogger("lean_spec_client_runner")


_QUIC_STREAM_CLOSE_COMPAT_INSTALLED = False
_REQRESP_BLOCK_CACHE_TRACKING_INSTALLED = False
_NETWORK_SERVICE_GOSSIP_TRACKING_INSTALLED = False
_IDENTIFY_PROTOCOL_COMPAT_INSTALLED = False
_CHILD_ONLY_AGGREGATION_COMPAT_INSTALLED = False
_TRUSTED_GOSSIP_ATTESTATION_COMPAT_INSTALLED = False
_CHECKPOINT_FINALIZATION_COMPAT_INSTALLED = False
_BLOCK_CACHE_BY_REQRESP_CLIENT: dict[int, dict[Bytes32, object]] = {}
_BLOCK_CACHE_BY_SYNC_SERVICE: dict[int, dict[Bytes32, object]] = {}
_SYNC_EVENT_CONTEXT: dict[int, tuple[LiveNetworkEventSource, Node]] = {}


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


def genesis_validator_count() -> int:
    raw_count = os.environ.get(GENESIS_VALIDATOR_COUNT_ENVIRONMENT_VARIABLE, "")
    if not raw_count:
        return DEFAULT_VALIDATOR_COUNT

    count = int(raw_count)
    if count < 0:
        raise ValueError(f"{GENESIS_VALIDATOR_COUNT_ENVIRONMENT_VARIABLE} must be non-negative")
    return count


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


def build_helper_bootnode_multiaddr(peer_id: str) -> str:
    advertise_ip = os.environ.get(HELPER_ADVERTISE_IP_ENVIRONMENT_VARIABLE)
    if not advertise_ip:
        raise ValueError(
            f"Missing {HELPER_ADVERTISE_IP_ENVIRONMENT_VARIABLE} for helper multiaddr generation"
        )

    return f"/ip4/{advertise_ip}/udp/{helper_p2p_port()}/quic-v1/p2p/{peer_id}"


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

    return load_validator_registry_from_manifest(ValidatorRegistry, node_id)


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
    validator_count = genesis_validator_count()
    prepare_runtime_assets(
        node_id,
        validator_count,
        genesis_extra_fields={
            "ATTESTATION_COMMITTEE_COUNT": 1,
            "MAX_ATTESTATIONS_DATA": 16,
            "NUM_VALIDATORS": validator_count,
            "VALIDATOR_COUNT": validator_count,
            "ACTIVE_EPOCH": 18,
        },
        validator_count_environment_variable=GENESIS_VALIDATOR_COUNT_ENVIRONMENT_VARIABLE,
    )
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
    metadata["bootnode_enr"] = build_helper_bootnode_enr(
        identity_key,
        helper_p2p_port(),
    )
    metadata["bootnode_qlean_enr"] = build_helper_bootnode_enr(
        identity_key,
        helper_p2p_port(),
        include_udp=False,
    )
    metadata["bootnode_multiaddr"] = build_helper_bootnode_multiaddr(
        str(connection_manager.peer_id)
    )
    event_source = await LiveNetworkEventSource.create(connection_manager=connection_manager)
    event_source._hive_identify_response = build_identify_response(identity_key)
    configure_event_source_network(event_source, helper_gossip_fork_digest())
    subscribe_gossip_topics(
        event_source,
        validator_registry,
        is_aggregator,
        helper_gossip_fork_digest(),
        include_committee_aggregation=True,
    )
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
    if api_server_supports_signed_block_getter(ApiServer):
        api_server_kwargs["signed_block_getter"] = lookup_published_block
    api_server = ApiServer(**api_server_kwargs)
    metadata_runner = await start_metadata_server(metadata, helper_metadata_port())
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

    listener_task = await start_listener_and_gossipsub(event_source, listen_addr())
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
                "enr": build_helper_bootnode_enr(identity_key, helper_p2p_port()),
                "qlean_enr": build_helper_bootnode_enr(
                    identity_key,
                    helper_p2p_port(),
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
