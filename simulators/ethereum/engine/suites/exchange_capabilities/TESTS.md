# `engine-exchange-capabilities` - Test Cases


Test Engine API exchange capabilities: https://github.com/ethereum/execution-apis/blob/main/src/engine/common.md#capabilities

## Run Suite

<details>
<summary>Command-line</summary>

```bash
./hive --client <CLIENTS> --sim ethereum/engine --sim.limit "engine-exchange-capabilities/"
```

</details>

## Test Case Categories

- [Shanghai](#category-shanghai)

- [Cancun](#category-cancun)

## Category: Shanghai

### Exchange Capabilities - Shanghai (Active) (Shanghai) (Client)

#### Run

<details>
<summary>Command-line</summary>

```bash
./hive --client <CLIENTS> --sim ethereum/engine --sim.limit "engine-exchange-capabilities/Exchange Capabilities - Shanghai (Active) (Shanghai) (Client)"
```

</details>

#### Description


- Start a node with the Shanghai fork configured at genesis
- Query engine_exchangeCapabilities and verify the capabilities returned
- Capabilities must include the following list:

- engine_newPayloadV1\n
- engine_newPayloadV2\n
- engine_forkchoiceUpdatedV1\n
- engine_forkchoiceUpdatedV2\n
- engine_getPayloadV1\n
- engine_getPayloadV2\n

### Exchange Capabilities - Shanghai (Not active) (Shanghai) (Client)

#### Run

<details>
<summary>Command-line</summary>

```bash
./hive --client <CLIENTS> --sim ethereum/engine --sim.limit "engine-exchange-capabilities/Exchange Capabilities - Shanghai (Not active) (Shanghai) (Client)"
```

</details>

#### Description


- Start a node with the Shanghai fork configured in the future
- Query engine_exchangeCapabilities and verify the capabilities returned
- Capabilities must include the following list, even when the fork is not active yet:

- engine_newPayloadV1\n
- engine_newPayloadV2\n
- engine_forkchoiceUpdatedV1\n
- engine_forkchoiceUpdatedV2\n
- engine_getPayloadV1\n
- engine_getPayloadV2\n

## Category: Cancun

### Exchange Capabilities - Cancun (Active) (Cancun) (Client)

#### Run

<details>
<summary>Command-line</summary>

```bash
./hive --client <CLIENTS> --sim ethereum/engine --sim.limit "engine-exchange-capabilities/Exchange Capabilities - Cancun (Active) (Cancun) (Client)"
```

</details>

#### Description


- Start a node with the Cancun fork configured at genesis
- Query engine_exchangeCapabilities and verify the capabilities returned
- Capabilities must include the following list:

- engine_newPayloadV1\n
- engine_newPayloadV2\n
- engine_newPayloadV3\n
- engine_forkchoiceUpdatedV1\n
- engine_forkchoiceUpdatedV2\n
- engine_forkchoiceUpdatedV3\n
- engine_getPayloadV1\n
- engine_getPayloadV2\n
- engine_getPayloadV3\n

### Exchange Capabilities - Cancun (Not active) (Cancun) (Client)

#### Run

<details>
<summary>Command-line</summary>

```bash
./hive --client <CLIENTS> --sim ethereum/engine --sim.limit "engine-exchange-capabilities/Exchange Capabilities - Cancun (Not active) (Cancun) (Client)"
```

</details>

#### Description


- Start a node with the Cancun fork configured in the future
- Query engine_exchangeCapabilities and verify the capabilities returned
- Capabilities must include the following list, even when the fork is not active yet:

- engine_newPayloadV1\n
- engine_newPayloadV2\n
- engine_newPayloadV3\n
- engine_forkchoiceUpdatedV1\n
- engine_forkchoiceUpdatedV2\n
- engine_forkchoiceUpdatedV3\n
- engine_getPayloadV1\n
- engine_getPayloadV2\n
- engine_getPayloadV3\n

