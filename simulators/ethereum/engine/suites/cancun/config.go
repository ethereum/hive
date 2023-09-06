package suite_cancun

import (
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/hive/simulators/ethereum/engine/test"
)

// Contains the base spec for all cancun tests.
type CancunBaseSpec struct {
	test.BaseSpec
	GetPayloadDelay uint64 // Delay between FcU and GetPayload calls
	TestSequence
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
