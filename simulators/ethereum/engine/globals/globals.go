package globals

import (
	"github.com/ethereum/go-ethereum/crypto"
	"crypto/ecdsa"
	"fmt"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/crypto"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/hive/hivesim"
)

type TestAccount struct {
	KeyHex  string
	Address *common.Address
	Key     *ecdsa.PrivateKey
}

func (a *TestAccount) GetKey() *ecdsa.PrivateKey {
	if a.Key == nil {
		key, err := crypto.HexToECDSA(a.KeyHex)
		if err != nil {
			panic(err)
		}
		a.Key = key
	}
	return a.Key
}

func (a *TestAccount) GetAddress() common.Address {
	if a.Address == nil {
		key := a.GetKey()
		addr := crypto.PubkeyToAddress(key.PublicKey)
		a.Address = &addr
	}
	return *a.Address
}

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

	// Accounts used for testing, including the vault account
	TestAccounts = []*TestAccount{
		{
			KeyHex: "63b508a03c3b5937ceb903af8b1b0c191012ef6eb7e9c3fb7afa94e5d214d376",
		},
		{
			KeyHex: "073786d6a43474fda30a10eb7b5c3a47a81dd1e2bf72cfb7f66d868ff4f9827c",
		},
		{
			KeyHex: "c99aa79d5b71da1ddbb020d9ca8c98fffb83b8317f4e06d9960e1ed102eaae73",
		},
		{
			KeyHex: "fcb29daf153be371a3cbaaf58ecd0c675b0ae83ca3ea61c33f71be795b007df1",
		},
		{
			KeyHex: "eac70caf07cca9ad271c9d80523b7e320013bd1d6bc941ef2b6eb51979323e62",
		},
		{
			KeyHex: "06ce09a48d7034b06b1c290ff88f26351d3d21001e82df46aaae9d2f6338b50d",
		},
		{
			KeyHex: "03f9ae8a83bbcbbbdc979ea4f22b040d82c5a32785488df63ec3066c8d89078a",
		},
		{
			KeyHex: "39731ae349b81dd5e7ad9f1e97e63883f4e469e46c5ce6aa6a1b6229a68a2c23",
		},
		{
			KeyHex: "f298d89779b49d5431162a40492b641f9c09d319fb0bdeffd94ad4b8919d9ccf",
		},
		{
			KeyHex: "c2a53ab2068f63f2fad05fa6b3b23ae997ceb68a4b7acac17c2354aeffb624a8",
		},
	}

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

// Global types
type ForkConfig struct {
	ShanghaiTimestamp *big.Int
	CancunTimestamp   *big.Int
}

func (f *ForkConfig) ConfigGenesis(genesis *core.Genesis) error {
	if f.ShanghaiTimestamp != nil {
		shanghaiTime := f.ShanghaiTimestamp.Uint64()
		genesis.Config.ShanghaiTime = &shanghaiTime

		if genesis.Timestamp >= shanghaiTime {
			// Remove PoW altogether
			genesis.Difficulty = common.Big0
			genesis.Config.TerminalTotalDifficulty = common.Big0
			genesis.Config.Clique = nil
			genesis.ExtraData = []byte{}
		}
	}
	if f.CancunTimestamp != nil {
		if genesis.Config.ShanghaiTime == nil {
			return fmt.Errorf("Cancun fork requires Shanghai fork")
		}
		cancunTime := f.CancunTimestamp.Uint64()
		genesis.Config.CancunTime = &cancunTime
		if *genesis.Config.ShanghaiTime > cancunTime {
			return fmt.Errorf("Cancun fork must be after Shanghai fork")
		}
		if genesis.Timestamp >= cancunTime {
			if genesis.BlobGasUsed == nil {
				genesis.BlobGasUsed = new(uint64)
			}
			if genesis.ExcessBlobGas == nil {
				genesis.ExcessBlobGas = new(uint64)
			}
			if genesis.BeaconRoot == nil {
				genesis.BeaconRoot = new(common.Hash)
			}
		}
	}
	return nil
}

func (f *ForkConfig) IsShanghai(blockTimestamp uint64) bool {
	return f.ShanghaiTimestamp != nil && new(big.Int).SetUint64(blockTimestamp).Cmp(f.ShanghaiTimestamp) >= 0
}

func (f *ForkConfig) IsCancun(blockTimestamp uint64) bool {
	return f.CancunTimestamp != nil && new(big.Int).SetUint64(blockTimestamp).Cmp(f.CancunTimestamp) >= 0
}

func (f *ForkConfig) ForkchoiceUpdatedVersion(timestamp uint64) int {
	if f.IsCancun(timestamp) {
		return 3
	} else if f.IsShanghai(timestamp) {
		return 2
	}
	return 1
}

func (f *ForkConfig) NewPayloadVersion(timestamp uint64) int {
	if f.IsCancun(timestamp) {
		return 3
	} else if f.IsShanghai(timestamp) {
		return 2
	}
	return 1
}

func (f *ForkConfig) GetPayloadVersion(timestamp uint64) int {
	if f.IsCancun(timestamp) {
		return 3
	} else if f.IsShanghai(timestamp) {
		return 2
	}
	return 1
}
