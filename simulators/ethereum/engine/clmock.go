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

var (
	DefaultSlotsToSafe      = big.NewInt(1)
	DefaultSlotsToFinalized = big.NewInt(2)
)

// Consensus Layer Client Mock used to sync the Execution Clients once the TTD has been reached
type CLMocker struct {
	*hivesim.T
	// List of Engine Clients being served by the CL Mocker
	EngineClients EngineClients
	// Lock required so no client is offboarded during block production.
	EngineClientsLock sync.Mutex
	// Number of required slots before a block which was set as Head moves to `safe` and `finalized` respectively
	SlotsToSafe      *big.Int
	SlotsToFinalized *big.Int

	// Block Production State
	NextBlockProducer *EngineClient
	NextFeeRecipient  common.Address
	NextPayloadID     *PayloadID

	// PoS Chain History Information
	PrevRandaoHistory      map[uint64]common.Hash
	ExecutedPayloadHistory map[uint64]ExecutableDataV1
	HeadHashHistory        []common.Hash

	// Latest broadcasted data using the PoS Engine API
	LatestHeadNumber        *big.Int
	LatestHeader            *types.Header
	LatestPayloadBuilt      ExecutableDataV1
	LatestPayloadAttributes PayloadAttributesV1
	LatestExecutedPayload   ExecutableDataV1
	LatestForkchoice        ForkchoiceStateV1

	// Merge related
	FirstPoSBlockNumber *big.Int
	TTDReached          bool

	// Global timeout after which all procedures shall stop
	Timeout <-chan time.Time
}

func NewCLMocker(t *hivesim.T, slotsToSafe *big.Int, slotsToFinalized *big.Int) *CLMocker {
	// Init random seed for different purposes
	seed := time.Now().Unix()
	t.Logf("Randomness seed: %v\n", seed)
	rand.Seed(seed)

	if slotsToSafe == nil {
		// Use default
		slotsToSafe = DefaultSlotsToSafe
	}
	if slotsToFinalized == nil {
		// Use default
		slotsToFinalized = DefaultSlotsToFinalized
	}
	// Create the new CL mocker
	newCLMocker := &CLMocker{
		T:                      t,
		EngineClients:          make([]*EngineClient, 0),
		PrevRandaoHistory:      map[uint64]common.Hash{},
		ExecutedPayloadHistory: map[uint64]ExecutableDataV1{},
		SlotsToSafe:            slotsToSafe,
		SlotsToFinalized:       slotsToFinalized,
		LatestHeader:           nil,
		FirstPoSBlockNumber:    nil,
		LatestHeadNumber:       nil,
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

	cl.LatestHeader = &td.Header
	cl.TTDReached = true
	cl.Logf("CLMocker: TTD has been reached at block %v (%v>=%v)\n", cl.LatestHeader.Number, td.TotalDifficulty.ToInt(), ec.TerminalTotalDifficulty)

	// Reset transition values
	cl.LatestHeadNumber = cl.LatestHeader.Number
	cl.HeadHashHistory = []common.Hash{cl.LatestHeader.Hash()}
	cl.FirstPoSBlockNumber = nil

	// Prepare initial forkchoice
	cl.LatestForkchoice = ForkchoiceStateV1{}
	cl.LatestForkchoice.HeadBlockHash = cl.LatestHeader.Hash()
	if cl.SlotsToSafe.Cmp(big0) == 0 {
		cl.LatestForkchoice.SafeBlockHash = cl.LatestHeader.Hash()
	}
	if cl.SlotsToFinalized.Cmp(big0) == 0 {
		cl.LatestForkchoice.FinalizedBlockHash = cl.LatestHeader.Hash()
	}

	// Broadcast initial ForkchoiceUpdated
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
		ec_id := (int(cl.LatestHeadNumber.Int64()) + i) % len(cl.EngineClients)
		cl.NextBlockProducer = cl.EngineClients[ec_id]

		// Get latest header. Number and hash must coincide with our view of the chain,
		// and only then we can build on top of this client's chain
		latestHeader, err := cl.NextBlockProducer.Eth.HeaderByNumber(cl.NextBlockProducer.Ctx(), nil)
		if err != nil {
			cl.Fatalf("CLMocker: Could not get latest block header while selecting client for payload production (%v): %v", cl.NextBlockProducer.Client.Container, err)
		}

		lastBlockHash := latestHeader.Hash()

		if cl.LatestHeader.Hash() != lastBlockHash || cl.LatestHeadNumber.Cmp(latestHeader.Number) != 0 {
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

	cl.LatestPayloadAttributes = PayloadAttributesV1{
		Timestamp:             cl.LatestHeader.Time + 1,
		PrevRandao:            nextPrevRandao,
		SuggestedFeeRecipient: cl.NextFeeRecipient,
	}

	// Save random value
	cl.PrevRandaoHistory[cl.LatestHeader.Number.Uint64()+1] = nextPrevRandao

	resp, err := cl.NextBlockProducer.EngineForkchoiceUpdatedV1(cl.NextBlockProducer.Ctx(), &cl.LatestForkchoice, &cl.LatestPayloadAttributes)
	if err != nil {
		cl.Fatalf("CLMocker: Could not send forkchoiceUpdatedV1 (%v): %v", cl.NextBlockProducer.Client.Container, err)
	}
	if resp.PayloadStatus.Status != Valid {
		cl.Fatalf("CLMocker: Unexpected forkchoiceUpdated Response from Payload builder: %v", resp)
	}
	if resp.PayloadStatus.LatestValidHash == nil || *resp.PayloadStatus.LatestValidHash != cl.LatestForkchoice.HeadBlockHash {
		cl.Fatalf("CLMocker: Unexpected forkchoiceUpdated LatestValidHash Response from Payload builder: %v != %v", resp.PayloadStatus.LatestValidHash, cl.LatestForkchoice.HeadBlockHash)
	}
	cl.NextPayloadID = resp.PayloadID
}

func (cl *CLMocker) getNextPayload() {
	var err error
	cl.LatestPayloadBuilt, err = cl.NextBlockProducer.EngineGetPayloadV1(cl.NextBlockProducer.Ctx(), cl.NextPayloadID)
	if err != nil {
		cl.Fatalf("CLMocker: Could not getPayload (%v, %v): %v", cl.NextBlockProducer.Client.Container, cl.NextPayloadID, err)
	}
	if cl.LatestPayloadBuilt.Timestamp != cl.LatestPayloadAttributes.Timestamp {
		cl.Fatalf("CLMocker: Incorrect Timestamp on payload built: %d != %d", cl.LatestPayloadBuilt.Timestamp, cl.LatestPayloadAttributes.Timestamp)
	}
	if cl.LatestPayloadBuilt.FeeRecipient != cl.LatestPayloadAttributes.SuggestedFeeRecipient {
		cl.Fatalf("CLMocker: Incorrect SuggestedFeeRecipient on payload built: %v != %v", cl.LatestPayloadBuilt.FeeRecipient, cl.LatestPayloadAttributes.SuggestedFeeRecipient)
	}
	if cl.LatestPayloadBuilt.PrevRandao != cl.LatestPayloadAttributes.PrevRandao {
		cl.Fatalf("CLMocker: Incorrect PrevRandao on payload built: %v != %v", cl.LatestPayloadBuilt.PrevRandao, cl.LatestPayloadAttributes.PrevRandao)
	}
	if cl.LatestPayloadBuilt.ParentHash != cl.LatestHeader.Hash() {
		cl.Fatalf("CLMocker: Incorrect ParentHash on payload built: %v != %v", cl.LatestPayloadBuilt.ParentHash, cl.LatestHeader.Hash())
	}
	if cl.LatestPayloadBuilt.Number != cl.LatestHeader.Number.Uint64()+1 {
		cl.Fatalf("CLMocker: Incorrect Number on payload built: %v != %v", cl.LatestPayloadBuilt.Number, cl.LatestHeader.Number.Uint64()+1)
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
				if resp.ExecutePayloadResponse.LatestValidHash != nil && *resp.ExecutePayloadResponse.LatestValidHash != (common.Hash{}) {
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
		} else if resp.ForkchoiceResponse.PayloadStatus.Status == Valid {
			// {payloadStatus: {status: VALID, latestValidHash: forkchoiceState.headBlockHash, validationError: null},
			//  payloadId: null}
			if *resp.ForkchoiceResponse.PayloadStatus.LatestValidHash != cl.LatestForkchoice.HeadBlockHash {
				cl.Fatalf("CLMocker: Incorrect LatestValidHash from ForkchoiceUpdated (%v): %v != %v\n", resp.Container, resp.ForkchoiceResponse.PayloadStatus.LatestValidHash, cl.LatestForkchoice.HeadBlockHash)
			}
			if resp.ForkchoiceResponse.PayloadStatus.ValidationError != nil {
				cl.Fatalf("CLMocker: Expected empty validationError: %s\n", resp.Container, *resp.ForkchoiceResponse.PayloadStatus.ValidationError)
			}
			if resp.ForkchoiceResponse.PayloadID != nil {
				cl.Fatalf("CLMocker: Expected empty PayloadID: %v\n", resp.Container, resp.ForkchoiceResponse.PayloadID)
			}
		} else if resp.ForkchoiceResponse.PayloadStatus.Status != Valid {
			cl.Logf("CLMocker: broadcastForkchoiceUpdated Response (%v): %v\n", resp.Container, resp.ForkchoiceResponse)
		}
	}

}

type BlockProcessCallbacks struct {
	OnPayloadProducerSelected func()
	OnGetPayloadID            func()
	OnGetPayload              func()
	OnNewPayloadBroadcast     func()
	OnForkchoiceBroadcast     func()
	OnSafeBlockChange         func()
	OnFinalizedBlockChange    func()
	OnVerificationEnd         func()
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
	previousForkchoice := cl.LatestForkchoice
	cl.HeadHashHistory = append(cl.HeadHashHistory, cl.LatestPayloadBuilt.BlockHash)

	cl.LatestForkchoice = ForkchoiceStateV1{}
	cl.LatestForkchoice.HeadBlockHash = cl.LatestPayloadBuilt.BlockHash
	if len(cl.HeadHashHistory) > int(cl.SlotsToSafe.Int64()) {
		cl.LatestForkchoice.SafeBlockHash = cl.HeadHashHistory[len(cl.HeadHashHistory)-int(cl.SlotsToSafe.Int64())-1]
	}
	if len(cl.HeadHashHistory) > int(cl.SlotsToFinalized.Int64()) {
		cl.LatestForkchoice.FinalizedBlockHash = cl.HeadHashHistory[len(cl.HeadHashHistory)-int(cl.SlotsToFinalized.Int64())-1]
	}
	cl.broadcastLatestForkchoice()

	if callbacks.OnForkchoiceBroadcast != nil {
		callbacks.OnForkchoiceBroadcast()
	}

	// Broadcast forkchoice updated with new SafeBlock to all clients
	if callbacks.OnSafeBlockChange != nil && previousForkchoice.SafeBlockHash != cl.LatestForkchoice.SafeBlockHash {
		callbacks.OnSafeBlockChange()
	}
	// Broadcast forkchoice updated with new FinalizedBlock to all clients
	if callbacks.OnFinalizedBlockChange != nil && previousForkchoice.FinalizedBlockHash != cl.LatestForkchoice.FinalizedBlockHash {
		callbacks.OnFinalizedBlockChange()
	}

	// Save the number of the first PoS block
	if cl.FirstPoSBlockNumber == nil {
		cl.FirstPoSBlockNumber = big.NewInt(int64(cl.LatestHeader.Number.Uint64() + 1))
	}

	// Save the header of the latest block in the PoS chain
	cl.LatestHeadNumber = cl.LatestHeadNumber.Add(cl.LatestHeadNumber, big1)

	// Check if any of the clients accepted the new payload
	cl.LatestHeader = nil
	for _, ec := range cl.EngineClients {
		newHeader, err := ec.Eth.HeaderByNumber(cl.NextBlockProducer.Ctx(), cl.LatestHeadNumber)
		if err != nil {
			continue
		}
		if newHeader.Hash() != cl.LatestPayloadBuilt.BlockHash {
			continue
		}
		// Check that the new finalized header has the correct properties
		// ommersHash == 0x1dcc4de8dec75d7aab85b567b6ccd41ad312451b948a7413f0a142fd40d49347
		if newHeader.UncleHash != types.EmptyUncleHash {
			cl.Fatalf("CLMocker: Client %v produced a new header with incorrect ommersHash: %v", ec.Container, newHeader.UncleHash)
		}
		// difficulty == 0
		if newHeader.Difficulty.Cmp(big0) != 0 {
			cl.Fatalf("CLMocker: Client %v produced a new header with incorrect difficulty: %v", ec.Container, newHeader.Difficulty)
		}
		// mixHash == prevRandao
		if newHeader.MixDigest != cl.PrevRandaoHistory[cl.LatestHeadNumber.Uint64()] {
			cl.Fatalf("CLMocker: Client %v produced a new header with incorrect mixHash: %v != %v", ec.Container, newHeader.MixDigest, cl.PrevRandaoHistory[cl.LatestHeadNumber.Uint64()])
		}
		// nonce == 0x0000000000000000
		if newHeader.Nonce != (types.BlockNonce{}) {
			cl.Fatalf("CLMocker: Client %v produced a new header with incorrect nonce: %v", ec.Container, newHeader.Nonce)
		}
		if len(newHeader.Extra) > 32 {
			cl.Fatalf("CLMocker: Client %v produced a new header with incorrect extraData (len > 32): %v", ec.Container, newHeader.Extra)
		}
		cl.LatestHeader = newHeader
	}
	if cl.LatestHeader == nil {
		cl.Fatalf("CLMocker: None of the clients accepted the newly constructed payload")
	}

	if callbacks.OnVerificationEnd != nil {
		callbacks.OnVerificationEnd()
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
