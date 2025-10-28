// Defines relevant constants.
package main

import (
	"path/filepath"
	"time"
)

const (
	CLIENT_PORT    = "8545"
	CLIENT_TIMEOUT = 5 * time.Second

	MAX_LINE_LENGTH = 10 * 1024 * 1024

	GENESIS_FILENAME   = "genesis.json"
	CHAIN_RLP_FILENAME = "chain.rlp"
)

var (
	GENESIS_FILEPATH   = filepath.Join(".", "tests", GENESIS_FILENAME)
	CHAIN_RLP_FILEPATH = filepath.Join(".", "tests", CHAIN_RLP_FILENAME)

	GenesisJsonAndChainRlpFileMapping = map[string]string{
		"genesis.json": GENESIS_FILEPATH,
		"chain.rlp":    CHAIN_RLP_FILEPATH,
	}
)
