#!/usr/bin/env python3

from __future__ import annotations

import asyncio
import gc
import logging
import os
from contextlib import suppress
from typing import Final


from lean_spec_runtime import (
    ATTESTATION_SECRET_FIELD,
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
    install_low_s_identity_signature_compatibility,
    install_setup_prover_compatibility,
    install_snappy_compress_fallback,
    keep_status_current,
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
)

from lean_spec.subspecs.api import ApiServer, ApiServerConfig
from lean_spec.subspecs.metrics import registry as metrics
from lean_spec.subspecs.networking.client import LiveNetworkEventSource
from lean_spec.subspecs.networking.transport.quic.connection import QuicConnectionManager
from lean_spec.subspecs.node import Node, NodeConfig
from lean_spec.subspecs.validator import ValidatorRegistry
from lean_spec.types import Bytes32

GOSSIP_FORK_DIGEST: Final = "devnet0"
LISTEN_ADDR: Final = "/ip4/0.0.0.0/udp/9001/quic-v1"
API_PORT: Final = 5052
HELPER_METADATA_PORT: Final = 5053
HELPER_QUIC_PORT: Final = 9001
DEFAULT_NODE_ID: Final = "lean_spec_0"
HELPER_IDENTITY_PRIVATE_KEY_HEX: Final = (
    "1111111111111111111111111111111111111111111111111111111111111111"
)
HELPER_IDENTITY_PRIVATE_KEY_ENVIRONMENT_VARIABLE: Final = (
    "HIVE_LEAN_HELPER_IDENTITY_PRIVATE_KEY"
)
VALIDATOR_COUNT: Final = 3

logger = logging.getLogger("lean_spec_client_runner")


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
        was_gc_enabled = gc.isenabled()
        if was_gc_enabled:
            gc.disable()
        try:
            if uses_latest_leanspec_format():
                validator_keys = {
                    ValidatorIndex(index): (
                        decode_secret_key(load_validator(index)[ATTESTATION_SECRET_FIELD]),
                        decode_secret_key(load_validator(index)[PROPOSAL_SECRET_FIELD]),
                    )
                    for index in validator_indices
                }
            else:
                validator_keys = {
                    ValidatorIndex(index): decode_secret_key(
                        load_validator(index)[ATTESTATION_SECRET_FIELD]
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
        finally:
            if was_gc_enabled:
                gc.enable()

    return load_validator_registry_from_manifest(ValidatorRegistry, node_id)


async def run() -> None:
    setup_logging()
    install_low_s_identity_signature_compatibility()
    install_snappy_compress_fallback()
    install_fast_ssz_deserialization()
    install_setup_prover_compatibility()
    metrics.init(name="lean-spec-client", version="0.0.1")

    node_id = os.environ.get("HIVE_NODE_ID", DEFAULT_NODE_ID)
    prepare_runtime_assets(node_id, VALIDATOR_COUNT)
    genesis = load_genesis()
    identity_private_key_hex = os.environ.get(
        HELPER_IDENTITY_PRIVATE_KEY_ENVIRONMENT_VARIABLE,
        HELPER_IDENTITY_PRIVATE_KEY_HEX,
    )
    identity_key = identity_keypair_from_private_key_hex(identity_private_key_hex)
    metadata = helper_genesis_metadata()
    metadata["bootnode_enr"] = build_helper_bootnode_enr(identity_key, HELPER_QUIC_PORT)
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
    event_source = await LiveNetworkEventSource.create(connection_manager=connection_manager)
    configure_event_source_network(event_source, GOSSIP_FORK_DIGEST)
    subscribe_gossip_topics(
        event_source,
        validator_registry,
        is_aggregator,
        GOSSIP_FORK_DIGEST,
    )
    logger.info("Helper peer_id=%s", connection_manager.peer_id)

    node = Node.from_genesis(
        build_node_config(
            node_config_type=NodeConfig,
            genesis=genesis,
            event_source=event_source,
            validator_registry=validator_registry,
            is_aggregator=is_aggregator,
            fork_digest=GOSSIP_FORK_DIGEST,
        )
    )
    api_server_kwargs = {
        "config": ApiServerConfig(port=API_PORT),
        "store_getter": lambda: node.sync_service.store,
    }
    metadata_runner = await start_metadata_server(metadata, HELPER_METADATA_PORT)
    refresh_status(event_source, node)

    published_blocks: dict[Bytes32, object] = {}
    if node.validator_service is not None:
        original_on_block = node.validator_service.on_block

        async def cache_and_publish_block(signed_block: object) -> None:
            cache_signed_block(published_blocks, signed_block)
            await original_on_block(signed_block)
            refresh_status(event_source, node)

        node.validator_service.on_block = cache_and_publish_block

    async def lookup_published_block(root: Bytes32) -> object | None:
        return published_blocks.get(root)

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

    def current_known_slot() -> object:
        store = node.sync_service.store
        return store.blocks[store.head].slot

    if api_server_supports_signed_block_getter(ApiServer):
        api_server_kwargs["signed_block_getter"] = lambda root: published_blocks.get(root)
    api_server = ApiServer(**api_server_kwargs)

    event_source.set_block_lookup(lookup_published_block)
    event_source.set_block_by_slot_lookup(lookup_published_block_by_slot_async)
    event_source.set_current_slot_lookup(current_known_slot)

    listener_task = await start_listener_and_gossipsub(event_source, LISTEN_ADDR)
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
