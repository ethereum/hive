package suite_withdrawals

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/hive/simulators/ethereum/engine/clmock"
	"github.com/ethereum/hive/simulators/ethereum/engine/test"
)

var (
	big0      = new(big.Int)
	big1      = big.NewInt(1)
	Head      *big.Int // Nil
	Pending   = big.NewInt(-2)
	Finalized = big.NewInt(-3)
	Safe      = big.NewInt(-4)
)

// Execution specification reference:
// https://github.com/ethereum/execution-apis/blob/main/src/engine/specification.md

var Tests = []test.Spec{
	{
		Name:              "Sanity Withdrawal Test",
		Run:               sanityWithdrawalTest,
		TTD:               0,
		ShanghaiTimestamp: big.NewInt(0),
	},
}

// Make a simple withdrawal and verify balance change on FCU.
func sanityWithdrawalTest(t *test.Env) {
	t.CLMock.WaitForTTD()
	withdrawalAddress := common.HexToAddress("0xAA")
	withdrawalAmount := big.NewInt(100)
	t.CLMock.ProduceSingleBlock(clmock.BlockProcessCallbacks{
		OnPayloadProducerSelected: func() {
			withdrawals := make(types.Withdrawals, 0)
			withdrawals = append(withdrawals, &types.Withdrawal{
				Index:     0,
				Validator: 0,
				Address:   withdrawalAddress,
				Amount:    withdrawalAmount,
			})
			t.CLMock.SetNextWithdrawals(withdrawals)
		},
	})

	r := t.TestEngine.TestBalanceAt(withdrawalAddress, nil)
	r.ExpectBalanceEqual(withdrawalAmount)
}
