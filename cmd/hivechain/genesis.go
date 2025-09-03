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
		"prague1",
	}

	// berachain-specific forks that extend standard forks:
	berachainForkNames = []string{
		"prague1", // extends prague
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
	if _, ok := forks["cancun"]; ok {
		chaincfg.BlobScheduleConfig = new(params.BlobScheduleConfig)
	}
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
			chaincfg.BlobScheduleConfig.Cancun = params.DefaultCancunBlobConfig
		case "prague":
			chaincfg.PragueTime = &timestamp
			chaincfg.BlobScheduleConfig.Prague = params.DefaultPragueBlobConfig
		case "prague1":
			chaincfg.Berachain.Prague1.Time = &timestamp
			chaincfg.Berachain.Prague1.MinimumBaseFeeWei = 1000000000
			chaincfg.Berachain.Prague1.BaseFeeChangeDenominator = 8
			chaincfg.Berachain.Prague1.PoLDistributorAddress = common.HexToAddress("0x4200000000000000000000000000000000000042")
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
	if cfg.berachain {
		return new(big.Int).SetUint64(0)
	}
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
	if cfg.berachain {
		addMockPoLContracts(g.Config.Berachain, g.Alloc)
	}
	return &g
}

func addCancunSystemContracts(ga types.GenesisAlloc) {
	ga[params.BeaconRootsAddress] = types.Account{
		Balance: big.NewInt(42),
		Code:    params.BeaconRootsCode,
	}
}

func addMockPoLContracts(cfg params.BerachainConfig, ga types.GenesisAlloc) {
	// Copied from the eth-genesis used in beacon-kit local devnet
	ga[cfg.Prague1.PoLDistributorAddress] = types.Account{
		Balance: big.NewInt(0),
		Code:    common.FromHex("608060405234801561000f575f80fd5b5060043610610034575f3560e01c8063163db71b1461003857806360644a6b14610052575b5f80fd5b6100405f5481565b60405190815260200160405180910390f35b610065610060366004610183565b610067565b005b3373fffffffffffffffffffffffffffffffffffffffe146100b4576040517f5e742c5a00000000000000000000000000000000000000000000000000000000815260040160405180910390fd5b5f805490806100c2836101f1565b90915550506040517f999da65b0000000000000000000000000000000000000000000000000000000081527342000000000000000000000000000000000000439063999da65b90610119908590859060040161024d565b5f604051808303815f87803b158015610130575f80fd5b505af1158015610142573d5f803e3d5ffd5b505050507f60b106db8802e863a4a9dc4af78cb0dd63feb55ad4ee60f0453c13309bfdbdd4828260405161017792919061024d565b60405180910390a15050565b5f8060208385031215610194575f80fd5b823567ffffffffffffffff8111156101aa575f80fd5b8301601f810185136101ba575f80fd5b803567ffffffffffffffff8111156101d0575f80fd5b8560208284010111156101e1575f80fd5b6020919091019590945092505050565b5f7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff8203610246577f4e487b71000000000000000000000000000000000000000000000000000000005f52601160045260245ffd5b5060010190565b60208152816020820152818360408301375f818301604090810191909152601f9092017fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffe016010191905056fea2646970667358221220e6dfa8c5d72bb391aff9e891c09a1f86d26cfd1d112aaab4b577bcbc5b94bafe64736f6c634300081a0033"),
		Nonce:   1,
	}
	ga[common.HexToAddress("0x4200000000000000000000000000000000000043")] = types.Account{
		Balance: big.NewInt(0),
		Code:    common.FromHex("608060405234801561000f575f80fd5b5060043610610034575f3560e01c80634b28f9a214610038578063999da65b14610052575b5f80fd5b6100405f5481565b60405190815260200160405180910390f35b6100656100603660046100b8565b610067565b005b5f8054908061007583610126565b91905055507fb3a8fa51f8d3759f320e88b7f8d3fb73a2a51b31b3324b37833c4816cf41e7c45f546040516100ac91815260200190565b60405180910390a15050565b5f80602083850312156100c9575f80fd5b823567ffffffffffffffff8111156100df575f80fd5b8301601f810185136100ef575f80fd5b803567ffffffffffffffff811115610105575f80fd5b856020828401011115610116575f80fd5b6020919091019590945092505050565b5f7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff820361017b577f4e487b71000000000000000000000000000000000000000000000000000000005f52601160045260245ffd5b506001019056fea2646970667358221220e5db2a9698e23f84b7c83d023ea72ca3f01627daffa1ff16ae15918143d892a564736f6c634300081a0033"),
		Nonce:   1,
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

	// Remove berachain-specific forks if berachain is not configured
	if !cfg.berachain {
		var filteredForks []string
		for _, fork := range forks {
			isBerachainFork := false
			for _, beraFork := range berachainForkNames {
				if fork == beraFork {
					isBerachainFork = true
					break
				}
			}
			if !isBerachainFork {
				filteredForks = append(filteredForks, fork)
			}
		}
		forks = filteredForks
	}

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
