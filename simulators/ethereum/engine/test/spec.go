package test

import (
	"errors"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/hive/simulators/ethereum/engine/clmock"
	"github.com/ethereum/hive/simulators/ethereum/engine/config"
	"github.com/ethereum/hive/simulators/ethereum/engine/globals"
	"github.com/ethereum/hive/simulators/ethereum/engine/helper"
)

type Spec interface {
	// Execute the test
	Execute(*Env)
	// Configure the CLMocker for this specific test
	ConfigureCLMock(*clmock.CLMocker)
	// Get the name of the test
	GetName() string
	// Get a brief description of the test
	GetAbout() string
	// Get the chain file to initialize the clients
	GetChainFile() string
	// Get the main fork for this test
	GetMainFork() config.Fork
	// Create a copy of the spec with a different main fork
	WithMainFork(config.Fork) Spec
	// Get the fork config for this test
	GetForkConfig() *config.ForkConfig
	// Get the genesis file to initialize the clients
	GetGenesis() *core.Genesis
	// Get the test transaction type to use throughout the test
	GetTestTransactionType() helper.TestTransactionType
	// Get the maximum execution time until a timeout is raised
	GetTimeout() int
	// Get whether mining is disabled for this test
	IsMiningDisabled() bool
}

type BaseSpec struct {
	// Name of the test
	Name string

	// Brief description of the test
	About string

	// Expectation Description
	Expectation string

	// Test procedure to execute the test
	Run func(*Env)

	// TerminalTotalDifficulty delta value.
	// Actual TTD is genesis.Difficulty + this value
	// Default: 0
	TTD int64

	// CL Mocker configuration for time increments
	BlockTimestampIncrement uint64

	// CL Mocker configuration for slots to `safe` and `finalized` respectively
	SlotsToSafe      *big.Int
	SlotsToFinalized *big.Int

	// CL Mocker configuration for SafeSlotsToImportOptimistically
	SafeSlotsToImportOptimistically *big.Int

	// Test maximum execution time until a timeout is raised.
	// Default: 60 seconds
	TimeoutSeconds int

	// Genesis file to be used for all clients launched during test
	// Default: genesis.json (init/genesis.json)
	GenesisFile      string
	GenesisTimestamp *uint64

	// Chain file to initialize the clients.
	// When used, clique consensus mechanism is disabled.
	// Default: None
	ChainFile string

	// Disables clique and PoW mining
	DisableMining bool

	// Transaction type to use throughout the test
	TestTransactionType helper.TestTransactionType

	// Fork Config
	MainFork         config.Fork
	ForkTime         uint64
	ForkHeight       uint64
	PreviousForkTime uint64
}

func (s BaseSpec) Execute(env *Env) {
	s.Run(env)
}

func (s BaseSpec) ConfigureCLMock(cl *clmock.CLMocker) {
	if s.SlotsToSafe != nil {
		cl.SlotsToSafe = s.SlotsToSafe
	}
	if s.SlotsToFinalized != nil {
		cl.SlotsToFinalized = s.SlotsToFinalized
	}
	if s.SafeSlotsToImportOptimistically != nil {
		cl.SafeSlotsToImportOptimistically = s.SafeSlotsToImportOptimistically
	}
	cl.BlockTimestampIncrement = new(big.Int).SetUint64(s.GetBlockTimeIncrements())
}

func (s BaseSpec) GetAbout() string {
	return s.About
}

func (s BaseSpec) GetChainFile() string {
	return s.ChainFile
}

func (s BaseSpec) GetMainFork() config.Fork {
	mainFork := s.MainFork
	if mainFork == "" {
		mainFork = config.Paris
	}
	return mainFork
}

func (s BaseSpec) WithMainFork(fork config.Fork) Spec {
	specCopy := s
	specCopy.MainFork = fork
	return specCopy
}

func (s BaseSpec) GetGenesisTimestamp() uint64 {
	genesisTimestamp := globals.GenesisTimestamp
	if s.GenesisTimestamp != nil {
		genesisTimestamp = *s.GenesisTimestamp
	}
	return genesisTimestamp
}

func (s BaseSpec) GetBlockTimeIncrements() uint64 {
	if s.BlockTimestampIncrement == 0 {
		return 1
	}
	return s.BlockTimestampIncrement
}

func (s BaseSpec) GetBlockTime(blockNumber uint64) uint64 {
	return s.GetGenesisTimestamp() + blockNumber*s.GetBlockTimeIncrements()
}

func (s BaseSpec) GetForkTime() uint64 {
	forkTime := s.ForkTime
	if s.ForkHeight > 0 {
		forkTime = s.GetBlockTime(s.ForkHeight)
	}
	return forkTime
}

func (s BaseSpec) GetForkConfig() *config.ForkConfig {
	forkTime := s.GetForkTime()
	previousForkTime := s.PreviousForkTime
	forkConfig := config.ForkConfig{}
	mainFork := s.GetMainFork()
	genesisTimestamp := s.GetGenesisTimestamp()
	if previousForkTime > forkTime {
		panic(errors.New("previous fork time cannot be greater than fork time"))
	}
	if mainFork == config.Paris {
		if forkTime > genesisTimestamp || previousForkTime != 0 {
			return nil // Cannot configure a fork before Paris, skip test
		}
	} else if mainFork == config.Shanghai {
		if previousForkTime != 0 {
			return nil // Cannot configure a fork before Shanghai, skip test
		}
		forkConfig.ShanghaiTimestamp = new(big.Int).SetUint64(forkTime)
	} else if mainFork == config.Cancun {
		forkConfig.ShanghaiTimestamp = new(big.Int).SetUint64(previousForkTime)
		forkConfig.CancunTimestamp = new(big.Int).SetUint64(forkTime)
	} else {
		panic(fmt.Errorf("unknown fork: %s", mainFork))
	}
	return &forkConfig
}

func (s BaseSpec) GetGenesis() *core.Genesis {
	// Load the default test genesis file
	genesisPath := "./init/genesis.json"
	if s.GenesisFile != "" {
		genesisPath = fmt.Sprintf("./init/%s", s.GenesisFile)
	}
	genesis := helper.LoadGenesis(genesisPath)

	// Set the terminal total difficulty
	genesis.Config.TerminalTotalDifficulty = big.NewInt(genesis.Difficulty.Int64() + s.TTD)
	if genesis.Difficulty.Cmp(genesis.Config.TerminalTotalDifficulty) <= 0 {
		genesis.Config.TerminalTotalDifficultyPassed = true
	}

	// Set the genesis timestamp if provided
	if s.GenesisTimestamp != nil {
		genesis.Timestamp = *s.GenesisTimestamp
	}

	// Add balance to all the test accounts
	for _, testAcc := range globals.TestAccounts {
		balance, ok := new(big.Int).SetString("123450000000000000000", 16)
		if !ok {
			panic(errors.New("failed to parse balance"))
		}
		genesis.Alloc[testAcc.GetAddress()] = core.GenesisAccount{
			Balance: balance,
		}
	}

	return &genesis
}

func (s BaseSpec) GetName() string {
	return s.Name
}

func (s BaseSpec) GetTestTransactionType() helper.TestTransactionType {
	return s.TestTransactionType
}

func (s BaseSpec) GetTimeout() int {
	return s.TimeoutSeconds
}

func (s BaseSpec) IsMiningDisabled() bool {
	return s.DisableMining
}

var LatestFork = config.ForkConfig{
	ShanghaiTimestamp: big.NewInt(0),
}
