package main

import (
	"context"
	"fmt"
	"math/big"
	"os"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/keystore"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
)

var (
	predeployedVaultContractSrc = `
pragma solidity ^0.4.6;

// The vault contract is used in the hive rpc-tests suite.
// From this preallocated contract accounts that are created
// during the tests are funded.
contract Vault {
    event Send(address indexed, uint);

    // sendSome send 'amount' wei 'to'
    function sendSome(address to, uint amount) {
        if (to.send(amount)) {
            Send(to, amount);
        }
    }
}`
	// address of the vault that is used to fund created test accounts
	predeployedVaultAddr = common.HexToAddress("0000000000000000000000000000000000000315")
	// vault ABI
	predeployedVaultABI = `[{"constant":false,"inputs":[{"name":"to","type":"address"},{"name":"amount","type":"uint256"}],"name":"sendSome","outputs":[],"payable":false,"type":"function"},{"anonymous":false,"inputs":[{"indexed":true,"name":"","type":"address"},{"indexed":false,"name":"","type":"uint256"}],"name":"Send","type":"event"}]`
	// wait for vaultTxConfirmationCount before a vault fund tx is considered confirmed
	vaultTxConfirmationCount = uint64(10)
	// software based keystore
	keyStore *keystore.KeyStore
	// account manager used to create new accounts and sign data
	accountsManager *accounts.Manager
	// default password for generated accounts
	defaultPassword = ""
)

var (
	testChainID      = big.NewInt(7)
	vaultAccountAddr = common.HexToAddress("0xcf49fda3be353c69b41ed96333cd24302da4556f")
	vaultKey, _      = crypto.HexToECDSA("63b508a03c3b5937ceb903af8b1b0c191012ef6eb7e9c3fb7afa94e5d214d376")
)

func init() {
	keyStore = keystore.NewKeyStore(os.TempDir(), keystore.StandardScryptN, keystore.StandardScryptP)
	accountsManager = accounts.NewManager(&accounts.Config{}, keyStore)
}

// createAndFundAccount creates a new account that is funded from the vault contract.
// It will panic when the account could not be created and funded.
func createAndFundAccountWithSubscription(t *TestEnv, amount *big.Int) accounts.Account {
	account, err := keyStore.NewAccount(defaultPassword)
	if err != nil {
		panic(err)
	}

	// each node has at least one 1 key with some of pre-allocated ether
	if amount == nil {
		amount = common.Big0
	}

	// setup subscriptions
	var (
		ctx      context.Context
		headsSub ethereum.Subscription
		heads    = make(chan *types.Header)
		logsSub  ethereum.Subscription
		logs     = make(chan types.Log)
		vault, _ = abi.JSON(strings.NewReader(predeployedVaultABI))
	)

	headsSub, err = t.Eth.SubscribeNewHead(ctx, heads)
	if err != nil {
		panic(fmt.Sprintf("Could not create new head subscription: %v", err))
	}
	defer headsSub.Unsubscribe()

	eventTopic := vault.Events["Send"].ID
	addressTopic := common.BytesToHash(common.LeftPadBytes(account.Address[:], 32))
	q := ethereum.FilterQuery{
		Addresses: []common.Address{predeployedVaultAddr},
		Topics:    [][]common.Hash{[]common.Hash{eventTopic}, []common.Hash{addressTopic}},
	}
	logsSub, err = t.Eth.SubscribeFilterLogs(ctx, q, logs)
	if err != nil {
		panic(fmt.Sprintf("Could not create log filter subscription: %v", err))
	}
	defer logsSub.Unsubscribe()

	// order the vault to send some ether
	tx := vaultSendSome(t, account.Address, amount)
	if err := t.Eth.SendTransaction(t.Ctx(), tx); err != nil {
		t.Fatalf("unable to send funding transaction: %v", err)
	}
	var (
		latestHeader *types.Header
		receivedLog  *types.Log
		timeout      = time.NewTimer(120 * time.Second)
	)

	// wait for confirmed log
	for {
		select {
		case head := <-heads:
			latestHeader = head
		case log := <-logs:
			if !log.Removed {
				receivedLog = &log
			} else if log.Removed && receivedLog != nil && receivedLog.BlockHash == log.BlockHash {
				// chain reorg!
				receivedLog = nil
			}
		case err := <-headsSub.Err():
			t.Fatalf("Could not fund new account: %v", err)
		case err := <-logsSub.Err():
			t.Fatalf("Could not fund new account: %v", err)
		case <-timeout.C:
			t.Fatal("Could not fund new account: timeout")
		}

		if latestHeader != nil && receivedLog != nil {
			if receivedLog.BlockNumber+vaultTxConfirmationCount <= latestHeader.Number.Uint64() {
				return account
			}
		}
	}

	return account
}

func nextNonce(t *TestEnv, account common.Address) uint64 {
	nonce, err := t.Eth.NonceAt(t.Ctx(), account, nil)
	if err != nil {
		t.Fatal(err)
	}
	return nonce
}

// createAndFundAccount creates a new account that is funded from the vault contract.
// It will panic when the account could not be created and funded.
func createAndFundAccount(t *TestEnv, amount *big.Int) accounts.Account {
	account, err := keyStore.NewAccount(defaultPassword)
	if err != nil {
		panic(err)
	}
	if amount == nil {
		amount = common.Big0
	}

	// order the vault to send some ether
	tx := vaultSendSome(t, account.Address, amount)
	if err := t.Eth.SendTransaction(t.Ctx(), tx); err != nil {
		t.Fatalf("unable to send funding transaction: %v", err)
	}

	// wait for vaultTxConfirmationCount confirmation by checking the balance vaultTxConfirmationCount blocks back.
	// createAndFundAccountWithSubscription for a better solution using logs
	for i := 0; i < 120; i++ {
		block, err := t.Eth.BlockByNumber(t.Ctx(), nil)
		if err != nil {
			panic(err)
		}
		if block.NumberU64() > vaultTxConfirmationCount {
			balance, err := t.Eth.BalanceAt(t.Ctx(), account.Address, new(big.Int).Sub(block.Number(), new(big.Int).SetUint64(vaultTxConfirmationCount)))
			if err != nil {
				panic(err)
			}
			if balance.Cmp(amount) >= 0 {
				return account
			}
		}
		time.Sleep(time.Second)
	}
	panic(fmt.Sprintf("Could not fund account 0x%x in transaction 0x%x", account.Address, tx.Hash()))
}

func vaultSendSome(t *TestEnv, recipient common.Address, amount *big.Int) *types.Transaction {
	vault, _ := abi.JSON(strings.NewReader(predeployedVaultABI))
	payload, err := vault.Pack("sendSome", recipient, amount)
	if err != nil {
		t.Fatalf("can't pack pack vault tx input: %v", err)
	}
	var (
		nonce    = nextNonce(t, vaultAccountAddr)
		gasLimit = uint64(75000)
		gasPrice = new(big.Int)
		txAmount = new(big.Int)
	)
	tx := types.NewTransaction(nonce, predeployedVaultAddr, txAmount, gasLimit, gasPrice, payload)
	signer := types.NewEIP155Signer(testChainID)
	signedTx, err := types.SignTx(tx, signer, vaultKey)
	if err != nil {
		t.Fatal("can't sign vault funding tx:", err)
	}
	return signedTx
}
