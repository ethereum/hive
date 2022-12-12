package helper

import (
	"context"
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

	api "github.com/ethereum/go-ethereum/core/beacon"
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
type LoggingRoundTrip struct {
	T     *hivesim.T
	Hc    *hivesim.Client
	Inner http.RoundTripper
}

func (rt *LoggingRoundTrip) RoundTrip(req *http.Request) (*http.Response, error) {
	// Read and log the request body.
	reqBytes, err := ioutil.ReadAll(req.Body)
	req.Body.Close()
	if err != nil {
		return nil, err
	}
	rt.T.Logf(">> (%s) %s", rt.Hc.Container, bytes.TrimSpace(reqBytes))
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
	rt.T.Logf("<< (%s) %s", rt.Hc.Container, bytes.TrimSpace(respBytes))
	return &respCopy, nil
}

type InvalidPayloadBlockField string

const (
	InvalidParentHash             InvalidPayloadBlockField = "ParentHash"
	InvalidStateRoot                                       = "StateRoot"
	InvalidReceiptsRoot                                    = "ReceiptsRoot"
	InvalidNumber                                          = "Number"
	InvalidGasLimit                                        = "GasLimit"
	InvalidGasUsed                                         = "GasUsed"
	InvalidTimestamp                                       = "Timestamp"
	InvalidPrevRandao                                      = "PrevRandao"
	InvalidOmmers                                          = "Ommers"
	RemoveTransaction                                      = "Incomplete Transactions"
	InvalidTransactionSignature                            = "Transaction Signature"
	InvalidTransactionNonce                                = "Transaction Nonce"
	InvalidTransactionGas                                  = "Transaction Gas"
	InvalidTransactionGasPrice                             = "Transaction GasPrice"
	InvalidTransactionGasTipPrice                          = "Transaction GasTipCapPrice"
	InvalidTransactionValue                                = "Transaction Value"
	InvalidTransactionChainID                              = "Transaction ChainID"
)

func TransactionInPayload(payload *api.ExecutableDataV1, tx *types.Transaction) bool {
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
				return errors.New(fmt.Sprintf("No information after PREVRANDAO operation"))
			}
			prevRandaoFound = true
			stack := *(er.StructLogs[i+1].Stack)
			if len(stack) < 1 {
				return errors.New(fmt.Sprintf("Invalid stack after PREVRANDAO operation: %v", l.Stack))
			}
			stackHash := common.HexToHash(stack[0])
			if stackHash != *expectedPrevRandao {
				return errors.New(fmt.Sprintf("Invalid stack after PREVRANDAO operation, %v != %v", stackHash, expectedPrevRandao))
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
func CalculateRealTTD(genesisPath string, ttdValue int64) int64 {
	g := LoadGenesis(genesisPath)
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
			return fmt.Errorf("Timeout reached")
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

func MakeTransaction(nonce uint64, recipient *common.Address, gasLimit uint64, amount *big.Int, payload []byte, txType TestTransactionType) (*types.Transaction, error) {
	var newTxData types.TxData

	var txTypeToUse int
	switch txType {
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
			To:       recipient,
			Value:    amount,
			Gas:      gasLimit,
			GasPrice: globals.GasPrice,
			Data:     payload,
		}
	case types.DynamicFeeTxType:
		gasFeeCap := new(big.Int).Set(globals.GasPrice)
		gasTipCap := new(big.Int).Set(globals.GasTipPrice)
		newTxData = &types.DynamicFeeTx{
			Nonce:     nonce,
			Gas:       gasLimit,
			GasTipCap: gasTipCap,
			GasFeeCap: gasFeeCap,
			To:        recipient,
			Value:     amount,
			Data:      payload,
		}
	}

	tx := types.NewTx(newTxData)
	signedTx, err := types.SignTx(tx, types.NewLondonSigner(globals.ChainID), globals.VaultKey)
	if err != nil {
		return nil, err
	}
	return signedTx, nil
}

// Determines if the error we got from sending the raw tx is because the client
// already knew the tx (might happen if we produced a re-org where the tx was
// unwind back into the txpool)
func SentTxAlreadyKnown(err error) bool {
	return strings.Contains(err.Error(), "already known")
}

func SendNextTransaction(testCtx context.Context, node client.EngineClient, recipient common.Address, amount *big.Int, payload []byte, txType TestTransactionType) (*types.Transaction, error) {
	nonce, err := node.GetNextAccountNonce(testCtx, globals.VaultAccountAddress)
	if err != nil {
		return nil, err
	}
	tx, err := MakeTransaction(nonce, &recipient, 75000, amount, payload, txType)
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

// Method that attempts to create a contract filled with zeros without going over the specified gasLimit
func MakeBigContractTransaction(nonce uint64, gasLimit uint64, txType TestTransactionType) (*types.Transaction, error) {
	// Total GAS: Gtransaction == 21000, Gcreate == 32000, Gcodedeposit == 200
	contractLength := uint64(0)
	if gasLimit > (21000 + 32000) {
		contractLength = (gasLimit - 21000 - 32000) / 200
		if contractLength >= 1 {
			// Reduce by 1 to guarantee using less gas than requested
			contractLength -= 1
		}
	}
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, contractLength)

	initCode := []byte{
		0x67, // PUSH8
	}
	initCode = append(initCode, buf...) // Size of the contract in byte length
	initCode = append(initCode, 0x38)   // CODESIZE == 0x00
	initCode = append(initCode, 0xF3)   // RETURN(offset, length)

	return MakeTransaction(nonce, nil, gasLimit, common.Big0, initCode, txType)
}

func SendNextBigContractTransaction(testCtx context.Context, node client.EngineClient, gasLimit uint64, txType TestTransactionType) (*types.Transaction, error) {
	nonce, err := node.GetNextAccountNonce(testCtx, globals.VaultAccountAddress)
	if err != nil {
		return nil, err
	}
	tx, err := MakeBigContractTransaction(nonce, gasLimit, txType)
	if err != nil {
		return nil, err
	}
	for {
		ctx, cancel := context.WithTimeout(testCtx, globals.RPCTimeout)
		defer cancel()
		err := node.SendTransaction(ctx, tx)
		if err == nil {
			return tx, nil
		}
		select {
		case <-time.After(time.Second):
		case <-testCtx.Done():
			return nil, testCtx.Err()
		}
	}
}
