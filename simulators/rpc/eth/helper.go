package main

import (
	"bytes"
	"crypto/ecdsa"
	"fmt"
	"math/big"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
	"golang.org/x/net/context"
)

var (
	// default timeout for RPC calls
	rpcTimeout = 5 * time.Second
	// unique chain identifier used to sign transaction
	chainID = new(big.Int).SetInt64(7) // used for signing transactions
)

// TestClient is an ethclient that exposed the CallContext function.
// This allows for calling custom RPC methods that are not exposed
// by the ethclient.
type TestClient struct {
	*ethclient.Client
	rc *rpc.Client
}

// CallContext is a helper method that forwards a raw RPC request to
// the underlying RPC client. This can be used to call RPC methods
// that are not supported by the ethclient.Client.
func (c *TestClient) CallContext(ctx context.Context, result interface{}, method string, args ...interface{}) error {
	return c.rc.CallContext(ctx, result, method, args...)
}

// Naive generic function that works in all situations.
// A better solution is to use logs to wait for confirmations.
func waitForTxConfirmations(client *TestClient, txHash common.Hash, n uint64) (*types.Receipt, error) {
	var (
		receipt    *types.Receipt
		startBlock *types.Block
		err        error
	)

	for i := 0; i < 90; i++ {
		ctx, _ := context.WithTimeout(context.Background(), rpcTimeout)
		receipt, err = client.TransactionReceipt(ctx, txHash)
		if err != nil && err != ethereum.NotFound {
			return nil, err
		}
		if receipt != nil {
			break
		}
		time.Sleep(time.Second)
	}

	if receipt == nil {
		return nil, ethereum.NotFound
	}

	ctx, _ := context.WithTimeout(context.Background(), rpcTimeout)
	if startBlock, err = client.BlockByNumber(ctx, nil); err != nil {
		return nil, err
	}

	for i := 0; i < 90; i++ {
		ctx, _ := context.WithTimeout(context.Background(), rpcTimeout)
		currentBlock, err := client.BlockByNumber(ctx, nil)
		if err != nil {
			return nil, err
		}

		if startBlock.NumberU64()+n >= currentBlock.NumberU64() {
			ctx, _ := context.WithTimeout(context.Background(), rpcTimeout)
			if checkReceipt, err := client.TransactionReceipt(ctx, txHash); checkReceipt != nil {
				if bytes.Compare(receipt.PostState, checkReceipt.PostState) == 0 {
					return receipt, nil
				} else { // chain reorg
					waitForTxConfirmations(client, txHash, n)
				}
			} else {
				return nil, err
			}
		}

		time.Sleep(time.Second)
	}

	return nil, ethereum.NotFound
}

/*
type ConfirmationRequest struct {
	Receipt chan *types.Receipt // holds the receipt on success
	Error   chan error          // error occurred

	n          uint64        // wait N confirmations
	includedIn uint64        // block included in
	accepted   chan struct{} // closed when the request is accepted
	txHash     common.Hash   // transaction to be confirmed
}

// CreateConfirmationRequest creates a new transaction confirmation requests
// that can be used to retrieve the receipt after it has n confirmation.
func CreateConfirmationRequest(n uint64, txHash common.Hash) *ConfirmationRequest {
	req := &ConfirmationRequest{
		n:        n,
		accepted: make(chan struct{}),
		txHash:   txHash,
		Receipt:  make(chan *types.Receipt),
		Error:    make(chan error),
	}

	confirmationQueue <- req
	<-req.accepted

	return req
}

func StartConfirmationLoop(clients chan *TestClient) error {
	var (
		pending = make(map[common.Hash]*ConfirmationRequest)
		waiting = make(map[common.Hash]*ConfirmationRequest)
		newHead = make(chan *types.Header)
	)

	client := <-clients
	defer func() { clients <- client }()

	// setup newHead subscription
	if err := setupNewHeadSubscription(client, newHead); err != nil {
		return err
	}

	// wait for requests/new blocks
	go func() {
		for {
			select {
			case req := <-confirmationQueue:
				pending[req.txHash] = req
				close(req.accepted)
			case head := <-newHead:
				client := <-clients

				ctx, _ := context.WithTimeout(context.Background(), rpcTimeout)
				block, err := client.BlockByHash(ctx, head.Hash())
				if err != nil {
					clients <- client
					continue
				}

				// check included tx waiting requests
				for _, tx := range block.Transactions() {
					if req, found := pending[tx.Hash()]; found {
						req.includedIn = block.NumberU64()
						waiting[tx.Hash()] = req
						delete(pending, tx.Hash())
					} else if req, found := waiting[tx.Hash()]; found {
						// tx included in a different block (reorg)
						req.includedIn = block.NumberU64()
					}
				}

				// search for tx that are confirmed
				for hash, req := range waiting {
					if req.includedIn <= block.NumberU64()-req.n {
						ctx, _ := context.WithTimeout(context.Background(), rpcTimeout)
						if receipt, err := client.TransactionReceipt(ctx, req.txHash); err == nil {
							req.Receipt <- receipt
						} else {
							req.Error <- err
						}
						delete(waiting, hash)
					}
				}
				clients <- client
			}
		}
	}()

	return nil
}

func setupNewHeadSubscription(client *TestClient, newHead chan *types.Header) error {
	ctx, _ := context.WithTimeout(context.Background(), rpcTimeout)
	_, err := client.SubscribeNewHead(ctx, newHead)
	if err == nil {
		return nil
	}

	if err == rpc.ErrNotificationsUnsupported { // fallback to polling
		ctx, _ := context.WithTimeout(context.Background(), rpcTimeout)
		currentBlock, err := client.BlockByNumber(ctx, nil)
		if err != nil {
			return err
		}

		go func() {
			for {
				time.Sleep(time.Second)

				ctx, _ := context.WithTimeout(context.Background(), rpcTimeout)
				block, err := client.BlockByNumber(ctx, nil)
				if err != nil {
					continue
				}

				for currentBlock.NumberU64() < block.NumberU64() {
					fetchedBlock, err := client.BlockByNumber(ctx, new(big.Int).SetUint64(currentBlock.NumberU64()+1))
					if err != nil {
						break
					}

					newHead <- fetchedBlock.Header()
					currentBlock = fetchedBlock
				}
			}
		}()
		return nil
	}

	return err
}

*/
func createHTTPClients(hosts []string) chan *TestClient {
	if len(hosts) == 0 {
		panic("Supply at least 1 host")
	}

	var (
		N       = 32
		clients = make(chan *TestClient, N)
	)

	for i := 0; i < N; i++ {
		client, err := rpc.Dial(fmt.Sprintf("http://%s:8545", hosts[i%len(hosts)]))
		if err != nil {
			panic(err)
		}
		clients <- &TestClient{ethclient.NewClient(client), client}
	}

	return clients
}

func createWebsocketClients(hosts []string) chan *TestClient {
	if len(hosts) == 0 {
		panic("Supply at least 1 host")
	}

	var (
		N       = 32
		clients = make(chan *TestClient, N)
	)

	for i := 0; i < N; i++ {
		client, err := rpc.Dial(fmt.Sprintf("ws://%s:8546", hosts[i%len(hosts)]))
		if err != nil {
			panic(err)
		}
		clients <- &TestClient{ethclient.NewClient(client), client}
	}

	return clients
}

// runTests is a utility function that calls the unit test with the
// client as second argument.
func runTest(test func(t *testing.T, client *TestClient), clients chan *TestClient) func(t *testing.T) {
	return func(t *testing.T) {
		client := <-clients
		test(t, client)
		clients <- client
	}
}

// SignTransaction signs the given transaction with the test account and returns it.
// It uses the EIP155 signing rules.
func SignTransaction(tx *types.Transaction, key *ecdsa.PrivateKey) (*types.Transaction, error) {
	signer := types.NewEIP155Signer(chainID)
	hash := signer.Hash(tx)
	signature, err := crypto.Sign(hash[:], key)
	if err != nil {
		return nil, err
	}
	return tx.WithSignature(signer, signature)

}
