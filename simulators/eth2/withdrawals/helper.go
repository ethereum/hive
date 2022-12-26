package main

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/hive/simulators/eth2/common/clients"
	consensus_config "github.com/ethereum/hive/simulators/eth2/common/config/consensus"
	"github.com/ethereum/hive/simulators/eth2/common/testnet"
	beacon "github.com/protolambda/zrnt/eth2/beacon/common"
)

// API call names
const (
	EngineForkchoiceUpdatedV1 = "engine_forkchoiceUpdatedV1"
	EngineGetPayloadV1        = "engine_getPayloadV1"
	EngineNewPayloadV1        = "engine_newPayloadV1"
	EthGetBlockByHash         = "eth_getBlockByHash"
	EthGetBlockByNumber       = "eth_getBlockByNumber"
)

// Engine API Types

type PayloadStatus string

const (
	Unknown          = ""
	Valid            = "VALID"
	Invalid          = "INVALID"
	Accepted         = "ACCEPTED"
	Syncing          = "SYNCING"
	InvalidBlockHash = "INVALID_BLOCK_HASH"
)

// Signer for all txs
type Signer struct {
	ChainID    *big.Int
	PrivateKey *ecdsa.PrivateKey
}

func (vs Signer) SignTx(
	baseTx *types.Transaction,
) (*types.Transaction, error) {
	signer := types.NewEIP155Signer(vs.ChainID)
	return types.SignTx(baseTx, signer, vs.PrivateKey)
}

var VaultSigner = Signer{
	ChainID:    CHAIN_ID,
	PrivateKey: VAULT_KEY,
}

func CheckCorrectWithdrawalBalances(
	ctx context.Context,
	ec *clients.ExecutionClient,
	kd []*consensus_config.KeyDetails,
) error {
	// At the moment, only check that the balances are greater than 1 Gwei
	for index := range kd {
		execAddress := common.Address{byte(index + 0x100)}
		balance, err := ec.BalanceAt(ctx, execAddress, nil)
		if err != nil {
			return fmt.Errorf(
				"unable to fetch account (%s) balance: %v",
				execAddress,
				err,
			)
		}
		if balance.Cmp(common.Big0) <= 0 {
			return fmt.Errorf(
				"FAIL: Account (%s) did not withdraw",
				execAddress,
			)
		}
		// Balance cannot be a value that is not a multiple of Gwei
		balanceGwei := new(big.Int).Div(balance, big.NewInt(1e9))
		if balance.Cmp(new(big.Int).Mul(balanceGwei, big.NewInt(1e9))) != 0 {
			return fmt.Errorf(
				"FAIL: Account (%s) withdrew a value that is not gwei: %d",
				execAddress, balance,
			)
		}
	}
	return nil
}

func ComputeBLSToExecutionDomain(
	t *testnet.Testnet,
) beacon.BLSDomain {
	return beacon.ComputeDomain(
		beacon.DOMAIN_BLS_TO_EXECUTION_CHANGE,
		t.Spec().CAPELLA_FORK_VERSION,
		t.GenesisValidatorsRoot(),
	)
}
