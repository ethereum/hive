package main

import (
	"context"
	"time"

	"github.com/ethereum/hive/hivesim"
	"github.com/ethereum/hive/optimism"
	"github.com/stretchr/testify/require"
)

var tests = []*optimism.TestSpec{
	{Name: "deposit simple tx through the portal", Run: simplePortalDepositTest},
	{Name: "deposit contract creation through the portal", Run: contractPortalDepositTest},
	{Name: "erc20 roundtrip through the bridge", Run: erc20RoundtripTest},
	{Name: "simple withdrawal", Run: simpleWithdrawalTest},
}

func main() {
	suite := hivesim.Suite{
		Name: "optimism l1ops",
		Description: `
Tests deposits, withdrawals, and other L1-related operations against a running node.
`[1:],
	}

	suite.Add(&hivesim.TestSpec{
		Name:        "l1ops",
		Description: "Tests L1 operations.",
		Run:         runAllTests(tests),
	})

	sim := hivesim.New()
	hivesim.MustRunSuite(sim, suite)
}

func runAllTests(tests []*optimism.TestSpec) func(t *hivesim.T) {
	return func(t *hivesim.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		defer cancel()

		d := optimism.NewDevnet(t)
		require.NoError(t, optimism.StartSequencerDevnet(ctx, d, &optimism.SequencerDevnetParams{
			MaxSeqDrift:   120,
			SeqWindowSize: 120,
			ChanTimeout:   30,
		}))

		optimism.RunTests(ctx, t, &optimism.RunTestsParams{
			Devnet:      d,
			Tests:       tests,
			Concurrency: 40,
		})
	}
}
