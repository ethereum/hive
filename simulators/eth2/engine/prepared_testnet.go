package main

import (
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"strings"
	"time"

	"github.com/holiman/uint256"
	blsu "github.com/protolambda/bls12-381-util"
	"github.com/protolambda/ztyp/view"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/hive/hivesim"
	"github.com/ethereum/hive/simulators/eth2/engine/setup"
	"github.com/protolambda/zrnt/eth2/beacon/common"
	"github.com/protolambda/zrnt/eth2/configs"
)

var (
	depositAddress                              common.Eth1Address
	DEFAULT_SAFE_SLOTS_TO_IMPORT_OPTIMISTICALLY = big.NewInt(128)
)

func init() {
	_ = depositAddress.UnmarshalText([]byte("0x4242424242424242424242424242424242424242"))
}

// PreparedTestnet has all the options for starting nodes, ready to build the network.
type PreparedTestnet struct {
	// Consensus chain configuration
	spec *common.Spec

	// Execution chain configuration and genesis info
	eth1Genesis *setup.Eth1Genesis
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
	keyTranches [][]*setup.KeyDetails
}

// Build all artifacts require to start a testnet.
func prepareTestnet(t *hivesim.T, env *testEnv, config *Config) *PreparedTestnet {
	eth1GenesisTime := common.Timestamp(time.Now().Unix())
	eth2GenesisTime := eth1GenesisTime + 30

	// Generate genesis for execution clients
	eth1Genesis := setup.BuildEth1Genesis(config.TerminalTotalDifficulty, uint64(eth1GenesisTime), config.Eth1Consensus)
	if config.InitialBaseFeePerGas != nil {
		eth1Genesis.Genesis.BaseFee = config.InitialBaseFeePerGas
	}
	eth1ConfigOpt := eth1Genesis.ToParams(depositAddress)
	eth1Bundle, err := setup.Eth1Bundle(eth1Genesis.Genesis)
	if err != nil {
		t.Fatal(err)
	}
	execNodeOpts := hivesim.Params{"HIVE_LOGLEVEL": os.Getenv("HIVE_LOGLEVEL")}
	jwtSecret := hivesim.Params{"HIVE_JWTSECRET": "true"}
	executionOpts := hivesim.Bundle(eth1ConfigOpt, eth1Bundle, execNodeOpts, jwtSecret)

	// Pre-generate PoW chains for clients that require it
	for i := 0; i < len(config.Nodes); i++ {
		if config.Nodes[i].ChainGenerator != nil {
			config.Nodes[i].Chain, err = config.Nodes[i].ChainGenerator.Generate(eth1Genesis)
			if err != nil {
				t.Fatal(err)
			}
			fmt.Printf("Generated chain for node %d:\n", i+1)
			for j, b := range config.Nodes[i].Chain {
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
	spec.Config.DEPOSIT_CHAIN_ID = eth1Genesis.Genesis.Config.ChainID.Uint64()
	spec.Config.DEPOSIT_NETWORK_ID = eth1Genesis.NetworkID
	spec.Config.ETH1_FOLLOW_DISTANCE = 1

	// Alter versions to avoid conflicts with mainnet values
	spec.Config.GENESIS_FORK_VERSION = common.Version{0x00, 0x00, 0x00, 0x0a}
	if config.AltairForkEpoch != nil {
		spec.Config.ALTAIR_FORK_EPOCH = common.Epoch(config.AltairForkEpoch.Uint64())
	}
	spec.Config.ALTAIR_FORK_VERSION = common.Version{0x01, 0x00, 0x00, 0x0a}
	if config.MergeForkEpoch != nil {
		spec.Config.BELLATRIX_FORK_EPOCH = common.Epoch(config.MergeForkEpoch.Uint64())
	}
	spec.Config.BELLATRIX_FORK_VERSION = common.Version{0x02, 0x00, 0x00, 0x0a}
	// TODO: Requires https://github.com/protolambda/zrnt/pull/38
	/*
		if config.CapellaForkEpoch != nil {
			spec.Config.CAPELLA_FORK_EPOCH = common.Epoch(config.CapellaForkEpoch.Uint64())
		}
		spec.Config.CAPELLA_FORK_VERSION = common.Version{0x03, 0x00, 0x00, 0x0a}
	*/
	spec.Config.SHARDING_FORK_VERSION = common.Version{0x04, 0x00, 0x00, 0x0a}
	if config.ValidatorCount == nil {
		t.Fatal(fmt.Errorf("ValidatorCount was not configured"))
	}
	spec.Config.MIN_GENESIS_ACTIVE_VALIDATOR_COUNT = config.ValidatorCount.Uint64()
	if config.SlotTime != nil {
		spec.Config.SECONDS_PER_SLOT = common.Timestamp(config.SlotTime.Uint64())
	}
	tdd, _ := uint256.FromBig(config.TerminalTotalDifficulty)
	spec.Config.TERMINAL_TOTAL_DIFFICULTY = view.Uint256View(*tdd)

	// Generate keys opts for validators
	shares := config.Nodes.Shares()
	// ExtraShares defines an extra set of keys that none of the nodes will have.
	// E.g. to produce an environment where none of the nodes has 50%+ of the keys.
	if config.ExtraShares != nil {
		shares = append(shares, config.ExtraShares.Uint64())
	}
	keyTranches := setup.KeyTranches(env.Keys, shares)

	consensusConfigOpts, err := setup.ConsensusConfigsBundle(spec, eth1Genesis.Genesis, config.ValidatorCount.Uint64())
	if err != nil {
		t.Fatal(err)
	}

	var optimisticSync hivesim.Params
	if config.SafeSlotsToImportOptimistically == nil {
		config.SafeSlotsToImportOptimistically = DEFAULT_SAFE_SLOTS_TO_IMPORT_OPTIMISTICALLY
	}
	optimisticSync = optimisticSync.Set("HIVE_ETH2_SAFE_SLOTS_TO_IMPORT_OPTIMISTICALLY", fmt.Sprintf("%d", config.SafeSlotsToImportOptimistically))

	// prepare genesis beacon state, with all the validators in it.
	state, err := setup.BuildBeaconState(spec, eth1Genesis.Genesis, eth2GenesisTime, env.Keys)
	if err != nil {
		t.Fatal(err)
	}

	// Write info so that the genesis state can be generated by the client
	stateOpt, err := setup.StateBundle(state)
	if err != nil {
		t.Fatal(err)
	}

	// Define additional start options for beacon chain
	commonOpts := hivesim.Params{
		"HIVE_ETH2_BN_API_PORT":                     fmt.Sprintf("%d", PortBeaconAPI),
		"HIVE_ETH2_BN_GRPC_PORT":                    fmt.Sprintf("%d", PortBeaconGRPC),
		"HIVE_ETH2_METRICS_PORT":                    fmt.Sprintf("%d", PortMetrics),
		"HIVE_ETH2_CONFIG_DEPOSIT_CONTRACT_ADDRESS": depositAddress.String(),
	}
	beaconOpts := hivesim.Bundle(
		commonOpts,
		hivesim.Params{
			"HIVE_CHECK_LIVE_PORT":        fmt.Sprintf("%d", PortBeaconAPI),
			"HIVE_ETH2_MERGE_ENABLED":     "1",
			"HIVE_ETH2_ETH1_GENESIS_TIME": fmt.Sprintf("%d", eth1Genesis.Genesis.Timestamp),
			"HIVE_ETH2_GENESIS_FORK":      config.activeFork(),
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
	}
}

// Prepares an execution client object with all the necessary information
// to start
func (p *PreparedTestnet) prepareExecutionNode(testnet *Testnet, eth1Def *hivesim.ClientDefinition, consensus setup.Eth1Consensus, ttd *big.Int, executionIndex int, chain []*types.Block) *ExecutionClient {
	testnet.Logf("Preparing execution node: %s (%s)", eth1Def.Name, eth1Def.Version)

	// This method will return the options used to run the client.
	// Requires a method that returns the rest of the currently running
	// execution clients on the network at startup.
	optionsGenerator := func() ([]hivesim.StartOption, error) {
		opts := []hivesim.StartOption{p.executionOpts}
		opts = append(opts, consensus.HiveParams(executionIndex))

		currentlyRunningEcs := testnet.ExecutionClients().Running()
		if len(currentlyRunningEcs) > 0 {
			bootnode, err := currentlyRunningEcs.Enodes()
			if err != nil {
				return nil, err
			}

			// Make the client connect to the first eth1 node, as a bootnode for the eth1 net
			opts = append(opts, hivesim.Params{"HIVE_BOOTNODE": bootnode})
		}
		if ttd != nil {
			opts = append(opts, hivesim.Params{"HIVE_TERMINAL_TOTAL_DIFFICULTY": fmt.Sprintf("%d", ttd.Int64())})
		}
		if chain != nil && len(chain) > 0 {
			// Bundle the chain into the container
			chainParam, err := setup.ChainBundle(chain)
			if err != nil {
				return nil, err
			}
			opts = append(opts, chainParam)
		}
		return opts, nil
	}
	return NewExecutionClient(testnet.T, eth1Def, optionsGenerator, PortEngineRPC+executionIndex)
}

// Prepares a beacon client object with all the necessary information
// to start
func (p *PreparedTestnet) prepareBeaconNode(testnet *Testnet, beaconDef *hivesim.ClientDefinition, ttd *big.Int, beaconIndex int, eth1Endpoints ...*ExecutionClient) *BeaconClient {
	testnet.Logf("Preparing beacon node: %s (%s)", beaconDef.Name, beaconDef.Version)

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
				return nil, fmt.Errorf("Attempted to start beacon node when the execution client is not yet running")
			}
			execNode := en.Proxy()
			userRPC, err := execNode.UserRPCAddress()
			if err != nil {
				return nil, fmt.Errorf("eth1 node used for beacon without available RPC: %v", err)
			}
			addrs = append(addrs, userRPC)
			engineRPC, err := execNode.EngineRPCAddress()
			if err != nil {
				return nil, fmt.Errorf("eth1 node used for beacon without available RPC: %v", err)
			}
			engineAddrs = append(engineAddrs, engineRPC)
		}
		opts = append(opts, hivesim.Params{
			"HIVE_ETH2_ETH1_RPC_ADDRS":        strings.Join(addrs, ","),
			"HIVE_ETH2_ETH1_ENGINE_RPC_ADDRS": strings.Join(engineAddrs, ","),
			"HIVE_ETH2_BEACON_NODE_INDEX":     fmt.Sprintf("%d", beaconIndex),
		})

		if bootnodeENRs, err := testnet.BeaconClients().Running().ENRs(); err != nil {
			return nil, fmt.Errorf("failed to get ENR as bootnode for every beacon node: %v", err)
		} else if bootnodeENRs != "" {
			opts = append(opts, hivesim.Params{"HIVE_ETH2_BOOTNODE_ENRS": bootnodeENRs})
		}

		if staticPeers, err := testnet.BeaconClients().Running().P2PAddrs(); err != nil {
			return nil, fmt.Errorf("failed to get p2paddr for every beacon node: %v", err)
		} else if staticPeers != "" {
			opts = append(opts, hivesim.Params{"HIVE_ETH2_STATIC_PEERS": staticPeers})
		}

		if ttd != nil {
			opts = append(opts, hivesim.Params{"HIVE_TERMINAL_TOTAL_DIFFICULTY": fmt.Sprintf("%d", ttd)})
		}
		return opts, nil
	}

	// TODO
	//if p.configName != "mainnet" && hasBuildTarget(beaconDef, p.configName) {
	//	opts = append(opts, hivesim.WithBuildTarget(p.configName))
	//}
	return NewBeaconClient(testnet.T, beaconDef, optionsGenerator, testnet.genesisTime, testnet.spec, beaconIndex)
}

// Prepares a validator client object with all the necessary information
// to eventually start the client.
func (p *PreparedTestnet) prepareValidatorClient(testnet *Testnet, validatorDef *hivesim.ClientDefinition, bn *BeaconClient, keyIndex int) *ValidatorClient {
	testnet.Logf("Preparing validator client: %s (%s)", validatorDef.Name, validatorDef.Version)
	if keyIndex >= len(p.keyTranches) {
		testnet.Fatalf("only have %d key tranches, cannot find index %d for VC", len(p.keyTranches), keyIndex)
	}
	// This method will return the options used to run the client.
	// Requires the beacon client object to which to connect.
	optionsGenerator := func(validatorKeys []*setup.KeyDetails) ([]hivesim.StartOption, error) {
		if !bn.IsRunning() {
			return nil, fmt.Errorf("Attempted to start a validator when the beacon node is not running")
		}
		// Hook up validator to beacon node
		bnAPIOpt := hivesim.Params{
			"HIVE_ETH2_BN_API_IP": bn.HiveClient.IP.String(),
		}
		keysOpt := setup.KeysBundle(validatorKeys)
		opts := []hivesim.StartOption{p.validatorOpts, keysOpt, bnAPIOpt}
		// TODO
		//if p.configName != "mainnet" && hasBuildTarget(validatorDef, p.configName) {
		//	opts = append(opts, hivesim.WithBuildTarget(p.configName))
		//}
		return opts, nil
	}
	return NewValidatorClient(testnet.T, validatorDef, optionsGenerator, p.keyTranches[keyIndex])
}
