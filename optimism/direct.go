package optimism

// This file is an alternative to compiling contracts, generating configs, running L1, deploying etc.
// Configs are generated here in the Go process, and contracts are predeployed into the right locations.

// InitRollupDirectGo initializes a L1, L2 and rollup configuration, purely in Go,
// premining the contracts into the genesis for later usage.
// As opposed to InitL1ChainHardhat which does a full hardhat deployment on a running chain.
func (d *Devnet) InitRollupDirectGo() {
	// TODO
	return
}
