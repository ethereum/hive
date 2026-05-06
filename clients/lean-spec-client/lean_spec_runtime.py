#!/usr/bin/env python3

from __future__ import annotations

from typing import Any

try:
    from lean_spec.forks import DEFAULT_REGISTRY
except ModuleNotFoundError:
    DEFAULT_REGISTRY = None

try:
    from lean_spec.subspecs.containers import Checkpoint
    from lean_spec.subspecs.containers.validator import SubnetId, ValidatorIndex
except (ImportError, ModuleNotFoundError):
    try:
        from lean_spec.types import Checkpoint, SubnetId, ValidatorIndex
    except (ImportError, ModuleNotFoundError):
        from lean_spec.forks.lstar.containers import Checkpoint
        from lean_spec.forks.lstar.containers.validator import SubnetId, ValidatorIndex


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
        if DEFAULT_REGISTRY is None:
            raise RuntimeError("LeanSpec NodeConfig requires a fork, but no fork registry exists")
        config["fork"] = DEFAULT_REGISTRY.current
        config["network_name"] = gossip_network_name(fork_digest)
    else:
        config["fork_digest"] = fork_digest

    return node_config_type(**config)
