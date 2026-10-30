package node

import (
	"context"
	"errors"
	"math/big"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/txpool"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/params"
)

func TestSendTransactionWaitsForPromotion(t *testing.T) {
	testTransactionPromotion(t, 1, func(n *GethNode, txs []*types.Transaction) error {
		return n.SendTransaction(context.Background(), txs[0])
	})
}

func TestSendTransactionsWaitsForPromotion(t *testing.T) {
	testTransactionPromotion(t, 2, func(n *GethNode, txs []*types.Transaction) error {
		return errors.Join(n.SendTransactions(context.Background(), txs[0], txs[1])...)
	})
}

func testTransactionPromotion(t *testing.T, count int, send func(*GethNode, []*types.Transaction) error) {
	t.Helper()
	key, err := crypto.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	address := crypto.PubkeyToAddress(key.PublicKey)
	config := *params.AllEthashProtocolChanges
	config.TerminalTotalDifficulty = new(big.Int)
	genesis := &core.Genesis{
		Config:     &config,
		GasLimit:   30_000_000,
		Difficulty: new(big.Int),
		Alloc: types.GenesisAlloc{
			address: {Balance: new(big.Int).Exp(big.NewInt(10), big.NewInt(24), nil)},
		},
	}
	n, err := newNode(GethNodeTestConfiguration{MaxPeers: big.NewInt(0)}, nil, genesis)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := n.Close(); err != nil {
			t.Error(err)
		}
	})
	pool := n.eth.TxPool()
	txs := make([]*types.Transaction, count+1)
	for i := range txs {
		txs[i] = types.MustSignNewTx(key, types.LatestSigner(&config), &types.LegacyTx{
			Nonce: uint64(i), To: &address, Gas: params.TxGas, GasPrice: big.NewInt(30 * params.GWei),
		})
	}

	// An unread event holds the current promotion run open, preventing later
	// transactions from being promoted until the subscription is released.
	events := make(chan core.NewTxsEvent)
	subscription := pool.SubscribeTransactions(events, false)
	t.Cleanup(subscription.Unsubscribe)
	if err := pool.Add(txs[:1], false)[0]; err != nil {
		t.Fatal(err)
	}
	waitForTransactionStatus(t, pool, txs[0], txpool.TxStatusPending)

	result := make(chan error, 1)
	go func() { result <- send(n, txs[1:]) }()
	waitForTransactionStatus(t, pool, txs[1], txpool.TxStatusQueued)
	select {
	case err := <-result:
		t.Fatalf("submission returned before transaction promotion: %v", err)
	case <-time.After(100 * time.Millisecond):
	}

	subscription.Unsubscribe()
	select {
	case err := <-result:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("submission did not finish after releasing promotion")
	}
	for _, tx := range txs[1:] {
		if status := pool.Status(tx.Hash()); status != txpool.TxStatusPending {
			t.Fatalf("transaction %s is not pending after submission: status %d", tx.Hash(), status)
		}
	}
}

func waitForTransactionStatus(t *testing.T, pool *txpool.TxPool, tx *types.Transaction, want txpool.TxStatus) {
	t.Helper()
	deadline := time.NewTimer(5 * time.Second)
	defer deadline.Stop()
	ticker := time.NewTicker(time.Millisecond)
	defer ticker.Stop()
	for pool.Status(tx.Hash()) != want {
		select {
		case <-deadline.C:
			t.Fatalf("transaction %s did not reach status %d", tx.Hash(), want)
		case <-ticker.C:
		}
	}
}
