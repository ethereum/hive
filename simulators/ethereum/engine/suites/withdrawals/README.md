# Shanghai Withdrawals Testing

This test suite verifies behavior of the Engine API on each client after the Shanghai upgrade, according to the specification defined at:
- https://github.com/ethereum/execution-apis/blob/main/src/engine/shanghai.md

## Shanghai Fork
- Genesis Shanghai: Tests the withdrawals fork happening since genesis (e.g. on a testnet).
- Shanghai Fork on Block 1: Tests the withdrawals fork happening directly after genesis.
- Shanghai Fork on Block 2: Tests the transition to the withdrawals fork after a single block has happened. Block 1 is sent with invalid non-null withdrawals payload and client is expected to respond with the appropriate error.
- Shanghai Fork on Block 3: Tests the transition to the withdrawals fork after two blocks have happened. Block 2 is sent with invalid non-null withdrawals payload (both in `engine_newPayloadV2` and the attributes of `engine_forkchoiceUpdatedV2`) and client is expected to respond with the appropriate error.

## Withdrawals
- Withdraw to a single account: Make multiple withdrawals to a single account.
- Withdraw to two accounts: Make multiple withdrawals to two different accounts, repeated in round-robin. Reasoning: There might be a difference in implementation when an account appears multiple times in the withdrawals list but the list is not in ordered sequence.
- Withdraw to many accounts: Make multiple withdrawals to 1024 different accounts. Execute many blocks this way.
- Withdraw zero amount: Make multiple withdrawals where the amount withdrawn is 0.
- Empty Withdrawals: Produce withdrawals block with zero withdrawals.

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
- Withdrawals Fork on Block 1 - 8 Block Re-Org NewPayload


## Max Initcode Tests
- [NOT IMPLEMENTED] Transactions exceeding max initcode should be immediately rejected:
  - Send two transactions with valid nonces each
    - `TxA`, a smart contract creating transaction with an initcode length equal to MAX_INITCODE_SIZE ([EIP-3860](https://eips.ethereum.org/EIPS/eip-3860))
    - `TxB`, a smart contract creating transaction with an initcode length equal to MAX_INITCODE_SIZE+1.
  - Verify that `TxB` returns error on `eth_sendRawTransaction` and also should be absend from the transaction pool of the client
  - Request a new payload from the client and verify that the payload built only includes `TxA`, and `TxB` is not included, nor the contract it could create is present in the `stateRoot`.