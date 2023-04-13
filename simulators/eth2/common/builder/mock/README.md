## Hive Configurable Builder API Mock Server

Instantiates a server that listens for Builder API (https://github.com/ethereum/builder-specs/) directives and responds with payloads built using an execution client.

Currently, the builder will produce payloads with the following correct fields:
- PrevRandao
- Timestamp
- SuggestedFeeRecipient
- Withdrawals

For the builder to function properly, the following parameters are necessary:
- Execution client: Required to build the payloads
- Beacon client: Required to fetch the state of the previous slot, and calculate, e.g., the prevrandao value

## Mock Payload Building
The builder can inject modifications into the built payloads at predefined slots by using configurable callbacks:
- Before sending the ForkchoiceUpdated directive to the execution client, by modifying the payload attributes, using `WithPayloadAttributesModifier` option
- Before responding with the build payload to the consensus client by modifying the any field in the payload, using `WithPayloadModifier` option

Both callbacks are supplied with either the `PayloadAttributesV1`/`PayloadAttributesV2` or the `ExecutionPayloadV1`/`ExecutionPayloadV2` object, and the beacon slot number of the payload request.

The callbacks must respond with a boolean indicating whether any modification was performed, and an error, if any.

Predefined invalidation can also be configured by using `WithPayloadInvalidatorAtEpoch`, `WithPayloadInvalidatorAtSlot`, `WithPayloadAttributesInvalidatorAtEpoch` or `WithPayloadAttributesInvalidatorAtSlot`.

## Payload Invalidation Types
- `state_root`: Inserts a random state root value in the built payload. Payload can only be deemed invalid after the payload has been unblinded
- `parent_hash`: Inserts a random parent hash value in the built payload. Payload can be deemed invalid without needing to unblind
- `coinbase`: Inserts a random address as coinbase in the built payload. Payload is not invalid
- `base_fee`: Increases the base fee value by 1 in the built payload. Payload can only be deemed invalid after the payload has been unblinded
- `uncle_hash`: Inserts a random uncle hash value in the built payload. Payload can be deemed invalid without needing to unblind
- `receipt_hash`: Inserts a random receipt hash value in the built payload. Payload can only be deemed invalid after the payload has been unblinded

## Payload Attributes Invalidation Types
- `remove_withdrawal`: Removes a withdrawal from the correct list of expected withdrawals
- `extra_withdrawal`: Inserts an extra withdrawal to the correct list of expected withdrawals
- `withdrawal_address`, `withdrawal_amount`, `withdrawal_validator_index`, `withdrawal_index`: Invalidates a single withdrawal from the correct list of expected withdrawals
- `timestamp`: Modifies the expected timestamp value of the block (-2 epochs)
- `prevrandao`/`random`: Modifies the expected prevrandao

The builder can also be configured to insert an error on:
- `/eth/v1/builder/header/{slot}/{parent_hash}/{pubkey}` using `WithErrorOnHeaderRequest` option
- `/eth/v1/builder/blinded_blocks` using `WithErrorOnPayloadReveal` option

Both callbacks are supplied with the beacon slot number of the payload/blinded block request.

The callback can then use the slot number to determine whether to throw an error or not.

## Mock Builder REST API
### Mock Error
- `DELETE` `/mock/errors/payload_request`: Disables errors on `/eth/v1/builder/header/...`
- `POST` `/mock/errors/payload_request`: Enables errors on `/eth/v1/builder/header/...`
- `POST` `/mock/errors/payload_request/<slot|epoch>/{slot/epoch number}`: Enables errors on `/eth/v1/builder/header/...` starting at the slot or epoch specified
- `DELETE` `/mock/errors/payload_reveal`: Disables errors on `/eth/v1/builder/blinded_blocks`
- `POST` `/mock/errors/payload_reveal`: Enables errors on `/eth/v1/builder/blinded_blocks`
- `POST` `/mock/errors/payload_reveal/<slot|epoch>/{slot/epoch number}`: Enables errors on `/eth/v1/builder/blinded_blocks` starting at the slot or epoch specified

### Mock Built Payloads
- `DELETE` `/mock/invalid/payload_attributes`: Disables any payload attributes modification
- `POST` `/mock/invalid/payload_attributes/{type}`: Enables specified [type](#payload-attributes-invalidation-types) payload attributes modification
- `POST` `/mock/invalid/payload_attributes/{type}/<slot|epoch>/{slot/epoch number}`: Enables specified [type](#payload-attributes-invalidation-types) payload attributes modification starting at the slot or epoch specified

- `DELETE` `/mock/invalid/payload`: Disables any modification to payload built
- `POST` `/mock/invalid/payload/{type}`: Enables specified [type](#payload-invalidation-types) of modification to payload built
- `POST` `/mock/invalid/payload/{type}/<slot|epoch>/{slot/epoch number}`: Enables specified [type](#payload-invalidation-types) of modification to payload built starting at the slot or epoch specified