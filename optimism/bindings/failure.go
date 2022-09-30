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
	ABI: "[{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"ok\",\"type\":\"uint256\"}],\"name\":\"Yep\",\"type\":\"event\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"_amount\",\"type\":\"uint256\"}],\"name\":\"burn\",\"outputs\":[],\"stateMutability\":\"payable\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"fail\",\"outputs\":[],\"stateMutability\":\"payable\",\"type\":\"function\"}]",
	Bin: "0x608060405234801561001057600080fd5b50610285806100206000396000f3fe6080604052600436106100295760003560e01c806342966c681461002e578063a9cc47181461004a575b600080fd5b610048600480360381019061004391906100fa565b610054565b005b610052610084565b005b6000805a90505b825a826100689190610156565b101561007f57816100789061018a565b915061005b565b505050565b6040517f08c379a00000000000000000000000000000000000000000000000000000000081526004016100b69061022f565b60405180910390fd5b600080fd5b6000819050919050565b6100d7816100c4565b81146100e257600080fd5b50565b6000813590506100f4816100ce565b92915050565b6000602082840312156101105761010f6100bf565b5b600061011e848285016100e5565b91505092915050565b7f4e487b7100000000000000000000000000000000000000000000000000000000600052601160045260246000fd5b6000610161826100c4565b915061016c836100c4565b92508282101561017f5761017e610127565b5b828203905092915050565b6000610195826100c4565b91507fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff82036101c7576101c6610127565b5b600182019050919050565b600082825260208201905092915050565b7f626967206661696c210000000000000000000000000000000000000000000000600082015250565b60006102196009836101d2565b9150610224826101e3565b602082019050919050565b600060208201905081810360008301526102488161020c565b905091905056fea26469706673582212206c9066085641202e49d5d997c43f820e5bc6b3985fe0eafd9d8362a2e9522b2664736f6c634300080d0033",
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

// Burn is a paid mutator transaction binding the contract method 0x42966c68.
//
// Solidity: function burn(uint256 _amount) payable returns()
func (_Failure *FailureTransactor) Burn(opts *bind.TransactOpts, _amount *big.Int) (*types.Transaction, error) {
	return _Failure.contract.Transact(opts, "burn", _amount)
}

// Burn is a paid mutator transaction binding the contract method 0x42966c68.
//
// Solidity: function burn(uint256 _amount) payable returns()
func (_Failure *FailureSession) Burn(_amount *big.Int) (*types.Transaction, error) {
	return _Failure.Contract.Burn(&_Failure.TransactOpts, _amount)
}

// Burn is a paid mutator transaction binding the contract method 0x42966c68.
//
// Solidity: function burn(uint256 _amount) payable returns()
func (_Failure *FailureTransactorSession) Burn(_amount *big.Int) (*types.Transaction, error) {
	return _Failure.Contract.Burn(&_Failure.TransactOpts, _amount)
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

// FailureYepIterator is returned from FilterYep and is used to iterate over the raw logs and unpacked data for Yep events raised by the Failure contract.
type FailureYepIterator struct {
	Event *FailureYep // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *FailureYepIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(FailureYep)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(FailureYep)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *FailureYepIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *FailureYepIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// FailureYep represents a Yep event raised by the Failure contract.
type FailureYep struct {
	Ok  *big.Int
	Raw types.Log // Blockchain specific contextual infos
}

// FilterYep is a free log retrieval operation binding the contract event 0x14806dfe21dd426fa23d551967947bd5266cc1e2eb26850ed515c8df9c841280.
//
// Solidity: event Yep(uint256 ok)
func (_Failure *FailureFilterer) FilterYep(opts *bind.FilterOpts) (*FailureYepIterator, error) {

	logs, sub, err := _Failure.contract.FilterLogs(opts, "Yep")
	if err != nil {
		return nil, err
	}
	return &FailureYepIterator{contract: _Failure.contract, event: "Yep", logs: logs, sub: sub}, nil
}

// WatchYep is a free log subscription operation binding the contract event 0x14806dfe21dd426fa23d551967947bd5266cc1e2eb26850ed515c8df9c841280.
//
// Solidity: event Yep(uint256 ok)
func (_Failure *FailureFilterer) WatchYep(opts *bind.WatchOpts, sink chan<- *FailureYep) (event.Subscription, error) {

	logs, sub, err := _Failure.contract.WatchLogs(opts, "Yep")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(FailureYep)
				if err := _Failure.contract.UnpackLog(event, "Yep", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseYep is a log parse operation binding the contract event 0x14806dfe21dd426fa23d551967947bd5266cc1e2eb26850ed515c8df9c841280.
//
// Solidity: event Yep(uint256 ok)
func (_Failure *FailureFilterer) ParseYep(log types.Log) (*FailureYep, error) {
	event := new(FailureYep)
	if err := _Failure.contract.UnpackLog(event, "Yep", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}
