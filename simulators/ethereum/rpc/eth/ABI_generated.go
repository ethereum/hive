// This file is an automatically generated Go binding. Do not modify as any
// change will likely be lost upon the next re-generation!

package main

import (
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

// TestContractABI is the input ABI used to generate the binding from.
const TestContractABI = "[{\"constant\":true,\"inputs\":[],\"name\":\"ui\",\"outputs\":[{\"name\":\"\",\"type\":\"uint256\"}],\"payable\":false,\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"name\":\"addr\",\"type\":\"address\"}],\"name\":\"getFromMap\",\"outputs\":[{\"name\":\"\",\"type\":\"uint256\"}],\"payable\":false,\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"addr\",\"type\":\"address\"},{\"name\":\"value\",\"type\":\"uint256\"}],\"name\":\"addToMap\",\"outputs\":[],\"payable\":false,\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"ui_\",\"type\":\"uint256\"},{\"name\":\"addr_\",\"type\":\"address\"}],\"name\":\"events\",\"outputs\":[],\"payable\":false,\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"name\":\"a\",\"type\":\"uint256\"},{\"name\":\"b\",\"type\":\"uint256\"},{\"name\":\"c\",\"type\":\"uint256\"}],\"name\":\"constFunc\",\"outputs\":[{\"name\":\"\",\"type\":\"uint256\"},{\"name\":\"\",\"type\":\"uint256\"},{\"name\":\"\",\"type\":\"uint256\"}],\"payable\":false,\"type\":\"function\"},{\"inputs\":[{\"name\":\"ui_\",\"type\":\"uint256\"}],\"payable\":false,\"type\":\"constructor\"},{\"anonymous\":false,\"inputs\":[],\"name\":\"E0\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"name\":\"\",\"type\":\"uint256\"}],\"name\":\"E1\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"name\":\"\",\"type\":\"uint256\"}],\"name\":\"E2\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"name\":\"\",\"type\":\"address\"}],\"name\":\"E3\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"name\":\"\",\"type\":\"address\"}],\"name\":\"E4\",\"type\":\"event\"},{\"anonymous\":true,\"inputs\":[{\"indexed\":false,\"name\":\"\",\"type\":\"uint256\"},{\"indexed\":false,\"name\":\"\",\"type\":\"address\"}],\"name\":\"E5\",\"type\":\"event\"}]"

// TestContract is an auto generated Go binding around an Ethereum contract.
type TestContract struct {
	TestContractCaller     // Read-only binding to the contract
	TestContractTransactor // Write-only binding to the contract
}

// TestContractCaller is an auto generated read-only Go binding around an Ethereum contract.
type TestContractCaller struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// TestContractTransactor is an auto generated write-only Go binding around an Ethereum contract.
type TestContractTransactor struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// TestContractSession is an auto generated Go binding around an Ethereum contract,
// with pre-set call and transact options.
type TestContractSession struct {
	Contract     *TestContract     // Generic contract binding to set the session for
	CallOpts     bind.CallOpts     // Call options to use throughout this session
	TransactOpts bind.TransactOpts // Transaction auth options to use throughout this session
}

// TestContractCallerSession is an auto generated read-only Go binding around an Ethereum contract,
// with pre-set call options.
type TestContractCallerSession struct {
	Contract *TestContractCaller // Generic contract caller binding to set the session for
	CallOpts bind.CallOpts       // Call options to use throughout this session
}

// TestContractTransactorSession is an auto generated write-only Go binding around an Ethereum contract,
// with pre-set transact options.
type TestContractTransactorSession struct {
	Contract     *TestContractTransactor // Generic contract transactor binding to set the session for
	TransactOpts bind.TransactOpts       // Transaction auth options to use throughout this session
}

// TestContractRaw is an auto generated low-level Go binding around an Ethereum contract.
type TestContractRaw struct {
	Contract *TestContract // Generic contract binding to access the raw methods on
}

// TestContractCallerRaw is an auto generated low-level read-only Go binding around an Ethereum contract.
type TestContractCallerRaw struct {
	Contract *TestContractCaller // Generic read-only contract binding to access the raw methods on
}

// TestContractTransactorRaw is an auto generated low-level write-only Go binding around an Ethereum contract.
type TestContractTransactorRaw struct {
	Contract *TestContractTransactor // Generic write-only contract binding to access the raw methods on
}

// NewTestContract creates a new instance of TestContract, bound to a specific deployed contract.
func NewTestContract(address common.Address, backend bind.ContractBackend) (*TestContract, error) {
	contract, err := bindTestContract(address, backend, backend)
	if err != nil {
		return nil, err
	}
	return &TestContract{TestContractCaller: TestContractCaller{contract: contract}, TestContractTransactor: TestContractTransactor{contract: contract}}, nil
}

// NewTestContractCaller creates a new read-only instance of TestContract, bound to a specific deployed contract.
func NewTestContractCaller(address common.Address, caller bind.ContractCaller) (*TestContractCaller, error) {
	contract, err := bindTestContract(address, caller, nil)
	if err != nil {
		return nil, err
	}
	return &TestContractCaller{contract: contract}, nil
}

// NewTestContractTransactor creates a new write-only instance of TestContract, bound to a specific deployed contract.
func NewTestContractTransactor(address common.Address, transactor bind.ContractTransactor) (*TestContractTransactor, error) {
	contract, err := bindTestContract(address, nil, transactor)
	if err != nil {
		return nil, err
	}
	return &TestContractTransactor{contract: contract}, nil
}

// bindTestContract binds a generic wrapper to an already deployed contract.
func bindTestContract(address common.Address, caller bind.ContractCaller, transactor bind.ContractTransactor) (*bind.BoundContract, error) {
	parsed, err := abi.JSON(strings.NewReader(TestContractABI))
	if err != nil {
		return nil, err
	}
	return bind.NewBoundContract(address, parsed, caller, transactor), nil
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_TestContract *TestContractRaw) Call(opts *bind.CallOpts, result interface{}, method string, params ...interface{}) error {
	return _TestContract.Contract.TestContractCaller.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_TestContract *TestContractRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _TestContract.Contract.TestContractTransactor.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_TestContract *TestContractRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _TestContract.Contract.TestContractTransactor.contract.Transact(opts, method, params...)
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_TestContract *TestContractCallerRaw) Call(opts *bind.CallOpts, result interface{}, method string, params ...interface{}) error {
	return _TestContract.Contract.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_TestContract *TestContractTransactorRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _TestContract.Contract.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_TestContract *TestContractTransactorRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _TestContract.Contract.contract.Transact(opts, method, params...)
}

// ConstFunc is a free data retrieval call binding the contract method 0xe6768b45.
//
// Solidity: function constFunc(a uint256, b uint256, c uint256) constant returns(uint256, uint256, uint256)
func (_TestContract *TestContractCaller) ConstFunc(opts *bind.CallOpts, a *big.Int, b *big.Int, c *big.Int) (*big.Int, *big.Int, *big.Int, error) {
	var (
		ret0 = new(*big.Int)
		ret1 = new(*big.Int)
		ret2 = new(*big.Int)
	)
	out := &[]interface{}{
		ret0,
		ret1,
		ret2,
	}
	err := _TestContract.contract.Call(opts, out, "constFunc", a, b, c)
	return *ret0, *ret1, *ret2, err
}

// ConstFunc is a free data retrieval call binding the contract method 0xe6768b45.
//
// Solidity: function constFunc(a uint256, b uint256, c uint256) constant returns(uint256, uint256, uint256)
func (_TestContract *TestContractSession) ConstFunc(a *big.Int, b *big.Int, c *big.Int) (*big.Int, *big.Int, *big.Int, error) {
	return _TestContract.Contract.ConstFunc(&_TestContract.CallOpts, a, b, c)
}

// ConstFunc is a free data retrieval call binding the contract method 0xe6768b45.
//
// Solidity: function constFunc(a uint256, b uint256, c uint256) constant returns(uint256, uint256, uint256)
func (_TestContract *TestContractCallerSession) ConstFunc(a *big.Int, b *big.Int, c *big.Int) (*big.Int, *big.Int, *big.Int, error) {
	return _TestContract.Contract.ConstFunc(&_TestContract.CallOpts, a, b, c)
}

// GetFromMap is a free data retrieval call binding the contract method 0xabd1a0cf.
//
// Solidity: function getFromMap(addr address) constant returns(uint256)
func (_TestContract *TestContractCaller) GetFromMap(opts *bind.CallOpts, addr common.Address) (*big.Int, error) {
	var (
		ret0 = new(*big.Int)
	)
	out := ret0
	err := _TestContract.contract.Call(opts, out, "getFromMap", addr)
	return *ret0, err
}

// GetFromMap is a free data retrieval call binding the contract method 0xabd1a0cf.
//
// Solidity: function getFromMap(addr address) constant returns(uint256)
func (_TestContract *TestContractSession) GetFromMap(addr common.Address) (*big.Int, error) {
	return _TestContract.Contract.GetFromMap(&_TestContract.CallOpts, addr)
}

// GetFromMap is a free data retrieval call binding the contract method 0xabd1a0cf.
//
// Solidity: function getFromMap(addr address) constant returns(uint256)
func (_TestContract *TestContractCallerSession) GetFromMap(addr common.Address) (*big.Int, error) {
	return _TestContract.Contract.GetFromMap(&_TestContract.CallOpts, addr)
}

// Ui is a free data retrieval call binding the contract method 0xa223e05d.
//
// Solidity: function ui() constant returns(uint256)
func (_TestContract *TestContractCaller) Ui(opts *bind.CallOpts) (*big.Int, error) {
	var (
		ret0 = new(*big.Int)
	)
	out := ret0
	err := _TestContract.contract.Call(opts, out, "ui")
	return *ret0, err
}

// Ui is a free data retrieval call binding the contract method 0xa223e05d.
//
// Solidity: function ui() constant returns(uint256)
func (_TestContract *TestContractSession) Ui() (*big.Int, error) {
	return _TestContract.Contract.Ui(&_TestContract.CallOpts)
}

// Ui is a free data retrieval call binding the contract method 0xa223e05d.
//
// Solidity: function ui() constant returns(uint256)
func (_TestContract *TestContractCallerSession) Ui() (*big.Int, error) {
	return _TestContract.Contract.Ui(&_TestContract.CallOpts)
}

// AddToMap is a paid mutator transaction binding the contract method 0xabfced1d.
//
// Solidity: function addToMap(addr address, value uint256) returns()
func (_TestContract *TestContractTransactor) AddToMap(opts *bind.TransactOpts, addr common.Address, value *big.Int) (*types.Transaction, error) {
	return _TestContract.contract.Transact(opts, "addToMap", addr, value)
}

// AddToMap is a paid mutator transaction binding the contract method 0xabfced1d.
//
// Solidity: function addToMap(addr address, value uint256) returns()
func (_TestContract *TestContractSession) AddToMap(addr common.Address, value *big.Int) (*types.Transaction, error) {
	return _TestContract.Contract.AddToMap(&_TestContract.TransactOpts, addr, value)
}

// AddToMap is a paid mutator transaction binding the contract method 0xabfced1d.
//
// Solidity: function addToMap(addr address, value uint256) returns()
func (_TestContract *TestContractTransactorSession) AddToMap(addr common.Address, value *big.Int) (*types.Transaction, error) {
	return _TestContract.Contract.AddToMap(&_TestContract.TransactOpts, addr, value)
}

// Events is a paid mutator transaction binding the contract method 0xe05c914a.
//
// Solidity: function events(ui_ uint256, addr_ address) returns()
func (_TestContract *TestContractTransactor) Events(opts *bind.TransactOpts, ui_ *big.Int, addr_ common.Address) (*types.Transaction, error) {
	return _TestContract.contract.Transact(opts, "events", ui_, addr_)
}

// Events is a paid mutator transaction binding the contract method 0xe05c914a.
//
// Solidity: function events(ui_ uint256, addr_ address) returns()
func (_TestContract *TestContractSession) Events(ui_ *big.Int, addr_ common.Address) (*types.Transaction, error) {
	return _TestContract.Contract.Events(&_TestContract.TransactOpts, ui_, addr_)
}

// Events is a paid mutator transaction binding the contract method 0xe05c914a.
//
// Solidity: function events(ui_ uint256, addr_ address) returns()
func (_TestContract *TestContractTransactorSession) Events(ui_ *big.Int, addr_ common.Address) (*types.Transaction, error) {
	return _TestContract.Contract.Events(&_TestContract.TransactOpts, ui_, addr_)
}
