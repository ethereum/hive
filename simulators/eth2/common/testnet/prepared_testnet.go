package testnet

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"strings"
	"time"

	"github.com/holiman/uint256"
	blsu "github.com/protolambda/bls12-381-util"
	"github.com/protolambda/ztyp/tree"
	"github.com/protolambda/ztyp/view"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/params"
	"github.com/protolambda/zrnt/eth2/beacon/common"
	"github.com/protolambda/zrnt/eth2/configs"

	"github.com/ethereum/hive/hivesim"
	"github.com/ethereum/hive/simulators/eth2/common/clients"
	cl "github.com/ethereum/hive/simulators/eth2/common/config/consensus"
	el "github.com/ethereum/hive/simulators/eth2/common/config/execution"
)

var (
	depositAddress                              common.Eth1Address
	DEFAULT_SAFE_SLOTS_TO_IMPORT_OPTIMISTICALLY = big.NewInt(128)
)

func init() {
	_ = depositAddress.UnmarshalText(
		[]byte("0x4242424242424242424242424242424242424242"),
	)
}

// PreparedTestnet has all the options for starting nodes, ready to build the network.
type PreparedTestnet struct {
	// Consensus chain configuration
	spec *common.Spec

	// Execution chain configuration and genesis info
	eth1Genesis *el.ExecutionGenesis
	// Consensus genesis state
	eth2Genesis common.BeaconState
	// Secret keys of validators, to fabricate extra signed test messages with during testnet/
	// E.g. to test a slashable offence that would not otherwise happen.
	keys *[]blsu.SecretKey

	// Configuration to apply to every node of the given type
	executionOpts hivesim.StartOption
	validatorOpts hivesim.StartOption
	beaconOpts    hivesim.StartOption

	// A tranche is a group of validator keys to run on 1 node
	keyTranches []map[common.ValidatorIndex]*cl.KeyDetails
}

// Prepares the fork timestamps of post-merge forks based on the
// consensus layer genesis time and the fork epochs
func prepareExecutionForkConfig(
	eth2GenesisTime common.Timestamp,
	config *Config,
) *params.ChainConfig {
	chainConfig := params.ChainConfig{}
	if config.CapellaForkEpoch != nil {
		if config.CapellaForkEpoch.Uint64() == 0 {
			chainConfig.ShanghaiTime = big.NewInt(int64(eth2GenesisTime))
		} else {
			chainConfig.ShanghaiTime = big.NewInt(int64(eth2GenesisTime) + config.SlotTime.Int64()*32)
		}
	}
	return &chainConfig
}

// Build all artifacts require to start a testnet.
func prepareTestnet(
	t *hivesim.T,
	env *Environment,
	config *Config,
) *PreparedTestnet {
	eth1GenesisTime := common.Timestamp(time.Now().Unix())
	eth2GenesisTime := eth1GenesisTime + 30

	// Generate genesis for execution clients
	eth1Genesis := el.BuildExecutionGenesis(
		config.TerminalTotalDifficulty,
		uint64(eth1GenesisTime),
		config.Eth1Consensus,
		prepareExecutionForkConfig(eth2GenesisTime, config),
		config.GenesisExecutionAccounts,
	)
	if config.InitialBaseFeePerGas != nil {
		eth1Genesis.Genesis.BaseFee = config.InitialBaseFeePerGas
	}
	eth1ConfigOpt := eth1Genesis.ToParams(depositAddress)
	eth1Bundle, err := el.ExecutionBundle(eth1Genesis.Genesis)
	if err != nil {
		t.Fatal(err)
	}
	execNodeOpts := hivesim.Params{
		"HIVE_LOGLEVEL": os.Getenv("HIVE_LOGLEVEL"),
		"HIVE_NODETYPE": "full",
	}
	jwtSecret := hivesim.Params{"HIVE_JWTSECRET": "true"}
	executionOpts := hivesim.Bundle(
		eth1ConfigOpt,
		eth1Bundle,
		execNodeOpts,
		jwtSecret,
	)

	// Pre-generate PoW chains for clients that require it
	for i := 0; i < len(config.NodeDefinitions); i++ {
		if config.NodeDefinitions[i].ChainGenerator != nil {
			config.NodeDefinitions[i].Chain, err = config.NodeDefinitions[i].ChainGenerator.Generate(
				eth1Genesis,
			)
			if err != nil {
				t.Fatal(err)
			}
			fmt.Printf("Generated chain for node %d:\n", i+1)
			for j, b := range config.NodeDefinitions[i].Chain {
				js, _ := json.MarshalIndent(b.Header(), "", "  ")
				fmt.Printf("Block %d: %s\n", j, js)
			}
		}
	}

	// Generate beacon spec
	//
	// TODO: specify build-target based on preset, to run clients in mainnet or minimal mode.
	// copy the default mainnet config, and make some minimal modifications for testnet usage
	specCpy := *configs.Mainnet
	spec := &specCpy
	spec.Config.DEPOSIT_CONTRACT_ADDRESS = depositAddress
	spec.Config.DEPOSIT_CHAIN_ID = view.Uint64View(
		eth1Genesis.Genesis.Config.ChainID.Uint64(),
	)
	spec.Config.DEPOSIT_NETWORK_ID = view.Uint64View(eth1Genesis.NetworkID)
	spec.Config.ETH1_FOLLOW_DISTANCE = 1

	// Alter versions to avoid conflicts with mainnet values
	spec.Config.GENESIS_FORK_VERSION = common.Version{0x00, 0x00, 0x00, 0x0a}
	if config.AltairForkEpoch != nil {
		spec.Config.ALTAIR_FORK_EPOCH = common.Epoch(
			config.AltairForkEpoch.Uint64(),
		)
	}
	spec.Config.ALTAIR_FORK_VERSION = common.Version{0x01, 0x00, 0x00, 0x0a}
	if config.BellatrixForkEpoch != nil {
		spec.Config.BELLATRIX_FORK_EPOCH = common.Epoch(
			config.BellatrixForkEpoch.Uint64(),
		)
	}
	spec.Config.BELLATRIX_FORK_VERSION = common.Version{0x02, 0x00, 0x00, 0x0a}
	if config.CapellaForkEpoch != nil {
		spec.Config.CAPELLA_FORK_EPOCH = common.Epoch(
			config.CapellaForkEpoch.Uint64(),
		)
	}
	spec.Config.CAPELLA_FORK_VERSION = common.Version{0x03, 0x00, 0x00, 0x0a}
	spec.Config.DENEB_FORK_VERSION = common.Version{0x04, 0x00, 0x00, 0x0a}
	if config.ValidatorCount == nil {
		t.Fatal(fmt.Errorf("ValidatorCount was not configured"))
	}
	spec.Config.MIN_GENESIS_ACTIVE_VALIDATOR_COUNT = view.Uint64View(
		config.ValidatorCount.Uint64(),
	)
	if config.SlotTime != nil {
		spec.Config.SECONDS_PER_SLOT = common.Timestamp(
			config.SlotTime.Uint64(),
		)
	}
	tdd, _ := uint256.FromBig(config.TerminalTotalDifficulty)
	spec.Config.TERMINAL_TOTAL_DIFFICULTY = view.Uint256View(*tdd)
	if el.IsEth1GenesisPostMerge(eth1Genesis.Genesis) {
		genesisBlock := eth1Genesis.Genesis.ToBlock()
		spec.Config.TERMINAL_BLOCK_HASH = tree.Root(
			genesisBlock.Hash(),
		)
		spec.Config.TERMINAL_BLOCK_HASH_ACTIVATION_EPOCH = common.Timestamp(0)
	}

	// Validators can exit immediately
	spec.Config.SHARD_COMMITTEE_PERIOD = 0
	spec.Config.CHURN_LIMIT_QUOTIENT = 2

	// Validators can withdraw immediately
	spec.Config.MIN_VALIDATOR_WITHDRAWABILITY_DELAY = 0

	spec.Config.PROPOSER_SCORE_BOOST = 40

	// Generate keys opts for validators
	shares := config.NodeDefinitions.Shares()
	// ExtraShares defines an extra set of keys that none of the nodes will have.
	// E.g. to produce an environment where none of the nodes has 50%+ of the keys.
	if config.ExtraShares != nil {
		shares = append(shares, config.ExtraShares.Uint64())
	}
	keyTranches := cl.KeyTranches(env.Keys, shares)

	consensusConfigOpts, err := cl.ConsensusConfigsBundle(
		spec,
		eth1Genesis.Genesis,
		config.ValidatorCount.Uint64(),
	)
	if err != nil {
		t.Fatal(err)
	}

	var optimisticSync hivesim.Params
	if config.SafeSlotsToImportOptimistically == nil {
		config.SafeSlotsToImportOptimistically = DEFAULT_SAFE_SLOTS_TO_IMPORT_OPTIMISTICALLY
	}
	optimisticSync = optimisticSync.Set(
		"HIVE_ETH2_SAFE_SLOTS_TO_IMPORT_OPTIMISTICALLY",
		fmt.Sprintf("%d", config.SafeSlotsToImportOptimistically),
	)

	// prepare genesis beacon state, with all the validators in it.
	state, err := cl.BuildBeaconState(
		spec,
		eth1Genesis.Genesis,
		eth2GenesisTime,
		env.Keys,
	)
	if err != nil {
		t.Fatal(err)
	}

	// Write info so that the genesis state can be generated by the client
	stateOpt, err := cl.StateBundle(state)
	if err != nil {
		t.Fatal(err)
	}

	// Define additional start options for beacon chain
	commonOpts := hivesim.Params{
		"HIVE_ETH2_BN_API_PORT": fmt.Sprintf(
			"%d",
			clients.PortBeaconAPI,
		),
		"HIVE_ETH2_BN_GRPC_PORT": fmt.Sprintf(
			"%d",
			clients.PortBeaconGRPC,
		),
		"HIVE_ETH2_METRICS_PORT": fmt.Sprintf(
			"%d",
			clients.PortMetrics,
		),
		"HIVE_ETH2_CONFIG_DEPOSIT_CONTRACT_ADDRESS": depositAddress.String(),
		"HIVE_ETH2_DEPOSIT_DEPLOY_BLOCK_HASH": fmt.Sprintf(
			"%s",
			eth1Genesis.Genesis.ToBlock().Hash(),
		),
	}
	beaconOpts := hivesim.Bundle(
		commonOpts,
		hivesim.Params{
			"HIVE_CHECK_LIVE_PORT": fmt.Sprintf(
				"%d",
				clients.PortBeaconAPI,
			),
			"HIVE_ETH2_MERGE_ENABLED": "1",
			"HIVE_ETH2_ETH1_GENESIS_TIME": fmt.Sprintf(
				"%d",
				eth1Genesis.Genesis.Timestamp,
			),
			"HIVE_ETH2_GENESIS_FORK": config.activeFork(),
		},
		stateOpt,
		consensusConfigOpts,
		optimisticSync,
	)

	validatorOpts := hivesim.Bundle(
		commonOpts,
		hivesim.Params{
			"HIVE_CHECK_LIVE_PORT": "0",
		},
		consensusConfigOpts,
	)

	return &PreparedTestnet{
		spec:          spec,
		eth1Genesis:   eth1Genesis,
		eth2Genesis:   state,
		keys:          env.Secrets,
		executionOpts: executionOpts,
		beaconOpts:    beaconOpts,
		validatorOpts: validatorOpts,
		keyTranches:   keyTranches,
	}
}

func (p *PreparedTestnet) createTestnet(t *hivesim.T) *Testnet {
	genesisTime, _ := p.eth2Genesis.GenesisTime()
	genesisValidatorsRoot, _ := p.eth2Genesis.GenesisValidatorsRoot()
	return &Testnet{
		T:                     t,
		genesisTime:           genesisTime,
		genesisValidatorsRoot: genesisValidatorsRoot,
		spec:                  p.spec,
		eth1Genesis:           p.eth1Genesis,
		eth2GenesisState:      p.eth2Genesis,
	}
}

// Prepares an execution client object with all the necessary information
// to start
func (p *PreparedTestnet) prepareExecutionNode(
	parentCtx context.Context,
	testnet *Testnet,
	eth1Def *hivesim.ClientDefinition,
	consensus el.ExecutionConsensus,
	ttd *big.Int,
	executionIndex int,
	chain []*types.Block,
	subnet string,
	logEngineCalls bool,
) *clients.ExecutionClient {
	testnet.Logf(
		"Preparing execution node: %s (%s)",
		eth1Def.Name,
		eth1Def.Version,
	)

	// This method will return the options used to run the client.
	// Requires a method that returns the rest of the currently running
	// execution clients on the network at startup.
	optionsGenerator := func() ([]hivesim.StartOption, error) {
		opts := []hivesim.StartOption{p.executionOpts}
		opts = append(opts, consensus.HiveParams(executionIndex))

		currentlyRunningEcs := testnet.ExecutionClients().
			Running().
			Subnet(subnet)
		if len(currentlyRunningEcs) > 0 {
			bootnode, err := currentlyRunningEcs.Enodes()
			if err != nil {
				return nil, err
			}

			// Make the client connect to the first eth1 node, as a bootnode for the eth1 net
			opts = append(opts, hivesim.Params{"HIVE_BOOTNODE": bootnode})
		}
		if ttd != nil {
			opts = append(
				opts,
				hivesim.Params{
					"HIVE_TERMINAL_TOTAL_DIFFICULTY": fmt.Sprintf(
						"%d",
						ttd.Int64(),
					),
				},
			)
		} else {
			genesis := testnet.ExecutionGenesis()
			genesisBlock := genesis.ToBlock()
			if genesis.Config.TerminalTotalDifficulty.Cmp(genesisBlock.Difficulty()) <= 0 {
				opts = append(
					opts,
					hivesim.Params{
						"HIVE_TERMINAL_BLOCK_HASH": fmt.Sprintf(
							"%s",
							genesisBlock.Hash(),
						),
					},
				)
				opts = append(
					opts,
					hivesim.Params{
						"HIVE_TERMINAL_BLOCK_NUMBER": fmt.Sprintf(
							"%d",
							genesisBlock.NumberU64(),
						),
					},
				)
			}
		}

		if len(chain) > 0 {
			// Bundle the chain into the container
			chainParam, err := el.ChainBundle(chain)
			if err != nil {
				return nil, err
			}
			opts = append(opts, chainParam)
		}
		return opts, nil
	}
	return clients.NewExecutionClient(
		testnet.T,
		eth1Def,
		optionsGenerator,
		executionIndex,
		clients.PortEngineRPC+executionIndex,
		subnet,
		ttd,
		logEngineCalls,
	)
}

// Prepares a beacon client object with all the necessary information
// to start
func (p *PreparedTestnet) prepareBeaconNode(
	parentCtx context.Context,
	testnet *Testnet,
	beaconDef *hivesim.ClientDefinition,
	ttd *big.Int,
	beaconIndex int,
	eth1Endpoints ...*clients.ExecutionClient,
) *clients.BeaconClient {
	testnet.Logf(
		"Preparing beacon node: %s (%s)",
		beaconDef.Name,
		beaconDef.Version,
	)

	// This method will return the options used to run the client.
	// Requires a method that returns the rest of the currently running
	// beacon clients on the network at startup.
	optionsGenerator := func() ([]hivesim.StartOption, error) {
		opts := []hivesim.StartOption{p.beaconOpts}

		// Hook up beacon node to (maybe multiple) eth1 nodes
		var addrs []string
		var engineAddrs []string
		for _, en := range eth1Endpoints {
			if !en.IsRunning() || en.Proxy() == nil {
				return nil, fmt.Errorf(
					"attempted to start beacon node when the execution client is not yet running",
				)
			}
			execNode := en.Proxy()
			userRPC, err := execNode.UserRPCAddress()
			if err != nil {
				return nil, fmt.Errorf(
					"eth1 node used for beacon without available RPC: %v",
					err,
				)
			}
			addrs = append(addrs, userRPC)
			engineRPC, err := execNode.EngineRPCAddress()
			if err != nil {
				return nil, fmt.Errorf(
					"eth1 node used for beacon without available RPC: %v",
					err,
				)
			}
			engineAddrs = append(engineAddrs, engineRPC)
		}
		opts = append(opts, hivesim.Params{
			"HIVE_ETH2_ETH1_RPC_ADDRS":        strings.Join(addrs, ","),
			"HIVE_ETH2_ETH1_ENGINE_RPC_ADDRS": strings.Join(engineAddrs, ","),
			"HIVE_ETH2_BEACON_NODE_INDEX":     fmt.Sprintf("%d", beaconIndex),
		})

		if bootnodeENRs, err := testnet.BeaconClients().Running().ENRs(parentCtx); err != nil {
			return nil, fmt.Errorf(
				"failed to get ENR as bootnode for every beacon node: %v",
				err,
			)
		} else if bootnodeENRs != "" {
			opts = append(opts, hivesim.Params{"HIVE_ETH2_BOOTNODE_ENRS": bootnodeENRs})
		}

		if staticPeers, err := testnet.BeaconClients().Running().P2PAddrs(parentCtx); err != nil {
			return nil, fmt.Errorf(
				"failed to get p2paddr for every beacon node: %v",
				err,
			)
		} else if staticPeers != "" {
			opts = append(opts, hivesim.Params{"HIVE_ETH2_STATIC_PEERS": staticPeers})
		}

		if ttd != nil {
			opts = append(
				opts,
				hivesim.Params{
					"HIVE_TERMINAL_TOTAL_DIFFICULTY": fmt.Sprintf("%d", ttd),
				},
			)
		}
		return opts, nil
	}

	// TODO
	//if p.configName != "mainnet" && hasBuildTarget(beaconDef, p.configName) {
	//	opts = append(opts, hivesim.WithBuildTarget(p.configName))
	//}
	return clients.NewBeaconClient(
		testnet.T,
		beaconDef,
		optionsGenerator,
		testnet.genesisTime,
		testnet.spec,
		beaconIndex,
		testnet.genesisValidatorsRoot,
	)
}

// Prepares a validator client object with all the necessary information
// to eventually start the client.
func (p *PreparedTestnet) prepareValidatorClient(
	parentCtx context.Context,
	testnet *Testnet,
	validatorDef *hivesim.ClientDefinition,
	bn *clients.BeaconClient,
	keyIndex int,
) *clients.ValidatorClient {
	testnet.Logf(
		"Preparing validator client: %s (%s)",
		validatorDef.Name,
		validatorDef.Version,
	)
	if keyIndex >= len(p.keyTranches) {
		testnet.Fatalf(
			"only have %d key tranches, cannot find index %d for VC",
			len(p.keyTranches),
			keyIndex,
		)
	}
	// This method will return the options used to run the client.
	// Requires the beacon client object to which to connect.
	optionsGenerator := func(validatorKeys map[common.ValidatorIndex]*cl.KeyDetails) ([]hivesim.StartOption, error) {
		if !bn.IsRunning() {
			return nil, fmt.Errorf(
				"attempted to start a validator when the beacon node is not running",
			)
		}
		// Hook up validator to beacon node
		bnAPIOpt := hivesim.Params{
			"HIVE_ETH2_BN_API_IP": bn.HiveClient.IP.String(),
		}
		keysOpt := cl.KeysBundle(validatorKeys)
		opts := []hivesim.StartOption{p.validatorOpts, keysOpt, bnAPIOpt}
		// TODO
		//if p.configName != "mainnet" && hasBuildTarget(validatorDef, p.configName) {
		//	opts = append(opts, hivesim.WithBuildTarget(p.configName))
		//}
		return opts, nil
	}
	return clients.NewValidatorClient(
		testnet.T,
		validatorDef,
		optionsGenerator,
		p.keyTranches[keyIndex],
	)
}
