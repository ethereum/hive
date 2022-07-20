package main

import "github.com/ethereum/hive/hivesim"

type Eth1Node struct {
	*hivesim.Client
	HTTPPort uint16
	WSPort   uint16
}

type L2Node struct {
	*hivesim.Client
	HTTPPort uint16
	WSPort   uint16
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
