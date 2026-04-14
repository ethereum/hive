#!/usr/bin/env python3

from __future__ import annotations

import asyncio
import json
import logging
import os
import shutil
import time
from contextlib import suppress
from pathlib import Path
from typing import Final

from aiohttp import web
import yaml
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
from lean_spec.subspecs.ssz.hash import hash_tree_root
from lean_spec.subspecs.validator import ValidatorRegistry
from lean_spec.types import Bytes32

GOSSIP_FORK_DIGEST: Final = "devnet0"
LISTEN_ADDR: Final = "/ip4/0.0.0.0/udp/9001/quic-v1"
API_PORT: Final = 5052
HELPER_METADATA_PORT: Final = 5053
BOOTNODE_DIAL_TIMEOUT_SECS: Final = 10.0
STATUS_REFRESH_INTERVAL_SECS: Final = 1.0
DEFAULT_NODE_ID: Final = "lean_spec_0"
PREPARED_ASSETS_DIR: Final = Path("/tmp/lean-spec-client")
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
HELPER_IDENTITY_PRIVATE_KEY_ENVIRONMENT_VARIABLE: Final = (
    "HIVE_LEAN_HELPER_IDENTITY_PRIVATE_KEY"
)
VALIDATOR_COUNT: Final = 3
ATTESTATION_PUBKEY_FIELD: Final = "attestation_public"
ATTESTATION_SECRET_FIELD: Final = "attestation_secret"
PROPOSAL_PUBKEY_FIELD: Final = "proposal_public"
PROPOSAL_SECRET_FIELD: Final = "proposal_secret"
LEGACY_PUBLIC_FIELD: Final = "public"
LEGACY_SECRET_FIELD: Final = "secret"

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


def parse_validator_indices() -> list[int]:
    raw_indices = os.environ.get("HIVE_LEAN_VALIDATOR_INDICES", "")
    if not raw_indices:
        return []

    return [int(index) for index in raw_indices.split(",") if index]


def devnet_label() -> str:
    return os.environ.get("HIVE_LEAN_DEVNET_LABEL", "devnet3")


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
    genesis = {"GENESIS_TIME": genesis_time, "NUM_VALIDATORS": len(validators)}

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


async def start_metadata_server(metadata: dict[str, object]) -> web.AppRunner:
    async def metadata_handler(_request: web.Request) -> web.Response:
        return web.json_response(metadata)

    app = web.Application()
    app.router.add_get("/hive/genesis", metadata_handler)

    runner = web.AppRunner(app)
    await runner.setup()
    site = web.TCPSite(runner, host="0.0.0.0", port=HELPER_METADATA_PORT)
    await site.start()
    logger.info("Helper metadata server listening on 0.0.0.0:%d", HELPER_METADATA_PORT)
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
    prepare_runtime_assets(node_id)
    genesis = load_genesis()
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

    identity_private_key_hex = os.environ.get(
        HELPER_IDENTITY_PRIVATE_KEY_ENVIRONMENT_VARIABLE,
        HELPER_IDENTITY_PRIVATE_KEY_HEX,
    )
    identity_key = IdentityKeypair.from_bytes(
        Bytes32(bytes.fromhex(identity_private_key_hex))
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
    metadata_runner = await start_metadata_server(metadata)
    refresh_status(event_source, node)

    published_blocks: dict[Bytes32, object] = {}
    if node.validator_service is not None:
        original_on_block = node.validator_service.on_block

        async def cache_and_publish_block(signed_block: object) -> None:
            block_root = hash_tree_root(extract_inner_block(signed_block))
            published_blocks[block_root] = signed_block
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
