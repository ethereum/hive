package main

import (
	"context"
	"fmt"
	"github.com/ethereum/hive/hivesim"
	"github.com/taikoxyz/hive/taiko"
	"strconv"
	"time"
)

func main() {
	suite := hivesim.Suite{
		Name:        "Sanity - L1 - Protocol",
		Description: "L1 - Protocol Sanity test suite",
	}
	suite.Add(&hivesim.ClientTestSpec{
		Name:        "getCodeTaikoL1",
		Description: "Asserts that the TaikoL1 smart contract is deployed",
		Run:         func(t *hivesim.T, c *hivesim.Client) { getCodeTaikoL1(t, c) },
	})

	sim := hivesim.New()
	hivesim.MustRun(sim, suite)
}

func getCodeTaikoL1(t *hivesim.T, c *hivesim.Client) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	taikoEnv := taiko.NewTestEnv(ctx, t)
	chainIdString := ""
	err := c.RPC().Call(&chainIdString, "eth_chainId")
	if err != nil {
		t.Fatal(err)
	}

	chainId, err := strconv.ParseInt(chainIdString, 0, 0)
	if err != nil {
		fmt.Println("Error: %e", err)
		return
	}
	if chainId != 31336 {
		t.Fatalf("ChainId is not equal 31336, it is %i", chainId)
	}
}
