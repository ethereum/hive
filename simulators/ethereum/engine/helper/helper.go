package helper

import (
	"context"
	"sync"
	"time"

	"bytes"
	"crypto/ecdsa"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"math/big"
	"math/rand"
	"net/http"
	"os"
	"strings"

	api "github.com/ethereum/go-ethereum/core/beacon"
	"github.com/ethereum/hive/simulators/ethereum/engine/client"
	"github.com/ethereum/hive/simulators/ethereum/engine/globals"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/ethereum/go-ethereum/trie"
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

type SignatureValues struct {
	V *big.Int
	R *big.Int
	S *big.Int
}

func SignatureValuesFromRaw(v *big.Int, r *big.Int, s *big.Int) SignatureValues {
	return SignatureValues{
		V: v,
		R: r,
		S: s,
	}
}

type CustomTransactionData struct {
	Nonce               *uint64
	GasPriceOrGasFeeCap *big.Int
	GasTipCap           *big.Int
	Gas                 *uint64
	To                  *common.Address
	Value               *big.Int
	Data                *[]byte
	ChainID             *big.Int
	Signature           *SignatureValues
}

func customizeTransaction(baseTransaction *types.Transaction, pk *ecdsa.PrivateKey, customData *CustomTransactionData) (*types.Transaction, error) {
	// Create a modified transaction base, from the base transaction and customData mix
	var (
		modifiedTxData types.TxData
	)

	switch baseTransaction.Type() {
	case types.LegacyTxType:
		modifiedLegacyTxBase := &types.LegacyTx{}

		if customData.Nonce != nil {
			modifiedLegacyTxBase.Nonce = *customData.Nonce
		} else {
			modifiedLegacyTxBase.Nonce = baseTransaction.Nonce()
		}
		if customData.GasPriceOrGasFeeCap != nil {
			modifiedLegacyTxBase.GasPrice = customData.GasPriceOrGasFeeCap
		} else {
			modifiedLegacyTxBase.GasPrice = baseTransaction.GasPrice()
		}
		if customData.GasTipCap != nil {
			return nil, fmt.Errorf("GasTipCap is not supported for LegacyTx type")
		}
		if customData.Gas != nil {
			modifiedLegacyTxBase.Gas = *customData.Gas
		} else {
			modifiedLegacyTxBase.Gas = baseTransaction.Gas()
		}
		if customData.To != nil {
			modifiedLegacyTxBase.To = customData.To
		} else {
			modifiedLegacyTxBase.To = baseTransaction.To()
		}
		if customData.Value != nil {
			modifiedLegacyTxBase.Value = customData.Value
		} else {
			modifiedLegacyTxBase.Value = baseTransaction.Value()
		}
		if customData.Data != nil {
			modifiedLegacyTxBase.Data = *customData.Data
		} else {
			modifiedLegacyTxBase.Data = baseTransaction.Data()
		}

		if customData.Signature != nil {
			modifiedLegacyTxBase.V = customData.Signature.V
			modifiedLegacyTxBase.R = customData.Signature.R
			modifiedLegacyTxBase.S = customData.Signature.S
		}

		modifiedTxData = modifiedLegacyTxBase

	case types.DynamicFeeTxType:
		modifiedDynamicFeeTxBase := &types.DynamicFeeTx{}

		if customData.Nonce != nil {
			modifiedDynamicFeeTxBase.Nonce = *customData.Nonce
		} else {
			modifiedDynamicFeeTxBase.Nonce = baseTransaction.Nonce()
		}
		if customData.GasPriceOrGasFeeCap != nil {
			modifiedDynamicFeeTxBase.GasFeeCap = customData.GasPriceOrGasFeeCap
		} else {
			modifiedDynamicFeeTxBase.GasFeeCap = baseTransaction.GasFeeCap()
		}
		if customData.GasTipCap != nil {
			modifiedDynamicFeeTxBase.GasTipCap = customData.GasTipCap
		} else {
			modifiedDynamicFeeTxBase.GasTipCap = baseTransaction.GasTipCap()
		}
		if customData.Gas != nil {
			modifiedDynamicFeeTxBase.Gas = *customData.Gas
		} else {
			modifiedDynamicFeeTxBase.Gas = baseTransaction.Gas()
		}
		if customData.To != nil {
			modifiedDynamicFeeTxBase.To = customData.To
		} else {
			modifiedDynamicFeeTxBase.To = baseTransaction.To()
		}
		if customData.Value != nil {
			modifiedDynamicFeeTxBase.Value = customData.Value
		} else {
			modifiedDynamicFeeTxBase.Value = baseTransaction.Value()
		}
		if customData.Data != nil {
			modifiedDynamicFeeTxBase.Data = *customData.Data
		} else {
			modifiedDynamicFeeTxBase.Data = baseTransaction.Data()
		}
		if customData.Signature != nil {
			modifiedDynamicFeeTxBase.V = customData.Signature.V
			modifiedDynamicFeeTxBase.R = customData.Signature.R
			modifiedDynamicFeeTxBase.S = customData.Signature.S
		}

		modifiedTxData = modifiedDynamicFeeTxBase

	}

	modifiedTx := types.NewTx(modifiedTxData)
	if customData.Signature == nil {
		// If a custom invalid signature was not specified, simply sign the transaction again
		if customData.ChainID == nil {
			// Use the default value if an invaild chain ID was not specified
			customData.ChainID = globals.ChainID
		}
		signer := types.NewLondonSigner(customData.ChainID)
		var err error
		if modifiedTx, err = types.SignTx(modifiedTx, signer, pk); err != nil {
			return nil, err
		}
	}
	return modifiedTx, nil
}

type CustomPayloadData struct {
	ParentHash    *common.Hash
	FeeRecipient  *common.Address
	StateRoot     *common.Hash
	ReceiptsRoot  *common.Hash
	LogsBloom     *[]byte
	PrevRandao    *common.Hash
	Number        *uint64
	GasLimit      *uint64
	GasUsed       *uint64
	Timestamp     *uint64
	ExtraData     *[]byte
	BaseFeePerGas *big.Int
	BlockHash     *common.Hash
	Transactions  *[][]byte
}

func calcTxsHash(txsBytes [][]byte) (common.Hash, error) {
	txs := make([]*types.Transaction, len(txsBytes))
	for i, bytesTx := range txsBytes {
		var currentTx types.Transaction
		err := currentTx.UnmarshalBinary(bytesTx)
		if err != nil {
			return common.Hash{}, err
		}
		txs[i] = &currentTx
	}
	return types.DeriveSha(types.Transactions(txs), trie.NewStackTrie(nil)), nil
}

// Construct a customized payload by taking an existing payload as base and mixing it CustomPayloadData
// BlockHash is calculated automatically.
func CustomizePayload(basePayload *api.ExecutableDataV1, customData *CustomPayloadData) (*api.ExecutableDataV1, error) {
	txs := basePayload.Transactions
	if customData.Transactions != nil {
		txs = *customData.Transactions
	}
	txsHash, err := calcTxsHash(txs)
	if err != nil {
		return nil, err
	}
	fmt.Printf("txsHash: %v\n", txsHash)
	// Start by filling the header with the basePayload information
	customPayloadHeader := types.Header{
		ParentHash:  basePayload.ParentHash,
		UncleHash:   types.EmptyUncleHash, // Could be overwritten
		Coinbase:    basePayload.FeeRecipient,
		Root:        basePayload.StateRoot,
		TxHash:      txsHash,
		ReceiptHash: basePayload.ReceiptsRoot,
		Bloom:       types.BytesToBloom(basePayload.LogsBloom),
		Difficulty:  big.NewInt(0), // could be overwritten
		Number:      big.NewInt(int64(basePayload.Number)),
		GasLimit:    basePayload.GasLimit,
		GasUsed:     basePayload.GasUsed,
		Time:        basePayload.Timestamp,
		Extra:       basePayload.ExtraData,
		MixDigest:   basePayload.Random,
		Nonce:       types.BlockNonce{0}, // could be overwritten
		BaseFee:     basePayload.BaseFeePerGas,
	}

	// Overwrite custom information
	if customData.ParentHash != nil {
		customPayloadHeader.ParentHash = *customData.ParentHash
	}
	if customData.FeeRecipient != nil {
		customPayloadHeader.Coinbase = *customData.FeeRecipient
	}
	if customData.StateRoot != nil {
		customPayloadHeader.Root = *customData.StateRoot
	}
	if customData.ReceiptsRoot != nil {
		customPayloadHeader.ReceiptHash = *customData.ReceiptsRoot
	}
	if customData.LogsBloom != nil {
		customPayloadHeader.Bloom = types.BytesToBloom(*customData.LogsBloom)
	}
	if customData.PrevRandao != nil {
		customPayloadHeader.MixDigest = *customData.PrevRandao
	}
	if customData.Number != nil {
		customPayloadHeader.Number = big.NewInt(int64(*customData.Number))
	}
	if customData.GasLimit != nil {
		customPayloadHeader.GasLimit = *customData.GasLimit
	}
	if customData.GasUsed != nil {
		customPayloadHeader.GasUsed = *customData.GasUsed
	}
	if customData.Timestamp != nil {
		customPayloadHeader.Time = *customData.Timestamp
	}
	if customData.ExtraData != nil {
		customPayloadHeader.Extra = *customData.ExtraData
	}
	if customData.BaseFeePerGas != nil {
		customPayloadHeader.BaseFee = customData.BaseFeePerGas
	}

	// Return the new payload
	return &api.ExecutableDataV1{
		ParentHash:    customPayloadHeader.ParentHash,
		FeeRecipient:  customPayloadHeader.Coinbase,
		StateRoot:     customPayloadHeader.Root,
		ReceiptsRoot:  customPayloadHeader.ReceiptHash,
		LogsBloom:     customPayloadHeader.Bloom[:],
		Random:        customPayloadHeader.MixDigest,
		Number:        customPayloadHeader.Number.Uint64(),
		GasLimit:      customPayloadHeader.GasLimit,
		GasUsed:       customPayloadHeader.GasUsed,
		Timestamp:     customPayloadHeader.Time,
		ExtraData:     customPayloadHeader.Extra,
		BaseFeePerGas: customPayloadHeader.BaseFee,
		BlockHash:     customPayloadHeader.Hash(),
		Transactions:  txs,
	}, nil
}

func (customData *CustomPayloadData) String() string {
	customFieldsList := make([]string, 0)
	if customData.ParentHash != nil {
		customFieldsList = append(customFieldsList, fmt.Sprintf("ParentHash=%s", customData.ParentHash.String()))
	}
	if customData.FeeRecipient != nil {
		customFieldsList = append(customFieldsList, fmt.Sprintf("Coinbase=%s", customData.FeeRecipient.String()))
	}
	if customData.StateRoot != nil {
		customFieldsList = append(customFieldsList, fmt.Sprintf("StateRoot=%s", customData.StateRoot.String()))
	}
	if customData.ReceiptsRoot != nil {
		customFieldsList = append(customFieldsList, fmt.Sprintf("ReceiptsRoot=%s", customData.ReceiptsRoot.String()))
	}
	if customData.LogsBloom != nil {
		customFieldsList = append(customFieldsList, fmt.Sprintf("LogsBloom=%v", types.BytesToBloom(*customData.LogsBloom)))
	}
	if customData.PrevRandao != nil {
		customFieldsList = append(customFieldsList, fmt.Sprintf("PrevRandao=%s", customData.PrevRandao.String()))
	}
	if customData.Number != nil {
		customFieldsList = append(customFieldsList, fmt.Sprintf("Number=%d", *customData.Number))
	}
	if customData.GasLimit != nil {
		customFieldsList = append(customFieldsList, fmt.Sprintf("GasLimit=%d", *customData.GasLimit))
	}
	if customData.GasUsed != nil {
		customFieldsList = append(customFieldsList, fmt.Sprintf("GasUsed=%d", *customData.GasUsed))
	}
	if customData.Timestamp != nil {
		customFieldsList = append(customFieldsList, fmt.Sprintf("Timestamp=%d", *customData.Timestamp))
	}
	if customData.ExtraData != nil {
		customFieldsList = append(customFieldsList, fmt.Sprintf("ExtraData=%v", *customData.ExtraData))
	}
	if customData.BaseFeePerGas != nil {
		customFieldsList = append(customFieldsList, fmt.Sprintf("BaseFeePerGas=%s", customData.BaseFeePerGas.String()))
	}
	if customData.Transactions != nil {
		customFieldsList = append(customFieldsList, fmt.Sprintf("Transactions=%v", customData.Transactions))
	}
	return strings.Join(customFieldsList, ", ")
}

type InvalidPayloadField string

const (
	InvalidParentHash             InvalidPayloadField = "ParentHash"
	InvalidStateRoot                                  = "StateRoot"
	InvalidReceiptsRoot                               = "ReceiptsRoot"
	InvalidNumber                                     = "Number"
	InvalidGasLimit                                   = "GasLimit"
	InvalidGasUsed                                    = "GasUsed"
	InvalidTimestamp                                  = "Timestamp"
	InvalidPrevRandao                                 = "PrevRandao"
	RemoveTransaction                                 = "Incomplete Transactions"
	InvalidTransactionSignature                       = "Transaction Signature"
	InvalidTransactionNonce                           = "Transaction Nonce"
	InvalidTransactionGas                             = "Transaction Gas"
	InvalidTransactionGasPrice                        = "Transaction GasPrice"
	InvalidTransactionGasTipPrice                     = "Transaction GasTipCapPrice"
	InvalidTransactionValue                           = "Transaction Value"
	InvalidTransactionChainID                         = "Transaction ChainID"
)

// This function generates an invalid payload by taking a base payload and modifying the specified field such that it ends up being invalid.
// One small consideration is that the payload needs to contain transactions and specially transactions using the PREVRANDAO opcode for all the fields to be compatible with this function.
func GenerateInvalidPayload(basePayload *api.ExecutableDataV1, payloadField InvalidPayloadField) (*api.ExecutableDataV1, error) {

	var customPayloadMod *CustomPayloadData
	switch payloadField {
	case InvalidParentHash:
		modParentHash := basePayload.ParentHash
		modParentHash[common.HashLength-1] = byte(255 - modParentHash[common.HashLength-1])
		customPayloadMod = &CustomPayloadData{
			ParentHash: &modParentHash,
		}
	case InvalidStateRoot:
		modStateRoot := basePayload.StateRoot
		modStateRoot[common.HashLength-1] = byte(255 - modStateRoot[common.HashLength-1])
		customPayloadMod = &CustomPayloadData{
			StateRoot: &modStateRoot,
		}
	case InvalidReceiptsRoot:
		modReceiptsRoot := basePayload.ReceiptsRoot
		modReceiptsRoot[common.HashLength-1] = byte(255 - modReceiptsRoot[common.HashLength-1])
		customPayloadMod = &CustomPayloadData{
			ReceiptsRoot: &modReceiptsRoot,
		}
	case InvalidNumber:
		modNumber := basePayload.Number - 1
		customPayloadMod = &CustomPayloadData{
			Number: &modNumber,
		}
	case InvalidGasLimit:
		modGasLimit := basePayload.GasLimit * 2
		customPayloadMod = &CustomPayloadData{
			GasLimit: &modGasLimit,
		}
	case InvalidGasUsed:
		modGasUsed := basePayload.GasUsed - 1
		customPayloadMod = &CustomPayloadData{
			GasUsed: &modGasUsed,
		}
	case InvalidTimestamp:
		modTimestamp := basePayload.Timestamp - 1
		customPayloadMod = &CustomPayloadData{
			Timestamp: &modTimestamp,
		}
	case InvalidPrevRandao:
		// This option potentially requires a transaction that uses the PREVRANDAO opcode.
		// Otherwise the payload will still be valid.
		modPrevRandao := common.Hash{}
		rand.Read(modPrevRandao[:])
		customPayloadMod = &CustomPayloadData{
			PrevRandao: &modPrevRandao,
		}
	case RemoveTransaction:
		emptyTxs := make([][]byte, 0)
		customPayloadMod = &CustomPayloadData{
			Transactions: &emptyTxs,
		}
	case InvalidTransactionSignature,
		InvalidTransactionNonce,
		InvalidTransactionGas,
		InvalidTransactionGasPrice,
		InvalidTransactionGasTipPrice,
		InvalidTransactionValue,
		InvalidTransactionChainID:

		if len(basePayload.Transactions) == 0 {
			return nil, fmt.Errorf("No transactions available for modification")
		}
		var baseTx types.Transaction
		if err := baseTx.UnmarshalBinary(basePayload.Transactions[0]); err != nil {
			return nil, err
		}
		var customTxData CustomTransactionData
		switch payloadField {
		case InvalidTransactionSignature:
			modifiedSignature := SignatureValuesFromRaw(baseTx.RawSignatureValues())
			modifiedSignature.R = modifiedSignature.R.Sub(modifiedSignature.R, common.Big1)
			customTxData.Signature = &modifiedSignature
		case InvalidTransactionNonce:
			customNonce := baseTx.Nonce() - 1
			customTxData.Nonce = &customNonce
		case InvalidTransactionGas:
			customGas := uint64(0)
			customTxData.Gas = &customGas
		case InvalidTransactionGasPrice:
			customTxData.GasPriceOrGasFeeCap = common.Big0
		case InvalidTransactionGasTipPrice:
			invalidGasTip := new(big.Int).Set(globals.GasTipPrice)
			invalidGasTip.Mul(invalidGasTip, big.NewInt(2))
			customTxData.GasTipCap = invalidGasTip
		case InvalidTransactionValue:
			// Vault account initially has 0x123450000000000000000, so this value should overflow
			customTxData.Value, _ = hexutil.DecodeBig("0x123450000000000000001")
		case InvalidTransactionChainID:
			customChainID := new(big.Int).Set(globals.ChainID)
			customChainID.Add(customChainID, common.Big1)
			customTxData.ChainID = customChainID
		}

		modifiedTx, err := customizeTransaction(&baseTx, globals.VaultKey, &customTxData)
		if err != nil {
			return nil, err
		}

		modifiedTxBytes, err := modifiedTx.MarshalBinary()
		if err != nil {
			return nil, err
		}
		modifiedTransactions := [][]byte{
			modifiedTxBytes,
		}
		customPayloadMod = &CustomPayloadData{
			Transactions: &modifiedTransactions,
		}
	}

	if customPayloadMod == nil {
		return nil, fmt.Errorf("Invalid payload field to corrupt: %s", payloadField)
	}

	alteredPayload, err := CustomizePayload(basePayload, customPayloadMod)
	if err != nil {
		return nil, err
	}

	return alteredPayload, nil
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
	return genesis.ToBlock(nil)
}

func CalculateTotalDifficulty(genesis core.Genesis, chain types.Blocks, lastBlock uint64) *big.Int {
	result := new(big.Int).Set(genesis.Difficulty)
	for _, b := range chain {
		if lastBlock != 0 && lastBlock == b.NumberU64() {
			break
		}
		result.Add(result, b.Difficulty())
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
			return ctx.Err()
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
