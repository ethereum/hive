package cancun

import (
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/types"
)

// ConfigGenesis configures the genesis block for the Cancun fork.
func ConfigGenesis(genesis *core.Genesis, forkTimestamp uint64) error {
	if genesis.Config.ShanghaiTime == nil {
		return fmt.Errorf("cancun fork requires shanghai fork")
	}
	genesis.Config.CancunTime = &forkTimestamp
	if *genesis.Config.ShanghaiTime > forkTimestamp {
		return fmt.Errorf("cancun fork must be after shanghai fork")
	}
	if genesis.Timestamp >= forkTimestamp {
		if genesis.BlobGasUsed == nil {
			genesis.BlobGasUsed = new(uint64)
		}
		if genesis.ExcessBlobGas == nil {
			genesis.ExcessBlobGas = new(uint64)
		}
	}

	// Add bytecode pre deploy to the EIP-4788 address.
	genesis.Alloc[BEACON_ROOTS_ADDRESS] = core.GenesisAccount{
		Balance: common.Big0,
		Nonce:   1,
		Code:    common.Hex2Bytes("3373fffffffffffffffffffffffffffffffffffffffe14604d57602036146024575f5ffd5b5f35801560495762001fff810690815414603c575f5ffd5b62001fff01545f5260205ff35b5f5ffd5b62001fff42064281555f359062001fff015500"),
	}

	return nil
}

// Configure specific test genesis accounts related to Cancun funtionality.
func ConfigTestAccounts(genesis *core.Genesis) error {
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
		address := common.BigToAddress(big.NewInt(0).Add(DATAHASH_START_ADDRESS, big.NewInt(int64(i))))
		// check first if the address is already in the genesis
		if _, ok := genesis.Alloc[address]; ok {
			panic(fmt.Errorf("reused address %s during genesis configuration for cancun", address.Hex()))
		}
		genesis.Alloc[address] = types.Account{
			Code:    datahashCode,
			Balance: common.Big0,
		}
	}

	return nil
}
