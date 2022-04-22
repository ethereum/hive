package main

import (
	"math/big"
	"math/rand"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/hive/hivesim"
)

// Consensus Layer Client Mock used to sync the Execution Clients once the TTD has been reached
type CLMocker struct {
	*hivesim.T
	// List of Engine Clients being served by the CL Mocker
	EngineClients EngineClients
	// Lock required so no client is offboarded during block production.
	EngineClientsLock sync.Mutex

	// Block Production State
	NextBlockProducer *EngineClient
	NextFeeRecipient  common.Address
	NextPayloadID     *PayloadID

	// PoS Chain History Information
	PrevRandaoHistory      map[uint64]common.Hash
	ExecutedPayloadHistory map[uint64]ExecutableDataV1

	// Latest broadcasted data using the PoS Engine API
	LatestFinalizedNumber *big.Int
	LatestFinalizedHeader *types.Header
	LatestPayloadBuilt    ExecutableDataV1
	LatestExecutedPayload ExecutableDataV1
	LatestForkchoice      ForkchoiceStateV1

	// Merge related
	FirstPoSBlockNumber *big.Int
	TTDReached          bool

	// Global timeout after which all procedures shall stop
	Timeout <-chan time.Time
}

func NewCLMocker(t *hivesim.T, tTD *big.Int) *CLMocker {
	// Init random seed for different purposes
	seed := time.Now().Unix()
	t.Logf("Randomness seed: %v\n", seed)
	rand.Seed(seed)

	// Create the new CL mocker
	newCLMocker := &CLMocker{
		T:                      t,
		EngineClients:          make([]*EngineClient, 0),
		PrevRandaoHistory:      map[uint64]common.Hash{},
		ExecutedPayloadHistory: map[uint64]ExecutableDataV1{},
		LatestFinalizedHeader:  nil,
		FirstPoSBlockNumber:    nil,
		LatestFinalizedNumber:  nil,
		TTDReached:             false,
		NextFeeRecipient:       common.Address{},
		LatestForkchoice: ForkchoiceStateV1{
			HeadBlockHash:      common.Hash{},
			SafeBlockHash:      common.Hash{},
			FinalizedBlockHash: common.Hash{},
		},
	}

	return newCLMocker
}

// Add a Client to be kept in sync with the latest payloads
func (cl *CLMocker) AddEngineClient(t *hivesim.T, c *hivesim.Client, ttd *big.Int) {
	cl.EngineClientsLock.Lock()
	defer cl.EngineClientsLock.Unlock()
	ec := NewEngineClient(t, c, ttd)
	cl.EngineClients = append(cl.EngineClients, ec)
}

// Remove a Client to stop sending latest payloads
func (cl *CLMocker) RemoveEngineClient(c *hivesim.Client) {
	cl.EngineClientsLock.Lock()
	defer cl.EngineClientsLock.Unlock()

	for i, engine := range cl.EngineClients {
		if engine.Client.Container == c.Container {
			cl.EngineClients = append(cl.EngineClients[:i], cl.EngineClients[i+1:]...)
			engine.Close()
		}
	}
}

// Close all the engine clients
func (cl *CLMocker) CloseClients() {
	for _, engine := range cl.EngineClients {
		engine.Close()
	}
}

// Sets the specified client's chain head as Terminal PoW block by sending the initial forkchoiceUpdated.
func (cl *CLMocker) setTTDBlockClient(ec *EngineClient) {
	var td *TotalDifficultyHeader
	if err := ec.cEth.CallContext(ec.Ctx(), &td, "eth_getBlockByNumber", "latest", false); err != nil {
		cl.Fatalf("CLMocker: Could not get latest header: %v", err)
	}
	if td == nil {
		cl.Fatalf("CLMocker: Returned TotalDifficultyHeader is invalid: %v", td)
	}
	cl.Logf("CLMocker: Returned TotalDifficultyHeader: %v", td)

	if td.TotalDifficulty.ToInt().Cmp(ec.TerminalTotalDifficulty) < 0 {
		cl.Fatalf("CLMocker: Attempted to set TTD Block when TTD had not been reached: %v > %v", ec.TerminalTotalDifficulty, td.TotalDifficulty.ToInt())
	}

	cl.LatestFinalizedHeader = &td.Header
	cl.TTDReached = true
	cl.Logf("CLMocker: TTD has been reached at block %v (%v>=%v)\n", cl.LatestFinalizedHeader.Number, td.TotalDifficulty.ToInt(), ec.TerminalTotalDifficulty)

	// Broadcast initial ForkchoiceUpdated
	cl.LatestForkchoice.HeadBlockHash = cl.LatestFinalizedHeader.Hash()
	cl.LatestForkchoice.SafeBlockHash = cl.LatestFinalizedHeader.Hash()
	cl.LatestForkchoice.FinalizedBlockHash = cl.LatestFinalizedHeader.Hash()
	cl.LatestFinalizedNumber = cl.LatestFinalizedHeader.Number
	anySuccess := false
	for _, resp := range cl.broadcastForkchoiceUpdated(&cl.LatestForkchoice, nil) {
		if resp.Error != nil {
			cl.Logf("CLMocker: forkchoiceUpdated Error: %v\n", resp.Error)
		} else {
			if resp.ForkchoiceResponse.PayloadStatus.Status == Valid {
				anySuccess = true
			} else {
				cl.Logf("CLMocker: forkchoiceUpdated Response: %v\n", resp.ForkchoiceResponse)
			}
		}
	}
	if !anySuccess {
		cl.Fatalf("CLMocker: None of the clients accepted forkchoiceUpdated")
	}
}

// This method waits for TTD in at least one of the clients, then sends the
// initial forkchoiceUpdated with the info obtained from the client.
func (cl *CLMocker) waitForTTD() {
	ec := cl.EngineClients.waitForTTD(cl.Timeout)
	if ec == nil {
		cl.Fatalf("CLMocker: Timeout while waiting for TTD")
	}
	// One of the clients has reached TTD, send the initial fcU with this client as head
	cl.setTTDBlockClient(ec)
}

// Check whether a block number is a PoS block
func (cl *CLMocker) isBlockPoS(bn *big.Int) bool {
	if cl.FirstPoSBlockNumber == nil || cl.FirstPoSBlockNumber.Cmp(bn) > 0 {
		return false
	}
	return true
}

// Picks the next payload producer from the set of clients registered
func (cl *CLMocker) pickNextPayloadProducer() {
	if len(cl.EngineClients) == 0 {
		cl.Fatalf("CLMocker: No clients left for block production")
	}

	for i := 0; i < len(cl.EngineClients); i++ {
		// Get a client to generate the payload
		ec_id := (int(cl.LatestFinalizedNumber.Int64()) + i) % len(cl.EngineClients)
		cl.NextBlockProducer = cl.EngineClients[ec_id]

		lastBlockNumber, err := cl.NextBlockProducer.Eth.BlockNumber(cl.NextBlockProducer.Ctx())
		if err != nil {
			cl.Fatalf("CLMocker: Could not get block number while selecting client for payload production (%v): %v", cl.NextBlockProducer.Client.Container, err)
		}

		if cl.LatestFinalizedNumber.Int64() != int64(lastBlockNumber) {
			// Selected client is not synced to the last block number, try again
			cl.NextBlockProducer = nil
			continue
		}

		latestHeader, err := cl.NextBlockProducer.Eth.HeaderByNumber(cl.NextBlockProducer.Ctx(), big.NewInt(int64(lastBlockNumber)))
		if err != nil {
			cl.Fatalf("CLMocker: Could not get block header while selecting client for payload production (%v): %v", cl.NextBlockProducer.Client.Container, err)
		}

		lastBlockHash := latestHeader.Hash()

		if cl.LatestFinalizedHeader.Hash() != lastBlockHash {
			// Selected client latest block hash does not match canonical chain, try again
			cl.NextBlockProducer = nil
			continue
		} else {
			break
		}

	}

	if cl.NextBlockProducer == nil {
		cl.Fatalf("CLMocker: Failed to obtain a client on the latest block number")
	}
}

func (cl *CLMocker) getNextPayloadID() {
	// Generate a random value for the PrevRandao field
	nextPrevRandao := common.Hash{}
	rand.Read(nextPrevRandao[:])

	payloadAttributes := PayloadAttributesV1{
		Timestamp:             cl.LatestFinalizedHeader.Time + 1,
		PrevRandao:            nextPrevRandao,
		SuggestedFeeRecipient: cl.NextFeeRecipient,
	}

	// Save random value
	cl.PrevRandaoHistory[cl.LatestFinalizedHeader.Number.Uint64()+1] = nextPrevRandao

	resp, err := cl.NextBlockProducer.EngineForkchoiceUpdatedV1(cl.NextBlockProducer.Ctx(), &cl.LatestForkchoice, &payloadAttributes)
	if err != nil {
		cl.Fatalf("CLMocker: Could not send forkchoiceUpdatedV1 (%v): %v", cl.NextBlockProducer.Client.Container, err)
	}
	if resp.PayloadStatus.Status != Valid {
		cl.Fatalf("CLMocker: Unexpected forkchoiceUpdated Response from Payload builder: %v", resp)
	}
	cl.NextPayloadID = resp.PayloadID
}

func (cl *CLMocker) getNextPayload() {
	var err error
	cl.LatestPayloadBuilt, err = cl.NextBlockProducer.EngineGetPayloadV1(cl.NextBlockProducer.Ctx(), cl.NextPayloadID)
	if err != nil {
		cl.Fatalf("CLMocker: Could not getPayload (%v, %v): %v", cl.NextBlockProducer.Client.Container, cl.NextPayloadID, err)
	}
}

func (cl *CLMocker) broadcastNextNewPayload() {
	// Broadcast the executePayload to all clients
	for _, resp := range cl.broadcastNewPayload(&cl.LatestPayloadBuilt) {
		if resp.Error != nil {
			cl.Logf("CLMocker: broadcastNewPayload Error (%v): %v\n", resp.Container, resp.Error)

		} else {
			if resp.ExecutePayloadResponse.Status == Valid {
				// The client is synced and the payload was immediately validated
				// https://github.com/ethereum/execution-apis/blob/main/src/engine/specification.md:
				// - If validation succeeds, the response MUST contain {status: VALID, latestValidHash: payload.blockHash}
				if resp.ExecutePayloadResponse.LatestValidHash == nil {
					cl.Fatalf("CLMocker: NewPayload returned VALID status with nil LatestValidHash, expected %v", cl.LatestPayloadBuilt.BlockHash)
				}
				if *resp.ExecutePayloadResponse.LatestValidHash != cl.LatestPayloadBuilt.BlockHash {
					cl.Fatalf("CLMocker: NewPayload returned VALID status with incorrect LatestValidHash==%v, expected %v", resp.ExecutePayloadResponse.LatestValidHash, cl.LatestPayloadBuilt.BlockHash)
				}
			} else if resp.ExecutePayloadResponse.Status == Accepted {
				// The client is not synced but the payload was accepted
				// https://github.com/ethereum/execution-apis/blob/main/src/engine/specification.md:
				// - {status: ACCEPTED, latestValidHash: null, validationError: null} if the following conditions are met:
				// the blockHash of the payload is valid
				// the payload doesn't extend the canonical chain
				// the payload hasn't been fully validated.
				nullHash := common.Hash{}
				if resp.ExecutePayloadResponse.LatestValidHash != nil && *resp.ExecutePayloadResponse.LatestValidHash != nullHash {
					cl.Fatalf("CLMocker: NewPayload returned ACCEPTED status with incorrect LatestValidHash==%v", resp.ExecutePayloadResponse.LatestValidHash)
				}
			} else {
				cl.Logf("CLMocker: broadcastNewPayload Response (%v): %v\n", resp.Container, resp.ExecutePayloadResponse)
			}
		}
	}
	cl.LatestExecutedPayload = cl.LatestPayloadBuilt
	cl.ExecutedPayloadHistory[cl.LatestPayloadBuilt.Number] = cl.LatestPayloadBuilt
}

func (cl *CLMocker) broadcastLatestForkchoice() {
	for _, resp := range cl.broadcastForkchoiceUpdated(&cl.LatestForkchoice, nil) {
		if resp.Error != nil {
			cl.Logf("CLMocker: broadcastForkchoiceUpdated Error (%v): %v\n", resp.Container, resp.Error)
		} else if resp.ForkchoiceResponse.PayloadStatus.Status != Valid {
			cl.Logf("CLMocker: broadcastForkchoiceUpdated Response (%v): %v\n", resp.Container, resp.ForkchoiceResponse)
		}
	}

}

type BlockProcessCallbacks struct {
	OnPayloadProducerSelected           func()
	OnGetPayloadID                      func()
	OnGetPayload                        func()
	OnNewPayloadBroadcast               func()
	OnHeadBlockForkchoiceBroadcast      func()
	OnSafeBlockForkchoiceBroadcast      func()
	OnFinalizedBlockForkchoiceBroadcast func()
}

func (cl *CLMocker) produceSingleBlock(callbacks BlockProcessCallbacks) {

	if !cl.TTDReached {
		cl.Fatalf("CLMocker: Attempted to create a block when the TTD had not yet been reached")
	}

	// Lock needed to ensure EngineClients is not modified mid block production
	cl.EngineClientsLock.Lock()
	defer cl.EngineClientsLock.Unlock()

	cl.pickNextPayloadProducer()

	if callbacks.OnPayloadProducerSelected != nil {
		callbacks.OnPayloadProducerSelected()
	}

	cl.getNextPayloadID()

	if callbacks.OnGetPayloadID != nil {
		callbacks.OnGetPayloadID()
	}

	// Give the client a delay between getting the payload ID and actually retrieving the payload
	time.Sleep(PayloadProductionClientDelay)

	cl.getNextPayload()

	if callbacks.OnGetPayload != nil {
		callbacks.OnGetPayload()
	}

	cl.broadcastNextNewPayload()

	if callbacks.OnNewPayloadBroadcast != nil {
		callbacks.OnNewPayloadBroadcast()
	}

	// Broadcast forkchoice updated with new HeadBlock to all clients
	cl.LatestForkchoice.HeadBlockHash = cl.LatestPayloadBuilt.BlockHash
	cl.broadcastLatestForkchoice()

	if callbacks.OnHeadBlockForkchoiceBroadcast != nil {
		callbacks.OnHeadBlockForkchoiceBroadcast()
	}

	// Broadcast forkchoice updated with new SafeBlock to all clients
	cl.LatestForkchoice.SafeBlockHash = cl.LatestPayloadBuilt.BlockHash
	cl.broadcastLatestForkchoice()

	if callbacks.OnSafeBlockForkchoiceBroadcast != nil {
		callbacks.OnSafeBlockForkchoiceBroadcast()
	}

	// Broadcast forkchoice updated with new FinalizedBlock to all clients
	cl.LatestForkchoice.FinalizedBlockHash = cl.LatestPayloadBuilt.BlockHash
	cl.broadcastLatestForkchoice()

	// Save the number of the first PoS block
	if cl.FirstPoSBlockNumber == nil {
		cl.FirstPoSBlockNumber = big.NewInt(int64(cl.LatestFinalizedHeader.Number.Uint64() + 1))
	}

	// Save the header of the latest block in the PoS chain
	cl.LatestFinalizedNumber = cl.LatestFinalizedNumber.Add(cl.LatestFinalizedNumber, big1)

	// Check if any of the clients accepted the new payload
	cl.LatestFinalizedHeader = nil
	for _, ec := range cl.EngineClients {
		newHeader, err := ec.Eth.HeaderByNumber(cl.NextBlockProducer.Ctx(), cl.LatestFinalizedNumber)
		if err != nil {
			continue
		}
		if newHeader.Hash() != cl.LatestPayloadBuilt.BlockHash {
			continue
		}
		cl.LatestFinalizedHeader = newHeader
	}
	if cl.LatestFinalizedHeader == nil {
		cl.Fatalf("CLMocker: None of the clients accepted the newly constructed payload")
	}

	if callbacks.OnFinalizedBlockForkchoiceBroadcast != nil {
		callbacks.OnFinalizedBlockForkchoiceBroadcast()
	}

}

// Loop produce PoS blocks by using the Engine API
func (cl *CLMocker) produceBlocks(blockCount int, callbacks BlockProcessCallbacks) {
	// Produce requested amount of blocks
	for i := 0; i < blockCount; i++ {
		cl.produceSingleBlock(callbacks)
	}
}

type ExecutePayloadOutcome struct {
	ExecutePayloadResponse *PayloadStatusV1
	Container              string
	Error                  error
}

func (cl *CLMocker) broadcastNewPayload(payload *ExecutableDataV1) []ExecutePayloadOutcome {
	responses := make([]ExecutePayloadOutcome, len(cl.EngineClients))
	for i, ec := range cl.EngineClients {
		responses[i].Container = ec.Container
		execPayloadResp, err := ec.EngineNewPayloadV1(ec.Ctx(), payload)
		if err != nil {
			ec.Errorf("CLMocker: Could not ExecutePayloadV1: %v", err)
			responses[i].Error = err
		} else {
			cl.Logf("CLMocker: Executed payload: %v", execPayloadResp)
			responses[i].ExecutePayloadResponse = &execPayloadResp
		}
	}
	return responses
}

type ForkChoiceOutcome struct {
	ForkchoiceResponse *ForkChoiceResponse
	Container          string
	Error              error
}

func (cl *CLMocker) broadcastForkchoiceUpdated(fcstate *ForkchoiceStateV1, payloadAttr *PayloadAttributesV1) []ForkChoiceOutcome {
	responses := make([]ForkChoiceOutcome, len(cl.EngineClients))
	for i, ec := range cl.EngineClients {
		responses[i].Container = ec.Container
		fcUpdatedResp, err := ec.EngineForkchoiceUpdatedV1(ec.Ctx(), fcstate, payloadAttr)
		if err != nil {
			ec.Errorf("CLMocker: Could not ForkchoiceUpdatedV1: %v", err)
			responses[i].Error = err
		} else {
			responses[i].ForkchoiceResponse = &fcUpdatedResp
		}
	}
	return responses
}
