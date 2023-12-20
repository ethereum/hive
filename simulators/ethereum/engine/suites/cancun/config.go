package suite_cancun

import (
	"github.com/ethereum/hive/simulators/ethereum/engine/client"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/hive/simulators/ethereum/engine/test"
)

var (
	/*
		Warm coinbase contract needs to check if EIP-3651 applied after shapella
		https://eips.ethereum.org/EIPS/eip-3651

		Contract bytecode saves coinbase access cost to the slot number of current block
		i.e. if current block number is 5 ==> coinbase access cost saved to slot 5 etc
	*/
	WARM_COINBASE_ADDRESS = common.HexToAddress("0x0101010101010101010101010101010101010101")
	warmCoinbaseCode      = []byte{
		0x5A, // GAS
		0x60, // PUSH1(0x00)
		0x00,
		0x60, // PUSH1(0x00)
		0x00,
		0x60, // PUSH1(0x00)
		0x00,
		0x60, // PUSH1(0x00)
		0x00,
		0x60, // PUSH1(0x00)
		0x00,
		0x41, // COINBASE
		0x60, // PUSH1(0xFF)
		0xFF,
		0xF1, // CALL
		0x5A, // GAS
		0x90, // SWAP1
		0x50, // POP - Call result
		0x90, // SWAP1
		0x03, // SUB
		0x60, // PUSH1(0x16) - GAS + PUSH * 6 + COINBASE
		0x16,
		0x90, // SWAP1
		0x03, // SUB
		0x43, // NUMBER
		0x55, // SSTORE
	}
	/*
		PUSH0 contract needs to check if EIP-3855 applied after shapella
		https://eips.ethereum.org/EIPS/eip-3855

		Contract bytecode reverts tx before the shapells (because PUSH0 opcode does not exists)
		After shapella hardfork it saves current block number to 0 slot
	*/
	PUSH0_ADDRESS = common.HexToAddress("0x0202020202020202020202020202020202020202")
	push0Code     = []byte{
		0x43, // NUMBER
		0x5F, // PUSH0
		0x55, // SSTORE
	}
	BEACON_ROOTS_ADDRESS = common.HexToAddress("0x000F3df6D732807Ef1319fB7B8bB8522d0Beac02")
)

// Contains the base spec for all cancun tests.
type CancunBaseSpec struct {
	test.BaseSpec
	GetPayloadDelay uint64 // Delay between FcU and GetPayload calls
	TestSequence
}

// Append the accounts we are going to withdraw to, which should also include
// bytecode for testing purposes.
func (cs *CancunBaseSpec) GetGenesis(base string) client.Genesis {

	genesis := cs.BaseSpec.GetGenesis(base)

	warmCoinbaseAcc := client.NewAccount()
	push0Acc := client.NewAccount()
	beaconRootsAcc := client.NewAccount()

	beaconRootsAcc.SetBalance(common.Big0)
	beaconRootsAcc.SetCode(common.Hex2Bytes("3373fffffffffffffffffffffffffffffffffffffffe14604d57602036146024575f5ffd5b5f35801560495762001fff810690815414603c575f5ffd5b62001fff01545f5260205ff35b5f5ffd5b62001fff42064281555f359062001fff015500"))

	genesis.AllocGenesis(BEACON_ROOTS_ADDRESS, beaconRootsAcc)

	warmCoinbaseAcc.SetBalance(common.Big0)
	warmCoinbaseAcc.SetCode(warmCoinbaseCode)

	genesis.AllocGenesis(WARM_COINBASE_ADDRESS, warmCoinbaseAcc)

	push0Acc.SetBalance(common.Big0)
	push0Acc.SetCode(push0Code)

	genesis.AllocGenesis(PUSH0_ADDRESS, push0Acc)
	return genesis
}

func (cs *CancunBaseSpec) GetPreShapellaBlockCount() int {
	return int(cs.BaseSpec.ForkHeight)
}

// Get the per-block timestamp increments configured for this test
func (cs *CancunBaseSpec) GetBlockTimeIncrements() uint64 {
	return 1
}

func (cs *CancunBaseSpec) waitForSetup(t *test.Env) {
	preShapellaBlocksTime := time.Duration(uint64(cs.GetPreShapellaBlockCount())*cs.GetBlockTimeIncrements()) * time.Second
	endOfSetupTimestamp := time.Unix(int64(*t.Genesis.Config().ShanghaiTime), 0).Add(-preShapellaBlocksTime)
	defer func() {
		t.CLMock.LatestHeader.Time = uint64(endOfSetupTimestamp.Unix())
	}()
	if time.Now().Unix() < endOfSetupTimestamp.Unix() {
		durationUntilFuture := time.Until(endOfSetupTimestamp)
		if durationUntilFuture > 0 {
			t.Logf("INFO: Waiting for setup: ~ %.2f min...", durationUntilFuture.Minutes())
			time.Sleep(durationUntilFuture)
		}
	}
}

// Base test case execution procedure for blobs tests.
func (cs *CancunBaseSpec) Execute(t *test.Env) {

	t.CLMock.WaitForTTD()
	cs.waitForSetup(t)

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
