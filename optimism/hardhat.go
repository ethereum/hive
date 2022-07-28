package optimism

import (
	"encoding/json"
	"fmt"
	"github.com/ethereum-optimism/optimism/op-node/rollup"
	"github.com/ethereum/go-ethereum/params"
	"time"
)

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

	// TODO: better way to configure testnet timestamp
	execInfo, err := d.Contracts.Client.Exec("configs.sh", fmt.Sprintf("%d", time.Now().Unix()+30))
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
