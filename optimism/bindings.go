package optimism

import (
	"github.com/ethereum-optimism/optimism/op-bindings/bindings"
	"github.com/ethereum-optimism/optimism/op-bindings/predeploys"
)

type BindingsL1 struct {
	OptimismPortal *bindings.OptimismPortal
}

func (d *Devnet) InitBindingsL1(eth1Index int) {
	cl := d.GetEth1(eth1Index).EthClient()

	contract, err := bindings.NewOptimismPortal(d.RollupCfg.DepositContractAddress, cl)
	if err != nil {
		d.T.Fatalf("failed optimism portal binding: %v", err)
		return
	}
	d.Bindings.BindingsL1.OptimismPortal = contract
}

type BindingsL2 struct {
	L1Block *bindings.L1Block
}

func (d *Devnet) InitBindingsL2(l2EngIndex int) {
	cl := d.GetOpL2Engine(l2EngIndex).EthClient()

	contract, err := bindings.NewL1Block(predeploys.L1BlockAddr, cl)
	if err != nil {
		d.T.Fatalf("failed optimism portal binding: %v", err)
		return
	}
	d.Bindings.BindingsL2.L1Block = contract
}

type Bindings struct {
	BindingsL1
	BindingsL2
}
