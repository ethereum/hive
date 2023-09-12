package client

import (
	"context"
	"math/big"

	api "github.com/ethereum/go-ethereum/beacon/engine"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/hive/hivesim"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	typ "github.com/ethereum/hive/simulators/ethereum/engine/types"
)

type Eth interface {
	BalanceAt(ctx context.Context, account common.Address, blockNumber *big.Int) (*big.Int, error)
	BlockByNumber(ctx context.Context, number *big.Int) (*types.Block, error)
	BlockNumber(ctx context.Context) (uint64, error)
	BlockByHash(ctx context.Context, hash common.Hash) (*types.Block, error)
	HeaderByNumber(ctx context.Context, number *big.Int) (*types.Header, error)
	HeaderByHash(ctx context.Context, hash common.Hash) (*types.Header, error)
	SendTransaction(ctx context.Context, tx typ.Transaction) error
	SendTransactions(ctx context.Context, txs ...typ.Transaction) []error
	StorageAt(ctx context.Context, account common.Address, key common.Hash, blockNumber *big.Int) ([]byte, error)
	StorageAtKeys(ctx context.Context, account common.Address, keys []common.Hash, blockNumber *big.Int) (map[common.Hash]*common.Hash, error)
	TransactionReceipt(ctx context.Context, txHash common.Hash) (*types.Receipt, error)
	NonceAt(ctx context.Context, account common.Address, blockNumber *big.Int) (uint64, error)
	TransactionByHash(ctx context.Context, hash common.Hash) (tx *types.Transaction, isPending bool, err error)
}

type Engine interface {
	ForkchoiceUpdatedV1(ctx context.Context, fcState *api.ForkchoiceStateV1, pAttributes *typ.PayloadAttributes) (api.ForkChoiceResponse, error)
	ForkchoiceUpdatedV2(ctx context.Context, fcState *api.ForkchoiceStateV1, pAttributes *typ.PayloadAttributes) (api.ForkChoiceResponse, error)
	ForkchoiceUpdatedV3(ctx context.Context, fcState *api.ForkchoiceStateV1, pAttributes *typ.PayloadAttributes) (api.ForkChoiceResponse, error)
	ForkchoiceUpdated(ctx context.Context, version int, fcState *api.ForkchoiceStateV1, pAttributes *typ.PayloadAttributes) (api.ForkChoiceResponse, error)

	GetPayloadV1(ctx context.Context, payloadId *api.PayloadID) (typ.ExecutableData, error)
	GetPayloadV2(ctx context.Context, payloadId *api.PayloadID) (typ.ExecutableData, *big.Int, error)
	GetPayloadV3(ctx context.Context, payloadId *api.PayloadID) (typ.ExecutableData, *big.Int, *typ.BlobsBundle, *bool, error)
	GetPayload(ctx context.Context, version int, payloadId *api.PayloadID) (typ.ExecutableData, *big.Int, *typ.BlobsBundle, *bool, error)

	NewPayload(ctx context.Context, version int, payload *typ.ExecutableData) (api.PayloadStatusV1, error)
	NewPayloadV1(ctx context.Context, payload *typ.ExecutableData) (api.PayloadStatusV1, error)
	NewPayloadV2(ctx context.Context, payload *typ.ExecutableData) (api.PayloadStatusV1, error)
	NewPayloadV3(ctx context.Context, payload *typ.ExecutableData) (api.PayloadStatusV1, error)

	GetPayloadBodiesByRangeV1(ctx context.Context, start uint64, count uint64) ([]*typ.ExecutionPayloadBodyV1, error)
	GetPayloadBodiesByHashV1(ctx context.Context, hashes []common.Hash) ([]*typ.ExecutionPayloadBodyV1, error)

	LatestForkchoiceSent() (fcState *api.ForkchoiceStateV1, pAttributes *typ.PayloadAttributes)
	LatestNewPayloadSent() (payload *typ.ExecutableData)

	LatestForkchoiceResponse() (fcuResponse *api.ForkChoiceResponse)
	LatestNewPayloadResponse() (payloadResponse *api.PayloadStatusV1)
}

type EngineAPIVersionResolver interface {
	ForkchoiceUpdatedVersion(headTimestamp uint64, payloadAttributesTimestamp *uint64) int
	NewPayloadVersion(timestamp uint64) int
	GetPayloadVersion(timestamp uint64) int
}

type EngineClient interface {
	// General Methods
	ID() string
	Close() error
	EnodeURL() (string, error)

	// Local Test Account Management
	GetLastAccountNonce(testCtx context.Context, account common.Address, head *types.Header) (uint64, error)
	GetNextAccountNonce(testCtx context.Context, account common.Address, head *types.Header) (uint64, error)
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
	Head                           *big.Int // Nil
	Pending                        = big.NewInt(-2)
	Finalized                      = big.NewInt(-3)
	Safe                           = big.NewInt(-4)
	LatestForkchoiceUpdatedVersion = 3
	LatestNewPayloadVersion        = 3
)
