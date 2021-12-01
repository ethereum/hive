package main

import (
	"fmt"
	"math/big"
	"math/rand"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/eth/catalyst"
)

// Consensus Client Mock used to sync the Execution Clients once the TTD has been reached
type CLMocker struct {
	tTD                *big.Int
	randomHistory      map[uint64]common.Hash
	catclients         []*CatalystClient
	difficultyByBlock  map[uint64]*big.Int
	lastKnownDiffBlock *big.Int

	PoSBlockProductionActivated bool
	blockProductionMustStop     bool
	firstPoSBlock               *big.Int
}

func NewCLMocker(tTD *big.Int) *CLMocker {
	// Init random seed for different purposes
	rand.Seed(time.Now().Unix())
	// Create the new CL mocker
	newCLMocker := &CLMocker{tTD, map[uint64]common.Hash{}, make([]*CatalystClient, 0), map[uint64]*big.Int{}, big.NewInt(-1), false, false, nil}
	time.AfterFunc(tTDCheck, newCLMocker.checkTTD)
	return newCLMocker
}

// Add a Client to be kept in sync with the latest payloads
func (cl *CLMocker) AddCatalystClient(newCatClient *CatalystClient) {
	cl.catclients = append(cl.catclients, newCatClient)
}

// TerminalTotalDifficulty check methods
// This method checks whether we have reached TTD and enables PoS block production when done.
func (cl *CLMocker) checkTTD() {

	if cl.isTTDReached() {
		time.AfterFunc(blockProductionPoS, cl.minePOSBlock)
		return
	}

	time.AfterFunc(tTDCheck, cl.checkTTD)
}

func (cl *CLMocker) currentTD() *big.Int {
	if cl.lastKnownDiffBlock.Cmp(big.NewInt(0)) < 0 {
		return big.NewInt(0)
	}
	tD := big.NewInt(0)
	for i := uint64(0); i <= cl.lastKnownDiffBlock.Uint64(); i++ {
		tD.Add(tD, cl.difficultyByBlock[i])
	}
	return tD
}

func (cl *CLMocker) isTTDReached() bool {
	if len(cl.catclients) == 0 {
		// We have no clients running yet, we have not reached TTD
		return false
	}
	// Pick a random client to get the total difficulty of its head
	ec := cl.catclients[rand.Intn(len(cl.catclients))]

	lastBlockN, err := ec.Eth.BlockNumber(ec.Ctx())
	if err != nil {
		ec.Fatalf("Could not get block number: %v", err)
	}

	for {
		if cl.lastKnownDiffBlock.Cmp(big.NewInt(int64(lastBlockN))) >= 0 {
			break
		}
		nextBlock := big.NewInt(0).Add(cl.lastKnownDiffBlock, big.NewInt(1))
		nextHeader, err := ec.Eth.HeaderByNumber(ec.Ctx(), nextBlock)
		if err != nil {
			ec.Fatalf("Could not get block header: %v", err)
		}
		cl.difficultyByBlock[nextBlock.Uint64()] = nextHeader.Difficulty
		cl.lastKnownDiffBlock = nextBlock
	}

	// If the total difficulty is greater than or equal to the terminal total difficulty
	// we can start PoS block production.

	if cl.currentTD().Cmp(terminalTotalDifficulty) >= 0 {
		fmt.Printf("DEBUG: Reached TTD at Block %v with TD=%v\n", lastBlockN, cl.currentTD())

		return true
	}
	fmt.Printf("DEBUG: TTD not reached at Block %v with TD=%v\n", lastBlockN, cl.currentTD())
	return false
}

// Engine Block Production Methods
func (cl *CLMocker) stopPoSBlockProduction() {
	cl.blockProductionMustStop = true
}

func (cl *CLMocker) isBlockPoS(bn *big.Int) bool {
	if cl.firstPoSBlock == nil {
		return false
	}
	if cl.firstPoSBlock.Cmp(bn) <= 0 {
		return true
	}
	return false
}

// Mine a PoS block by using the catalyst Engine API
func (cl *CLMocker) minePOSBlock() {
	// Get the client generating the payload
	ec := cl.catclients[rand.Intn(len(cl.catclients))]

	lastBlockN, err := ec.Eth.BlockNumber(ec.Ctx())
	if err != nil {
		ec.Fatalf("Could not get block number: %v", err)
	}

	latestHeader, err := ec.Eth.HeaderByNumber(ec.Ctx(), big.NewInt(int64(lastBlockN)))
	if err != nil {
		ec.Fatalf("Could not get block header: %v", err)
	}

	lastBlockHash := latestHeader.Hash()

	fmt.Printf("lastBlock.Hash(): %v\n", lastBlockHash)

	forkchoiceState := catalyst.ForkchoiceStateV1{
		HeadBlockHash:      lastBlockHash,
		SafeBlockHash:      lastBlockHash,
		FinalizedBlockHash: lastBlockHash,
	}

	// Generate a random value for the Random field
	nextRandom := common.Hash{}
	for i := 0; i < common.HashLength; i++ {
		nextRandom[i] = byte(rand.Uint32())
	}

	payloadAttributes := catalyst.PayloadAttributesV1{
		Timestamp:    latestHeader.Time + 1,
		Random:       nextRandom,
		FeeRecipient: common.Address{},
	}

	fmt.Printf("payloadAttributes: %v\n", payloadAttributes)

	resp, err := ec.EngineForkchoiceUpdatedV1(ec.Ctx(), &forkchoiceState, &payloadAttributes)
	if err != nil {
		ec.Fatalf("Could not send forkchoiceUpdatedV1: %v", err)
	}
	fmt.Printf("Forkchoice Status: %v\n", resp.Status)
	fmt.Printf("resp.PayloadID: %v\n", resp.PayloadID)

	payload, err := ec.EngineGetPayloadV1(ec.Ctx(), resp.PayloadID)
	if err != nil {
		ec.Fatalf("Could not getPayload (%v): %v", resp.PayloadID, err)
	}

	fmt.Printf("payload.BlockHash: %v\n", payload.BlockHash)
	fmt.Printf("payload.StateRoot: %v\n", payload.StateRoot)
	for i := 0; i < len(payload.Transactions); i++ {
		fmt.Printf("payload.Transactions[%v]: %v\n", i, hexutil.Bytes(payload.Transactions[i]).String())
	}

	for _, ec_p := range cl.catclients {
		execPayloadResp, err := ec_p.EngineExecutePayloadV1(ec_p.Ctx(), &payload)

		if err != nil {
			ec_p.Fatalf("Could not executePayload (%v): %v", resp.PayloadID, err)
		}
		fmt.Printf("execPayloadResp.Status: %v\n", execPayloadResp.Status)

		forkchoiceState = catalyst.ForkchoiceStateV1{
			HeadBlockHash:      payload.BlockHash,
			SafeBlockHash:      payload.BlockHash,
			FinalizedBlockHash: payload.BlockHash,
		}

		resp, err = ec_p.EngineForkchoiceUpdatedV1(ec_p.Ctx(), &forkchoiceState, nil)
		if err != nil {
			ec_p.Fatalf("Could not send forkchoiceUpdatedV1: %v", err)
		}

		fmt.Printf("resp.Status: %v\n", resp.Status)

	}

	cl.randomHistory[lastBlockN+1] = nextRandom

	if cl.firstPoSBlock == nil {
		cl.firstPoSBlock = big.NewInt(int64(lastBlockN + 1))
	}

	if !cl.blockProductionMustStop {
		time.AfterFunc(blockProductionPoS, cl.minePOSBlock)
	}
}
