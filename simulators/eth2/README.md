# Eth2 in Hive

This is *experimental*.

## Configuration

Chain configuration is provided via environment vars, for the clients to build configs in their format of preference.
The configuration (incl. formatting of values) matches the Eth2 config spec ([Example](https://github.com/ethereum/eth2.0-specs/blob/dev/configs/mainnet/phase0.yaml)).

**Each configuration var is prefixed with `HIVE_ETH2_CONFIG_`**.

Only a subset of the configuration is provided, since not every var may be runtime-configurable. Hive requires:

```
CONFIG_NAME
GENESIS_DELAY
SECONDS_PER_SLOT
SLOTS_PER_EPOCH
MIN_GENESIS_ACTIVE_VALIDATOR_COUNT
MIN_GENESIS_TIME
DEPOSIT_CHAIN_ID
DEPOSIT_NETWORK_ID
DEPOSIT_CONTRACT_ADDRESS
GENESIS_FORK_VERSION
GENESIS_DELAY
```

The `CONFIG_NAME` is set to `mainnet`, different compile-time configurations may be introduced when 
Eth2 clients converge on one for Hive testing, to limit docker image builds.

## Container preparation

Note `{}` is used for variable substitution, not part of content.

### Validator Client

#### Files

Keystore files, encrypted. The `pubkey` is lowercase, hex with 0x prefix, of validator signing key.
Lower crypto parameters may be used to speed up tests.
See [eip-2335](https://github.com/ethereum/EIPs/blob/master/EIPS/eip-2335.md) for the keystore format.
```
/hive/input/keystores/{pubkey}/keystore.json
```

Secret for the matching keystore. The `pubkey` is lowercase, hex with 0x prefix, of validator signing key.
Secret files have mode `0600`. The secret file is raw and bytes can be loaded directly.
No special characters are used for ease with text processing.
```
/hive/input/secrets/{pubkey}
```

#### Env vars

The common Eth2 config vars, plus:

```yaml
HIVE_ETH2_BN_API_ADDR: "{ip}:{port}"  # IP and port separated with colon.
                                      # HTTP API address, or can be assumed to be GRPC if using Prysm.
HIVE_ETH2_BN_VENDOR: "{vendor}"  # lowercase name of BN type. E.g. "prysm"
```

### Beacon Node

#### Files

An Eth2 genesis state, encoded with SSZ.
```
/hive/input/genesis.ssz
```

#### Env vars

The common Eth2 config vars, plus:

```
TODO
```