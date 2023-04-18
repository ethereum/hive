package test

import (
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/hive/simulators/ethereum/engine/clmock"
	"github.com/ethereum/hive/simulators/ethereum/engine/helper"
)

type ForkConfig struct {
	// Shanghai Fork Timestamp
	ShanghaiTimestamp *big.Int
}

type ConsensusConfig struct {
	SlotsToSafe                     *big.Int
	SlotsToFinalized                *big.Int
	SafeSlotsToImportOptimistically int64
}

type SpecInterface interface {
	Execute(*Env)
	ConfigureCLMock(*clmock.CLMocker)
	GetAbout() string
	GetConsensusConfig() ConsensusConfig
	GetChainFile() string
	GetForkConfig() ForkConfig
	GetGenesis() *core.Genesis
	GetName() string
	GetTestTransactionType() helper.TestTransactionType
	GetTimeout() int
	GetTTD() int64
	IsMiningDisabled() bool
}

type Spec struct {
	// Name of the test
	Name string

	// Brief description of the test
	About string

	// Test procedure to execute the test
	Run func(*Env)

	// TerminalTotalDifficulty delta value.
	// Actual TTD is genesis.Difficulty + this value
	// Default: 0
	TTD int64

	// CL Mocker configuration for slots to `safe` and `finalized` respectively
	SlotsToSafe      *big.Int
	SlotsToFinalized *big.Int

	// CL Mocker configuration for SafeSlotsToImportOptimistically
	SafeSlotsToImportOptimistically int64

	// Test maximum execution time until a timeout is raised.
	// Default: 60 seconds
	TimeoutSeconds int

	// Genesis file to be used for all clients launched during test
	// Default: genesis.json (init/genesis.json)
	GenesisFile string

	// Chain file to initialize the clients.
	// When used, clique consensus mechanism is disabled.
	// Default: None
	ChainFile string

	// Disables clique and PoW mining
	DisableMining bool

	// Transaction type to use throughout the test
	TestTransactionType helper.TestTransactionType

	// Fork Config
	ForkConfig
}

func (s Spec) Execute(env *Env) {
	s.Run(env)
}

func (s Spec) ConfigureCLMock(*clmock.CLMocker) {
	// No-op
}

func (s Spec) GetAbout() string {
	return s.About
}

func (s Spec) GetConsensusConfig() ConsensusConfig {
	return ConsensusConfig{
		SlotsToSafe:                     s.SlotsToSafe,
		SlotsToFinalized:                s.SlotsToFinalized,
		SafeSlotsToImportOptimistically: s.SafeSlotsToImportOptimistically,
	}
}
func (s Spec) GetChainFile() string {
	return s.ChainFile
}

func (s Spec) GetForkConfig() ForkConfig {
	return s.ForkConfig
}

func (s Spec) GetGenesis() *core.Genesis {
	genesisPath := "./init/genesis.json"
	if s.GenesisFile != "" {
		genesisPath = fmt.Sprintf("./init/%s", s.GenesisFile)
	}
	genesis := helper.LoadGenesis(genesisPath)
	return &genesis
}

func (s Spec) GetName() string {
	return s.Name
}

func (s Spec) GetTestTransactionType() helper.TestTransactionType {
	return s.TestTransactionType
}

func (s Spec) GetTimeout() int {
	return s.TimeoutSeconds
}

func (s Spec) GetTTD() int64 {
	return s.TTD
}

func (s Spec) IsMiningDisabled() bool {
	return s.DisableMining
}

var LatestFork = ForkConfig{
	ShanghaiTimestamp: big.NewInt(0),
}
