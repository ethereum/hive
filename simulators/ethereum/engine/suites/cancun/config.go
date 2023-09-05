package suite_cancun

import (
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/hive/simulators/ethereum/engine/clmock"
	"github.com/ethereum/hive/simulators/ethereum/engine/globals"
	"github.com/ethereum/hive/simulators/ethereum/engine/test"
)

// Contains the base spec for all cancun tests.
type CancunBaseSpec struct {
	test.Spec
	TimeIncrements   uint64 // Timestamp increments per block throughout the test
	GetPayloadDelay  uint64 // Delay between FcU and GetPayload calls
	CancunForkHeight uint64 // Cancun activation fork height
	GenesisTimestamp *uint64
	TestSequence
}

// Timestamp delta between genesis and the cancun fork
func (cs *CancunBaseSpec) GetCancunGenesisTimeDelta() uint64 {
	return cs.CancunForkHeight * cs.GetBlockTimeIncrements()
}

func (cs *CancunBaseSpec) GetGenesisTimestamp() uint64 {
	if cs.GenesisTimestamp != nil {
		return *cs.GenesisTimestamp
	}
	return uint64(globals.GenesisTimestamp)
}

// Calculates Cancun fork timestamp given the amount of blocks that need to be
// produced beforehand.
func (cs *CancunBaseSpec) GetCancunForkTime() uint64 {
	return cs.GetGenesisTimestamp() + cs.GetCancunGenesisTimeDelta()
}

// Generates the fork config, including cancun fork timestamp.
func (cs *CancunBaseSpec) GetForkConfig() globals.ForkConfig {
	return globals.ForkConfig{
		ShanghaiTimestamp: big.NewInt(0), // No test starts before Shanghai
		CancunTimestamp:   new(big.Int).SetUint64(cs.GetCancunForkTime()),
	}
}

// Get the per-block timestamp increments configured for this test
func (cs *CancunBaseSpec) GetBlockTimeIncrements() uint64 {
	if cs.TimeIncrements == 0 {
		return 1
	}
	return cs.TimeIncrements
}

// Timestamp delta between genesis and the cancun fork
func (cs *CancunBaseSpec) GetBlobsGenesisTimeDelta() uint64 {
	return cs.CancunForkHeight * cs.GetBlockTimeIncrements()
}

// Calculates Cancun fork timestamp given the amount of blocks that need to be
// produced beforehand.
func (cs *CancunBaseSpec) GetBlobsForkTime() uint64 {
	return cs.GetGenesisTimestamp() + cs.GetBlobsGenesisTimeDelta()
}

// Append the accounts that are going to use the BLOBHASH with, we will also
// predeploy the EIP-4788 bytecode at the HISTORY_STORAGE_ADDRESS.
func (cs *CancunBaseSpec) GetGenesis() *core.Genesis {
	genesis := cs.Spec.GetGenesis()

	// Add accounts that use the BLOBHASH opcode
	datahashCode := []byte{
		0x5F, // PUSH0
		0x80, // DUP1
		0x49, // DATAHASH
		0x55, // SSTORE
		0x60, // PUSH1(0x01)
		0x01,
		0x80, // DUP1
		0x49, // DATAHASH
		0x55, // SSTORE
		0x60, // PUSH1(0x02)
		0x02,
		0x80, // DUP1
		0x49, // DATAHASH
		0x55, // SSTORE
		0x60, // PUSH1(0x03)
		0x03,
		0x80, // DUP1
		0x49, // DATAHASH
		0x55, // SSTORE
	}

	for i := 0; i < DATAHASH_ADDRESS_COUNT; i++ {
		address := big.NewInt(0).Add(DATAHASH_START_ADDRESS, big.NewInt(int64(i)))
		genesis.Alloc[common.BigToAddress(address)] = core.GenesisAccount{
			Code:    datahashCode,
			Balance: common.Big0,
		}
	}

	// Add bytecode pre deploy to the EIP-4788 address.
	genesis.Alloc[BEACON_ROOTS_ADDRESS] = core.GenesisAccount{
		Balance: common.Big0,
		Nonce:   1,
		Code:    common.Hex2Bytes("3373fffffffffffffffffffffffffffffffffffffffe14604457602036146024575f5ffd5b620180005f350680545f35146037575f5ffd5b6201800001545f5260205ff35b42620180004206555f3562018000420662018000015500"),
	}

	return genesis
}

// Changes the CL Mocker default time increments of 1 to the value specified
// in the test spec.
func (cs *CancunBaseSpec) ConfigureCLMock(cl *clmock.CLMocker) {
	cl.BlockTimestampIncrement = big.NewInt(int64(cs.GetBlockTimeIncrements()))
}

// Base test case execution procedure for blobs tests.
func (cs *CancunBaseSpec) Execute(t *test.Env) {

	t.CLMock.WaitForTTD()

	blobTestCtx := &CancunTestContext{
		Env:            t,
		TestBlobTxPool: new(TestBlobTxPool),
	}

	blobTestCtx.TestBlobTxPool.HashesByIndex = make(map[uint64]common.Hash)

	if cs.GetPayloadDelay != 0 {
		t.CLMock.PayloadProductionClientDelay = time.Duration(cs.GetPayloadDelay) * time.Second
	}

	for stepId, step := range cs.TestSequence {
		t.Logf("INFO: Executing step %d: %s", stepId+1, step.Description())
		if err := step.Execute(blobTestCtx); err != nil {
			t.Fatalf("FAIL: Error executing step %d: %v", stepId+1, err)
		}
	}

}

type CancunForkSpec struct {
	CancunBaseSpec
	GenesisTimestamp  uint64
	ShanghaiTimestamp uint64
	CancunTimestamp   uint64

	ProduceBlocksBeforePeering uint64
}

func (cs *CancunForkSpec) GetGenesisTimestamp() uint64 {
	return cs.GenesisTimestamp
}

// Get Cancun fork timestamp.
func (cs *CancunForkSpec) GetCancunForkTime() uint64 {
	return cs.CancunTimestamp
}

// Generates the fork config, including cancun fork timestamp.
func (cs *CancunForkSpec) GetForkConfig() globals.ForkConfig {
	return globals.ForkConfig{
		ShanghaiTimestamp: new(big.Int).SetUint64(cs.ShanghaiTimestamp),
		CancunTimestamp:   new(big.Int).SetUint64(cs.CancunTimestamp),
	}
}

// Genesis generation.
func (cs *CancunForkSpec) GetGenesis() *core.Genesis {
	// Calculate the cancun fork height
	cs.CancunForkHeight = (cs.CancunTimestamp - cs.GenesisTimestamp) / cs.GetBlockTimeIncrements()
	genesis := cs.CancunBaseSpec.GetGenesis()
	genesis.Timestamp = cs.GenesisTimestamp
	return genesis
}

func (cs *CancunForkSpec) Execute(t *test.Env) {
	// Add steps to check the sequence
	cs.TestSequence = TestSequence{
		NewPayloads{
			PayloadCount: cs.ProduceBlocksBeforePeering,
		},
		DevP2PClientPeering{
			ClientIndex: 0,
		},
	}

	cs.CancunBaseSpec.Execute(t)
}
