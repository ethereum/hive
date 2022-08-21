package optimism

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/ethereum-optimism/optimism/op-node/rollup"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/stretchr/testify/require"
	"sync"
	"time"

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
	L1Vault     *Vault
	L2Vault     *Vault

	Proposer *ProposerNode
	Batcher  *BatcherNode

	L1Cfg     *core.Genesis
	L2Cfg     *core.Genesis
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
	roles := Roles(clientTypes)
	t.Logf("creating devnet with client roles: %s", roles)
	return &Devnet{
		T:           t,
		Clients:     roles,
		MnemonicCfg: DefaultMnemonicConfig,
		Secrets:     secrets,
		L1Vault:     NewVault(t, L1ChainIDBig),
		L2Vault:     NewVault(t, L2ChainIDBig),
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

	// Even though these exact params are already specified,
	// the mapper thing in go-ethereum removes the fields in favor of the Env var fields.
	// So we preserve them by doing this hack.
	eth1Params := Eth1GenesisToParams(d.L1Cfg.Config)

	eth1Cfg, err := json.Marshal(d.L1Cfg)
	if err != nil {
		d.T.Fatalf("failed to serialize genesis state: %v", err)
	}
	eth1CfgOpt := bytesFile("/genesis.json", eth1Cfg)

	input := []hivesim.StartOption{eth1CfgOpt, eth1Params}
	if len(d.Eth1s) == 0 {
		// we only make the first eth1 node a miner
		input = append(input, hivesim.Params{
			"HIVE_CLIQUE_PRIVATEKEY": EncodePrivKey(d.Secrets.CliqueSigner).String()[2:], // no 0x prefix here
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
		// just repeat that of the genesis block (rollup cfg is not initialized yet)
		"HIVE_ETHERBASE": d.L2Cfg.Coinbase.String(),
	}
	input := []hivesim.StartOption{defaultSettings}

	l2GenesisCfg, err := json.Marshal(d.L2Cfg)
	if err != nil {
		d.T.Fatalf("failed to encode l2 genesis: %v", err)
		return
	}
	input = append(input, bytesFile("/genesis.json", l2GenesisCfg))
	input = append(input, defaultJWTFile)
	input = append(input, opts...)

	c := &OpL2Engine{ELNode{d.T.StartClient(d.Clients.OpL2[0].Name, input...)}}
	d.T.Logf("added op-l2 %d: %s", len(d.OpL2Engines), c.IP)
	d.OpL2Engines = append(d.OpL2Engines, c)
}

// AddOpNode creates a new Optimism rollup node. This requires a rollup config to be created previously.
func (d *Devnet) AddOpNode(eth1Index int, l2EngIndex int, sequencer bool, opts ...hivesim.StartOption) {
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
	seqStr := "false"
	if sequencer {
		seqStr = "true"
	}
	defaultSettings := HiveUnpackParams{
		opnf.L1NodeAddr.EnvVar:           eth1Node.WsRpcEndpoint(),
		opnf.L2EngineAddr.EnvVar:         l2Engine.WsRpcEndpoint(),
		opnf.RollupConfig.EnvVar:         "/rollup_config.json",
		opnf.RPCListenAddr.EnvVar:        "0.0.0.0",
		opnf.RPCListenPort.EnvVar:        fmt.Sprintf("%d", RollupRPCPort),
		opnf.L1TrustRPC.EnvVar:           "false",
		opnf.L2EngineJWTSecret.EnvVar:    defaultJWTPath,
		opnf.LogLevelFlag.EnvVar:         "debug",
		opnf.SequencerEnabledFlag.EnvVar: seqStr,
		opnf.SequencerL1Confs.EnvVar:     "0",
		opnf.SequencerP2PKeyFlag.EnvVar:  defaultP2PSequencerKeyPath,
	}
	input := []hivesim.StartOption{defaultSettings.Params()}

	rollupCfg, err := json.Marshal(d.RollupCfg)
	if err != nil {
		d.T.Fatalf("failed to encode l2 genesis: %v", err)
		return
	}
	input = append(input, bytesFile("/rollup_config.json", rollupCfg))
	input = append(input, defaultJWTFile)
	input = append(input, defaultP2pSequencerKeyFile)
	input = append(input, opts...)

	c := &OpNode{d.T.StartClient(d.Clients.OpNode[0].Name, input...)}
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

	defaultSettings := HiveUnpackParams{
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
		"OP_PROPOSER_LOG_LEVEL":                   "debug",
	}
	input := []hivesim.StartOption{defaultSettings.Params()}
	input = append(input, opts...)

	c := &ProposerNode{d.T.StartClient(d.Clients.OpProposer[0].Name, input...)}
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

	defaultSettings := HiveUnpackParams{
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
		"OP_BATCHER_LOG_LEVEL":                     "debug",
	}
	input := []hivesim.StartOption{defaultSettings.Params()}
	input = append(input, opts...)

	c := &BatcherNode{d.T.StartClient(d.Clients.OpBatcher[0].Name, input...)}
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

func (d *Devnet) WaitUpEth1(i int, wait time.Duration) {
	ctx, cancel := context.WithTimeout(context.Background(), wait)
	defer cancel()
	_, err := d.GetEth1(i).EthClient().ChainID(ctx)
	require.NoError(d.T, err, "eth1 node should be up within %s", wait)
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

func (d *Devnet) WaitUpOpL2Engine(i int, wait time.Duration) {
	ctx, cancel := context.WithTimeout(context.Background(), wait)
	defer cancel()
	_, err := d.GetOpL2Engine(i).EthClient().ChainID(ctx)
	require.NoError(d.T, err, "op l2 engine node should be up within %s", wait)
}

func (d *Devnet) L1Client(i int) *ethclient.Client {
	return d.GetEth1(i).EthClient()
}

func (d *Devnet) L2Client(i int) *ethclient.Client {
	return d.GetOpL2Engine(i).EthClient()
}

type SequencerDevnetParams struct {
	MaxSeqDrift             uint64
	SeqWindowSize           uint64
	ChanTimeout             uint64
	AdditionalGenesisAllocs core.GenesisAlloc
}

func StartSequencerDevnet(ctx context.Context, d *Devnet, params *SequencerDevnetParams) error {
	d.InitContracts()
	d.InitHardhatDeployConfig("earliest", params.MaxSeqDrift, params.SeqWindowSize, params.ChanTimeout)
	d.InitL1Hardhat()
	d.AddEth1()
	d.WaitUpEth1(0, time.Second*10)

	//var block *types.Block
	//if err := WaitBlock(ctx, d.L1Client(0), params.SeqWindowSize); err != nil {
	//	return err
	//}

	d.DeployL1Hardhat()
	d.InitL2Hardhat(params.AdditionalGenesisAllocs)
	d.AddOpL2()
	d.WaitUpOpL2Engine(0, time.Second*10)
	d.InitRollupHardhat()

	d.AddOpNode(0, 0, true)
	d.AddOpBatcher(0, 0, 0)
	d.AddOpProposer(0, 0, 0)

	block, err := d.L1Client(0).BlockByNumber(ctx, nil)
	if err != nil {
		return err
	}
	expHeight := uint64(time.Now().Sub(time.Unix(int64(block.Time()), 0)).Seconds() / 2)
	if err := WaitBlock(ctx, d.L2Client(0), expHeight); err != nil {
		return err
	}
	return nil
}
