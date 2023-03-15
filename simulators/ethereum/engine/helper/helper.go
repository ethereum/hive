package helper

import (
	"context"
	"crypto/ecdsa"
	"strings"
	"sync"
	"time"

	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"math/big"
	"net/http"
	"os"

	api "github.com/ethereum/go-ethereum/beacon/engine"
	"github.com/ethereum/hive/simulators/ethereum/engine/client"
	"github.com/ethereum/hive/simulators/ethereum/engine/globals"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/ethereum/hive/hivesim"
)

// From ethereum/rpc:

// LoggingRoundTrip writes requests and responses to the test log.
type LogF interface {
	Logf(format string, values ...interface{})
}

type LoggingRoundTrip struct {
	Logger LogF
	ID     string
	Inner  http.RoundTripper
}

func (rt *LoggingRoundTrip) RoundTrip(req *http.Request) (*http.Response, error) {
	// Read and log the request body.
	reqBytes, err := ioutil.ReadAll(req.Body)
	req.Body.Close()
	if err != nil {
		return nil, err
	}
	rt.Logger.Logf(">> (%s) %s", rt.ID, bytes.TrimSpace(reqBytes))
	reqCopy := *req
	reqCopy.Body = ioutil.NopCloser(bytes.NewReader(reqBytes))

	// Do the round trip.
	resp, err := rt.Inner.RoundTrip(&reqCopy)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Read and log the response bytes.
	respBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	respCopy := *resp
	respCopy.Body = ioutil.NopCloser(bytes.NewReader(respBytes))
	rt.Logger.Logf("<< (%s) %s", rt.ID, bytes.TrimSpace(respBytes))
	return &respCopy, nil
}

type InvalidPayloadBlockField string

const (
	InvalidParentHash             = "ParentHash"
	InvalidStateRoot              = "StateRoot"
	InvalidReceiptsRoot           = "ReceiptsRoot"
	InvalidNumber                 = "Number"
	InvalidGasLimit               = "GasLimit"
	InvalidGasUsed                = "GasUsed"
	InvalidTimestamp              = "Timestamp"
	InvalidPrevRandao             = "PrevRandao"
	InvalidOmmers                 = "Ommers"
	InvalidWithdrawals            = "Withdrawals"
	RemoveTransaction             = "Incomplete Transactions"
	InvalidTransactionSignature   = "Transaction Signature"
	InvalidTransactionNonce       = "Transaction Nonce"
	InvalidTransactionGas         = "Transaction Gas"
	InvalidTransactionGasPrice    = "Transaction GasPrice"
	InvalidTransactionGasTipPrice = "Transaction GasTipCapPrice"
	InvalidTransactionValue       = "Transaction Value"
	InvalidTransactionChainID     = "Transaction ChainID"
)

func TransactionInPayload(payload *api.ExecutableData, tx *types.Transaction) bool {
	for _, bytesTx := range payload.Transactions {
		var currentTx types.Transaction
		if err := currentTx.UnmarshalBinary(bytesTx); err == nil {
			if currentTx.Hash() == tx.Hash() {
				return true
			}
		}
	}
	return false
}

// Use client specific rpc methods to debug a transaction that includes the PREVRANDAO opcode
func DebugPrevRandaoTransaction(ctx context.Context, c *rpc.Client, clientType string, tx *types.Transaction, expectedPrevRandao *common.Hash) error {
	switch clientType {
	case "go-ethereum":
		return gethDebugPrevRandaoTransaction(ctx, c, tx, expectedPrevRandao)
	case "nethermind":
		return nethermindDebugPrevRandaoTransaction(ctx, c, tx, expectedPrevRandao)
	}
	fmt.Printf("debug_traceTransaction, no method to test client type %v", clientType)
	return nil
}

func gethDebugPrevRandaoTransaction(ctx context.Context, c *rpc.Client, tx *types.Transaction, expectedPrevRandao *common.Hash) error {
	type StructLogRes struct {
		Pc      uint64             `json:"pc"`
		Op      string             `json:"op"`
		Gas     uint64             `json:"gas"`
		GasCost uint64             `json:"gasCost"`
		Depth   int                `json:"depth"`
		Error   string             `json:"error,omitempty"`
		Stack   *[]string          `json:"stack,omitempty"`
		Memory  *[]string          `json:"memory,omitempty"`
		Storage *map[string]string `json:"storage,omitempty"`
	}

	type ExecutionResult struct {
		Gas         uint64         `json:"gas"`
		Failed      bool           `json:"failed"`
		ReturnValue string         `json:"returnValue"`
		StructLogs  []StructLogRes `json:"structLogs"`
	}

	var er *ExecutionResult
	if err := c.CallContext(ctx, &er, "debug_traceTransaction", tx.Hash()); err != nil {
		return err
	}
	if er == nil {
		return errors.New("debug_traceTransaction returned empty result")
	}
	prevRandaoFound := false
	for i, l := range er.StructLogs {
		if l.Op == "DIFFICULTY" || l.Op == "PREVRANDAO" {
			if i+1 >= len(er.StructLogs) {
				return errors.New("no information after PREVRANDAO operation")
			}
			prevRandaoFound = true
			stack := *(er.StructLogs[i+1].Stack)
			if len(stack) < 1 {
				return fmt.Errorf("invalid stack after PREVRANDAO operation: %v", l.Stack)
			}
			stackHash := common.HexToHash(stack[0])
			if stackHash != *expectedPrevRandao {
				return fmt.Errorf("invalid stack after PREVRANDAO operation, %v != %v", stackHash, expectedPrevRandao)
			}
		}
	}
	if !prevRandaoFound {
		return errors.New("PREVRANDAO opcode not found")
	}
	return nil
}

func nethermindDebugPrevRandaoTransaction(ctx context.Context, c *rpc.Client, tx *types.Transaction, expectedPrevRandao *common.Hash) error {
	var er *interface{}
	if err := c.CallContext(ctx, &er, "trace_transaction", tx.Hash()); err != nil {
		return err
	}
	return nil
}

func bytesSource(data []byte) func() (io.ReadCloser, error) {
	return func() (io.ReadCloser, error) {
		return ioutil.NopCloser(bytes.NewReader(data)), nil
	}
}

func LoadChain(path string) types.Blocks {
	fh, err := os.Open(path)
	if err != nil {
		panic(err)
	}
	defer fh.Close()
	var reader io.Reader = fh
	stream := rlp.NewStream(reader, 0)

	blocks := make(types.Blocks, 0)
	for {
		var b types.Block
		if err := stream.Decode(&b); err == io.EOF {
			break
		} else if err != nil {
			panic(err)
		}
		blocks = append(blocks, &b)
	}
	return blocks
}

func LoadGenesis(path string) core.Genesis {
	contents, err := ioutil.ReadFile(path)
	if err != nil {
		panic(fmt.Errorf("can't to read genesis file: %v", err))
	}
	var genesis core.Genesis
	if err := json.Unmarshal(contents, &genesis); err != nil {
		panic(fmt.Errorf("can't parse genesis JSON: %v", err))
	}
	return genesis
}

func LoadGenesisBlock(path string) *types.Block {
	genesis := LoadGenesis(path)
	return genesis.ToBlock()
}

func GenesisStartOption(genesis *core.Genesis) (hivesim.StartOption, error) {
	out, err := json.Marshal(genesis)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize genesis state: %v", err)
	}
	return hivesim.WithDynamicFile("/genesis.json", bytesSource(out)), nil
}

func CalculateTotalDifficulty(genesis core.Genesis, chain types.Blocks, lastBlock uint64) *big.Int {
	result := new(big.Int).Set(genesis.Difficulty)
	for _, b := range chain {
		result.Add(result, b.Difficulty())
		if lastBlock != 0 && lastBlock == b.NumberU64() {
			break
		}
	}
	return result
}

// TTD is the value specified in the test.Spec + Genesis.Difficulty
func CalculateRealTTD(g *core.Genesis, ttdValue int64) int64 {
	return g.Difficulty.Int64() + ttdValue
}

var (
	// Time between checks of a client reaching Terminal Total Difficulty
	TTDCheckPeriod = time.Second * 1
)

// TTD Check Methods
func CheckTTD(ec client.EngineClient, ctx context.Context) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, globals.RPCTimeout)
	defer cancel()
	td, err := ec.GetTotalDifficulty(ctx)
	if err == nil {
		return td.Cmp(ec.TerminalTotalDifficulty()) >= 0, nil
	}
	return false, err
}

type WaitTTDResponse struct {
	ec  client.EngineClient
	err error
}

// Wait until the TTD is reached by a single client.
func WaitForTTD(ec client.EngineClient, wg *sync.WaitGroup, done chan<- WaitTTDResponse, ctx context.Context) {
	defer wg.Done()
	for {
		select {
		case <-time.After(TTDCheckPeriod):
			ttdReached, err := CheckTTD(ec, ctx)
			if err == nil && ttdReached {
				select {
				case done <- WaitTTDResponse{
					ec:  ec,
					err: err,
				}:
				case <-ctx.Done():
				}
				return
			}
		case <-ctx.Done():
			return
		}
	}
}

// Wait until the TTD is reached by a single client with a timeout.
// Returns true if the TTD has been reached, false when timeout occurred.
func WaitForTTDWithTimeout(ec client.EngineClient, ctx context.Context) error {
	for {
		select {
		case <-time.After(TTDCheckPeriod):
			ttdReached, err := CheckTTD(ec, ctx)
			if err != nil {
				return err
			}
			if ttdReached {
				return nil
			}
		case <-ctx.Done():
			return fmt.Errorf("timeout reached")
		}
	}
}

// Wait until the TTD is reached by any of the engine clients
func WaitAnyClientForTTD(ecs []client.EngineClient, testCtx context.Context) (client.EngineClient, error) {
	var wg sync.WaitGroup
	defer wg.Wait()
	done := make(chan WaitTTDResponse)
	cancel := make(chan interface{})
	defer close(cancel)
	for _, ec := range ecs {
		wg.Add(1)
		ctx, cancel := context.WithCancel(testCtx)
		defer cancel()
		go WaitForTTD(ec, &wg, done, ctx)
	}
	select {
	case r := <-done:
		return r.ec, r.err
	case <-testCtx.Done():
		return nil, testCtx.Err()
	}
}

type TestTransactionType int

const (
	UnspecifiedTransactionType TestTransactionType = iota
	LegacyTxOnly
	DynamicFeeTxOnly
)

type TransactionCreator interface {
	MakeTransaction(nonce uint64) (*types.Transaction, error)
}

type BaseTransactionCreator struct {
	Recipient  *common.Address
	GasLimit   uint64
	Amount     *big.Int
	Payload    []byte
	TxType     TestTransactionType
	PrivateKey *ecdsa.PrivateKey
}

func (tc *BaseTransactionCreator) MakeTransaction(nonce uint64) (*types.Transaction, error) {
	var newTxData types.TxData

	var txTypeToUse int
	switch tc.TxType {
	case UnspecifiedTransactionType:
		// Test case has no specific type of transaction to use.
		// Select the type of tx based on the nonce.
		switch nonce % 2 {
		case 0:
			txTypeToUse = types.LegacyTxType
		case 1:
			txTypeToUse = types.DynamicFeeTxType
		}
	case LegacyTxOnly:
		txTypeToUse = types.LegacyTxType
	case DynamicFeeTxOnly:
		txTypeToUse = types.DynamicFeeTxType
	}

	// Build the transaction depending on the specified type
	switch txTypeToUse {
	case types.LegacyTxType:
		newTxData = &types.LegacyTx{
			Nonce:    nonce,
			To:       tc.Recipient,
			Value:    tc.Amount,
			Gas:      tc.GasLimit,
			GasPrice: globals.GasPrice,
			Data:     tc.Payload,
		}
	case types.DynamicFeeTxType:
		gasFeeCap := new(big.Int).Set(globals.GasPrice)
		gasTipCap := new(big.Int).Set(globals.GasTipPrice)
		newTxData = &types.DynamicFeeTx{
			Nonce:     nonce,
			Gas:       tc.GasLimit,
			GasTipCap: gasTipCap,
			GasFeeCap: gasFeeCap,
			To:        tc.Recipient,
			Value:     tc.Amount,
			Data:      tc.Payload,
		}
	}

	tx := types.NewTx(newTxData)
	key := tc.PrivateKey
	if key == nil {
		key = globals.VaultKey
	}
	signedTx, err := types.SignTx(tx, types.NewLondonSigner(globals.ChainID), key)
	if err != nil {
		return nil, err
	}
	return signedTx, nil
}

// Create a contract filled with zeros without going over the specified GasLimit
type BigContractTransactionCreator struct {
	BaseTransactionCreator
}

func (tc *BigContractTransactionCreator) MakeTransaction(nonce uint64) (*types.Transaction, error) {
	// Total GAS: Gtransaction == 21000, Gcreate == 32000, Gcodedeposit == 200
	contractLength := uint64(0)
	if tc.GasLimit > (21000 + 32000) {
		contractLength = (tc.GasLimit - 21000 - 32000) / 200
		if contractLength >= 1 {
			// Reduce by 1 to guarantee using less gas than requested
			contractLength -= 1
		}
	}
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, contractLength)

	tc.Payload = []byte{
		0x67, // PUSH8
	}
	tc.Payload = append(tc.Payload, buf...) // Size of the contract in byte length
	tc.Payload = append(tc.Payload, 0x38)   // CODESIZE == 0x00
	tc.Payload = append(tc.Payload, 0xF3)   // RETURN(offset, length)
	if tc.Recipient != nil {
		panic("invalid configuration for big contract tx creator")
	}
	return tc.BaseTransactionCreator.MakeTransaction(nonce)
}

// Create a tx with the specified initcode length (all zeros)
type BigInitcodeTransactionCreator struct {
	BaseTransactionCreator
	InitcodeLength int
	PadByte        uint8
	Initcode       []byte
}

func (tc *BigInitcodeTransactionCreator) MakeTransaction(nonce uint64) (*types.Transaction, error) {
	// This method caches the payload with the crafted initcode after first execution.
	if tc.Payload == nil {
		// Prepare initcode payload
		if tc.Initcode != nil {
			if len(tc.Initcode) > tc.InitcodeLength {
				panic(fmt.Errorf("invalid initcode (too big)"))
			}
			tc.Payload = tc.Initcode
		} else {
			tc.Payload = []byte{}
		}

		for {
			if len(tc.Payload) == tc.InitcodeLength {
				break
			}
			tc.Payload = append(tc.Payload, tc.PadByte)
		}
	}
	if tc.Recipient != nil {
		panic("invalid configuration for big contract tx creator")
	}
	return tc.BaseTransactionCreator.MakeTransaction(nonce)
}

// Determines if the error we got from sending the raw tx is because the client
// already knew the tx (might happen if we produced a re-org where the tx was
// unwind back into the txpool)
func SentTxAlreadyKnown(err error) bool {
	return strings.Contains(err.Error(), "already known") || strings.Contains(err.Error(), "already in the TxPool") ||
		strings.Contains(err.Error(), "AlreadyKnown")
}

func SendNextTransaction(testCtx context.Context, node client.EngineClient, txCreator TransactionCreator) (*types.Transaction, error) {
	nonce, err := node.GetNextAccountNonce(testCtx, globals.VaultAccountAddress)
	if err != nil {
		return nil, err
	}
	tx, err := txCreator.MakeTransaction(nonce)
	if err != nil {
		return nil, err
	}
	for {
		ctx, cancel := context.WithTimeout(testCtx, globals.RPCTimeout)
		defer cancel()
		err := node.SendTransaction(ctx, tx)
		if err == nil {
			return tx, nil
		} else if SentTxAlreadyKnown(err) {
			return tx, nil
		}
		select {
		case <-time.After(time.Second):
		case <-testCtx.Done():
			return nil, testCtx.Err()
		}
	}
}

func SendNextTransactions(testCtx context.Context, node client.EngineClient, txCreator TransactionCreator, txCount uint64) ([]*types.Transaction, error) {
	var err error
	nonce, err := node.GetNextAccountNonce(testCtx, globals.VaultAccountAddress)
	if err != nil {
		return nil, err
	}
	txs := make([]*types.Transaction, txCount)
	for i := range txs {
		txs[i], err = txCreator.MakeTransaction(nonce)
		if err != nil {
			return nil, err
		}
		nonce++
	}
	ctx, cancel := context.WithTimeout(testCtx, globals.RPCTimeout)
	defer cancel()
	errs := node.SendTransactions(ctx, txs)
	for _, err := range errs {
		if err != nil && !SentTxAlreadyKnown(err) {
			return txs, err
		}
	}
	node.UpdateNonce(testCtx, globals.VaultAccountAddress, nonce+txCount)
	return txs, nil
}
