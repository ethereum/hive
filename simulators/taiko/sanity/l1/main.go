package main

import (
	"context"
	"fmt"
	"github.com/ethereum/hive/hivesim"
	"strconv"
	"time"
)

func main() {
	suite := hivesim.Suite{
		Name:        "Sanity - L1",
		Description: "L1 Sanity test suite",
	}
	suite.Add(&hivesim.ClientTestSpec{
		Name:        "chainId31336",
		Description: "Asserts that the ChainId is equal 31336",
		Run:         func(t *hivesim.T, c *hivesim.Client) { chainId31336(t, c) },
	})

	sim := hivesim.New()
	hivesim.MustRun(sim, suite)
}

func chainId31336(t *hivesim.T, c *hivesim.Client) {
	_, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

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
