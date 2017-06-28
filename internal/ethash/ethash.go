package main

import (
	"flag"
	"os"
	"path/filepath"

	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/ethash"
)

var (
	blockFlag = flag.Int("block", 0, "Block number for which to generate the DAG")
	outFlag   = flag.String("out", filepath.Join(os.Getenv("HOME"), ".ethash"), "Output folder in which to generate the DAH")
)

func main() {
	// Generate the requested DAG
	flag.Parse()
	if err := ethash.MakeDAG(uint64(*blockFlag), *outFlag); err != nil {
		log.Crit("Failed to generate DAG", "err", err)
	}
}
