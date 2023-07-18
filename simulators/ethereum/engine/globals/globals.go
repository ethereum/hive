package globals

import (
	"github.com/ethereum/go-ethereum/crypto"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/hive/hivesim"
)

var (

	// Test chain parameters
	ChainID          = big.NewInt(7)
	GasPrice         = big.NewInt(30 * params.GWei)
	GasTipPrice      = big.NewInt(1 * params.GWei)
	NetworkID        = big.NewInt(7)
	GenesisTimestamp = int64(0x1234)

	// RPC Timeout for every call
	RPCTimeout = 100 * time.Second

	// Engine, Eth ports
	EthPortHTTP    = 8545
	EnginePortHTTP = 8551

	// JWT Authentication Related
	DefaultJwtTokenSecretBytes = []byte("secretsecretsecretsecretsecretse") // secretsecretsecretsecretsecretse
	MaxTimeDriftSeconds        = int64(60)

	// This is the account that sends vault funding transactions.
	VaultAccountAddress    = common.HexToAddress("0x59f80ed315477f9f0059D862713A7b082A599217")
	VaultKey, _            = crypto.HexToECDSA("ff804d09c833619af673fa99c92ae506d30ff60f37ad41a3d098dcf714db1e4a")
	GnoVaultAccountAddress = common.HexToAddress("0xcC4e00A72d871D6c328BcFE9025AD93d0a26dF51")
	GnoVaultVaultKey, _    = crypto.HexToECDSA("82fcff5c93519f3615d6a92a5a7d146ee305082d3d768d63eb1b45f11f419346")

	// Global test case timeout
	DefaultTestCaseTimeout = time.Minute * 10

	// Confirmation blocks
	PoWConfirmationBlocks = uint64(15)
	PoSConfirmationBlocks = uint64(1)

	// Test related
	PrevRandaoContractAddr = common.HexToAddress("0000000000000000000000000000000000000316")

	// Clique Related
	MinerPKHex   = "9c647b8b7c4e7c3490668fb6c11473619db80c93704c70893d3813af4090c39c"
	MinerAddrHex = "658bdf435d810c91414ec09147daa6db62406379"

	DefaultClientEnv = hivesim.Params{
		"HIVE_NETWORK_ID":          NetworkID.String(),
		"HIVE_CHAIN_ID":            ChainID.String(),
		"HIVE_FORK_HOMESTEAD":      "0",
		"HIVE_FORK_TANGERINE":      "0",
		"HIVE_FORK_SPURIOUS":       "0",
		"HIVE_FORK_BYZANTIUM":      "0",
		"HIVE_FORK_CONSTANTINOPLE": "0",
		"HIVE_FORK_PETERSBURG":     "0",
		"HIVE_FORK_ISTANBUL":       "0",
		"HIVE_FORK_MUIR_GLACIER":   "0",
		"HIVE_FORK_BERLIN":         "0",
		"HIVE_FORK_LONDON":         "0",
		// Tests use clique PoA to mine new blocks until the TTD is reached, then PoS takes over.
		"HIVE_CLIQUE_PERIOD":     "1",
		"HIVE_CLIQUE_PRIVATEKEY": MinerPKHex,
		"HIVE_MINER":             MinerAddrHex,
		// Merge related
		"HIVE_MERGE_BLOCK_ID": "100",
	}
)
