package helper

import (
	"context"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/hive/simulators/ethereum/engine/client"
)

// stubEngineClient satisfies client.EngineClient with only TransactionByHash
// implemented; the remaining methods are never exercised by the tests below.
type stubEngineClient struct {
	client.EngineClient
	txByHash func(ctx context.Context, hash common.Hash) (*types.Transaction, bool, error)
}

func (s *stubEngineClient) TransactionByHash(ctx context.Context, hash common.Hash) (*types.Transaction, bool, error) {
	return s.txByHash(ctx, hash)
}

func TestWaitForTransactionPending(t *testing.T) {
	txHash := common.HexToHash("0x1234")

	t.Run("immediate", func(t *testing.T) {
		calls := 0
		node := &stubEngineClient{txByHash: func(_ context.Context, hash common.Hash) (*types.Transaction, bool, error) {
			calls++
			if hash != txHash {
				t.Fatalf("queried wrong hash: %v", hash)
			}
			return nil, true, nil
		}}
		if err := waitForTransactionPending(context.Background(), node, txHash); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if calls != 1 {
			t.Fatalf("expected one poll, got %d", calls)
		}
	})

	t.Run("eventual", func(t *testing.T) {
		calls := 0
		node := &stubEngineClient{txByHash: func(_ context.Context, _ common.Hash) (*types.Transaction, bool, error) {
			calls++
			return nil, calls > 2, nil
		}}
		if err := waitForTransactionPending(context.Background(), node, txHash); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if calls != 3 {
			t.Fatalf("expected three polls, got %d", calls)
		}
	})

	t.Run("context", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()
		node := &stubEngineClient{txByHash: func(_ context.Context, _ common.Hash) (*types.Transaction, bool, error) {
			return nil, false, nil
		}}
		start := time.Now()
		if err := waitForTransactionPending(ctx, node, txHash); err == nil {
			t.Fatal("expected an error on expired context")
		}
		if time.Since(start) > txPendingTimeout {
			t.Fatal("wait exceeded the bounded timeout")
		}
	})
}
