package main

import (
	"context"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
)

// Test Engine API Helper Structs

type TestEngineClient struct {
	*TestEnv
	*EngineClient
}

func NewTestEngineClient(t *TestEnv, ec *EngineClient) *TestEngineClient {
	return &TestEngineClient{
		TestEnv:      t,
		EngineClient: ec,
	}
}

// ForkchoiceUpdatedV1

type ForkchoiceResponseExpectObject struct {
	*TestEnv
	Response ForkChoiceResponse
	Error    error
}

func (tec *TestEngineClient) TestEngineForkchoiceUpdatedV1(fcState *ForkchoiceStateV1, pAttributes *PayloadAttributesV1) *ForkchoiceResponseExpectObject {
	resp, err := tec.EngineClient.EngineForkchoiceUpdatedV1(tec.EngineClient.Ctx(), fcState, pAttributes)
	return &ForkchoiceResponseExpectObject{
		TestEnv:  tec.TestEnv,
		Response: resp,
		Error:    err,
	}
}

func (exp *ForkchoiceResponseExpectObject) ExpectNoError() {
	if exp.Error != nil {
		exp.Fatalf("FAIL (%s): Unexpected error on EngineForkchoiceUpdatedV1: %v, expected=<None>", exp.TestName, exp.Error)
	}
}

func (exp *ForkchoiceResponseExpectObject) ExpectNoValidationError() {
	if exp.Response.PayloadStatus.ValidationError != nil {
		exp.Fatalf("FAIL (%s): Unexpected validation error on EngineForkchoiceUpdatedV1: %v, expected=<None>", exp.TestName, exp.Response.PayloadStatus.ValidationError)
	}
}

func (exp *ForkchoiceResponseExpectObject) ExpectError() {
	if exp.Error == nil {
		exp.Fatalf("FAIL (%s): Expected error on EngineForkchoiceUpdatedV1: response=%v", exp.TestName, exp.Response)
	}
}

func (exp *ForkchoiceResponseExpectObject) ExpectPayloadStatus(ps PayloadStatus) {
	exp.ExpectNoError()
	if exp.Response.PayloadStatus.Status != ps {
		exp.Fatalf("FAIL (%s): Unexpected status response on EngineForkchoiceUpdatedV1: %v, expected=%v", exp.TestName, exp.Response.PayloadStatus.Status, ps)
	}
}

func (exp *ForkchoiceResponseExpectObject) ExpectAnyPayloadStatus(statuses ...PayloadStatus) {
	exp.ExpectNoError()
	for _, status := range statuses {
		if exp.Response.PayloadStatus.Status == status {
			return
		}
	}
	exp.Fatalf("FAIL (%s): Unexpected status response on EngineForkchoiceUpdatedV1: %v, expected=%v", exp.TestName, exp.Response.PayloadStatus.Status, PayloadStatusSlice(statuses).String())
}

func (exp *ForkchoiceResponseExpectObject) ExpectLatestValidHash(lvh *common.Hash) {
	exp.ExpectNoError()
	if ((lvh == nil || exp.Response.PayloadStatus.LatestValidHash == nil) && exp.Response.PayloadStatus.LatestValidHash != lvh) ||
		(lvh != nil && exp.Response.PayloadStatus.LatestValidHash != nil && *exp.Response.PayloadStatus.LatestValidHash != *lvh) {
		exp.Fatalf("FAIL (%v): Unexpected LatestValidHash on EngineForkchoiceUpdatedV1: %v, expected=%v", exp.TestName, exp.Response.PayloadStatus.LatestValidHash, lvh)
	}
}

func (exp *ForkchoiceResponseExpectObject) ExpectPayloadID(pid *PayloadID) {
	exp.ExpectNoError()
	if ((exp.Response.PayloadID == nil || pid == nil) && exp.Response.PayloadID != pid) ||
		(exp.Response.PayloadID != nil && pid != nil && *exp.Response.PayloadID != *pid) {
		exp.Fatalf("FAIL (%v): Unexpected PayloadID on EngineForkchoiceUpdatedV1: %v, expected=%v", exp.TestName, exp.Response.PayloadID, pid)
	}
}

// NewPayloadV1

type NewPayloadResponseExpectObject struct {
	*TestEnv
	Status PayloadStatusV1
	Error  error
}

func (tec *TestEngineClient) TestEngineNewPayloadV1(payload *ExecutableDataV1) *NewPayloadResponseExpectObject {
	status, err := tec.EngineClient.EngineNewPayloadV1(tec.EngineClient.Ctx(), payload)
	return &NewPayloadResponseExpectObject{
		TestEnv: tec.TestEnv,
		Status:  status,
		Error:   err,
	}
}

func (exp *NewPayloadResponseExpectObject) ExpectNoError() {
	if exp.Error != nil {
		exp.Fatalf("FAIL (%s): Expected no error on EngineNewPayloadV1: error=%v", exp.TestName, exp.Error)
	}
}

func (exp *NewPayloadResponseExpectObject) ExpectError() {
	if exp.Error == nil {
		exp.Fatalf("FAIL (%s): Expected error on EngineNewPayloadV1: status=%v", exp.TestName, exp.Status)
	}
}

func (exp *NewPayloadResponseExpectObject) ExpectNoValidationError() {
	if exp.Status.ValidationError != nil {
		exp.Fatalf("FAIL (%s): Unexpected validation error on EngineNewPayloadV1: %v, expected=<None>", exp.TestName, exp.Status.ValidationError)
	}
}

func (exp *NewPayloadResponseExpectObject) ExpectStatus(ps PayloadStatus) {
	exp.ExpectNoError()
	if exp.Status.Status != ps {
		exp.Fatalf("FAIL (%s): Unexpected status response on EngineNewPayloadV1: %v, expected=%v", exp.TestName, exp.Status.Status, ps)
	}
}

func (exp *NewPayloadResponseExpectObject) ExpectStatusEither(statuses ...PayloadStatus) {
	exp.ExpectNoError()
	for _, status := range statuses {
		if exp.Status.Status == status {
			return
		}
	}
	exp.Fatalf("FAIL (%s): Unexpected status response on EngineNewPayloadV1: %v, expected=%v", exp.TestName, exp.Status.Status, PayloadStatusSlice(statuses).String())
}

func (exp *NewPayloadResponseExpectObject) ExpectLatestValidHash(lvh *common.Hash) {
	exp.ExpectNoError()
	if ((lvh == nil || exp.Status.LatestValidHash == nil) && exp.Status.LatestValidHash != lvh) ||
		(lvh != nil && exp.Status.LatestValidHash != nil && *exp.Status.LatestValidHash != *lvh) {
		exp.Fatalf("FAIL (%v): Unexpected LatestValidHash on EngineNewPayloadV1: %v, expected=%v", exp.TestName, exp.Status.LatestValidHash, lvh)
	}
}

// GetPayloadV1
type GetPayloadResponseExpectObject struct {
	*TestEnv
	Payload ExecutableDataV1
	Error   error
}

func (tec *TestEngineClient) TestEngineGetPayloadV1(payloadID *PayloadID) *GetPayloadResponseExpectObject {
	payload, err := tec.EngineClient.EngineGetPayloadV1(tec.EngineClient.Ctx(), payloadID)
	return &GetPayloadResponseExpectObject{
		TestEnv: tec.TestEnv,
		Payload: payload,
		Error:   err,
	}
}

func (exp *GetPayloadResponseExpectObject) ExpectNoError() {
	if exp.Error != nil {
		exp.Fatalf("FAIL (%s): Expected no error on EngineGetPayloadV1: error=%v", exp.TestName, exp.Error)
	}
}

func (exp *GetPayloadResponseExpectObject) ExpectError() {
	if exp.Error == nil {
		exp.Fatalf("FAIL (%s): Expected error on EngineGetPayloadV1: payload=%v", exp.TestName, exp.Payload)
	}
}

func (exp *GetPayloadResponseExpectObject) ExpectPayloadParentHash(expectedParentHash common.Hash) {
	exp.ExpectNoError()
	if exp.Payload.ParentHash != expectedParentHash {
		exp.Fatalf("FAIL (%s): Unexpected parent hash for payload on EngineGetPayloadV1: %v, expected=%v", exp.TestName, exp.Payload.ParentHash, expectedParentHash)
	}
}

func (exp *GetPayloadResponseExpectObject) ExpectPayloadBlockNumber(expectedBlockNumber uint64) {
	exp.ExpectNoError()
	if exp.Payload.Number != expectedBlockNumber {
		exp.Fatalf("FAIL (%s): Unexpected block number for payload on EngineGetPayloadV1: %v, expected=%v", exp.TestName, exp.Payload.Number, expectedBlockNumber)
	}
}

func (exp *GetPayloadResponseExpectObject) ExpectPrevRandao(expectedPrevRandao common.Hash) {
	exp.ExpectNoError()
	if exp.Payload.PrevRandao != expectedPrevRandao {
		exp.Fatalf("FAIL (%s): Unexpected prevRandao for payload on EngineGetPayloadV1: %v, expected=%v", exp.TestName, exp.Payload.PrevRandao, expectedPrevRandao)
	}
}

func (exp *GetPayloadResponseExpectObject) ExpectTimestamp(expectedTimestamp uint64) {
	exp.ExpectNoError()
	if exp.Payload.Timestamp != expectedTimestamp {
		exp.Fatalf("FAIL (%s): Unexpected timestamp for payload on EngineGetPayloadV1: %v, expected=%v", exp.TestName, exp.Payload.Timestamp, expectedTimestamp)
	}
}

// Test Eth JSON-RPC Helper Structs
type TestEthClient struct {
	*TestEnv
	*ethclient.Client
	lastCtx context.Context
}

func NewTestEthClient(t *TestEnv, eth *ethclient.Client) *TestEthClient {
	return &TestEthClient{
		TestEnv: t,
		Client:  eth,
	}
}

// BlockNumber
type BlockNumberResponseExpectObject struct {
	*TestEnv
	Call   string
	Number uint64
	Error  error
}

func (teth *TestEthClient) TestBlockNumber() *BlockNumberResponseExpectObject {
	number, err := teth.BlockNumber(teth.Ctx())
	return &BlockNumberResponseExpectObject{
		TestEnv: teth.TestEnv,
		Call:    "BlockNumber",
		Number:  number,
		Error:   err,
	}
}

func (exp *BlockNumberResponseExpectObject) ExpectNoError() {
	if exp.Error != nil {
		exp.Fatalf("FAIL (%s): Unexpected error on %s: %v, expected=<None>", exp.TestName, exp.Call, exp.Error)
	}
}

func (exp *BlockNumberResponseExpectObject) ExpectNumber(number uint64) {
	exp.ExpectNoError()
	if exp.Number != number {
		exp.Fatalf("FAIL (%s): Unexpected block number on %s: %d, expected=%d", exp.TestName, exp.Call, exp.Number, number)
	}
}

// Header

type HeaderResponseExpectObject struct {
	*TestEnv
	Call   string
	Header *types.Header
	Error  error
}

func (teth *TestEthClient) TestHeaderByNumber(number *big.Int) *HeaderResponseExpectObject {
	header, err := teth.HeaderByNumber(teth.Ctx(), number)
	return &HeaderResponseExpectObject{
		TestEnv: teth.TestEnv,
		Call:    "HeaderByNumber",
		Header:  header,
		Error:   err,
	}
}

func (exp *HeaderResponseExpectObject) ExpectNoError() {
	if exp.Error != nil {
		exp.Fatalf("FAIL (%s): Unexpected error on %s: %v, expected=<None>", exp.TestName, exp.Call, exp.Error)
	}
}

func (exp *HeaderResponseExpectObject) ExpectHash(expHash common.Hash) {
	exp.ExpectNoError()
	if exp.Header.Hash() != expHash {
		exp.Fatalf("FAIL (%s): Unexpected hash on %s: %v, expected=%v", exp.TestName, exp.Call, exp.Header.Hash(), expHash)
	}
}

// Block

type BlockResponseExpectObject struct {
	*TestEnv
	Call  string
	Block *types.Block
	Error error
}

func (teth *TestEthClient) TestBlockByNumber(number *big.Int) *BlockResponseExpectObject {
	block, err := teth.BlockByNumber(teth.Ctx(), number)
	return &BlockResponseExpectObject{
		TestEnv: teth.TestEnv,
		Call:    "BlockByNumber",
		Block:   block,
		Error:   err,
	}
}

func (teth *TestEthClient) TestBlockByHash(hash common.Hash) *BlockResponseExpectObject {
	block, err := teth.BlockByHash(teth.Ctx(), hash)
	return &BlockResponseExpectObject{
		TestEnv: teth.TestEnv,
		Call:    "BlockByHash",
		Block:   block,
		Error:   err,
	}
}

func (exp *BlockResponseExpectObject) ExpectError() {
	if exp.Error == nil {
		exp.Fatalf("FAIL (%s): Expected error on %s: block=%v", exp.TestName, exp.Call, exp.Block)
	}
}

func (exp *BlockResponseExpectObject) ExpectNoError() {
	if exp.Error != nil {
		exp.Fatalf("FAIL (%s): Unexpected error on %s: %v, expected=<None>", exp.TestName, exp.Call, exp.Error)
	}
}

func (exp *BlockResponseExpectObject) ExpectHash(expHash common.Hash) {
	exp.ExpectNoError()
	if exp.Block.Hash() != expHash {
		exp.Fatalf("FAIL (%s): Unexpected hash on %s: %v, expected=%v", exp.TestName, exp.Call, exp.Block.Hash(), expHash)
	}
}

func (exp *BlockResponseExpectObject) ExpectTransactionCountEqual(expTxCount int) {
	exp.ExpectNoError()
	if len(exp.Block.Transactions()) != expTxCount {
		exp.Fatalf("FAIL (%s): Unexpected transaction count on %s: %v, expected=%v", exp.TestName, exp.Call, len(exp.Block.Transactions()), expTxCount)
	}
}

func (exp *BlockResponseExpectObject) ExpectTransactionCountGreaterThan(expTxCount int) {
	exp.ExpectNoError()
	if len(exp.Block.Transactions()) <= expTxCount {
		exp.Fatalf("FAIL (%s): Unexpected transaction count on %s: %v, expected > %v", exp.TestName, exp.Call, len(exp.Block.Transactions()), expTxCount)
	}
}

func (exp *BlockResponseExpectObject) ExpectCoinbase(expCoinbase common.Address) {
	exp.ExpectNoError()
	if exp.Block.Coinbase() != expCoinbase {
		exp.Fatalf("FAIL (%s): Unexpected coinbase on %s: %v, expected=%v", exp.TestName, exp.Call, exp.Block.Coinbase(), expCoinbase)
	}
}

// Balance

type BalanceResponseExpectObject struct {
	*TestEnv
	Call    string
	Balance *big.Int
	Error   error
}

func (teth *TestEthClient) TestBalanceAt(account common.Address, number *big.Int) *BalanceResponseExpectObject {
	balance, err := teth.BalanceAt(teth.Ctx(), account, number)
	return &BalanceResponseExpectObject{
		TestEnv: teth.TestEnv,
		Call:    "BalanceAt",
		Balance: balance,
		Error:   err,
	}
}

func (exp *BalanceResponseExpectObject) ExpectNoError() {
	if exp.Error != nil {
		exp.Fatalf("FAIL (%s): Unexpected error on %s: %v, expected=<None>", exp.TestName, exp.Call, exp.Error)
	}
}

func (exp *BalanceResponseExpectObject) ExpectBalanceEqual(expBalance *big.Int) {
	exp.ExpectNoError()
	if ((expBalance == nil || exp.Balance == nil) && expBalance != exp.Balance) ||
		(expBalance != nil && exp.Balance != nil && expBalance.Cmp(exp.Balance) != 0) {
		exp.Fatalf("FAIL (%s): Unexpected balance on %s: %v, expected=%v", exp.TestName, exp.Call, exp.Balance, expBalance)
	}
}

// Storage

type StorageResponseExpectObject struct {
	*TestEnv
	Call    string
	Storage []byte
	Error   error
}

func (teth *TestEthClient) TestStorageAt(account common.Address, key common.Hash, number *big.Int) *StorageResponseExpectObject {
	storage, err := teth.StorageAt(teth.Ctx(), account, key, number)
	return &StorageResponseExpectObject{
		TestEnv: teth.TestEnv,
		Call:    "StorageAt",
		Storage: storage,
		Error:   err,
	}
}

func (exp *StorageResponseExpectObject) ExpectNoError() {
	if exp.Error != nil {
		exp.Fatalf("FAIL (%s): Unexpected error on %s: %v, expected=<None>", exp.TestName, exp.Call, exp.Error)
	}
}

func (exp *StorageResponseExpectObject) ExpectBigIntStorageEqual(expBigInt *big.Int) {
	exp.ExpectNoError()
	bigInt := big.NewInt(0)
	bigInt.SetBytes(exp.Storage)
	if ((bigInt == nil || expBigInt == nil) && bigInt != expBigInt) ||
		(bigInt != nil && expBigInt != nil && bigInt.Cmp(expBigInt) != 0) {
		exp.Fatalf("FAIL (%s): Unexpected storage on %s: %v, expected=%v", exp.TestName, exp.Call, bigInt, expBigInt)
	}
}

func (exp *StorageResponseExpectObject) ExpectStorageEqual(expStorage common.Hash) {
	exp.ExpectNoError()
	if expStorage != common.BytesToHash(exp.Storage) {
		exp.Fatalf("FAIL (%s): Unexpected storage on %s: %v, expected=%v", exp.TestName, exp.Call, exp.Storage, expStorage)
	}
}

// Ctx returns a context with the default timeout.
// For subsequent calls to Ctx, it also cancels the previous context.
func (t *TestEthClient) Ctx() context.Context {
	if t.lastCtx != nil {
		t.lastCancel()
	}
	t.lastCtx, t.lastCancel = context.WithTimeout(context.Background(), rpcTimeout)
	return t.lastCtx
}
