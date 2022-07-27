package test

import (
	"context"
	"fmt"
	"math/big"
	"runtime"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	api "github.com/ethereum/go-ethereum/core/beacon"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/hive/simulators/ethereum/engine/client"
	"github.com/ethereum/hive/simulators/ethereum/engine/globals"
)

// Print the caller to this file
func PrintExpectStack(t *Env) {
	_, currentFile, _, _ := runtime.Caller(0)
	for i := 1; ; i++ {
		_, file, line, ok := runtime.Caller(i)
		if !ok {
			return
		}
		if file == currentFile {
			continue
		}
		fmt.Printf("DEBUG (%s): Failed `Expect*` routine called from: file=%s, line=%d\n", t.TestName, file, line)
		return
	}
}

type PayloadStatus string

const (
	Unknown          PayloadStatus = ""
	Valid                          = "VALID"
	Invalid                        = "INVALID"
	Syncing                        = "SYNCING"
	Accepted                       = "ACCEPTED"
	InvalidBlockHash               = "INVALID_BLOCK_HASH"
)

func StatusesToString(statuses []PayloadStatus) []string {
	result := make([]string, len(statuses))
	for i, s := range statuses {
		result[i] = string(s)
	}
	return result
}

// Test Engine API Helper Structs
type ExpectEnv struct {
	*Env
}

func (e ExpectEnv) Fatalf(format string, values ...interface{}) {
	PrintExpectStack(e.Env)
	e.Env.Fatalf(format, values...)
}

type TestEngineClient struct {
	*ExpectEnv
	client.EngineClient
}

func NewTestEngineClient(t *Env, ec client.EngineClient) *TestEngineClient {
	return &TestEngineClient{
		ExpectEnv:    &ExpectEnv{t},
		EngineClient: ec,
	}
}

// ForkchoiceUpdatedV1

type ForkchoiceResponseExpectObject struct {
	*ExpectEnv
	Response api.ForkChoiceResponse
	Error    error
}

func (tec *TestEngineClient) TestEngineForkchoiceUpdatedV1(fcState *api.ForkchoiceStateV1, pAttributes *api.PayloadAttributesV1) *ForkchoiceResponseExpectObject {
	ctx, cancel := context.WithTimeout(tec.TestContext, globals.RPCTimeout)
	defer cancel()
	resp, err := tec.Engine.ForkchoiceUpdatedV1(ctx, fcState, pAttributes)
	return &ForkchoiceResponseExpectObject{
		ExpectEnv: &ExpectEnv{tec.Env},
		Response:  resp,
		Error:     err,
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

func (exp *ForkchoiceResponseExpectObject) ExpectErrorCode(code int) {
	// TODO: Actually check error code
	if exp.Error == nil {
		exp.Fatalf("FAIL (%s): Expected error on EngineForkchoiceUpdatedV1: response=%v", exp.TestName, exp.Response)
	}
}

func (exp *ForkchoiceResponseExpectObject) ExpectPayloadStatus(ps PayloadStatus) {
	exp.ExpectNoError()
	if PayloadStatus(exp.Response.PayloadStatus.Status) != ps {
		exp.Fatalf("FAIL (%s): Unexpected status response on EngineForkchoiceUpdatedV1: %v, expected=%v", exp.TestName, exp.Response.PayloadStatus.Status, ps)
	}
}

func (exp *ForkchoiceResponseExpectObject) ExpectAnyPayloadStatus(statuses ...PayloadStatus) {
	exp.ExpectNoError()
	for _, status := range statuses {
		if PayloadStatus(exp.Response.PayloadStatus.Status) == status {
			return
		}
	}
	exp.Fatalf("FAIL (%s): Unexpected status response on EngineForkchoiceUpdatedV1: %v, expected=%v", exp.TestName, exp.Response.PayloadStatus.Status, StatusesToString(statuses))
}

func (exp *ForkchoiceResponseExpectObject) ExpectLatestValidHash(lvh *common.Hash) {
	exp.ExpectNoError()
	if ((lvh == nil || exp.Response.PayloadStatus.LatestValidHash == nil) && exp.Response.PayloadStatus.LatestValidHash != lvh) ||
		(lvh != nil && exp.Response.PayloadStatus.LatestValidHash != nil && *exp.Response.PayloadStatus.LatestValidHash != *lvh) {
		exp.Fatalf("FAIL (%v): Unexpected LatestValidHash on EngineForkchoiceUpdatedV1: %v, expected=%v", exp.TestName, exp.Response.PayloadStatus.LatestValidHash, lvh)
	}
}

func (exp *ForkchoiceResponseExpectObject) ExpectPayloadID(pid *api.PayloadID) {
	exp.ExpectNoError()
	if ((exp.Response.PayloadID == nil || pid == nil) && exp.Response.PayloadID != pid) ||
		(exp.Response.PayloadID != nil && pid != nil && *exp.Response.PayloadID != *pid) {
		exp.Fatalf("FAIL (%v): Unexpected PayloadID on EngineForkchoiceUpdatedV1: %v, expected=%v", exp.TestName, exp.Response.PayloadID, pid)
	}
}

// NewPayloadV1

type NewPayloadResponseExpectObject struct {
	*ExpectEnv
	Status api.PayloadStatusV1
	Error  error
}

func (tec *TestEngineClient) TestEngineNewPayloadV1(payload *api.ExecutableDataV1) *NewPayloadResponseExpectObject {
	ctx, cancel := context.WithTimeout(tec.TestContext, globals.RPCTimeout)
	defer cancel()
	status, err := tec.Engine.NewPayloadV1(ctx, payload)
	return &NewPayloadResponseExpectObject{
		ExpectEnv: &ExpectEnv{tec.Env},
		Status:    status,
		Error:     err,
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

func (exp *NewPayloadResponseExpectObject) ExpectErrorCode(code int) {
	// TODO: Actually check error code
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
	if PayloadStatus(exp.Status.Status) != ps {
		exp.Fatalf("FAIL (%s): Unexpected status response on EngineNewPayloadV1: %v, expected=%v", exp.TestName, exp.Status.Status, ps)
	}
}

func (exp *NewPayloadResponseExpectObject) ExpectStatusEither(statuses ...PayloadStatus) {
	exp.ExpectNoError()
	for _, status := range statuses {
		if PayloadStatus(exp.Status.Status) == status {
			return
		}
	}

	exp.Fatalf("FAIL (%s): Unexpected status response on EngineNewPayloadV1: %v, expected=%v", exp.TestName, exp.Status.Status, strings.Join(StatusesToString(statuses), ","))
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
	*ExpectEnv
	Payload api.ExecutableDataV1
	Error   error
}

func (tec *TestEngineClient) TestEngineGetPayloadV1(payloadID *api.PayloadID) *GetPayloadResponseExpectObject {
	ctx, cancel := context.WithTimeout(tec.TestContext, globals.RPCTimeout)
	defer cancel()
	payload, err := tec.Engine.GetPayloadV1(ctx, payloadID)
	return &GetPayloadResponseExpectObject{
		ExpectEnv: &ExpectEnv{tec.Env},
		Payload:   payload,
		Error:     err,
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

func (exp *GetPayloadResponseExpectObject) ExpectErrorCode(code int) {
	// TODO: Actually check error code
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
	if exp.Payload.Random != expectedPrevRandao {
		exp.Fatalf("FAIL (%s): Unexpected prevRandao for payload on EngineGetPayloadV1: %v, expected=%v", exp.TestName, exp.Payload.Random, expectedPrevRandao)
	}
}

func (exp *GetPayloadResponseExpectObject) ExpectTimestamp(expectedTimestamp uint64) {
	exp.ExpectNoError()
	if exp.Payload.Timestamp != expectedTimestamp {
		exp.Fatalf("FAIL (%s): Unexpected timestamp for payload on EngineGetPayloadV1: %v, expected=%v", exp.TestName, exp.Payload.Timestamp, expectedTimestamp)
	}
}

// BlockNumber
type BlockNumberResponseExpectObject struct {
	*ExpectEnv
	Call   string
	Number uint64
	Error  error
}

func (tec *TestEngineClient) TestBlockNumber() *BlockNumberResponseExpectObject {
	ctx, cancel := context.WithTimeout(tec.TestContext, globals.RPCTimeout)
	defer cancel()
	number, err := tec.Eth.BlockNumber(ctx)
	return &BlockNumberResponseExpectObject{
		ExpectEnv: &ExpectEnv{tec.Env},
		Call:      "BlockNumber",
		Number:    number,
		Error:     err,
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
	*ExpectEnv
	Call   string
	Header *types.Header
	Error  error
}

func (tec *TestEngineClient) TestHeaderByNumber(number *big.Int) *HeaderResponseExpectObject {
	ctx, cancel := context.WithTimeout(tec.TestContext, globals.RPCTimeout)
	defer cancel()
	header, err := tec.Engine.HeaderByNumber(ctx, number)
	return &HeaderResponseExpectObject{
		ExpectEnv: &ExpectEnv{tec.Env},
		Call:      "HeaderByNumber",
		Header:    header,
		Error:     err,
	}
}

func (exp *HeaderResponseExpectObject) ExpectNoError() {
	if exp.Error != nil {
		exp.Fatalf("FAIL (%s): Unexpected error on %s: %v, expected=<None>", exp.TestName, exp.Call, exp.Error)
	}
}

func (exp *HeaderResponseExpectObject) ExpectError() {
	if exp.Error == nil {
		exp.Fatalf("FAIL (%s): Expected error on %s: header=%v", exp.TestName, exp.Call, exp.Header)
	}
}

func (exp *HeaderResponseExpectObject) ExpectErrorCode(code int) {
	// TODO: Actually check error code
	if exp.Error == nil {
		exp.Fatalf("FAIL (%s): Expected error on %s: header=%v", exp.TestName, exp.Call, exp.Header)
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
	*ExpectEnv
	Call  string
	Block *types.Block
	Error error
}

func (tec *TestEngineClient) TestBlockByNumber(number *big.Int) *BlockResponseExpectObject {
	ctx, cancel := context.WithTimeout(tec.TestContext, globals.RPCTimeout)
	defer cancel()
	block, err := tec.Eth.BlockByNumber(ctx, number)
	return &BlockResponseExpectObject{
		ExpectEnv: &ExpectEnv{tec.Env},
		Call:      "BlockByNumber",
		Block:     block,
		Error:     err,
	}
}

func (tec *TestEngineClient) TestBlockByHash(hash common.Hash) *BlockResponseExpectObject {
	ctx, cancel := context.WithTimeout(tec.TestContext, globals.RPCTimeout)
	defer cancel()
	block, err := tec.Eth.BlockByHash(ctx, hash)
	return &BlockResponseExpectObject{
		ExpectEnv: &ExpectEnv{tec.Env},
		Call:      "BlockByHash",
		Block:     block,
		Error:     err,
	}
}

func (exp *BlockResponseExpectObject) ExpectError() {
	if exp.Error == nil {
		exp.Fatalf("FAIL (%s): Expected error on %s: block=%v", exp.TestName, exp.Call, exp.Block)
	}
}

func (exp *BlockResponseExpectObject) ExpectErrorCode(code int) {
	// TODO: Actually check error code
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
	*ExpectEnv
	Call    string
	Balance *big.Int
	Error   error
}

func (tec *TestEngineClient) TestBalanceAt(account common.Address, number *big.Int) *BalanceResponseExpectObject {
	ctx, cancel := context.WithTimeout(tec.TestContext, globals.RPCTimeout)
	defer cancel()
	balance, err := tec.Eth.BalanceAt(ctx, account, number)
	return &BalanceResponseExpectObject{
		ExpectEnv: &ExpectEnv{tec.Env},
		Call:      "BalanceAt",
		Balance:   balance,
		Error:     err,
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
	*ExpectEnv
	Call    string
	Storage []byte
	Error   error
}

func (tec *TestEngineClient) TestStorageAt(account common.Address, key common.Hash, number *big.Int) *StorageResponseExpectObject {
	ctx, cancel := context.WithTimeout(tec.TestContext, globals.RPCTimeout)
	defer cancel()
	storage, err := tec.Eth.StorageAt(ctx, account, key, number)
	return &StorageResponseExpectObject{
		ExpectEnv: &ExpectEnv{tec.Env},
		Call:      "StorageAt",
		Storage:   storage,
		Error:     err,
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

// Transaction Receipt
type TransactionReceiptExpectObject struct {
	*ExpectEnv
	Call    string
	Receipt *types.Receipt
	Error   error
}

func (tec *TestEngineClient) TestTransactionReceipt(txHash common.Hash) *TransactionReceiptExpectObject {
	ctx, cancel := context.WithTimeout(tec.TestContext, globals.RPCTimeout)
	defer cancel()
	receipt, err := tec.Eth.TransactionReceipt(ctx, txHash)
	return &TransactionReceiptExpectObject{
		ExpectEnv: &ExpectEnv{tec.Env},
		Call:      "TransactionReceipt",
		Receipt:   receipt,
		Error:     err,
	}
}

func (exp *TransactionReceiptExpectObject) ExpectError() {
	if exp.Error == nil {
		exp.Fatalf("FAIL (%s): Expected error on %s: block=%v", exp.TestName, exp.Call, exp.Receipt)
	}
}

func (exp *TransactionReceiptExpectObject) ExpectErrorCode(code int) {
	// TODO: Actually check error code
	if exp.Error == nil {
		exp.Fatalf("FAIL (%s): Expected error on %s: block=%v", exp.TestName, exp.Call, exp.Receipt)
	}
}

func (exp *TransactionReceiptExpectObject) ExpectNoError() {
	if exp.Error != nil {
		exp.Fatalf("FAIL (%s): Unexpected error on %s: %v, expected=<None>", exp.TestName, exp.Call, exp.Error)
	}
}

func (exp *TransactionReceiptExpectObject) ExpectTransactionHash(expectedHash common.Hash) {
	exp.ExpectNoError()
	if exp.Receipt.TxHash != expectedHash {
		exp.Fatalf("FAIL (%s): Unexpected transaction hash on %s: %v, expected=%v", exp.TestName, exp.Call, exp.Receipt.TxHash, expectedHash)
	}
}

func (exp *TransactionReceiptExpectObject) ExpectBlockHash(expectedHash common.Hash) {
	exp.ExpectNoError()
	if exp.Receipt.BlockHash != expectedHash {
		exp.Fatalf("FAIL (%s): Unexpected transaction block hash on %s: %v, blockhash=%v, expected=%v", exp.TestName, exp.Call, exp.Receipt.TxHash, exp.Receipt.BlockHash, expectedHash)
	}
}
