#!/usr/bin/env python3

from __future__ import annotations

import asyncio
import logging
import os
from contextlib import suppress
from pathlib import Path
from typing import Final

from lean_spec.subspecs.api import ApiServer, ApiServerConfig
from lean_spec.subspecs.chain.config import ATTESTATION_COMMITTEE_COUNT
from lean_spec.subspecs.containers import Checkpoint
from lean_spec.subspecs.containers.validator import SubnetId
from lean_spec.subspecs.genesis.config import GenesisConfig
from lean_spec.subspecs.metrics import registry as metrics
from lean_spec.subspecs.networking.client import LiveNetworkEventSource
from lean_spec.subspecs.networking.enr import ENR
from lean_spec.subspecs.networking.gossipsub import GossipTopic
from lean_spec.subspecs.networking.reqresp.message import Status
from lean_spec.subspecs.networking.transport.identity import IdentityKeypair
from lean_spec.subspecs.networking.transport.quic.connection import QuicConnectionManager
from lean_spec.subspecs.node import Node, NodeConfig
from lean_spec.subspecs.validator import ValidatorRegistry
from lean_spec.types import Bytes32

GOSSIP_FORK_DIGEST: Final = "devnet0"
LISTEN_ADDR: Final = "/ip4/0.0.0.0/udp/9001/quic-v1"
API_PORT: Final = 5052
BOOTNODE_DIAL_TIMEOUT_SECS: Final = 10.0
DEFAULT_NODE_ID: Final = "lean_spec_0"
PREPARED_ASSETS_DIR: Final = Path("/tmp/lean-spec-client")
GENESIS_CONFIG_PATH: Final = PREPARED_ASSETS_DIR / "genesis" / "config.yaml"
VALIDATOR_KEYS_DIR: Final = PREPARED_ASSETS_DIR / "keys"
VALIDATORS_YAML_PATH: Final = VALIDATOR_KEYS_DIR / "validators.yaml"
VALIDATOR_MANIFEST_PATH: Final = (
    VALIDATOR_KEYS_DIR / "hash-sig-keys" / "validator-keys-manifest.yaml"
)
HELPER_IDENTITY_PRIVATE_KEY_HEX: Final = (
    "1111111111111111111111111111111111111111111111111111111111111111"
)

logger = logging.getLogger("lean_spec_client_runner")


def setup_logging() -> None:
    logging.basicConfig(
        level=logging.INFO,
        format="%(asctime)s %(levelname)-8s %(name)s: %(message)s",
        datefmt="%Y-%m-%d %H:%M:%S",
    )


def parse_bootnodes() -> list[str]:
    raw_bootnodes = os.environ.get("HIVE_BOOTNODES", "")
    if not raw_bootnodes:
        return []

    return [bootnode for bootnode in raw_bootnodes.split(",") if bootnode]


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
    block_topic = GossipTopic.block(GOSSIP_FORK_DIGEST).to_topic_id()
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
        topic = GossipTopic.attestation_subnet(GOSSIP_FORK_DIGEST, subnet_id).to_topic_id()
        event_source.subscribe_gossip_topic(topic)


async def start_listener_and_gossipsub(
    event_source: LiveNetworkEventSource,
) -> asyncio.Task[None]:
    logger.info("Starting listener on %s", LISTEN_ADDR)
    listener_task = asyncio.create_task(event_source.listen(LISTEN_ADDR), name="lean-spec-listener")
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
    metrics.init(name="lean-spec-client", version="0.0.1")

    node_id = os.environ.get("HIVE_NODE_ID", DEFAULT_NODE_ID)
    genesis = load_genesis()
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

    identity_key = IdentityKeypair.from_bytes(
        Bytes32(bytes.fromhex(HELPER_IDENTITY_PRIVATE_KEY_HEX))
    )
    connection_manager = await QuicConnectionManager.create(identity_key)
    event_source = await LiveNetworkEventSource.create(connection_manager=connection_manager)
    event_source.set_fork_digest(GOSSIP_FORK_DIGEST)
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
            fork_digest=GOSSIP_FORK_DIGEST,
            is_aggregator=is_aggregator,
        )
    )
    api_server = ApiServer(
        config=ApiServerConfig(port=API_PORT),
        store_getter=lambda: node.sync_service.store,
    )

    head_block = node.store.blocks[node.store.head]
    status = Status(
        finalized=node.store.latest_finalized,
        head=Checkpoint(root=node.store.head, slot=head_block.slot),
    )
    event_source.set_status(status)

    await dial_bootnodes(event_source, bootnodes)
    listener_task = await start_listener_and_gossipsub(event_source)
    event_source._running = True

    node_task = asyncio.create_task(
        node.run(install_signal_handlers=False),
        name="lean-spec-node",
    )
    api_task = asyncio.create_task(api_server.run(), name="lean-spec-api")

    try:
        done, pending = await asyncio.wait(
            {node_task, api_task},
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
        listener_task.cancel()
        with suppress(asyncio.CancelledError):
            await listener_task
        with suppress(asyncio.CancelledError):
            await node_task
        with suppress(asyncio.CancelledError):
            await api_task


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
