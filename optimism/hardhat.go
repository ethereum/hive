package optimism

import (
	"encoding/json"
	"github.com/ethereum-optimism/optimism/op-node/rollup"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/params"
)

type HardhatDeployConfig struct {
	SubmissionInterval           uint64         `json:"submissionInterval"`
	GenesisOutput                common.Hash    `json:"genesisOutput"`
	HistoricalBlocks             uint64         `json:"historicalBlocks"`
	StartingBlockNumber          uint64         `json:"startingBlockNumber"`
	L2BlockTime                  uint64         `json:"l2BlockTime"`
	L1StartingBlockTag           string         `json:"l1StartingBlockTag"`
	SequencerAddress             common.Address `json:"sequencerAddress"`
	L2CrossDomainMessengerOwner  common.Address `json:"l2CrossDomainMessengerOwner"`
	GasPriceOracleOwner          common.Address `json:"gasPriceOracleOwner"`
	GasPriceOracleOverhead       uint64         `json:"gasPriceOracleOverhead"`
	GasPriceOracleScalar         uint64         `json:"gasPriceOracleScalar"`
	GasPriceOracleDecimals       uint64         `json:"gasPriceOracleDecimals"`
	L1BlockInitialNumber         uint64         `json:"l1BlockInitialNumber"`
	L1BlockInitialTimestamp      uint64         `json:"l1BlockInitialTimestamp"`
	L1BlockInitialBasefee        uint64         `json:"l1BlockInitialBasefee"`
	L1BlockInitialHash           common.Hash    `json:"l1BlockInitialHash"`
	L1BlockInitialSequenceNumber uint64         `json:"l1BlockInitialSequenceNumber"`
	GenesisBlockExtradata        hexutil.Bytes  `json:"genesisBlockExtradata"`
	GenesisBlockGasLimit         hexutil.Uint64 `json:"genesisBlockGasLimit"`
	GenesisBlockChainid          uint64         `json:"genesisBlockChainid"`
	FundDevAccounts              bool           `json:"fundDevAccounts"`
	P2pSequencerAddress          common.Address `json:"p2pSequencerAddress"`
	DeploymentWaitConfirmations  uint64         `json:"deploymentWaitConfirmations"`
	MaxSequencerDrift            uint64         `json:"maxSequencerDrift"`
	SequencerWindowSize          uint64         `json:"sequencerWindowSize"`
	ChannelTimeout               uint64         `json:"channelTimeout"`
	ProxyAdmin                   common.Address `json:"proxyAdmin"`
	OptimismBaseFeeRecipient     common.Address `json:"optimismBaseFeeRecipient"`
	OptimismL1FeeRecipient       common.Address `json:"optimismL1FeeRecipient"`
	OptimismL2FeeRecipient       common.Address `json:"optimismL2FeeRecipient"`
	OutputOracleOwner            common.Address `json:"outputOracleOwner"`
	BatchSenderAddress           common.Address `json:"batchSenderAddress"`
}

type HardhatGeneratedConfigs struct {
	L1     *params.ChainConfig `json:"l1"`
	L2     *params.ChainConfig `json:"l2"`
	Rollup *rollup.Config      `json:"rollup"`
}

// InitRollupHardhat creates a L1, L2 and rollup configuration using a hardhat task.
// After the L1 chain is running DeployL1Hardhat will need to run to deploy the L1 contracts.
// This depends on InitContracts()
func (d *Devnet) InitRollupHardhat() {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.L1Cfg != nil {
		d.T.Fatalf("already initialized L1 chain config: %v", d.L1Cfg)
		return
	}
	var extraDataL1 hexutil.Bytes
	extraDataL1 = append(extraDataL1, make([]byte, 32)...)
	extraDataL1 = append(extraDataL1, d.Addresses.CliqueSigner[:]...)
	extraDataL1 = append(extraDataL1, make([]byte, 65)...)

	deployConf := HardhatDeployConfig{
		SubmissionInterval:          6,
		GenesisOutput:               common.Hash{},
		HistoricalBlocks:            0,
		StartingBlockNumber:         0,
		L2BlockTime:                 2,
		L1StartingBlockTag:          "earliest",
		SequencerAddress:            d.Addresses.CliqueSigner,
		L2CrossDomainMessengerOwner: common.Address{},
		GasPriceOracleOwner:         common.Address{},
		GasPriceOracleOverhead:      2100,
		GasPriceOracleScalar:        1000000,
		GasPriceOracleDecimals:      6,

		L1BlockInitialNumber:         0,
		L1BlockInitialTimestamp:      0,
		L1BlockInitialBasefee:        10,
		L1BlockInitialHash:           common.Hash{},
		L1BlockInitialSequenceNumber: 0,

		GenesisBlockExtradata: extraDataL1,

		GenesisBlockGasLimit:        15000000,
		GenesisBlockChainid:         111,
		FundDevAccounts:             true,
		P2pSequencerAddress:         d.Addresses.SequencerP2P,
		DeploymentWaitConfirmations: 1,
		MaxSequencerDrift:           100,
		SequencerWindowSize:         4,
		ChannelTimeout:              40,
		ProxyAdmin:                  d.Addresses.Deployer,
		OptimismBaseFeeRecipient:    common.Address{0: 0x42, 19: 0xf1}, // tbd
		OptimismL1FeeRecipient:      d.Addresses.Batcher,
		OptimismL2FeeRecipient:      common.Address{0: 0x42, 19: 0xf0}, // tbd
		OutputOracleOwner:           common.Address{},                  // tbd
		BatchSenderAddress:          d.Addresses.Batcher,
	}

	deployConfBytes, err := json.Marshal(&deployConf)
	if err != nil {
		d.T.Fatalf("failed to marshal hardhat deploy config: %v", err)
		return
	}

	execInfo, err := d.Contracts.Client.Exec("configs.sh", string(deployConfBytes))
	if err != nil {
		if execInfo != nil {
			d.T.Log(execInfo.Stdout)
			d.T.Error(execInfo.Stderr)
		}
		d.T.Fatalf("failed to create rollup configs: %v", err)
		return
	}
	if execInfo.ExitCode != 0 {
		d.T.Log(execInfo.Stdout)
		d.T.Error(execInfo.Stderr)
		d.T.Fatalf("rollup configs script exit code non-zero: %d", execInfo.ExitCode)
		return
	}
	var hhConfigs HardhatGeneratedConfigs
	if err := json.Unmarshal([]byte(execInfo.Stdout), &hhConfigs); err != nil {
		d.T.Fatalf("failed to decode hardhat chain configs")
	}
	d.L1Cfg = hhConfigs.L1
	d.L2Cfg = hhConfigs.L2
	d.RollupCfg = hhConfigs.Rollup
	return
}

// DeployL1Hardhat runs the deploy script for L1 contracts, and initializes the L1 contract bindings.
// Run InitContracts first.
func (d *Devnet) DeployL1Hardhat() {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.Contracts == nil {
		d.T.Fatal("op contracts client needs to be initialized first")
		return
	}

	execInfo, err := d.Contracts.Client.Exec("deploy_l1.sh")
	if err != nil {
		if execInfo != nil {
			d.T.Log(execInfo.Stdout)
			d.T.Error(execInfo.Stderr)
		}
		d.T.Fatalf("failed to deploy L1 contracts: %v", err)
		return
	}
	d.T.Log(execInfo.Stdout)
	if execInfo.ExitCode != 0 {
		d.T.Error(execInfo.Stderr)
		d.T.Fatalf("contract deployment exit code non-zero: %d", execInfo.ExitCode)
		return
	}

	execInfo, err = d.Contracts.Client.Exec("deployments.sh")
	if err != nil {
		if execInfo != nil {
			d.T.Log(execInfo.Stdout)
			d.T.Error(execInfo.Stderr)
		}
		d.T.Fatalf("failed to get deployments data: %v", err)
		return
	}
	if execInfo.ExitCode != 0 {
		d.T.Error(execInfo.Stderr)
		d.T.Fatalf("deployments data exit code non-zero: %d", execInfo.ExitCode)
		return
	}
	var hhDeployments HardhatDeploymentsL1
	if err := json.Unmarshal([]byte(execInfo.Stdout), &hhDeployments); err != nil {
		d.T.Fatalf("failed to decode hardhat deployments")
	}
	d.Deployments.DeploymentsL1 = DeploymentsL1{
		L1CrossDomainMessengerProxy: hhDeployments.L1CrossDomainMessengerProxy.Address,
		L1StandardBridgeProxy:       hhDeployments.L1StandardBridgeProxy.Address,
		L2OutputOracleProxy:         hhDeployments.L1StandardBridgeProxy.Address,
		OptimismPortalProxy:         hhDeployments.L1StandardBridgeProxy.Address,
	}
	return
}
