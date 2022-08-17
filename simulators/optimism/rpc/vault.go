package main

import (
	"context"
	"crypto/ecdsa"
	"errors"
	"fmt"
	"github.com/ethereum/go-ethereum/params"
	"math/big"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
)

var (
	// Address of the vault in genesis.
	vaultAddr = common.HexToAddress("0xf39fd6e51aad88f6f4ce6ab8827279cfffb92266")
	// This is the account that sends vault funding transactions.
	vaultKey, _ = crypto.HexToECDSA("ac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80")
	// Number of blocks to wait before funding tx is considered valid.
	vaultTxConfirmationCount = uint64(5)
)

// vault creates accounts for testing and funds them. An instance of the vault contract is
// deployed in the genesis block. When creating a new account using createAccount, the
// account is funded by sending a transaction to this contract.
//
// The purpose of the vault is allowing tests to run concurrently without worrying about
// nonce assignment and unexpected balance changes.
type vault struct {
	mu sync.Mutex
	// This tracks the account nonce of the vault account.
	nonce uint64
	// Created accounts are tracked in this map.
	accounts map[common.Address]*ecdsa.PrivateKey
}

func newVault() *vault {
	return &vault{
		accounts: make(map[common.Address]*ecdsa.PrivateKey),
	}
}

// generateKey creates a new account key and stores it.
func (v *vault) generateKey() common.Address {
	key, err := crypto.GenerateKey()
	if err != nil {
		panic(fmt.Errorf("can't generate account key: %v", err))
	}
	addr := crypto.PubkeyToAddress(key.PublicKey)

	v.mu.Lock()
	defer v.mu.Unlock()
	v.accounts[addr] = key
	return addr
}

// findKey returns the private key for an address.
func (v *vault) findKey(addr common.Address) *ecdsa.PrivateKey {
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.accounts[addr]
}

// signTransaction signs the given transaction with the test account and returns it.
// It uses the EIP155 signing rules.
func (v *vault) signTransaction(sender common.Address, tx *types.Transaction) (*types.Transaction, error) {
	key := v.findKey(sender)
	if key == nil {
		return nil, fmt.Errorf("sender account %v not in vault", sender)
	}
	signer := types.LatestSignerForChainID(l2ChainID)
	return types.SignTx(tx, signer, key)
}

// createAndFundAccount creates a new account that is funded from the vault contract.
// It will panic when the account could not be created and funded.
func (v *vault) createAccountWithSubscription(t *TestEnv, amount *big.Int) common.Address {
	if amount == nil {
		amount = new(big.Int)
	}
	address := v.generateKey()

	// setup subscriptions
	var (
		headsSub ethereum.Subscription
		heads    = make(chan *types.Header)
		logsSub  ethereum.Subscription
	)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// listen for new heads
	headsSub, err := t.Eth.SubscribeNewHead(ctx, heads)
	if err != nil {
		t.Fatal("could not create new head subscription:", err)
	}
	defer headsSub.Unsubscribe()

	// order the vault to send some ether
	tx := v.makeFundingTx(t, address, amount)
	if err := t.Eth.SendTransaction(ctx, tx); err != nil {
		t.Fatalf("unable to send funding transaction: %v", err)
	}

	// wait for confirmed log
	var (
		latestHeader *types.Header
		receivedLog  *types.Log
		timeout      = time.NewTimer(120 * time.Second)
	)
	for {
		select {
		case head := <-heads:
			latestHeader = head
		case err := <-headsSub.Err():
			t.Fatalf("could not fund new account: %v", err)
		case err := <-logsSub.Err():
			t.Fatalf("could not fund new account: %v", err)
		case <-timeout.C:
			t.Fatal("could not fund new account: timeout")
		}

		if latestHeader != nil && receivedLog != nil {
			if receivedLog.BlockNumber+vaultTxConfirmationCount <= latestHeader.Number.Uint64() {
				return address
			}
		}
	}

	return address
}

// createAccount creates a new account that is funded from the vault contract.
// It will panic when the account could not be created and funded.
func (v *vault) createAccount(t *TestEnv, amount *big.Int) common.Address {
	if amount == nil {
		amount = new(big.Int)
	}
	address := v.generateKey()

	// order the vault to send some ether
	tx := v.makeFundingTx(t, address, amount)
	if err := t.Eth.SendTransaction(t.Ctx(), tx); err != nil {
		t.Fatalf("unable to send funding transaction: %v", err)
	}

	for i := 0; i < 60; i++ {
		receipt, err := t.Eth.TransactionReceipt(t.Ctx(), tx.Hash())
		if err != nil && !errors.Is(err, ethereum.NotFound) {
			t.Fatal("error getting transaction receipt:", err)
		}
		if receipt != nil {
			return address
		}
		time.Sleep(time.Second)
	}

	t.Fatal("timed out getting transaction receipt")
	return common.Address{}
}

func (v *vault) makeFundingTx(t *TestEnv, recipient common.Address, amount *big.Int) *types.Transaction {
	var (
		nonce    = v.nextNonce()
		gasLimit = uint64(75000)
	)
	tx := types.NewTx(&types.DynamicFeeTx{
		Nonce:     nonce,
		Gas:       gasLimit,
		GasTipCap: big.NewInt(1 * params.GWei),
		GasFeeCap: gasPrice,
		To:        &recipient,
		Value:     amount,
	})
	signer := types.LatestSignerForChainID(l2ChainID)
	signedTx, err := types.SignTx(tx, signer, vaultKey)
	if err != nil {
		t.Fatal("can't sign vault funding tx:", err)
	}
	return signedTx
}

// nextNonce generates the nonce of a funding transaction.
func (v *vault) nextNonce() uint64 {
	v.mu.Lock()
	defer v.mu.Unlock()

	nonce := v.nonce
	v.nonce++
	return nonce
}
