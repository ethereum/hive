## Hive Configurable Builder API Mock Server

Instantiates a server that listens for Builder API (https://github.com/ethereum/builder-specs/) directives and responds with payloads built using an execution client.

The builder can inject modifications into the built payloads at predefined slots by using configurable callbacks:
- Before sending the ForkchoiceUpdated directive to the execution client, by modifying the payload attributes, using `WithPayloadAttributesModifier` option
- Before responding with the build payload to the consensus client by modifying the any field in the payload, using `WithPayloadModifier` option

Both callbacks are supplied with either the `PayloadAttributesV1`/`PayloadAttributesV2` or the `ExecutionPayloadV1`/`ExecutionPayloadV2` object, and the beacon slot number of the payload request.

The callbacks must respond with a boolean indicating whether any modification was performed, and an error, if any.

The builder can also be configured to insert an error on:
- `/eth/v1/builder/header/{slot}/{parent_hash}/{pubkey}` using `WithErrorOnHeaderRequest` option
- `/eth/v1/builder/blinded_blocks` using `WithErrorOnPayloadReveal` option

Both callbacks are supplied with the beacon slot number of the payload/blinded block request.

The callback can then use the slot number to determine whether to throw an error or not.

Currently, the builder will produce payloads with the following correct fields:
- PrevRandao
- Timestamp
- SuggestedFeeRecipient
- Withdrawals

For the builder to function properly, the following parameters are necessary:
- Execution client: Required to build the payloads
- Beacon client: Required to fetch the state of the previous slot, and calculate, e.g., the prevrandao value
- Beacon Spec: Required so the builder is aware of fork specific changes in the built payloads, as well as the beacon blocks