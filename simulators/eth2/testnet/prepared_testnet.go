package main

import (
	"fmt"
	"math/big"
	"os"
	"strings"

	"github.com/ethereum/hive/hivesim"
	"github.com/ethereum/hive/simulators/eth2/testnet/setup"
	"github.com/protolambda/zrnt/eth2/beacon/common"
	"github.com/protolambda/zrnt/eth2/configs"
)

var depositAddress common.Eth1Address
var mnemonic = "couple kiwi radio river setup fortune hunt grief buddy forward perfect empty slim wear bounce drift execute nation tobacco dutch chapter festival ice fog"

func init() {
	depositAddress.UnmarshalText([]byte("0x4242424242424242424242424242424242424242"))
}

// PreparedTestnet has all the options for starting nodes, ready to build the network.
type PreparedTestnet struct {
	// Consensus chain configuration
	spec *common.Spec

	// Execution chain configuration and genesis info
	eth1Genesis *setup.Eth1Genesis

	// configuration to apply to every node of the given type
	executionOpts []hivesim.StartOption
	validatorOpts []hivesim.StartOption
	beaconOpts    []hivesim.StartOption

	// a tranche is a group of validator keys to run on 1 node
	keyTranches []hivesim.StartOption
}

func buildExecutionClientOpts(depositAddress common.Eth1Address, ttd *big.Int) (*setup.Eth1Genesis, []hivesim.StartOption, error) {
	genesis := setup.BuildEth1Genesis(ttd)
	config := genesis.ToParams(depositAddress)
	bundle, err := setup.Eth1Bundle(genesis.Genesis)
	return genesis, []hivesim.StartOption{config, bundle}, err
}

func buildBeaconSpec(depositAddress common.Eth1Address, ttd uint64, chainId uint64, networkId uint64, genesisTime uint64) *common.Spec {
	// TODO: specify build-target based on preset, to run clients in mainnet or minimal mode.
	// copy the default mainnet config, and make some minimal modifications for testnet usage
	spec := *configs.Mainnet
	spec.Config.GENESIS_FORK_VERSION = common.Version{0xff, 0, 0, 0}
	spec.Config.ALTAIR_FORK_VERSION = common.Version{0xff, 0, 0, 1}
	spec.Config.ALTAIR_FORK_EPOCH = 0 // TODO: time altair fork
	spec.Config.MERGE_FORK_EPOCH = 0  // TODO: time merge fork
	spec.Config.MERGE_FORK_VERSION = common.Version{0xff, 0, 0, 2}
	spec.Config.DEPOSIT_CONTRACT_ADDRESS = depositAddress
	spec.Config.DEPOSIT_CHAIN_ID = chainId
	spec.Config.DEPOSIT_NETWORK_ID = networkId
	spec.Config.ETH1_FOLLOW_DISTANCE = 1
	spec.Config.MIN_GENESIS_ACTIVE_VALIDATOR_COUNT = 64
	spec.Config.SECONDS_PER_SLOT = 1
	spec.Config.TERMINAL_TOTAL_DIFFICULTY = ttd
	spec.Config.MIN_GENESIS_TIME = common.Timestamp(genesisTime)
	return &spec
}

func generateValidatorKeyBundles(mnemonic string, valCount uint64, keyTranches uint64) ([]hivesim.StartOption, error) {
	keySrc := &setup.MnemonicsKeySource{
		From:       0,
		To:         valCount,
		Validator:  mnemonic,
		Withdrawal: mnemonic,
	}
	keys, err := keySrc.Keys()
	if err != nil {
		return nil, err
	}
	keyOpts := make([]hivesim.StartOption, 0, keyTranches)
	for i := uint64(0); i < keyTranches; i++ {
		// Give each validator client an equal subset of the genesis validator keys
		startIndex := valCount * i / keyTranches
		endIndex := valCount * (i + 1) / keyTranches
		keyOpts = append(keyOpts, setup.KeysBundle(keys[startIndex:endIndex]))
	}
	return keyOpts, nil
}

func prepareTestnet(t *hivesim.T, valCount uint64, keyTranches uint64, ttd *big.Int) *PreparedTestnet {
	// build genesis for execution clients
	eth1Genesis, executionOpts, err := buildExecutionClientOpts(depositAddress, ttd)
	if err != nil {
		t.Fatal(err)
	}
	executionOpts = append(executionOpts, hivesim.Params{
		"HIVE_CATALYST_ENABLED": "1",
		"HIVE_LOGLEVEL":         os.Getenv("HIVE_LOGLEVEL"),
		"HIVE_NODETYPE":         "full",
	})

	// generate keys for validators
	t.Logf("generating %d validator keys...", valCount)
	keyOpts, err := generateValidatorKeyBundles(mnemonic, valCount, keyTranches)
	if err != nil {
		t.Fatal(err)
	}

	// prepare genesis beacon state, with all of the validators in it.
	t.Log("building beacon state...")
	spec := buildBeaconSpec(depositAddress, ttd.Uint64(), eth1Genesis.Genesis.Config.ChainID.Uint64(), eth1Genesis.NetworkID, eth1Genesis.Genesis.Timestamp)

	// create configurations for beacon and validator clients
	eth2Config := setup.Eth2ConfigToParams(&spec.Config)

	beaconOpts := []hivesim.StartOption{
		eth2Config,
		hivesim.Params{
			"HIVE_ETH2_BN_API_PORT":  fmt.Sprintf("%d", PortBeaconAPI),
			"HIVE_ETH2_BN_GRPC_PORT": fmt.Sprintf("%d", PortBeaconGRPC),
			"HIVE_ETH2_METRICS_PORT": fmt.Sprintf("%d", PortMetrics),
			"HIVE_CHECK_LIVE_PORT":   fmt.Sprintf("%d", PortBeaconAPI),
		},
	}
	validatorOpts := []hivesim.StartOption{
		eth2Config,
		hivesim.Params{
			"HIVE_ETH2_BN_API_PORT":  fmt.Sprintf("%d", PortBeaconAPI),
			"HIVE_ETH2_BN_GRPC_PORT": fmt.Sprintf("%d", PortBeaconGRPC),
			"HIVE_ETH2_VC_API_PORT":  fmt.Sprintf("%d", PortValidatorAPI),
			"HIVE_ETH2_METRICS_PORT": fmt.Sprintf("%d", PortMetrics),
			"HIVE_CHECK_LIVE_PORT":   "0", // TODO not every validator client has an API or metrics to check if it's live
		},
	}

	// We need a new genesis time, so we take the template state and prepare a tar with updated time
	stateBundleOpt, err := setup.StateBundle(spec, mnemonic, valCount, keyTranches)
	if err != nil {
		t.Fatal(err)
	}
	beaconOpts = append(beaconOpts, stateBundleOpt...)

	return &PreparedTestnet{
		spec:          spec,
		eth1Genesis:   eth1Genesis,
		executionOpts: executionOpts,
		beaconOpts:    beaconOpts,
		validatorOpts: validatorOpts,
		keyTranches:   keyOpts,
	}
}

func (p *PreparedTestnet) createTestnet(t *hivesim.T) *Testnet {
	return &Testnet{
		t:           t,
		spec:        p.spec,
		eth1Genesis: p.eth1Genesis,
	}
}

func (p *PreparedTestnet) startEth1Node(testnet *Testnet, eth1Def *hivesim.ClientDefinition, simulated bool) {
	testnet.t.Logf("starting eth1 node: %s (%s)", eth1Def.Name, eth1Def.Version)

	opts := p.executionOpts
	if len(testnet.eth1) == 0 {
		if !simulated {
			// we only make the first eth1 node a miner
			opts = append(opts, hivesim.Params{"HIVE_MINER": "0x1212121212121212121212121212121212121212"})
		}
	} else {
		bootnode, err := testnet.eth1[0].EnodeURL()
		if err != nil {
			testnet.t.Fatalf("failed to get eth1 bootnode URL: %v", err)
		}

		// Make the client connect to the first eth1 node, as a bootnode for the eth1 net
		opts = append(opts, hivesim.Params{"HIVE_BOOTNODE": bootnode})
	}

	en := &Eth1Node{testnet.t.StartClient(eth1Def.Name, opts...)}
	testnet.eth1 = append(testnet.eth1, en)
}

func (p *PreparedTestnet) startBeaconNode(testnet *Testnet, beaconDef *hivesim.ClientDefinition, eth1Endpoints []int) {
	testnet.t.Logf("starting beacon node: %s (%s)", beaconDef.Name, beaconDef.Version)

	opts := p.beaconOpts
	// Hook up beacon node to (maybe multiple) eth1 nodes
	for _, index := range eth1Endpoints {
		if index < 0 || index >= len(testnet.eth1) {
			testnet.t.Fatalf("only have %d eth1 nodes, cannot find index %d for BN", len(testnet.eth1), index)
		}
	}

	var addrs []string
	for _, index := range eth1Endpoints {
		eth1Node := testnet.eth1[index]
		userRPC, err := eth1Node.UserRPCAddress()
		if err != nil {
			testnet.t.Fatalf("eth1 node used for beacon without available RPC: %v", err)
		}
		addrs = append(addrs, userRPC)
	}
	opts = append(opts, hivesim.Params{"HIVE_ETH2_ETH1_RPC_ADDRS": strings.Join(addrs, ",")})

	if len(testnet.beacons) > 0 {
		bootnodeENR, err := testnet.beacons[0].ENR()
		if err != nil {
			testnet.t.Fatalf("failed to get ENR as bootnode for beacon node: %v", err)
		}
		opts = append(opts, hivesim.Params{"HIVE_ETH2_BOOTNODE_ENRS": bootnodeENR})
	}

	// TODO
	//if p.configName != "mainnet" && hasBuildTarget(beaconDef, p.configName) {
	//	opts = append(opts, hivesim.WithBuildTarget(p.configName))
	//}
	bn := NewBeaconNode(testnet.t.StartClient(beaconDef.Name, opts...))
	testnet.beacons = append(testnet.beacons, bn)
}

func (p *PreparedTestnet) startValidatorClient(testnet *Testnet, validatorDef *hivesim.ClientDefinition, bnIndex int, keyIndex int) {
	testnet.t.Logf("starting validator client: %s (%s)", validatorDef.Name, validatorDef.Version)

	if bnIndex >= len(testnet.beacons) {
		testnet.t.Fatalf("only have %d beacon nodes, cannot find index %d for VC", len(testnet.beacons), bnIndex)
	}
	bn := testnet.beacons[bnIndex]
	// Hook up validator to beacon node
	bnAPIOpt := hivesim.Params{
		"HIVE_ETH2_BN_API_IP": bn.IP.String(),
	}
	if keyIndex >= len(p.keyTranches) {
		testnet.t.Fatalf("only have %d key tranches, cannot find index %d for VC", len(p.keyTranches), keyIndex)
	}
	keysOpt := p.keyTranches[keyIndex]
	opts := p.validatorOpts
	opts = append(opts, keysOpt)
	opts = append(opts, bnAPIOpt)
	// TODO
	//if p.configName != "mainnet" && hasBuildTarget(validatorDef, p.configName) {
	//	opts = append(opts, hivesim.WithBuildTarget(p.configName))
	//}
	vc := &ValidatorClient{testnet.t.StartClient(validatorDef.Name, opts...)}
	testnet.validators = append(testnet.validators, vc)
}
