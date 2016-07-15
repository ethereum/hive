package main

import (
	"flag"
	"log"
	"os"
	"path/filepath"

	"github.com/ethereum/ethash"
	"github.com/ethereum/go-ethereum/logger/glog"
)

var (
	blockFlag = flag.Int("block", 0, "Block number for which to generate the DAG")
	outFlag   = flag.String("out", filepath.Join(os.Getenv("HOME"), ".ethash"), "Output folder in which to generate the DAH")
)

func main() {
	// Enable logging for the DAG generator
	glog.SetV(3)
	glog.SetToStderr(true)

	// Generate the requested DAG
	flag.Parse()
	if err := ethash.MakeDAG(uint64(*blockFlag), *outFlag); err != nil {
		log.Fatalf("Failed to generate DAG: %v", err)
	}
}
