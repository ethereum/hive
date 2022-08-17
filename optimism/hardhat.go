package optimism

import (
	"encoding/json"
	"github.com/ethereum-optimism/optimism/op-node/rollup"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/hive/hivesim"
)

var (
	L2ChainID = 902
)

type HardhatDeployConfig struct {
	L1StartingBlockTag string `json:"l1StartingBlockTag"`
	L1ChainID          uint64 `json:"l1ChainID"`
	L2ChainID          uint64 `json:"l2ChainID"`
	L2BlockTime        uint64 `json:"l2BlockTime"`

	MaxSequencerDrift      uint64         `json:"maxSequencerDrift"`
	SequencerWindowSize    uint64         `json:"sequencerWindowSize"`
	ChannelTimeout         uint64         `json:"channelTimeout"`
	P2pSequencerAddress    common.Address `json:"p2pSequencerAddress"`
	OptimismL2FeeRecipient common.Address `json:"optimismL2FeeRecipient"`
	BatchInboxAddress      common.Address `json:"batchInboxAddress"`
	BatchSenderAddress     common.Address `json:"batchSenderAddress"`

	L2OutputOracleSubmissionInterval uint64         `json:"l2OutputOracleSubmissionInterval"`
	L2OutputOracleStartingTimestamp  int            `json:"l2OutputOracleStartingTimestamp"`
	L2OutputOracleProposer           common.Address `json:"l2OutputOracleProposer"`
	L2OutputOracleOwner              common.Address `json:"l2OutputOracleOwner"`

	L1BlockTime                 uint64         `json:"l1BlockTime"`
	L1GenesisBlockNonce         hexutil.Uint64 `json:"l1GenesisBlockNonce"`
	CliqueSignerAddress         common.Address `json:"cliqueSignerAddress"`
	L1GenesisBlockGasLimit      hexutil.Uint64 `json:"l1GenesisBlockGasLimit"`
	L1GenesisBlockDifficulty    hexutil.Uint64 `json:"l1GenesisBlockDifficulty"`
	L1GenesisBlockMixHash       common.Hash    `json:"l1GenesisBlockMixHash"`
	L1GenesisBlockCoinbase      common.Address `json:"l1GenesisBlockCoinbase"`
	L1GenesisBlockNumber        hexutil.Uint64 `json:"l1GenesisBlockNumber"`
	L1GenesisBlockGasUsed       hexutil.Uint64 `json:"l1GenesisBlockGasUsed"`
	L1GenesisBlockParentHash    common.Hash    `json:"l1GenesisBlockParentHash"`
	L1GenesisBlockBaseFeePerGas hexutil.Uint64 `json:"l1GenesisBlockBaseFeePerGas"`

	L2GenesisBlockNonce         hexutil.Uint64 `json:"l2GenesisBlockNonce"`
	L2GenesisBlockExtraData     hexutil.Bytes  `json:"l2GenesisBlockExtraData"`
	L2GenesisBlockGasLimit      hexutil.Uint64 `json:"l2GenesisBlockGasLimit"`
	L2GenesisBlockDifficulty    hexutil.Uint64 `json:"l2GenesisBlockDifficulty"`
	L2GenesisBlockMixHash       common.Hash    `json:"l2GenesisBlockMixHash"`
	L2GenesisBlockCoinbase      common.Address `json:"l2GenesisBlockCoinbase"`
	L2GenesisBlockNumber        hexutil.Uint64 `json:"l2GenesisBlockNumber"`
	L2GenesisBlockGasUsed       hexutil.Uint64 `json:"l2GenesisBlockGasUsed"`
	L2GenesisBlockParentHash    common.Hash    `json:"l2GenesisBlockParentHash"`
	L2GenesisBlockBaseFeePerGas hexutil.Uint64 `json:"l2GenesisBlockBaseFeePerGas"`

	OptimismBaseFeeRecipient    common.Address `json:"optimismBaseFeeRecipient"`
	OptimismL1FeeRecipient      common.Address `json:"optimismL1FeeRecipient"`
	L2CrossDomainMessengerOwner common.Address `json:"l2CrossDomainMessengerOwner"`
	GasPriceOracleOwner         common.Address `json:"gasPriceOracleOwner"`
	GasPriceOracleOverhead      uint64         `json:"gasPriceOracleOverhead"`
	GasPriceOracleScalar        uint64         `json:"gasPriceOracleScalar"`
	GasPriceOracleDecimals      uint64         `json:"gasPriceOracleDecimals"`

	ProxyAdmin                  common.Address `json:"proxyAdmin"`
	FundDevAccounts             bool           `json:"fundDevAccounts"`
	DeploymentWaitConfirmations uint64         `json:"deploymentWaitConfirmations"`
}

func (d *Devnet) RunScript(name string, command ...string) *hivesim.ExecInfo {
	execInfo, err := d.Contracts.Client.Exec(command...)
	if err != nil {
		if execInfo != nil {
			d.T.Log(execInfo.Stdout)
			d.T.Error(execInfo.Stderr)
		}
		d.T.Fatalf("failed to run %s: %v", name, err)
		return nil
	}
	if execInfo.ExitCode != 0 {
		d.T.Log(execInfo.Stdout)
		d.T.Error(execInfo.Stderr)
		d.T.Fatalf("script %s exit code non-zero: %d", name, execInfo.ExitCode)
		return nil
	}
	return execInfo
}

func (d *Devnet) InitHardhatDeployConfig(l1StartingBlockTag string, maxSeqDrift uint64, seqWindowSize uint64, chanTimeout uint64) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.T.Log("creating hardhat deploy config")

	deployConf := HardhatDeployConfig{
		L1StartingBlockTag: l1StartingBlockTag,
		L1ChainID:          901,
		L2ChainID:          uint64(L2ChainID),
		L2BlockTime:        2,

		MaxSequencerDrift:      maxSeqDrift,
		SequencerWindowSize:    seqWindowSize,
		ChannelTimeout:         chanTimeout,
		P2pSequencerAddress:    d.Addresses.SequencerP2P,
		OptimismL2FeeRecipient: common.Address{0: 0x42, 19: 0xf0}, // tbd
		BatchInboxAddress:      common.Address{0: 0x42, 19: 0xff}, // tbd
		BatchSenderAddress:     d.Addresses.Batcher,

		L2OutputOracleSubmissionInterval: 6,
		L2OutputOracleStartingTimestamp:  -1,
		L2OutputOracleProposer:           d.Addresses.Proposer,
		L2OutputOracleOwner:              common.Address{}, // tbd

		L1BlockTime:                 15,
		L1GenesisBlockNonce:         0,
		CliqueSignerAddress:         d.Addresses.CliqueSigner,
		L1GenesisBlockGasLimit:      15_000_000,
		L1GenesisBlockDifficulty:    1,
		L1GenesisBlockMixHash:       common.Hash{},
		L1GenesisBlockCoinbase:      common.Address{},
		L1GenesisBlockNumber:        0,
		L1GenesisBlockGasUsed:       0,
		L1GenesisBlockParentHash:    common.Hash{},
		L1GenesisBlockBaseFeePerGas: 1000_000_000, // 1 gwei

		L2GenesisBlockNonce:         0,
		L2GenesisBlockExtraData:     []byte{},
		L2GenesisBlockGasLimit:      15_000_000,
		L2GenesisBlockDifficulty:    1,
		L2GenesisBlockMixHash:       common.Hash{},
		L2GenesisBlockCoinbase:      common.Address{0: 0x42, 19: 0xf0}, // matching OptimismL2FeeRecipient
		L2GenesisBlockNumber:        0,
		L2GenesisBlockGasUsed:       0,
		L2GenesisBlockParentHash:    common.Hash{},
		L2GenesisBlockBaseFeePerGas: 1000_000_000,

		OptimismBaseFeeRecipient:    common.Address{0: 0x42, 19: 0xf1}, // tbd
		OptimismL1FeeRecipient:      d.Addresses.Batcher,
		L2CrossDomainMessengerOwner: common.Address{0: 0x42, 19: 0xf2}, // tbd
		GasPriceOracleOwner:         common.Address{0: 0x42, 19: 0xf3}, // tbd
		GasPriceOracleOverhead:      2100,
		GasPriceOracleScalar:        1000_000,
		GasPriceOracleDecimals:      6,

		ProxyAdmin:                  common.Address{0: 0x42, 19: 0xf4}, // tbd
		FundDevAccounts:             true,
		DeploymentWaitConfirmations: 1,
	}

	deployConfBytes, err := json.Marshal(&deployConf)
	if err != nil {
		d.T.Fatalf("failed to marshal hardhat deploy config: %v", err)
		return
	}

	_ = d.RunScript("deploy config", "deploy_config.sh", string(deployConfBytes))
	d.T.Log("created hardhat deploy config")
}

// InitRollupHardhat creates a deploy-config and the initial L1 configuration using a hardhat task.
// After the L1 chain is running DeployL1Hardhat will need to run to deploy the L1 contracts.
// This depends on InitContracts()
func (d *Devnet) InitL1Hardhat() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.T.Log("creating L1 genesis with hardhat")

	if d.L1Cfg != nil {
		d.T.Fatalf("already initialized L1 chain config: %v", d.L1Cfg)
		return
	}
	execInfo := d.RunScript("L1 config", "config_l1.sh")

	var l1Cfg core.Genesis
	if err := json.Unmarshal([]byte(execInfo.Stdout), &l1Cfg); err != nil {
		d.T.Fatalf("failed to decode hardhat L1 chain config")
	}
	d.L1Cfg = &l1Cfg
	d.T.Log("created L1 genesis with hardhat")
	return
}

func (d *Devnet) InitL2Hardhat(additionalAlloc core.GenesisAlloc) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.T.Log("creating L2 genesis with hardhat")

	if d.L2Cfg != nil {
		d.T.Fatalf("already initialized L2 chain config: %v", d.L2Cfg)
		return
	}
	if len(d.Eth1s) == 0 {
		d.T.Fatal("no L1 eth1 node to fetch L1 info from for L2 config")
	}

	execInfo := d.RunScript("L2 config", "config_l2.sh",
		d.Eth1s[0].HttpRpcEndpoint(), EncodePrivKey(d.Secrets.Deployer).String()[2:], d.L1Cfg.Config.ChainID.String())

	var l2Cfg core.Genesis
	if err := json.Unmarshal([]byte(execInfo.Stdout), &l2Cfg); err != nil {
		d.T.Fatalf("failed to decode hardhat L2 chain config")
	}

	if additionalAlloc != nil {
		for k, v := range additionalAlloc {
			l2Cfg.Alloc[k] = v
		}
	}

	d.L2Cfg = &l2Cfg
	d.T.Log("created L2 genesis with hardhat")
	return
}

func (d *Devnet) InitRollupHardhat() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.T.Log("creating rollup config with hardhat")

	if d.RollupCfg != nil {
		d.T.Fatalf("already initialized rollup config: %v", d.RollupCfg)
		return
	}
	if len(d.Eth1s) == 0 {
		d.T.Fatal("no L1 eth1 node to fetch L1 info from for rollup config")
	}
	if len(d.OpL2Engines) == 0 {
		d.T.Fatal("no L2 engine node to fetch L2 info from for rollup config")
	}

	execInfo := d.RunScript("rollup config",
		"config_rollup.sh", d.Eth1s[0].HttpRpcEndpoint(), d.OpL2Engines[0].HttpRpcEndpoint(),
		EncodePrivKey(d.Secrets.Deployer).String()[2:], d.L1Cfg.Config.ChainID.String())

	var rollupCfg rollup.Config
	if err := json.Unmarshal([]byte(execInfo.Stdout), &rollupCfg); err != nil {
		d.T.Fatalf("failed to decode hardhat rollup config")
	}
	d.RollupCfg = &rollupCfg
	d.T.Log("created rollup config with hardhat")
	return
}

// DeployL1Hardhat runs the deploy script for L1 contracts, and initializes the L1 contract bindings.
// Run InitContracts first.
func (d *Devnet) DeployL1Hardhat() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.T.Log("deploying L1 contracts with hardhat")

	if d.Contracts == nil {
		d.T.Fatal("op contracts client needs to be initialized first")
		return
	}
	if len(d.Eth1s) == 0 {
		d.T.Fatal("no L1 eth1 node to deploy contracts to")
	}

	execInfo := d.RunScript("deploy to L1", "deploy_l1.sh",
		d.Eth1s[0].HttpRpcEndpoint(), EncodePrivKey(d.Secrets.Deployer).String()[2:], d.L1Cfg.Config.ChainID.String())
	d.T.Log("deployed contracts", execInfo.Stdout, execInfo.Stderr)

	execInfo = d.RunScript("deployments", "deployments.sh")
	var hhDeployments HardhatDeploymentsL1
	if err := json.Unmarshal([]byte(execInfo.Stdout), &hhDeployments); err != nil {
		d.T.Fatalf("failed to decode hardhat deployments")
	}
	d.Deployments.DeploymentsL1 = DeploymentsL1{
		L1CrossDomainMessengerProxy: hhDeployments.L1CrossDomainMessengerProxy.Address,
		L1StandardBridgeProxy:       hhDeployments.L1StandardBridgeProxy.Address,
		L2OutputOracleProxy:         hhDeployments.L2OutputOracleProxy.Address,
		OptimismPortalProxy:         hhDeployments.OptimismPortalProxy.Address,
	}
	d.T.Log("deployed L1 contracts with hardhat")
	return
}
