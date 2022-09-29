// Code generated - DO NOT EDIT.
// This file is a generated binding and any manual changes will be lost.

package bindings

import (
	"errors"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum"
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

// SimpleERC20MetaData contains all meta data concerning the SimpleERC20 contract.
var SimpleERC20MetaData = &bind.MetaData{
	ABI: "[{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"_initialAmount\",\"type\":\"uint256\"},{\"internalType\":\"string\",\"name\":\"_tokenName\",\"type\":\"string\"},{\"internalType\":\"uint8\",\"name\":\"_decimalUnits\",\"type\":\"uint8\"},{\"internalType\":\"string\",\"name\":\"_tokenSymbol\",\"type\":\"string\"}],\"stateMutability\":\"nonpayable\",\"type\":\"constructor\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"_owner\",\"type\":\"address\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"_spender\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"_value\",\"type\":\"uint256\"}],\"name\":\"Approval\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"_from\",\"type\":\"address\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"_to\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"_value\",\"type\":\"uint256\"}],\"name\":\"Transfer\",\"type\":\"event\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"_owner\",\"type\":\"address\"},{\"internalType\":\"address\",\"name\":\"_spender\",\"type\":\"address\"}],\"name\":\"allowance\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"remaining\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"},{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"name\":\"allowed\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"_spender\",\"type\":\"address\"},{\"internalType\":\"uint256\",\"name\":\"_value\",\"type\":\"uint256\"}],\"name\":\"approve\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"success\",\"type\":\"bool\"}],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"_owner\",\"type\":\"address\"}],\"name\":\"balanceOf\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"balance\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"name\":\"balances\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"decimals\",\"outputs\":[{\"internalType\":\"uint8\",\"name\":\"\",\"type\":\"uint8\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"destroy\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"name\",\"outputs\":[{\"internalType\":\"string\",\"name\":\"\",\"type\":\"string\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"symbol\",\"outputs\":[{\"internalType\":\"string\",\"name\":\"\",\"type\":\"string\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"totalSupply\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"_to\",\"type\":\"address\"},{\"internalType\":\"uint256\",\"name\":\"_value\",\"type\":\"uint256\"}],\"name\":\"transfer\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"success\",\"type\":\"bool\"}],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"_from\",\"type\":\"address\"},{\"internalType\":\"address\",\"name\":\"_to\",\"type\":\"address\"},{\"internalType\":\"uint256\",\"name\":\"_value\",\"type\":\"uint256\"}],\"name\":\"transferFrom\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"success\",\"type\":\"bool\"}],\"stateMutability\":\"nonpayable\",\"type\":\"function\"}]",
	Bin: "0x60806040523480156200001157600080fd5b506040516200142d3803806200142d83398181016040528101906200003791906200039e565b836000803373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020019081526020016000208190555083600581905550826002908051906020019062000099929190620000d8565b5081600360006101000a81548160ff021916908360ff1602179055508060049080519060200190620000cd929190620000d8565b5050505050620004b2565b828054620000e6906200047d565b90600052602060002090601f0160209004810192826200010a576000855562000156565b82601f106200012557805160ff191683800117855562000156565b8280016001018555821562000156579182015b828111156200015557825182559160200191906001019062000138565b5b50905062000165919062000169565b5090565b5b80821115620001845760008160009055506001016200016a565b5090565b6000604051905090565b600080fd5b600080fd5b6000819050919050565b620001b1816200019c565b8114620001bd57600080fd5b50565b600081519050620001d181620001a6565b92915050565b600080fd5b600080fd5b6000601f19601f8301169050919050565b7f4e487b7100000000000000000000000000000000000000000000000000000000600052604160045260246000fd5b6200022c82620001e1565b810181811067ffffffffffffffff821117156200024e576200024d620001f2565b5b80604052505050565b60006200026362000188565b905062000271828262000221565b919050565b600067ffffffffffffffff821115620002945762000293620001f2565b5b6200029f82620001e1565b9050602081019050919050565b60005b83811015620002cc578082015181840152602081019050620002af565b83811115620002dc576000848401525b50505050565b6000620002f9620002f38462000276565b62000257565b905082815260208101848484011115620003185762000317620001dc565b5b62000325848285620002ac565b509392505050565b600082601f830112620003455762000344620001d7565b5b815162000357848260208601620002e2565b91505092915050565b600060ff82169050919050565b620003788162000360565b81146200038457600080fd5b50565b60008151905062000398816200036d565b92915050565b60008060008060808587031215620003bb57620003ba62000192565b5b6000620003cb87828801620001c0565b945050602085015167ffffffffffffffff811115620003ef57620003ee62000197565b5b620003fd878288016200032d565b9350506040620004108782880162000387565b925050606085015167ffffffffffffffff81111562000434576200043362000197565b5b62000442878288016200032d565b91505092959194509250565b7f4e487b7100000000000000000000000000000000000000000000000000000000600052602260045260246000fd5b600060028204905060018216806200049657607f821691505b602082108103620004ac57620004ab6200044e565b5b50919050565b610f6b80620004c26000396000f3fe608060405234801561001057600080fd5b50600436106100b45760003560e01c80635c658165116100715780635c658165146101a357806370a08231146101d357806383197ef01461020357806395d89b411461020d578063a9059cbb1461022b578063dd62ed3e1461025b576100b4565b806306fdde03146100b9578063095ea7b3146100d757806318160ddd1461010757806323b872dd1461012557806327e235e314610155578063313ce56714610185575b600080fd5b6100c161028b565b6040516100ce9190610af2565b60405180910390f35b6100f160048036038101906100ec9190610bad565b610319565b6040516100fe9190610c08565b60405180910390f35b61010f61040b565b60405161011c9190610c32565b60405180910390f35b61013f600480360381019061013a9190610c4d565b610411565b60405161014c9190610c08565b60405180910390f35b61016f600480360381019061016a9190610ca0565b6106f7565b60405161017c9190610c32565b60405180910390f35b61018d61070f565b60405161019a9190610ce9565b60405180910390f35b6101bd60048036038101906101b89190610d04565b610722565b6040516101ca9190610c32565b60405180910390f35b6101ed60048036038101906101e89190610ca0565b610747565b6040516101fa9190610c32565b60405180910390f35b61020b61078f565b005b6102156107a8565b6040516102229190610af2565b60405180910390f35b61024560048036038101906102409190610bad565b610836565b6040516102529190610c08565b60405180910390f35b61027560048036038101906102709190610d04565b6109d2565b6040516102829190610c32565b60405180910390f35b6002805461029890610d73565b80601f01602080910402602001604051908101604052809291908181526020018280546102c490610d73565b80156103115780601f106102e657610100808354040283529160200191610311565b820191906000526020600020905b8154815290600101906020018083116102f457829003601f168201915b505050505081565b600081600160003373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002060008573ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001908152602001600020819055508273ffffffffffffffffffffffffffffffffffffffff163373ffffffffffffffffffffffffffffffffffffffff167f8c5be1e5ebec7d5bd14f71427d1e84f3dd0314c0f7b2291e5b200ac8c7c3b925846040516103f99190610c32565b60405180910390a36001905092915050565b60055481565b600080600160008673ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002060003373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001908152602001600020549050826000808773ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002054101580156104e15750828110155b610520576040517f08c379a000000000000000000000000000000000000000000000000000000000815260040161051790610df0565b60405180910390fd5b826000808673ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001908152602001600020600082825461056e9190610e3f565b92505081905550826000808773ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002060008282546105c39190610e95565b925050819055507fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff8110156106865782600160008773ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002060003373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001908152602001600020600082825461067e9190610e95565b925050819055505b8373ffffffffffffffffffffffffffffffffffffffff168573ffffffffffffffffffffffffffffffffffffffff167fddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef856040516106e39190610c32565b60405180910390a360019150509392505050565b60006020528060005260406000206000915090505481565b600360009054906101000a900460ff1681565b6001602052816000526040600020602052806000526040600020600091509150505481565b60008060008373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001908152602001600020549050919050565b3373ffffffffffffffffffffffffffffffffffffffff16ff5b600480546107b590610d73565b80601f01602080910402602001604051908101604052809291908181526020018280546107e190610d73565b801561082e5780601f106108035761010080835404028352916020019161082e565b820191906000526020600020905b81548152906001019060200180831161081157829003601f168201915b505050505081565b6000816000803373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020019081526020016000205410156108b9576040517f08c379a00000000000000000000000000000000000000000000000000000000081526004016108b090610f15565b60405180910390fd5b816000803373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002060008282546109079190610e95565b92505081905550816000808573ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001908152602001600020600082825461095c9190610e3f565b925050819055508273ffffffffffffffffffffffffffffffffffffffff163373ffffffffffffffffffffffffffffffffffffffff167fddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef846040516109c09190610c32565b60405180910390a36001905092915050565b6000600160008473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002060008373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002054905092915050565b600081519050919050565b600082825260208201905092915050565b60005b83811015610a93578082015181840152602081019050610a78565b83811115610aa2576000848401525b50505050565b6000601f19601f8301169050919050565b6000610ac482610a59565b610ace8185610a64565b9350610ade818560208601610a75565b610ae781610aa8565b840191505092915050565b60006020820190508181036000830152610b0c8184610ab9565b905092915050565b600080fd5b600073ffffffffffffffffffffffffffffffffffffffff82169050919050565b6000610b4482610b19565b9050919050565b610b5481610b39565b8114610b5f57600080fd5b50565b600081359050610b7181610b4b565b92915050565b6000819050919050565b610b8a81610b77565b8114610b9557600080fd5b50565b600081359050610ba781610b81565b92915050565b60008060408385031215610bc457610bc3610b14565b5b6000610bd285828601610b62565b9250506020610be385828601610b98565b9150509250929050565b60008115159050919050565b610c0281610bed565b82525050565b6000602082019050610c1d6000830184610bf9565b92915050565b610c2c81610b77565b82525050565b6000602082019050610c476000830184610c23565b92915050565b600080600060608486031215610c6657610c65610b14565b5b6000610c7486828701610b62565b9350506020610c8586828701610b62565b9250506040610c9686828701610b98565b9150509250925092565b600060208284031215610cb657610cb5610b14565b5b6000610cc484828501610b62565b91505092915050565b600060ff82169050919050565b610ce381610ccd565b82525050565b6000602082019050610cfe6000830184610cda565b92915050565b60008060408385031215610d1b57610d1a610b14565b5b6000610d2985828601610b62565b9250506020610d3a85828601610b62565b9150509250929050565b7f4e487b7100000000000000000000000000000000000000000000000000000000600052602260045260246000fd5b60006002820490506001821680610d8b57607f821691505b602082108103610d9e57610d9d610d44565b5b50919050565b7f62616420616c6c6f77616e636500000000000000000000000000000000000000600082015250565b6000610dda600d83610a64565b9150610de582610da4565b602082019050919050565b60006020820190508181036000830152610e0981610dcd565b9050919050565b7f4e487b7100000000000000000000000000000000000000000000000000000000600052601160045260246000fd5b6000610e4a82610b77565b9150610e5583610b77565b9250827fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff03821115610e8a57610e89610e10565b5b828201905092915050565b6000610ea082610b77565b9150610eab83610b77565b925082821015610ebe57610ebd610e10565b5b828203905092915050565b7f696e73756666696369656e742062616c616e6365000000000000000000000000600082015250565b6000610eff601483610a64565b9150610f0a82610ec9565b602082019050919050565b60006020820190508181036000830152610f2e81610ef2565b905091905056fea2646970667358221220254563d055790a284838c3de2703de33b9224efe15c84ef819c82efd817692f664736f6c634300080d0033",
}

// SimpleERC20ABI is the input ABI used to generate the binding from.
// Deprecated: Use SimpleERC20MetaData.ABI instead.
var SimpleERC20ABI = SimpleERC20MetaData.ABI

// SimpleERC20Bin is the compiled bytecode used for deploying new contracts.
// Deprecated: Use SimpleERC20MetaData.Bin instead.
var SimpleERC20Bin = SimpleERC20MetaData.Bin

// DeploySimpleERC20 deploys a new Ethereum contract, binding an instance of SimpleERC20 to it.
func DeploySimpleERC20(auth *bind.TransactOpts, backend bind.ContractBackend, _initialAmount *big.Int, _tokenName string, _decimalUnits uint8, _tokenSymbol string) (common.Address, *types.Transaction, *SimpleERC20, error) {
	parsed, err := SimpleERC20MetaData.GetAbi()
	if err != nil {
		return common.Address{}, nil, nil, err
	}
	if parsed == nil {
		return common.Address{}, nil, nil, errors.New("GetABI returned nil")
	}

	address, tx, contract, err := bind.DeployContract(auth, *parsed, common.FromHex(SimpleERC20Bin), backend, _initialAmount, _tokenName, _decimalUnits, _tokenSymbol)
	if err != nil {
		return common.Address{}, nil, nil, err
	}
	return address, tx, &SimpleERC20{SimpleERC20Caller: SimpleERC20Caller{contract: contract}, SimpleERC20Transactor: SimpleERC20Transactor{contract: contract}, SimpleERC20Filterer: SimpleERC20Filterer{contract: contract}}, nil
}

// SimpleERC20 is an auto generated Go binding around an Ethereum contract.
type SimpleERC20 struct {
	SimpleERC20Caller     // Read-only binding to the contract
	SimpleERC20Transactor // Write-only binding to the contract
	SimpleERC20Filterer   // Log filterer for contract events
}

// SimpleERC20Caller is an auto generated read-only Go binding around an Ethereum contract.
type SimpleERC20Caller struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// SimpleERC20Transactor is an auto generated write-only Go binding around an Ethereum contract.
type SimpleERC20Transactor struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// SimpleERC20Filterer is an auto generated log filtering Go binding around an Ethereum contract events.
type SimpleERC20Filterer struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// SimpleERC20Session is an auto generated Go binding around an Ethereum contract,
// with pre-set call and transact options.
type SimpleERC20Session struct {
	Contract     *SimpleERC20      // Generic contract binding to set the session for
	CallOpts     bind.CallOpts     // Call options to use throughout this session
	TransactOpts bind.TransactOpts // Transaction auth options to use throughout this session
}

// SimpleERC20CallerSession is an auto generated read-only Go binding around an Ethereum contract,
// with pre-set call options.
type SimpleERC20CallerSession struct {
	Contract *SimpleERC20Caller // Generic contract caller binding to set the session for
	CallOpts bind.CallOpts      // Call options to use throughout this session
}

// SimpleERC20TransactorSession is an auto generated write-only Go binding around an Ethereum contract,
// with pre-set transact options.
type SimpleERC20TransactorSession struct {
	Contract     *SimpleERC20Transactor // Generic contract transactor binding to set the session for
	TransactOpts bind.TransactOpts      // Transaction auth options to use throughout this session
}

// SimpleERC20Raw is an auto generated low-level Go binding around an Ethereum contract.
type SimpleERC20Raw struct {
	Contract *SimpleERC20 // Generic contract binding to access the raw methods on
}

// SimpleERC20CallerRaw is an auto generated low-level read-only Go binding around an Ethereum contract.
type SimpleERC20CallerRaw struct {
	Contract *SimpleERC20Caller // Generic read-only contract binding to access the raw methods on
}

// SimpleERC20TransactorRaw is an auto generated low-level write-only Go binding around an Ethereum contract.
type SimpleERC20TransactorRaw struct {
	Contract *SimpleERC20Transactor // Generic write-only contract binding to access the raw methods on
}

// NewSimpleERC20 creates a new instance of SimpleERC20, bound to a specific deployed contract.
func NewSimpleERC20(address common.Address, backend bind.ContractBackend) (*SimpleERC20, error) {
	contract, err := bindSimpleERC20(address, backend, backend, backend)
	if err != nil {
		return nil, err
	}
	return &SimpleERC20{SimpleERC20Caller: SimpleERC20Caller{contract: contract}, SimpleERC20Transactor: SimpleERC20Transactor{contract: contract}, SimpleERC20Filterer: SimpleERC20Filterer{contract: contract}}, nil
}

// NewSimpleERC20Caller creates a new read-only instance of SimpleERC20, bound to a specific deployed contract.
func NewSimpleERC20Caller(address common.Address, caller bind.ContractCaller) (*SimpleERC20Caller, error) {
	contract, err := bindSimpleERC20(address, caller, nil, nil)
	if err != nil {
		return nil, err
	}
	return &SimpleERC20Caller{contract: contract}, nil
}

// NewSimpleERC20Transactor creates a new write-only instance of SimpleERC20, bound to a specific deployed contract.
func NewSimpleERC20Transactor(address common.Address, transactor bind.ContractTransactor) (*SimpleERC20Transactor, error) {
	contract, err := bindSimpleERC20(address, nil, transactor, nil)
	if err != nil {
		return nil, err
	}
	return &SimpleERC20Transactor{contract: contract}, nil
}

// NewSimpleERC20Filterer creates a new log filterer instance of SimpleERC20, bound to a specific deployed contract.
func NewSimpleERC20Filterer(address common.Address, filterer bind.ContractFilterer) (*SimpleERC20Filterer, error) {
	contract, err := bindSimpleERC20(address, nil, nil, filterer)
	if err != nil {
		return nil, err
	}
	return &SimpleERC20Filterer{contract: contract}, nil
}

// bindSimpleERC20 binds a generic wrapper to an already deployed contract.
func bindSimpleERC20(address common.Address, caller bind.ContractCaller, transactor bind.ContractTransactor, filterer bind.ContractFilterer) (*bind.BoundContract, error) {
	parsed, err := abi.JSON(strings.NewReader(SimpleERC20ABI))
	if err != nil {
		return nil, err
	}
	return bind.NewBoundContract(address, parsed, caller, transactor, filterer), nil
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_SimpleERC20 *SimpleERC20Raw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _SimpleERC20.Contract.SimpleERC20Caller.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_SimpleERC20 *SimpleERC20Raw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _SimpleERC20.Contract.SimpleERC20Transactor.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_SimpleERC20 *SimpleERC20Raw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _SimpleERC20.Contract.SimpleERC20Transactor.contract.Transact(opts, method, params...)
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_SimpleERC20 *SimpleERC20CallerRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _SimpleERC20.Contract.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_SimpleERC20 *SimpleERC20TransactorRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _SimpleERC20.Contract.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_SimpleERC20 *SimpleERC20TransactorRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _SimpleERC20.Contract.contract.Transact(opts, method, params...)
}

// Allowance is a free data retrieval call binding the contract method 0xdd62ed3e.
//
// Solidity: function allowance(address _owner, address _spender) view returns(uint256 remaining)
func (_SimpleERC20 *SimpleERC20Caller) Allowance(opts *bind.CallOpts, _owner common.Address, _spender common.Address) (*big.Int, error) {
	var out []interface{}
	err := _SimpleERC20.contract.Call(opts, &out, "allowance", _owner, _spender)

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// Allowance is a free data retrieval call binding the contract method 0xdd62ed3e.
//
// Solidity: function allowance(address _owner, address _spender) view returns(uint256 remaining)
func (_SimpleERC20 *SimpleERC20Session) Allowance(_owner common.Address, _spender common.Address) (*big.Int, error) {
	return _SimpleERC20.Contract.Allowance(&_SimpleERC20.CallOpts, _owner, _spender)
}

// Allowance is a free data retrieval call binding the contract method 0xdd62ed3e.
//
// Solidity: function allowance(address _owner, address _spender) view returns(uint256 remaining)
func (_SimpleERC20 *SimpleERC20CallerSession) Allowance(_owner common.Address, _spender common.Address) (*big.Int, error) {
	return _SimpleERC20.Contract.Allowance(&_SimpleERC20.CallOpts, _owner, _spender)
}

// Allowed is a free data retrieval call binding the contract method 0x5c658165.
//
// Solidity: function allowed(address , address ) view returns(uint256)
func (_SimpleERC20 *SimpleERC20Caller) Allowed(opts *bind.CallOpts, arg0 common.Address, arg1 common.Address) (*big.Int, error) {
	var out []interface{}
	err := _SimpleERC20.contract.Call(opts, &out, "allowed", arg0, arg1)

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// Allowed is a free data retrieval call binding the contract method 0x5c658165.
//
// Solidity: function allowed(address , address ) view returns(uint256)
func (_SimpleERC20 *SimpleERC20Session) Allowed(arg0 common.Address, arg1 common.Address) (*big.Int, error) {
	return _SimpleERC20.Contract.Allowed(&_SimpleERC20.CallOpts, arg0, arg1)
}

// Allowed is a free data retrieval call binding the contract method 0x5c658165.
//
// Solidity: function allowed(address , address ) view returns(uint256)
func (_SimpleERC20 *SimpleERC20CallerSession) Allowed(arg0 common.Address, arg1 common.Address) (*big.Int, error) {
	return _SimpleERC20.Contract.Allowed(&_SimpleERC20.CallOpts, arg0, arg1)
}

// BalanceOf is a free data retrieval call binding the contract method 0x70a08231.
//
// Solidity: function balanceOf(address _owner) view returns(uint256 balance)
func (_SimpleERC20 *SimpleERC20Caller) BalanceOf(opts *bind.CallOpts, _owner common.Address) (*big.Int, error) {
	var out []interface{}
	err := _SimpleERC20.contract.Call(opts, &out, "balanceOf", _owner)

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// BalanceOf is a free data retrieval call binding the contract method 0x70a08231.
//
// Solidity: function balanceOf(address _owner) view returns(uint256 balance)
func (_SimpleERC20 *SimpleERC20Session) BalanceOf(_owner common.Address) (*big.Int, error) {
	return _SimpleERC20.Contract.BalanceOf(&_SimpleERC20.CallOpts, _owner)
}

// BalanceOf is a free data retrieval call binding the contract method 0x70a08231.
//
// Solidity: function balanceOf(address _owner) view returns(uint256 balance)
func (_SimpleERC20 *SimpleERC20CallerSession) BalanceOf(_owner common.Address) (*big.Int, error) {
	return _SimpleERC20.Contract.BalanceOf(&_SimpleERC20.CallOpts, _owner)
}

// Balances is a free data retrieval call binding the contract method 0x27e235e3.
//
// Solidity: function balances(address ) view returns(uint256)
func (_SimpleERC20 *SimpleERC20Caller) Balances(opts *bind.CallOpts, arg0 common.Address) (*big.Int, error) {
	var out []interface{}
	err := _SimpleERC20.contract.Call(opts, &out, "balances", arg0)

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// Balances is a free data retrieval call binding the contract method 0x27e235e3.
//
// Solidity: function balances(address ) view returns(uint256)
func (_SimpleERC20 *SimpleERC20Session) Balances(arg0 common.Address) (*big.Int, error) {
	return _SimpleERC20.Contract.Balances(&_SimpleERC20.CallOpts, arg0)
}

// Balances is a free data retrieval call binding the contract method 0x27e235e3.
//
// Solidity: function balances(address ) view returns(uint256)
func (_SimpleERC20 *SimpleERC20CallerSession) Balances(arg0 common.Address) (*big.Int, error) {
	return _SimpleERC20.Contract.Balances(&_SimpleERC20.CallOpts, arg0)
}

// Decimals is a free data retrieval call binding the contract method 0x313ce567.
//
// Solidity: function decimals() view returns(uint8)
func (_SimpleERC20 *SimpleERC20Caller) Decimals(opts *bind.CallOpts) (uint8, error) {
	var out []interface{}
	err := _SimpleERC20.contract.Call(opts, &out, "decimals")

	if err != nil {
		return *new(uint8), err
	}

	out0 := *abi.ConvertType(out[0], new(uint8)).(*uint8)

	return out0, err

}

// Decimals is a free data retrieval call binding the contract method 0x313ce567.
//
// Solidity: function decimals() view returns(uint8)
func (_SimpleERC20 *SimpleERC20Session) Decimals() (uint8, error) {
	return _SimpleERC20.Contract.Decimals(&_SimpleERC20.CallOpts)
}

// Decimals is a free data retrieval call binding the contract method 0x313ce567.
//
// Solidity: function decimals() view returns(uint8)
func (_SimpleERC20 *SimpleERC20CallerSession) Decimals() (uint8, error) {
	return _SimpleERC20.Contract.Decimals(&_SimpleERC20.CallOpts)
}

// Name is a free data retrieval call binding the contract method 0x06fdde03.
//
// Solidity: function name() view returns(string)
func (_SimpleERC20 *SimpleERC20Caller) Name(opts *bind.CallOpts) (string, error) {
	var out []interface{}
	err := _SimpleERC20.contract.Call(opts, &out, "name")

	if err != nil {
		return *new(string), err
	}

	out0 := *abi.ConvertType(out[0], new(string)).(*string)

	return out0, err

}

// Name is a free data retrieval call binding the contract method 0x06fdde03.
//
// Solidity: function name() view returns(string)
func (_SimpleERC20 *SimpleERC20Session) Name() (string, error) {
	return _SimpleERC20.Contract.Name(&_SimpleERC20.CallOpts)
}

// Name is a free data retrieval call binding the contract method 0x06fdde03.
//
// Solidity: function name() view returns(string)
func (_SimpleERC20 *SimpleERC20CallerSession) Name() (string, error) {
	return _SimpleERC20.Contract.Name(&_SimpleERC20.CallOpts)
}

// Symbol is a free data retrieval call binding the contract method 0x95d89b41.
//
// Solidity: function symbol() view returns(string)
func (_SimpleERC20 *SimpleERC20Caller) Symbol(opts *bind.CallOpts) (string, error) {
	var out []interface{}
	err := _SimpleERC20.contract.Call(opts, &out, "symbol")

	if err != nil {
		return *new(string), err
	}

	out0 := *abi.ConvertType(out[0], new(string)).(*string)

	return out0, err

}

// Symbol is a free data retrieval call binding the contract method 0x95d89b41.
//
// Solidity: function symbol() view returns(string)
func (_SimpleERC20 *SimpleERC20Session) Symbol() (string, error) {
	return _SimpleERC20.Contract.Symbol(&_SimpleERC20.CallOpts)
}

// Symbol is a free data retrieval call binding the contract method 0x95d89b41.
//
// Solidity: function symbol() view returns(string)
func (_SimpleERC20 *SimpleERC20CallerSession) Symbol() (string, error) {
	return _SimpleERC20.Contract.Symbol(&_SimpleERC20.CallOpts)
}

// TotalSupply is a free data retrieval call binding the contract method 0x18160ddd.
//
// Solidity: function totalSupply() view returns(uint256)
func (_SimpleERC20 *SimpleERC20Caller) TotalSupply(opts *bind.CallOpts) (*big.Int, error) {
	var out []interface{}
	err := _SimpleERC20.contract.Call(opts, &out, "totalSupply")

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// TotalSupply is a free data retrieval call binding the contract method 0x18160ddd.
//
// Solidity: function totalSupply() view returns(uint256)
func (_SimpleERC20 *SimpleERC20Session) TotalSupply() (*big.Int, error) {
	return _SimpleERC20.Contract.TotalSupply(&_SimpleERC20.CallOpts)
}

// TotalSupply is a free data retrieval call binding the contract method 0x18160ddd.
//
// Solidity: function totalSupply() view returns(uint256)
func (_SimpleERC20 *SimpleERC20CallerSession) TotalSupply() (*big.Int, error) {
	return _SimpleERC20.Contract.TotalSupply(&_SimpleERC20.CallOpts)
}

// Approve is a paid mutator transaction binding the contract method 0x095ea7b3.
//
// Solidity: function approve(address _spender, uint256 _value) returns(bool success)
func (_SimpleERC20 *SimpleERC20Transactor) Approve(opts *bind.TransactOpts, _spender common.Address, _value *big.Int) (*types.Transaction, error) {
	return _SimpleERC20.contract.Transact(opts, "approve", _spender, _value)
}

// Approve is a paid mutator transaction binding the contract method 0x095ea7b3.
//
// Solidity: function approve(address _spender, uint256 _value) returns(bool success)
func (_SimpleERC20 *SimpleERC20Session) Approve(_spender common.Address, _value *big.Int) (*types.Transaction, error) {
	return _SimpleERC20.Contract.Approve(&_SimpleERC20.TransactOpts, _spender, _value)
}

// Approve is a paid mutator transaction binding the contract method 0x095ea7b3.
//
// Solidity: function approve(address _spender, uint256 _value) returns(bool success)
func (_SimpleERC20 *SimpleERC20TransactorSession) Approve(_spender common.Address, _value *big.Int) (*types.Transaction, error) {
	return _SimpleERC20.Contract.Approve(&_SimpleERC20.TransactOpts, _spender, _value)
}

// Destroy is a paid mutator transaction binding the contract method 0x83197ef0.
//
// Solidity: function destroy() returns()
func (_SimpleERC20 *SimpleERC20Transactor) Destroy(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _SimpleERC20.contract.Transact(opts, "destroy")
}

// Destroy is a paid mutator transaction binding the contract method 0x83197ef0.
//
// Solidity: function destroy() returns()
func (_SimpleERC20 *SimpleERC20Session) Destroy() (*types.Transaction, error) {
	return _SimpleERC20.Contract.Destroy(&_SimpleERC20.TransactOpts)
}

// Destroy is a paid mutator transaction binding the contract method 0x83197ef0.
//
// Solidity: function destroy() returns()
func (_SimpleERC20 *SimpleERC20TransactorSession) Destroy() (*types.Transaction, error) {
	return _SimpleERC20.Contract.Destroy(&_SimpleERC20.TransactOpts)
}

// Transfer is a paid mutator transaction binding the contract method 0xa9059cbb.
//
// Solidity: function transfer(address _to, uint256 _value) returns(bool success)
func (_SimpleERC20 *SimpleERC20Transactor) Transfer(opts *bind.TransactOpts, _to common.Address, _value *big.Int) (*types.Transaction, error) {
	return _SimpleERC20.contract.Transact(opts, "transfer", _to, _value)
}

// Transfer is a paid mutator transaction binding the contract method 0xa9059cbb.
//
// Solidity: function transfer(address _to, uint256 _value) returns(bool success)
func (_SimpleERC20 *SimpleERC20Session) Transfer(_to common.Address, _value *big.Int) (*types.Transaction, error) {
	return _SimpleERC20.Contract.Transfer(&_SimpleERC20.TransactOpts, _to, _value)
}

// Transfer is a paid mutator transaction binding the contract method 0xa9059cbb.
//
// Solidity: function transfer(address _to, uint256 _value) returns(bool success)
func (_SimpleERC20 *SimpleERC20TransactorSession) Transfer(_to common.Address, _value *big.Int) (*types.Transaction, error) {
	return _SimpleERC20.Contract.Transfer(&_SimpleERC20.TransactOpts, _to, _value)
}

// TransferFrom is a paid mutator transaction binding the contract method 0x23b872dd.
//
// Solidity: function transferFrom(address _from, address _to, uint256 _value) returns(bool success)
func (_SimpleERC20 *SimpleERC20Transactor) TransferFrom(opts *bind.TransactOpts, _from common.Address, _to common.Address, _value *big.Int) (*types.Transaction, error) {
	return _SimpleERC20.contract.Transact(opts, "transferFrom", _from, _to, _value)
}

// TransferFrom is a paid mutator transaction binding the contract method 0x23b872dd.
//
// Solidity: function transferFrom(address _from, address _to, uint256 _value) returns(bool success)
func (_SimpleERC20 *SimpleERC20Session) TransferFrom(_from common.Address, _to common.Address, _value *big.Int) (*types.Transaction, error) {
	return _SimpleERC20.Contract.TransferFrom(&_SimpleERC20.TransactOpts, _from, _to, _value)
}

// TransferFrom is a paid mutator transaction binding the contract method 0x23b872dd.
//
// Solidity: function transferFrom(address _from, address _to, uint256 _value) returns(bool success)
func (_SimpleERC20 *SimpleERC20TransactorSession) TransferFrom(_from common.Address, _to common.Address, _value *big.Int) (*types.Transaction, error) {
	return _SimpleERC20.Contract.TransferFrom(&_SimpleERC20.TransactOpts, _from, _to, _value)
}

// SimpleERC20ApprovalIterator is returned from FilterApproval and is used to iterate over the raw logs and unpacked data for Approval events raised by the SimpleERC20 contract.
type SimpleERC20ApprovalIterator struct {
	Event *SimpleERC20Approval // Event containing the contract specifics and raw log

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
func (it *SimpleERC20ApprovalIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(SimpleERC20Approval)
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
		it.Event = new(SimpleERC20Approval)
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
func (it *SimpleERC20ApprovalIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *SimpleERC20ApprovalIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// SimpleERC20Approval represents a Approval event raised by the SimpleERC20 contract.
type SimpleERC20Approval struct {
	Owner   common.Address
	Spender common.Address
	Value   *big.Int
	Raw     types.Log // Blockchain specific contextual infos
}

// FilterApproval is a free log retrieval operation binding the contract event 0x8c5be1e5ebec7d5bd14f71427d1e84f3dd0314c0f7b2291e5b200ac8c7c3b925.
//
// Solidity: event Approval(address indexed _owner, address indexed _spender, uint256 _value)
func (_SimpleERC20 *SimpleERC20Filterer) FilterApproval(opts *bind.FilterOpts, _owner []common.Address, _spender []common.Address) (*SimpleERC20ApprovalIterator, error) {

	var _ownerRule []interface{}
	for _, _ownerItem := range _owner {
		_ownerRule = append(_ownerRule, _ownerItem)
	}
	var _spenderRule []interface{}
	for _, _spenderItem := range _spender {
		_spenderRule = append(_spenderRule, _spenderItem)
	}

	logs, sub, err := _SimpleERC20.contract.FilterLogs(opts, "Approval", _ownerRule, _spenderRule)
	if err != nil {
		return nil, err
	}
	return &SimpleERC20ApprovalIterator{contract: _SimpleERC20.contract, event: "Approval", logs: logs, sub: sub}, nil
}

// WatchApproval is a free log subscription operation binding the contract event 0x8c5be1e5ebec7d5bd14f71427d1e84f3dd0314c0f7b2291e5b200ac8c7c3b925.
//
// Solidity: event Approval(address indexed _owner, address indexed _spender, uint256 _value)
func (_SimpleERC20 *SimpleERC20Filterer) WatchApproval(opts *bind.WatchOpts, sink chan<- *SimpleERC20Approval, _owner []common.Address, _spender []common.Address) (event.Subscription, error) {

	var _ownerRule []interface{}
	for _, _ownerItem := range _owner {
		_ownerRule = append(_ownerRule, _ownerItem)
	}
	var _spenderRule []interface{}
	for _, _spenderItem := range _spender {
		_spenderRule = append(_spenderRule, _spenderItem)
	}

	logs, sub, err := _SimpleERC20.contract.WatchLogs(opts, "Approval", _ownerRule, _spenderRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(SimpleERC20Approval)
				if err := _SimpleERC20.contract.UnpackLog(event, "Approval", log); err != nil {
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

// ParseApproval is a log parse operation binding the contract event 0x8c5be1e5ebec7d5bd14f71427d1e84f3dd0314c0f7b2291e5b200ac8c7c3b925.
//
// Solidity: event Approval(address indexed _owner, address indexed _spender, uint256 _value)
func (_SimpleERC20 *SimpleERC20Filterer) ParseApproval(log types.Log) (*SimpleERC20Approval, error) {
	event := new(SimpleERC20Approval)
	if err := _SimpleERC20.contract.UnpackLog(event, "Approval", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// SimpleERC20TransferIterator is returned from FilterTransfer and is used to iterate over the raw logs and unpacked data for Transfer events raised by the SimpleERC20 contract.
type SimpleERC20TransferIterator struct {
	Event *SimpleERC20Transfer // Event containing the contract specifics and raw log

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
func (it *SimpleERC20TransferIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(SimpleERC20Transfer)
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
		it.Event = new(SimpleERC20Transfer)
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
func (it *SimpleERC20TransferIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *SimpleERC20TransferIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// SimpleERC20Transfer represents a Transfer event raised by the SimpleERC20 contract.
type SimpleERC20Transfer struct {
	From  common.Address
	To    common.Address
	Value *big.Int
	Raw   types.Log // Blockchain specific contextual infos
}

// FilterTransfer is a free log retrieval operation binding the contract event 0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef.
//
// Solidity: event Transfer(address indexed _from, address indexed _to, uint256 _value)
func (_SimpleERC20 *SimpleERC20Filterer) FilterTransfer(opts *bind.FilterOpts, _from []common.Address, _to []common.Address) (*SimpleERC20TransferIterator, error) {

	var _fromRule []interface{}
	for _, _fromItem := range _from {
		_fromRule = append(_fromRule, _fromItem)
	}
	var _toRule []interface{}
	for _, _toItem := range _to {
		_toRule = append(_toRule, _toItem)
	}

	logs, sub, err := _SimpleERC20.contract.FilterLogs(opts, "Transfer", _fromRule, _toRule)
	if err != nil {
		return nil, err
	}
	return &SimpleERC20TransferIterator{contract: _SimpleERC20.contract, event: "Transfer", logs: logs, sub: sub}, nil
}

// WatchTransfer is a free log subscription operation binding the contract event 0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef.
//
// Solidity: event Transfer(address indexed _from, address indexed _to, uint256 _value)
func (_SimpleERC20 *SimpleERC20Filterer) WatchTransfer(opts *bind.WatchOpts, sink chan<- *SimpleERC20Transfer, _from []common.Address, _to []common.Address) (event.Subscription, error) {

	var _fromRule []interface{}
	for _, _fromItem := range _from {
		_fromRule = append(_fromRule, _fromItem)
	}
	var _toRule []interface{}
	for _, _toItem := range _to {
		_toRule = append(_toRule, _toItem)
	}

	logs, sub, err := _SimpleERC20.contract.WatchLogs(opts, "Transfer", _fromRule, _toRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(SimpleERC20Transfer)
				if err := _SimpleERC20.contract.UnpackLog(event, "Transfer", log); err != nil {
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

// ParseTransfer is a log parse operation binding the contract event 0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef.
//
// Solidity: event Transfer(address indexed _from, address indexed _to, uint256 _value)
func (_SimpleERC20 *SimpleERC20Filterer) ParseTransfer(log types.Log) (*SimpleERC20Transfer, error) {
	event := new(SimpleERC20Transfer)
	if err := _SimpleERC20.contract.UnpackLog(event, "Transfer", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}
