# Engine API Test Suite using Consensus Layer Mocking

The Engine API suite runs a set of tests where the execution client is started in Proof of Work mode with a certain TTD (Terminal Total Difficulty). When the TTD is reached, the client switches to produce blocks using the PoS consensus mechanism.

The PoS consensus relies on the Engine API, which receives commands from the consensus clients. This suite mimics a consensus client using a mocking tool to test specific scenarios and combinations of Engine API calls.

Clients with support for the merge are required to run this suite:
 - merge-go-ethereum


## Architecture

The tests by default are started by running a single execution client, and a single instance of the CL Mocker.

Each test case has a default Terminal Total Difficulty of 0, unless specified otherwise.

The secondary client can be spawned and synced if the test case requires so.

Given the following example,

    hive --client=clientA,clientB --sim=ethereum/engine

tests will have the following execution flow: 

for each testCase:
   - clientA:
      - starts (PoW or PoS, depending on TTD)
      - CL Mocker:
         - starts
         - testCaseN:
            - starts
            - (optional) clientA/clientB:
               - starts
               - stops
            - verifications complete
            - stops
         - stops
      - stops
   - clientB:
      - (Same steps repeated)
      - ..

## Engine API Test Cases

General positive and negative test cases based on the description in https://github.com/ethereum/execution-apis/blob/main/src/engine/specification.md

### Engine API Negative Test Cases:
- Invalid Terminal Block in ForkchoiceUpdated:  
Client should reject ForkchoiceUpdated directives if the referenced HeadBlockHash does not meet the TTD requirement.

- Invalid GetPayload Under PoW:  
Client must reject GetPayload directives under PoW.

- Invalid Terminal Block in NewPayload:  
Client must reject NewPayload directives if the referenced ParentHash does not meet the TTD requirement.

- Inconsistent Head/Safe/Finalized in ForkchoiceState:
Send an inconsistent ForkchoiceState with a known payload that belongs to a side chain as head, safe or finalized:
Having `A: Genesis <- P1 <- P2 <- P3`, `B: Genesis <- P1' <- P2' <- P3'`, 
send `fcU(Head: P3', Safe: P2, Finalized: P1)`, `fcU(Head: P3, Safe: P2', Finalized: P1)`, and `fcU(Head: P3, Safe: P2, Finalized: P1')`

- Unknown HeadBlockHash:  
Perform a forkchoiceUpdated call with an unknown (random) HeadBlockHash, the client should initiate the syncing process.

- Unknown SafeBlockHash:  
Perform a forkchoiceUpdated call with an unknown (random) SafeBlockHash, the client should throw an error since the hash is not an ancestor to the HeadBlockHash.

- Unknown FinalizedBlockHash:  
Perform a forkchoiceUpdated call with an unknown (random) FinalizedBlockHash, the client should throw an error.

- ForkchoiceUpdated Invalid Payload Attributes:
Perform a forkchoiceUpdated call with valid forkchoice but invalid payload attributes.
Expected outcome is that the forkchoiceUpdate proceeds, but the call returns an error.

- Pre-TTD ForkchoiceUpdated After PoS Switch:  
Perform a forkchoiceUpdated call using a block hash part of the canonical chain that precedes the block where the TTD occurred. (Behavior is undefined for this edge case and not verified, but should not produce unrecoverable error)

### Eth RPC Status on ForkchoiceUpdated Events:
- `latest` Block after NewPayload:  
Verify the Block returned by the Eth RPC after a new payload is executed. Eth RPC should still return previous block.

- `latest` Block after New HeadBlockHash:  
Verify the Block returned by the Eth RPC after a new HeadBlockHash is set using forkchoiceUpdated. Eth RPC should return new block.

- `safe` Block after New SafeBlockHash:  
Verify the Block returned by the Eth RPC using the `safe` keyword after a new `SafeBlockHash` is set using `forkchoiceUpdated`. Eth RPC should return block set on the `SafeBlockHash`. When no block has been set as `SafeBlockHash` (`SafeBlockHash==0x00..00`) or PoW mode still, using `safe` shall return error.

- `finalized` Block after New FinalizedBlockHash:  
Verify the Block returned by the Eth RPC using the `finalized` keyword after a new `FinalizedBlockHash` is set using `forkchoiceUpdated`. Eth RPC should return block set on the `FinalizedBlockHash`. When no block has been set as `FinalizedBlockHash` (`FinalizedBlockHash==0x00..00`) or PoW mode still, using `finalized` shall return error.

- `safe`, `finalized` on Canonical Chain:
Verify the Block returned by the Eth RPC using the `finalized`/`safe` keyword after following the canonical chain is correctly updated to the canonical values set by `forkchoiceUpdated` calls.

- `latest` Block after Reorg:  
Verify the Block returned by the Eth RPC after a forkchoiceUpdated reorgs HeadBlockHash/SafeBlockHash to a sidechain and back. Eth RPC should return the appropriate block everytime.

### Payload Execution
- Re-Execute Payload:  
Re-execute already executed payloads (10) and verify that no errors occur.

- Multiple New Payloads Extending Canonical Chain:  
Send 80 valid NewPayload directives extending the canonical chain. Only one of the payloads is selected using ForkchoiceUpdated directive.

- Consecutive Payload Execution:  
Launch a first client and produce N payloads.  
Launch a second client and send payloads (NewPayload) consecutive order skipping ForkchoiceUpdated calls.  
The payloads should be validated correctly.

- Valid NewPayload->ForkchoiceUpdated on Syncing Client:
Skip sending NewPayload to the client, but send the ForkchoiceUpdated to this missing payload, which will send the client to Syncing, then send the valid payload. Response should be either `ACCEPTED` or `SYNCING`.

- NewPayload with Missing ForkchoiceUpdated
Chain `Genesis <- ... <- TB <- P1 <- P2 <- ... <- Pn`, `TB` is a valid terminal block. Secondary EL starts with `TB` as the head. `newPayload(P1)`, `newPayload(P2)`, ... `newPayload(Pn)` are sent to the secondary EL, all responses shall be `{status: VALID, latestValidHash: payload.blockHash}`. `forkchoiceUpdated(head: Pn, safe: Pn-1, finalized:Pn-2)` is sent and verified.

- Payload Build after New Invalid Payload:  
Build on top of the latest valid payload after an invalid payload has been received:
`P <- INV_P`, `newPayload(INV_P)`, `fcU(head: P, payloadAttributes: attrs)`, `getPayload(…)`

- Build Payload with Invalid ChainID Transaction:  
Attempt to produce a payload `P` after a transaction `INV_TX` with an invalid Chain ID was sent to the client using `eth_sendRawTransaction`.
Verify that `INV_TX` is not included in the produced payload `P`.

### Invalid Payload Tests
- Bad Hash on NewPayload:  
Send a NewPayload directive to the client including an incorrect BlockHash, should result in an error in all the following cases:
   - NewPayload while not syncing, on canonical chain
   - NewPayload while not syncing, on side chain
   - NewPayload while syncing, on canonical chain
   - NewPayload while syncing, on side chain

- ParentHash==BlockHash on NewPayload:  
Send a NewPayload directive to the client including ParentHash that is equal to the BlockHash (Incorrect hash).

- Invalid Transition Payload:
Build canonical chain `A: Genesis <- ... <- TB <- P1 <- P2` where TB is a valid terminal block.
Create an invalid transition payload `INV_P1` with parent `TB`.
`newPayload(INV_P1)` must return `{status: INVALID/ACCEPTED, latestValidHash: 0x00..00}`.
`forkchoiceUpdated(head: INV_P1, safe: 0x00..00, finalized: 0x00..00)` must return `{status: INVALID, latestValidHash: 0x00..00}`.

- Invalid Field NewPayload:  
Send an invalid payload in NewPayload by modifying fields of a valid ExecutablePayload while maintaining a valid BlockHash.
After attempting to NewPayload/ForkchoiceUpdated the invalid payload, also attempt to send a valid payload that contains the previously modified invalid payload as parent (should also fail).
Test also has variants with a missing parent payload (client is syncing):
I.e. Skip sending NewPayload to the client, but send the ForkchoiceUpdated to this missing payload, which will send the client to Syncing, then send the invalid payload.
Modify fields including:
   - ParentHash
   - StateRoot
   - ReceiptsRoot
   - BlockNumber
   - GasLimit
   - GasUsed
   - Timestamp
   - PrevRandao
   - Removing a Transaction
   - Transaction with incorrect fields:
      - Signature
      - Nonce
      - GasPrice
      - GasTip
      - Gas
      - Value
      - ChainID

- Invalid Ancestor Re-Org Tests
Attempt to re-org to an unknown side chain which at some point contains an invalid payload.
The side chain is constructed in parallel while the CL Mock builds the canonical chain, but changing the extraData to simply produce a different hash.
At a given point, the side chain invalidates one of the payloads by modifying one of the payload fields (See "Invalid Field NewPayload").
Once the side chain reaches a certain deviation height (N) from the commonAncestor, the CL switches to it by either of the following methods:
a) `newPayload` each of the payloads in the side chain, and the invalid payload shall return `INVALID`
b) Force the main client to obtain the chain via syncing by sending the entire invalid chain to a modified geth node, which is capable of accepting and relaying the invalid payloads.
Method (b) results in the client returning `ACCEPTED` or `SYNCING` on the `newPayload(INV_P)`, but eventually the client must return `INVALID` to the head of the side chain because it was built on top of an invalid payload.
```
commonAncestor◄─▲── P1 ◄─ P2 ◄─ P3 ◄─ ... ◄─ Pn
		          │
		          └── P1' ◄─ P2' ◄─ ... ◄─ INV_P ◄─ ... ◄─ Pn'
```


### Re-org using Engine API
- Transaction Reorg:  
Send transactions that modify the state tree after the PoS switch and verify that the modifications are correctly rolled back when a ForkchoiceUpdated event occurs with a block on a different chain than the block where the transaction was included.

- Sidechain Reorg:  
Send a transaction that modifies the state, ForkchoiceUpdate to the payload containing this transaction. Then send an alternative payload, that produces a different state with the same transaction, and use ForkchoiceUpdated to change into this sidechain.

- Re-Org Back into Canonical Chain:  
Test that performing a re-org back into a previous block of the canonical chain does not produce errors and the chain is still capable of progressing.

- Re-Org Back to Canonical Chain From Syncing Chain:
Build an alternative chain of 10 payloads.
Perform `newPayload(P10')` + `fcU(P10')`, which should result in client `SYNCING`. Verify that the client can re-org back to the canonical chain after sending `newPayload(P11)` + `fcU(P11)`.

- Import and re-org to previously validated payload on a side chain:
Attempt to re-org to one of the sidechain (previously validated) payloads, but not the leaf, and also build a new payload from this sidechain.

- Safe Re-Org to Side Chain
Perform a re-org of the safe block (and head block) to a valid sidechain.

### Suggested Fee Recipient in Payload creation
- Suggested Fee Recipient Test:  
Set the fee recipient to a custom address and verify that (a) balance is not increased when no fees are collected (b) balance is increased appropriately when fees are collected. Test using Legacy and EIP-1559 transaction types.

### PrevRandao Opcode:
- PrevRandao Opcode Transactions:  
Send transactions that modify the state to the value of the 'DIFFICULTY' opcode and verify that:  
(a) the state is equal to the difficulty on blocks before the TTD is crossed  
(b) the state is equal to the PrevRandao value provided using forkchoiceUpdated after PoS transition.  

## Engine API Sync Tests:
- Sync Client Post Merge:  
Launch a first client and verify that the transition to PoS occurs.  
Launch a second client and verify that it successfully syncs to the first client's chain.

## Engine API Transition (Merge) Tests:

Test cases using multiple Proof of Work chains to test the client's behavior when the Terminal Total Difficulty is reached by two different blocks simultaneously and the Engine API takes over.

- Single Block PoW Re-org to Higher-Total-Difficulty Chain, Equal Height:  
   - Client `C1` starts with chain `G <- A`, Client `C2` starts with chain `G <- B`  
   - Blocks `A` and `B` reach `TTD`, but block `B` has higher difficulty than `A`  
   - `forkchoiceUpdated` is sent to `C1` with `A` as Head  
   - `forkchoiceUpdated` is sent to `C1` with `B` as Head  
   - Verify that `C1` Re-orgs to chain `G <- B`  

- Single Block PoW Re-org to Higher-Total-Difficulty Chain, Equal Height (Transition Payload Sync):  
   - Client `C1` starts with chain `G <- A`, Client `C2` starts with chain `G <- B`  
   - Blocks `A` and `B` reach `TTD`, but block `B` has higher difficulty than `A` 
   - `forkchoiceUpdated` is sent to `C1` with `A` as Head  
   - `forkchoiceUpdated` is sent to `C1` with `B` as Head  
   - `P1` is produced by `C2` on top of `G <- B`
   - `newPayload(P1)` is sent to `C1` and verification is made that, after sync, `{status: VALID, latestValidHash: P1.BlockHash}` is returned

- Single Block PoW Re-org to Lower-Total-Difficulty Chain, Equal Height:  
   - Client `C1` starts with chain `G <- A`, Client `C2` starts with chain `G <- B`  
   - Blocks `A` and `B` reach `TTD`, but block `A` has higher difficulty than `B`  
   - `forkchoiceUpdated` is sent to `C1` with `A` as Head  
   - `forkchoiceUpdated` is sent to `C1` with `B` as Head  
   - Verify that `C1` Re-orgs to chain `G <- B`  

- Two Block PoW Re-org to Higher-Total-Difficulty Chain, Equal Height:  
   - Client `C1` starts with chain `G <- A <- B`, Client `C2` starts with chain `G <- C <- D`  
   - Blocks `B` and `D` reach `TTD`, but block `D` has higher total difficulty than `B`  
   - `forkchoiceUpdated` is sent to `C1` with `B` as Head  
   - `forkchoiceUpdated` is sent to `C1` with `D` as Head  
   - Verify that `C1` Re-orgs to chain `G <- C <- D`  

- Two Block PoW Re-org to Lower-Total-Difficulty Chain, Equal Height:  
   - Client `C1` starts with chain `G <- A <- B`, Client `C2` starts with chain `G <- C <- D`  
   - Blocks `B` and `D` reach `TTD`, but block `B` has higher total difficulty than `D`  
   - `forkchoiceUpdated` is sent to `C1` with `B` as Head  
   - `forkchoiceUpdated` is sent to `C1` with `D` as Head  
   - Verify that `C1` Re-orgs to chain `G <- C <- D`  

- Two Block PoW Re-org to Higher-Height Chain:  
   - Client `C1` starts with chain `G <- A`, Client `C2` starts with chain `G <- B <- C`  
   - Blocks `A` and `C` reach `TTD`, but block `C` has higher height than `A`  
   - `forkchoiceUpdated` is sent to `C1` with `A` as Head  
   - `forkchoiceUpdated` is sent to `C1` with `C` as Head  
   - Verify that `C1` Re-orgs to chain `G <- B <- C`  

- Two Block PoW Re-org to Lower-Height Chain:  
   - Client `C1` starts with chain `G <- A <- B`, Client `C2` starts with chain `G <- C`  
   - Blocks `B` and `C` reach `TTD`, but block `B` has higher height than `C`  
   - `forkchoiceUpdated` is sent to `C1` with `B` as Head  
   - `forkchoiceUpdated` is sent to `C1` with `C` as Head  
   - Verify that `C1` Re-orgs to chain `G <- C`  

- Two Block PoW Re-org to Lower-Height Chain, Transaction Overwrite
   - Client `C1` starts with chain `G <- A <- B`, Client `C2` starts with chain `G <- C` 
   - Blocks `B` and `C` reach `TTD`, but block `B` has higher height than `C`  
   - Block `A` and `C` contain `TX1`, and Block `B` contains `TX2`
   - `TX1` and `TX2` use the `DIFFICULTY`/`PREVRANDAO` opcode into storage 
   - `forkchoiceUpdated` is sent to `C1` with `B` as Head  
   - `forkchoiceUpdated` is sent to `C1` with `C` as Head  
   - Verify that `C1` Re-orgs to chain `G <- C`  
   - PoS chain on top of `C` contains `TX2`, which was originally part of the PoW chain
   - Verify that the resulting PoS storage contains `PREVRANDAO` instead of `DIFFICULTY`

- Two Block Post-PoS Re-org to Higher-Total-Difficulty PoW Chain:
   - Client `C1` starts with chain `G <- A`
   - `forkchoiceUpdated` is sent to `C1` with `A` as Head
   - Two PoS blocks are produced on top of `A`
   - Client `C2` starts with chain `G <- B`  
   - Blocks `A` and `B` reach `TTD`, but block `B` has higher difficulty than `A`  
   - `forkchoiceUpdated` is sent to `C1` with `B` as Head  
   - PoS Chain is continued on top of `B`
   - Verify that `C1` Re-orgs to chain `G <- B <- ...`

- Two Block Post-PoS Re-org to Lower-Total-Difficulty PoW Chain:
   - Client `C1` starts with chain `G <- A`
   - `forkchoiceUpdated` is sent to `C1` with `A` as Head
   - Two PoS blocks are produced on top of `A`
   - Client `C2` starts with chain `G <- B`  
   - Blocks `A` and `B` reach `TTD`, but block `A` has higher difficulty than `B`  
   - `forkchoiceUpdated` is sent to `C1` with `B` as Head  
   - PoS Chain is continued on top of `B`
   - Verify that `C1` Re-orgs to chain `G <- B <- ...`

- Two Block Post-PoS Re-org to Higher-Height PoW Chain:
   - Client `C1` starts with chain `G <- A`
   - `forkchoiceUpdated` is sent to `C1` with `A` as Head
   - Two PoS blocks are produced on top of `A`
   - Client `C2` starts with chain `G <- B <- C`  
   - Blocks `A` and `C` reach `TTD`, but have different heights  
   - `forkchoiceUpdated` is sent to `C1` with `C` as Head  
   - PoS Chain is continued on top of `C`
   - Verify that `C1` Re-orgs to chain `G <- B <- C <- ...`  

- Two Block Post-PoS Re-org to Lower-Height PoW Chain:
   - Client `C1` starts with chain `G <- A <- B`
   - `forkchoiceUpdated` is sent to `C1` with B as Head
   - Two PoS blocks are produced on top of B
   - Client `C2` starts with chain `G <- C`  
   - Blocks `B` and `C` reach `TTD`, but have different heights  
   - `forkchoiceUpdated` is sent to `C1` with `C` as Head  
   - PoS Chain is continued on top of `C`
   - Verify that `C1` Re-orgs to chain `G <- C <- ...`  

- Transition to a Chain with Invalid Terminal Block, Higher Configured Total Difficulty
   - Client `C1` starts with chain `G <- A`, Client `C2` starts with chain `G <- A <- B`  
   - `C1`'s configured `TTD` is reached by `A`
   - `C2`'s configured `TTD` is reached by `B`
   - `forkchoiceUpdated` is sent to both clients with `B` as Head  
   - PoS chain with transition payload `P1` is continued on top of `B`
   - Verify that `C1` never re-orgs to chain `G <- A <- B <- P1 <- ...` due to incorrect Terminal block

- Transition to a Chain with Invalid Terminal Block, Higher Configured Total Difficulty (Transition Payload Sync)
   - Client `C1` starts with chain `G <- A`, Client `C2` starts with chain `G <- A <- B`  
   - `C1`'s configured `TTD` is reached by `A`
   - `C2`'s configured `TTD` is reached by `B`
   - `forkchoiceUpdated` is sent to both clients with `B` as Head  
   - `P1` is prepared by `C2` on top of `G <- A <- B`
   - `newPayload(P1)` is sent to `C1` and verification is made that, after syncing, `{status: INVALID, latestValidHash: 0x00..00}` is returned

- Transition to a Chain with Invalid Terminal Block, Lower Configured Total Difficulty
   - Client `C1` starts with chain `G <- A <- B`, Client `C2` starts with chain `G <- A`  
   - `C1`'s configured `TTD` is reached by `B`
   - `C2`'s configured `TTD` is reached by `A`
   - `forkchoiceUpdated` is sent to both clients with `A` as Head  
   - PoS chain with transition payload `P1` is continued on top of `A`
   - Verify that `C1` never re-orgs to chain `G <- A <- P1 <- ...` due to incorrect Terminal block 

- Transition to a Chain with Invalid Terminal Block, Lower Configured Total Difficulty (Transition Payload Sync)
   - Client `C1` starts with chain `G <- A <- B`, Client `C2` starts with chain `G <- A`  
   - `C1`'s configured `TTD` is reached by `B`
   - `C2`'s configured `TTD` is reached by `A`
   - `forkchoiceUpdated` is sent to both clients with `A` as Head  
   - `P1` is prepared by `C2` on top of `G <- A`
   - `newPayload(P1)` is sent to `C1` and verification is made that, after syncing, `{status: INVALID, latestValidHash: 0x00..00}` is returned

- Halt syncing to PoW chain:  
   - Client `C1` starts with chain `G <- A`, Client `C2` starts with chain `G <- A <- B`  
   - Block `A` reaches `TTD`, but `C2` has a higher `TTD` and accepts block `B` (simulating a client not complying with the merge)  
   - Verify that `C1` does not follow `C2` chain to block `B`  

- Long PoW Chain Sync:  
   - Client `C1` starts with chain `G <- PoW1`, Client `C2` starts with chain `G <- PoW1 <- ... <- PoW1024`
   - Block `PoW1024` reaches TTD, and the CL Mock continues the PoS chain on top of this block
   - Verify that `C1` syncs the remaining PoW blocks and also the PoS chain built on top of `PoW1024`

- Stop processing gossiped Post-TTD PoW blocks
   - Clients `C1`, `PoW Producer` and `PoW Receiver` start with `G <- PoW1`
   - `PoW Producer` and `PoW Receiver` are connected only to `C1`, not between each other
   - `PoW Producer` continues mining chain `PoW1 <- PoW2 <- PoW3 <- ... <- PoWN`
   - For `C1` and `PoW Receiver`, `PoW3` reaches `TTD`
   - Verify that `PoW Receiver` receives exactly 2 gossiped new blocks

- Terminal blocks are gossiped
   - Clients `C1`, `PoW Producer` and `PoW Receiver` start with `G <- PoW1`
   - `PoW Producer` and `PoW Receiver` are connected only to `C1`, not between each other
   - `PoW Producer` continues mining chains `PoW1 <- PoW2 <- PoW3 <- PoW4`, `PoW1 <- PoW2 <- PoW3 <- PoW5`, ..., `PoW1 <- PoW2 <- PoW3 <- PoW8`
   - Blocks `PoW4` through `PoW8` are all siblings and all reach TTD
   - Verify that `PoW Receiver` receives exactly 7 gossiped new blocks (`PoW2` through `PoW8`)

- Terminal blocks are gossiped (Common Ancestor Depth 5)
   - Clients `C1`, `PoW Producer` and `PoW Receiver` start with `G <- PoW1`
   - `PoW Producer` and `PoW Receiver` are connected only to `C1`, not between each other
   - `PoW Producer` continues mining chains `PoW1 <- PoW2 <- PoW3 <- PoW4 <- PoW5 <- PoW6` and `PoW1 <- PoW2' <- PoW3' <- PoW4' <- PoW5' <- PoW6'`
   - Blocks `PoW2` and `PoW2'` have same parent `PoW1`, but have different hashes (e.g. different `ethash` seal)
   - Blocks `PoW6` and `PoW6'` reach TTD
   - Verify that `PoW Receiver` receives exactly 10 gossiped new blocks (`PoW2` through `PoW6` and `PoW2'` through `PoW6'`)

- Build Payload After Multiple Terminal blocks via gossip
   - Clients `C1` and `PoW Producer` start with `G <- PoW1`
   - `PoW Producer` continues mining chains `PoW1 <- PoW2 <- PoW3`, `PoW1 <- PoW2 <- PoW4`, ..., `PoW1 <- PoW2 <- PoW7`
   - Blocks `PoW3` through `PoW7` are all siblings and all reach TTD
   - Send `newPayload(P1)` where `P1.parentHash == PoW7` to `C1`
   - Verify that `C1` immediately validates `P1` and returns `VALID`

- Build Payload After Multiple Terminal blocks via gossip (Common Ancestor Depth 5)
   - Clients `C1` and `PoW Producer` start with `G <- PoW1`
   - `PoW Producer` continues mining chains `PoW1 <- PoW2 <- PoW3 <- PoW4 <- PoW5 <- PoW6` and `PoW1 <- PoW2' <- PoW3' <- PoW4' <- PoW5' <- PoW6'`
   - Blocks `PoW2` and `PoW2'` have same parent `PoW1`, but have different hashes (e.g. different `ethash` seal)
   - Blocks `PoW6` and `PoW6'` reach TTD
   - Send `newPayload(P1)` where `P1.parentHash == PoW6'` to `C1`
   - Verify that `C1` immediately validates `P1` and returns `VALID`

- Transition on an Invalid Terminal Execution - Difficulty
   - Clients `C1` and `PoW Producer` start with `G <- PoW1`
   - `PoW Producer` mines and gossips `PoW2`
   - `PoW2` has a `difficulty` value which reaches `TTD`, but it's not the correct expected value for `PoW2` according to `ethash` consensus rules
   - Send `newPayload(P1)` where `P1.parentHash == PoW2` to `C1`
   - Verify that `C1` does not follow `PoS` chain built on top of `P1` and its head still points to `PoW1`

- Transition on an Invalid Terminal Execution - Distant Future
   - Clients `C1` and `PoW Producer` start with `G <- PoW1`
   - `PoW Producer` mines and gossips `PoW2`
   - `PoW2` has a correct `difficulty` value and reaches `TTD`, but its `timestamp` value is 60 seconds into the future
   - Send `newPayload(P1)` where `P1.parentHash == PoW2` to `C1`
   - Verify that `C1` does not follow `PoS` chain built on top of `P1` and its head still points to `PoW1`

- Transition on an Invalid Terminal Execution - Sealed MixHash/Nonce
   - Clients `C1` and `PoW Producer` start with `G <- PoW1`
   - `PoW Producer` mines and gossips `PoW2`
   - `PoW2` has a correct `difficulty` value and reaches `TTD`, but its `mixHash`/`nonce` values are incorrect according to `ethash` consensus rules
   - Send `newPayload(P1)` where `P1.parentHash == PoW2` to `C1`
   - Verify that `C1` does not follow `PoS` chain built on top of `P1` and its head still points to `PoW1`


## Engine API Authentication (JWT) Tests:
- No time drift, correct secret:  
Engine API call where the `iat` claim contains no time drift, and the secret to calculate the token is correct.
No error is expected.

- No time drift, incorrect secret (shorter):  
Engine API call where the `iat` claim contains no time drift, but the secret to calculate the token is incorrectly shorter.
Invalid token error is expected.

- No time drift, incorrect secret (longer):  
Engine API call where the `iat` claim contains no time drift, but the secret to calculate the token is incorrectly longer.
Invalid token error is expected.

- Negative time drift, exceeding limit, correct secret:  
Engine API call where the `iat` claim contains a negative time drift greater than the maximum threshold, but the secret to calculate the token is correct.
Invalid token error is expected.

- Negative time drift, within limit, correct secret:  
Engine API call where the `iat` claim contains a negative time drift smaller than the maximum threshold, and the secret to calculate the token is correct.
No error is expected.

- Positive time drift, exceeding limit, correct secret:  
Engine API call where the `iat` claim contains a positive time drift greater than the maximum threshold, but the secret to calculate the token is correct.
Invalid token error is expected.

- Positive time drift, within limit, correct secret:  
Engine API call where the `iat` claim contains a positive time drift smaller than the maximum threshold, and the secret to calculate the token is correct.
No error is expected.