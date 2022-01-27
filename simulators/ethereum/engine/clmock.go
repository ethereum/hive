package main

import (
	"errors"
	"math/big"
	"math/rand"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/eth/catalyst"
	"github.com/ethereum/hive/hivesim"
)

// Semaphore type channel to be used as signal for current CLMocker step in block production
type Semaphore chan bool

func (m Semaphore) Wait() bool {
	_, open := <-m
	return !open
}

func (m Semaphore) Yield() {
	m <- true
}

// Consensus Layer Client Mock used to sync the Execution Clients once the TTD has been reached
type CLMocker struct {
	*hivesim.T
	// List of Engine Clients being served by the CL Mocker
	EngineClients []*EngineClient
	// Lock required so no client is offboarded during block production.
	EngineClientsLock sync.Mutex

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
	FirstPoSBlockNumber        *big.Int
	TTDReached                 bool
	PoSBlockProductionFinished bool

	/*
		The test cases can request values from channel to "wake up" at a specific
		point during the PoS block production procedure and, when the CL mocker has
		reached it, the channel allows the test case to continue.
	*/
	OnPayloadPrepare                 Semaphore
	OnGetPayload                     Semaphore
	OnExecutePayload                 Semaphore
	OnHeadBlockForkchoiceUpdate      Semaphore
	OnSafeBlockForkchoiceUpdate      Semaphore
	OnFinalizedBlockForkchoiceUpdate Semaphore
}

func NewCLMocker(t *hivesim.T) *CLMocker {
	// Init random seed for different purposes
	seed := time.Now().Unix()
	t.Logf("Randomness seed: %v\n", seed)
	rand.Seed(seed)

	// Create the new CL mocker
	newCLMocker := &CLMocker{
		T:                          t,
		EngineClients:              make([]*EngineClient, 0),
		RandomHistory:              map[uint64]common.Hash{},
		ExecutedPayloadHistory:     map[uint64]catalyst.ExecutableDataV1{},
		LatestFinalizedHeader:      nil,
		PoSBlockProductionFinished: false,
		BlockProductionMustStop:    false,
		FirstPoSBlockNumber:        nil,
		LatestFinalizedNumber:      nil,
		TTDReached:                 false,
		NextFeeRecipient:           common.Address{},
		LatestForkchoice: catalyst.ForkchoiceStateV1{
			HeadBlockHash:      common.Hash{},
			SafeBlockHash:      common.Hash{},
			FinalizedBlockHash: common.Hash{},
		},
		OnPayloadPrepare:                 make(Semaphore, 1),
		OnGetPayload:                     make(Semaphore, 1),
		OnExecutePayload:                 make(Semaphore, 1),
		OnHeadBlockForkchoiceUpdate:      make(Semaphore, 1),
		OnSafeBlockForkchoiceUpdate:      make(Semaphore, 1),
		OnFinalizedBlockForkchoiceUpdate: make(Semaphore, 1),
	}

	// Start timer to check when the TTD has been reached
	time.AfterFunc(tTDCheckPeriod, newCLMocker.checkTTD)

	return newCLMocker
}

// Add a Client to be kept in sync with the latest payloads
func (cl *CLMocker) AddEngineClient(ec *EngineClient) {
	cl.EngineClientsLock.Lock()
	defer cl.EngineClientsLock.Unlock()
	cl.EngineClients = append(cl.EngineClients, ec)
}

// Remove a Client to stop sending latest payloads
func (cl *CLMocker) RemoveEngineClient(ec *EngineClient) {
	cl.EngineClientsLock.Lock()
	defer cl.EngineClientsLock.Unlock()

	for i, engine := range cl.EngineClients {
		if engine == ec {
			cl.EngineClients = append(cl.EngineClients[:i], cl.EngineClients[i+1:]...)
		}
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
	if err := ec.c.CallContext(ec.Ctx(), &td, "eth_getBlockByNumber", "latest", false); err != nil {
		cl.Fatalf("CLMocker: Could not get latest totalDifficulty: %v", err)
	}
	if td.TotalDifficulty.ToInt().Cmp(terminalTotalDifficulty) < 0 {
		time.AfterFunc(tTDCheckPeriod, cl.checkTTD)
		return
	}
	var err error
	cl.TTDReached = true
	cl.LatestFinalizedHeader, err = ec.Eth.HeaderByNumber(ec.Ctx(), nil)
	if err != nil {
		cl.Fatalf("CLMocker: Could not get block header: %v", err)
	}
	cl.Logf("CLMocker: TTD has been reached at block %v\n", cl.LatestFinalizedHeader.Number)
	// Broadcast initial ForkchoiceUpdated
	cl.LatestForkchoice.HeadBlockHash = cl.LatestFinalizedHeader.Hash()
	cl.LatestForkchoice.SafeBlockHash = cl.LatestFinalizedHeader.Hash()
	cl.LatestForkchoice.FinalizedBlockHash = cl.LatestFinalizedHeader.Hash()
	for _, resp := range cl.broadcastForkchoiceUpdated(&cl.LatestForkchoice, nil) {
		if resp.Error != nil {
			cl.Logf("CLMocker: forkchoiceUpdated Error: %v\n", resp.Error)
		} else if resp.ForkchoiceResponse.Status != "SUCCESS" {
			cl.Logf("CLMocker: forkchoiceUpdated Response: %v\n", resp.ForkchoiceResponse)
		}
	}
	time.AfterFunc(PoSBlockProductionPeriod, cl.producePoSBlocks)
	return
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
// A transaction can be included to be sent before getPayload if necessary
func (cl *CLMocker) setNextFeeRecipient(feeRecipient common.Address, ec *EngineClient, tx *types.Transaction) (*big.Int, error) {
	for {
		closed := cl.OnPayloadPrepare.Wait()
		if closed {
			return nil, errors.New("CLMocker finished producing blocks")
		}
		if ec == nil || (cl.NextBlockProducer != nil && cl.NextBlockProducer.Equals(ec)) {
			defer cl.OnPayloadPrepare.Yield()
			cl.NextFeeRecipient = feeRecipient
			if tx != nil {
				err := ec.Eth.SendTransaction(ec.Ctx(), tx)
				if err != nil {
					return nil, err
				}
			}
			return big.NewInt(cl.LatestFinalizedNumber.Int64() + 1), nil
		}
		// Unlock and keep trying to get the requested Engine Client
		cl.OnPayloadPrepare.Yield()
		time.Sleep(PoSBlockProductionPeriod)
	}
}

// Close all semaphores in the given CLMocker instance
func (cl *CLMocker) closeSemaphores() {
	close(cl.OnPayloadPrepare)
	close(cl.OnGetPayload)
	close(cl.OnExecutePayload)
	close(cl.OnHeadBlockForkchoiceUpdate)
	close(cl.OnSafeBlockForkchoiceUpdate)
	close(cl.OnFinalizedBlockForkchoiceUpdate)
}

func (cl *CLMocker) produceSinglePoSBlock() {
	var (
		lastBlockNumber uint64
		err             error
	)

	// Lock needed to ensure EngineClients is not modified mid block production
	cl.EngineClientsLock.Lock()
	defer cl.EngineClientsLock.Unlock()

	for {
		// Get a random client to generate the payload
		ec_id := rand.Intn(len(cl.EngineClients))
		cl.NextBlockProducer = cl.EngineClients[ec_id]

		lastBlockNumber, err = cl.NextBlockProducer.Eth.BlockNumber(cl.NextBlockProducer.Ctx())
		if err != nil {
			cl.Fatalf("CLMocker: Could not get block number while selecting client for payload production (%v): %v", cl.NextBlockProducer.Client.Container, err)
		}

		lastBlockNumberBig := big.NewInt(int64(lastBlockNumber))

		if cl.LatestFinalizedNumber != nil && cl.LatestFinalizedNumber.Cmp(lastBlockNumberBig) != 0 {
			// Selected client is not synced to the last block number, try again
			continue
		}

		latestHeader, err := cl.NextBlockProducer.Eth.HeaderByNumber(cl.NextBlockProducer.Ctx(), lastBlockNumberBig)
		if err != nil {
			cl.Fatalf("CLMocker: Could not get block header while selecting client for payload production (%v): %v", cl.NextBlockProducer.Client.Container, err)
		}

		lastBlockHash := latestHeader.Hash()

		if cl.LatestFinalizedHeader.Hash() != lastBlockHash {
			// Selected client latest block hash does not match canonical chain, try again
			continue
		} else {
			break
		}

	}

	// Yield before preparing the next payload
	cl.OnPayloadPrepare.Yield()
	<-cl.OnPayloadPrepare

	// Generate a random value for the Random field
	nextRandom := common.Hash{}
	rand.Read(nextRandom[:])

	payloadAttributes := catalyst.PayloadAttributesV1{
		Timestamp:             cl.LatestFinalizedHeader.Time + 1,
		Random:                nextRandom,
		SuggestedFeeRecipient: cl.NextFeeRecipient,
	}

	resp, err := cl.NextBlockProducer.EngineForkchoiceUpdatedV1(cl.NextBlockProducer.Ctx(), &cl.LatestForkchoice, &payloadAttributes)
	if err != nil {
		cl.Fatalf("CLMocker: Could not send forkchoiceUpdatedV1 (%v): %v", cl.NextBlockProducer.Client.Container, err)
	}
	if resp.Status != "SUCCESS" {
		cl.Logf("CLMocker: forkchoiceUpdated Response: %v\n", resp)
	}

	cl.LatestPayloadBuilt, err = cl.NextBlockProducer.EngineGetPayloadV1(cl.NextBlockProducer.Ctx(), resp.PayloadID)
	if err != nil {
		cl.Fatalf("CLMocker: Could not getPayload (%v, %v): %v", cl.NextBlockProducer.Client.Container, resp.PayloadID, err)
	}

	// Yield after the new payload is built and retrieved using GetPayload
	cl.OnGetPayload.Yield()
	<-cl.OnGetPayload

	// Broadcast the executePayload to all clients
	for i, resp := range cl.broadcastExecutePayload(&cl.LatestPayloadBuilt) {
		if resp.Error != nil {
			cl.Logf("CLMocker: broadcastExecutePayload Error (%v): %v\n", i, resp.Error)

		} else if resp.ExecutePayloadResponse.Status != "VALID" {
			cl.Logf("CLMocker: broadcastExecutePayload Response (%v): %v\n", i, resp.ExecutePayloadResponse)
		}
	}
	cl.LatestExecutedPayload = cl.LatestPayloadBuilt
	cl.ExecutedPayloadHistory[cl.LatestPayloadBuilt.Number] = cl.LatestPayloadBuilt

	// Yield after the new payload has been broadcasted for execution
	cl.OnExecutePayload.Yield()
	<-cl.OnExecutePayload

	// Broadcast forkchoice updated with new HeadBlock to all clients
	cl.LatestForkchoice.HeadBlockHash = cl.LatestPayloadBuilt.BlockHash
	for i, resp := range cl.broadcastForkchoiceUpdated(&cl.LatestForkchoice, nil) {
		if resp.Error != nil {
			cl.Logf("CLMocker: broadcastForkchoiceUpdated Error (%v): %v\n", i, resp.Error)
		} else if resp.ForkchoiceResponse.Status != "SUCCESS" {
			cl.Logf("CLMocker: broadcastForkchoiceUpdated Response (%v): %v\n", i, resp.ForkchoiceResponse)
		}
	}

	// Yield after fcU has been broadcasted with new payload as HeadBlock
	cl.OnHeadBlockForkchoiceUpdate.Yield()
	<-cl.OnHeadBlockForkchoiceUpdate

	// Broadcast forkchoice updated with new SafeBlock to all clients
	cl.LatestForkchoice.SafeBlockHash = cl.LatestPayloadBuilt.BlockHash
	for i, resp := range cl.broadcastForkchoiceUpdated(&cl.LatestForkchoice, nil) {
		if resp.Error != nil {
			cl.Logf("CLMocker: broadcastForkchoiceUpdated Error (%v): %v\n", i, resp.Error)
		} else if resp.ForkchoiceResponse.Status != "SUCCESS" {
			cl.Logf("CLMocker: broadcastForkchoiceUpdated Response (%v): %v\n", i, resp.ForkchoiceResponse)
		}
	}

	// Yield after fcU has been broadcasted with new payload as SafeBlock
	cl.OnSafeBlockForkchoiceUpdate.Yield()
	<-cl.OnSafeBlockForkchoiceUpdate

	// Broadcast forkchoice updated with new FinalizedBlock to all clients
	cl.LatestForkchoice.FinalizedBlockHash = cl.LatestPayloadBuilt.BlockHash
	for i, resp := range cl.broadcastForkchoiceUpdated(&cl.LatestForkchoice, nil) {
		if resp.Error != nil {
			cl.Logf("CLMocker: broadcastForkchoiceUpdated Error (%v): %v\n", i, resp.Error)
		} else if resp.ForkchoiceResponse.Status != "SUCCESS" {
			cl.Logf("CLMocker: broadcastForkchoiceUpdated Response (%v): %v\n", i, resp.ForkchoiceResponse)
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

	// Check if any of the clients accepted the new payload
	cl.LatestFinalizedHeader = nil
	for _, ec := range cl.EngineClients {
		newHeader, err := ec.Eth.HeaderByNumber(cl.NextBlockProducer.Ctx(), cl.LatestFinalizedNumber)
		if err == nil {
			cl.LatestFinalizedHeader = newHeader
			break
		}
	}
	if cl.LatestFinalizedHeader == nil {
		cl.Fatalf("CLMocker: None of the clients accepted the newly constructed payload")
	}

	// Yield after block production has completed a new block
	cl.OnFinalizedBlockForkchoiceUpdate.Yield()
	<-cl.OnFinalizedBlockForkchoiceUpdate
}

// Loop produce PoS blocks by using the Engine API
func (cl *CLMocker) producePoSBlocks() {
	// Defer closing all semaphores
	defer cl.closeSemaphores()

	// Defer signaling that we are not producing more blocks
	defer func() { cl.PoSBlockProductionFinished = true }()

	// Begin PoS block production
	for !cl.BlockProductionMustStop {
		cl.produceSinglePoSBlock()
		time.Sleep(PoSBlockProductionPeriod)
	}

}

type ExecutePayloadOutcome struct {
	ExecutePayloadResponse *catalyst.ExecutePayloadResponse
	Error                  error
}

func (cl *CLMocker) broadcastExecutePayload(payload *catalyst.ExecutableDataV1) []ExecutePayloadOutcome {
	responses := make([]ExecutePayloadOutcome, len(cl.EngineClients))
	for i, ec := range cl.EngineClients {
		execPayloadResp, err := ec.EngineExecutePayloadV1(ec.Ctx(), payload)
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
	ForkchoiceResponse *catalyst.ForkChoiceResponse
	Error              error
}

func (cl *CLMocker) broadcastForkchoiceUpdated(fcstate *catalyst.ForkchoiceStateV1, payloadAttr *catalyst.PayloadAttributesV1) []ForkChoiceOutcome {
	responses := make([]ForkChoiceOutcome, len(cl.EngineClients))
	for i, ec := range cl.EngineClients {
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
