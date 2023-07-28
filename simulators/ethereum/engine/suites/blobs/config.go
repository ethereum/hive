package suite_blobs

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/hive/simulators/ethereum/engine/clmock"
	"github.com/ethereum/hive/simulators/ethereum/engine/globals"
)

// Timestamp delta between genesis and the withdrawals fork
func (bs *BlobsBaseSpec) GetCancunGenesisTimeDelta() uint64 {
	return bs.CancunForkHeight * bs.GetBlockTimeIncrements()
}

// Calculates Shanghai fork timestamp given the amount of blocks that need to be
// produced beforehand.
func (bs *BlobsBaseSpec) GetCancunForkTime() uint64 {
	return uint64(globals.GenesisTimestamp) + bs.GetCancunGenesisTimeDelta()
}

// Generates the fork config, including cancun fork timestamp.
func (bs *BlobsBaseSpec) GetForkConfig() globals.ForkConfig {
	return globals.ForkConfig{
		ShanghaiTimestamp: big.NewInt(0), // No test starts before Shanghai
		CancunTimestamp:   new(big.Int).SetUint64(bs.GetCancunForkTime()),
	}
}

// Get the per-block timestamp increments configured for this test
func (bs *BlobsBaseSpec) GetBlockTimeIncrements() uint64 {
	return 1
}

// Timestamp delta between genesis and the withdrawals fork
func (bs *BlobsBaseSpec) GetBlobsGenesisTimeDelta() uint64 {
	return bs.CancunForkHeight * bs.GetBlockTimeIncrements()
}

// Calculates Shanghai fork timestamp given the amount of blocks that need to be
// produced beforehand.
func (bs *BlobsBaseSpec) GetBlobsForkTime() uint64 {
	return uint64(globals.GenesisTimestamp) + bs.GetBlobsGenesisTimeDelta()
}

// Append the accounts we are going to withdraw to, which should also include
// bytecode for testing purposes.
func (bs *BlobsBaseSpec) GetGenesis() *core.Genesis {
	genesis := bs.Spec.GetGenesis()

	// Remove PoW altogether
	genesis.Difficulty = common.Big0
	genesis.Config.TerminalTotalDifficulty = common.Big0
	genesis.Config.Clique = nil
	genesis.ExtraData = []byte{}

	if bs.CancunForkHeight == 0 {
		genesis.BlobGasUsed = pUint64(0)
		genesis.ExcessBlobGas = pUint64(0)
		// TODO (DEVNET 8): Add parent beacon block
	}

	// Add accounts that use the DATAHASH opcode
	datahashCode := []byte{
		0x5F, // PUSH0
		0x80, // DUP1
		0x49, // DATAHASH
		0x55, // SSTORE
		0x60, // PUSH1(0x01)
		0x01,
		0x80, // DUP1
		0x49, // DATAHASH
		0x55, // SSTORE
		0x60, // PUSH1(0x02)
		0x02,
		0x80, // DUP1
		0x49, // DATAHASH
		0x55, // SSTORE
		0x60, // PUSH1(0x03)
		0x03,
		0x80, // DUP1
		0x49, // DATAHASH
		0x55, // SSTORE
	}

	for i := 0; i < DATAHASH_ADDRESS_COUNT; i++ {
		address := big.NewInt(0).Add(DATAHASH_START_ADDRESS, big.NewInt(int64(i)))
		genesis.Alloc[common.BigToAddress(address)] = core.GenesisAccount{
			Code:    datahashCode,
			Balance: common.Big0,
		}
	}

	return genesis
}

// Changes the CL Mocker default time increments of 1 to the value specified
// in the test spec.
func (bs *BlobsBaseSpec) ConfigureCLMock(cl *clmock.CLMocker) {
	cl.BlockTimestampIncrement = big.NewInt(int64(bs.GetBlockTimeIncrements()))
}
