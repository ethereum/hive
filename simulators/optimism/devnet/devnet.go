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

	genesisTimestamp   string
	withdrawerBytecode string
	l1BlockBytecode    string
	l2Genesis string

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
	var eth1, l2, op, l2os *hivesim.ClientDefinition
	for _, client := range clientTypes {
		if client.HasRole("ops-l1") {
			eth1 = client
		}
		if client.HasRole("ops-l2") {
			l2 = client
		}
		if client.HasRole("ops-opnode") {
			op = client
		}
		if client.HasRole("ops-l2os") {
			l2os = client
		}
	}

	if eth1 == nil || l2 == nil || op == nil || l2os == nil {
		d.t.Fatal("ops-l1, ops-l2, ops-opnode, ops-l2os required")
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
	d.eth1 = &Eth1Node{d.t.StartClient(eth1.Name, opts...)}

	d.nodes["ops-l1"] = eth1
	d.nodes["ops-l2"] = l2
	d.nodes["ops-opnode"] = op
	d.nodes["ops-l2os"] = l2os
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

	l2OutputOracle, err := d.Cat("/hive/contracts/deployments/devnetL1/L2OutputOracle.json")
	if err != nil {
		return err
	}
	optimismPortal, err := d.Cat("/hive/contracts/deployments/devnetL1/OptimismPortal.json")
	if err != nil {
		return err
	}
	d.l2OutputOracle = l2OutputOracle
	d.optimismPortal = optimismPortal

	withdrawerBytecode, err := d.Cat("/hive/contracts/artifacts/contracts/L2/Withdrawer.sol/Withdrawer.json")
	if err != nil {
		return err
	}
	l1BlockBytecode, err := d.Cat("/hive/contracts/artifacts/contracts/L2/L1Block.sol/L1Block.json")
	if err != nil {
		return err
	}
	d.withdrawerBytecode = withdrawerBytecode
	d.l1BlockBytecode = l1BlockBytecode

	return nil
}

func (d *Devnet) StartL2() error {
	l2 := d.nodes["ops-l2"]

	executionOpts := hivesim.Params{
		"HIVE_CHECK_LIVE_PORT":     "9545",
		"HIVE_LOGLEVEL":            os.Getenv("HIVE_LOGLEVEL"),
		"HIVE_NODETYPE":            "full",
		"HIVE_NETWORK_ID":          networkID.String(),
		"HIVE_CHAIN_ID":            chainID.String(),
	}

	genesisTimestampOpt := hivesim.WithDynamicFile("/genesis_timestamp", bytesSource([]byte(d.genesisTimestamp)))
	withdrawerBytecodeOpt := hivesim.WithDynamicFile("/Withdrawer.json", bytesSource([]byte(d.withdrawerBytecode)))
	l1BlockBytecodeOpt := hivesim.WithDynamicFile("/L1Block.json", bytesSource([]byte(d.l1BlockBytecode)))
	opts := []hivesim.StartOption{executionOpts, genesisTimestampOpt, withdrawerBytecodeOpt, l1BlockBytecodeOpt}
	d.l2 = &L2Node{d.t.StartClient(l2.Name, opts...)}
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
	op := d.nodes["ops-opnode"]

	executionOpts := hivesim.Params{
		"HIVE_CHECK_LIVE_PORT":  "7545",
		"HIVE_CATALYST_ENABLED": "1",
		"HIVE_LOGLEVEL":         os.Getenv("HIVE_LOGLEVEL"),
		"HIVE_NODETYPE":         "full",
	}

	optimismPortalOpt := hivesim.WithDynamicFile("/OptimismPortal.json", bytesSource([]byte(d.optimismPortal)))
	opts := []hivesim.StartOption{executionOpts, optimismPortalOpt}
	d.op = &OpNode{d.t.StartClient(op.Name, opts...)}
	return nil
}

func (d *Devnet) StartL2OS() error {
	op := d.nodes["ops-l2os"]

	executionOpts := hivesim.Params{
		"HIVE_CHECK_LIVE_PORT":  "0",
		"HIVE_CATALYST_ENABLED": "1",
		"HIVE_LOGLEVEL":         os.Getenv("HIVE_LOGLEVEL"),
		"HIVE_NODETYPE":         "full",
	}

	l2OutputOracleOpt := hivesim.WithDynamicFile("/L2OutputOracle.json", bytesSource([]byte(d.l2OutputOracle)))
	opts := []hivesim.StartOption{executionOpts, l2OutputOracleOpt}
	d.op = &OpNode{d.t.StartClient(op.Name, opts...)}
	return nil
}
