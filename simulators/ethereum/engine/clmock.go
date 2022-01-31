package main

import (
	"math/big"
	"math/rand"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/beacon"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/hive/hivesim"
)

// Semaphore type channel to be used as signal for current CLMocker step in block production
type Semaphore chan bool

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
	NextBlockProducer *EngineClient
	NextFeeRecipient  common.Address

	// PoS Chain History Information
	RandomHistory          map[uint64]common.Hash
	ExecutedPayloadHistory map[uint64]beacon.ExecutableDataV1

	// Latest broadcasted data using the PoS Engine API
	LatestFinalizedNumber *big.Int
	LatestFinalizedHeader *types.Header
	LatestPayloadBuilt    beacon.ExecutableDataV1
	LatestExecutedPayload beacon.ExecutableDataV1
	LatestForkchoice      beacon.ForkchoiceStateV1

	// Merge related
	FirstPoSBlockNumber     *big.Int
	TTDReached              bool
	TerminalTotalDifficulty *big.Int

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

	// CLMocker channel which is closed when the instance has completely finished,
	// must be closed only by the internal CLMocker procedures.
	OnExit chan interface{}

	// Internal CLMocker channel which will be closed to initiate shutdown,
	// must be closed externally only by calling the CLMocker.shutdown() function.
	shutdownRequested chan interface{}
}

func NewCLMocker(t *hivesim.T, tTD *big.Int) *CLMocker {
	// Init random seed for different purposes
	seed := time.Now().Unix()
	t.Logf("Randomness seed: %v\n", seed)
	rand.Seed(seed)

	// Create the new CL mocker
	newCLMocker := &CLMocker{
		T:                       t,
		EngineClients:           make([]*EngineClient, 0),
		RandomHistory:           map[uint64]common.Hash{},
		ExecutedPayloadHistory:  map[uint64]beacon.ExecutableDataV1{},
		LatestFinalizedHeader:   nil,
		FirstPoSBlockNumber:     nil,
		LatestFinalizedNumber:   nil,
		TTDReached:              false,
		TerminalTotalDifficulty: tTD,
		NextFeeRecipient:        common.Address{},
		LatestForkchoice: beacon.ForkchoiceStateV1{
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
		OnExit:                           make(chan interface{}),
		shutdownRequested:                make(chan interface{}),
	}

	// Start thread to check when the TTD has been reached
	go newCLMocker.checkTTD()

	return newCLMocker
}

// Add a Client to be kept in sync with the latest payloads
func (cl *CLMocker) AddEngineClient(t *hivesim.T, c *hivesim.Client) {
	cl.EngineClientsLock.Lock()
	defer cl.EngineClientsLock.Unlock()
	ec := NewEngineClient(t, c)
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

// Exit function, called only internally to terminate the CLMocker instance
func (cl *CLMocker) Exit() {
	for _, engine := range cl.EngineClients {
		engine.Close()
	}
	close(cl.OnExit)
}

// Helper struct to fetch the TotalDifficulty
type TD struct {
	TotalDifficulty *hexutil.Big `json:"totalDifficulty"`
}

// Check whether we have reached TTD and then enable PoS block production.
// This function must NOT be executed after we have reached TTD.
func (cl *CLMocker) checkTTD() {
	for {
		select {
		case <-time.After(tTDCheckPeriod):
			if len(cl.EngineClients) == 0 {
				// We have no clients running yet, we have not reached TTD
				continue
			}
			// Pick a random client to get the total difficulty of its head
			ec := cl.EngineClients[rand.Intn(len(cl.EngineClients))]

			var td *TD
			if err := ec.c.CallContext(ec.Ctx(), &td, "eth_getBlockByNumber", "latest", false); err != nil {
				cl.Exit()
				cl.Fatalf("CLMocker: Could not get latest totalDifficulty: %v", err)
			}
			if td.TotalDifficulty.ToInt().Cmp(cl.TerminalTotalDifficulty) < 0 {
				// TTD not reached yet
				continue
			}

			var err error
			cl.TTDReached = true
			cl.LatestFinalizedHeader, err = ec.Eth.HeaderByNumber(ec.Ctx(), nil)
			if err != nil {
				cl.Exit()
				cl.Fatalf("CLMocker: Could not get block header: %v", err)
			}

			cl.Logf("CLMocker: TTD has been reached at block %v (%v>=%v)\n", cl.LatestFinalizedHeader.Number, td.TotalDifficulty, cl.TerminalTotalDifficulty)
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
					if resp.ForkchoiceResponse.PayloadStatus.Status == "VALID" {
						anySuccess = true
					} else {
						cl.Logf("CLMocker: forkchoiceUpdated Response: %v\n", resp.ForkchoiceResponse)
					}
				}
			}
			if !anySuccess {
				cl.Exit()
				cl.Fatalf("CLMocker: None of the clients accepted forkchoiceUpdated")
			}
			go cl.producePoSBlocks()
			return
		case <-cl.shutdownRequested:
			cl.Exit()
			return
		case <-cl.OnExit:
			// Should not happen
			return
		}
	}
}

// Engine Block Production Methods
func (cl *CLMocker) Shutdown() {
	close(cl.shutdownRequested)
}

// Check whether a block number is a PoS block
func (cl *CLMocker) isBlockPoS(bn *big.Int) bool {
	if cl.FirstPoSBlockNumber == nil || cl.FirstPoSBlockNumber.Cmp(bn) > 0 {
		return false
	}
	return true
}

func (cl *CLMocker) produceSinglePoSBlock() {
	var (
		lastBlockNumber uint64
		err             error
	)

	// Lock needed to ensure EngineClients is not modified mid block production
	cl.EngineClientsLock.Lock()
	defer cl.EngineClientsLock.Unlock()

	if len(cl.EngineClients) == 0 {
		cl.Fatalf("CLMocker: No clients left for block production")
	}

	for i := 0; i < len(cl.EngineClients); i++ {
		// Get a client to generate the payload
		ec_id := (int(cl.LatestFinalizedNumber.Int64()) + i) % len(cl.EngineClients)
		cl.NextBlockProducer = cl.EngineClients[ec_id]

		lastBlockNumber, err = cl.NextBlockProducer.Eth.BlockNumber(cl.NextBlockProducer.Ctx())
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

	// Yield before preparing the next payload
	cl.OnPayloadPrepare.Yield()
	<-cl.OnPayloadPrepare

	// Generate a random value for the Random field
	nextRandom := common.Hash{}
	rand.Read(nextRandom[:])

	payloadAttributes := beacon.PayloadAttributesV1{
		Timestamp:             cl.LatestFinalizedHeader.Time + 1,
		Random:                nextRandom,
		SuggestedFeeRecipient: cl.NextFeeRecipient,
	}

	resp, err := cl.NextBlockProducer.EngineForkchoiceUpdatedV1(cl.NextBlockProducer.Ctx(), &cl.LatestForkchoice, &payloadAttributes)
	if err != nil {
		cl.Fatalf("CLMocker: Could not send forkchoiceUpdatedV1 (%v): %v", cl.NextBlockProducer.Client.Container, err)
	}
	if resp.PayloadStatus.Status != "VALID" {
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
	for _, resp := range cl.broadcastNewPayload(&cl.LatestPayloadBuilt) {
		if resp.Error != nil {
			cl.Logf("CLMocker: broadcastNewPayload Error (%v): %v\n", resp.Container, resp.Error)

		} else {
			if resp.ExecutePayloadResponse.Status == "ACCEPTED" {
				if resp.ExecutePayloadResponse.LatestValidHash != cl.LatestPayloadBuilt.BlockHash {
					// https://github.com/ethereum/execution-apis/blob/main/src/engine/specification.md:
					// - If validation succeeds, the response MUST contain {status: VALID, latestValidHash: payload.blockHash}
					cl.Fatalf("FAIL (%v): NewPayload returned ACCEPTED status with incorrect LatestValidHash, %v!=%v", resp.ExecutePayloadResponse.LatestValidHash, cl.LatestPayloadBuilt.BlockHash)
				}
			} else {
				cl.Logf("CLMocker: broadcastNewPayload Response (%v): %v\n", resp.Container, resp.ExecutePayloadResponse)
			}
		}
	}
	cl.LatestExecutedPayload = cl.LatestPayloadBuilt
	cl.ExecutedPayloadHistory[cl.LatestPayloadBuilt.Number] = cl.LatestPayloadBuilt

	// Yield after the new payload has been broadcasted for execution
	cl.OnExecutePayload.Yield()
	<-cl.OnExecutePayload

	// Broadcast forkchoice updated with new HeadBlock to all clients
	cl.LatestForkchoice.HeadBlockHash = cl.LatestPayloadBuilt.BlockHash
	for _, resp := range cl.broadcastForkchoiceUpdated(&cl.LatestForkchoice, nil) {
		if resp.Error != nil {
			cl.Logf("CLMocker: broadcastForkchoiceUpdated Error (%v): %v\n", resp.Container, resp.Error)
		} else if resp.ForkchoiceResponse.PayloadStatus.Status != "VALID" {
			cl.Logf("CLMocker: broadcastForkchoiceUpdated Response (%v): %v\n", resp.Container, resp.ForkchoiceResponse)
		}
	}

	// Yield after fcU has been broadcasted with new payload as HeadBlock
	cl.OnHeadBlockForkchoiceUpdate.Yield()
	<-cl.OnHeadBlockForkchoiceUpdate

	// Broadcast forkchoice updated with new SafeBlock to all clients
	cl.LatestForkchoice.SafeBlockHash = cl.LatestPayloadBuilt.BlockHash
	for _, resp := range cl.broadcastForkchoiceUpdated(&cl.LatestForkchoice, nil) {
		if resp.Error != nil {
			cl.Logf("CLMocker: broadcastForkchoiceUpdated Error (%v): %v\n", resp.Container, resp.Error)
		} else if resp.ForkchoiceResponse.PayloadStatus.Status != "VALID" {
			cl.Logf("CLMocker: broadcastForkchoiceUpdated Response (%v): %v\n", resp.Container, resp.ForkchoiceResponse)
		}
	}

	// Yield after fcU has been broadcasted with new payload as SafeBlock
	cl.OnSafeBlockForkchoiceUpdate.Yield()
	<-cl.OnSafeBlockForkchoiceUpdate

	// Broadcast forkchoice updated with new FinalizedBlock to all clients
	cl.LatestForkchoice.FinalizedBlockHash = cl.LatestPayloadBuilt.BlockHash
	for _, resp := range cl.broadcastForkchoiceUpdated(&cl.LatestForkchoice, nil) {
		if resp.Error != nil {
			cl.Logf("CLMocker: broadcastForkchoiceUpdated Error (%v): %v\n", resp.Container, resp.Error)
		} else if resp.ForkchoiceResponse.PayloadStatus.Status != "VALID" {
			cl.Logf("CLMocker: broadcastForkchoiceUpdated Response (%v): %v\n", resp.Container, resp.ForkchoiceResponse)
		}
	}

	// Save random value
	cl.RandomHistory[cl.LatestFinalizedHeader.Number.Uint64()+1] = nextRandom

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
	defer func() { cl.Exit() }()

	// Defer producing one last block to ensure everything is ok after the test
	defer cl.produceSinglePoSBlock()

	// Begin PoS block production
	for {
		select {
		case <-time.After(PoSBlockProductionPeriod):
			cl.produceSinglePoSBlock()
		case <-cl.shutdownRequested:
			return
		}
	}
}

type ExecutePayloadOutcome struct {
	ExecutePayloadResponse *beacon.PayloadStatusV1
	Container              string
	Error                  error
}

func (cl *CLMocker) broadcastNewPayload(payload *beacon.ExecutableDataV1) []ExecutePayloadOutcome {
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
	ForkchoiceResponse *beacon.ForkChoiceResponse
	Container          string
	Error              error
}

func (cl *CLMocker) broadcastForkchoiceUpdated(fcstate *beacon.ForkchoiceStateV1, payloadAttr *beacon.PayloadAttributesV1) []ForkChoiceOutcome {
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
