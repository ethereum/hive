package client

import (
	"context"
	"math/big"

	api "github.com/ethereum/go-ethereum/core/beacon"
	"github.com/ethereum/hive/hivesim"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

type Eth interface {
	BalanceAt(ctx context.Context, account common.Address, blockNumber *big.Int) (*big.Int, error)
	BlockByNumber(ctx context.Context, number *big.Int) (*types.Block, error)
	BlockNumber(ctx context.Context) (uint64, error)
	BlockByHash(ctx context.Context, hash common.Hash) (*types.Block, error)
	HeaderByNumber(ctx context.Context, number *big.Int) (*types.Header, error)
	SendTransaction(ctx context.Context, tx *types.Transaction) error
	StorageAt(ctx context.Context, account common.Address, key common.Hash, blockNumber *big.Int) ([]byte, error)
	TransactionReceipt(ctx context.Context, txHash common.Hash) (*types.Receipt, error)
	NonceAt(ctx context.Context, account common.Address, blockNumber *big.Int) (uint64, error)
}

type Engine interface {
	ForkchoiceUpdatedV1(ctx context.Context, fcState *api.ForkchoiceStateV1, pAttributes *api.PayloadAttributesV1) (api.ForkChoiceResponse, error)
	GetPayloadV1(ctx context.Context, payloadId *api.PayloadID) (api.ExecutableDataV1, error)
	NewPayloadV1(ctx context.Context, payload *api.ExecutableDataV1) (api.PayloadStatusV1, error)
}

type EngineClient interface {
	// General Methods
	ID() string
	Close() error
	EnodeURL() (string, error)

	// Local Test Account Management
	GetNextAccountNonce(testCtx context.Context, account common.Address) (uint64, error)

	// TTD Methods
	TerminalTotalDifficulty() *big.Int
	GetTotalDifficulty(context.Context) (*big.Int, error)

	// Engine, Eth Interfaces
	Engine
	Eth
}

type EngineStarter interface {
	StartClient(T *hivesim.T, testContext context.Context, ClientParams hivesim.Params, ClientFiles hivesim.Params, bootClient EngineClient) (EngineClient, error)
}

var (
	Head      *big.Int // Nil
	Pending   = big.NewInt(-2)
	Finalized = big.NewInt(-3)
	Safe      = big.NewInt(-4)
)
