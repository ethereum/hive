package main

import (
	"fmt"
	"math/big"
	"math/rand"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/eth/catalyst"
)

// Consensus Layer Client Mock used to sync the Execution Clients once the TTD has been reached
type CLMocker struct {
	// List of Engine Clients being served by the CL Mocker
	EngineClients []*EngineClient

	// Block Production Information
	NextBlockProducer       *EngineClient
	BlockProductionMustStop bool
	NextFeeRecipient        common.Address

	// PoS Chain History Information
	RandomHistory          map[uint64]common.Hash
	ExecutedPayloadHistory map[uint64]catalyst.ExecutableDataV1

	// Latest broadcasted data using the PoS Engine API
	LatestFinalizedNumber *big.Int
	LatestFinalizedHeader *types.Header
	LatestPayloadBuilt    catalyst.ExecutableDataV1
	LatestExecutedPayload catalyst.ExecutableDataV1
	LatestForkchoice      catalyst.ForkchoiceStateV1

	// Merge related
	FirstPoSBlockNumber         *big.Int
	TTDReached                  bool
	PoSBlockProductionActivated bool

	/* Set-Reset-Lock: Use LockSet() to guarantee that the test case finds the
	environment as expected, and not as previously modified by another test.

	The test cases can request a lock to "wake up" at a specific point during
	the PoS block production procedure and, when the CL mocker has reached it,
	the lock is released to let only one test case do its testing.
	*/
	PayloadBuildMutex                SetResetLock
	NewGetPayloadMutex               SetResetLock
	NewExecutePayloadMutex           SetResetLock
	NewHeadBlockForkchoiceMutex      SetResetLock
	NewSafeBlockForkchoiceMutex      SetResetLock
	NewFinalizedBlockForkchoiceMutex SetResetLock
}

func NewCLMocker() *CLMocker {
	// Init random seed for different purposes
	rand.Seed(time.Now().Unix())

	// Create the new CL mocker
	newCLMocker := &CLMocker{
		EngineClients:               make([]*EngineClient, 0),
		RandomHistory:               map[uint64]common.Hash{},
		ExecutedPayloadHistory:      map[uint64]catalyst.ExecutableDataV1{},
		LatestFinalizedHeader:       nil,
		PoSBlockProductionActivated: false,
		BlockProductionMustStop:     false,
		FirstPoSBlockNumber:         nil,
		LatestFinalizedNumber:       nil,
		TTDReached:                  false,
		NextFeeRecipient:            common.Address{},
		LatestForkchoice: catalyst.ForkchoiceStateV1{
			HeadBlockHash:      common.Hash{},
			SafeBlockHash:      common.Hash{},
			FinalizedBlockHash: common.Hash{},
		},
	}

	// Lock the mutexes. To be unlocked on next PoS block production.
	newCLMocker.PayloadBuildMutex.LockReset()
	newCLMocker.NewGetPayloadMutex.LockReset()
	newCLMocker.NewExecutePayloadMutex.LockReset()
	newCLMocker.NewHeadBlockForkchoiceMutex.LockReset()
	newCLMocker.NewSafeBlockForkchoiceMutex.LockReset()
	newCLMocker.NewFinalizedBlockForkchoiceMutex.LockReset()

	// Start timer to check when the TTD has been reached
	time.AfterFunc(tTDCheckPeriod, newCLMocker.checkTTD)

	return newCLMocker
}

// Add a Client to be kept in sync with the latest payloads
func (cl *CLMocker) AddEngineClient(newEngineClient *EngineClient) {
	cl.EngineClients = append(cl.EngineClients, newEngineClient)
}

// Remove a Client to stop sending latest payloads
func (cl *CLMocker) RemoveEngineClient(removeEngineClient *EngineClient) {
	i := -1
	for j := 0; j < len(cl.EngineClients); j++ {
		if cl.EngineClients[j] == removeEngineClient {
			i = j
			break
		}
	}
	if i >= 0 {
		cl.EngineClients[i] = cl.EngineClients[len(cl.EngineClients)-1]
		cl.EngineClients = cl.EngineClients[:len(cl.EngineClients)-1]
	}
}

// Helper struct to fetch the TotalDifficulty
type TD struct {
	TotalDifficulty *hexutil.Big `json:"totalDifficulty"`
}

// Check whether we have reached TTD and then enable PoS block production.
// This function must NOT be executed after we have reached TTD.
func (cl *CLMocker) checkTTD() {
	if len(cl.EngineClients) == 0 {
		// We have no clients running yet, we have not reached TTD
		time.AfterFunc(tTDCheckPeriod, cl.checkTTD)
		return
	}

	// Pick a random client to get the total difficulty of its head
	ec := cl.EngineClients[rand.Intn(len(cl.EngineClients))]

	var td *TD
	err := ec.c.CallContext(ec.Ctx(), &td, "eth_getBlockByNumber", "latest", false)
	if err != nil {
		ec.Fatalf("Could not get latest totalDifficulty: %v", err)
	}
	if td.TotalDifficulty.ToInt().Cmp(terminalTotalDifficulty) >= 0 {
		cl.TTDReached = true
		cl.LatestFinalizedHeader, err = ec.Eth.HeaderByNumber(ec.Ctx(), nil)
		if err != nil {
			ec.Fatalf("Could not get block header: %v", err)
		}
		fmt.Printf("TTD has been reached at block %v\n", cl.LatestFinalizedHeader.Number)
		// Broadcast initial ForkchoiceUpdated
		cl.LatestForkchoice.HeadBlockHash = cl.LatestFinalizedHeader.Hash()
		cl.LatestForkchoice.SafeBlockHash = cl.LatestFinalizedHeader.Hash()
		cl.LatestForkchoice.FinalizedBlockHash = cl.LatestFinalizedHeader.Hash()
		for _, resp := range cl.broadcastForkchoiceUpdated(&cl.LatestForkchoice, nil) {
			if resp.Status != "SUCCESS" {
				fmt.Printf("forkchoiceUpdated Response: %v\n", resp)
			}
		}
		time.AfterFunc(PoSBlockProductionPeriod, cl.minePOSBlock)
		return
	}
	time.AfterFunc(tTDCheckPeriod, cl.checkTTD)
}

// Engine Block Production Methods
func (cl *CLMocker) stopPoSBlockProduction() {
	cl.BlockProductionMustStop = true
}

// Check whether a block number is a PoS block
func (cl *CLMocker) isBlockPoS(bn *big.Int) bool {
	if cl.FirstPoSBlockNumber == nil || cl.FirstPoSBlockNumber.Cmp(bn) > 0 {
		return false
	}
	return true
}

// Sets the fee recipient for the next block and returns the number where it will be included.
func (cl *CLMocker) setNextFeeRecipient(feeRecipient common.Address, ec *EngineClient) *big.Int {
	for {
		cl.PayloadBuildMutex.LockSet()
		if ec == nil || cl.NextBlockProducer == ec {
			cl.NextFeeRecipient = feeRecipient
			defer cl.PayloadBuildMutex.Unlock()
			return big.NewInt(cl.LatestFinalizedNumber.Int64() + 1)
		}
		// Unlock and keep trying to get the requested Engine Client
		cl.PayloadBuildMutex.Unlock()
	}
}

// Unlock all locks in the given CLMocker instance
func unlockAll(cl *CLMocker) {
	cl.PayloadBuildMutex.Unlock()
	cl.NewGetPayloadMutex.Unlock()
	cl.NewExecutePayloadMutex.Unlock()
	cl.NewHeadBlockForkchoiceMutex.Unlock()
	cl.NewSafeBlockForkchoiceMutex.Unlock()
	cl.NewFinalizedBlockForkchoiceMutex.Unlock()
}

// Mine a PoS block by using the Engine API
func (cl *CLMocker) minePOSBlock() {
	if cl.BlockProductionMustStop {
		unlockAll(cl)
		return
	}
	var lastBlockNumber uint64
	var err error
	for {
		// Get a random client to generate the payload
		ec_id := rand.Intn(len(cl.EngineClients))
		cl.NextBlockProducer = cl.EngineClients[ec_id]

		lastBlockNumber, err = cl.NextBlockProducer.Eth.BlockNumber(cl.NextBlockProducer.Ctx())
		if err != nil {
			unlockAll(cl)
			cl.NextBlockProducer.Fatalf("Could not get block number: %v", err)
		}

		latestHeader, err := cl.NextBlockProducer.Eth.HeaderByNumber(cl.NextBlockProducer.Ctx(), big.NewInt(int64(lastBlockNumber)))
		if err != nil {
			unlockAll(cl)
			cl.NextBlockProducer.Fatalf("Could not get block header: %v", err)
		}

		lastBlockHash := latestHeader.Hash()

		if cl.LatestFinalizedHeader.Hash() != lastBlockHash {
			continue
		} else {
			break
		}

	}

	cl.PayloadBuildMutex.Unlock()
	cl.PayloadBuildMutex.LockReset()

	// Generate a random value for the Random field
	nextRandom := common.Hash{}
	for i := 0; i < common.HashLength; i++ {
		nextRandom[i] = byte(rand.Uint32())
	}

	payloadAttributes := catalyst.PayloadAttributesV1{
		Timestamp:             cl.LatestFinalizedHeader.Time + 1,
		Random:                nextRandom,
		SuggestedFeeRecipient: cl.NextFeeRecipient,
	}

	resp, err := cl.NextBlockProducer.EngineForkchoiceUpdatedV1(cl.NextBlockProducer.Ctx(), &cl.LatestForkchoice, &payloadAttributes)
	if err != nil {
		unlockAll(cl)
		cl.NextBlockProducer.Fatalf("Could not send forkchoiceUpdatedV1: %v", err)
	}
	if resp.Status != "SUCCESS" {
		fmt.Printf("forkchoiceUpdated Response: %v\n", resp)
	}

	cl.LatestPayloadBuilt, err = cl.NextBlockProducer.EngineGetPayloadV1(cl.NextBlockProducer.Ctx(), resp.PayloadID)
	if err != nil {
		unlockAll(cl)
		cl.NextBlockProducer.Fatalf("Could not getPayload (%v): %v", resp.PayloadID, err)
	}

	// Trigger actions for a new payload built.
	cl.NewGetPayloadMutex.Unlock()
	cl.NewGetPayloadMutex.LockReset()

	// Broadcast the executePayload to all clients
	for i, resp := range cl.broadcastExecutePayload(&cl.LatestPayloadBuilt) {
		if resp.Status != "VALID" {
			fmt.Printf("resp (%v): %v\n", i, resp)
		}
	}
	cl.LatestExecutedPayload = cl.LatestPayloadBuilt
	cl.ExecutedPayloadHistory[cl.LatestPayloadBuilt.Number] = cl.LatestPayloadBuilt

	// Trigger actions for new executePayload broadcast
	cl.NewExecutePayloadMutex.Unlock()
	cl.NewExecutePayloadMutex.LockReset()

	// Broadcast forkchoice updated with new HeadBlock to all clients
	cl.LatestForkchoice.HeadBlockHash = cl.LatestPayloadBuilt.BlockHash
	for i, resp := range cl.broadcastForkchoiceUpdated(&cl.LatestForkchoice, nil) {
		if resp.Status != "SUCCESS" {
			fmt.Printf("resp (%v): %v\n", i, resp)
		}
	}
	// Trigger actions for new HeadBlock forkchoice broadcast
	cl.NewHeadBlockForkchoiceMutex.Unlock()
	cl.NewHeadBlockForkchoiceMutex.LockReset()

	// Broadcast forkchoice updated with new SafeBlock to all clients
	cl.LatestForkchoice.SafeBlockHash = cl.LatestPayloadBuilt.BlockHash
	for i, resp := range cl.broadcastForkchoiceUpdated(&cl.LatestForkchoice, nil) {
		if resp.Status != "SUCCESS" {
			fmt.Printf("resp (%v): %v\n", i, resp)
		}
	}
	// Trigger actions for new SafeBlock forkchoice broadcast
	cl.NewSafeBlockForkchoiceMutex.Unlock()
	cl.NewSafeBlockForkchoiceMutex.LockReset()

	// Broadcast forkchoice updated with new FinalizedBlock to all clients
	cl.LatestForkchoice.FinalizedBlockHash = cl.LatestPayloadBuilt.BlockHash
	for i, resp := range cl.broadcastForkchoiceUpdated(&cl.LatestForkchoice, nil) {
		if resp.Status != "SUCCESS" {
			fmt.Printf("resp (%v): %v\n", i, resp)
		}
	}

	// Save random value
	cl.RandomHistory[cl.LatestFinalizedHeader.Number.Uint64()+1] = nextRandom

	// Save the number of the first PoS block
	if cl.FirstPoSBlockNumber == nil {
		cl.FirstPoSBlockNumber = big.NewInt(int64(cl.LatestFinalizedHeader.Number.Uint64() + 1))
	}

	// Save the header of the latest block in the PoS chain
	cl.LatestFinalizedNumber = big.NewInt(int64(lastBlockNumber + 1))
	cl.LatestFinalizedHeader, err = cl.NextBlockProducer.Eth.HeaderByNumber(cl.NextBlockProducer.Ctx(), cl.LatestFinalizedNumber)
	if err != nil {
		unlockAll(cl)
		cl.NextBlockProducer.Fatalf("Could not get block header: %v", err)
	}

	// Switch protocol HTTP<>WS for all clients
	for _, ec := range cl.EngineClients {
		ec.SwitchProtocol()
	}

	// Trigger that we have finished producing a block
	cl.NewFinalizedBlockForkchoiceMutex.Unlock()

	// Exit if we need to
	if !cl.BlockProductionMustStop {
		// Lock BlockProducedMutex until we produce a new block
		cl.NewFinalizedBlockForkchoiceMutex.LockReset()
		time.AfterFunc(PoSBlockProductionPeriod, cl.minePOSBlock)
	} else {
		unlockAll(cl)
	}
}

func (cl *CLMocker) broadcastExecutePayload(payload *catalyst.ExecutableDataV1) []catalyst.ExecutePayloadResponse {
	responses := make([]catalyst.ExecutePayloadResponse, len(cl.EngineClients))
	for i, ec := range cl.EngineClients {
		execPayloadResp, err := ec.EngineExecutePayloadV1(ec.Ctx(), payload)
		if err != nil {
			ec.Fatalf("Could not ExecutePayloadV1: %v", err)
		}
		responses[i] = execPayloadResp
	}
	return responses
}

func (cl *CLMocker) broadcastForkchoiceUpdated(fcstate *catalyst.ForkchoiceStateV1, payloadAttr *catalyst.PayloadAttributesV1) []catalyst.ForkChoiceResponse {
	responses := make([]catalyst.ForkChoiceResponse, len(cl.EngineClients))
	for i, ec := range cl.EngineClients {
		fcUpdatedResp, err := ec.EngineForkchoiceUpdatedV1(ec.Ctx(), fcstate, payloadAttr)
		if err != nil {
			ec.Fatalf("Could not ForkchoiceUpdatedV1: %v", err)
		}
		responses[i] = fcUpdatedResp
	}
	return responses
}
