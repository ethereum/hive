package suite_engine

import (
	"context"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/hive/simulators/ethereum/engine/clmock"
	"github.com/ethereum/hive/simulators/ethereum/engine/config"
	"github.com/ethereum/hive/simulators/ethereum/engine/globals"
	"github.com/ethereum/hive/simulators/ethereum/engine/helper"
	"github.com/ethereum/hive/simulators/ethereum/engine/test"
	typ "github.com/ethereum/hive/simulators/ethereum/engine/types"
)

type PrevRandaoTransactionTest struct {
	test.BaseSpec
	BlockCount int
}

func (s PrevRandaoTransactionTest) WithMainFork(fork config.Fork) test.Spec {
	specCopy := s
	specCopy.MainFork = fork
	return specCopy
}

func (t PrevRandaoTransactionTest) GetName() string {
	return fmt.Sprintf("PrevRandao Opcode Transactions Test, %s", t.TestTransactionType)
}

func (tc PrevRandaoTransactionTest) Execute(t *test.Env) {
	// Create a single block to not having to build on top of genesis
	t.CLMock.ProduceSingleBlock(clmock.BlockProcessCallbacks{})

	startBlockNumber := t.CLMock.LatestHeader.Number.Uint64() + 1

	// Send transactions in PoS, the value of the storage in these blocks must match the prevRandao value
	var (
		blockCount     = 10
		currentTxIndex = 0
		txs            = make([]typ.Transaction, 0)
	)
	if tc.BlockCount > 0 {
		blockCount = tc.BlockCount
	}
	t.CLMock.ProduceBlocks(blockCount, clmock.BlockProcessCallbacks{
		OnPayloadProducerSelected: func() {
			tx, err := t.SendNextTransaction(
				t.TestContext,
				t.Engine,
				&helper.BaseTransactionCreator{
					Recipient:  &globals.PrevRandaoContractAddr,
					Amount:     big0,
					Payload:    nil,
					TxType:     t.TestTransactionType,
					GasLimit:   75000,
					ForkConfig: t.ForkConfig,
				},
			)
			if err != nil {
				t.Fatalf("FAIL (%s): Error trying to send transaction: %v", t.TestName, err)
			}
			txs = append(txs, tx)
			currentTxIndex++
		},
		OnForkchoiceBroadcast: func() {
			// Check the transaction tracing, which is client specific
			expectedPrevRandao := t.CLMock.PrevRandaoHistory[t.CLMock.LatestHeader.Number.Uint64()+1]
			ctx, cancel := context.WithTimeout(t.TestContext, globals.RPCTimeout)
			defer cancel()
			if err := helper.DebugPrevRandaoTransaction(ctx, t.Client.RPC(), t.Client.Type, txs[currentTxIndex-1],
				&expectedPrevRandao); err != nil {
				t.Fatalf("FAIL (%s): Error during transaction tracing: %v", t.TestName, err)
			}
		},
	})

	for i := uint64(startBlockNumber); i <= t.CLMock.LatestExecutedPayload.Number; i++ {
		checkPrevRandaoValue(t, t.CLMock.PrevRandaoHistory[i], i)
	}
}

func checkPrevRandaoValue(t *test.Env, expectedPrevRandao common.Hash, blockNumber uint64) {
	storageKey := common.Hash{}
	storageKey[31] = byte(blockNumber)
	r := t.TestEngine.TestStorageAt(globals.PrevRandaoContractAddr, storageKey, nil)
	r.ExpectStorageEqual(expectedPrevRandao)
}
