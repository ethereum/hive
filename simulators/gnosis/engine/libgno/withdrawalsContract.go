// Code generated - DO NOT EDIT.
// This file is a generated binding and any manual changes will be lost.

package libgno

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

// LibgnoMetaData contains all meta data concerning the Libgno contract.
var LibgnoMetaData = &bind.MetaData{
	ABI: "[{\"inputs\":[{\"internalType\":\"address\",\"name\":\"_token\",\"type\":\"address\"}],\"stateMutability\":\"nonpayable\",\"type\":\"constructor\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"bytes\",\"name\":\"pubkey\",\"type\":\"bytes\"},{\"indexed\":false,\"internalType\":\"bytes\",\"name\":\"withdrawal_credentials\",\"type\":\"bytes\"},{\"indexed\":false,\"internalType\":\"bytes\",\"name\":\"amount\",\"type\":\"bytes\"},{\"indexed\":false,\"internalType\":\"bytes\",\"name\":\"signature\",\"type\":\"bytes\"},{\"indexed\":false,\"internalType\":\"bytes\",\"name\":\"index\",\"type\":\"bytes\"}],\"name\":\"DepositEvent\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"address\",\"name\":\"account\",\"type\":\"address\"}],\"name\":\"Paused\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"address\",\"name\":\"account\",\"type\":\"address\"}],\"name\":\"Unpaused\",\"type\":\"event\"},{\"inputs\":[],\"name\":\"pause\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"paused\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"stake_token\",\"outputs\":[{\"internalType\":\"contractIERC20\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"unpause\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"bytes\",\"name\":\"\",\"type\":\"bytes\"}],\"name\":\"validator_withdrawal_credentials\",\"outputs\":[{\"internalType\":\"bytes32\",\"name\":\"\",\"type\":\"bytes32\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"name\":\"withdrawableAmount\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"get_deposit_root\",\"outputs\":[{\"internalType\":\"bytes32\",\"name\":\"\",\"type\":\"bytes32\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"get_deposit_count\",\"outputs\":[{\"internalType\":\"bytes\",\"name\":\"\",\"type\":\"bytes\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"bytes\",\"name\":\"pubkey\",\"type\":\"bytes\"},{\"internalType\":\"bytes\",\"name\":\"withdrawal_credentials\",\"type\":\"bytes\"},{\"internalType\":\"bytes\",\"name\":\"signature\",\"type\":\"bytes\"},{\"internalType\":\"bytes32\",\"name\":\"deposit_data_root\",\"type\":\"bytes32\"},{\"internalType\":\"uint256\",\"name\":\"stake_amount\",\"type\":\"uint256\"}],\"name\":\"deposit\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"bytes\",\"name\":\"pubkeys\",\"type\":\"bytes\"},{\"internalType\":\"bytes\",\"name\":\"withdrawal_credentials\",\"type\":\"bytes\"},{\"internalType\":\"bytes\",\"name\":\"signatures\",\"type\":\"bytes\"},{\"internalType\":\"bytes32[]\",\"name\":\"deposit_data_roots\",\"type\":\"bytes32[]\"}],\"name\":\"batchDeposit\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"},{\"internalType\":\"uint256\",\"name\":\"stake_amount\",\"type\":\"uint256\"},{\"internalType\":\"bytes\",\"name\":\"data\",\"type\":\"bytes\"}],\"name\":\"onTokenTransfer\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"bytes4\",\"name\":\"interfaceId\",\"type\":\"bytes4\"}],\"name\":\"supportsInterface\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"pure\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"_token\",\"type\":\"address\"},{\"internalType\":\"address\",\"name\":\"_to\",\"type\":\"address\"}],\"name\":\"claimTokens\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"_address\",\"type\":\"address\"}],\"name\":\"claimWithdrawal\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address[]\",\"name\":\"_addresses\",\"type\":\"address[]\"}],\"name\":\"claimWithdrawals\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"_maxNumberOfFailedWithdrawalsToProcess\",\"type\":\"uint256\"},{\"internalType\":\"uint64[]\",\"name\":\"_amounts\",\"type\":\"uint64[]\"},{\"internalType\":\"address[]\",\"name\":\"_addresses\",\"type\":\"address[]\"}],\"name\":\"executeSystemWithdrawals\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"contractIUnwrapper\",\"name\":\"_unwrapper\",\"type\":\"address\"},{\"internalType\":\"contractIERC20\",\"name\":\"_token\",\"type\":\"address\"}],\"name\":\"unwrapTokens\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"addr\",\"type\":\"address\"}],\"name\":\"flushTokensTo\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"}]",
}

// LibgnoABI is the input ABI used to generate the binding from.
// Deprecated: Use LibgnoMetaData.ABI instead.
var LibgnoABI = LibgnoMetaData.ABI

// Libgno is an auto generated Go binding around an Ethereum contract.
type Libgno struct {
	LibgnoCaller     // Read-only binding to the contract
	LibgnoTransactor // Write-only binding to the contract
	LibgnoFilterer   // Log filterer for contract events
}

// LibgnoCaller is an auto generated read-only Go binding around an Ethereum contract.
type LibgnoCaller struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// LibgnoTransactor is an auto generated write-only Go binding around an Ethereum contract.
type LibgnoTransactor struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// LibgnoFilterer is an auto generated log filtering Go binding around an Ethereum contract events.
type LibgnoFilterer struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// LibgnoSession is an auto generated Go binding around an Ethereum contract,
// with pre-set call and transact options.
type LibgnoSession struct {
	Contract     *Libgno           // Generic contract binding to set the session for
	CallOpts     bind.CallOpts     // Call options to use throughout this session
	TransactOpts bind.TransactOpts // Transaction auth options to use throughout this session
}

// LibgnoCallerSession is an auto generated read-only Go binding around an Ethereum contract,
// with pre-set call options.
type LibgnoCallerSession struct {
	Contract *LibgnoCaller // Generic contract caller binding to set the session for
	CallOpts bind.CallOpts // Call options to use throughout this session
}

// LibgnoTransactorSession is an auto generated write-only Go binding around an Ethereum contract,
// with pre-set transact options.
type LibgnoTransactorSession struct {
	Contract     *LibgnoTransactor // Generic contract transactor binding to set the session for
	TransactOpts bind.TransactOpts // Transaction auth options to use throughout this session
}

// LibgnoRaw is an auto generated low-level Go binding around an Ethereum contract.
type LibgnoRaw struct {
	Contract *Libgno // Generic contract binding to access the raw methods on
}

// LibgnoCallerRaw is an auto generated low-level read-only Go binding around an Ethereum contract.
type LibgnoCallerRaw struct {
	Contract *LibgnoCaller // Generic read-only contract binding to access the raw methods on
}

// LibgnoTransactorRaw is an auto generated low-level write-only Go binding around an Ethereum contract.
type LibgnoTransactorRaw struct {
	Contract *LibgnoTransactor // Generic write-only contract binding to access the raw methods on
}

// NewLibgno creates a new instance of Libgno, bound to a specific deployed contract.
func NewLibgno(address common.Address, backend bind.ContractBackend) (*Libgno, error) {
	contract, err := bindLibgno(address, backend, backend, backend)
	if err != nil {
		return nil, err
	}
	return &Libgno{LibgnoCaller: LibgnoCaller{contract: contract}, LibgnoTransactor: LibgnoTransactor{contract: contract}, LibgnoFilterer: LibgnoFilterer{contract: contract}}, nil
}

// NewLibgnoCaller creates a new read-only instance of Libgno, bound to a specific deployed contract.
func NewLibgnoCaller(address common.Address, caller bind.ContractCaller) (*LibgnoCaller, error) {
	contract, err := bindLibgno(address, caller, nil, nil)
	if err != nil {
		return nil, err
	}
	return &LibgnoCaller{contract: contract}, nil
}

// NewLibgnoTransactor creates a new write-only instance of Libgno, bound to a specific deployed contract.
func NewLibgnoTransactor(address common.Address, transactor bind.ContractTransactor) (*LibgnoTransactor, error) {
	contract, err := bindLibgno(address, nil, transactor, nil)
	if err != nil {
		return nil, err
	}
	return &LibgnoTransactor{contract: contract}, nil
}

// NewLibgnoFilterer creates a new log filterer instance of Libgno, bound to a specific deployed contract.
func NewLibgnoFilterer(address common.Address, filterer bind.ContractFilterer) (*LibgnoFilterer, error) {
	contract, err := bindLibgno(address, nil, nil, filterer)
	if err != nil {
		return nil, err
	}
	return &LibgnoFilterer{contract: contract}, nil
}

// bindLibgno binds a generic wrapper to an already deployed contract.
func bindLibgno(address common.Address, caller bind.ContractCaller, transactor bind.ContractTransactor, filterer bind.ContractFilterer) (*bind.BoundContract, error) {
	parsed, err := abi.JSON(strings.NewReader(LibgnoABI))
	if err != nil {
		return nil, err
	}
	return bind.NewBoundContract(address, parsed, caller, transactor, filterer), nil
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_Libgno *LibgnoRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _Libgno.Contract.LibgnoCaller.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_Libgno *LibgnoRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _Libgno.Contract.LibgnoTransactor.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_Libgno *LibgnoRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _Libgno.Contract.LibgnoTransactor.contract.Transact(opts, method, params...)
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_Libgno *LibgnoCallerRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _Libgno.Contract.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_Libgno *LibgnoTransactorRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _Libgno.Contract.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_Libgno *LibgnoTransactorRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _Libgno.Contract.contract.Transact(opts, method, params...)
}

// GetDepositCount is a free data retrieval call binding the contract method 0x621fd130.
//
// Solidity: function get_deposit_count() view returns(bytes)
func (_Libgno *LibgnoCaller) GetDepositCount(opts *bind.CallOpts) ([]byte, error) {
	var out []interface{}
	err := _Libgno.contract.Call(opts, &out, "get_deposit_count")

	if err != nil {
		return *new([]byte), err
	}

	out0 := *abi.ConvertType(out[0], new([]byte)).(*[]byte)

	return out0, err

}

// GetDepositCount is a free data retrieval call binding the contract method 0x621fd130.
//
// Solidity: function get_deposit_count() view returns(bytes)
func (_Libgno *LibgnoSession) GetDepositCount() ([]byte, error) {
	return _Libgno.Contract.GetDepositCount(&_Libgno.CallOpts)
}

// GetDepositCount is a free data retrieval call binding the contract method 0x621fd130.
//
// Solidity: function get_deposit_count() view returns(bytes)
func (_Libgno *LibgnoCallerSession) GetDepositCount() ([]byte, error) {
	return _Libgno.Contract.GetDepositCount(&_Libgno.CallOpts)
}

// GetDepositRoot is a free data retrieval call binding the contract method 0xc5f2892f.
//
// Solidity: function get_deposit_root() view returns(bytes32)
func (_Libgno *LibgnoCaller) GetDepositRoot(opts *bind.CallOpts) ([32]byte, error) {
	var out []interface{}
	err := _Libgno.contract.Call(opts, &out, "get_deposit_root")

	if err != nil {
		return *new([32]byte), err
	}

	out0 := *abi.ConvertType(out[0], new([32]byte)).(*[32]byte)

	return out0, err

}

// GetDepositRoot is a free data retrieval call binding the contract method 0xc5f2892f.
//
// Solidity: function get_deposit_root() view returns(bytes32)
func (_Libgno *LibgnoSession) GetDepositRoot() ([32]byte, error) {
	return _Libgno.Contract.GetDepositRoot(&_Libgno.CallOpts)
}

// GetDepositRoot is a free data retrieval call binding the contract method 0xc5f2892f.
//
// Solidity: function get_deposit_root() view returns(bytes32)
func (_Libgno *LibgnoCallerSession) GetDepositRoot() ([32]byte, error) {
	return _Libgno.Contract.GetDepositRoot(&_Libgno.CallOpts)
}

// Paused is a free data retrieval call binding the contract method 0x5c975abb.
//
// Solidity: function paused() view returns(bool)
func (_Libgno *LibgnoCaller) Paused(opts *bind.CallOpts) (bool, error) {
	var out []interface{}
	err := _Libgno.contract.Call(opts, &out, "paused")

	if err != nil {
		return *new(bool), err
	}

	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)

	return out0, err

}

// Paused is a free data retrieval call binding the contract method 0x5c975abb.
//
// Solidity: function paused() view returns(bool)
func (_Libgno *LibgnoSession) Paused() (bool, error) {
	return _Libgno.Contract.Paused(&_Libgno.CallOpts)
}

// Paused is a free data retrieval call binding the contract method 0x5c975abb.
//
// Solidity: function paused() view returns(bool)
func (_Libgno *LibgnoCallerSession) Paused() (bool, error) {
	return _Libgno.Contract.Paused(&_Libgno.CallOpts)
}

// StakeToken is a free data retrieval call binding the contract method 0x640415bf.
//
// Solidity: function stake_token() view returns(address)
func (_Libgno *LibgnoCaller) StakeToken(opts *bind.CallOpts) (common.Address, error) {
	var out []interface{}
	err := _Libgno.contract.Call(opts, &out, "stake_token")

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// StakeToken is a free data retrieval call binding the contract method 0x640415bf.
//
// Solidity: function stake_token() view returns(address)
func (_Libgno *LibgnoSession) StakeToken() (common.Address, error) {
	return _Libgno.Contract.StakeToken(&_Libgno.CallOpts)
}

// StakeToken is a free data retrieval call binding the contract method 0x640415bf.
//
// Solidity: function stake_token() view returns(address)
func (_Libgno *LibgnoCallerSession) StakeToken() (common.Address, error) {
	return _Libgno.Contract.StakeToken(&_Libgno.CallOpts)
}

// SupportsInterface is a free data retrieval call binding the contract method 0x01ffc9a7.
//
// Solidity: function supportsInterface(bytes4 interfaceId) pure returns(bool)
func (_Libgno *LibgnoCaller) SupportsInterface(opts *bind.CallOpts, interfaceId [4]byte) (bool, error) {
	var out []interface{}
	err := _Libgno.contract.Call(opts, &out, "supportsInterface", interfaceId)

	if err != nil {
		return *new(bool), err
	}

	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)

	return out0, err

}

// SupportsInterface is a free data retrieval call binding the contract method 0x01ffc9a7.
//
// Solidity: function supportsInterface(bytes4 interfaceId) pure returns(bool)
func (_Libgno *LibgnoSession) SupportsInterface(interfaceId [4]byte) (bool, error) {
	return _Libgno.Contract.SupportsInterface(&_Libgno.CallOpts, interfaceId)
}

// SupportsInterface is a free data retrieval call binding the contract method 0x01ffc9a7.
//
// Solidity: function supportsInterface(bytes4 interfaceId) pure returns(bool)
func (_Libgno *LibgnoCallerSession) SupportsInterface(interfaceId [4]byte) (bool, error) {
	return _Libgno.Contract.SupportsInterface(&_Libgno.CallOpts, interfaceId)
}

// ValidatorWithdrawalCredentials is a free data retrieval call binding the contract method 0x24db4c46.
//
// Solidity: function validator_withdrawal_credentials(bytes ) view returns(bytes32)
func (_Libgno *LibgnoCaller) ValidatorWithdrawalCredentials(opts *bind.CallOpts, arg0 []byte) ([32]byte, error) {
	var out []interface{}
	err := _Libgno.contract.Call(opts, &out, "validator_withdrawal_credentials", arg0)

	if err != nil {
		return *new([32]byte), err
	}

	out0 := *abi.ConvertType(out[0], new([32]byte)).(*[32]byte)

	return out0, err

}

// ValidatorWithdrawalCredentials is a free data retrieval call binding the contract method 0x24db4c46.
//
// Solidity: function validator_withdrawal_credentials(bytes ) view returns(bytes32)
func (_Libgno *LibgnoSession) ValidatorWithdrawalCredentials(arg0 []byte) ([32]byte, error) {
	return _Libgno.Contract.ValidatorWithdrawalCredentials(&_Libgno.CallOpts, arg0)
}

// ValidatorWithdrawalCredentials is a free data retrieval call binding the contract method 0x24db4c46.
//
// Solidity: function validator_withdrawal_credentials(bytes ) view returns(bytes32)
func (_Libgno *LibgnoCallerSession) ValidatorWithdrawalCredentials(arg0 []byte) ([32]byte, error) {
	return _Libgno.Contract.ValidatorWithdrawalCredentials(&_Libgno.CallOpts, arg0)
}

// WithdrawableAmount is a free data retrieval call binding the contract method 0xbe7ab51b.
//
// Solidity: function withdrawableAmount(address ) view returns(uint256)
func (_Libgno *LibgnoCaller) WithdrawableAmount(opts *bind.CallOpts, arg0 common.Address) (*big.Int, error) {
	var out []interface{}
	err := _Libgno.contract.Call(opts, &out, "withdrawableAmount", arg0)

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// WithdrawableAmount is a free data retrieval call binding the contract method 0xbe7ab51b.
//
// Solidity: function withdrawableAmount(address ) view returns(uint256)
func (_Libgno *LibgnoSession) WithdrawableAmount(arg0 common.Address) (*big.Int, error) {
	return _Libgno.Contract.WithdrawableAmount(&_Libgno.CallOpts, arg0)
}

// WithdrawableAmount is a free data retrieval call binding the contract method 0xbe7ab51b.
//
// Solidity: function withdrawableAmount(address ) view returns(uint256)
func (_Libgno *LibgnoCallerSession) WithdrawableAmount(arg0 common.Address) (*big.Int, error) {
	return _Libgno.Contract.WithdrawableAmount(&_Libgno.CallOpts, arg0)
}

// BatchDeposit is a paid mutator transaction binding the contract method 0xc82655b7.
//
// Solidity: function batchDeposit(bytes pubkeys, bytes withdrawal_credentials, bytes signatures, bytes32[] deposit_data_roots) returns()
func (_Libgno *LibgnoTransactor) BatchDeposit(opts *bind.TransactOpts, pubkeys []byte, withdrawal_credentials []byte, signatures []byte, deposit_data_roots [][32]byte) (*types.Transaction, error) {
	return _Libgno.contract.Transact(opts, "batchDeposit", pubkeys, withdrawal_credentials, signatures, deposit_data_roots)
}

// BatchDeposit is a paid mutator transaction binding the contract method 0xc82655b7.
//
// Solidity: function batchDeposit(bytes pubkeys, bytes withdrawal_credentials, bytes signatures, bytes32[] deposit_data_roots) returns()
func (_Libgno *LibgnoSession) BatchDeposit(pubkeys []byte, withdrawal_credentials []byte, signatures []byte, deposit_data_roots [][32]byte) (*types.Transaction, error) {
	return _Libgno.Contract.BatchDeposit(&_Libgno.TransactOpts, pubkeys, withdrawal_credentials, signatures, deposit_data_roots)
}

// BatchDeposit is a paid mutator transaction binding the contract method 0xc82655b7.
//
// Solidity: function batchDeposit(bytes pubkeys, bytes withdrawal_credentials, bytes signatures, bytes32[] deposit_data_roots) returns()
func (_Libgno *LibgnoTransactorSession) BatchDeposit(pubkeys []byte, withdrawal_credentials []byte, signatures []byte, deposit_data_roots [][32]byte) (*types.Transaction, error) {
	return _Libgno.Contract.BatchDeposit(&_Libgno.TransactOpts, pubkeys, withdrawal_credentials, signatures, deposit_data_roots)
}

// ClaimTokens is a paid mutator transaction binding the contract method 0x69ffa08a.
//
// Solidity: function claimTokens(address _token, address _to) returns()
func (_Libgno *LibgnoTransactor) ClaimTokens(opts *bind.TransactOpts, _token common.Address, _to common.Address) (*types.Transaction, error) {
	return _Libgno.contract.Transact(opts, "claimTokens", _token, _to)
}

// ClaimTokens is a paid mutator transaction binding the contract method 0x69ffa08a.
//
// Solidity: function claimTokens(address _token, address _to) returns()
func (_Libgno *LibgnoSession) ClaimTokens(_token common.Address, _to common.Address) (*types.Transaction, error) {
	return _Libgno.Contract.ClaimTokens(&_Libgno.TransactOpts, _token, _to)
}

// ClaimTokens is a paid mutator transaction binding the contract method 0x69ffa08a.
//
// Solidity: function claimTokens(address _token, address _to) returns()
func (_Libgno *LibgnoTransactorSession) ClaimTokens(_token common.Address, _to common.Address) (*types.Transaction, error) {
	return _Libgno.Contract.ClaimTokens(&_Libgno.TransactOpts, _token, _to)
}

// ClaimWithdrawal is a paid mutator transaction binding the contract method 0xa3066aab.
//
// Solidity: function claimWithdrawal(address _address) returns()
func (_Libgno *LibgnoTransactor) ClaimWithdrawal(opts *bind.TransactOpts, _address common.Address) (*types.Transaction, error) {
	return _Libgno.contract.Transact(opts, "claimWithdrawal", _address)
}

// ClaimWithdrawal is a paid mutator transaction binding the contract method 0xa3066aab.
//
// Solidity: function claimWithdrawal(address _address) returns()
func (_Libgno *LibgnoSession) ClaimWithdrawal(_address common.Address) (*types.Transaction, error) {
	return _Libgno.Contract.ClaimWithdrawal(&_Libgno.TransactOpts, _address)
}

// ClaimWithdrawal is a paid mutator transaction binding the contract method 0xa3066aab.
//
// Solidity: function claimWithdrawal(address _address) returns()
func (_Libgno *LibgnoTransactorSession) ClaimWithdrawal(_address common.Address) (*types.Transaction, error) {
	return _Libgno.Contract.ClaimWithdrawal(&_Libgno.TransactOpts, _address)
}

// ClaimWithdrawals is a paid mutator transaction binding the contract method 0xbb30b8fd.
//
// Solidity: function claimWithdrawals(address[] _addresses) returns()
func (_Libgno *LibgnoTransactor) ClaimWithdrawals(opts *bind.TransactOpts, _addresses []common.Address) (*types.Transaction, error) {
	return _Libgno.contract.Transact(opts, "claimWithdrawals", _addresses)
}

// ClaimWithdrawals is a paid mutator transaction binding the contract method 0xbb30b8fd.
//
// Solidity: function claimWithdrawals(address[] _addresses) returns()
func (_Libgno *LibgnoSession) ClaimWithdrawals(_addresses []common.Address) (*types.Transaction, error) {
	return _Libgno.Contract.ClaimWithdrawals(&_Libgno.TransactOpts, _addresses)
}

// ClaimWithdrawals is a paid mutator transaction binding the contract method 0xbb30b8fd.
//
// Solidity: function claimWithdrawals(address[] _addresses) returns()
func (_Libgno *LibgnoTransactorSession) ClaimWithdrawals(_addresses []common.Address) (*types.Transaction, error) {
	return _Libgno.Contract.ClaimWithdrawals(&_Libgno.TransactOpts, _addresses)
}

// Deposit is a paid mutator transaction binding the contract method 0x0cac9f31.
//
// Solidity: function deposit(bytes pubkey, bytes withdrawal_credentials, bytes signature, bytes32 deposit_data_root, uint256 stake_amount) returns()
func (_Libgno *LibgnoTransactor) Deposit(opts *bind.TransactOpts, pubkey []byte, withdrawal_credentials []byte, signature []byte, deposit_data_root [32]byte, stake_amount *big.Int) (*types.Transaction, error) {
	return _Libgno.contract.Transact(opts, "deposit", pubkey, withdrawal_credentials, signature, deposit_data_root, stake_amount)
}

// Deposit is a paid mutator transaction binding the contract method 0x0cac9f31.
//
// Solidity: function deposit(bytes pubkey, bytes withdrawal_credentials, bytes signature, bytes32 deposit_data_root, uint256 stake_amount) returns()
func (_Libgno *LibgnoSession) Deposit(pubkey []byte, withdrawal_credentials []byte, signature []byte, deposit_data_root [32]byte, stake_amount *big.Int) (*types.Transaction, error) {
	return _Libgno.Contract.Deposit(&_Libgno.TransactOpts, pubkey, withdrawal_credentials, signature, deposit_data_root, stake_amount)
}

// Deposit is a paid mutator transaction binding the contract method 0x0cac9f31.
//
// Solidity: function deposit(bytes pubkey, bytes withdrawal_credentials, bytes signature, bytes32 deposit_data_root, uint256 stake_amount) returns()
func (_Libgno *LibgnoTransactorSession) Deposit(pubkey []byte, withdrawal_credentials []byte, signature []byte, deposit_data_root [32]byte, stake_amount *big.Int) (*types.Transaction, error) {
	return _Libgno.Contract.Deposit(&_Libgno.TransactOpts, pubkey, withdrawal_credentials, signature, deposit_data_root, stake_amount)
}

// ExecuteSystemWithdrawals is a paid mutator transaction binding the contract method 0x79d0c0bc.
//
// Solidity: function executeSystemWithdrawals(uint256 _maxNumberOfFailedWithdrawalsToProcess, uint64[] _amounts, address[] _addresses) returns()
func (_Libgno *LibgnoTransactor) ExecuteSystemWithdrawals(opts *bind.TransactOpts, _maxNumberOfFailedWithdrawalsToProcess *big.Int, _amounts []uint64, _addresses []common.Address) (*types.Transaction, error) {
	return _Libgno.contract.Transact(opts, "executeSystemWithdrawals", _maxNumberOfFailedWithdrawalsToProcess, _amounts, _addresses)
}

// ExecuteSystemWithdrawals is a paid mutator transaction binding the contract method 0x79d0c0bc.
//
// Solidity: function executeSystemWithdrawals(uint256 _maxNumberOfFailedWithdrawalsToProcess, uint64[] _amounts, address[] _addresses) returns()
func (_Libgno *LibgnoSession) ExecuteSystemWithdrawals(_maxNumberOfFailedWithdrawalsToProcess *big.Int, _amounts []uint64, _addresses []common.Address) (*types.Transaction, error) {
	return _Libgno.Contract.ExecuteSystemWithdrawals(&_Libgno.TransactOpts, _maxNumberOfFailedWithdrawalsToProcess, _amounts, _addresses)
}

// ExecuteSystemWithdrawals is a paid mutator transaction binding the contract method 0x79d0c0bc.
//
// Solidity: function executeSystemWithdrawals(uint256 _maxNumberOfFailedWithdrawalsToProcess, uint64[] _amounts, address[] _addresses) returns()
func (_Libgno *LibgnoTransactorSession) ExecuteSystemWithdrawals(_maxNumberOfFailedWithdrawalsToProcess *big.Int, _amounts []uint64, _addresses []common.Address) (*types.Transaction, error) {
	return _Libgno.Contract.ExecuteSystemWithdrawals(&_Libgno.TransactOpts, _maxNumberOfFailedWithdrawalsToProcess, _amounts, _addresses)
}

// FlushTokensTo is a paid mutator transaction binding the contract method 0xd74d11e9.
//
// Solidity: function flushTokensTo(address addr) returns()
func (_Libgno *LibgnoTransactor) FlushTokensTo(opts *bind.TransactOpts, addr common.Address) (*types.Transaction, error) {
	return _Libgno.contract.Transact(opts, "flushTokensTo", addr)
}

// FlushTokensTo is a paid mutator transaction binding the contract method 0xd74d11e9.
//
// Solidity: function flushTokensTo(address addr) returns()
func (_Libgno *LibgnoSession) FlushTokensTo(addr common.Address) (*types.Transaction, error) {
	return _Libgno.Contract.FlushTokensTo(&_Libgno.TransactOpts, addr)
}

// FlushTokensTo is a paid mutator transaction binding the contract method 0xd74d11e9.
//
// Solidity: function flushTokensTo(address addr) returns()
func (_Libgno *LibgnoTransactorSession) FlushTokensTo(addr common.Address) (*types.Transaction, error) {
	return _Libgno.Contract.FlushTokensTo(&_Libgno.TransactOpts, addr)
}

// OnTokenTransfer is a paid mutator transaction binding the contract method 0xa4c0ed36.
//
// Solidity: function onTokenTransfer(address , uint256 stake_amount, bytes data) returns(bool)
func (_Libgno *LibgnoTransactor) OnTokenTransfer(opts *bind.TransactOpts, arg0 common.Address, stake_amount *big.Int, data []byte) (*types.Transaction, error) {
	return _Libgno.contract.Transact(opts, "onTokenTransfer", arg0, stake_amount, data)
}

// OnTokenTransfer is a paid mutator transaction binding the contract method 0xa4c0ed36.
//
// Solidity: function onTokenTransfer(address , uint256 stake_amount, bytes data) returns(bool)
func (_Libgno *LibgnoSession) OnTokenTransfer(arg0 common.Address, stake_amount *big.Int, data []byte) (*types.Transaction, error) {
	return _Libgno.Contract.OnTokenTransfer(&_Libgno.TransactOpts, arg0, stake_amount, data)
}

// OnTokenTransfer is a paid mutator transaction binding the contract method 0xa4c0ed36.
//
// Solidity: function onTokenTransfer(address , uint256 stake_amount, bytes data) returns(bool)
func (_Libgno *LibgnoTransactorSession) OnTokenTransfer(arg0 common.Address, stake_amount *big.Int, data []byte) (*types.Transaction, error) {
	return _Libgno.Contract.OnTokenTransfer(&_Libgno.TransactOpts, arg0, stake_amount, data)
}

// Pause is a paid mutator transaction binding the contract method 0x8456cb59.
//
// Solidity: function pause() returns()
func (_Libgno *LibgnoTransactor) Pause(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _Libgno.contract.Transact(opts, "pause")
}

// Pause is a paid mutator transaction binding the contract method 0x8456cb59.
//
// Solidity: function pause() returns()
func (_Libgno *LibgnoSession) Pause() (*types.Transaction, error) {
	return _Libgno.Contract.Pause(&_Libgno.TransactOpts)
}

// Pause is a paid mutator transaction binding the contract method 0x8456cb59.
//
// Solidity: function pause() returns()
func (_Libgno *LibgnoTransactorSession) Pause() (*types.Transaction, error) {
	return _Libgno.Contract.Pause(&_Libgno.TransactOpts)
}

// Unpause is a paid mutator transaction binding the contract method 0x3f4ba83a.
//
// Solidity: function unpause() returns()
func (_Libgno *LibgnoTransactor) Unpause(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _Libgno.contract.Transact(opts, "unpause")
}

// Unpause is a paid mutator transaction binding the contract method 0x3f4ba83a.
//
// Solidity: function unpause() returns()
func (_Libgno *LibgnoSession) Unpause() (*types.Transaction, error) {
	return _Libgno.Contract.Unpause(&_Libgno.TransactOpts)
}

// Unpause is a paid mutator transaction binding the contract method 0x3f4ba83a.
//
// Solidity: function unpause() returns()
func (_Libgno *LibgnoTransactorSession) Unpause() (*types.Transaction, error) {
	return _Libgno.Contract.Unpause(&_Libgno.TransactOpts)
}

// UnwrapTokens is a paid mutator transaction binding the contract method 0x4694bd1e.
//
// Solidity: function unwrapTokens(address _unwrapper, address _token) returns()
func (_Libgno *LibgnoTransactor) UnwrapTokens(opts *bind.TransactOpts, _unwrapper common.Address, _token common.Address) (*types.Transaction, error) {
	return _Libgno.contract.Transact(opts, "unwrapTokens", _unwrapper, _token)
}

// UnwrapTokens is a paid mutator transaction binding the contract method 0x4694bd1e.
//
// Solidity: function unwrapTokens(address _unwrapper, address _token) returns()
func (_Libgno *LibgnoSession) UnwrapTokens(_unwrapper common.Address, _token common.Address) (*types.Transaction, error) {
	return _Libgno.Contract.UnwrapTokens(&_Libgno.TransactOpts, _unwrapper, _token)
}

// UnwrapTokens is a paid mutator transaction binding the contract method 0x4694bd1e.
//
// Solidity: function unwrapTokens(address _unwrapper, address _token) returns()
func (_Libgno *LibgnoTransactorSession) UnwrapTokens(_unwrapper common.Address, _token common.Address) (*types.Transaction, error) {
	return _Libgno.Contract.UnwrapTokens(&_Libgno.TransactOpts, _unwrapper, _token)
}

// LibgnoDepositEventIterator is returned from FilterDepositEvent and is used to iterate over the raw logs and unpacked data for DepositEvent events raised by the Libgno contract.
type LibgnoDepositEventIterator struct {
	Event *LibgnoDepositEvent // Event containing the contract specifics and raw log

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
func (it *LibgnoDepositEventIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(LibgnoDepositEvent)
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
		it.Event = new(LibgnoDepositEvent)
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
func (it *LibgnoDepositEventIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *LibgnoDepositEventIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// LibgnoDepositEvent represents a DepositEvent event raised by the Libgno contract.
type LibgnoDepositEvent struct {
	Pubkey                []byte
	WithdrawalCredentials []byte
	Amount                []byte
	Signature             []byte
	Index                 []byte
	Raw                   types.Log // Blockchain specific contextual infos
}

// FilterDepositEvent is a free log retrieval operation binding the contract event 0x649bbc62d0e31342afea4e5cd82d4049e7e1ee912fc0889aa790803be39038c5.
//
// Solidity: event DepositEvent(bytes pubkey, bytes withdrawal_credentials, bytes amount, bytes signature, bytes index)
func (_Libgno *LibgnoFilterer) FilterDepositEvent(opts *bind.FilterOpts) (*LibgnoDepositEventIterator, error) {

	logs, sub, err := _Libgno.contract.FilterLogs(opts, "DepositEvent")
	if err != nil {
		return nil, err
	}
	return &LibgnoDepositEventIterator{contract: _Libgno.contract, event: "DepositEvent", logs: logs, sub: sub}, nil
}

// WatchDepositEvent is a free log subscription operation binding the contract event 0x649bbc62d0e31342afea4e5cd82d4049e7e1ee912fc0889aa790803be39038c5.
//
// Solidity: event DepositEvent(bytes pubkey, bytes withdrawal_credentials, bytes amount, bytes signature, bytes index)
func (_Libgno *LibgnoFilterer) WatchDepositEvent(opts *bind.WatchOpts, sink chan<- *LibgnoDepositEvent) (event.Subscription, error) {

	logs, sub, err := _Libgno.contract.WatchLogs(opts, "DepositEvent")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(LibgnoDepositEvent)
				if err := _Libgno.contract.UnpackLog(event, "DepositEvent", log); err != nil {
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

// ParseDepositEvent is a log parse operation binding the contract event 0x649bbc62d0e31342afea4e5cd82d4049e7e1ee912fc0889aa790803be39038c5.
//
// Solidity: event DepositEvent(bytes pubkey, bytes withdrawal_credentials, bytes amount, bytes signature, bytes index)
func (_Libgno *LibgnoFilterer) ParseDepositEvent(log types.Log) (*LibgnoDepositEvent, error) {
	event := new(LibgnoDepositEvent)
	if err := _Libgno.contract.UnpackLog(event, "DepositEvent", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// LibgnoPausedIterator is returned from FilterPaused and is used to iterate over the raw logs and unpacked data for Paused events raised by the Libgno contract.
type LibgnoPausedIterator struct {
	Event *LibgnoPaused // Event containing the contract specifics and raw log

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
func (it *LibgnoPausedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(LibgnoPaused)
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
		it.Event = new(LibgnoPaused)
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
func (it *LibgnoPausedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *LibgnoPausedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// LibgnoPaused represents a Paused event raised by the Libgno contract.
type LibgnoPaused struct {
	Account common.Address
	Raw     types.Log // Blockchain specific contextual infos
}

// FilterPaused is a free log retrieval operation binding the contract event 0x62e78cea01bee320cd4e420270b5ea74000d11b0c9f74754ebdbfc544b05a258.
//
// Solidity: event Paused(address account)
func (_Libgno *LibgnoFilterer) FilterPaused(opts *bind.FilterOpts) (*LibgnoPausedIterator, error) {

	logs, sub, err := _Libgno.contract.FilterLogs(opts, "Paused")
	if err != nil {
		return nil, err
	}
	return &LibgnoPausedIterator{contract: _Libgno.contract, event: "Paused", logs: logs, sub: sub}, nil
}

// WatchPaused is a free log subscription operation binding the contract event 0x62e78cea01bee320cd4e420270b5ea74000d11b0c9f74754ebdbfc544b05a258.
//
// Solidity: event Paused(address account)
func (_Libgno *LibgnoFilterer) WatchPaused(opts *bind.WatchOpts, sink chan<- *LibgnoPaused) (event.Subscription, error) {

	logs, sub, err := _Libgno.contract.WatchLogs(opts, "Paused")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(LibgnoPaused)
				if err := _Libgno.contract.UnpackLog(event, "Paused", log); err != nil {
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

// ParsePaused is a log parse operation binding the contract event 0x62e78cea01bee320cd4e420270b5ea74000d11b0c9f74754ebdbfc544b05a258.
//
// Solidity: event Paused(address account)
func (_Libgno *LibgnoFilterer) ParsePaused(log types.Log) (*LibgnoPaused, error) {
	event := new(LibgnoPaused)
	if err := _Libgno.contract.UnpackLog(event, "Paused", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// LibgnoUnpausedIterator is returned from FilterUnpaused and is used to iterate over the raw logs and unpacked data for Unpaused events raised by the Libgno contract.
type LibgnoUnpausedIterator struct {
	Event *LibgnoUnpaused // Event containing the contract specifics and raw log

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
func (it *LibgnoUnpausedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(LibgnoUnpaused)
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
		it.Event = new(LibgnoUnpaused)
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
func (it *LibgnoUnpausedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *LibgnoUnpausedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// LibgnoUnpaused represents a Unpaused event raised by the Libgno contract.
type LibgnoUnpaused struct {
	Account common.Address
	Raw     types.Log // Blockchain specific contextual infos
}

// FilterUnpaused is a free log retrieval operation binding the contract event 0x5db9ee0a495bf2e6ff9c91a7834c1ba4fdd244a5e8aa4e537bd38aeae4b073aa.
//
// Solidity: event Unpaused(address account)
func (_Libgno *LibgnoFilterer) FilterUnpaused(opts *bind.FilterOpts) (*LibgnoUnpausedIterator, error) {

	logs, sub, err := _Libgno.contract.FilterLogs(opts, "Unpaused")
	if err != nil {
		return nil, err
	}
	return &LibgnoUnpausedIterator{contract: _Libgno.contract, event: "Unpaused", logs: logs, sub: sub}, nil
}

// WatchUnpaused is a free log subscription operation binding the contract event 0x5db9ee0a495bf2e6ff9c91a7834c1ba4fdd244a5e8aa4e537bd38aeae4b073aa.
//
// Solidity: event Unpaused(address account)
func (_Libgno *LibgnoFilterer) WatchUnpaused(opts *bind.WatchOpts, sink chan<- *LibgnoUnpaused) (event.Subscription, error) {

	logs, sub, err := _Libgno.contract.WatchLogs(opts, "Unpaused")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(LibgnoUnpaused)
				if err := _Libgno.contract.UnpackLog(event, "Unpaused", log); err != nil {
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

// ParseUnpaused is a log parse operation binding the contract event 0x5db9ee0a495bf2e6ff9c91a7834c1ba4fdd244a5e8aa4e537bd38aeae4b073aa.
//
// Solidity: event Unpaused(address account)
func (_Libgno *LibgnoFilterer) ParseUnpaused(log types.Log) (*LibgnoUnpaused, error) {
	event := new(LibgnoUnpaused)
	if err := _Libgno.contract.UnpackLog(event, "Unpaused", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}
