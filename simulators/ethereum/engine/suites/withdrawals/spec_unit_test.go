package suite_withdrawals

import (
	"testing"

	"github.com/ethereum/hive/simulators/ethereum/engine/globals"
)

type BaseSpecExpected struct {
	Spec                             WithdrawalsBaseSpec
	ExpectedBlockTimeIncrements      uint64
	ExpectedWithdrawalsTimestamp     uint64
	ExpectedPreWithdrawalsBlockCount uint64
	ExpectedTotalPayloadCount        uint64
}

var baseSpecTestCases = []BaseSpecExpected{
	{
		Spec:                             WithdrawalsBaseSpec{},
		ExpectedBlockTimeIncrements:      1,
		ExpectedWithdrawalsTimestamp:     uint64(globals.GenesisTimestamp),
		ExpectedPreWithdrawalsBlockCount: 0,
		ExpectedTotalPayloadCount:        0,
	},
	{
		Spec: WithdrawalsBaseSpec{
			WithdrawalsBlockCount: 1,
		},
		ExpectedBlockTimeIncrements:      1,
		ExpectedWithdrawalsTimestamp:     uint64(globals.GenesisTimestamp),
		ExpectedPreWithdrawalsBlockCount: 0,
		ExpectedTotalPayloadCount:        1,
	},
}

func TestBaseSpecFunctions(t *testing.T) {
	for i, tc := range baseSpecTestCases {
		spec := tc.Spec
		if spec.GetBlockTimeIncrements() != tc.ExpectedBlockTimeIncrements {
			t.Fatalf("tc %d: unexpected block timestamp increments, expected=%d, got=%d", i, tc.ExpectedBlockTimeIncrements, spec.GetBlockTimeIncrements())
		}
		if spec.GetWithdrawalsForkTime() != tc.ExpectedWithdrawalsTimestamp {
			t.Fatalf("tc %d: unexpected withdrawals timestamp, expected=%d, got=%d", i, tc.ExpectedWithdrawalsTimestamp, spec.GetWithdrawalsForkTime())
		}
		if spec.GetPreWithdrawalsBlockCount() != tc.ExpectedPreWithdrawalsBlockCount {
			t.Fatalf("tc %d: unexpected pre-withdrawals block count, expected=%d, got=%d", i, tc.ExpectedPreWithdrawalsBlockCount, spec.GetPreWithdrawalsBlockCount())
		}
		if spec.GetTotalPayloadCount() != tc.ExpectedTotalPayloadCount {
			t.Fatalf("tc %d: unexpected total payload count, expected=%d, got=%d", i, tc.ExpectedTotalPayloadCount, spec.GetTotalPayloadCount())
		}
	}
}

type ReOrgSpecExpected struct {
	Spec                                   WithdrawalsReorgSpec
	ExpectedSidechainSplitHeight           uint64
	ExpectedSidechainBlockTimeIncrements   uint64
	ExpectedSidechainWithdrawalsForkHeight uint64
}

var reorgSpecTestCases = []ReOrgSpecExpected{
	{
		Spec: WithdrawalsReorgSpec{
			WithdrawalsBaseSpec: WithdrawalsBaseSpec{
				WithdrawalsForkHeight: 1,
				WithdrawalsBlockCount: 16,
			},
			ReOrgBlockCount: 1,
		},
		ExpectedSidechainSplitHeight:           16,
		ExpectedSidechainBlockTimeIncrements:   1,
		ExpectedSidechainWithdrawalsForkHeight: 1,
	},
	{
		Spec: WithdrawalsReorgSpec{
			WithdrawalsBaseSpec: WithdrawalsBaseSpec{
				TimeIncrements:        12,
				WithdrawalsForkHeight: 4,
				WithdrawalsBlockCount: 1,
			},
			ReOrgBlockCount:         1,
			SidechainTimeIncrements: 1,
		},
		ExpectedSidechainSplitHeight:           4,
		ExpectedSidechainBlockTimeIncrements:   1,
		ExpectedSidechainWithdrawalsForkHeight: 15,
	},
	{
		Spec: WithdrawalsReorgSpec{
			WithdrawalsBaseSpec: WithdrawalsBaseSpec{
				TimeIncrements:        1,
				WithdrawalsForkHeight: 4,
				WithdrawalsBlockCount: 4,
			},
			ReOrgBlockCount:         6,
			SidechainTimeIncrements: 4,
		},
		ExpectedSidechainSplitHeight:           2,
		ExpectedSidechainBlockTimeIncrements:   4,
		ExpectedSidechainWithdrawalsForkHeight: 2,
	},
	{
		Spec: WithdrawalsReorgSpec{
			WithdrawalsBaseSpec: WithdrawalsBaseSpec{
				TimeIncrements:        1,
				WithdrawalsForkHeight: 8,
				WithdrawalsBlockCount: 8,
			},
			ReOrgBlockCount:         10,
			SidechainTimeIncrements: 2,
		},
		ExpectedSidechainSplitHeight:           6,
		ExpectedSidechainBlockTimeIncrements:   2,
		ExpectedSidechainWithdrawalsForkHeight: 7,
	},
	{
		Spec: WithdrawalsReorgSpec{
			WithdrawalsBaseSpec: WithdrawalsBaseSpec{
				TimeIncrements:        2,
				WithdrawalsForkHeight: 8,
				WithdrawalsBlockCount: 8,
			},
			ReOrgBlockCount:         10,
			SidechainTimeIncrements: 1,
		},
		ExpectedSidechainSplitHeight:           6,
		ExpectedSidechainBlockTimeIncrements:   1,
		ExpectedSidechainWithdrawalsForkHeight: 11,
	},
}

func TestReorgSpecFunctions(t *testing.T) {
	for i, tc := range reorgSpecTestCases {
		spec := tc.Spec
		if spec.GetSidechainSplitHeight() != tc.ExpectedSidechainSplitHeight {
			t.Fatalf("tc %d: unexpected sidechain split height, expected=%d, got=%d", i, tc.ExpectedSidechainSplitHeight, spec.GetSidechainSplitHeight())
		}
		if spec.GetSidechainBlockTimeIncrements() != tc.ExpectedSidechainBlockTimeIncrements {
			t.Fatalf("tc %d: unexpected sidechain block timestamp increments, expected=%d, got=%d", i, tc.ExpectedSidechainBlockTimeIncrements, spec.GetSidechainBlockTimeIncrements())
		}
		if spec.GetSidechainWithdrawalsForkHeight() != tc.ExpectedSidechainWithdrawalsForkHeight {
			t.Fatalf("tc %d: unexpected sidechain withdrawals fork height, expected=%d, got=%d", i, tc.ExpectedSidechainWithdrawalsForkHeight, spec.GetSidechainWithdrawalsForkHeight())
		}
	}
}
