package main

import (
	"fmt"
	"github.com/ethereum/hive/hivesim"
	"github.com/ethereum/hive/simulators/eth2/testnet/setup"
	"github.com/protolambda/zrnt/eth2/beacon/common"
	"github.com/protolambda/zrnt/eth2/configs"
	"strings"
	"time"
)

// PreparedTestnet has all the options for starting nodes, ready to build the network.
type PreparedTestnet struct {
	// Consensus chain configuration
	spec                  *common.Spec
	// Execution chain configuration and genesis info
	eth1Genesis           *setup.Eth1Genesis
	// Consensus genesis state
	eth2Genesis           common.BeaconState

	// configuration to apply to every node of the given type
	commonEth1Params      hivesim.StartOption
	commonValidatorParams hivesim.StartOption
	commonBeaconParams    hivesim.StartOption

	// embeds eth1 configuration into a node
	eth1ConfigOpt         hivesim.StartOption
	// embeds eth2 configuration into a node
	eth2ConfigOpt         hivesim.StartOption
	// embeds the genesis beacon state into a node
	beaconStateOpt        hivesim.StartOption

	// a tranche is a group of validator keys to run on 1 node
	keyTranches           []hivesim.StartOption
}

func prepareTestnet(t *hivesim.T, valCount uint64, keyTranches uint64) *PreparedTestnet {

	var depositAddress common.Eth1Address
	depositAddress.UnmarshalText([]byte("0x4242424242424242424242424242424242424242"))

	eth1Genesis := setup.BuildEth1Genesis()
	eth1Config := eth1Genesis.ToParams(depositAddress)

	var spec *common.Spec
	{
		// TODO: specify build-target based on preset, to run clients in mainnet or minimal mode.
		// copy the default mainnet config, and make some minimal modifications for testnet usage
		tmp := *configs.Mainnet
		tmp.Config.GENESIS_FORK_VERSION = common.Version{0xff, 0, 0, 0}
		tmp.Config.ALTAIR_FORK_VERSION = common.Version{0xff, 0, 0, 1}
		tmp.Config.ALTAIR_FORK_EPOCH = 10 // TODO: time altair fork
		tmp.Config.DEPOSIT_CONTRACT_ADDRESS = common.Eth1Address(eth1Genesis.DepositAddress)
		tmp.Config.DEPOSIT_CHAIN_ID = eth1Genesis.Genesis.Config.ChainID.Uint64()
		tmp.Config.DEPOSIT_NETWORK_ID = eth1Genesis.NetworkID
		spec = &tmp
	}

	eth2Config := setup.Eth2ConfigToParams(&spec.Config)

	t.Logf("generating %d validator keys...", valCount)
	mnemonic := "couple kiwi radio river setup fortune hunt grief buddy forward perfect empty slim wear bounce drift execute nation tobacco dutch chapter festival ice fog"
	keySrc := &setup.MnemonicsKeySource{
		From:       0,
		To:         valCount,
		Validator:  mnemonic,
		Withdrawal: mnemonic,
	}
	keys, err := keySrc.Keys()
	if err != nil {
		t.Fatal(err)
	}
	keyOpts := make([]hivesim.StartOption, 0, keyTranches)
	for i := uint64(0); i < keyTranches; i++ {
		// Give each validator client an equal subset of the genesis validator keys
		startIndex := valCount * i / keyTranches
		endIndex := valCount * (i + 1) / keyTranches
		keyOpts = append(keyOpts, setup.KeysBundle(keys[startIndex:endIndex]))
	}

	t.Log("building beacon state...")
	// prepare genesis beacon state, with all of the validators in it.
	state, err := setup.BuildBeaconState(eth1Genesis, spec, keys)
	if err != nil {
		t.Fatal(err)
	}
	// Make all eth1 nodes mine blocks. Merge fork would deactivate this on the fly.
	eth1Params := hivesim.Params{
	}
	beaconParams := hivesim.Params{
		"HIVE_ETH2_BN_API_PORT": fmt.Sprintf("%d", PortBeaconAPI),
		"HIVE_ETH2_BN_GRPC_PORT": fmt.Sprintf("%d", PortBeaconGRPC),
		"HIVE_ETH2_METRICS_PORT": fmt.Sprintf("%d", PortMetrics),
		"HIVE_CHECK_LIVE_PORT": fmt.Sprintf("%d", PortBeaconAPI),
	}
	validatorParams := hivesim.Params{
		"HIVE_ETH2_BN_API_PORT": fmt.Sprintf("%d", PortBeaconAPI),
		"HIVE_ETH2_BN_GRPC_PORT": fmt.Sprintf("%d", PortBeaconGRPC),
		"HIVE_ETH2_VC_API_PORT": fmt.Sprintf("%d", PortValidatorAPI),
		"HIVE_ETH2_METRICS_PORT": fmt.Sprintf("%d", PortMetrics),
		"HIVE_CHECK_LIVE_PORT": "0",  // TODO not every validator client has an API or metrics to check if it's live
	}

	// we need a new genesis time, so we take the template state and prepare a tar with updated time
	stateOpt, err := setup.StateBundle(state, time.Now().Add(2*time.Minute))
	if err != nil {
		t.Fatal(err)
	}

	t.Log("prepared testnet!")
	return &PreparedTestnet{
		spec:                  spec,
		eth1Genesis:           eth1Genesis,
		eth2Genesis:           state,
		commonEth1Params:      eth1Params,
		commonBeaconParams:    beaconParams,
		commonValidatorParams: validatorParams,
		eth1ConfigOpt:         eth1Config,
		eth2ConfigOpt:         eth2Config,
		beaconStateOpt:        stateOpt,
		keyTranches:           keyOpts,
	}
}

func (p *PreparedTestnet) createTestnet(t *hivesim.T) *Testnet {
	time, _ := p.eth2Genesis.GenesisTime()
	valRoot, _ := p.eth2Genesis.GenesisValidatorsRoot()
	return &Testnet{
		t: t,
		genesisTime:           time,
		genesisValidatorsRoot: valRoot,
		spec:                  p.spec,
		eth1Genesis:           p.eth1Genesis,
	}
}

func (p *PreparedTestnet) startEth1Node(testnet *Testnet, eth1Def *hivesim.ClientDefinition) {
	testnet.t.Logf("starting eth1 node: %s (%s)", eth1Def.Name, eth1Def.Version)

	opts := []hivesim.StartOption{
		p.eth1ConfigOpt, p.commonEth1Params,
	}
	if len(testnet.eth1) > 0 {
		bootnode, err := testnet.eth1[0].EnodeURL()
		if err != nil {
			testnet.t.Fatalf("failed to get eth1 bootnode URL: %v", err)
		}

		// Make the client connect to the first eth1 node, as a bootnode for the eth1 net
		opts = append(opts, hivesim.Params{"HIVE_BOOTNODE": bootnode})
	} else {
		// we only make the first eth1 node a miner
		opts = append(opts, hivesim.Params{"HIVE_MINER": "0x1212121212121212121212121212121212121212"})
	}

	en := &Eth1Node{testnet.t.StartClient(eth1Def.Name, opts...)}
	testnet.eth1 = append(testnet.eth1, en)
}

func (p *PreparedTestnet) startBeaconNode(testnet *Testnet, beaconDef *hivesim.ClientDefinition, eth1Endpoints []int) {
	testnet.t.Logf("starting beacon node: %s (%s)", beaconDef.Name, beaconDef.Version)

	opts := []hivesim.StartOption{p.eth2ConfigOpt, p.beaconStateOpt, p.commonBeaconParams}
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
	opts := []hivesim.StartOption{
		p.eth2ConfigOpt, keysOpt, p.commonValidatorParams, bnAPIOpt,
	}
	// TODO
	//if p.configName != "mainnet" && hasBuildTarget(validatorDef, p.configName) {
	//	opts = append(opts, hivesim.WithBuildTarget(p.configName))
	//}
	vc := &ValidatorClient{testnet.t.StartClient(validatorDef.Name, opts...)}
	testnet.validators = append(testnet.validators, vc)
}
