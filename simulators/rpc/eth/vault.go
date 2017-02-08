package main

import (
	"context"
	"fmt"
	"math/big"
	"strings"
	"testing"
	"time"

	"os"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/keystore"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
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

func init() {
	keyStore = keystore.NewKeyStore(os.TempDir(), keystore.StandardScryptN, keystore.StandardScryptP)
	accountsManager = accounts.NewManager(keyStore)
}

// nodeAddress returns the first account from the node and unlocks it.
func nodeAddress(t *testing.T, client *TestClient) common.Address {
	ctx, _ := context.WithTimeout(context.Background(), rpcTimeout)
	var accounts []common.Address
	if err := client.CallContext(ctx, &accounts, "eth_accounts"); err != nil {
		panic(err)
	}

	addr := accounts[0]

	ctx, _ = context.WithTimeout(context.Background(), rpcTimeout)
	var success bool
	if err := client.CallContext(ctx, &success, "personal_unlockAccount", addr, "", 3600); err != nil {
		t.Fatalf("Unable to unlock account 0x%x: %v", addr, err)
	}
	if !success {
		t.Fatalf("Unable to unlock node account 0x%x", addr)
	}

	return addr
}

// createAndFundAccount creates a new account that is funded from the vault contract.
// It will panic when the account could not be created and funded.
func createAndFundAccountWithSubscription(t *testing.T, amount *big.Int, client *TestClient) accounts.Account {
	account, err := keyStore.NewAccount(defaultPassword)
	if err != nil {
		panic(err)
	}

	// each node has at least one 1 key with some of pre-allocated ether
	fromAddr := nodeAddress(t, client)

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

	ctx, _ = context.WithTimeout(context.Background(), rpcTimeout)
	headsSub, err = client.SubscribeNewHead(ctx, heads)
	if err != nil {
		panic(fmt.Sprintf("Could not create new head subscription: %v", err))
	}
	defer headsSub.Unsubscribe()

	eventTopic := vault.Events["Send"].Id()
	addressTopic := common.BytesToHash(common.LeftPadBytes(account.Address[:], 32))
	ctx, _ = context.WithTimeout(context.Background(), rpcTimeout)
	q := ethereum.FilterQuery{
		Addresses: []common.Address{predeployedVaultAddr},
		Topics:    [][]common.Hash{[]common.Hash{eventTopic}, []common.Hash{addressTopic}},
	}
	logsSub, err = client.SubscribeFilterLogs(ctx, q, logs)
	if err != nil {
		panic(fmt.Sprintf("Could not create log filter subscription: %v", err))
	}
	defer logsSub.Unsubscribe()

	// order the vault to send some ether
	payload, err := vault.Pack("sendSome", account.Address, amount)
	if err != nil {
		t.Fatalf("Unable to send some ether to new account: %v", err)
	}

	txPayload := map[string]interface{}{
		"from": fromAddr,
		"to":   predeployedVaultAddr,
		"data": hexutil.Bytes(payload),
		"gas":  hexutil.EncodeBig(big.NewInt(75000)),
	}

	ctx, _ = context.WithTimeout(context.Background(), rpcTimeout)
	var txHash common.Hash
	if err := client.CallContext(ctx, &txHash, "eth_sendTransaction", txPayload); err != nil {
		t.Fatalf("Unable to send funding transaction: %v", err)
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
			} else if log.Removed && receivedLog != nil && receivedLog.BlockHash == log.BlockHash { // chain reorg
				receivedLog = nil
			}
		case err := <-headsSub.Err():
			t.Fatalf("Could not fund new account: %v", err)
		case err := <-logsSub.Err():
			t.Fatalf("Could not not fund new account: %v", err)
		case <-timeout.C:
			t.Fatal("Could not not fund new account: timeout")
		}

		if latestHeader != nil && receivedLog != nil {
			if receivedLog.BlockNumber+vaultTxConfirmationCount <= latestHeader.Number.Uint64() {
				return account
			}
		}
	}

	return account
}

// createAndFundAccount creates a new account that is funded from the vault contract.
// It will panic when the account could not be created and funded.
func createAndFundAccount(t *testing.T, amount *big.Int, client *TestClient) accounts.Account {
	account, err := keyStore.NewAccount(defaultPassword)
	if err != nil {
		panic(err)
	}

	// each node has at least one 1 key with some of pre-allocated ether
	fromAddr := nodeAddress(t, client)

	if amount == nil {
		amount = common.Big0
	}

	// order the vault to send some ether
	vault, _ := abi.JSON(strings.NewReader(predeployedVaultABI))
	payload, err := vault.Pack("sendSome", account.Address, amount)
	if err != nil {
		t.Fatalf("Unable to send some ether to new account: %v", err)
	}

	txPayload := map[string]interface{}{
		"from": fromAddr,
		"to":   predeployedVaultAddr,
		"data": hexutil.Bytes(payload),
		"gas":  hexutil.EncodeBig(big.NewInt(75000)),
	}

	ctx, _ := context.WithTimeout(context.Background(), rpcTimeout)
	var txHash common.Hash
	if err := client.CallContext(ctx, &txHash, "eth_sendTransaction", txPayload); err != nil {
		t.Fatalf("Unable to send funding transaction: %v", err)
	}

	// wait for vaultTxConfirmationCount confirmation by checking the balance vaultTxConfirmationCount blocks back.
	// createAndFundAccountWithSubscription for a better solution using logs
	for i := 0; i < 120; i++ {
		ctx, _ := context.WithTimeout(context.Background(), rpcTimeout)
		block, err := client.BlockByNumber(ctx, nil)
		if err != nil {
			panic(err)
		}

		if block.NumberU64() > vaultTxConfirmationCount {
			ctx, _ := context.WithTimeout(context.Background(), rpcTimeout)
			balance, err := client.BalanceAt(ctx, account.Address, new(big.Int).Sub(block.Number(), new(big.Int).SetUint64(vaultTxConfirmationCount)))
			if err != nil {
				panic(err)
			}
			if balance.Cmp(amount) >= 0 {
				return account
			}
		}

		time.Sleep(time.Second)
	}

	panic(fmt.Sprintf("Could not fund account 0x%x in transaction 0x%x", account.Address, txHash))
}
