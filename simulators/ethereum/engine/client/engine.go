package client

import (
	"context"
	"math/big"

	"github.com/ethereum/go-ethereum/core"
	api "github.com/ethereum/go-ethereum/beacon/engine"
	"github.com/ethereum/hive/hivesim"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"

	client_types "github.com/ethereum/hive/simulators/ethereum/engine/client/types"
)

type Eth interface {
	BalanceAt(ctx context.Context, account common.Address, blockNumber *big.Int) (*big.Int, error)
	BlockByNumber(ctx context.Context, number *big.Int) (*types.Block, error)
	BlockNumber(ctx context.Context) (uint64, error)
	BlockByHash(ctx context.Context, hash common.Hash) (*types.Block, error)
	HeaderByNumber(ctx context.Context, number *big.Int) (*types.Header, error)
	SendTransaction(ctx context.Context, tx *types.Transaction) error
	SendTransactions(ctx context.Context, txs []*types.Transaction) []error
	StorageAt(ctx context.Context, account common.Address, key common.Hash, blockNumber *big.Int) ([]byte, error)
	TransactionReceipt(ctx context.Context, txHash common.Hash) (*types.Receipt, error)
	NonceAt(ctx context.Context, account common.Address, blockNumber *big.Int) (uint64, error)
	TransactionByHash(ctx context.Context, hash common.Hash) (tx *types.Transaction, isPending bool, err error)
}

type Engine interface {
	ForkchoiceUpdatedV1(ctx context.Context, fcState *api.ForkchoiceStateV1, pAttributes *api.PayloadAttributes) (api.ForkChoiceResponse, error)
	ForkchoiceUpdatedV2(ctx context.Context, fcState *api.ForkchoiceStateV1, pAttributes *api.PayloadAttributes) (api.ForkChoiceResponse, error)

	GetPayloadV1(ctx context.Context, payloadId *api.PayloadID) (api.ExecutableData, error)
	GetPayloadV2(ctx context.Context, payloadId *api.PayloadID) (api.ExecutableData, *big.Int, error)

	NewPayloadV1(ctx context.Context, payload *client_types.ExecutableDataV1) (api.PayloadStatusV1, error)
	NewPayloadV2(ctx context.Context, payload *api.ExecutableData) (api.PayloadStatusV1, error)

	GetPayloadBodiesByRangeV1(ctx context.Context, start uint64, count uint64) ([]*client_types.ExecutionPayloadBodyV1, error)
	GetPayloadBodiesByHashV1(ctx context.Context, hashes []common.Hash) ([]*client_types.ExecutionPayloadBodyV1, error)

	LatestForkchoiceSent() (fcState *api.ForkchoiceStateV1, pAttributes *api.PayloadAttributes)
	LatestNewPayloadSent() (payload *api.ExecutableData)

	LatestForkchoiceResponse() (fcuResponse *api.ForkChoiceResponse)
	LatestNewPayloadResponse() (payloadResponse *api.PayloadStatusV1)
}

type EngineClient interface {
	// General Methods
	ID() string
	Close() error
	EnodeURL() (string, error)

	// Local Test Account Management
	GetNextAccountNonce(testCtx context.Context, account common.Address) (uint64, error)
	UpdateNonce(testCtx context.Context, account common.Address, newNonce uint64) error

	// TTD Methods
	TerminalTotalDifficulty() *big.Int
	GetTotalDifficulty(context.Context) (*big.Int, error)

	// Test Methods
	PostRunVerifications() error

	// Engine, Eth Interfaces
	Engine
	Eth
}

type EngineStarter interface {
	StartClient(T *hivesim.T, testContext context.Context, genesis *core.Genesis, ClientParams hivesim.Params, ClientFiles hivesim.Params, bootClients ...EngineClient) (EngineClient, error)
}

var (
	Head      *big.Int // Nil
	Pending   = big.NewInt(-2)
	Finalized = big.NewInt(-3)
	Safe      = big.NewInt(-4)
)
