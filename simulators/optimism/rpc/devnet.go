package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"math/big"
	"os"

	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/hive/hivesim"
)

var (
	// parameters used for signing transactions
	chainID  = big.NewInt(901)
	gasPrice = big.NewInt(30 * params.GWei)

	// would be nice to use a networkID that's different from chainID,
	// but some clients don't support the distinction properly.
	networkID = big.NewInt(901)
)

func bytesSource(data []byte) func() (io.ReadCloser, error) {
	return func() (io.ReadCloser, error) {
		return ioutil.NopCloser(bytes.NewReader(data)), nil
	}
}

type Devnet struct {
	t *hivesim.T

	eth1 *Eth1Node
	l2   *L2Node
	op   *OpNode
	l2os *L2OSNode
	bss  *BSSNode

	genesisTimestamp string
	l2Genesis        string

	l2ToL1MessagePasserJSON          string
	l2CrossDomainMessengerJSON       string
	optimismMintableTokenFactoryJSON string
	l2StandardBridgeJSON             string
	l1BlockJSON                      string

	l2OutputOracle string
	optimismPortal string

	nodes map[string]*hivesim.ClientDefinition
	ctx   context.Context
}

func (d *Devnet) Start() {
	clientTypes, err := d.t.Sim.ClientTypes()
	if err != nil {
		d.t.Fatal(err)
	}
	var eth1, l2, op, l2os, bss *hivesim.ClientDefinition
	for _, client := range clientTypes {
		if client.HasRole("op-l1") {
			eth1 = client
		}
		if client.HasRole("op-l2") {
			l2 = client
		}
		if client.HasRole("op-node") {
			op = client
		}
		if client.HasRole("op-proposer") {
			l2os = client
		}
		if client.HasRole("op-batcher") {
			bss = client
		}
	}

	if eth1 == nil || l2 == nil || op == nil || l2os == nil || bss == nil {
		d.t.Fatal("op-l1, op-l2, op-node, op-proposer, op-batcher required")
	}

	// Generate genesis for execution clients
	//    eth1Genesis := setup.BuildEth1Genesis(config.TerminalTotalDifficulty, uint64(eth1GenesisTime))
	//    eth1ConfigOpt := eth1Genesis.ToParams(depositAddress)
	//    eth1Bundle, err := setup.Eth1Bundle(eth1Genesis.Genesis)
	//    if err != nil {
	//            t.Fatal(err)
	//    }
	var eth1ConfigOpt, eth1Bundle hivesim.Params
	execNodeOpts := hivesim.Params{
		"HIVE_CATALYST_ENABLED": "1",
		"HIVE_LOGLEVEL":         os.Getenv("HIVE_LOGLEVEL"),
		"HIVE_NODETYPE":         "full",
	}
	executionOpts := hivesim.Bundle(eth1ConfigOpt, eth1Bundle, execNodeOpts)

	opts := []hivesim.StartOption{executionOpts}
	d.eth1 = &Eth1Node{d.t.StartClient(eth1.Name, opts...), 8545, 8546}

	d.nodes["op-l1"] = eth1
	d.nodes["op-l2"] = l2
	d.nodes["op-node"] = op
	d.nodes["op-proposer"] = l2os
	d.nodes["op-batcher"] = bss
}

func (d *Devnet) Wait() error {
	// TODO: wait until rpc connects
	client := ethclient.NewClient(d.eth1.Client.RPC())
	_, err := client.ChainID(d.ctx)
	return err
}

func (d *Devnet) DeployL1() error {
	execInfo, err := d.eth1.Client.Exec("deploy.sh")
	fmt.Println(execInfo.Stdout)
	return err
}

func (d *Devnet) Cat(path string) (string, error) {
	execInfo, err := d.eth1.Client.Exec("cat.sh", path)
	if err != nil {
		return "", err
	}
	return execInfo.Stdout, nil
}

func (d *Devnet) InitL2() error {
	genesisTimestamp, err := d.Cat("/hive/genesis_timestamp")
	if err != nil {
		return err
	}
	d.genesisTimestamp = genesisTimestamp

	l2OutputOracle, err := d.Cat("/hive/optimism/packages/contracts-bedrock/deployments/devnetL1/L2OutputOracleProxy.json")
	if err != nil {
		return err
	}
	d.l2OutputOracle = l2OutputOracle

	optimismPortal, err := d.Cat("/hive/optimism/packages/contracts-bedrock/deployments/devnetL1/OptimismPortalProxy.json")
	if err != nil {
		return err
	}
	d.optimismPortal = optimismPortal

	l2ToL1MessagePasserJSON, err := d.Cat("/hive/optimism/packages/contracts-bedrock/artifacts/contracts/L2/L2ToL1MessagePasser.sol/L2ToL1MessagePasser.json")
	if err != nil {
		return err
	}
	d.l2ToL1MessagePasserJSON = l2ToL1MessagePasserJSON

	l2CrossDomainMessengerJSON, err := d.Cat("/hive/optimism/packages/contracts-bedrock/artifacts/contracts/L2/L2CrossDomainMessenger.sol/L2CrossDomainMessenger.json")
	if err != nil {
		return err
	}
	d.l2CrossDomainMessengerJSON = l2CrossDomainMessengerJSON

	optimismMintableTokenFactoryJSON, err := d.Cat("/hive/optimism/packages/contracts-bedrock/artifacts/contracts/universal/OptimismMintableTokenFactoryProxy.sol/OptimismMintableTokenFactoryProxy.json")
	if err != nil {
		return err
	}
	d.optimismMintableTokenFactoryJSON = optimismMintableTokenFactoryJSON

	l2StandardBridgeJSON, err := d.Cat("/hive/optimism/packages/contracts-bedrock/artifacts/contracts/L2/L2StandardBridge.sol/L2StandardBridge.json")
	if err != nil {
		return err
	}
	d.l2StandardBridgeJSON = l2StandardBridgeJSON

	l1BlockJSON, err := d.Cat("/hive/optimism/packages/contracts-bedrock/artifacts/contracts/L2/L1Block.sol/L1Block.json")
	if err != nil {
		return err
	}
	d.l1BlockJSON = l1BlockJSON

	return nil
}

func (d *Devnet) StartL2() error {
	l2 := d.nodes["op-l2"]

	executionOpts := hivesim.Params{
		"HIVE_CHECK_LIVE_PORT": "9545",
		"HIVE_LOGLEVEL":        os.Getenv("HIVE_LOGLEVEL"),
		"HIVE_NODETYPE":        "full",
		"HIVE_NETWORK_ID":      networkID.String(),
		"HIVE_CHAIN_ID":        chainID.String(),
	}

	genesisTimestampOpt := hivesim.WithDynamicFile("/genesis_timestamp", bytesSource([]byte(d.genesisTimestamp)))
	l2ToL1MessagePasserOpt := hivesim.WithDynamicFile("/L2ToL1MessagePasser.json", bytesSource([]byte(d.l2ToL1MessagePasserJSON)))
	l2CrossDomainMessengerOpt := hivesim.WithDynamicFile("/L2CrossDomainMessenger.json", bytesSource([]byte(d.l2CrossDomainMessengerJSON)))
	optimismMintableTokenFactoryOpt := hivesim.WithDynamicFile("/OptimismMintableTokenFactoryProxy.json", bytesSource([]byte(d.optimismMintableTokenFactoryJSON)))
	l2StandardBridgeOpt := hivesim.WithDynamicFile("/L2StandardBridge.json", bytesSource([]byte(d.l2StandardBridgeJSON)))
	l1BlockOpt := hivesim.WithDynamicFile("/L1Block.json", bytesSource([]byte(d.l1BlockJSON)))
	opts := []hivesim.StartOption{executionOpts, genesisTimestampOpt, l2ToL1MessagePasserOpt, l2CrossDomainMessengerOpt, optimismMintableTokenFactoryOpt, l2StandardBridgeOpt, l1BlockOpt}
	d.l2 = &L2Node{d.t.StartClient(l2.Name, opts...), 9545, 9546}
	return nil
}

func (d *Devnet) InitOp() error {
	execInfo, err := d.l2.Client.Exec("cat.sh", "/hive/genesis-l2.json")
	if err != nil {
		return err
	}
	d.l2Genesis = execInfo.Stdout
	return nil
}

func (d *Devnet) StartOp() error {
	op := d.nodes["op-node"]

	executionOpts := hivesim.Params{
		"HIVE_CHECK_LIVE_PORT":  "7545",
		"HIVE_CATALYST_ENABLED": "1",
		"HIVE_LOGLEVEL":         os.Getenv("HIVE_LOGLEVEL"),
		"HIVE_NODETYPE":         "full",

		"HIVE_L1_URL":             fmt.Sprintf("http://%s:%d", d.eth1.IP, d.eth1.HTTPPort),
		"HIVE_L2_URL":             fmt.Sprintf("http://%s:%d", d.l2.IP, d.l2.HTTPPort),
		"HIVE_L1_ETH_RPC_FLAG":    fmt.Sprintf("--l1=ws://%s:%d", d.eth1.IP, d.eth1.WSPort),
		"HIVE_L2_ENGINE_RPC_FLAG": fmt.Sprintf("--l2=ws://%s:%d", d.l2.IP, d.l2.WSPort),
	}

	if op.HasRole("op-sequencer") {
		executionOpts = executionOpts.Set("HIVE_SEQUENCER_ENABLED_FLAG", "--sequencer.enabled")
		executionOpts = executionOpts.Set("HIVE_SEQUENCER_KEY_FLAG", "--p2p.sequencer.key=/config/p2p-sequencer-key.txt")
	}

	optimismPortalOpt := hivesim.WithDynamicFile("/OptimismPortalProxy.json", bytesSource([]byte(d.optimismPortal)))
	opts := []hivesim.StartOption{executionOpts, optimismPortalOpt}
	d.op = &OpNode{d.t.StartClient(op.Name, opts...), 7545}
	return nil
}

func (d *Devnet) StartL2OS() error {
	l2os := d.nodes["op-proposer"]

	executionOpts := hivesim.Params{
		"HIVE_CHECK_LIVE_PORT":  "0",
		"HIVE_CATALYST_ENABLED": "1",
		"HIVE_LOGLEVEL":         os.Getenv("HIVE_LOGLEVEL"),
		"HIVE_NODETYPE":         "full",

		"HIVE_L1_ETH_RPC_FLAG": fmt.Sprintf("--l1-eth-rpc=http://%s:%d", d.eth1.IP, d.eth1.HTTPPort),
		"HIVE_L2_ETH_RPC_FLAG": fmt.Sprintf("--l2-eth-rpc=http://%s:%d", d.l2.IP, d.l2.HTTPPort),
		"HIVE_ROLLUP_RPC_FLAG": fmt.Sprintf("--rollup-rpc=http://%s:%d", d.op.IP, d.op.HTTPPort),
	}

	l2OutputOracleOpt := hivesim.WithDynamicFile("/L2OutputOracleProxy.json", bytesSource([]byte(d.l2OutputOracle)))
	opts := []hivesim.StartOption{executionOpts, l2OutputOracleOpt}
	d.l2os = &L2OSNode{d.t.StartClient(l2os.Name, opts...)}
	return nil
}

func (d *Devnet) StartBSS() error {
	bss := d.nodes["op-batcher"]

	executionOpts := hivesim.Params{
		"HIVE_CHECK_LIVE_PORT":  "0",
		"HIVE_CATALYST_ENABLED": "1",
		"HIVE_LOGLEVEL":         os.Getenv("HIVE_LOGLEVEL"),
		"HIVE_NODETYPE":         "full",

		"HIVE_L1_ETH_RPC_FLAG": fmt.Sprintf("--l1-eth-rpc=http://%s:%d", d.eth1.IP, d.eth1.HTTPPort),
		"HIVE_L2_ETH_RPC_FLAG": fmt.Sprintf("--l2-eth-rpc=http://%s:%d", d.l2.IP, d.l2.HTTPPort),
		"HIVE_ROLLUP_RPC_FLAG": fmt.Sprintf("--rollup-rpc=http://%s:%d", d.op.IP, d.op.HTTPPort),
	}

	optimismPortalOpt := hivesim.WithDynamicFile("/OptimismPortalProxy.json", bytesSource([]byte(d.optimismPortal)))
	opts := []hivesim.StartOption{executionOpts, optimismPortalOpt}
	d.bss = &BSSNode{d.t.StartClient(bss.Name, opts...)}
	return nil
}
