// # Test suite for EIP-4844 tests
package suite_eip4844

import (
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus/misc"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/hive/simulators/ethereum/engine/clmock"
	"github.com/ethereum/hive/simulators/ethereum/engine/globals"
	"github.com/ethereum/hive/simulators/ethereum/engine/helper"
	suite_withdrawals "github.com/ethereum/hive/simulators/ethereum/engine/suites/withdrawals"
	"github.com/ethereum/hive/simulators/ethereum/engine/test"
)

var (
	InvalidParamsError = -32602
)

var Tests = []test.SpecInterface{
	&EIP4844BaseSpec{
		WithdrawalsBaseSpec: suite_withdrawals.WithdrawalsBaseSpec{
			Spec: test.Spec{
				Name:  "Eip4844 Fork on Genesis",
				About: "Tests the eip4844 fork happening since genesis (e.g. on a testnet).",
			},
		},
		EIP4844ForkHeight: 0,
	},

	&EIP4844BaseSpec{
		WithdrawalsBaseSpec: suite_withdrawals.WithdrawalsBaseSpec{
			Spec: test.Spec{
				Name: "Eip4844 Fork on Block 1",
				About: `
			Tests the eip4844 fork happening directly after genesis.
			Asserts that excessDataGas is zero.
			`,
			},
		},
		EIP4844ForkHeight:          1,
		EIP4844BlockCount:          1,
		BlobTransactionsPerPayload: 0,
	},

	&EIP4844BaseSpec{
		WithdrawalsBaseSpec: suite_withdrawals.WithdrawalsBaseSpec{
			Spec: test.Spec{
				Name: "Eip4844 Fork on Block 1 with blobs",
				About: `
			Tests the eip4844 fork happening directly after genesis.
			`,
			},
		},
		EIP4844ForkHeight:          1,
		EIP4844BlockCount:          1,
		BlobTransactionsPerPayload: 1,
	},

	&EIP4844BaseSpec{
		WithdrawalsBaseSpec: suite_withdrawals.WithdrawalsBaseSpec{
			Spec: test.Spec{
				Name: "Eip4844 Fork on Block 2 with blobs",
				About: `
			Tests the eip4844 fork after a single block has happened.
			Block 1 is sent with invalid non-null excessDataGas payload and
			client is expected to respond with the appropriate error.
			`,
			},
		},
		EIP4844ForkHeight:          2,
		EIP4844BlockCount:          1,
		BlobTransactionsPerPayload: 4,
	},

	&EIP4844BaseSpec{
		WithdrawalsBaseSpec: suite_withdrawals.WithdrawalsBaseSpec{
			Spec: test.Spec{
				Name: "Eip4844 Fork on Block 3 with blobs",
				About: `
			Tests the eip4844 fork after two blocks have happened.
			Block 2 is sent with invalid non-null excessDataGas payload and
			client is expected to respond with the appropriate error.
			`,
			},
		},
		EIP4844ForkHeight:          3,
		EIP4844BlockCount:          1,
		BlobTransactionsPerPayload: 4,
	},
}

type EIP4844BaseSpec struct {
	suite_withdrawals.WithdrawalsBaseSpec
	EIP4844ForkHeight          uint64 // EIP-4844 activation fork height
	EIP4844BlockCount          uint64 // Number of blocks on or after the eip4844 fork
	BlobTransactionsPerPayload uint64 // Number of blob transactions per eip4844 payload
	BlobsPerTransaction        uint64 // Number of blobs per blob transaction
	BlobTransactionHistory     map[uint64]*types.Transaction
}

func (e *EIP4844BaseSpec) GetEIP4844GenesisTimeDelta() uint64 {
	return e.EIP4844ForkHeight * e.GetBlockTimeIncrements()
}

func (e *EIP4844BaseSpec) GetEIP4844ForkTime() uint64 {
	return uint64(globals.GenesisTimestamp) + e.GetEIP4844GenesisTimeDelta()
}

func (e *EIP4844BaseSpec) GetForkConfig() test.ForkConfig {
	config := e.WithdrawalsBaseSpec.GetForkConfig()
	config.ShardingTimestamp = big.NewInt(int64(e.GetEIP4844ForkTime()))
	return config
}

func (e *EIP4844BaseSpec) GetPreEIP4844BlockCount() uint64 {
	if e.EIP4844ForkHeight == 0 {
		return 0
	} else {
		return e.EIP4844ForkHeight - 1
	}
}

func (e *EIP4844BaseSpec) GetBlobsPerTransaction() uint64 {
	if e.BlobsPerTransaction == 0 {
		return 1
	}
	return e.BlobsPerTransaction
}

func (e *EIP4844BaseSpec) Execute(t *test.Env) {
	t.CLMock.WaitForTTD()

	e.BlobTransactionHistory = make(map[uint64]*types.Transaction)

	// Check if we have pre-Cancun blocks
	if e.GetEIP4844ForkTime() > uint64(globals.GenesisTimestamp) {
		r := t.TestEngine.TestBlockByNumber(nil)
		r.ExpectationDescription = `
		Requested "latest" block expecting genesis to contain
		excessDataGas=nil, because genesis.timestamp < eip4844Time
		`
		r.ExpectExcessDataGas(nil)
	} else {
		// Genesis is post Cancun, so we expect the block to have excessDataGas=0
		r := t.TestEngine.TestBlockByNumber(nil)
		r.ExpectationDescription = `
		Requested "latest" block expecting genesis to contain
		excessDataGas=0, because genesis.timestamp >= eip4844Time
		`
		r.ExpectExcessDataGas(new(big.Int))
	}

	//e.WithdrawalsBaseSpec.Execute(t)

	// TODO: withdrawals testing after the eip4844 fork

	t.CLMock.ProduceBlocks(int(e.GetPreWithdrawalsBlockCount()), clmock.BlockProcessCallbacks{})

	// Produce any remaining blocks necessary to reach eip4844 fork
	blocksTillCancun := e.GetPreEIP4844BlockCount() - e.GetPreWithdrawalsBlockCount()
	t.CLMock.ProduceBlocks(int(blocksTillCancun), clmock.BlockProcessCallbacks{
		OnGetPayload: func() {
			// Send produced payload but try to include non-nil
			// `excessDataGas`, it should fail.
			eip4844Payload, err := helper.CustomizePayload(&t.CLMock.LatestPayloadBuilt, &helper.CustomPayloadData{
				ExcessDataGas: big.NewInt(0),
			})
			if err != nil {
				t.Fatalf("Unable to append excessDataGas: %v", err)
			}
			r := t.TestEngine.TestEngineNewPayloadV3(eip4844Payload)
			r.ExpectationDescription = "Sent pre-cancun payload using NewPayloadV3+ExcessDataGas, INVALID status is expected"
			r.ExpectStatus(test.Invalid)
		},
		OnNewPayloadBroadcast: func() {
			// We sent a pre-cancun FCU.
			// Keep expecting `nil` until Cancun.
			r := t.TestEngine.TestBlockByNumber(nil)
			r.ExpectationDescription = fmt.Sprintf(`
				Requested "latest" block expecting block to contain
				excessDataGas=nil, because (block %d).timestamp < shardingTime
				`,
				t.CLMock.LatestPayloadBuilt.Number,
			)
			r.ExpectExcessDataGas(nil)
		},
	})

	createBlobs := func(i uint64) types.Blobs {
		blobs := make([]types.Blob, e.GetBlobsPerTransaction())
		for j := range blobs {
			id := []byte(fmt.Sprintf("%s-%d-%d", t.TestName, i, j))
			copy(blobs[j][:], id)
		}
		return blobs
	}

	prevExcessDataGas := new(big.Int)
	// Produce post-Cancun blocks
	t.CLMock.ProduceBlocks(int(e.EIP4844BlockCount), clmock.BlockProcessCallbacks{
		OnPayloadProducerSelected: func() {
			t.Logf("INFO (%s) Creating %d blob transactions", t.TestName, e.BlobTransactionsPerPayload)
			// Add blob transactions
			for i := uint64(0); i < e.BlobTransactionsPerPayload; i++ {
				blobs := createBlobs(i)
				tx, err := helper.SendNextTransaction(
					t.TestContext,
					t.CLMock.NextBlockProducer,
					&helper.BaseTransactionCreator{
						Recipient: &globals.PrevRandaoContractAddr,
						Amount:    common.Big1,
						Payload:   nil,
						TxType:    helper.BlobTxOnly,
						Blobs:     blobs,
						GasLimit:  210000,
					},
				)
				if err != nil {
					t.Fatalf("FAIL (%s): Error trying to send transaction: %v", t.TestName, err)
				}
				e.BlobTransactionHistory[t.CLMock.CurrentPayloadNumber] = tx
			}
		},
		OnForkchoiceBroadcast: func() {
			// Assuming one blob per transaction
			if len(t.CLMock.LatestBlobsBundle.Blobs) != int(e.BlobTransactionsPerPayload) {
				t.Fatalf("FAIL (%s): Invalid number of blobs in bundle, expected %d, got %d", t.TestName, e.BlobTransactionsPerPayload, len(t.CLMock.LatestBlobsBundle.Blobs))
			}

			r := t.TestEngine.TestBlockByNumber(nil)
			r.ExpectTransactionCountEqual(int(e.BlobTransactionsPerPayload))

			// Sanity check included blob transactions
			for _, expectedTx := range e.BlobTransactionHistory {
				tx := r.Block.Transaction(expectedTx.Hash())
				if tx.Type() != types.BlobTxType {
					t.Fatalf("FAIL (%s): Invalid transaction type, expected %d, got %d", t.TestName, types.BlobTxType, tx.Type())
				}
				dataHashes := tx.DataHashes()
				if len(dataHashes) == 0 {
					t.Fatalf("FAIL (%s): Invalid dataHashes length, expected non-zero", t.TestName)
				}
			}

			VerifyComputedExcessDataGas(t, prevExcessDataGas, r.Block)
			prevExcessDataGas.Set(r.Block.Header().ExcessDataGas)
		},
	})
}

func VerifyComputedExcessDataGas(t *test.Env, prevExcessDataGas *big.Int, block *types.Block) {
	if block.Header().ExcessDataGas == nil {
		t.Fatalf("FAIL (%s): Expected non-nil excessDataGas", t.TestName)
	}
	expected := misc.CalcExcessDataGas(prevExcessDataGas, len(block.Transactions()))
	if expected.Cmp(block.ExcessDataGas()) != 0 {
		t.Fatalf("FAIL (%s): Invalid excessDataGas, expected %v, got %v", t.TestName, expected, block.ExcessDataGas())
	}
}
