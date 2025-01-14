package main

import (
	"fmt"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/params"
	"golang.org/x/exp/slices"
)

var initialBalance, _ = new(big.Int).SetString("1000000000000000000000000000000000000", 10)

const (
	genesisBaseFee = params.InitialBaseFee
	blocktimeSec   = 10 // hard-coded in core.GenerateChain
)

// Ethereum mainnet forks in order of introduction.
var (
	allForkNames = append(preMergeForkNames, posForkNames...)
	lastFork     = allForkNames[len(allForkNames)-1]

	// these are block-number based:
	preMergeForkNames = []string{
		"homestead",
		"tangerinewhistle",
		"spuriousdragon",
		"byzantium",
		"constantinople",
		"petersburg",
		"istanbul",
		"muirglacier",
		"berlin",
		"london",
		"arrowglacier",
		"grayglacier",
		"merge",
	}

	// forks after the merge are timestamp-based:
	posForkNames = []string{
		"shanghai",
		"cancun",
		"prague",
	}
)

// createChainConfig creates a chain configuration.
func (cfg *generatorConfig) createChainConfig() *params.ChainConfig {
	chaincfg := new(params.ChainConfig)

	chainid, _ := new(big.Int).SetString("3503995874084926", 10)
	chaincfg.ChainID = chainid

	// Set consensus algorithm.
	chaincfg.Ethash = new(params.EthashConfig)

	// Apply forks.
	forks := cfg.forkBlocks()
	for fork, b := range forks {
		timestamp := cfg.blockTimestamp(b)

		switch fork {
		// number-based forks
		case "homestead":
			chaincfg.HomesteadBlock = new(big.Int).SetUint64(b)
		case "tangerinewhistle":
			chaincfg.EIP150Block = new(big.Int).SetUint64(b)
		case "spuriousdragon":
			chaincfg.EIP155Block = new(big.Int).SetUint64(b)
			chaincfg.EIP158Block = new(big.Int).SetUint64(b)
		case "byzantium":
			chaincfg.ByzantiumBlock = new(big.Int).SetUint64(b)
		case "constantinople":
			chaincfg.ConstantinopleBlock = new(big.Int).SetUint64(b)
		case "petersburg":
			chaincfg.PetersburgBlock = new(big.Int).SetUint64(b)
		case "istanbul":
			chaincfg.IstanbulBlock = new(big.Int).SetUint64(b)
		case "muirglacier":
			chaincfg.MuirGlacierBlock = new(big.Int).SetUint64(b)
		case "berlin":
			chaincfg.BerlinBlock = new(big.Int).SetUint64(b)
		case "london":
			chaincfg.LondonBlock = new(big.Int).SetUint64(b)
		case "arrowglacier":
			chaincfg.ArrowGlacierBlock = new(big.Int).SetUint64(b)
		case "grayglacier":
			chaincfg.GrayGlacierBlock = new(big.Int).SetUint64(b)
		case "merge":
			chaincfg.MergeNetsplitBlock = new(big.Int).SetUint64(b)
		// time-based forks
		case "shanghai":
			chaincfg.ShanghaiTime = &timestamp
		case "cancun":
			chaincfg.CancunTime = &timestamp
		case "prague":
			chaincfg.PragueTime = &timestamp
		default:
			panic(fmt.Sprintf("unknown fork name %q", fork))
		}
	}

	// Special case for merged-from-genesis networks.
	// Need to assign TTD here because the genesis block won't be processed by GenerateChain.
	if chaincfg.MergeNetsplitBlock != nil && chaincfg.MergeNetsplitBlock.Sign() == 0 {
		chaincfg.TerminalTotalDifficulty = cfg.genesisDifficulty()
	}

	return chaincfg
}

func (cfg *generatorConfig) genesisDifficulty() *big.Int {
	return new(big.Int).Set(params.MinimumDifficulty)
}

// createGenesis creates the genesis block and config.
func (cfg *generatorConfig) createGenesis() *core.Genesis {
	var g core.Genesis
	g.Config = cfg.createChainConfig()

	// Block attributes.
	g.Difficulty = cfg.genesisDifficulty()
	g.ExtraData = []byte("hivechain")
	g.GasLimit = params.GenesisGasLimit * 8
	zero := new(big.Int)
	if g.Config.IsLondon(zero) {
		g.BaseFee = big.NewInt(genesisBaseFee)
	}

	// Initialize allocation.
	// Here we add balance to known accounts and initialize built-in contracts.
	g.Alloc = make(types.GenesisAlloc)
	for _, acc := range knownAccounts {
		g.Alloc[acc.addr] = types.Account{Balance: initialBalance}
	}
	addCancunSystemContracts(g.Alloc)
	addPragueSystemContracts(g.Alloc)
	addSnapTestContract(g.Alloc)
	addEmitContract(g.Alloc)

	return &g
}

func addCancunSystemContracts(ga types.GenesisAlloc) {
	ga[params.BeaconRootsAddress] = types.Account{
		Balance: big.NewInt(42),
		Code:    params.BeaconRootsCode,
	}
}

func addPragueSystemContracts(ga types.GenesisAlloc) {
	ga[params.HistoryStorageAddress] = types.Account{Balance: big.NewInt(1), Code: params.HistoryStorageCode}
	ga[params.WithdrawalQueueAddress] = types.Account{Balance: big.NewInt(1), Code: params.WithdrawalQueueCode}
	ga[params.ConsolidationQueueAddress] = types.Account{Balance: big.NewInt(1), Code: params.ConsolidationQueueCode}
}

func addSnapTestContract(ga types.GenesisAlloc) {
	addr := common.HexToAddress("0x8bebc8ba651aee624937e7d897853ac30c95a067")
	h := common.HexToHash
	ga[addr] = types.Account{
		Balance: big.NewInt(1),
		Nonce:   1,
		Storage: map[common.Hash]common.Hash{
			h("0x01"): h("0x01"),
			h("0x02"): h("0x02"),
			h("0x03"): h("0x03"),
		},
	}
}

const emitAddr = "0x7dcd17433742f4c0ca53122ab541d0ba67fc27df"

func addEmitContract(ga types.GenesisAlloc) {
	addr := common.HexToAddress(emitAddr)
	ga[addr] = types.Account{
		Code:    emitCode,
		Balance: new(big.Int),
	}
}

// forkBlocks computes the block numbers where forks occur. Forks get enabled based on the
// forkInterval. If the total number of requested blocks (chainLength) is lower than
// necessary, the remaining forks activate on the last chain block.
func (cfg *generatorConfig) forkBlocks() map[string]uint64 {
	lastIndex := cfg.lastForkIndex()
	forks := allForkNames[:lastIndex+1]
	forkBlocks := make(map[string]uint64)

	// If merged chain is specified, schedule all pre-merge forks at block zero.
	if cfg.merged {
		for _, fork := range preMergeForkNames {
			if len(forks) == 0 {
				break
			}
			forkBlocks[fork] = 0
			if forks[0] != fork {
				panic("unexpected fork in allForkNames: " + forks[0])
			}
			forks = forks[1:]
		}
	}
	// Schedule remaining forks according to interval.
	for block := 0; block <= cfg.chainLength && len(forks) > 0; {
		fork := forks[0]
		forks = forks[1:]
		forkBlocks[fork] = uint64(block)
		block += cfg.forkInterval
	}
	// If the chain length cannot accommodate the spread of forks with the chosen
	// interval, schedule the remaining forks at the last block.
	for _, f := range forks {
		forkBlocks[f] = uint64(cfg.chainLength)
	}
	return forkBlocks
}

// lastForkIndex returns the index of the latest enabled for in allForkNames.
func (cfg *generatorConfig) lastForkIndex() int {
	if cfg.lastFork == "" || cfg.lastFork == "frontier" {
		return len(allForkNames) - 1
	}
	index := slices.Index(allForkNames, strings.ToLower(cfg.lastFork))
	if index == -1 {
		panic(fmt.Sprintf("unknown lastFork name %q", cfg.lastFork))
	}
	return index
}

func (cfg *generatorConfig) blockTimestamp(num uint64) uint64 {
	return num * blocktimeSec
}
