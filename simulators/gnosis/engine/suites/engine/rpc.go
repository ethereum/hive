package suite_engine

import (
	"fmt"
	"math/big"

	"github.com/ethereum/hive/simulators/ethereum/engine/clmock"
	"github.com/ethereum/hive/simulators/ethereum/engine/config"
	"github.com/ethereum/hive/simulators/ethereum/engine/helper"
	"github.com/ethereum/hive/simulators/ethereum/engine/test"
	typ "github.com/ethereum/hive/simulators/ethereum/engine/types"
)

type BlockStatusRPCCheckType string

const (
	LatestOnNewPayload            BlockStatusRPCCheckType = "Latest Block on NewPayload"
	LatestOnHeadBlockHash         BlockStatusRPCCheckType = "Latest Block on HeadBlockHash Update"
	SafeOnSafeBlockHash           BlockStatusRPCCheckType = "Safe Block on SafeBlockHash Update"
	FinalizedOnFinalizedBlockHash BlockStatusRPCCheckType = "Finalized Block on FinalizedBlockHash Update"
)

type BlockStatus struct {
	test.BaseSpec
	CheckType BlockStatusRPCCheckType
	// TODO: Syncing   bool
}

func (s BlockStatus) WithMainFork(fork config.Fork) test.Spec {
	specCopy := s
	specCopy.MainFork = fork
	return specCopy
}

func (b BlockStatus) GetName() string {
	return fmt.Sprintf("RPC: %s", b.CheckType)
}

// Test to verify Block information available at the Eth RPC after NewPayload/ForkchoiceUpdated
func (b BlockStatus) Execute(t *test.Env) {
	switch b.CheckType {
	case SafeOnSafeBlockHash, FinalizedOnFinalizedBlockHash:
		var number *big.Int
		if b.CheckType == SafeOnSafeBlockHash {
			number = Safe
		} else {
			number = Finalized
		}
		p := t.TestEngine.TestHeaderByNumber(number)
		p.ExpectError()
	}

	// Produce blocks before starting the test
	t.CLMock.ProduceBlocks(5, clmock.BlockProcessCallbacks{})

	var tx typ.Transaction
	callbacks := clmock.BlockProcessCallbacks{
		OnPayloadProducerSelected: func() {
			var err error
			tx, err = t.SendNextTransaction(
				t.TestContext,
				t.Engine, &helper.BaseTransactionCreator{
					Recipient:  &ZeroAddr,
					Amount:     big1,
					Payload:    nil,
					TxType:     t.TestTransactionType,
					GasLimit:   75000,
					ForkConfig: t.ForkConfig,
				},
			)
			if err != nil {
				t.Fatalf("FAIL (%s): Error trying to send transaction: %v", err)
			}
		},
	}

	switch b.CheckType {
	case LatestOnNewPayload:
		callbacks.OnGetPayload = func() {
			r := t.TestEngine.TestHeaderByNumber(Head)
			r.ExpectHash(t.CLMock.LatestForkchoice.HeadBlockHash)

			s := t.TestEngine.TestBlockNumber()
			s.ExpectNumber(t.CLMock.LatestHeadNumber.Uint64())

			p := t.TestEngine.TestHeaderByNumber(Head)
			p.ExpectHash(t.CLMock.LatestForkchoice.HeadBlockHash)

			// Check that the receipt for the transaction we just sent is still not available
			q := t.TestEngine.TestTransactionReceipt(tx.Hash())
			q.ExpectError()
		}
	case LatestOnHeadBlockHash:
		callbacks.OnForkchoiceBroadcast = func() {
			r := t.TestEngine.TestHeaderByNumber(Head)
			r.ExpectHash(t.CLMock.LatestForkchoice.HeadBlockHash)

			s := t.TestEngine.TestTransactionReceipt(tx.Hash())
			s.ExpectTransactionHash(tx.Hash())
		}
	case SafeOnSafeBlockHash:
		callbacks.OnSafeBlockChange = func() {
			r := t.TestEngine.TestHeaderByNumber(Safe)
			r.ExpectHash(t.CLMock.LatestForkchoice.SafeBlockHash)
		}
	case FinalizedOnFinalizedBlockHash:
		callbacks.OnFinalizedBlockChange = func() {
			r := t.TestEngine.TestHeaderByNumber(Finalized)
			r.ExpectHash(t.CLMock.LatestForkchoice.FinalizedBlockHash)
		}
	}

	// Perform the test
	t.CLMock.ProduceSingleBlock(callbacks)
}
