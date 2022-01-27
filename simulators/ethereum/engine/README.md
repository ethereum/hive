# Engine API Test Suite using Consensus Layer Mocking

The Engine API suite runs a set of tests where the execution client is started in Proof of Work mode with a certain TTD (Terminal Total Difficulty). When the TTD is reached, the client switches to produce blocks using the PoS consensus mechanism.

The PoS consensus relies on the Engine API, which receives commands from the consensus clients. This suite mimics a consensus client using a mocking tool to test specific scenarios and combinations of Engine API calls.

The Consensus Layer mocker switches between HTTP and WS calls throughout the tests' runtime.

Clients with support for the merge are required to run this suite and, at the moment, none of the clients' versions in the docker containers support this functionality. Therefore local modifications to the clients' Dockerfile are required before running this suite to verify using a valid branch/version.

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

## Test Cases

Engine API Negative Test Cases:
- Engine API Proof of Work: Make Engine API calls while the client is still in PoW mode, which should be rejected.
- Unknown HeadBlockHash: Perform a forkchoiceUpdated call with an unknown (random) HeadBlockHash, the client should initiate the syncing process.
- Unknown SafeBlockHash: Perform a forkchoiceUpdated call with an unknown (random) SafeBlockHash, the client should throw an error since the hash is not an ancestor to the HeadBlockHash.
- Unknown FinalizedBlockHash: Perform a forkchoiceUpdated call with an unknown (random) FinalizedBlockHash, the client should initiate the syncing process.
- Pre-TTD Block Hash: Perform a forkchoiceUpdated call using a block hash part of the canonical chain that precedes the block where the TTD occurred. (Behavior is undefined for this edge case and not verified)
- Bad blockhash on ExecutePayload: Send an ExecutePayload directive to the client including an incorrect BlockHash, should result in an error.
- Invalid Field in ExecutePayload: Modify fields of the ExecutablePayload while maintaining a valid BlockHash, including:
   - ParentHash
   - StateRoot
   - ReceiptsRoot
   - BlockNumber
   - GasLimit
   - GasUsed
   - Timestamp

Eth RPC Status on ForkchoiceUpdated Events:
- Latest Block after ExecutePayload: Verify the Block returned by the Eth RPC after a new payload is executed. Eth RPC should still return previous block.
- Latest Block after New HeadBlock: Verify the Block returned by the Eth RPC after a new HeadBlockHash is set using forkchoiceUpdated. Eth RPC should still return previous block.
- Latest Block after New SafeBlock: Verify the Block returned by the Eth RPC after a new SafeBlockHash is set using forkchoiceUpdated. Eth RPC should return new block.
- Latest Block after New FinalizedBlock: Verify the Block returned by the Eth RPC after a new FinalizedBlockHash is set using forkchoiceUpdated. Eth RPC should return new block.
- Latest Block after Reorg: Verify the Block returned by the Eth RPC after a forkchoiceUpdated reorgs HeadBlockHash/SafeBlockHash to their previous value. Eth RPC should return previous block.

Payload Execution
- Re-Execute Payload: Re-execute already executed payloads and verify that no errors occur.

Transactions
- Transaction Reorg using ForkchoiceUpdated: Send transactions that modify the state tree after the PoS switch and verify that the modifications are correctly rolled back when a ForkchoiceUpdated event occurs with a block older than the block where the transaction was included.

Suggested Fee Recipient in Payload creation
- Suggested Fee Recipient Test: Set the fee recipient to a custom address and verify that (a) balance is not increased when no fees are collected (b) balance is increased appropriately when fees are collected.

Random Opcode:
- Random Opcode Transactions: Send transactions that modify the state to the value of the 'DIFFICULTY' opcode and verify that (a) the state is equal to the difficulty on blocks before the TTD is crossed (b) the state is equal to the Random value provided using forkchoiceUpdated after PoS transition.