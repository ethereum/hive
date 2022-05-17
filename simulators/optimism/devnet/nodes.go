package main

import "github.com/ethereum/hive/hivesim"

type Eth1Node struct {
	*hivesim.Client
}

type L2Node struct {
	*hivesim.Client
}

type OpNode struct {
	*hivesim.Client
}

type L2OSNode struct {
	*hivesim.Client
}

type BSSNode struct {
	*hivesim.Client
}
