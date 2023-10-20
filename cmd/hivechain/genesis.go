package main

import (
	"fmt"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/params"
	"golang.org/x/exp/slices"
)

var initialBalance, _ = new(big.Int).SetString("1000000000000000000000000000000000000", 10)

const (
	ethashMinimumDifficulty = 131072
	genesisBaseFee          = params.InitialBaseFee
	blocktimeSec            = 10 // hard-coded in core.GenerateChain
)

// Ethereum mainnet forks in order of introduction.
var (
	allForkNames = append(numberBasedForkNames, timeBasedForkNames...)
	lastFork     = allForkNames[len(allForkNames)-1]

	numberBasedForkNames = []string{
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

	timeBasedForkNames = []string{
		"shanghai",
		"cancun",
		"prague",
	}
)

// createChainConfig creates a chain configuration.
func (cfg *generatorConfig) createChainConfig() *params.ChainConfig {
	chaincfg := new(params.ChainConfig)

	chainid, _ := new(big.Int).SetString("35039958740849263516136087381459012528369084397061947147216452157383146382873", 10)
	chaincfg.ChainID = chainid
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
			chaincfg.TerminalTotalDifficultyPassed = true
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
		chaincfg.TerminalTotalDifficulty = big.NewInt(ethashMinimumDifficulty)
	}

	return chaincfg
}

// createGenesis creates the genesis block and config.
func (cfg *generatorConfig) createGenesis() *core.Genesis {
	var g core.Genesis
	g.Config = cfg.createChainConfig()

	// Block attributes.
	g.Difficulty = big.NewInt(ethashMinimumDifficulty)
	g.ExtraData = []byte("hivechain")
	g.GasLimit = params.GenesisGasLimit * 8
	zero := new(big.Int)
	if g.Config.IsLondon(zero) {
		g.BaseFee = big.NewInt(genesisBaseFee)
	}

	// Initialize allocation.
	// Here we add balance to the known accounts.
	g.Alloc = make(core.GenesisAlloc)
	for _, acc := range knownAccounts {
		g.Alloc[acc.addr] = core.GenesisAccount{Balance: initialBalance}
	}

	// Also deploy the beacon chain deposit contract.
	// dca := common.HexToAddress(depositContractAddr)
	// dcc := hexutil.MustDecode("0x" + depositCode)
	// g.Alloc[dca] = core.GenesisAccount{Code: dcc}

	return &g
}

// forkBlocks computes the block numbers where forks occur. Forks get enabled based on the
// forkInterval. If the total number of requested blocks (chainLength) is lower than
// necessary, the remaining forks activate on the last chain block.
func (cfg *generatorConfig) forkBlocks() map[string]uint64 {
	lastIndex := cfg.lastForkIndex()
	forks := allForkNames[:lastIndex+1]
	forkBlocks := make(map[string]uint64)
	for block := 0; block <= cfg.chainLength && len(forks) > 0; {
		fork := forks[0]
		forks = forks[1:]
		forkBlocks[fork] = uint64(block)
		block += cfg.forkInterval
	}
	for _, f := range forks {
		forkBlocks[f] = uint64(cfg.chainLength)
	}
	return forkBlocks
}

// lastForkIndex returns the index of the latest enabled for in allForkNames.
func (cfg *generatorConfig) lastForkIndex() int {
	if cfg.lastFork == "" {
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
