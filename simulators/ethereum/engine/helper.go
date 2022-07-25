package main

import (
	"bytes"
	"context"
	"crypto/ecdsa"
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
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/ethereum/go-ethereum/trie"
	"github.com/ethereum/hive/hivesim"
)

// default timeout for RPC calls
var rpcTimeout = 10 * time.Second

// From ethereum/rpc:

// loggingRoundTrip writes requests and responses to the test log.
type loggingRoundTrip struct {
	t     *hivesim.T
	hc    *hivesim.Client
	inner http.RoundTripper
}

func (rt *loggingRoundTrip) RoundTrip(req *http.Request) (*http.Response, error) {
	// Read and log the request body.
	reqBytes, err := ioutil.ReadAll(req.Body)
	req.Body.Close()
	if err != nil {
		return nil, err
	}
	rt.t.Logf(">> (%s) %s", rt.hc.Container, bytes.TrimSpace(reqBytes))
	reqCopy := *req
	reqCopy.Body = ioutil.NopCloser(bytes.NewReader(reqBytes))

	// Do the round trip.
	resp, err := rt.inner.RoundTrip(&reqCopy)
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
	rt.t.Logf("<< (%s) %s", rt.hc.Container, bytes.TrimSpace(respBytes))
	return &respCopy, nil
}

func TransactionInPayload(payload *ExecutableDataV1, tx *types.Transaction) bool {
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
			customData.ChainID = chainID
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
func customizePayload(basePayload *ExecutableDataV1, customData *CustomPayloadData) (*ExecutableDataV1, error) {
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
		MixDigest:   basePayload.PrevRandao,
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
	return &ExecutableDataV1{
		ParentHash:    customPayloadHeader.ParentHash,
		FeeRecipient:  customPayloadHeader.Coinbase,
		StateRoot:     customPayloadHeader.Root,
		ReceiptsRoot:  customPayloadHeader.ReceiptHash,
		LogsBloom:     customPayloadHeader.Bloom[:],
		PrevRandao:    customPayloadHeader.MixDigest,
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
func generateInvalidPayload(basePayload *ExecutableDataV1, payloadField InvalidPayloadField) (*ExecutableDataV1, error) {

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
			modifiedSignature.R = modifiedSignature.R.Sub(modifiedSignature.R, big1)
			customTxData.Signature = &modifiedSignature
		case InvalidTransactionNonce:
			customNonce := baseTx.Nonce() - 1
			customTxData.Nonce = &customNonce
		case InvalidTransactionGas:
			customGas := uint64(0)
			customTxData.Gas = &customGas
		case InvalidTransactionGasPrice:
			customTxData.GasPriceOrGasFeeCap = big0
		case InvalidTransactionGasTipPrice:
			invalidGasTip := new(big.Int).Set(gasTipPrice)
			invalidGasTip.Mul(invalidGasTip, big.NewInt(2))
			customTxData.GasTipCap = invalidGasTip
		case InvalidTransactionValue:
			// Vault account initially has 0x123450000000000000000, so this value should overflow
			customTxData.Value, _ = hexutil.DecodeBig("0x123450000000000000001")
		case InvalidTransactionChainID:
			customChainID := new(big.Int).Set(chainID)
			customChainID.Add(customChainID, big1)
			customTxData.ChainID = customChainID
		}

		modifiedTx, err := customizeTransaction(&baseTx, vaultKey, &customTxData)
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

	alteredPayload, err := customizePayload(basePayload, customPayloadMod)
	if err != nil {
		return nil, err
	}

	return alteredPayload, nil
}

// Use client specific rpc methods to debug a transaction that includes the PREVRANDAO opcode
func debugPrevRandaoTransaction(ctx context.Context, c *rpc.Client, clientType string, tx *types.Transaction, expectedPrevRandao *common.Hash) error {
	switch clientType {
	case "merge-go-ethereum":
		return gethDebugPrevRandaoTransaction(ctx, c, tx, expectedPrevRandao)
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

func loadChain(path string) types.Blocks {
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

func loadGenesis(path string) core.Genesis {
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

func loadGenesisBlock(path string) *types.Block {
	genesis := loadGenesis(path)
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

// Helper structs to fetch the TotalDifficulty
type TD struct {
	TotalDifficulty *hexutil.Big `json:"totalDifficulty"`
}
type TotalDifficultyHeader struct {
	types.Header
	TD
}

func (tdh *TotalDifficultyHeader) UnmarshalJSON(data []byte) error {
	if err := json.Unmarshal(data, &tdh.Header); err != nil {
		return err
	}
	if err := json.Unmarshal(data, &tdh.TD); err != nil {
		return err
	}
	return nil
}
