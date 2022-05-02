package main

import (
	"bytes"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/params"
)

var (
	// test contract deploy code, will deploy the contract with 1234 as argument
	deployCode = common.Hex2Bytes("6060604052346100005760405160208061048c833981016040528080519060200190919050505b8060008190555080600160003373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001908152602001600020819055505b505b610409806100836000396000f30060606040526000357c0100000000000000000000000000000000000000000000000000000000900463ffffffff168063a223e05d1461006a578063abd1a0cf1461008d578063abfced1d146100d4578063e05c914a14610110578063e6768b451461014c575b610000565b346100005761007761019d565b6040518082815260200191505060405180910390f35b34610000576100be600480803573ffffffffffffffffffffffffffffffffffffffff169060200190919050506101a3565b6040518082815260200191505060405180910390f35b346100005761010e600480803573ffffffffffffffffffffffffffffffffffffffff169060200190919080359060200190919050506101ed565b005b346100005761014a600480803590602001909190803573ffffffffffffffffffffffffffffffffffffffff16906020019091905050610236565b005b346100005761017960048080359060200190919080359060200190919080359060200190919050506103c4565b60405180848152602001838152602001828152602001935050505060405180910390f35b60005481565b6000600160008373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020019081526020016000205490505b919050565b80600160008473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001908152602001600020819055505b5050565b7f6031a8d62d7c95988fa262657cd92107d90ed96e08d8f867d32f26edfe85502260405180905060405180910390a17f47e2689743f14e97f7dcfa5eec10ba1dff02f83b3d1d4b9c07b206cbbda66450826040518082815260200191505060405180910390a1817fa48a6b249a5084126c3da369fbc9b16827ead8cb5cdc094b717d3f1dcd995e2960405180905060405180910390a27f7890603b316f3509577afd111710f9ebeefa15e12f72347d9dffd0d65ae3bade81604051808273ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200191505060405180910390a18073ffffffffffffffffffffffffffffffffffffffff167f7efef9ea3f60ddc038e50cccec621f86a0195894dc0520482abf8b5c6b659e4160405180905060405180910390a28181604051808381526020018273ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020019250505060405180910390a05b5050565b6000600060008585859250925092505b935093509390505600a165627a7a72305820aaf842d0d0c35c45622c5263cbb54813d2974d3999c8c38551d7c613ea2bc117002900000000000000000000000000000000000000000000000000000000000004d2")
	// test contract code as deployed
	runtimeCode = common.Hex2Bytes("60606040526000357c0100000000000000000000000000000000000000000000000000000000900463ffffffff168063a223e05d1461006a578063abd1a0cf1461008d578063abfced1d146100d4578063e05c914a14610110578063e6768b451461014c575b610000565b346100005761007761019d565b6040518082815260200191505060405180910390f35b34610000576100be600480803573ffffffffffffffffffffffffffffffffffffffff169060200190919050506101a3565b6040518082815260200191505060405180910390f35b346100005761010e600480803573ffffffffffffffffffffffffffffffffffffffff169060200190919080359060200190919050506101ed565b005b346100005761014a600480803590602001909190803573ffffffffffffffffffffffffffffffffffffffff16906020019091905050610236565b005b346100005761017960048080359060200190919080359060200190919080359060200190919050506103c4565b60405180848152602001838152602001828152602001935050505060405180910390f35b60005481565b6000600160008373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020019081526020016000205490505b919050565b80600160008473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001908152602001600020819055505b5050565b7f6031a8d62d7c95988fa262657cd92107d90ed96e08d8f867d32f26edfe85502260405180905060405180910390a17f47e2689743f14e97f7dcfa5eec10ba1dff02f83b3d1d4b9c07b206cbbda66450826040518082815260200191505060405180910390a1817fa48a6b249a5084126c3da369fbc9b16827ead8cb5cdc094b717d3f1dcd995e2960405180905060405180910390a27f7890603b316f3509577afd111710f9ebeefa15e12f72347d9dffd0d65ae3bade81604051808273ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200191505060405180910390a18073ffffffffffffffffffffffffffffffffffffffff167f7efef9ea3f60ddc038e50cccec621f86a0195894dc0520482abf8b5c6b659e4160405180905060405180910390a28181604051808381526020018273ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020019250505060405180910390a05b5050565b6000600060008585859250925092505b935093509390505600a165627a7a72305820aaf842d0d0c35c45622c5263cbb54813d2974d3999c8c38551d7c613ea2bc1170029")
)

var (
	big0 = new(big.Int)
	big1 = big.NewInt(1)
)

// genesisByHash fetches the known genesis header and compares
// it against the genesis file to determine if block fields are
// returned correct.
func genesisHeaderByHashTest(t *TestEnv) {
	gblock := t.LoadGenesis()

	headerByHash, err := t.Eth.HeaderByHash(t.Ctx(), gblock.Hash())
	if err != nil {
		t.Fatalf("Unable to fetch block %x: %v", gblock.Hash(), err)
	}
	if d := diff(gblock.Header(), headerByHash); d != "" {
		t.Fatal("genesis header reported by node differs from expected header:\n", d)
	}
}

// headerByNumberTest fetched the known genesis header and compares
// it against the genesis file to determine if block fields are
// returned correct.
func genesisHeaderByNumberTest(t *TestEnv) {
	gblock := t.LoadGenesis()

	headerByNum, err := t.Eth.HeaderByNumber(t.Ctx(), big0)
	if err != nil {
		t.Fatalf("Unable to fetch genesis block: %v", err)
	}
	if d := diff(gblock.Header(), headerByNum); d != "" {
		t.Fatal("genesis header reported by node differs from expected header:\n", d)
	}
}

// genesisBlockByHashTest fetched the known genesis block and compares it against
// the genesis file to determine if block fields are returned correct.
func genesisBlockByHashTest(t *TestEnv) {
	gblock := t.LoadGenesis()

	blockByHash, err := t.Eth.BlockByHash(t.Ctx(), gblock.Hash())
	if err != nil {
		t.Fatalf("Unable to fetch block %x: %v", gblock.Hash(), err)
	}
	if d := diff(gblock.Header(), blockByHash.Header()); d != "" {
		t.Fatal("genesis header reported by node differs from expected header:\n", d)
	}
}

// genesisBlockByNumberTest retrieves block 0 since that is the only block
// that is known through the genesis.json file and tests if block
// fields matches the fields defined in the genesis file.
func genesisBlockByNumberTest(t *TestEnv) {
	gblock := t.LoadGenesis()

	blockByNum, err := t.Eth.BlockByNumber(t.Ctx(), big0)
	if err != nil {
		t.Fatalf("Unable to fetch genesis block: %v", err)
	}
	if d := diff(gblock.Header(), blockByNum.Header()); d != "" {
		t.Fatal("genesis header reported by node differs from expected header:\n", d)
	}
}


// deployContractTest deploys `contractSrc` and tests if the code and state
// on the contract address contain the expected values (as set in the ctor).
func deployContractTest(t *TestEnv) {
	var (
		address = t.Vault.createAccount(t, big.NewInt(params.Ether))
		nonce   = uint64(0)

		expectedContractAddress = crypto.CreateAddress(address, nonce)
		gasLimit                = uint64(1200000)
	)

	rawTx := types.NewContractCreation(nonce, big0, gasLimit, gasPrice, deployCode)
	deployTx, err := t.Vault.signTransaction(address, rawTx)
	if err != nil {
		t.Fatalf("Unable to sign deploy tx: %v", err)
	}

	// deploy contract
	if err := t.Eth.SendTransaction(t.Ctx(), deployTx); err != nil {
		t.Fatalf("Unable to send transaction: %v", err)
	}

	t.Logf("Deploy transaction: 0x%x", deployTx.Hash())

	// fetch transaction receipt for contract address
	var contractAddress common.Address
	receipt, err := waitForTxConfirmations(t, deployTx.Hash(), 5)
	if err != nil {
		t.Fatalf("Unable to retrieve receipt: %v", err)
	}

	// ensure receipt has the expected address
	if expectedContractAddress != receipt.ContractAddress {
		t.Fatalf("Contract deploy on different address, expected %x, got %x", expectedContractAddress, contractAddress)
	}

	// test deployed code matches runtime code
	code, err := t.Eth.CodeAt(t.Ctx(), receipt.ContractAddress, nil)
	if err != nil {
		t.Fatalf("Unable to fetch contract code: %v", err)
	}
	if bytes.Compare(runtimeCode, code) != 0 {
		t.Errorf("Deployed code doesn't match, expected %x, got %x", runtimeCode, code)
	}

	// test contract state, pos 0 must be 1234
	value, err := t.Eth.StorageAt(t.Ctx(), receipt.ContractAddress, common.Hash{}, nil)
	if err == nil {
		v := new(big.Int).SetBytes(value)
		if v.Uint64() != 1234 {
			t.Errorf("Unexpected value on %x:0x01, expected 1234, got %d", receipt.ContractAddress, v)
		}
	} else {
		t.Errorf("Unable to retrieve storage pos 0x01 on address %x: %v", contractAddress, err)
	}

	// test contract state, map on pos 1 with key myAccount must be 1234
	storageKey := make([]byte, 64)
	copy(storageKey[12:32], address.Bytes())
	storageKey[63] = 1
	storageKey = crypto.Keccak256(storageKey)

	value, err = t.Eth.StorageAt(t.Ctx(), receipt.ContractAddress, common.BytesToHash(storageKey), nil)
	if err == nil {
		v := new(big.Int).SetBytes(value)
		if v.Uint64() != 1234 {
			t.Errorf("Unexpected value in map, expected 1234, got %d", v)
		}
	} else {
		t.Fatalf("Unable to retrieve value in map: %v", err)
	}
}

// deployContractOutOfGasTest tries to deploy `contractSrc` with insufficient gas. It
// checks the receipts reflects the "out of gas" event and code / state isn't created in
// the contract address.
func deployContractOutOfGasTest(t *TestEnv) {
	var (
		address         = t.Vault.createAccount(t, big.NewInt(params.Ether))
		nonce           = uint64(0)
		contractAddress = crypto.CreateAddress(address, nonce)
		gasLimit        = uint64(240000) // insufficient gas
	)
	t.Logf("calculated contract address: %x", contractAddress)

	// Deploy the contract.
	rawTx := types.NewContractCreation(nonce, big0, gasLimit, gasPrice, deployCode)
	deployTx, err := t.Vault.signTransaction(address, rawTx)
	if err != nil {
		t.Fatalf("unable to sign deploy tx: %v", err)
	}
	t.Logf("out of gas tx: %x", deployTx.Hash())
	if err := t.Eth.SendTransaction(t.Ctx(), deployTx); err != nil {
		t.Fatalf("unable to send transaction: %v", err)
	}

	// Wait for the transaction receipt.
	receipt, err := waitForTxConfirmations(t, deployTx.Hash(), 5)
	if err != nil {
		t.Fatalf("unable to fetch tx receipt: %v", err)
	}
	// Check receipt fields.
	if receipt.Status != types.ReceiptStatusFailed {
		t.Errorf("receipt has status %d, want %d", receipt.Status, types.ReceiptStatusFailed)
	}
	if receipt.GasUsed != gasLimit {
		t.Errorf("receipt has gasUsed %d, want %d", receipt.GasUsed, gasLimit)
	}
	if receipt.ContractAddress != contractAddress {
		t.Errorf("receipt has contract address %x, want %x", receipt.ContractAddress, contractAddress)
	}
	if receipt.BlockHash == (common.Hash{}) {
		t.Errorf("receipt has empty block hash", receipt.BlockHash)
	}
	// Check that nothing is deployed at the contract address.
	code, err := t.Eth.CodeAt(t.Ctx(), contractAddress, nil)
	if err != nil {
		t.Fatalf("unable to fetch code: %v", err)
	}
	if len(code) != 0 {
		t.Errorf("expected no code deployed but got %x", code)
	}
}

// syncProgressTest only tests if this function is supported by the node.
func syncProgressTest(t *TestEnv) {
	_, err := t.Eth.SyncProgress(t.Ctx())
	if err != nil {
		t.Fatalf("Unable to determine sync progress: %v", err)
	}
}

// newHeadSubscriptionTest tests whether
func newHeadSubscriptionTest(t *TestEnv) {
	var (
		heads = make(chan *types.Header)
	)

	sub, err := t.Eth.SubscribeNewHead(t.Ctx(), heads)
	if err != nil {
		t.Fatalf("Unable to subscribe to new heads: %v", err)
	}

	defer sub.Unsubscribe()
	for i := 0; i < 10; i++ {
		select {
		case newHead := <-heads:
			header, err := t.Eth.HeaderByHash(t.Ctx(), newHead.Hash())
			if err != nil {
				t.Fatalf("Unable to fetch header: %v", err)
			}
			if header == nil {
				t.Fatalf("Unable to fetch header %s", newHead.Hash())
			}
		case err := <-sub.Err():
			t.Fatalf("Received errors: %v", err)
		}
	}
}

// balanceAndNonceAtTest creates a new account and transfers funds to it.
// It then tests if the balance and nonce of the sender and receiver
// address are updated correct.
func balanceAndNonceAtTest(t *TestEnv) {
	var (
		sourceAddr  = t.Vault.createAccount(t, big.NewInt(params.Ether))
		sourceNonce = uint64(0)
		targetAddr  = t.Vault.createAccount(t, nil)
	)

	// Get current balance
	sourceAddressBalanceBefore, err := t.Eth.BalanceAt(t.Ctx(), sourceAddr, nil)
	if err != nil {
		t.Fatalf("Unable to retrieve balance: %v", err)
	}

	expected := big.NewInt(params.Ether)
	if sourceAddressBalanceBefore.Cmp(expected) != 0 {
		t.Errorf("Expected balance %d, got %d", expected, sourceAddressBalanceBefore)
	}

	nonceBefore, err := t.Eth.NonceAt(t.Ctx(), sourceAddr, nil)
	if err != nil {
		t.Fatalf("Unable to determine nonce: %v", err)
	}
	if nonceBefore != sourceNonce {
		t.Fatalf("Invalid nonce, want %d, got %d", sourceNonce, nonceBefore)
	}

	// send 1234 wei to target account and verify balances and nonces are updated
	var (
		amount   = big.NewInt(1234)
		gasLimit = uint64(50000)
	)
	rawTx := types.NewTransaction(sourceNonce, targetAddr, amount, gasLimit, gasPrice, nil)
	valueTx, err := t.Vault.signTransaction(sourceAddr, rawTx)
	if err != nil {
		t.Fatalf("Unable to sign value tx: %v", err)
	}
	sourceNonce++

	t.Logf("BalanceAt: send %d wei from 0x%x to 0x%x in 0x%x", valueTx.Value(), sourceAddr, targetAddr, valueTx.Hash())
	if err := t.Eth.SendTransaction(t.Ctx(), valueTx); err != nil {
		t.Fatalf("Unable to send transaction: %v", err)
	}

	var receipt *types.Receipt
	for {
		receipt, err = t.Eth.TransactionReceipt(t.Ctx(), valueTx.Hash())
		if receipt != nil {
			break
		}
		if err != ethereum.NotFound {
			t.Fatalf("Could not fetch receipt for 0x%x: %v", valueTx.Hash(), err)
		}
		time.Sleep(time.Second)
	}

	// ensure balances have been updated
	accountBalanceAfter, err := t.Eth.BalanceAt(t.Ctx(), sourceAddr, nil)
	if err != nil {
		t.Fatalf("Unable to retrieve balance: %v", err)
	}
	balanceTargetAccountAfter, err := t.Eth.BalanceAt(t.Ctx(), targetAddr, nil)
	if err != nil {
		t.Fatalf("Unable to retrieve balance: %v", err)
	}

	// expected balance is previous balance - tx amount - tx fee (gasUsed * gasPrice)
	exp := new(big.Int).Set(sourceAddressBalanceBefore)
	exp.Sub(exp, amount)
	exp.Sub(exp, new(big.Int).Mul(big.NewInt(int64(receipt.GasUsed)), valueTx.GasPrice()))

	if exp.Cmp(accountBalanceAfter) != 0 {
		t.Errorf("Expected sender account to have a balance of %d, got %d", exp, accountBalanceAfter)
	}
	if balanceTargetAccountAfter.Cmp(amount) != 0 {
		t.Errorf("Expected new account to have a balance of %d, got %d", valueTx.Value(), balanceTargetAccountAfter)
	}

	// ensure nonce is incremented by 1
	nonceAfter, err := t.Eth.NonceAt(t.Ctx(), sourceAddr, nil)
	if err != nil {
		t.Fatalf("Unable to determine nonce: %v", err)
	}
	expectedNonce := nonceBefore + 1
	if expectedNonce != nonceAfter {
		t.Fatalf("Invalid nonce, want %d, got %d", expectedNonce, nonceAfter)
	}
}
