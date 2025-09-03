package main

import (
	"crypto/ecdsa"
	"fmt"
	"math"
	"math/big"
	"math/rand"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus"
	"github.com/ethereum/go-ethereum/consensus/beacon"
	"github.com/ethereum/go-ethereum/consensus/ethash"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/triedb"
	"golang.org/x/exp/slices"
)

// generatorConfig is the configuration of the chain generator.
type generatorConfig struct {
	// genesis options
	forkInterval int    // number of blocks between forks
	lastFork     string // last enabled fork
	merged       bool   // create a proof-of-stake chain
	berachain    bool   // create a berachain with prague1 fork

	// chain options
	txInterval  int // frequency of blocks containing transactions
	txCount     int // number of txs in block
	chainLength int // number of generated blocks

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
	rand     *rand.Rand

	// Modifier lists.
	virgins   []*modifierInstance
	mods      []*modifierInstance
	modOffset int

	// for write/export
	blockchain *core.BlockChain
	clRequests map[uint64][][]byte
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
		cfg:        cfg,
		genesis:    genesis,
		rand:       rand.New(rand.NewSource(10)),
		td:         new(big.Int).Set(genesis.Difficulty),
		virgins:    cfg.createBlockModifiers(),
		accounts:   slices.Clone(knownAccounts),
		clRequests: make(map[uint64][][]byte),
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

// run produces a chain and writes it.
func (g *generator) run() error {
	db := rawdb.NewMemoryDatabase()
	engine := beacon.New(ethash.NewFaker())

	// Init genesis block.
	trieconfig := *triedb.HashDefaults
	trieconfig.Preimages = true
	triedb := triedb.NewDatabase(db, &trieconfig)
	genesis := g.genesis.MustCommit(db, triedb)
	g.clRequests[0] = [][]byte{}

	// Create the blocks.
	chain, _ := core.GenerateChain(g.genesis.Config, genesis, engine, db, g.cfg.chainLength, g.modifyBlock)

	// Import the chain. This runs all block validation rules.
	bc, err := g.importChain(engine, chain)
	if err != nil {
		return err
	}

	g.blockchain = bc
	return g.write()
}

func (g *generator) importChain(engine consensus.Engine, chain []*types.Block) (*core.BlockChain, error) {
	db := rawdb.NewMemoryDatabase()
	config := core.DefaultConfig().WithStateScheme("hash")
	config.Preimages = true
	blockchain, err := core.NewBlockChain(db, g.genesis, engine, config)
	if err != nil {
		return nil, fmt.Errorf("can't create blockchain: %v", err)
	}

	i, err := blockchain.InsertChain(chain)
	if err != nil {
		blockchain.Stop()
		return nil, fmt.Errorf("chain validation error (block %d): %v", chain[i].Number(), err)
	}
	return blockchain, nil
}

func (g *generator) modifyBlock(i int, gen *core.BlockGen) {
	fmt.Println("generating block", gen.Number())
	g.setDifficulty(gen)
	g.setParentBeaconRoot(gen)
	g.setParentProposerPubkey(gen)
	g.runModifiers(i, gen)
	g.clRequests[gen.Number().Uint64()] = gen.ConsensusLayerRequests()
}

func (g *generator) setDifficulty(gen *core.BlockGen) {
	chaincfg := g.genesis.Config
	mergeblock := chaincfg.MergeNetsplitBlock
	if mergeblock == nil {
		mergeblock = new(big.Int).SetUint64(math.MaxUint64)
	}
	switch gen.Number().Cmp(mergeblock) {
	case 1:
		gen.SetPoS()
	case 0:
		gen.SetPoS()
		chaincfg.TerminalTotalDifficulty = new(big.Int).Set(g.td)
	default:
		g.td = g.td.Add(g.td, gen.Difficulty())
	}
}

func (g *generator) setParentBeaconRoot(gen *core.BlockGen) {
	if g.genesis.Config.IsCancun(gen.Number(), gen.Timestamp()) {
		var h common.Hash
		g.rand.Read(h[:])
		gen.SetParentBeaconRoot(h)
	}
}

func (g *generator) setParentProposerPubkey(gen *core.BlockGen) {
	if g.genesis.Config.IsPrague1(gen.Number(), gen.Timestamp()) {
		var h common.Pubkey
		g.rand.Read(h[:])
		err := gen.SetParentProposerPubkey(h)
		if err != nil {
			panic(err)
		}
	}
}

// runModifiers executes the chain modifiers.
func (g *generator) runModifiers(i int, gen *core.BlockGen) {
	totalMods := len(g.mods) + len(g.virgins)
	if totalMods == 0 || g.cfg.txInterval == 0 || i%g.cfg.txInterval != 0 {
		return
	}

	ctx := &genBlockContext{index: i, block: gen, gen: g}

	// Modifier scheduling: we cycle through the available modifiers until enough have
	// executed successfully. It also stops when all of them return false from apply()
	// because this usually means there is no gas left.
	count := 0
	refused := 0 // count of consecutive times apply() returned false
	run := func(mod *modifierInstance) bool {
		ok := mod.apply(ctx)
		if ok {
			fmt.Println("    -", mod.name)
			count++
			refused = 0
		} else {
			refused++
		}
		return ok
	}

	// In order to avoid a pathological situation where a modifier never executes because
	// of unfortunate scheduling, we first try modifiers from g.virgins.
	for i := 0; i < len(g.virgins) && count < g.cfg.txCount; i++ {
		mod := g.virgins[i]
		if run(mod) {
			g.mods = append(g.mods, mod)
			g.virgins = append(g.virgins[:i], g.virgins[i+1:]...)
			i--
		}
	}
	// If there is any space left, fill it using g.mods.
	for len(g.mods) > 0 && count < g.cfg.txCount && refused < totalMods {
		index := g.modOffset % len(g.mods)
		run(g.mods[index])
		g.modOffset++
	}
}
