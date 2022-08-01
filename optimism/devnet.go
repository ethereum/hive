package optimism

import (
	"encoding/json"
	"fmt"
	"github.com/ethereum-optimism/optimism/op-node/rollup"
	"github.com/ethereum/go-ethereum/params"
	"sync"

	opbf "github.com/ethereum-optimism/optimism/op-batcher/flags"
	opnf "github.com/ethereum-optimism/optimism/op-node/flags"
	oppf "github.com/ethereum-optimism/optimism/op-proposer/flags"
	"github.com/ethereum/hive/hivesim"
)

type Devnet struct {
	mu sync.Mutex

	T       *hivesim.T
	Clients *ClientsByRole

	Contracts   *OpContracts
	Eth1s       []*Eth1Node
	OpL2Engines []*OpL2Engine
	OpNodes     []*OpNode

	Proposer *ProposerNode
	Batcher  *BatcherNode

	L1Cfg     *params.ChainConfig
	L2Cfg     *params.ChainConfig
	RollupCfg *rollup.Config

	MnemonicCfg MnemonicConfig
	Secrets     *Secrets
	Addresses   *Addresses

	Deployments Deployments
	Bindings    Bindings
}

func NewDevnet(t *hivesim.T) *Devnet {
	clientTypes, err := t.Sim.ClientTypes()
	if err != nil {
		t.Fatalf("failed to retrieve list of client types: %v", err)
	}
	// we may want to make this configurable, but let's default to the same keys for every hive net
	secrets, err := DefaultMnemonicConfig.Secrets()
	if err != nil {
		t.Fatalf("failed to parse testnet secrets: %v", err)
	}

	return &Devnet{
		T:           t,
		Clients:     Roles(clientTypes),
		MnemonicCfg: DefaultMnemonicConfig,
		Secrets:     secrets,
		Addresses:   secrets.Addresses(),
	}
}

func (d *Devnet) InitContracts(opts ...hivesim.StartOption) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if len(d.Clients.OpContracts) == 0 {
		d.T.Fatal("no op-contracts client types found")
		return
	}
	if d.Contracts != nil {
		d.T.Fatalf("already initialized op contracts client: %v", d.Contracts)
		return
	}
	clDef := d.Clients.OpContracts[0]
	cl := d.T.StartClient(clDef.Name, opts...)
	d.Contracts = &OpContracts{cl}
	return
}

// AddEth1 creates a new L1 eth1 client. This requires a L1 chain config to be created previously.
func (d *Devnet) AddEth1(opts ...hivesim.StartOption) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if len(d.Clients.Eth1) == 0 {
		d.T.Fatal("no eth1 client types found")
		return
	}
	if d.L1Cfg == nil {
		d.T.Fatal("no eth1 L1 chain configuration found")
		return
	}

	// Even though the exact configuration is provided already,
	// hive allows overrides of individual config vars by env params.
	// If any eth1 client uses these params, let's just make sure they are there.
	eth1Params := Eth1GenesisToParams(d.L1Cfg)

	eth1Cfg, err := json.Marshal(d.L1Cfg)
	if err != nil {
		d.T.Fatalf("failed to serialize genesis state: %v", err)
	}
	eth1CfgOpt := bytesFile("genesis.json", eth1Cfg)

	input := []hivesim.StartOption{eth1CfgOpt, eth1Params}
	if len(d.Eth1s) == 0 {
		// we only make the first eth1 node a miner
		input = append(input, hivesim.Params{
			"HIVE_CLIQUE_PRIVATEKEY": EncodePrivKey(d.Secrets.CliqueSigner).String(),
			"HIVE_MINER":             d.Addresses.CliqueSigner.String(),
		})
	} else {
		bootnode, err := d.Eth1s[0].EnodeURL()
		if err != nil {
			d.T.Fatalf("failed to get eth1 bootnode URL: %v", err)
		}
		// Make the client connect to the first eth1 node, as a bootnode for the eth1 net
		input = append(input, hivesim.Params{"HIVE_BOOTNODE": bootnode})
	}

	c := &Eth1Node{ELNode{d.T.StartClient(d.Clients.Eth1[0].Name, input...)}}
	d.T.Logf("added eth1 node %d of type %s: %s", len(d.OpL2Engines), d.Clients.Eth1[0].Name, c.IP)
	d.Eth1s = append(d.Eth1s, c)
}

// AddOpL2 creates a new Optimism L2 execution engine. This requires a L2 chain config to be created previously.
func (d *Devnet) AddOpL2(opts ...hivesim.StartOption) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if len(d.Clients.OpL2) == 0 {
		d.T.Fatal("no op-l2 engine client types found")
		return
	}
	if d.L2Cfg == nil {
		d.T.Fatal("no op-l2 chain configuration found")
		return
	}
	defaultSettings := hivesim.Params{
		"HIVE_ETH1_LOGLEVEL": "3",
	}
	input := []hivesim.StartOption{defaultSettings}

	l2GenesisCfg, err := json.Marshal(&d.L2Cfg)
	if err != nil {
		d.T.Fatalf("failed to encode l2 genesis: %v", err)
		return
	}
	input = append(input, bytesFile("genesis.json", l2GenesisCfg))
	input = append(input, opts...)

	c := &OpL2Engine{ELNode{d.T.StartClient(d.Clients.OpL2[0].Name, opts...)}}
	d.T.Logf("added op-l2 %d: %s", len(d.OpL2Engines), c.IP)
	d.OpL2Engines = append(d.OpL2Engines, c)
}

// AddOpNode creates a new Optimism rollup node. This requires a rollup config to be created previously.
func (d *Devnet) AddOpNode(eth1Index int, l2EngIndex int, opts ...hivesim.StartOption) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if len(d.Clients.OpNode) == 0 {
		d.T.Fatal("no op-node client types found")
		return
	}
	if d.RollupCfg == nil {
		d.T.Fatal("no rollup configuration found")
		return
	}
	eth1Node := d.GetEth1(eth1Index)
	l2Engine := d.GetOpL2Engine(l2EngIndex)

	defaultSettings := UnprefixedParams{
		opnf.L1NodeAddr.EnvVar:        eth1Node.WsRpcEndpoint(),
		opnf.L2EngineAddr.EnvVar:      l2Engine.WsRpcEndpoint(),
		opnf.RPCListenAddr.EnvVar:     "127.0.0.1",
		opnf.RPCListenPort.EnvVar:     fmt.Sprintf("%d", RollupRPCPort),
		opnf.L1TrustRPC.EnvVar:        "false",
		opnf.L2EngineJWTSecret.EnvVar: defaultJWTPath,
		opnf.LogLevelFlag.EnvVar:      "debug",
	}
	input := []hivesim.StartOption{defaultSettings.Params()}
	input = append(input, defaultJWTFile)
	input = append(input, opts...)

	c := &OpNode{d.T.StartClient(d.Clients.OpNode[0].Name, opts...)}
	d.T.Logf("added op-node %d: %s", len(d.OpNodes), c.IP)
	d.OpNodes = append(d.OpNodes, c)
}

func (d *Devnet) AddOpProposer(eth1Index int, l2EngIndex int, opNodeIndex int, opts ...hivesim.StartOption) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if len(d.Clients.OpProposer) == 0 {
		d.T.Fatal("no op-proposer client types found")
		return
	}
	if d.Proposer != nil {
		d.T.Fatal("already initialized op proposer")
		return
	}

	eth1Node := d.GetEth1(eth1Index)
	opNode := d.GetOpNode(opNodeIndex)
	l2Engine := d.GetOpL2Engine(l2EngIndex)

	defaultSettings := UnprefixedParams{
		oppf.L1EthRpcFlag.EnvVar:                  eth1Node.WsRpcEndpoint(),
		oppf.L2EthRpcFlag.EnvVar:                  l2Engine.WsRpcEndpoint(),
		oppf.RollupRpcFlag.EnvVar:                 opNode.HttpRpcEndpoint(),
		oppf.L2OOAddressFlag.EnvVar:               d.Deployments.L2OutputOracleProxy.String(),
		oppf.PollIntervalFlag.EnvVar:              "10s",
		oppf.NumConfirmationsFlag.EnvVar:          "1",
		oppf.SafeAbortNonceTooLowCountFlag.EnvVar: "3",
		oppf.ResubmissionTimeoutFlag.EnvVar:       "30s",
		oppf.MnemonicFlag.EnvVar:                  d.MnemonicCfg.Mnemonic,
		oppf.L2OutputHDPathFlag.EnvVar:            d.MnemonicCfg.Proposer,
		oppf.LogLevelFlag.EnvVar:                  "debug",
	}
	input := []hivesim.StartOption{defaultSettings.Params()}
	input = append(input, opts...)

	c := &ProposerNode{d.T.StartClient(d.Clients.OpProposer[0].Name, opts...)}
	d.T.Logf("added op-proposer: %s", c.IP)
	d.Proposer = c
}

func (d *Devnet) AddOpBatcher(eth1Index int, l2EngIndex int, opNodeIndex int, opts ...hivesim.StartOption) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if len(d.Clients.OpBatcher) == 0 {
		d.T.Fatal("no op-batcher client types found")
		return
	}
	if d.Batcher != nil {
		d.T.Fatal("already initialized op batcher")
		return
	}

	eth1Node := d.GetEth1(eth1Index)
	opNode := d.GetOpNode(opNodeIndex)
	l2Engine := d.GetOpL2Engine(l2EngIndex)

	defaultSettings := UnprefixedParams{
		opbf.L1EthRpcFlag.EnvVar:                   eth1Node.WsRpcEndpoint(),
		opbf.L2EthRpcFlag.EnvVar:                   l2Engine.WsRpcEndpoint(),
		opbf.RollupRpcFlag.EnvVar:                  opNode.HttpRpcEndpoint(),
		opbf.MinL1TxSizeBytesFlag.EnvVar:           "1",
		opbf.MaxL1TxSizeBytesFlag.EnvVar:           "120000",
		opbf.ChannelTimeoutFlag.EnvVar:             "40",
		opbf.PollIntervalFlag.EnvVar:               "1s",
		opbf.NumConfirmationsFlag.EnvVar:           "1",
		opbf.SafeAbortNonceTooLowCountFlag.EnvVar:  "3",
		opbf.ResubmissionTimeoutFlag.EnvVar:        "30s",
		opbf.MnemonicFlag.EnvVar:                   d.MnemonicCfg.Mnemonic,
		opbf.SequencerHDPathFlag.EnvVar:            d.MnemonicCfg.Batcher,
		opbf.SequencerBatchInboxAddressFlag.EnvVar: d.RollupCfg.BatchInboxAddress.String(),
		opbf.LogLevelFlag.EnvVar:                   "debug",
	}
	input := []hivesim.StartOption{defaultSettings.Params()}
	input = append(input, opts...)

	c := &BatcherNode{d.T.StartClient(d.Clients.OpBatcher[0].Name, opts...)}
	d.T.Logf("added op-batcher: %s", c.IP)
	d.Batcher = c
}

func (d *Devnet) GetEth1(i int) *Eth1Node {
	if i < 0 || i >= len(d.Eth1s) {
		d.T.Fatalf("only have %d eth1 nodes, cannot find %d", len(d.Eth1s), i)
		return nil
	}
	return d.Eth1s[i]
}

func (d *Devnet) GetOpNode(i int) *OpNode {
	if i < 0 || i >= len(d.OpNodes) {
		d.T.Fatalf("only have %d op rollup nodes, cannot find %d", len(d.OpNodes), i)
		return nil
	}
	return d.OpNodes[i]
}

func (d *Devnet) GetOpL2Engine(i int) *OpL2Engine {
	if i < 0 || i >= len(d.OpL2Engines) {
		d.T.Fatalf("only have %d op-l2 nodes, cannot find %d", len(d.OpL2Engines), i)
		return nil
	}
	return d.OpL2Engines[i]
}
