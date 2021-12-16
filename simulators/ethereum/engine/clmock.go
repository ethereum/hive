package main

import (
	"fmt"
	"math/big"
	"math/rand"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/eth/catalyst"
)

// Consensus Layer Client Mock used to sync the Execution Clients once the TTD has been reached
type CLMocker struct {
	EngineClients           []*EngineClient
	RandomHistory           map[uint64]common.Hash
	BlockProductionMustStop bool

	// Latest broadcasted data using the PoS Engine API
	LatestFinalizedNumber *big.Int
	LatestFinalizedHeader *types.Header
	LatestExecutedPayload catalyst.ExecutableDataV1
	LatestForkchoice      catalyst.ForkchoiceStateV1

	// Merge related
	FirstPoSBlockNumber         *big.Int
	TTDReached                  bool
	PoSBlockProductionActivated bool
	NextFeeRecipient            common.Address

	// PoS Status Mutexes: Set-Reset lock is used to guarantee each test finds the environment as expected,
	// and not as previously modified by another test.
	PayloadBuildMutex                SetResetLock
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

	lastBlockNumber, err := ec.Eth.BlockNumber(ec.Ctx())
	if err != nil {
		ec.Fatalf("Could not get block number: %v", err)
	}

	currentTD := big.NewInt(0)
	for i := 0; i <= int(lastBlockNumber); i++ {
		cl.LatestFinalizedHeader, err = ec.Eth.HeaderByNumber(ec.Ctx(), big.NewInt(int64(i)))
		if err != nil {
			ec.Fatalf("Could not get block header: %v", err)
		}
		currentTD.Add(currentTD, cl.LatestFinalizedHeader.Difficulty)
	}

	if currentTD.Cmp(terminalTotalDifficulty) >= 0 {
		cl.TTDReached = true
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
	if cl.FirstPoSBlockNumber == nil {
		return false
	}
	if cl.FirstPoSBlockNumber.Cmp(bn) <= 0 {
		return true
	}
	return false
}

// Sets the fee recipient for the next block and returns the number where it will be included.
func (cl *CLMocker) setNextFeeRecipient(feeRecipient common.Address) *big.Int {
	cl.PayloadBuildMutex.LockSet()
	cl.NextFeeRecipient = feeRecipient
	cl.PayloadBuildMutex.Unlock()
	// Wait until the next block is produced using our feeRecipient
	cl.NewFinalizedBlockForkchoiceMutex.Lock()
	defer cl.NewFinalizedBlockForkchoiceMutex.Unlock()

	// Reset NextFeeRecipient
	cl.NextFeeRecipient = common.Address{}

	return cl.LatestFinalizedNumber
}

// Mine a PoS block by using the Engine API
func (cl *CLMocker) minePOSBlock() {
	if cl.BlockProductionMustStop {
		cl.NewExecutePayloadMutex.Unlock()
		cl.NewHeadBlockForkchoiceMutex.Unlock()
		cl.NewSafeBlockForkchoiceMutex.Unlock()
		cl.NewFinalizedBlockForkchoiceMutex.Unlock()
		return
	}
	var ec *EngineClient
	var lastBlockNumber uint64
	var err error
	for {
		// Get a random client to generate the payload
		ec_id := rand.Intn(len(cl.EngineClients))
		ec = cl.EngineClients[ec_id]

		lastBlockNumber, err = ec.Eth.BlockNumber(ec.Ctx())
		if err != nil {
			cl.NewExecutePayloadMutex.Unlock()
			cl.NewHeadBlockForkchoiceMutex.Unlock()
			cl.NewSafeBlockForkchoiceMutex.Unlock()
			cl.NewFinalizedBlockForkchoiceMutex.Unlock()
			ec.Fatalf("Could not get block number: %v", err)
		}

		latestHeader, err := ec.Eth.HeaderByNumber(ec.Ctx(), big.NewInt(int64(lastBlockNumber)))
		if err != nil {
			cl.NewExecutePayloadMutex.Unlock()
			cl.NewHeadBlockForkchoiceMutex.Unlock()
			cl.NewSafeBlockForkchoiceMutex.Unlock()
			cl.NewFinalizedBlockForkchoiceMutex.Unlock()
			ec.Fatalf("Could not get block header: %v", err)
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

	resp, err := ec.EngineForkchoiceUpdatedV1(ec.Ctx(), &cl.LatestForkchoice, &payloadAttributes)
	if err != nil {
		cl.NewExecutePayloadMutex.Unlock()
		cl.NewHeadBlockForkchoiceMutex.Unlock()
		cl.NewSafeBlockForkchoiceMutex.Unlock()
		cl.NewFinalizedBlockForkchoiceMutex.Unlock()
		ec.Fatalf("Could not send forkchoiceUpdatedV1: %v", err)
	}
	if resp.Status != "SUCCESS" {
		fmt.Printf("forkchoiceUpdated Response: %v\n", resp)
	}

	payload, err := ec.EngineGetPayloadV1(ec.Ctx(), resp.PayloadID)
	if err != nil {
		cl.NewExecutePayloadMutex.Unlock()
		cl.NewHeadBlockForkchoiceMutex.Unlock()
		cl.NewSafeBlockForkchoiceMutex.Unlock()
		cl.NewFinalizedBlockForkchoiceMutex.Unlock()
		ec.Fatalf("Could not getPayload (%v): %v", resp.PayloadID, err)
	}

	// Broadcast the executePayload to all clients
	for i, resp := range cl.broadcastExecutePayload(&payload) {
		if resp.Status != "VALID" {
			fmt.Printf("resp (%v): %v\n", i, resp)
		}
	}
	cl.LatestExecutedPayload = payload

	// Trigger actions for new executePayload broadcast
	cl.NewExecutePayloadMutex.Unlock()
	cl.NewExecutePayloadMutex.LockReset()

	// Broadcast forkchoice updated with new HeadBlock to all clients
	cl.LatestForkchoice.HeadBlockHash = payload.BlockHash
	for i, resp := range cl.broadcastForkchoiceUpdated(&cl.LatestForkchoice, nil) {
		if resp.Status != "SUCCESS" {
			fmt.Printf("resp (%v): %v\n", i, resp)
		}
	}
	// Trigger actions for new HeadBlock forkchoice broadcast
	cl.NewHeadBlockForkchoiceMutex.Unlock()
	cl.NewHeadBlockForkchoiceMutex.LockReset()

	// Broadcast forkchoice updated with new SafeBlock to all clients
	cl.LatestForkchoice.SafeBlockHash = payload.BlockHash
	for i, resp := range cl.broadcastForkchoiceUpdated(&cl.LatestForkchoice, nil) {
		if resp.Status != "SUCCESS" {
			fmt.Printf("resp (%v): %v\n", i, resp)
		}
	}
	// Trigger actions for new SafeBlock forkchoice broadcast
	cl.NewSafeBlockForkchoiceMutex.Unlock()
	cl.NewSafeBlockForkchoiceMutex.LockReset()

	// Broadcast forkchoice updated with new FinalizedBlock to all clients
	cl.LatestForkchoice.FinalizedBlockHash = payload.BlockHash
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
	cl.LatestFinalizedHeader, err = ec.Eth.HeaderByNumber(ec.Ctx(), cl.LatestFinalizedNumber)
	if err != nil {
		cl.NewExecutePayloadMutex.Unlock()
		cl.NewHeadBlockForkchoiceMutex.Unlock()
		cl.NewSafeBlockForkchoiceMutex.Unlock()
		cl.NewFinalizedBlockForkchoiceMutex.Unlock()
		ec.Fatalf("Could not get block header: %v", err)
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
