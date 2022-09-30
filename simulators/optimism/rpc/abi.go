package main

import (
	"math/big"
	"math/rand"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/hive/optimism"
	"github.com/ethereum/hive/simulators/optimism/rpc/testcontract"
)

//go:generate abigen -abi ./contractABI.json -pkg testcontract -type Contract -out ./testcontract/contract.go

// callContractTest uses the generated ABI binding to call methods in the
// pre-deployed contract.
func callContractTest(t *LegacyTestEnv) {
	contract, err := testcontract.NewContractCaller(predeployedContractAddr, t.Eth)
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
func transactContractTest(t *LegacyTestEnv) {
	var (
		address = t.Vault.CreateAccount(t.Ctx(), t.Eth, big.NewInt(params.Ether))
		nonce   = uint64(0)

		expectedContractAddress = crypto.CreateAddress(address, nonce)
		gasPrice                = big.NewInt(30 * params.GWei)
		gasLimit                = uint64(1200000)

		contractABI, _ = abi.JSON(strings.NewReader(predeployedContractABI))
		intArg         = big.NewInt(rand.Int63())
		addrArg        = address
	)

	rawTx := types.NewContractCreation(nonce, big0, gasLimit, gasPrice, deployCode)
	deployTx, err := t.Vault.SignTransaction(address, rawTx)
	nonce++
	if err != nil {
		t.Fatalf("Unable to sign deploy tx: %v", err)
	}

	// deploy contract
	if err := t.Eth.SendTransaction(t.Ctx(), deployTx); err != nil {
		t.Fatalf("Unable to send transaction: %v", err)
	}

	t.Logf("Deploy ABI Test contract transaction: 0x%x", deployTx.Hash())

	// fetch transaction receipt for contract address
	var contractAddress common.Address
	receipt, err := optimism.WaitReceiptOK(t.Ctx(), t.Eth, deployTx.Hash())
	if err != nil {
		t.Fatalf("Unable to retrieve receipt %v: %v", deployTx.Hash(), err)
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

	eventsTx := types.NewTransaction(nonce, predeployedContractAddr, big0, 500000, gasPrice, payload)
	tx, err := t.Vault.SignTransaction(address, eventsTx)
	nonce++
	if err != nil {
		t.Fatalf("Unable to sign deploy tx: %v", err)
	}
	if err := t.Eth.SendTransaction(t.Ctx(), tx); err != nil {
		t.Fatalf("Unable to send transaction: %v", err)
	}

	t.Logf("Waiting for receipt for events tx %v", tx.Hash())

	// wait for transaction
	receipt, err = optimism.WaitReceiptOK(t.Ctx(), t.Eth, tx.Hash())
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
