package testnet

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/protolambda/zrnt/eth2/beacon"
	"github.com/protolambda/zrnt/eth2/beacon/common"
	"github.com/protolambda/zrnt/eth2/configs"

	"github.com/ethereum/hive/hivesim"
	"github.com/ethereum/hive/simulators/eth2/common/clients"
	cl "github.com/ethereum/hive/simulators/eth2/common/config/consensus"
	cl_genesis "github.com/ethereum/hive/simulators/eth2/common/config/consensus/genesis"
	el "github.com/ethereum/hive/simulators/eth2/common/config/execution"
	"github.com/ethereum/hive/simulators/eth2/common/utils"
	beacon_client "github.com/marioevz/eth-clients/clients/beacon"
	exec_client "github.com/marioevz/eth-clients/clients/execution"
	validator_client "github.com/marioevz/eth-clients/clients/validator"
	mock_builder "github.com/marioevz/mock-builder/mock"
	builder_types "github.com/marioevz/mock-builder/types"
)

var (
	depositAddress                              common.Eth1Address
	DEFAULT_SAFE_SLOTS_TO_IMPORT_OPTIMISTICALLY = big.NewInt(128)
	DEFAULT_MAX_CONSECUTIVE_ERRORS_ON_WAITS     = 3
)

func init() {
	_ = depositAddress.UnmarshalText(
		[]byte("0x4242424242424242424242424242424242424242"),
	)
}

// PreparedTestnet has all the options for starting nodes, ready to build the network.
type PreparedTestnet struct {
	// Consensus chain configuration
	Spec *common.Spec

	// Execution chain configuration and genesis info
	ExecutionGenesis *el.ExecutionGenesis
	// Consensus genesis state
	BeaconGenesis common.BeaconState

	ValidatorsSetupDetails cl.ValidatorsSetupDetails

	// Configuration to apply to every node of the given type
	executionOpts hivesim.StartOption
	validatorOpts hivesim.StartOption
	beaconOpts    hivesim.StartOption

	// A tranche is a group of validator keys to run on 1 node
	keyTranches []cl.ValidatorsKeys
}

func getLogLevelString() string {
	logLevelInt, _ := strconv.Atoi(os.Getenv("HIVE_LOGLEVEL"))
	switch logLevelInt {
	case 2:
		return "warn"
	case 3:
		return "info"
	case 4:
		return "debug"
	case 5:
		return "trace"
	}
	return "error"
}

// Build all artifacts require to start a testnet.
func PrepareTestnet(
	env *Environment,
	config *Config,
) (*PreparedTestnet, error) {
	genesisTime := common.Timestamp(time.Now().Unix()) + 30

	// Sanitize configuration according to the clients used
	if err := config.FillDefaults(); err != nil {
		return nil, fmt.Errorf("FAIL: error filling defaults: %v", err)
	}

	if configJson, err := json.MarshalIndent(config, "", "  "); err != nil {
		panic(err)
	} else {
		fmt.Printf("Testnet config: %s\n", configJson)
	}

	// Generate genesis for execution clients
	chainConfig, err := el.BuildChainConfig(
		config.TerminalTotalDifficulty,
		uint64(genesisTime),
		config.SlotsPerEpoch.Uint64(),
		config.SlotTime.Uint64(),
		config.ForkConfig,
	)
	if err != nil {
		return nil, fmt.Errorf("error producing chainConfig: %v", err)
	}

	executionGenesis, err := el.BuildExecutionGenesis(
		uint64(genesisTime),
		config.Eth1Consensus,
		chainConfig,
		config.GenesisExecutionAccounts,
		config.InitialBaseFeePerGas,
	)
	if err != nil {
		return nil, fmt.Errorf("error producing execution genesis: %v", err)
	}

	eth1ConfigOpt := executionGenesis.ToParams(depositAddress)
	eth1Bundle, err := el.ExecutionBundle(executionGenesis.Genesis)
	if err != nil {
		return nil, fmt.Errorf("unable to bundle execution genesis: %v", err)
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
				executionGenesis,
			)
			if err != nil {
				return nil, fmt.Errorf("unable to generate PoW chains for node %d: %v", i, err)
			}
			fmt.Printf("Generated chain for node %d:\n", i+1)
			for j, b := range config.NodeDefinitions[i].Chain {
				js, _ := json.MarshalIndent(b.Header(), "", "  ")
				fmt.Printf("Block %d: %s\n", j, js)
			}
		}
	}

	spec, err := cl.BuildSpec(
		configs.Mainnet,
		config.ForkConfig,
		config.ConsensusConfig,
		depositAddress,
		executionGenesis,
	)
	if err != nil {
		return nil, fmt.Errorf("error producing spec: %v", err)
	}

	// Generate keys opts for validators
	shares := config.NodeDefinitions.Shares()
	// ExtraShares defines an extra set of keys that none of the nodes will have.
	// E.g. to produce an environment where none of the nodes has 50%+ of the keys.
	if config.ExtraShares != nil {
		shares = append(shares, config.ExtraShares.Uint64())
	}
	keyTranches := env.Validators.KeyTranches(shares)

	consensusConfigOpts, err := cl.ConsensusConfigsBundle(
		spec,
		executionGenesis.Hash,
		config.ValidatorCount.Uint64(),
	)
	if err != nil {
		return nil, fmt.Errorf("error producing consensus config bundle: %v", err)
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
	state, err := cl_genesis.BuildBeaconState(
		spec,
		executionGenesis.Block,
		genesisTime,
		env.Validators,
	)
	if err != nil {
		return nil, fmt.Errorf("error producing beacon genesis state: %v", err)
	}
	genValRoot, err := state.GenesisValidatorsRoot()
	if err != nil {
		return nil, fmt.Errorf("error producing genesis validators root: %v", err)
	}
	fmt.Printf("Genesis validators root: %s\n", genValRoot)

	forkDecoder := beacon.NewForkDecoder(spec, genValRoot)
	for _, currentConfig := range []struct {
		ForkName      string
		VersionConfig *common.ForkDigest
	}{
		{
			ForkName:      "genesis",
			VersionConfig: &forkDecoder.Genesis,
		},
		{
			ForkName:      "altair",
			VersionConfig: &forkDecoder.Altair,
		},
		{
			ForkName:      "bellatrix",
			VersionConfig: &forkDecoder.Bellatrix,
		},
		{
			ForkName:      "capella",
			VersionConfig: &forkDecoder.Capella,
		},
		{
			ForkName:      "deneb",
			VersionConfig: &forkDecoder.Deneb,
		},
	} {
		fmt.Printf(
			"Fork %s digest: %s\n",
			currentConfig.ForkName,
			currentConfig.VersionConfig.String(),
		)
	}

	// Write info so that the genesis state can be generated by the client
	stateOpt, err := cl.StateBundle(state)
	if err != nil {
		return nil, fmt.Errorf("error producing state bundle: %v", err)
	}

	// Define additional start options for beacon chain
	commonOpts := hivesim.Params{
		"HIVE_ETH2_BN_GRPC_PORT": fmt.Sprintf(
			"%d",
			beacon_client.PortBeaconGRPC,
		),
		"HIVE_ETH2_METRICS_PORT": fmt.Sprintf(
			"%d",
			beacon_client.PortMetrics,
		),
		"HIVE_ETH2_CONFIG_DEPOSIT_CONTRACT_ADDRESS": depositAddress.String(),
		"HIVE_ETH2_DEPOSIT_DEPLOY_BLOCK_HASH": fmt.Sprintf(
			"%s",
			executionGenesis.Hash,
		),
	}
	beaconParams := hivesim.Params{
		"HIVE_CHECK_LIVE_PORT": fmt.Sprintf(
			"%d",
			beacon_client.PortBeaconAPI,
		),
		"HIVE_ETH2_MERGE_ENABLED": "1",
		"HIVE_ETH2_ETH1_GENESIS_TIME": fmt.Sprintf(
			"%d",
			executionGenesis.Genesis.Timestamp,
		),
		"HIVE_ETH2_GENESIS_FORK": config.GenesisBeaconFork(),
	}
	if config.DisablePeerScoring {
		beaconParams["HIVE_ETH2_DISABLE_PEER_SCORING"] = "1"
	}

	beaconOpts := hivesim.Bundle(
		commonOpts,
		beaconParams,
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
		Spec:                   spec,
		ExecutionGenesis:       executionGenesis,
		BeaconGenesis:          state,
		ValidatorsSetupDetails: env.Validators,
		executionOpts:          executionOpts,
		beaconOpts:             beaconOpts,
		validatorOpts:          validatorOpts,
		keyTranches:            keyTranches,
	}, nil
}

func (p *PreparedTestnet) createTestnet(t *hivesim.T) *Testnet {
	genesisTime, _ := p.BeaconGenesis.GenesisTime()
	genesisValidatorsRoot, _ := p.BeaconGenesis.GenesisValidatorsRoot()
	validators, err := utils.NewValidators(p.Spec, p.BeaconGenesis, p.ValidatorsSetupDetails.KeysMap(0))
	if err != nil {
		panic(err)
	}
	return &Testnet{
		T:                     t,
		genesisTime:           genesisTime,
		genesisValidatorsRoot: genesisValidatorsRoot,
		spec:                  p.Spec,
		executionGenesis:      p.ExecutionGenesis,
		eth2GenesisState:      p.BeaconGenesis,

		Validators:      validators,
		ValidatorGroups: make(map[string]*utils.Validators),

		// Testing
		maxConsecutiveErrorsOnWaits: DEFAULT_MAX_CONSECUTIVE_ERRORS_ON_WAITS,
	}
}

// Prepares an execution client object with all the necessary information
// to start
func (p *PreparedTestnet) prepareExecutionNode(
	parentCtx context.Context,
	testnet *Testnet,
	eth1Def *hivesim.ClientDefinition,
	consensus el.ExecutionConsensus,
	chain []*types.Block,
	config exec_client.ExecutionClientConfig,
) *exec_client.ExecutionClient {
	testnet.Logf(
		"Preparing execution node: %s (%s)",
		eth1Def.Name,
		eth1Def.Version,
	)

	cm := &clients.HiveManagedClient{
		T:                    testnet.T,
		HiveClientDefinition: eth1Def,
		Port:                 exec_client.PortEngineRPC,
	}

	// This method will return the options used to run the client.
	// Requires a method that returns the rest of the currently running
	// execution clients on the network at startup.
	cm.OptionsGenerator = func() ([]hivesim.StartOption, error) {
		opts := []hivesim.StartOption{p.executionOpts}
		opts = append(opts, consensus.HiveParams(config.ClientIndex))

		currentlyRunningEcs := testnet.ExecutionClients().
			Running().
			Subnet(config.Subnet)
		if len(currentlyRunningEcs) > 0 {
			bootnode, err := currentlyRunningEcs.Enodes()
			if err != nil {
				return nil, err
			}

			// Make the client connect to the first eth1 node, as a bootnode for the eth1 net
			opts = append(opts, hivesim.Params{"HIVE_BOOTNODE": bootnode})
		}
		opts = append(
			opts,
			hivesim.Params{
				"HIVE_TERMINAL_TOTAL_DIFFICULTY": fmt.Sprintf(
					"%d",
					config.TerminalTotalDifficulty,
				),
			},
		)
		genesis := testnet.ExecutionGenesis().ToBlock()
		if config.TerminalTotalDifficulty <= genesis.Difficulty().Int64() {
			opts = append(
				opts,
				hivesim.Params{
					"HIVE_TERMINAL_BLOCK_HASH": fmt.Sprintf(
						"%s",
						genesis.Hash(),
					),
				},
			)
			opts = append(
				opts,
				hivesim.Params{
					"HIVE_TERMINAL_BLOCK_NUMBER": fmt.Sprintf(
						"%d",
						genesis.NumberU64(),
					),
				},
			)
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

	return &exec_client.ExecutionClient{
		Client: cm,
		Logger: testnet.T,
		Config: config,
	}
}

// Prepares a beacon client object with all the necessary information
// to start
func (p *PreparedTestnet) prepareBeaconNode(
	parentCtx context.Context,
	testnet *Testnet,
	beaconDef *hivesim.ClientDefinition,
	enableBuilders bool,
	builderOptions []mock_builder.Option,
	config beacon_client.BeaconClientConfig,
	eth1Endpoints ...*exec_client.ExecutionClient,
) *beacon_client.BeaconClient {
	testnet.Logf(
		"Preparing beacon node: %s (%s)",
		beaconDef.Name,
		beaconDef.Version,
	)

	if len(eth1Endpoints) == 0 {
		panic(fmt.Errorf("at least 1 execution endpoint is required"))
	}

	cm := &clients.HiveManagedClient{
		T:                    testnet.T,
		HiveClientDefinition: beaconDef,
		Port:                 int64(config.BeaconAPIPort),
	}

	cl := &beacon_client.BeaconClient{
		Client: cm,
		Logger: testnet.T,
		Config: config,
	}

	if enableBuilders {
		simIP, err := testnet.T.Sim.ContainerNetworkIP(
			testnet.T.SuiteID,
			"bridge",
			"simulation",
		)
		if err != nil {
			panic(err)
		}

		options := []mock_builder.Option{
			mock_builder.WithExternalIP(net.ParseIP(simIP)),
			mock_builder.WithPort(
				mock_builder.DEFAULT_BUILDER_PORT + config.ClientIndex,
			),
			mock_builder.WithID(config.ClientIndex),
			mock_builder.WithBeaconGenesisTime(testnet.genesisTime),
			mock_builder.WithSpec(p.Spec),
			mock_builder.WithLogLevel(getLogLevelString()),
		}

		if builderOptions != nil {
			options = append(options, builderOptions...)
		}

		cl.Builder, err = mock_builder.NewMockBuilder(
			parentCtx,
			eth1Endpoints[0],
			cl,
			options...,
		)
		if err != nil {
			panic(err)
		}
	}

	// This method will return the options used to run the client.
	// Requires a method that returns the rest of the currently running
	// beacon clients on the network at startup.
	cm.OptionsGenerator = func() ([]hivesim.StartOption, error) {
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
			"HIVE_ETH2_BEACON_NODE_INDEX": fmt.Sprintf(
				"%d",
				config.ClientIndex,
			),
			"HIVE_ETH2_BN_API_PORT": fmt.Sprintf(
				"%d",
				cl.Config.BeaconAPIPort,
			),
		})

		currentlyRunningBcs := testnet.BeaconClients().
			Running().
			Subnet(config.Subnet)
		if len(currentlyRunningBcs) > 0 {
			if bootnodeENRs, err := currentlyRunningBcs.ENRs(parentCtx); err != nil {
				return nil, fmt.Errorf(
					"failed to get ENR as bootnode for every beacon node: %v",
					err,
				)
			} else if bootnodeENRs != "" {
				opts = append(opts, hivesim.Params{"HIVE_ETH2_BOOTNODE_ENRS": bootnodeENRs})
			}

			if staticPeers, err := currentlyRunningBcs.P2PAddrs(parentCtx); err != nil {
				return nil, fmt.Errorf(
					"failed to get p2paddr for every beacon node: %v",
					err,
				)
			} else if staticPeers != "" {
				opts = append(opts, hivesim.Params{"HIVE_ETH2_STATIC_PEERS": staticPeers})
			}
		}

		opts = append(
			opts,
			hivesim.Params{
				"HIVE_TERMINAL_TOTAL_DIFFICULTY": fmt.Sprintf(
					"%d",
					config.TerminalTotalDifficulty,
				),
			},
		)

		if cl.Builder != nil {
			if builder, ok := cl.Builder.(builder_types.Builder); ok {
				opts = append(opts, hivesim.Params{
					"HIVE_ETH2_BUILDER_ENDPOINT": builder.Address(),
				})
			} else {
				panic(fmt.Errorf("builder is not a Builder"))
			}
		}

		// TODO
		//if p.configName != "mainnet" && hasBuildTarget(beaconDef, p.configName) {
		//	opts = append(opts, hivesim.WithBuildTarget(p.configName))
		//}

		return opts, nil
	}

	return cl
}

// Prepares a validator client object with all the necessary information
// to eventually start the client.
func (p *PreparedTestnet) prepareValidatorClient(
	parentCtx context.Context,
	testnet *Testnet,
	validatorDef *hivesim.ClientDefinition,
	bn *beacon_client.BeaconClient,
	keyIndex int,
) *validator_client.ValidatorClient {
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
	keys := p.keyTranches[keyIndex]

	cm := &clients.HiveManagedClient{
		T:                    testnet.T,
		HiveClientDefinition: validatorDef,
	}

	// This method will return the options used to run the client.
	// Requires the beacon client object to which to connect.
	cm.OptionsGenerator = func() ([]hivesim.StartOption, error) {
		if !bn.IsRunning() {
			return nil, fmt.Errorf(
				"attempted to start a validator when the beacon node is not running",
			)
		}
		// Hook up validator to beacon node
		bnAPIOpt := hivesim.Params{
			"HIVE_ETH2_BN_API_IP": bn.GetHost(),
		}
		bnAPIOpt = bnAPIOpt.Set(
			"HIVE_ETH2_BN_API_PORT",
			fmt.Sprintf("%d", bn.Config.BeaconAPIPort),
		)
		if testnet.blobber != nil {
			simIP, err := testnet.T.Sim.ContainerNetworkIP(
				testnet.T.SuiteID,
				"bridge",
				"simulation",
			)
			if err != nil {
				panic(err)
			}

			p := testnet.blobber.AddBeaconClient(bn, true)
			bnAPIOpt = bnAPIOpt.Set(
				"HIVE_ETH2_BN_API_IP",
				simIP,
			)
			bnAPIOpt = bnAPIOpt.Set(
				"HIVE_ETH2_BN_API_PORT",
				fmt.Sprintf("%d", p.Port()),
			)
		}
		opts := []hivesim.StartOption{p.validatorOpts, keys.Bundle(), bnAPIOpt}

		if bn.Builder != nil {
			if builder, ok := bn.Builder.(builder_types.Builder); ok {
				opts = append(opts, hivesim.Params{
					"HIVE_ETH2_BUILDER_ENDPOINT": builder.Address(),
				})
			}
		}

		// TODO
		//if p.configName != "mainnet" && hasBuildTarget(validatorDef, p.configName) {
		//	opts = append(opts, hivesim.WithBuildTarget(p.configName))
		//}
		return opts, nil
	}

	return &validator_client.ValidatorClient{
		Client:       cm,
		Logger:       testnet.T,
		ClientIndex:  keyIndex,
		Keys:         keys,
		BeaconClient: bn,
	}
}
