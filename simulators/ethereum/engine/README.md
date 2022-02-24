# Engine API Test Suite using Consensus Layer Mocking

The Engine API suite runs a set of tests where the execution client is started in Proof of Work mode with a certain TTD (Terminal Total Difficulty). When the TTD is reached, the client switches to produce blocks using the PoS consensus mechanism.

The PoS consensus relies on the Engine API, which receives commands from the consensus clients. This suite mimics a consensus client using a mocking tool to test specific scenarios and combinations of Engine API calls.

Clients with support for the merge are required to run this suite:
 - merge-go-ethereum
 - merge-nethermind


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
      - ...

## Engine API Test Cases

General positive and negative test cases based on the description in https://github.com/ethereum/execution-apis/blob/main/src/engine/specification.md

### Engine API Negative Test Cases:
- Invalid Terminal Block in ForkchoiceUpdated:  
Client should reject ForkchoiceUpdated directives if the referenced HeadBlockHash does not meet the TTD requirement.

- Invalid GetPayload Under PoW:  
Client must reject GetPayload directives under PoW.

- Invalid Terminal Block in NewPayload:  
Client must reject NewPayload directives if the referenced ParentHash does not meet the TTD requirement.

- Unknown HeadBlockHash:  
Perform a forkchoiceUpdated call with an unknown (random) HeadBlockHash, the client should initiate the syncing process.

- Unknown SafeBlockHash:  
Perform a forkchoiceUpdated call with an unknown (random) SafeBlockHash, the client should throw an error since the hash is not an ancestor to the HeadBlockHash.

- Unknown FinalizedBlockHash:  
Perform a forkchoiceUpdated call with an unknown (random) FinalizedBlockHash, the client should throw an error.

- Pre-TTD Block Hash:  
Perform a forkchoiceUpdated call using a block hash part of the canonical chain that precedes the block where the TTD occurred. (Behavior is undefined for this edge case and not verified, but should not produce unrecoverable error)

- Bad blockhash on NewPayload:  
Send a NewPayload directive to the client including an incorrect BlockHash, should result in an error.

- ParentHash==BlockHash on NewPayload:  
Send a NewPayload directive to the client including ParentHash that is equal to the BlockHash (Incorrect hash).

- Invalid Field in NewPayload:  
Modify fields of the ExecutablePayload while maintaining a valid BlockHash, including:
   - ParentHash
   - StateRoot
   - ReceiptsRoot
   - BlockNumber
   - GasLimit
   - GasUsed
   - Timestamp
   - PrevRandao
   - Transaction with incorrect fields:
      - Signature
      - Nonce
      - GasPrice
      - Gas
      - Value

### Eth RPC Status on ForkchoiceUpdated Events:
- Latest Block after NewPayload:  
Verify the Block returned by the Eth RPC after a new payload is executed. Eth RPC should still return previous block.

- Latest Block after New HeadBlock:  
Verify the Block returned by the Eth RPC after a new HeadBlockHash is set using forkchoiceUpdated. Eth RPC should return new block.

- Latest Block after New SafeBlock:  
Verify the Block returned by the Eth RPC after a new SafeBlockHash is set using forkchoiceUpdated. Eth RPC should return new block.

- Latest Block after New FinalizedBlock:  
Verify the Block returned by the Eth RPC after a new FinalizedBlockHash is set using forkchoiceUpdated. Eth RPC should return new block.

- Latest Block after Reorg:  
Verify the Block returned by the Eth RPC after a forkchoiceUpdated reorgs HeadBlockHash/SafeBlockHash to their previous value. Eth RPC should return previous block.

### Payload Execution
- Re-Execute Payload:  
Re-execute already executed payloads (10) and verify that no errors occur.

- Multiple New Payloads Extending Canonical Chain:  
Send 80 valid NewPayload directives extending the canonical chain. Only one of the payloads is selected using ForkchoiceUpdated directive.

- Out of Order Payload Execution:  
Launch a first client and produce N payloads.  
Launch a second client and send payloads (NewPayload) in reverse order (N, N - 1, ..., 1).  
The payloads should be ACCEPTED/SYNCING, and the last payload should be VALID (since payload 1 successfully links the chain with the Genesis).

### Transaction Reorg using Engine API
- Transaction Reorg using ForkchoiceUpdated:  
Send transactions that modify the state tree after the PoS switch and verify that the modifications are correctly rolled back when a ForkchoiceUpdated event occurs with a block older than the block where the transaction was included.

- Sidechain Reorg:  
Send a transaction that modifies the state, ForkchoiceUpdate to the payload containing this transaction. Then send an alternative payload, that produces a different state with the same transaction, and use ForkchoiceUpdated to change into this sidechain.

### Suggested Fee Recipient in Payload creation
- Suggested Fee Recipient Test:  
Set the fee recipient to a custom address and verify that (a) balance is not increased when no fees are collected (b) balance is increased appropriately when fees are collected.

### PrevRandao Opcode:
- PrevRandao Opcode Transactions:  
Send transactions that modify the state to the value of the 'DIFFICULTY' opcode and verify that:  
(a) the state is equal to the difficulty on blocks before the TTD is crossed  
(b) the state is equal to the PrevRandao value provided using forkchoiceUpdated after PoS transition.  

### Sync Tests:
- Sync Client Post Merge:  
Launch a first client and verify that the transition to PoS occurs.  
Launch a second client and verify that it successfully syncs to the first client's chain.

## Engine API Merge Tests:

Test cases using multiple Proof of Work chains to test the client's behavior when the Terminal Total Difficulty is reached by two different blocks simultaneously and the Engine API takes over.

- Single Block PoW Re-org to Higher-Total-Difficulty Chain, Equal Height:  
Client 1 starts with chain G -> A, Client 2 starts with chain G -> B.  
Blocks A and B reach TTD, but block B has higher difficulty than A.  
ForkchoiceUpdated is sent to Client 1 with A as Head.  
ForkchoiceUpdated is sent to Client 1 with B as Head.  
Verification is made that Client 1 Re-orgs to chain G -> B.  

- Single Block PoW Re-org to Lower-Total-Difficulty Chain, Equal Height:  
Client 1 starts with chain G -> A, Client 2 starts with chain G -> B.  
Blocks A and B reach TTD, but block A has higher difficulty than B.  
ForkchoiceUpdated is sent to Client 1 with A as Head.  
ForkchoiceUpdated is sent to Client 1 with B as Head.  
Verification is made that Client 1 Re-orgs to chain G -> B.  

- Two Block PoW Re-org to Higher-Total-Difficulty Chain, Equal Height:  
Client 1 starts with chain G -> A -> B, Client 2 starts with chain G -> C -> D.  
Blocks B and D reach TTD, but block D has higher total difficulty than B.  
ForkchoiceUpdated is sent to Client 1 with B as Head.  
ForkchoiceUpdated is sent to Client 1 with D as Head.  
Verification is made that Client 1 Re-orgs to chain G -> C -> D.  

- Two Block PoW Re-org to Lower-Total-Difficulty Chain, Equal Height:  
Client 1 starts with chain G -> A -> B, Client 2 starts with chain G -> C -> D.  
Blocks B and D reach TTD, but block B has higher total difficulty than D.  
ForkchoiceUpdated is sent to Client 1 with B as Head.  
ForkchoiceUpdated is sent to Client 1 with D as Head.  
Verification is made that Client 1 Re-orgs to chain G -> C -> D.  

- Two Block PoW Re-org to Higher-Height Chain:  
Client 1 starts with chain G -> A, Client 2 starts with chain G -> B -> C.  
Blocks A and C reach TTD, but block C has higher height than A.  
ForkchoiceUpdated is sent to Client 1 with A as Head.  
ForkchoiceUpdated is sent to Client 1 with C as Head.  
Verification is made that Client 1 Re-orgs to chain G -> B -> C.  

- Two Block PoW Re-org to Lower-Height Chain:  
 Client 1 starts with chain G -> A -> B, Client 2 starts with chain G -> C.  
 Blocks B and C reach TTD, but block B has higher height than C.  
 ForkchoiceUpdated is sent to Client 1 with B as Head.  
 ForkchoiceUpdated is sent to Client 1 with C as Head.  
 Verification is made that Client 1 Re-orgs to chain G -> C.  

- Halt following PoW chain:  
 Client 1 starts with chain G -> A, Client 2 starts with chain G -> A -> B.  
 Block A reaches TTD, but Client 2 has a higher TTD and accepts block B (simulating a client not complying with the merge).  
 Verification is made that Client 1 does not follow Client 2 chain to block B.  