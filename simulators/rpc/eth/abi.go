package main

import (
	"context"
	"math/big"
	"math/rand"
	"testing"

	"strings"

	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
)

// callContractTest uses the generated ABI binding to call methods in the
// pre-deployed contract.
func callContractTest(t *testing.T, client *TestClient) {
	t.Parallel()

	contract, err := NewTestContractCaller(predeployedContractAddr, client)
	if err != nil {
		t.Fatalf("Unable to instantiate contract caller: %v", err)
	}

	opts := &bind.CallOpts{Pending: true}
	value, err := contract.Ui(opts)
	if err != nil {
		t.Fatalf("Unable to fetch `ui` variable: %v", err)
	}

	expected, _ := new(big.Int).SetString("0x1234", 0)
	if expected.Cmp(value) != 0 {
		t.Fatalf("UI variable has invalid value, want %d, got %d", expected, value)
	}

	expected.SetString("0x1", 0)
	value, err = contract.GetFromMap(opts, predeployedContractWithAddress)
	if err != nil {
		t.Fatalf("Unable to fetch map value: %v", err)
	}
	if expected.Cmp(value) != 0 {
		t.Errorf("Invalid value retrieve from address=>uint mapping, want %d, got %d", expected, value)
	}

	expA, expB, expC := big.NewInt(1111), big.NewInt(2222), big.NewInt(3333)
	a, b, c, err := contract.ConstFunc(opts, expA, expB, expC)
	if err != nil {
		t.Fatalf("Unable to call ConstFunc: %v", err)
	}

	if expA.Cmp(a) != 0 {
		t.Errorf("A has invalid value, want %d, got %d", expA, a)
	}
	if expB.Cmp(b) != 0 {
		t.Errorf("B has invalid value, want %d, got %d", expB, b)
	}
	if expC.Cmp(c) != 0 {
		t.Errorf("C has invalid value, want %d, got %d", expC, c)
	}
}

// transactContractTest deploys a new contract and sends transactions to it and
// waits for logs.
func transactContractTest(t *testing.T, client *TestClient) {
	t.Parallel()

	var (
		account = createAndFundAccount(t, new(big.Int).Mul(common.Big1, common.Ether), client)
		address = account.Address
		nonce   = uint64(0)

		expectedContractAddress = crypto.CreateAddress(address, nonce)
		gasPrice                = new(big.Int).Mul(big.NewInt(30), common.Shannon)
		gasLimit                = big.NewInt(1200000)

		contractABI, _ = abi.JSON(strings.NewReader(predeployedContractABI))
		intArg         = big.NewInt(rand.Int63())
		addrArg        = address
	)

	rawTx := types.NewContractCreation(nonce, common.Big0, gasLimit, gasPrice, deployCode)
	deployTx, err := SignTransaction(rawTx, account)
	nonce++
	if err != nil {
		t.Fatalf("Unable to sign deploy tx: %v", err)
	}

	// deploy contract
	ctx, _ := context.WithTimeout(context.Background(), rpcTimeout)
	if err := client.SendTransaction(ctx, deployTx); err != nil {
		t.Fatalf("Unable to send transaction: %v", err)
	}

	t.Logf("Deploy ABI Test contract transaction: 0x%x", deployTx.Hash())

	// fetch transaction receipt for contract address
	var contractAddress common.Address
	receipt, err := waitForTxConfirmations(client, deployTx.Hash(), 5)
	if err != nil {
		t.Fatalf("Unable to retrieve receipt: %v", err)
	}

	// ensure receipt has the expected address
	if expectedContractAddress != receipt.ContractAddress {
		t.Fatalf("Contract deploy on different address, expected %x, got %x", expectedContractAddress, contractAddress)
	}

	t.Logf("ABI test contract deployed on 0x%x", receipt.ContractAddress)

	// send transaction to events method
	payload, err := contractABI.Pack("events", intArg, addrArg)
	if err != nil {
		t.Fatalf("Unable to prepare tx payload: %v", err)
	}

	eventsTx := types.NewTransaction(nonce, predeployedContractAddr, common.Big0, big.NewInt(500000), gasPrice, payload)
	tx, err := SignTransaction(eventsTx, account)
	nonce++
	if err != nil {
		t.Fatalf("Unable to sign deploy tx: %v", err)
	}

	ctx, _ = context.WithTimeout(context.Background(), rpcTimeout)
	if err := client.SendTransaction(ctx, tx); err != nil {
		t.Fatalf("Unable to send transaction: %v", err)
	}

	t.Logf("Wait for receipt for events tx 0x%x", tx.Hash())

	// wait for transaction
	receipt, err = waitForTxConfirmations(client, tx.Hash(), 0)
	if err != nil {
		t.Fatalf("Unable to send transaction to events method: %v", err)
	}

	var (
		intArgBytes  = common.LeftPadBytes(intArg.Bytes(), 32)
		addrArgBytes = common.LeftPadBytes(addrArg.Bytes(), 32)
	)

	if len(receipt.Logs) != 6 {
		t.Fatalf("Want 6 logs, got %d", len(receipt.Logs))
	}

	validateLog(t, tx, *receipt.Logs[0], predeployedContractAddr, receipt.Logs[0].Index+0, contractABI.Events["E0"], nil)
	validateLog(t, tx, *receipt.Logs[1], predeployedContractAddr, receipt.Logs[0].Index+1, contractABI.Events["E1"], intArgBytes)
	validateLog(t, tx, *receipt.Logs[2], predeployedContractAddr, receipt.Logs[0].Index+2, contractABI.Events["E2"], intArgBytes)
	validateLog(t, tx, *receipt.Logs[3], predeployedContractAddr, receipt.Logs[0].Index+3, contractABI.Events["E3"], addrArgBytes)
	validateLog(t, tx, *receipt.Logs[4], predeployedContractAddr, receipt.Logs[0].Index+4, contractABI.Events["E4"], addrArgBytes)
	validateLog(t, tx, *receipt.Logs[5], predeployedContractAddr, receipt.Logs[0].Index+5, contractABI.Events["E5"], intArgBytes, addrArgBytes)
}

// transactContractSubscriptionTest deploys a new contract and sends transactions to it and
// waits for logs. It uses subscription to track logs.
func transactContractSubscriptionTest(t *testing.T, client *TestClient) {
	t.Parallel()

	var (
		account = createAndFundAccountWithSubscription(t, new(big.Int).Mul(common.Big1, common.Ether), client)
		address = account.Address
		nonce   = uint64(0)

		expectedContractAddress = crypto.CreateAddress(address, nonce)
		gasPrice                = new(big.Int).Mul(big.NewInt(30), common.Shannon)
		gasLimit                = big.NewInt(1200000)

		contractABI, _ = abi.JSON(strings.NewReader(predeployedContractABI))
		intArg         = big.NewInt(rand.Int63())
		addrArg        = account.Address

		logs = make(chan types.Log)
	)

	// deploy contract
	rawTx := types.NewContractCreation(nonce, common.Big0, gasLimit, gasPrice, deployCode)
	deployTx, err := SignTransaction(rawTx, account)
	nonce++
	if err != nil {
		t.Fatalf("Unable to sign deploy tx: %v", err)
	}

	ctx, _ := context.WithTimeout(context.Background(), rpcTimeout)
	if err := client.SendTransaction(ctx, deployTx); err != nil {
		t.Fatalf("Unable to send transaction: %v", err)
	}

	t.Logf("Deploy ABI Test contract transaction: 0x%x", deployTx.Hash())

	// fetch transaction receipt for contract address
	receipt, err := waitForTxConfirmations(client, deployTx.Hash(), 5)
	if err != nil {
		t.Fatalf("Unable to retrieve receipt: %v", err)
	}

	// ensure receipt has the expected address
	if expectedContractAddress != receipt.ContractAddress {
		t.Fatalf("Contract deploy on different address, expected %x, got %x", expectedContractAddress, receipt.ContractAddress)
	}

	t.Logf("ABI test contract deployed on 0x%x", receipt.ContractAddress)

	// setup log subscription
	ctx, _ = context.WithTimeout(context.Background(), rpcTimeout)
	q := ethereum.FilterQuery{Addresses: []common.Address{receipt.ContractAddress}}
	sub, err := client.SubscribeFilterLogs(ctx, q, logs)
	if err != nil {
		t.Fatalf("Unable to create log subscription: %v", err)
	}

	contractAddress := receipt.ContractAddress
	t.Logf("Log filter created on contract 0x%x", contractAddress)

	defer sub.Unsubscribe()

	contract, err := NewTestContractTransactor(receipt.ContractAddress, client)
	if err != nil {
		t.Fatalf("Could not instantiate contract instance: %v", err)
	}

	// send transaction to events method
	opts := &bind.TransactOpts{
		From:  address,
		Nonce: new(big.Int).SetUint64(nonce),
		Signer: func(signer types.Signer, addr common.Address, tx *types.Transaction) (*types.Transaction, error) {
			return SignTransaction(tx, account)
		},
	}

	tx, err := contract.Events(opts, intArg, addrArg)
	if err != nil {
		t.Fatalf("Could not send events transaction: %v", err)
	}

	t.Logf("Send events transaction 0x%x", tx.Hash())

	// wait for logs
	var collectedLogs []types.Log
	timer := time.NewTimer(60 * time.Second)
	for len(collectedLogs) < 6 {
		select {
		case log := <-logs:
			collectedLogs = append(collectedLogs, log)
		case err := <-sub.Err():
			t.Fatalf("Received error from subscription: %v", err)
		case <-timer.C:
			t.Fatal("Waiting for logs took too long")
		}
	}

	// check logs
	var (
		intArgBytes  = common.LeftPadBytes(intArg.Bytes(), 32)
		addrArgBytes = common.LeftPadBytes(addrArg.Bytes(), 32)
	)

	validateLog(t, tx, collectedLogs[0], contractAddress, collectedLogs[0].Index+0, contractABI.Events["E0"], nil)
	validateLog(t, tx, collectedLogs[1], contractAddress, collectedLogs[0].Index+1, contractABI.Events["E1"], intArgBytes)
	validateLog(t, tx, collectedLogs[2], contractAddress, collectedLogs[0].Index+2, contractABI.Events["E2"], intArgBytes)
	validateLog(t, tx, collectedLogs[3], contractAddress, collectedLogs[0].Index+3, contractABI.Events["E3"], addrArgBytes)
	validateLog(t, tx, collectedLogs[4], contractAddress, collectedLogs[0].Index+4, contractABI.Events["E4"], addrArgBytes)
	validateLog(t, tx, collectedLogs[5], contractAddress, collectedLogs[0].Index+5, contractABI.Events["E5"], intArgBytes, addrArgBytes)
}
