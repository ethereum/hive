// Code generated - DO NOT EDIT.
// This file is a generated binding and any manual changes will be lost.

package bindings

import (
	"errors"
	"math/big"
	"strings"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/event"
)

// Reference imports to suppress errors if they are not otherwise used.
var (
	_ = errors.New
	_ = big.NewInt
	_ = strings.NewReader
	_ = ethereum.NotFound
	_ = bind.Bind
	_ = common.Big1
	_ = types.BloomLookup
	_ = event.NewSubscription
)

// FailureMetaData contains all meta data concerning the Failure contract.
var FailureMetaData = &bind.MetaData{
	ABI: "[{\"inputs\":[],\"name\":\"fail\",\"outputs\":[],\"stateMutability\":\"payable\",\"type\":\"function\"}]",
	Bin: "0x608060405234801561001057600080fd5b5061010f806100206000396000f3fe608060405260043610601c5760003560e01c8063a9cc4718146021575b600080fd5b60276029565b005b6040517f08c379a000000000000000000000000000000000000000000000000000000000815260040160599060bb565b60405180910390fd5b600082825260208201905092915050565b7f626967206661696c210000000000000000000000000000000000000000000000600082015250565b600060a76009836062565b915060b0826073565b602082019050919050565b6000602082019050818103600083015260d281609c565b905091905056fea264697066735822122068a0ae7d220da2e9f14cc48aec07eb227130c4a4ae2d89dca9c16c132f2c64fe64736f6c634300080d0033",
}

// FailureABI is the input ABI used to generate the binding from.
// Deprecated: Use FailureMetaData.ABI instead.
var FailureABI = FailureMetaData.ABI

// FailureBin is the compiled bytecode used for deploying new contracts.
// Deprecated: Use FailureMetaData.Bin instead.
var FailureBin = FailureMetaData.Bin

// DeployFailure deploys a new Ethereum contract, binding an instance of Failure to it.
func DeployFailure(auth *bind.TransactOpts, backend bind.ContractBackend) (common.Address, *types.Transaction, *Failure, error) {
	parsed, err := FailureMetaData.GetAbi()
	if err != nil {
		return common.Address{}, nil, nil, err
	}
	if parsed == nil {
		return common.Address{}, nil, nil, errors.New("GetABI returned nil")
	}

	address, tx, contract, err := bind.DeployContract(auth, *parsed, common.FromHex(FailureBin), backend)
	if err != nil {
		return common.Address{}, nil, nil, err
	}
	return address, tx, &Failure{FailureCaller: FailureCaller{contract: contract}, FailureTransactor: FailureTransactor{contract: contract}, FailureFilterer: FailureFilterer{contract: contract}}, nil
}

// Failure is an auto generated Go binding around an Ethereum contract.
type Failure struct {
	FailureCaller     // Read-only binding to the contract
	FailureTransactor // Write-only binding to the contract
	FailureFilterer   // Log filterer for contract events
}

// FailureCaller is an auto generated read-only Go binding around an Ethereum contract.
type FailureCaller struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// FailureTransactor is an auto generated write-only Go binding around an Ethereum contract.
type FailureTransactor struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// FailureFilterer is an auto generated log filtering Go binding around an Ethereum contract events.
type FailureFilterer struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// FailureSession is an auto generated Go binding around an Ethereum contract,
// with pre-set call and transact options.
type FailureSession struct {
	Contract     *Failure          // Generic contract binding to set the session for
	CallOpts     bind.CallOpts     // Call options to use throughout this session
	TransactOpts bind.TransactOpts // Transaction auth options to use throughout this session
}

// FailureCallerSession is an auto generated read-only Go binding around an Ethereum contract,
// with pre-set call options.
type FailureCallerSession struct {
	Contract *FailureCaller // Generic contract caller binding to set the session for
	CallOpts bind.CallOpts  // Call options to use throughout this session
}

// FailureTransactorSession is an auto generated write-only Go binding around an Ethereum contract,
// with pre-set transact options.
type FailureTransactorSession struct {
	Contract     *FailureTransactor // Generic contract transactor binding to set the session for
	TransactOpts bind.TransactOpts  // Transaction auth options to use throughout this session
}

// FailureRaw is an auto generated low-level Go binding around an Ethereum contract.
type FailureRaw struct {
	Contract *Failure // Generic contract binding to access the raw methods on
}

// FailureCallerRaw is an auto generated low-level read-only Go binding around an Ethereum contract.
type FailureCallerRaw struct {
	Contract *FailureCaller // Generic read-only contract binding to access the raw methods on
}

// FailureTransactorRaw is an auto generated low-level write-only Go binding around an Ethereum contract.
type FailureTransactorRaw struct {
	Contract *FailureTransactor // Generic write-only contract binding to access the raw methods on
}

// NewFailure creates a new instance of Failure, bound to a specific deployed contract.
func NewFailure(address common.Address, backend bind.ContractBackend) (*Failure, error) {
	contract, err := bindFailure(address, backend, backend, backend)
	if err != nil {
		return nil, err
	}
	return &Failure{FailureCaller: FailureCaller{contract: contract}, FailureTransactor: FailureTransactor{contract: contract}, FailureFilterer: FailureFilterer{contract: contract}}, nil
}

// NewFailureCaller creates a new read-only instance of Failure, bound to a specific deployed contract.
func NewFailureCaller(address common.Address, caller bind.ContractCaller) (*FailureCaller, error) {
	contract, err := bindFailure(address, caller, nil, nil)
	if err != nil {
		return nil, err
	}
	return &FailureCaller{contract: contract}, nil
}

// NewFailureTransactor creates a new write-only instance of Failure, bound to a specific deployed contract.
func NewFailureTransactor(address common.Address, transactor bind.ContractTransactor) (*FailureTransactor, error) {
	contract, err := bindFailure(address, nil, transactor, nil)
	if err != nil {
		return nil, err
	}
	return &FailureTransactor{contract: contract}, nil
}

// NewFailureFilterer creates a new log filterer instance of Failure, bound to a specific deployed contract.
func NewFailureFilterer(address common.Address, filterer bind.ContractFilterer) (*FailureFilterer, error) {
	contract, err := bindFailure(address, nil, nil, filterer)
	if err != nil {
		return nil, err
	}
	return &FailureFilterer{contract: contract}, nil
}

// bindFailure binds a generic wrapper to an already deployed contract.
func bindFailure(address common.Address, caller bind.ContractCaller, transactor bind.ContractTransactor, filterer bind.ContractFilterer) (*bind.BoundContract, error) {
	parsed, err := abi.JSON(strings.NewReader(FailureABI))
	if err != nil {
		return nil, err
	}
	return bind.NewBoundContract(address, parsed, caller, transactor, filterer), nil
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_Failure *FailureRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _Failure.Contract.FailureCaller.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_Failure *FailureRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _Failure.Contract.FailureTransactor.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_Failure *FailureRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _Failure.Contract.FailureTransactor.contract.Transact(opts, method, params...)
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_Failure *FailureCallerRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _Failure.Contract.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_Failure *FailureTransactorRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _Failure.Contract.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_Failure *FailureTransactorRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _Failure.Contract.contract.Transact(opts, method, params...)
}

// Fail is a paid mutator transaction binding the contract method 0xa9cc4718.
//
// Solidity: function fail() payable returns()
func (_Failure *FailureTransactor) Fail(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _Failure.contract.Transact(opts, "fail")
}

// Fail is a paid mutator transaction binding the contract method 0xa9cc4718.
//
// Solidity: function fail() payable returns()
func (_Failure *FailureSession) Fail() (*types.Transaction, error) {
	return _Failure.Contract.Fail(&_Failure.TransactOpts)
}

// Fail is a paid mutator transaction binding the contract method 0xa9cc4718.
//
// Solidity: function fail() payable returns()
func (_Failure *FailureTransactorSession) Fail() (*types.Transaction, error) {
	return _Failure.Contract.Fail(&_Failure.TransactOpts)
}
