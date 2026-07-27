package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/ethereum/go-ethereum/beacon/engine"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/params"
)

func TestAmsterdamChainAndEngineOutputs(t *testing.T) {
	g := newAmsterdamTestGenerator(t)
	activation := *g.genesis.Config.AmsterdamTime / blocktimeSec

	preFork := g.blockchain.GetBlockByNumber(activation - 1)
	if preFork.BlockAccessListHash() != nil {
		t.Fatal("pre-Amsterdam block has blockAccessListHash")
	}
	if preFork.AccessList() != nil {
		t.Fatal("pre-Amsterdam block has a block access list")
	}
	if preFork.SlotNumber() != nil {
		t.Fatal("pre-Amsterdam block has slotNumber")
	}

	postFork := g.blockchain.GetBlockByNumber(activation)
	if postFork.BlockAccessListHash() == nil {
		t.Fatal("Amsterdam block is missing blockAccessListHash")
	}
	if postFork.AccessList() == nil {
		t.Fatal("Amsterdam block is missing block access list")
	}
	if postFork.SlotNumber() == nil {
		t.Fatal("Amsterdam block is missing slotNumber")
	}
	if got, want := *postFork.BlockAccessListHash(), postFork.AccessList().Hash(); got != want {
		t.Fatalf("block access list hash mismatch: got %s, want %s", got, want)
	}
	for i, receipt := range g.blockchain.GetReceiptsByHash(postFork.Hash()) {
		if receipt.Status != 1 {
			t.Errorf("Amsterdam transaction %d reverted (gas used %d)", i, receipt.GasUsed)
		}
	}

	np := g.block2newpayload(postFork)
	if np.Method != "engine_newPayloadV5" {
		t.Fatalf("wrong newPayload method: got %q", np.Method)
	}
	if len(np.Params) != 4 {
		t.Fatalf("wrong newPayloadV5 parameter count: got %d, want 4", len(np.Params))
	}
	payload, ok := np.Params[0].(*engine.ExecutableData)
	if !ok {
		t.Fatalf("wrong payload type %T", np.Params[0])
	}
	if payload.SlotNumber == nil {
		t.Fatal("newPayloadV5 payload is missing slotNumber")
	}
	if len(payload.BlockAccessList) == 0 {
		t.Fatal("newPayloadV5 payload is missing blockAccessList")
	}
	if got, want := crypto.Keccak256Hash(payload.BlockAccessList), *postFork.BlockAccessListHash(); got != want {
		t.Fatalf("payload block access list hash mismatch: got %s, want %s", got, want)
	}
	encodedRequests, ok := np.Params[3].([]hexutil.Bytes)
	if !ok {
		t.Fatalf("wrong execution requests type %T", np.Params[3])
	}
	for i, request := range encodedRequests {
		if len(request) == 0 {
			t.Fatalf("execution request %d is empty", i)
		}
	}
	encodedRequest, err := json.Marshal(np)
	if err != nil {
		t.Fatal(err)
	}
	for _, prefix := range [][]byte{[]byte(`"0x03`), []byte(`"0x04`)} {
		if !bytes.Contains(encodedRequest, prefix) {
			t.Errorf("newPayloadV5 JSON is missing hex request prefix %s", prefix)
		}
	}

	fcu := g.block2fcu(postFork)
	if fcu.Method != "engine_forkchoiceUpdatedV4" {
		t.Fatalf("wrong forkchoiceUpdated method: got %q", fcu.Method)
	}
	if len(fcu.Params) != 3 {
		t.Fatalf("wrong forkchoiceUpdatedV4 parameter count: got %d, want 3", len(fcu.Params))
	}

	requestTypes := make(map[byte]bool)
	for _, request := range g.clRequests[activation] {
		if len(request) > 0 {
			requestTypes[request[0]] = true
		}
	}
	for _, requestType := range []byte{0x03, 0x04} {
		if !requestTypes[requestType] {
			t.Errorf("Amsterdam block is missing request type 0x%02x", requestType)
		}
	}
}

func TestAmsterdamConfigAndForkEnv(t *testing.T) {
	cfg := generatorConfig{
		merged:       true,
		lastFork:     "amsterdam",
		forkInterval: 2,
		chainLength:  14,
		outputDir:    t.TempDir(),
		outputs:      []string{},
	}
	cfg, err := cfg.withDefaults()
	if err != nil {
		t.Fatal(err)
	}
	g := newGenerator(cfg)
	if g.genesis.Config.AmsterdamTime == nil {
		t.Fatal("Amsterdam timestamp is missing from chain config")
	}

	for addr, want := range map[common.Address][]byte{
		params.BuilderDepositAddress: params.BuilderDepositCode,
		params.BuilderExitAddress:    params.BuilderExitCode,
	} {
		account, ok := g.genesis.Alloc[addr]
		if !ok {
			t.Fatalf("Amsterdam system contract %s is missing from genesis", addr)
		}
		if !bytes.Equal(account.Code, want) {
			t.Fatalf("Amsterdam system contract %s has wrong code", addr)
		}
	}

	if err := g.writeForkEnv(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(cfg.outputDir, "forkenv.json"))
	if err != nil {
		t.Fatal(err)
	}
	var env map[string]string
	if err := json.Unmarshal(data, &env); err != nil {
		t.Fatal(err)
	}
	if got, want := env["HIVE_AMSTERDAM_TIMESTAMP"], "120"; got != want {
		t.Fatalf("wrong Amsterdam forkenv timestamp: got %q, want %q", got, want)
	}
}

func newAmsterdamTestGenerator(t *testing.T) *generator {
	t.Helper()

	var disabled []string
	for name := range modRegistry {
		if name != "tx-request-eip8282-deposit" && name != "tx-request-eip8282-exit" {
			disabled = append(disabled, name)
		}
	}
	cfg := generatorConfig{
		merged:       true,
		lastFork:     "amsterdam",
		forkInterval: 2,
		chainLength:  14,
		txInterval:   1,
		txCount:      2,
		outputDir:    t.TempDir(),
		outputs:      []string{},
		disabledMods: disabled,
	}
	cfg, err := cfg.withDefaults()
	if err != nil {
		t.Fatal(err)
	}
	g := newGenerator(cfg)
	if err := g.run(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(g.blockchain.Stop)
	return g
}
