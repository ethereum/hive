// Package suite_focil contains Engine API tests for EIP-7805 FOCIL.
package suite_focil

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/ethereum/hive/hivesim"
	"github.com/ethereum/hive/simulators/ethereum/engine/globals"
	"github.com/ethereum/hive/simulators/ethereum/engine/helper"
)

//go:embed genesis.json
var focilGenesisJSON []byte

const (
	maxBytesPerInclusionList = 8192
	rpcTimeout               = 20 * time.Second
)

var (
	zeroHash    common.Hash
	prevRandao  = common.HexToHash("0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef")
	feeReceiver = common.HexToAddress("0x0000000000000000000000000000000000000f0c")
	txTo        = common.HexToAddress("0x000000000000000000000000000000000000c0de")
)

type forkchoiceState struct {
	HeadBlockHash      common.Hash `json:"headBlockHash"`
	SafeBlockHash      common.Hash `json:"safeBlockHash"`
	FinalizedBlockHash common.Hash `json:"finalizedBlockHash"`
}

type payloadAttributesV5 struct {
	Timestamp                 hexutil.Uint64      `json:"timestamp"`
	Random                    common.Hash         `json:"prevRandao"`
	SuggestedFeeRecipient     common.Address      `json:"suggestedFeeRecipient"`
	Withdrawals               []*types.Withdrawal `json:"withdrawals"`
	ParentBeaconBlockRoot     common.Hash         `json:"parentBeaconBlockRoot"`
	SlotNumber                hexutil.Uint64      `json:"slotNumber"`
	InclusionListTransactions []hexutil.Bytes     `json:"inclusionListTransactions"`
}

type payloadStatus struct {
	Status          string       `json:"status"`
	LatestValidHash *common.Hash `json:"latestValidHash"`
	ValidationError *string      `json:"validationError"`
}

type forkchoiceResponse struct {
	PayloadStatus payloadStatus `json:"payloadStatus"`
	PayloadID     *string       `json:"payloadId"`
}

type payloadEnvelope struct {
	ExecutionPayload  json.RawMessage `json:"executionPayload"`
	ExecutionRequests []hexutil.Bytes `json:"executionRequests"`
}

type payloadHeader struct {
	BlockHash common.Hash    `json:"blockHash"`
	Timestamp hexutil.Uint64 `json:"timestamp"`
}

type ethBlock struct {
	Hash      common.Hash    `json:"hash"`
	Number    hexutil.Uint64 `json:"number"`
	Timestamp hexutil.Uint64 `json:"timestamp"`
}

type inclusionListCase struct {
	name        string
	description string
	txs         []hexutil.Bytes
}

func ClientParameters() hivesim.Params {
	return globals.DefaultClientEnv.
		Set("HIVE_TERMINAL_TOTAL_DIFFICULTY_PASSED", "1").
		Set("HIVE_TERMINAL_TOTAL_DIFFICULTY", "0").
		Set("HIVE_MERGE_BLOCK_ID", "0").
		Set("HIVE_SHANGHAI_TIMESTAMP", "0").
		Set("HIVE_CANCUN_TIMESTAMP", "0").
		Set("HIVE_PRAGUE_TIMESTAMP", "0").
		Set("HIVE_OSAKA_TIMESTAMP", "0").
		Set("HIVE_AMSTERDAM_TIMESTAMP", "0").
		Set("HIVE_BOGOTA_TIMESTAMP", "0")
}

// RunLoader is the suite entry point. It builds a FOCIL-specific genesis with
// the Prague-era system contracts (EIP-2935/6110/7002/7251 + EIP-4788) pre-deployed
// and runs the FOCIL tests against each eth1 client with that genesis. Owning the
// genesis here keeps the suite self-contained under suites/focil/.
func RunLoader(t *hivesim.T) {
	var genesis core.Genesis
	if err := json.Unmarshal(focilGenesisJSON, &genesis); err != nil {
		t.Fatalf("focil: parse embedded genesis: %v", err)
	}
	fundTestAccounts(t, &genesis)
	startOpt, err := helper.GenesisStartOption(&genesis)
	if err != nil {
		t.Fatalf("focil: build genesis start option: %v", err)
	}
	params := ClientParameters()

	clients, err := t.Sim.ClientTypes()
	if err != nil {
		t.Fatalf("focil: list clients: %v", err)
	}
	for _, ct := range clients {
		if !ct.HasRole("eth1") {
			continue
		}
		ct := ct
		t.Run(hivesim.TestSpec{
			Name:        fmt.Sprintf("engine-focil tests (%s)", ct.Name),
			Description: "FOCIL Engine API inclusion-list handling tests.",
			AlwaysRun:   true,
			Run: func(st *hivesim.T) {
				c := st.StartClient(ct.Name, params, startOpt)
				Run(st, c)
			},
		})
	}
}

func fundTestAccounts(t *hivesim.T, genesis *core.Genesis) {
	if genesis.Alloc == nil {
		genesis.Alloc = core.GenesisAlloc{}
	}
	balance, ok := new(big.Int).SetString("123450000000000000000", 16)
	if !ok {
		t.Fatal("focil: failed to parse test account balance")
	}
	for _, testAcc := range globals.TestAccounts {
		account := genesis.Alloc[testAcc.GetAddress()]
		account.Balance = new(big.Int).Set(balance)
		genesis.Alloc[testAcc.GetAddress()] = account
	}
}

func Run(t *hivesim.T, c *hivesim.Client) {
	t.Run(hivesim.TestSpec{
		Name:        fmt.Sprintf("engine_getInclusionListV1 unknown parent (%s)", c.Type),
		Description: "Requests an inclusion list for a missing parent block and verifies the Engine API Unknown parent error.",
		Run:         func(st *hivesim.T) { testGetInclusionListUnknownParent(st, c) },
	})

	t.Run(hivesim.TestSpec{
		Name:        fmt.Sprintf("engine_getInclusionListV1 response validation (%s)", c.Type),
		Description: "Requests an inclusion list from the client and verifies the returned byte list is size-bounded, non-blob, and safe to deserialize.",
		Run:         func(st *hivesim.T) { testGetInclusionList(st, c) },
	})

	t.Run(hivesim.TestSpec{
		Name:        fmt.Sprintf("engine_forkchoiceUpdatedV5 accepts IL larger than MAX_BYTES_PER_INCLUSION_LIST (%s)", c.Type),
		Description: "Sends a 10 KiB inclusion list via engine_forkchoiceUpdatedV5 and verifies it is accepted; the size cap only applies to engine_getInclusionListV1 responses.",
		Run:         func(st *hivesim.T) { testForkchoiceUpdatedAcceptsLargeIL(st, c) },
	})

	for _, tc := range makeInclusionListCases(t) {
		tc := tc
		t.Run(hivesim.TestSpec{
			Name:        fmt.Sprintf("FOCIL IL bytes: %s (%s)", tc.name, c.Type),
			Description: tc.description,
			Run: func(st *hivesim.T) {
				testInclusionListBytes(st, c, tc.txs)
			},
		})
	}
}

func testGetInclusionListUnknownParent(t *hivesim.T, c *hivesim.Client) {
	parent := common.HexToHash("0x000000000000000000000000000000000000000000000000000000000000f0c1")

	ctx, cancel := context.WithTimeout(context.Background(), rpcTimeout)
	defer cancel()
	var il []hexutil.Bytes
	err := c.EngineAPI().CallContext(ctx, &il, "engine_getInclusionListV1", parent)
	if err == nil {
		t.Fatal("engine_getInclusionListV1 accepted an unknown parent")
	}
	rpcErr, ok := err.(rpc.Error)
	if !ok {
		t.Fatalf("engine_getInclusionListV1 returned non-RPC error for unknown parent: %v", err)
	}
	if rpcErr.ErrorCode() != *globals.UNKNOWN_PARENT {
		t.Fatalf("engine_getInclusionListV1 returned error code %d for unknown parent, want %d: %v", rpcErr.ErrorCode(), *globals.UNKNOWN_PARENT, err)
	}
}

func testGetInclusionList(t *hivesim.T, c *hivesim.Client) {
	sendMempoolTransactions(t, c)
	parent := latestBlock(t, c).Hash

	var il []hexutil.Bytes
	callEngine(t, c, &il, "engine_getInclusionListV1", parent)
	validateInclusionListBytes(t, il)
	t.Logf("engine_getInclusionListV1 returned %d transaction byte strings", len(il))
}

func testForkchoiceUpdatedAcceptsLargeIL(t *hivesim.T, c *hivesim.Client) {
	const ilSize = 10 * 1024
	il := []hexutil.Bytes{bytes.Repeat([]byte{0x42}, ilSize)}
	if encodedLen := inclusionListRLPSize(t, il); encodedLen <= maxBytesPerInclusionList {
		t.Fatalf("test vector RLP size is %d bytes, expected > %d", encodedLen, maxBytesPerInclusionList)
	}

	parent := latestBlock(t, c)
	timestamp := uint64(parent.Timestamp) + 1
	fcu := forkchoiceUpdatedV5(t, c, parent.Hash, timestamp, il)
	if fcu.PayloadStatus.Status == "INVALID" || fcu.PayloadStatus.Status == "INCLUSION_LIST_UNSATISFIED" {
		t.Fatalf("engine_forkchoiceUpdatedV5 rejected a %d-byte inclusion list with status %s", ilSize, fcu.PayloadStatus.Status)
	}
	if fcu.PayloadID == nil || *fcu.PayloadID == "" {
		t.Fatalf("engine_forkchoiceUpdatedV5 did not return a payloadId for a %d-byte IL: %+v", ilSize, fcu)
	}
}

func testInclusionListBytes(t *hivesim.T, c *hivesim.Client, il []hexutil.Bytes) {
	parent := latestBlock(t, c)
	timestamp := uint64(parent.Timestamp) + 1
	fcu := forkchoiceUpdatedV5(t, c, parent.Hash, timestamp, il)
	if fcu.PayloadID == nil || *fcu.PayloadID == "" {
		t.Fatalf("engine_forkchoiceUpdatedV5 did not return a payloadId, response: %+v", fcu)
	}
	if fcu.PayloadStatus.Status == "INVALID" || fcu.PayloadStatus.Status == "INCLUSION_LIST_UNSATISFIED" {
		t.Fatalf("engine_forkchoiceUpdatedV5 rejected harmless IL bytes with status %s", fcu.PayloadStatus.Status)
	}

	envelope := getPayloadV6(t, c, *fcu.PayloadID)
	header := decodePayloadHeader(t, envelope.ExecutionPayload)

	status := newPayloadV6(t, c, envelope, il)
	if status.Status == "INVALID" || status.Status == "INCLUSION_LIST_UNSATISFIED" {
		t.Fatalf("engine_newPayloadV6 rejected harmless IL bytes with status %s", status.Status)
	}

	forkchoiceUpdatedV5NoPayload(t, c, header.BlockHash)
}

func makeInclusionListCases(t *hivesim.T) []inclusionListCase {
	type2Tx := makeRawTransaction(t, types.DynamicFeeTxType, 0, 10_000, []byte("focil-single-type-2"))
	legacyTx := makeRawTransaction(t, types.LegacyTxType, 1, 10_000, []byte("focil-legacy"))
	accessListTx := makeRawTransaction(t, types.AccessListTxType, 2, 10_000, []byte("focil-access-list"))
	dynamicFeeTx := makeRawTransaction(t, types.DynamicFeeTxType, 3, 10_000, []byte("focil-dynamic-fee"))

	return []inclusionListCase{
		{
			name:        "empty",
			description: "Sends an empty inclusion list through forkchoiceUpdatedV5 and newPayloadV6.",
			txs:         []hexutil.Bytes{},
		},
		{
			name:        "garbage bytes",
			description: "Sends malformed transaction bytes and requires clients to treat them like an empty IL rather than crashing.",
			txs:         []hexutil.Bytes{{0xde, 0xad, 0xbe, 0xef}, {}, {0x02, 0xc0}},
		},
		{
			name:        "single type-2 transaction",
			description: "Sends one valid EIP-1559 transaction byte string and only requires successful deserialization.",
			txs:         []hexutil.Bytes{type2Tx},
		},
		{
			name:        "mixed transaction types",
			description: "Sends legacy, access-list, and dynamic-fee transaction byte strings in one IL.",
			txs:         []hexutil.Bytes{legacyTx, accessListTx, dynamicFeeTx},
		},
	}
}

func forkchoiceUpdatedV5(t *hivesim.T, c *hivesim.Client, head common.Hash, timestamp uint64, il []hexutil.Bytes) forkchoiceResponse {
	var resp forkchoiceResponse
	callEngine(t, c, &resp, "engine_forkchoiceUpdatedV5", forkchoiceState{
		HeadBlockHash:      head,
		SafeBlockHash:      zeroHash,
		FinalizedBlockHash: zeroHash,
	}, payloadAttributes(timestamp, il))
	return resp
}

func forkchoiceUpdatedV5NoPayload(t *hivesim.T, c *hivesim.Client, head common.Hash) {
	var resp forkchoiceResponse
	callEngine(t, c, &resp, "engine_forkchoiceUpdatedV5", forkchoiceState{
		HeadBlockHash:      head,
		SafeBlockHash:      zeroHash,
		FinalizedBlockHash: zeroHash,
	}, nil)
	if resp.PayloadStatus.Status == "INVALID" {
		t.Fatalf("canonical forkchoiceUpdatedV5 returned INVALID: %+v", resp)
	}
}

func payloadAttributes(timestamp uint64, il []hexutil.Bytes) payloadAttributesV5 {
	if il == nil {
		il = []hexutil.Bytes{}
	}
	return payloadAttributesV5{
		Timestamp:                 hexutil.Uint64(timestamp),
		Random:                    prevRandao,
		SuggestedFeeRecipient:     feeReceiver,
		Withdrawals:               []*types.Withdrawal{},
		ParentBeaconBlockRoot:     zeroHash,
		SlotNumber:                hexutil.Uint64(timestamp),
		InclusionListTransactions: il,
	}
}

func getPayloadV6(t *hivesim.T, c *hivesim.Client, payloadID string) payloadEnvelope {
	var envelope payloadEnvelope
	callEngine(t, c, &envelope, "engine_getPayloadV6", payloadID)
	if len(envelope.ExecutionPayload) == 0 {
		t.Fatal("engine_getPayloadV6 returned an empty executionPayload")
	}
	return envelope
}

func newPayloadV6(t *hivesim.T, c *hivesim.Client, envelope payloadEnvelope, il []hexutil.Bytes) payloadStatus {
	var status payloadStatus
	err := callNewPayloadV6(c, &status, envelope, il)
	if err != nil {
		t.Fatalf("engine_newPayloadV6 failed: %v", err)
	}
	return status
}

func callNewPayloadV6(c *hivesim.Client, status *payloadStatus, envelope payloadEnvelope, il []hexutil.Bytes) error {
	executionRequests := envelope.ExecutionRequests
	if executionRequests == nil {
		executionRequests = []hexutil.Bytes{}
	}
	if il == nil {
		il = []hexutil.Bytes{}
	}
	ctx, cancel := context.WithTimeout(context.Background(), rpcTimeout)
	defer cancel()
	return c.EngineAPI().CallContext(ctx, status, "engine_newPayloadV6",
		envelope.ExecutionPayload,
		[]common.Hash{},
		zeroHash,
		executionRequests,
		il,
	)
}

func latestBlock(t *hivesim.T, c *hivesim.Client) ethBlock {
	ctx, cancel := context.WithTimeout(context.Background(), rpcTimeout)
	defer cancel()
	var block ethBlock
	if err := c.RPC().CallContext(ctx, &block, "eth_getBlockByNumber", "latest", false); err != nil {
		t.Fatalf("eth_getBlockByNumber latest failed: %v", err)
	}
	if block.Hash == zeroHash {
		t.Fatal("latest block has zero hash")
	}
	return block
}

func decodePayloadHeader(t *hivesim.T, payload json.RawMessage) payloadHeader {
	var header payloadHeader
	if err := json.Unmarshal(payload, &header); err != nil {
		t.Fatalf("could not decode execution payload header fields: %v", err)
	}
	if header.BlockHash == zeroHash {
		t.Fatal("execution payload has zero blockHash")
	}
	return header
}

func sendMempoolTransactions(t *hivesim.T, c *hivesim.Client) {
	for i, txType := range []uint8{types.LegacyTxType, types.AccessListTxType, types.DynamicFeeTxType} {
		raw := makeRawTransaction(t, txType, uint64(i), 0, []byte(fmt.Sprintf("focil-mempool-%d", i)))
		ctx, cancel := context.WithTimeout(context.Background(), rpcTimeout)
		err := c.RPC().CallContext(ctx, nil, "eth_sendRawTransaction", hexutil.Bytes(raw))
		cancel()
		if err != nil && !strings.Contains(err.Error(), "already known") {
			t.Fatalf("eth_sendRawTransaction failed for tx type %d: %v", txType, err)
		}
	}
}

func makeRawTransaction(t *hivesim.T, txType uint8, accountIndex uint64, nonce uint64, data []byte) hexutil.Bytes {
	account := globals.TestAccounts[accountIndex]
	var txData types.TxData
	switch txType {
	case types.LegacyTxType:
		txData = &types.LegacyTx{
			Nonce:    nonce,
			GasPrice: globals.GasPrice,
			Gas:      50_000,
			To:       &txTo,
			Value:    big.NewInt(1),
			Data:     data,
		}
	case types.AccessListTxType:
		txData = &types.AccessListTx{
			ChainID:  globals.ChainID,
			Nonce:    nonce,
			GasPrice: globals.GasPrice,
			Gas:      50_000,
			To:       &txTo,
			Value:    big.NewInt(1),
			Data:     data,
		}
	case types.DynamicFeeTxType:
		txData = &types.DynamicFeeTx{
			ChainID:   globals.ChainID,
			Nonce:     nonce,
			GasTipCap: globals.GasTipPrice,
			GasFeeCap: globals.GasPrice,
			Gas:       50_000,
			To:        &txTo,
			Value:     big.NewInt(1),
			Data:      data,
		}
	default:
		t.Fatalf("unsupported tx type in test vector: %d", txType)
	}

	signed, err := types.SignTx(types.NewTx(txData), types.NewCancunSigner(globals.ChainID), account.GetKey())
	if err != nil {
		t.Fatalf("could not sign tx type %d: %v", txType, err)
	}
	raw, err := signed.MarshalBinary()
	if err != nil {
		t.Fatalf("could not marshal tx type %d: %v", txType, err)
	}
	return raw
}

func validateInclusionListBytes(t *hivesim.T, il []hexutil.Bytes) {
	encodedLen := inclusionListRLPSize(t, il)
	if encodedLen > maxBytesPerInclusionList {
		t.Fatalf("inclusion list RLP size is %d bytes, max is %d", encodedLen, maxBytesPerInclusionList)
	}

	for i, raw := range il {
		if len(raw) == 0 {
			t.Logf("IL item %d is empty; treating it like a non-deserializable transaction", i)
			continue
		}
		if raw[0] == types.BlobTxType {
			t.Fatalf("IL item %d is a blob transaction; FOCIL ILs must not include type 3", i)
		}
		var tx types.Transaction
		if err := tx.UnmarshalBinary(raw); err != nil {
			t.Logf("IL item %d is not a transaction (%v); treating it like empty IL input", i, err)
			continue
		}
		if tx.Type() == types.BlobTxType {
			t.Fatalf("IL item %d decoded as a blob transaction; FOCIL ILs must not include type 3", i)
		}
	}
}

func inclusionListRLPSize(t *hivesim.T, il []hexutil.Bytes) int {
	asBytes := make([][]byte, len(il))
	for i := range il {
		asBytes[i] = il[i]
	}
	encoded, err := rlp.EncodeToBytes(asBytes)
	if err != nil {
		t.Fatalf("could not RLP encode inclusion-list transaction bytes: %v", err)
	}
	return len(encoded)
}

func callEngine(t *hivesim.T, c *hivesim.Client, result interface{}, method string, args ...interface{}) {
	ctx, cancel := context.WithTimeout(context.Background(), rpcTimeout)
	defer cancel()
	if err := c.EngineAPI().CallContext(ctx, result, method, args...); err != nil {
		t.Fatalf("%s failed: %v", method, err)
	}
}
