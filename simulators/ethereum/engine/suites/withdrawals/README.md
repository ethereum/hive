# Shanghai Withdrawals Testing

This test suite verifies behavior of the Engine API on each client after the Shanghai upgrade, according to the specification defined at:
- https://github.com/ethereum/execution-apis/blob/main/src/engine/shanghai.md

## Shanghai Fork
- Genesis Shanghai: Tests the withdrawals fork happening since genesis (e.g. on a testnet).
- Shanghai Fork on Block 1: Tests the withdrawals fork happening directly after genesis.
- Shanghai Fork on Block 2: Tests the transition to the withdrawals fork after a single block has happened. Block 1 is used to send invalid non-null withdrawals payload.
- Shanghai Fork on Block 3: Tests the transition to the withdrawals fork after two blocks have happened. Block 1 and 2 are used to send invalid non-null withdrawals payload.
- `INVALID` response on corrupted block hash on `engine_newPayloadV2`

All test cases contain following verifications:
- Verify client responds with error -32602 on the following scenarios:
  - Send `ExecutionPayloadV2` using a custom valid (correct BlockHash) execution payload that includes an empty array for withdrawals on `timestamp < SHANGHAI_TIMESTAMP` using `engine_newPayloadV2`
  - Send `PayloadAttributesV2` using an empty array for withdrawals on `timestamp < SHANGHAI_TIMESTAMP` using `engine_forkchoiceUpdatedV2`
  - Send `ExecutionPayloadV1` using a custom valid (correct BlockHash) execution payload that includes `null` as withdrawals on `timestamp >= SHANGHAI_TIMESTAMP` using `engine_newPayloadV2`
  - Send `PayloadAttributesV2` using `null` for withdrawals on `timestamp < SHANGHAI_TIMESTAMP` using `engine_forkchoiceUpdatedV2`
- Use `engine_forkchoiceUpdatedV2` and `engine_newPayloadV2` to send pre-Shanghai payloads/payload attributes, and verify method call succeeds.
- Use `engine_getPayloadV2` to get a pre-Shanghai payload, and verify method call succeeds

## Withdrawals
- Withdraw to a single account: Make multiple withdrawals to a single account.
- Withdraw to two accounts: Make multiple withdrawals to two different accounts, repeated in round-robin. Reasoning: There might be a difference in implementation when an account appears multiple times in the withdrawals list but the list is not in ordered sequence.
- Withdraw to many accounts: Make multiple withdrawals to 1024 different accounts. Execute many blocks this way.
- Withdraw zero amount: Make multiple withdrawals where the amount withdrawn is 0.
- Empty Withdrawals: Produce withdrawals block with zero withdrawals.

All test cases contain the following verifications:
- Verify all withdrawal amounts (sent in Gwei) correctly translate to a wei balance increase in the execution client (after the payload has been set as head using `engine_forkchoiceUpdatedV2`).
- Verify using `eth_getBalance` that the balance of all withdrawn accounts match the expected amount on each block of the chain and `latest`.
- Payload returned by `engine_getPayloadV2` contains the same list of withdrawals as were passed by `PayloadAttributesV2` in the `engine_forkchoiceUpdatedV2` method call.

## Sync to Shanghai
- Sync after 2 blocks - Shanghai on Block 1 - Single Withdrawal Account - No Transactions:
  - Spawn a first client
  - Go through Shanghai fork on Block 1
  - Withdraw to a single account 16 times each block for 2 blocks
  - Spawn a secondary client and send FCUV2(head)
  - Wait for sync and verify withdrawn account's balance
- Sync after 2 blocks - Shanghai on Block 1 - Single Withdrawal Account:
  - Spawn a first client
  - Go through Shanghai fork on Block 1
  - Withdraw to a single account 16 times each block for 2 blocks
  - Spawn a secondary client and send FCUV2(head)
  - Wait for sync and verify withdrawn account's balance
- Sync after 2 blocks - Shanghai on Genesis - Single Withdrawal Account:
  - Spawn a first client with Shanghai on Genesis
  - Withdraw to a single account 16 times each block for 8 blocks
  - Spawn a secondary client and send FCUV2(head)
  - Wait for sync and verify withdrawn account's balance
- Sync after 2 blocks - Shanghai on Block 2 - Multiple Withdrawal Accounts - No Transactions:
  - Spawn a first client
  - Go through Shanghai fork on Block 2
  - Withdraw to 16 accounts each block for 2 blocks
  - Spawn a secondary client and send FCUV2(head)
  - Wait for sync and verify withdrawn account's balance
- Sync after 2 blocks - Shanghai on Block 2 - Multiple Withdrawal Accounts:
  - Spawn a first client
  - Go through Shanghai fork on Block 2
  - Withdraw to 16 accounts each block for 2 blocks
  - Spawn a secondary client and send FCUV2(head)
  - Wait for sync and verify withdrawn account's balance
- Sync after 128 blocks - Withdrawals on Block 2 - Multiple Withdrawal Accounts:
  - Spawn a first client
  - Go through Shanghai fork on Block 2
  - Withdraw to many accounts 16 times each block for 128 blocks
  - Spawn a secondary client and send FCUV2(head)
  - Wait for sync and verify withdrawn account's balance
- [NOT IMPLEMENTED] Error Syncing Null Withdrawals Block after Shanghai:
  - Spawn a first client with configuration `shanghaiTime==2`
  - Send Block 1 with `timestamp==1`
  - Go through Shanghai fork on Block 2 (`timestamp==2`)
  - Withdraw to a single account 16 times each block for 2 blocks
  - Spawn a secondary client with configuration `shanghaiTime==1` and send FCUV2(Block 2)
  - Wait for sync and verify that `engine_newPayload` and `engine_forkchoiceUpdated` return `INVALID` for Block 2.

## Shanghai Fork Re-Org Tests

- Shanghai Fork on Block 1 - 16 Post-Fork Blocks - 1 Block Re-Org via NewPayload:
  - Spawn a two clients `A` and `B`
  - Go through Shanghai fork on Block 1
  - Produce 15 blocks on both clients `A` and `B`, withdrawing to 16 accounts each block
  - Produce a canonical chain `A` block 16 on client `A`, withdrawing to the same 16 accounts
  - Produce a side chain `B` block 16 on client `B`, withdrawing to different 16 accounts
  - Send `NewPayload(B[16])+FcU(B[16])` to client `A`
  - Verify the re-org was correctly applied along with the withdrawal balances.

- Shanghai Fork on Block 1 - 8 Block Re-Org NewPayload
  - Spawn a two clients `A` and `B`
  - Go through Shanghai fork on Block 1
  - Produce 8 blocks on both clients `A` and `B`, withdrawing to 16 accounts each block
  - Produce canonical chain `A` blocks 9-16 on client `A`, withdrawing to the same 16 accounts
  - Produce side chain `B` blocks 9-16 on client `B`, withdrawing to different 16 accounts
  - Send `NewPayload(B[9])+FcU(B[9])..NewPayload(B[16])+FcU(B[16])` to client `A`
  - Verify the re-org was correctly applied along with the withdrawal balances.

- Shanghai Fork on Block 1 - 8 Block Re-Org Sync
  - Spawn a two clients `A` and `B`
  - Go through Shanghai fork on Block 1
  - Produce 8 blocks on both clients `A` and `B`, withdrawing to 16 accounts each block
  - Produce canonical chain `A` blocks 9-16 on client `A`, withdrawing to the same 16 accounts
  - Produce side chain `B` blocks 9-16 on client `B`, withdrawing to different 16 accounts
  - Send `NewPayload(B[16])+FcU(B[16])` to client `A`
  - Verify client `A` syncs side chain blocks from client `B` re-org was correctly applied along with the withdrawal balances.

- Shanghai Fork on Block 8 - 10 Block Re-Org NewPayload
  - Spawn a two clients `A` and `B`
  - Produce 6 blocks on both clients `A` and `B`
  - Produce canonical chain `A` blocks 7-16 on client `A`
  - Produce side chain `B` blocks 7-16 on client `B`
  - Shanghai fork occurs on blocks `A[8]` and `B[8]`
  - Send `NewPayload(B[7])+FcU(B[7])..NewPayload(B[16])+FcU(B[16])` to client `A`
  - Verify the re-org was correctly applied along with the withdrawal balances.

- Shanghai Fork on Block 8 - 10 Block Re-Org Sync
  - Spawn a two clients `A` and `B`
  - Produce 6 blocks on both clients `A` and `B`
  - Produce canonical chain `A` blocks 7-16 on client `A`
  - Produce side chain `B` blocks 7-16 on client `B`
  - Shanghai fork occurs on blocks `A[8]` and `B[8]`
  - Send `NewPayload(B[16])+FcU(B[16])` to client `A`
  - Verify client `A` syncs side chain blocks from client `B` re-org was correctly applied along with the withdrawal balances.

- Shanghai Fork on Canonical Block 8 / Side Block 7 - 10 Block Re-Org NewPayload
  - Spawn a two clients `A` and `B`
  - Produce 6 blocks on both clients `A` and `B`
  - Produce canonical chain `A` blocks 7-16 on client `A` with timestamp increments of 1.
  - Produce side chain `B` blocks 7-16 on client `B` with timestamp increments of 2
  - Shanghai fork occurs on blocks `A[8]` and `B[7]`
  - Send `NewPayload(B[7])+FcU(B[7])..NewPayload(B[16])+FcU(B[16])` to client `A`
  - Verify the re-org was correctly applied along with the withdrawal balances.

- Shanghai Fork on Canonical Block 8 / Side Block 7 - 10 Block Re-Org Sync
  - Spawn a two clients `A` and `B`
  - Produce 6 blocks on both clients `A` and `B`
  - Produce canonical chain `A` blocks 7-16 on client `A` with timestamp increments of 1.
  - Produce side chain `B` blocks 7-16 on client `B` with timestamp increments of 2
  - Shanghai fork occurs on blocks `A[8]` and `B[7]`
  - Send `NewPayload(B[16])+FcU(B[16])` to client `A`
  - Verify client `A` syncs side chain blocks from client `B` re-org was correctly applied along with the withdrawal balances.

- Shanghai Fork on Canonical Block 8 / Side Block 9 - 10 Block Re-Org NewPayload
  - Spawn a two clients `A` and `B`
  - Produce 6 blocks on both clients `A` and `B`, with timestamp increments of 2
  - Produce canonical chain `A` blocks 7-16 on client `A` with timestamp increments of 2.
  - Produce side chain `B` blocks 7-16 on client `B` with timestamp increments of 1
  - Shanghai fork occurs on blocks `A[8]` and `B[9]`
  - Send `NewPayload(B[7])+FcU(B[7])..NewPayload(B[16])+FcU(B[16])` to client `A`
  - Verify the re-org was correctly applied along with the withdrawal balances.

- Shanghai Fork on Canonical Block 8 / Side Block 9 - 10 Block Re-Org Sync
  - Spawn a two clients `A` and `B`
  - Produce 6 blocks on both clients `A` and `B`, with timestamp increments of 2
  - Produce canonical chain `A` blocks 7-16 on client `A` with timestamp increments of 2.
  - Produce side chain `B` blocks 7-16 on client `B` with timestamp increments of 1
  - Shanghai fork occurs on blocks `A[8]` and `B[9]`
  - Send `NewPayload(B[16])+FcU(B[16])` to client `A`
  - Verify client `A` syncs side chain blocks from client `B` re-org was correctly applied along with the withdrawal balances.

## Max Initcode Tests
- Transactions exceeding max initcode should be immediately rejected:
  - Send two transactions with valid nonces each
    - `TxA`, a smart contract creating transaction with an initcode length equal to MAX_INITCODE_SIZE ([EIP-3860](https://eips.ethereum.org/EIPS/eip-3860))
    - `TxB`, a smart contract creating transaction with an initcode length equal to MAX_INITCODE_SIZE+1.
  - Verify that `TxB` returns error on `eth_sendRawTransaction` and also should be absent from the transaction pool of the client
  - Request a new payload from the client and verify that the payload built only includes `TxA`, and `TxB` is not included, nor the contract it could create is present in the `stateRoot`.
  - Create a modified payload where `TxA` is replaced by `TxB` and send using `engine_newPayloadV2`
  - Verify that `engine_newPayloadV2` returns `INVALID` nad `latestValidHash` points to the latest valid payload in the canonical chain.

## GetPayloadBodies Tests

- Payload Bodies By Range - Shanghai Fork on Block 16 - 16 Withdrawal Blocks
  - Launch client `A` and create a canonical chain consisting of 32 blocks, where the first shanghai block is number 17
  - Payloads produced of the following characteristics
    - [x] 16 Transactions, 16 Withdrawals
    - [x] 0 Transactions, 0 Withdrawals
  - Send extra payloads `32'` and `33'` such that `31 <- 32' <- 33'` using `engine_newPayloadV2` 
  - Make multiple requests to obtain the payload bodies from the canonical chain (see `./tests.go` for full list).
  - Verify that:
    - Payload bodies of blocks before the Shanghai fork contain `withdrawals==null`
    - All transactions and withdrawals are in the correct format and order.
    - Requested payload bodies past the highest known block are ignored and absent from the returned list
    - Payloads `32'` and `33'` are ignored by all requests since they are not part of the canonical chain.

- Payload Bodies By Hash/Range After Sync - Shanghai Fork on Block 16 - 16 Withdrawal Blocks
  - Launch client `A` and create a canonical chain consisting of 32 blocks, where the first shanghai block is number 17
  - Payloads produced have: 16 Transactions, 16 Withdrawals
  - Launch client `B` and send `NewPayload(P32)` + `FcU(P32)` to it.
  - Wait until client `B` syncs the canonical chain, or timeout.
  - Make multiple requests to obtain the payload bodies from the canonical chain (see `./tests.go` for full list) to client `B`.
  - Verify that:
    - Payload bodies of blocks before the Shanghai fork contain `withdrawals==null`
    - All transactions and withdrawals are in the correct format and order.
    - Requested payload bodies past the highest known block are ignored and absent from the returned list

- Payload Bodies By Hash - Shanghai Fork on Block 16 - 16 Withdrawal Blocks
  - Launch client `A` and create a canonical chain consisting of 32 blocks, where the first shanghai block is number 17
  - Payloads produced of the following characteristics
    - [x] 16 Transactions, 16 Withdrawals
    - [x] 0 Transactions, 0 Withdrawals
  - Make multiple requests to obtain the payload bodies from the canonical chain (see `./tests.go` for full list).
  - Verify that:
    - Payload bodies of blocks before the Shanghai fork contain `withdrawals==null`
    - All transactions and withdrawals are in the correct format and order.
    - Requested payload bodies of unknown hashes are returned as null in the returned list

## Block Value Tests
- Block Value on GetPayloadV2 Post-Shanghai
  - Create a Shanghai chain where the fork transition happens at block 1
  - Send transactions, submit forkchoice and get payload built
  - Verify transactions were included in payload created
  - Set forkchoice head to the new payload
  - Calculate the block value by requesting the transaction receipts
  - Verify that the `blockValue` returned by `engine_getPayloadV2` matches the expected calculated value
