// Code generated - DO NOT EDIT.
// This file is a generated binding and any manual changes will be lost.

package taiko

import (
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
	_ = big.NewInt
	_ = strings.NewReader
	_ = ethereum.NotFound
	_ = bind.Bind
	_ = common.Big1
	_ = types.BloomLookup
	_ = event.NewSubscription
)

// ContractABI is the input ABI used to generate the binding from.
const ContractABI = "[{\"constant\":true,\"inputs\":[],\"name\":\"ui\",\"outputs\":[{\"name\":\"\",\"type\":\"uint256\"}],\"payable\":false,\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"name\":\"addr\",\"type\":\"address\"}],\"name\":\"getFromMap\",\"outputs\":[{\"name\":\"\",\"type\":\"uint256\"}],\"payable\":false,\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"addr\",\"type\":\"address\"},{\"name\":\"value\",\"type\":\"uint256\"}],\"name\":\"addToMap\",\"outputs\":[],\"payable\":false,\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"ui_\",\"type\":\"uint256\"},{\"name\":\"addr_\",\"type\":\"address\"}],\"name\":\"events\",\"outputs\":[],\"payable\":false,\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"name\":\"a\",\"type\":\"uint256\"},{\"name\":\"b\",\"type\":\"uint256\"},{\"name\":\"c\",\"type\":\"uint256\"}],\"name\":\"constFunc\",\"outputs\":[{\"name\":\"\",\"type\":\"uint256\"},{\"name\":\"\",\"type\":\"uint256\"},{\"name\":\"\",\"type\":\"uint256\"}],\"payable\":false,\"type\":\"function\"},{\"inputs\":[{\"name\":\"ui_\",\"type\":\"uint256\"}],\"payable\":false,\"type\":\"constructor\"},{\"anonymous\":false,\"inputs\":[],\"name\":\"E0\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"name\":\"\",\"type\":\"uint256\"}],\"name\":\"E1\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"name\":\"\",\"type\":\"uint256\"}],\"name\":\"E2\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"name\":\"\",\"type\":\"address\"}],\"name\":\"E3\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"name\":\"\",\"type\":\"address\"}],\"name\":\"E4\",\"type\":\"event\"},{\"anonymous\":true,\"inputs\":[{\"indexed\":false,\"name\":\"\",\"type\":\"uint256\"},{\"indexed\":false,\"name\":\"\",\"type\":\"address\"}],\"name\":\"E5\",\"type\":\"event\"}]"

// Contract is an auto generated Go binding around an Ethereum contract.
type Contract struct {
	ContractCaller     // Read-only binding to the contract
	ContractTransactor // Write-only binding to the contract
	ContractFilterer   // Log filterer for contract events
}

// ContractCaller is an auto generated read-only Go binding around an Ethereum contract.
type ContractCaller struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// ContractTransactor is an auto generated write-only Go binding around an Ethereum contract.
type ContractTransactor struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// ContractFilterer is an auto generated log filtering Go binding around an Ethereum contract events.
type ContractFilterer struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// ContractSession is an auto generated Go binding around an Ethereum contract,
// with pre-set call and transact options.
type ContractSession struct {
	Contract     *Contract         // Generic contract binding to set the session for
	CallOpts     bind.CallOpts     // Call options to use throughout this session
	TransactOpts bind.TransactOpts // Transaction auth options to use throughout this session
}

// ContractCallerSession is an auto generated read-only Go binding around an Ethereum contract,
// with pre-set call options.
type ContractCallerSession struct {
	Contract *ContractCaller // Generic contract caller binding to set the session for
	CallOpts bind.CallOpts   // Call options to use throughout this session
}

// ContractTransactorSession is an auto generated write-only Go binding around an Ethereum contract,
// with pre-set transact options.
type ContractTransactorSession struct {
	Contract     *ContractTransactor // Generic contract transactor binding to set the session for
	TransactOpts bind.TransactOpts   // Transaction auth options to use throughout this session
}

// ContractRaw is an auto generated low-level Go binding around an Ethereum contract.
type ContractRaw struct {
	Contract *Contract // Generic contract binding to access the raw methods on
}

// ContractCallerRaw is an auto generated low-level read-only Go binding around an Ethereum contract.
type ContractCallerRaw struct {
	Contract *ContractCaller // Generic read-only contract binding to access the raw methods on
}

// ContractTransactorRaw is an auto generated low-level write-only Go binding around an Ethereum contract.
type ContractTransactorRaw struct {
	Contract *ContractTransactor // Generic write-only contract binding to access the raw methods on
}

// NewContract creates a new instance of Contract, bound to a specific deployed contract.
func NewContract(address common.Address, backend bind.ContractBackend) (*Contract, error) {
	contract, err := bindContract(address, backend, backend, backend)
	if err != nil {
		return nil, err
	}
	return &Contract{ContractCaller: ContractCaller{contract: contract}, ContractTransactor: ContractTransactor{contract: contract}, ContractFilterer: ContractFilterer{contract: contract}}, nil
}

// NewContractCaller creates a new read-only instance of Contract, bound to a specific deployed contract.
func NewContractCaller(address common.Address, caller bind.ContractCaller) (*ContractCaller, error) {
	contract, err := bindContract(address, caller, nil, nil)
	if err != nil {
		return nil, err
	}
	return &ContractCaller{contract: contract}, nil
}

// NewContractTransactor creates a new write-only instance of Contract, bound to a specific deployed contract.
func NewContractTransactor(address common.Address, transactor bind.ContractTransactor) (*ContractTransactor, error) {
	contract, err := bindContract(address, nil, transactor, nil)
	if err != nil {
		return nil, err
	}
	return &ContractTransactor{contract: contract}, nil
}

// NewContractFilterer creates a new log filterer instance of Contract, bound to a specific deployed contract.
func NewContractFilterer(address common.Address, filterer bind.ContractFilterer) (*ContractFilterer, error) {
	contract, err := bindContract(address, nil, nil, filterer)
	if err != nil {
		return nil, err
	}
	return &ContractFilterer{contract: contract}, nil
}

// bindContract binds a generic wrapper to an already deployed contract.
func bindContract(address common.Address, caller bind.ContractCaller, transactor bind.ContractTransactor, filterer bind.ContractFilterer) (*bind.BoundContract, error) {
	parsed, err := abi.JSON(strings.NewReader(ContractABI))
	if err != nil {
		return nil, err
	}
	return bind.NewBoundContract(address, parsed, caller, transactor, filterer), nil
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_Contract *ContractRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _Contract.Contract.ContractCaller.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_Contract *ContractRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _Contract.Contract.ContractTransactor.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_Contract *ContractRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _Contract.Contract.ContractTransactor.contract.Transact(opts, method, params...)
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_Contract *ContractCallerRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _Contract.Contract.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_Contract *ContractTransactorRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _Contract.Contract.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_Contract *ContractTransactorRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _Contract.Contract.contract.Transact(opts, method, params...)
}

// ConstFunc is a free data retrieval call binding the contract method 0xe6768b45.
//
// Solidity: function constFunc(uint256 a, uint256 b, uint256 c) returns(uint256, uint256, uint256)
func (_Contract *ContractCaller) ConstFunc(opts *bind.CallOpts, a *big.Int, b *big.Int, c *big.Int) (*big.Int, *big.Int, *big.Int, error) {
	var out []interface{}
	err := _Contract.contract.Call(opts, &out, "constFunc", a, b, c)

	if err != nil {
		return *new(*big.Int), *new(*big.Int), *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)
	out1 := *abi.ConvertType(out[1], new(*big.Int)).(**big.Int)
	out2 := *abi.ConvertType(out[2], new(*big.Int)).(**big.Int)

	return out0, out1, out2, err

}

// ConstFunc is a free data retrieval call binding the contract method 0xe6768b45.
//
// Solidity: function constFunc(uint256 a, uint256 b, uint256 c) returns(uint256, uint256, uint256)
func (_Contract *ContractSession) ConstFunc(a *big.Int, b *big.Int, c *big.Int) (*big.Int, *big.Int, *big.Int, error) {
	return _Contract.Contract.ConstFunc(&_Contract.CallOpts, a, b, c)
}

// ConstFunc is a free data retrieval call binding the contract method 0xe6768b45.
//
// Solidity: function constFunc(uint256 a, uint256 b, uint256 c) returns(uint256, uint256, uint256)
func (_Contract *ContractCallerSession) ConstFunc(a *big.Int, b *big.Int, c *big.Int) (*big.Int, *big.Int, *big.Int, error) {
	return _Contract.Contract.ConstFunc(&_Contract.CallOpts, a, b, c)
}

// GetFromMap is a free data retrieval call binding the contract method 0xabd1a0cf.
//
// Solidity: function getFromMap(address addr) returns(uint256)
func (_Contract *ContractCaller) GetFromMap(opts *bind.CallOpts, addr common.Address) (*big.Int, error) {
	var out []interface{}
	err := _Contract.contract.Call(opts, &out, "getFromMap", addr)

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// GetFromMap is a free data retrieval call binding the contract method 0xabd1a0cf.
//
// Solidity: function getFromMap(address addr) returns(uint256)
func (_Contract *ContractSession) GetFromMap(addr common.Address) (*big.Int, error) {
	return _Contract.Contract.GetFromMap(&_Contract.CallOpts, addr)
}

// GetFromMap is a free data retrieval call binding the contract method 0xabd1a0cf.
//
// Solidity: function getFromMap(address addr) returns(uint256)
func (_Contract *ContractCallerSession) GetFromMap(addr common.Address) (*big.Int, error) {
	return _Contract.Contract.GetFromMap(&_Contract.CallOpts, addr)
}

// Ui is a free data retrieval call binding the contract method 0xa223e05d.
//
// Solidity: function ui() returns(uint256)
func (_Contract *ContractCaller) Ui(opts *bind.CallOpts) (*big.Int, error) {
	var out []interface{}
	err := _Contract.contract.Call(opts, &out, "ui")

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// Ui is a free data retrieval call binding the contract method 0xa223e05d.
//
// Solidity: function ui() returns(uint256)
func (_Contract *ContractSession) Ui() (*big.Int, error) {
	return _Contract.Contract.Ui(&_Contract.CallOpts)
}

// Ui is a free data retrieval call binding the contract method 0xa223e05d.
//
// Solidity: function ui() returns(uint256)
func (_Contract *ContractCallerSession) Ui() (*big.Int, error) {
	return _Contract.Contract.Ui(&_Contract.CallOpts)
}

// AddToMap is a paid mutator transaction binding the contract method 0xabfced1d.
//
// Solidity: function addToMap(address addr, uint256 value) returns()
func (_Contract *ContractTransactor) AddToMap(opts *bind.TransactOpts, addr common.Address, value *big.Int) (*types.Transaction, error) {
	return _Contract.contract.Transact(opts, "addToMap", addr, value)
}

// AddToMap is a paid mutator transaction binding the contract method 0xabfced1d.
//
// Solidity: function addToMap(address addr, uint256 value) returns()
func (_Contract *ContractSession) AddToMap(addr common.Address, value *big.Int) (*types.Transaction, error) {
	return _Contract.Contract.AddToMap(&_Contract.TransactOpts, addr, value)
}

// AddToMap is a paid mutator transaction binding the contract method 0xabfced1d.
//
// Solidity: function addToMap(address addr, uint256 value) returns()
func (_Contract *ContractTransactorSession) AddToMap(addr common.Address, value *big.Int) (*types.Transaction, error) {
	return _Contract.Contract.AddToMap(&_Contract.TransactOpts, addr, value)
}

// Events is a paid mutator transaction binding the contract method 0xe05c914a.
//
// Solidity: function events(uint256 ui_, address addr_) returns()
func (_Contract *ContractTransactor) Events(opts *bind.TransactOpts, ui_ *big.Int, addr_ common.Address) (*types.Transaction, error) {
	return _Contract.contract.Transact(opts, "events", ui_, addr_)
}

// Events is a paid mutator transaction binding the contract method 0xe05c914a.
//
// Solidity: function events(uint256 ui_, address addr_) returns()
func (_Contract *ContractSession) Events(ui_ *big.Int, addr_ common.Address) (*types.Transaction, error) {
	return _Contract.Contract.Events(&_Contract.TransactOpts, ui_, addr_)
}

// Events is a paid mutator transaction binding the contract method 0xe05c914a.
//
// Solidity: function events(uint256 ui_, address addr_) returns()
func (_Contract *ContractTransactorSession) Events(ui_ *big.Int, addr_ common.Address) (*types.Transaction, error) {
	return _Contract.Contract.Events(&_Contract.TransactOpts, ui_, addr_)
}

// ContractE0Iterator is returned from FilterE0 and is used to iterate over the raw logs and unpacked data for E0 events raised by the Contract contract.
type ContractE0Iterator struct {
	Event *ContractE0 // Event containing the contract specifics and raw log

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
func (it *ContractE0Iterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(ContractE0)
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
		it.Event = new(ContractE0)
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
func (it *ContractE0Iterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *ContractE0Iterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// ContractE0 represents a E0 event raised by the Contract contract.
type ContractE0 struct {
	Raw types.Log // Blockchain specific contextual infos
}

// FilterE0 is a free log retrieval operation binding the contract event 0x6031a8d62d7c95988fa262657cd92107d90ed96e08d8f867d32f26edfe855022.
//
// Solidity: event E0()
func (_Contract *ContractFilterer) FilterE0(opts *bind.FilterOpts) (*ContractE0Iterator, error) {

	logs, sub, err := _Contract.contract.FilterLogs(opts, "E0")
	if err != nil {
		return nil, err
	}
	return &ContractE0Iterator{contract: _Contract.contract, event: "E0", logs: logs, sub: sub}, nil
}

// WatchE0 is a free log subscription operation binding the contract event 0x6031a8d62d7c95988fa262657cd92107d90ed96e08d8f867d32f26edfe855022.
//
// Solidity: event E0()
func (_Contract *ContractFilterer) WatchE0(opts *bind.WatchOpts, sink chan<- *ContractE0) (event.Subscription, error) {

	logs, sub, err := _Contract.contract.WatchLogs(opts, "E0")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(ContractE0)
				if err := _Contract.contract.UnpackLog(event, "E0", log); err != nil {
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

// ParseE0 is a log parse operation binding the contract event 0x6031a8d62d7c95988fa262657cd92107d90ed96e08d8f867d32f26edfe855022.
//
// Solidity: event E0()
func (_Contract *ContractFilterer) ParseE0(log types.Log) (*ContractE0, error) {
	event := new(ContractE0)
	if err := _Contract.contract.UnpackLog(event, "E0", log); err != nil {
		return nil, err
	}
	return event, nil
}

// ContractE1Iterator is returned from FilterE1 and is used to iterate over the raw logs and unpacked data for E1 events raised by the Contract contract.
type ContractE1Iterator struct {
	Event *ContractE1 // Event containing the contract specifics and raw log

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
func (it *ContractE1Iterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(ContractE1)
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
		it.Event = new(ContractE1)
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
func (it *ContractE1Iterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *ContractE1Iterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// ContractE1 represents a E1 event raised by the Contract contract.
type ContractE1 struct {
	Arg0 *big.Int
	Raw  types.Log // Blockchain specific contextual infos
}

// FilterE1 is a free log retrieval operation binding the contract event 0x47e2689743f14e97f7dcfa5eec10ba1dff02f83b3d1d4b9c07b206cbbda66450.
//
// Solidity: event E1(uint256 arg0)
func (_Contract *ContractFilterer) FilterE1(opts *bind.FilterOpts) (*ContractE1Iterator, error) {

	logs, sub, err := _Contract.contract.FilterLogs(opts, "E1")
	if err != nil {
		return nil, err
	}
	return &ContractE1Iterator{contract: _Contract.contract, event: "E1", logs: logs, sub: sub}, nil
}

// WatchE1 is a free log subscription operation binding the contract event 0x47e2689743f14e97f7dcfa5eec10ba1dff02f83b3d1d4b9c07b206cbbda66450.
//
// Solidity: event E1(uint256 arg0)
func (_Contract *ContractFilterer) WatchE1(opts *bind.WatchOpts, sink chan<- *ContractE1) (event.Subscription, error) {

	logs, sub, err := _Contract.contract.WatchLogs(opts, "E1")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(ContractE1)
				if err := _Contract.contract.UnpackLog(event, "E1", log); err != nil {
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

// ParseE1 is a log parse operation binding the contract event 0x47e2689743f14e97f7dcfa5eec10ba1dff02f83b3d1d4b9c07b206cbbda66450.
//
// Solidity: event E1(uint256 arg0)
func (_Contract *ContractFilterer) ParseE1(log types.Log) (*ContractE1, error) {
	event := new(ContractE1)
	if err := _Contract.contract.UnpackLog(event, "E1", log); err != nil {
		return nil, err
	}
	return event, nil
}

// ContractE2Iterator is returned from FilterE2 and is used to iterate over the raw logs and unpacked data for E2 events raised by the Contract contract.
type ContractE2Iterator struct {
	Event *ContractE2 // Event containing the contract specifics and raw log

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
func (it *ContractE2Iterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(ContractE2)
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
		it.Event = new(ContractE2)
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
func (it *ContractE2Iterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *ContractE2Iterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// ContractE2 represents a E2 event raised by the Contract contract.
type ContractE2 struct {
	Arg0 *big.Int
	Raw  types.Log // Blockchain specific contextual infos
}

// FilterE2 is a free log retrieval operation binding the contract event 0xa48a6b249a5084126c3da369fbc9b16827ead8cb5cdc094b717d3f1dcd995e29.
//
// Solidity: event E2(uint256 indexed arg0)
func (_Contract *ContractFilterer) FilterE2(opts *bind.FilterOpts, arg0 []*big.Int) (*ContractE2Iterator, error) {

	var arg0Rule []interface{}
	for _, arg0Item := range arg0 {
		arg0Rule = append(arg0Rule, arg0Item)
	}

	logs, sub, err := _Contract.contract.FilterLogs(opts, "E2", arg0Rule)
	if err != nil {
		return nil, err
	}
	return &ContractE2Iterator{contract: _Contract.contract, event: "E2", logs: logs, sub: sub}, nil
}

// WatchE2 is a free log subscription operation binding the contract event 0xa48a6b249a5084126c3da369fbc9b16827ead8cb5cdc094b717d3f1dcd995e29.
//
// Solidity: event E2(uint256 indexed arg0)
func (_Contract *ContractFilterer) WatchE2(opts *bind.WatchOpts, sink chan<- *ContractE2, arg0 []*big.Int) (event.Subscription, error) {

	var arg0Rule []interface{}
	for _, arg0Item := range arg0 {
		arg0Rule = append(arg0Rule, arg0Item)
	}

	logs, sub, err := _Contract.contract.WatchLogs(opts, "E2", arg0Rule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(ContractE2)
				if err := _Contract.contract.UnpackLog(event, "E2", log); err != nil {
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

// ParseE2 is a log parse operation binding the contract event 0xa48a6b249a5084126c3da369fbc9b16827ead8cb5cdc094b717d3f1dcd995e29.
//
// Solidity: event E2(uint256 indexed arg0)
func (_Contract *ContractFilterer) ParseE2(log types.Log) (*ContractE2, error) {
	event := new(ContractE2)
	if err := _Contract.contract.UnpackLog(event, "E2", log); err != nil {
		return nil, err
	}
	return event, nil
}

// ContractE3Iterator is returned from FilterE3 and is used to iterate over the raw logs and unpacked data for E3 events raised by the Contract contract.
type ContractE3Iterator struct {
	Event *ContractE3 // Event containing the contract specifics and raw log

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
func (it *ContractE3Iterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(ContractE3)
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
		it.Event = new(ContractE3)
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
func (it *ContractE3Iterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *ContractE3Iterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// ContractE3 represents a E3 event raised by the Contract contract.
type ContractE3 struct {
	Arg0 common.Address
	Raw  types.Log // Blockchain specific contextual infos
}

// FilterE3 is a free log retrieval operation binding the contract event 0x7890603b316f3509577afd111710f9ebeefa15e12f72347d9dffd0d65ae3bade.
//
// Solidity: event E3(address arg0)
func (_Contract *ContractFilterer) FilterE3(opts *bind.FilterOpts) (*ContractE3Iterator, error) {

	logs, sub, err := _Contract.contract.FilterLogs(opts, "E3")
	if err != nil {
		return nil, err
	}
	return &ContractE3Iterator{contract: _Contract.contract, event: "E3", logs: logs, sub: sub}, nil
}

// WatchE3 is a free log subscription operation binding the contract event 0x7890603b316f3509577afd111710f9ebeefa15e12f72347d9dffd0d65ae3bade.
//
// Solidity: event E3(address arg0)
func (_Contract *ContractFilterer) WatchE3(opts *bind.WatchOpts, sink chan<- *ContractE3) (event.Subscription, error) {

	logs, sub, err := _Contract.contract.WatchLogs(opts, "E3")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(ContractE3)
				if err := _Contract.contract.UnpackLog(event, "E3", log); err != nil {
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

// ParseE3 is a log parse operation binding the contract event 0x7890603b316f3509577afd111710f9ebeefa15e12f72347d9dffd0d65ae3bade.
//
// Solidity: event E3(address arg0)
func (_Contract *ContractFilterer) ParseE3(log types.Log) (*ContractE3, error) {
	event := new(ContractE3)
	if err := _Contract.contract.UnpackLog(event, "E3", log); err != nil {
		return nil, err
	}
	return event, nil
}

// ContractE4Iterator is returned from FilterE4 and is used to iterate over the raw logs and unpacked data for E4 events raised by the Contract contract.
type ContractE4Iterator struct {
	Event *ContractE4 // Event containing the contract specifics and raw log

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
func (it *ContractE4Iterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(ContractE4)
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
		it.Event = new(ContractE4)
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
func (it *ContractE4Iterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *ContractE4Iterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// ContractE4 represents a E4 event raised by the Contract contract.
type ContractE4 struct {
	Arg0 common.Address
	Raw  types.Log // Blockchain specific contextual infos
}

// FilterE4 is a free log retrieval operation binding the contract event 0x7efef9ea3f60ddc038e50cccec621f86a0195894dc0520482abf8b5c6b659e41.
//
// Solidity: event E4(address indexed arg0)
func (_Contract *ContractFilterer) FilterE4(opts *bind.FilterOpts, arg0 []common.Address) (*ContractE4Iterator, error) {

	var arg0Rule []interface{}
	for _, arg0Item := range arg0 {
		arg0Rule = append(arg0Rule, arg0Item)
	}

	logs, sub, err := _Contract.contract.FilterLogs(opts, "E4", arg0Rule)
	if err != nil {
		return nil, err
	}
	return &ContractE4Iterator{contract: _Contract.contract, event: "E4", logs: logs, sub: sub}, nil
}

// WatchE4 is a free log subscription operation binding the contract event 0x7efef9ea3f60ddc038e50cccec621f86a0195894dc0520482abf8b5c6b659e41.
//
// Solidity: event E4(address indexed arg0)
func (_Contract *ContractFilterer) WatchE4(opts *bind.WatchOpts, sink chan<- *ContractE4, arg0 []common.Address) (event.Subscription, error) {

	var arg0Rule []interface{}
	for _, arg0Item := range arg0 {
		arg0Rule = append(arg0Rule, arg0Item)
	}

	logs, sub, err := _Contract.contract.WatchLogs(opts, "E4", arg0Rule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(ContractE4)
				if err := _Contract.contract.UnpackLog(event, "E4", log); err != nil {
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

// ParseE4 is a log parse operation binding the contract event 0x7efef9ea3f60ddc038e50cccec621f86a0195894dc0520482abf8b5c6b659e41.
//
// Solidity: event E4(address indexed arg0)
func (_Contract *ContractFilterer) ParseE4(log types.Log) (*ContractE4, error) {
	event := new(ContractE4)
	if err := _Contract.contract.UnpackLog(event, "E4", log); err != nil {
		return nil, err
	}
	return event, nil
}
