package suite_engine

import (
	"context"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/hive/simulators/ethereum/engine/clmock"
	"github.com/ethereum/hive/simulators/ethereum/engine/config"
	"github.com/ethereum/hive/simulators/ethereum/engine/globals"
	"github.com/ethereum/hive/simulators/ethereum/engine/helper"
	"github.com/ethereum/hive/simulators/ethereum/engine/test"
)

type SuggestedFeeRecipientTest struct {
	test.BaseSpec
	TransactionCount int
}

func (s SuggestedFeeRecipientTest) WithMainFork(fork config.Fork) test.Spec {
	specCopy := s
	specCopy.MainFork = fork
	return specCopy
}

func (t SuggestedFeeRecipientTest) GetName() string {
	return fmt.Sprintf("Suggested Fee Recipient Test, %s", t.TestTransactionType)
}

func (tc SuggestedFeeRecipientTest) Execute(t *test.Env) {
	// Create a single block to not having to build on top of genesis
	t.CLMock.ProduceSingleBlock(clmock.BlockProcessCallbacks{})

	// Verify that, in a block with transactions, fees are accrued by the suggestedFeeRecipient
	feeRecipient := common.Address{}
	t.Rand.Read(feeRecipient[:])
	txRecipient := common.Address{}
	t.Rand.Read(txRecipient[:])

	// Send multiple transactions
	for i := 0; i < tc.TransactionCount; i++ {
		_, err := t.SendNextTransaction(
			t.TestContext,
			t.Engine,
			&helper.BaseTransactionCreator{
				Recipient:  &txRecipient,
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
	}
	// Produce the next block with the fee recipient set
	t.CLMock.NextFeeRecipient = feeRecipient
	t.CLMock.ProduceSingleBlock(clmock.BlockProcessCallbacks{})

	// Calculate the fees and check that they match the balance of the fee recipient
	r := t.TestEngine.TestBlockByNumber(Head)
	r.ExpectTransactionCountEqual(tc.TransactionCount)
	r.ExpectCoinbase(feeRecipient)
	blockIncluded := r.Block

	feeRecipientFees := big.NewInt(0)
	for _, tx := range blockIncluded.Transactions() {
		effGasTip, err := tx.EffectiveGasTip(blockIncluded.BaseFee())
		if err != nil {
			t.Fatalf("FAIL (%s): unable to obtain EffectiveGasTip: %v", t.TestName, err)
		}
		ctx, cancel := context.WithTimeout(t.TestContext, globals.RPCTimeout)
		defer cancel()
		receipt, err := t.Eth.TransactionReceipt(ctx, tx.Hash())
		if err != nil {
			t.Fatalf("FAIL (%s): unable to obtain receipt: %v", t.TestName, err)
		}
		feeRecipientFees = feeRecipientFees.Add(feeRecipientFees, effGasTip.Mul(effGasTip, big.NewInt(int64(receipt.GasUsed))))
	}

	s := t.TestEngine.TestBalanceAt(feeRecipient, nil)
	s.ExpectBalanceEqual(feeRecipientFees)

	// Produce another block without txns and get the balance again
	t.CLMock.NextFeeRecipient = feeRecipient
	t.CLMock.ProduceSingleBlock(clmock.BlockProcessCallbacks{})

	s = t.TestEngine.TestBalanceAt(feeRecipient, nil)
	s.ExpectBalanceEqual(feeRecipientFees)
}
