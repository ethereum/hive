# Withdrawals Interop Hive Simulator

The simulator contains the implementation of the tests described in this
document.


## General Considerations for all tests
- A single validating node comprises an execution client, beacon client and validator client (unless specified otherwise)
- All validating nodes have the same number of validators (unless specified otherwise)
- An importer node is a node that has no validators but might be connected to the network
- Execution client Shanghai timestamp transition is automatically calculated from the Capella Epoch timestamp


## Test Cases

### Capella/Shanghai Transition

* [ ] Capella/Shanghai Transition
  <details>
  <summary>Click for details</summary>
  
  - Start two validating nodes that start on Bellatrix/Merge genesis and transition to Capella/Shanghai on Epoch 1
  - Verify that both nodes reach finality and produce execution blocks after transition
  
  </details>

### Capella/Shanghai Genesis

* [ ] Capella/Shanghai Genesis
  <details>
  <summary>Click for details</summary>
  
  - Start two validating nodes that start on Capella/Shanghai genesis
  - Verify that both nodes progress the chain and produce execution blocks after genesis
  
  </details>

### BLS-To-Execution-Change

* [ ] BLS-To-Execution-Changes
  <details>
  <summary>Click for details</summary>
  
  - Start two validating nodes on either Capella/Shanghai or Bellatrix/Merge genesis (Depending on test case)
  - All genesis validators have BLS withdrawal credentials
  - Sign and submit BLS-To-Execution-Changes of all validators to change withdrawal credentials to different execution addresses
  - Verify on the beacon state that withdrawal credentials are updated
  - Verify on the execution client that all validators partially withdraw (Balances above 1 gwei)

  * [ ] Test on Capella/Shanghai genesis
  * [ ] Test on Bellatrix/Merge genesis, submit BLS-To-Execution-Changes before transition to Capella/Shanghai
  * [ ] Test on Bellatrix/Merge genesis, submit BLS-To-Execution-Changes after transition to Capella/Shanghai

  </details>

* [ ] BLS-To-Execution-Changes of Exited/Slashed Validators
  <details>
  <summary>Click for details</summary>
  
  - Start two validating nodes on Bellatrix/Merge genesis
  - Genesis must contain at least 128 validators
  - All genesis validators have BLS withdrawal credentials
  - Capella/Shanghai transition occurs on Epoch 1
  - During Epoch 0 of the Beacon Chain:
    - Slash 32 validators
    - Exit 32 validators
  - Sign and submit BLS-To-Execution-Changes of all validators to change withdrawal credentials to different execution addresses
  - Verify on the beacon state:
    - Withdrawal credentials are updated
    - Validators' balances drop to zero
  - Verify on the execution client that exited/slashed validators fully withdraw 

  </details>

* [ ] BLS-To-Execution-Changes Broadcast
  <details>
  <summary>Click for details</summary>
  
  - Start two validating nodes and one importer node on either Capella/Shanghai or Bellatrix/Merge genesis (Depending on test case)
  - All genesis validators have BLS withdrawal credentials
  - Sign and submit BLS-To-Execution-Changes to the importer node of all validators to change withdrawal credentials to different execution addresses
  - Verify on the beacon state that withdrawal credentials are updated
  - Verify on the execution client that all validators partially withdraw (Balances above 1 gwei)

  * [ ] Test on Capella/Shanghai genesis
  * [ ] Test on Bellatrix/Merge genesis, submit BLS-To-Execution-Changes before transition to Capella/Shanghai
  * [ ] Test on Bellatrix/Merge genesis, submit BLS-To-Execution-Changes after transition to Capella/Shanghai

  </details>

### Full Withdrawals

* [ ] Full Withdrawal of Exited/Slashed Validators
  <details>
  <summary>Click for details</summary>
  
  - Start two validating nodes on Capella/Shanghai genesis
  - Genesis must contain at least 128 validators
  - All genesis validators have Execution Address credentials
  - During Epoch 0 of the Beacon Chain:
    - Slash 32 validators
    - Exit 32 validators
  - Verify on the beacon state:
    - Validators' balances drop to zero immediatelly on exit/slash (consider max withdrawals per payload)
  - Verify on the execution client that exited/slashed validators fully withdraw 

  </details>

### Partial Withdrawals

* [ ] Partial Withdrawal of Validators
  <details>
  <summary>Click for details</summary>
  
  - Start two validating nodes on Capella/Shanghai genesis
  - All genesis validators have Execution Address credentials
  - Verify on the beacon state:
    - Withdrawal credentials match the expected execution addresses
  - Verify on the execution client that validators are partially withdrawing 

  </details>

### Withdrawals During Re-Orgs

* [ ] Partial Withdrawal of Validators on Re-Org
  <details>
  <summary>Click for details</summary>
  
  - Start three validating nodes on Capella/Shanghai genesis
  - Two nodes, `A` and `B`, are connected to each other, and one node `C` is disconnected from the others
  - All genesis validators have BLS withdrawal credentials
  - On Epoch 0, submit BLS-To-Execution changes to node `C` of all the validating keys contained in this same node
  - Verify that the BLS-To-Execution changes eventually happen on the chain that is being consutructed by this node. Also verify the partial withdrawals on the execution chain
  - Submit BLS-To-Execution changes to nodes `A` and `B` of all the validating keys contained in node `C`, but the execution addresses must differ of the ones originally submitted to node `C`
  - Connect node `C` to nodes `A` and `B`
  - Wait until node `C` re-orgs to chain formed by nodes `A` and `B`
  - Verify on the beacon state `C`:
    - Withdrawal credentials are correctly updated to the execution addresses specified on nodes `A` and `B`
  - Verify on the execution client:
    - Withdrawal addresses specified on node `C` are empty
    - Withdrawal addresses specified on node `A` and `B` are partially withdrawing

  </details>

* [ ] Full Withdrawal of Validators on Re-Org
  <details>
  <summary>Click for details</summary>
  
  - Start three validating nodes on Capella/Shanghai genesis
  - Two nodes, `A` and `B`, are connected to each other, and one node `C` is disconnected from the others
  - Genesis must contain at least 128 validators
  - All genesis validators have BLS withdrawal credentials
  - On Epoch 0, submit voluntary exits of 10 validators from each node to all nodes
  - On Epoch 0, submit BLS-To-Execution changes to node `C` of all the exited validating keys
  - Verify that the BLS-To-Execution changes eventually happen on the chain that is being consutructed by this node. Also verify the full and partial withdrawals on the execution chain
  - Submit BLS-To-Execution changes to nodes `A` and `B` of all the exited validating keys, but the execution addresses must differ of the ones originally submitted to node `C`
  - Connect node `C` to nodes `A` and `B`
  - Wait until node `C` re-orgs to chain formed by nodes `A` and `B`
  - Verify on the beacon state `C`:
    - Withdrawal credentials are correctly updated to the execution addresses specified on nodes `A` and `B`
  - Verify on the execution client:
    - Withdrawal addresses specified on node `C` are empty
    - Withdrawal addresses specified on node `A` and `B` are fully withdrawing

  </details>
