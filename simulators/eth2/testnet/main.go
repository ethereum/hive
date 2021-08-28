package main

import (
	"errors"
	"fmt"
	"github.com/ethereum/hive/hivesim"
	"github.com/ethereum/hive/simulators/eth2/testnet/setup"
	"github.com/protolambda/zrnt/eth2/beacon/common"
	"github.com/protolambda/zrnt/eth2/configs"
	"strings"
	"time"
)

func main() {
	var suite = hivesim.Suite{
		Name:        "eth2-testnet",
		Description: `Run different eth2 testnets.`,
	}
	suite.Add(hivesim.TestSpec{
		Name:        "testnets",
		Description: "Collection of different testnet compositions and assertions.",
		Run: func(t *hivesim.T) {
			clientTypes, err := t.Sim.ClientTypes()
			if err != nil {
				t.Fatal(err)
			}
			nodeChoices(clientTypes).runTests(t)
		},
	})
	hivesim.MustRunSuite(hivesim.New(), suite)
}

type NodeChoices struct {
	beacon    []*hivesim.ClientDefinition
	validator []*hivesim.ClientDefinition
	eth1      []*hivesim.ClientDefinition
	other     []*hivesim.ClientDefinition
}

func nodeChoices(available []*hivesim.ClientDefinition) *NodeChoices {
	var out NodeChoices
	for _, client := range available {
		if client.HasRole("beacon") {
			out.beacon = append(out.beacon, client)
		}
		if client.HasRole("validator") {
			out.validator = append(out.validator, client)
		}
		if client.HasRole("eth1") {
			out.eth1 = append(out.eth1, client)
		}
	}
	return &out
}

func ComposeTestnetTest(name string, description string, build ...TestnetDecorator) hivesim.TestSpec {
	return hivesim.TestSpec{
		Name:        name,
		Description: description,
		Run: func(t *hivesim.T) {
			testnet := NewTestnet(t)
			for _, td := range build {
				td(testnet)
			}
			// TODO: change this based on testnet configuration, or do polling
			time.Sleep(time.Minute * 10)

			// TODO: ask all beacon nodes if they have finalized
			for _, b := range testnet.beacons {
				// apiAddr := fmt.Sprintf("http://%s:4000", b.IP)
				testnet.t.Errorf("node %s failed to finalize", b.Container)
				// TODO use Go API bindings to request finality status, error per non-finalized node.
			}
		},
	}
}

func (nc *NodeChoices) runTests(t *hivesim.T) {
	for _, beaconDef := range nc.beacon {

		validatorDef, err := nc.preferredValidator(beaconDef.Name)
		if err != nil {
			t.Fatal(err)
		}
		t.Run(ComposeTestnetTest(
			"single-client-testnet",
			"This runs quick eth2 single-client testnets, beacon nodes matched with preferred validator type, and dummy eth1 endpoint.",
			func(testnet *Testnet) {

				prep := prepareTestnet(t)
				for i := 0; i < len(prep.keys); i++ {
					prep.buildBeaconNode(beaconDef, nil)(testnet)
					prep.buildValidatorClient(validatorDef, i, i)(testnet)
				}
			},
		))
	}

	/*
		TODO More testnet ideas:

		Name:        "two-client-testnet",
		Description: "This runs quick eth2 testnets with combinations of 2 client types, beacon nodes matched with preferred validator type, and dummy eth1 endpoint.",
		Name:        "all-client-testnet",
		Description: "This runs a quick eth2 testnet with all client types, beacon nodes matched with preferred validator type, and dummy eth1 endpoint.",
		Name:        "cross-single-client-testnet",
		Description: "This runs a quick eth2 single-client testnet, but beacon nodes are matched with all validator types",
	*/
}

func (nc *NodeChoices) preferredValidator(beacon string) (*hivesim.ClientDefinition, error) {
	beaconVendor := strings.Split(beacon, "-")[0]
	for _, v := range nc.validator {
		valVendor := strings.Split(v.Name, "-")[0]
		if beaconVendor == valVendor {
			return v, nil
		}
	}
	return nil, fmt.Errorf("no preferred validator for beacon %s", beacon)
}

// PreparedTestnet has all the options for starting nodes, ready to build the network.
type PreparedTestnet struct {
	spec                  *common.Spec
	eth1Genesis           *setup.Eth1Genesis
	commonEth1Params      hivesim.StartOption
	commonValidatorParams hivesim.StartOption
	commonBeaconParams    hivesim.StartOption
	eth1ConfigOpt         hivesim.StartOption
	eth2ConfigOpt         hivesim.StartOption
	beaconStateOpt        hivesim.StartOption
	keys                  []hivesim.StartOption
}

func prepareTestnet(t *hivesim.T) *PreparedTestnet {

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

	valCount := uint64(4096)
	keyPartitions := uint64(4)

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
	keyOpts := make([]hivesim.StartOption, keyPartitions)
	for i := uint64(0); i < keyPartitions; i++ {
		// Give each validator client an equal subset of the genesis validator keys
		startIndex := valCount * i / keyPartitions
		endIndex := valCount * (i + 1) / keyPartitions
		keyOpts = append(keyOpts, setup.KeysBundle(keys[startIndex:endIndex]))
	}

	// prepare genesis beacon state, with all of the validators in it.
	state, err := setup.BuildBeaconState(eth1Genesis, spec, keys)
	if err != nil {
		t.Fatal(err)
	}
	// Make all eth1 nodes mine blocks. Merge fork would deactivate this on the fly.
	eth1Params := hivesim.Params{
		"HIVE_MINER": "0x1212121212121212121212121212121212121212",
	}
	beaconParams := hivesim.Params{
		"HIVE_ETH2_BN_API_PORT": "4000",
	}
	validatorParams := hivesim.Params{
		"HIVE_ETH2_BN_API_PORT": "4000",
	}

	// we need a new genesis time, so we take the template state and prepare a tar with updated time
	stateOpt, err := setup.StateBundle(state, time.Now().Add(2*time.Minute))
	if err != nil {
		t.Fatal(err)
	}

	return &PreparedTestnet{
		spec:                  spec,
		eth1Genesis:           eth1Genesis,
		commonEth1Params:      eth1Params,
		commonBeaconParams:    beaconParams,
		commonValidatorParams: validatorParams,
		eth1ConfigOpt:         eth1Config,
		eth2ConfigOpt:         eth2Config,
		beaconStateOpt:        stateOpt,
		keys:                  keyOpts,
	}
}

type TestnetDecorator func(testnet *Testnet)

type BeaconNode struct {
	*hivesim.Client
}

func (bn *BeaconNode) ENR() (string, error) {
	// TODO expose getter for P2P address
	return "", nil
}

func (bn *BeaconNode) EnodeURL() (string, error) {
	return "", errors.New("beacon node does not have an discv4 Enode URL, use ENR or multi-address instead")
}

// TODO expose beacon API endpoint

type Testnet struct {
	t          *hivesim.T
	beacons    []*BeaconNode
	validators []*hivesim.Client
	eth1       []*hivesim.Client
}

func NewTestnet(t *hivesim.T) *Testnet {
	return &Testnet{t: t}
}

func (p *PreparedTestnet) buildEth1Node(eth1Def *hivesim.ClientDefinition) TestnetDecorator {
	return func(testnet *Testnet) {
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
		}

		n := testnet.t.StartClient(eth1Def.Name, opts...)

		testnet.eth1 = append(testnet.eth1, n)
	}
}

func (p *PreparedTestnet) buildBeaconNode(beaconDef *hivesim.ClientDefinition, eth1Endpoints []int) TestnetDecorator {
	return func(testnet *Testnet) {
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
			addrs = append(addrs, fmt.Sprintf("http://%v:8545", eth1Node.IP))
		}
		opts = append(opts, hivesim.Params{"ETH1_RPC_ADDRS": strings.Join(addrs, ",")})

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
		bn := &BeaconNode{Client: testnet.t.StartClient(beaconDef.Name, opts...)}
		testnet.beacons = append(testnet.beacons, bn)
	}
}

func (p *PreparedTestnet) buildValidatorClient(validatorDef *hivesim.ClientDefinition, bnIndex int, keyIndex int) func(testnet *Testnet) {
	return func(testnet *Testnet) {
		if bnIndex >= len(testnet.beacons) {
			testnet.t.Fatalf("only have %d beacon nodes, cannot find index %d for VC", len(testnet.beacons), bnIndex)
		}
		bn := testnet.beacons[bnIndex]
		// Hook up validator to beacon node
		bnAPIOpt := hivesim.Params{
			"HIVE_ETH2_BN_API_IP": bn.IP.String(),
		}
		if keyIndex >= len(p.keys) {
			testnet.t.Fatalf("only have %d key tranches, cannot find index %d for VC", len(p.keys), keyIndex)
		}
		keysOpt := p.keys[keyIndex]
		opts := []hivesim.StartOption{
			p.eth2ConfigOpt, keysOpt, p.commonValidatorParams, bnAPIOpt,
		}
		// TODO
		//if p.configName != "mainnet" && hasBuildTarget(validatorDef, p.configName) {
		//	opts = append(opts, hivesim.WithBuildTarget(p.configName))
		//}
		vc := testnet.t.StartClient(validatorDef.Name, opts...)
		testnet.validators = append(testnet.validators, vc)
	}
}
