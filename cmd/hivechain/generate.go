package main

import (
	"crypto/ecdsa"
	"fmt"
	"math"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus/beacon"
	"github.com/ethereum/go-ethereum/consensus/ethash"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/trie"
	"golang.org/x/exp/slices"
)

// generatorConfig is the configuration of the chain generator.
type generatorConfig struct {
	// genesis options
	forkInterval int    // number of blocks between forks
	lastFork     string // last enabled fork

	// chain options
	modifiers   []string // enabled generators
	txInterval  int      // frequency of blocks containing transactions
	txCount     int      // number of txs in block
	chainLength int      // number of generated blocks

	// output options
	outputs   []string // enabled outputs
	outputDir string   // path where output files should be placed
}

func (cfg generatorConfig) withDefaults() (generatorConfig, error) {
	if cfg.lastFork == "" {
		cfg.lastFork = lastFork
	}
	if cfg.txInterval == 0 {
		cfg.txInterval = 1
	}
	if cfg.outputs == nil {
		cfg.outputs = []string{"genesis", "chain", "txinfo"}
	}
	return cfg, nil
}

// generator is the central object in the chain generation process.
// It holds the configuration, state, and all instantiated transaction generators.
type generator struct {
	cfg      generatorConfig
	genesis  *core.Genesis
	td       *big.Int
	accounts []genAccount

	modlist   []*modifierInstance
	modOffset int

	// for write/export
	blockchain *core.BlockChain
}

type modifierInstance struct {
	name string
	blockModifier
}

type genAccount struct {
	addr common.Address
	key  *ecdsa.PrivateKey
}

func newGenerator(cfg generatorConfig) *generator {
	genesis := cfg.createGenesis()
	return &generator{
		cfg:      cfg,
		genesis:  genesis,
		td:       new(big.Int).Set(genesis.Difficulty),
		modlist:  cfg.createBlockModifiers(),
		accounts: slices.Clone(knownAccounts),
	}
}

func (cfg *generatorConfig) createBlockModifiers() (list []*modifierInstance) {
	for name, new := range modRegistry {
		list = append(list, &modifierInstance{
			name:          name,
			blockModifier: new(),
		})
	}
	slices.SortFunc(list, func(a, b *modifierInstance) int {
		return strings.Compare(a.name, b.name)
	})
	return list
}

// run produces a chain.
func (g *generator) run() error {
	db := rawdb.NewMemoryDatabase()
	triedb := trie.NewDatabase(db, trie.HashDefaults)
	genesis := g.genesis.MustCommit(db, triedb)
	config := g.genesis.Config

	powEngine := ethash.NewFaker()
	posEngine := beacon.New(powEngine)
	engine := posEngine

	// Create the PoW chain.
	chain, _ := core.GenerateChain(config, genesis, engine, db, g.cfg.chainLength, g.modifyBlock)

	// Import the chain. This runs all block validation rules.
	cacheconfig := core.DefaultCacheConfigWithScheme("hash")
	cacheconfig.Preimages = true
	vmconfig := vm.Config{EnablePreimageRecording: true}
	blockchain, err := core.NewBlockChain(db, cacheconfig, g.genesis, nil, engine, vmconfig, nil, nil)
	if err != nil {
		return fmt.Errorf("can't create blockchain: %v", err)
	}
	defer blockchain.Stop()
	if i, err := blockchain.InsertChain(chain); err != nil {
		return fmt.Errorf("chain validation error (block %d): %v", chain[i].Number(), err)
	}

	// Write the outputs.
	g.blockchain = blockchain
	return g.write()
}

func (g *generator) modifyBlock(i int, gen *core.BlockGen) {
	fmt.Println("generating block", gen.Number())
	g.setDifficulty(i, gen)
	g.runModifiers(i, gen)
}

func (g *generator) setDifficulty(i int, gen *core.BlockGen) {
	chaincfg := g.genesis.Config
	mergeblock := chaincfg.MergeNetsplitBlock
	if mergeblock == nil {
		mergeblock = new(big.Int).SetUint64(math.MaxUint64)
	}
	mergecmp := gen.Number().Cmp(mergeblock)
	if mergecmp > 0 {
		gen.SetPoS()
		return
	}

	prev := gen.PrevBlock(i - 1)
	diff := ethash.CalcDifficulty(g.genesis.Config, gen.Timestamp(), prev.Header())
	if mergecmp == 0 {
		gen.SetPoS()
		chaincfg.TerminalTotalDifficulty = new(big.Int).Set(g.td)
	} else {
		g.td = g.td.Add(g.td, diff)
		gen.SetDifficulty(diff)
	}
}

// runModifiers executes the chain modifiers.
func (g *generator) runModifiers(i int, gen *core.BlockGen) {
	if len(g.modlist) == 0 || g.cfg.txInterval == 0 || i%g.cfg.txInterval != 0 {
		return
	}

	ctx := &genBlockContext{index: i, block: gen, gen: g}
	if gen.Number().Uint64() > 0 {
		prev := gen.PrevBlock(-1)
		ctx.gasLimit = core.CalcGasLimit(prev.GasLimit(), g.genesis.GasLimit)
	}

	// Modifier scheduling: we cycle through the available modifiers until enough have
	// executed successfully. It also stops when all of them return false from apply()
	// because this usually means there is no gas left.
	count := 0
	refused := 0 // count of consecutive times apply() returned false
	for ; count < g.cfg.txCount && refused < len(g.modlist); g.modOffset++ {
		index := g.modOffset % len(g.modlist)
		mod := g.modlist[index]
		ok := mod.apply(ctx)
		if ok {
			fmt.Println("    -", mod.name)
			count++
			refused = 0
		} else {
			refused++
		}
	}
}
