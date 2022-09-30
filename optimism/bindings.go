package optimism

import (
	"github.com/ethereum-optimism/optimism/op-bindings/bindings"
	"github.com/ethereum-optimism/optimism/op-bindings/predeploys"
)

type BindingsL1 struct {
	OptimismPortal         *bindings.OptimismPortal
	L1CrossDomainMessenger *bindings.L1CrossDomainMessenger
	L1StandardBridge       *bindings.L1StandardBridge
}

func (d *Devnet) InitBindingsL1(eth1Index int) {
	cl := d.GetEth1(eth1Index).EthClient()

	portal, err := bindings.NewOptimismPortal(d.RollupCfg.DepositContractAddress, cl)
	if err != nil {
		d.T.Fatalf("failed optimism portal binding: %v", err)
		return
	}
	d.Bindings.BindingsL1.OptimismPortal = portal

	l1XDM, err := bindings.NewL1CrossDomainMessenger(predeploys.DevL1CrossDomainMessengerAddr, cl)
	if err != nil {
		d.T.Fatalf("failed optimism portal binding: %v", err)
		return
	}
	d.Bindings.BindingsL1.L1CrossDomainMessenger = l1XDM

	l1SB, err := bindings.NewL1StandardBridge(predeploys.DevL1StandardBridgeAddr, cl)
	if err != nil {
		d.T.Fatalf("failed optimism portal binding: %v", err)
		return
	}
	d.Bindings.BindingsL1.L1StandardBridge = l1SB
}

type BindingsL2 struct {
	L1Block                      *bindings.L1Block
	OptimismMintableERC20Factory *bindings.OptimismMintableERC20Factory
	L2CrossDomainMessenger       *bindings.L2CrossDomainMessenger
	L2StandardBridge             *bindings.L2StandardBridge
}

func (d *Devnet) InitBindingsL2(l2EngIndex int) {
	cl := d.GetOpL2Engine(l2EngIndex).EthClient()

	l1Block, err := bindings.NewL1Block(predeploys.L1BlockAddr, cl)
	if err != nil {
		d.T.Fatalf("failed optimism portal binding: %v", err)
		return
	}
	d.Bindings.BindingsL2.L1Block = l1Block

	factory, err := bindings.NewOptimismMintableERC20Factory(predeploys.OptimismMintableERC20FactoryAddr, cl)
	if err != nil {
		d.T.Fatalf("failed erc20 factory binding: %v", err)
		return
	}
	d.Bindings.BindingsL2.OptimismMintableERC20Factory = factory

	l2XDM, err := bindings.NewL2CrossDomainMessenger(predeploys.L2CrossDomainMessengerAddr, cl)
	if err != nil {
		d.T.Fatalf("failed l1XDM factory binding: %v", err)
		return
	}
	d.Bindings.BindingsL2.L2CrossDomainMessenger = l2XDM

	l2SB, err := bindings.NewL2StandardBridge(predeploys.L2StandardBridgeAddr, cl)
	if err != nil {
		d.T.Fatalf("failed l1XDM factory binding: %v", err)
		return
	}
	d.Bindings.BindingsL2.L2StandardBridge = l2SB
}

type Bindings struct {
	BindingsL1
	BindingsL2
}
