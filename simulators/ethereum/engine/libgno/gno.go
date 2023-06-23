package libgno

import (
	_ "embed"
	"fmt"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"math/big"
	"strings"
)

// MAX_FAILED_WITHDRAWALS_TO_PROCESS represents the maximum number of failed withdrawals to process.
const MAX_FAILED_WITHDRAWALS_TO_PROCESS = 4

const GAS_LIMIT = 1000000

// SYSTEM_SENDER represents the address of the system sender.
// var SYSTEM_SENDER = common.HexToAddress("0xfffffffffffffffffffffffffffffffffffffffe")
var GNOTokenAddress = common.HexToAddress("0xbabe2bed00000000000000000000000000000002")
var WithdrawalsContractAddress = common.HexToAddress("0xbabe2bed00000000000000000000000000000003")

// GNOWithdrawalContractABI represents the path to the GNO withdrawal contract ABI.
//
//go:embed withdrawals.json
var GNOWithdrawalContractABI string

// GNOTokenContractABI represents the path to the GNO token contract ABI.
//
//go:embed sbctoken.json
var GNOTokenContractABI string

var ErrorAmountAndAddressDifferentLength = fmt.Errorf("amount and addresses must be the same length")
var ErrorLoadingWithdrawalContract = fmt.Errorf("error loading withdrawal contract")
var ErrorLoadingGNOTokenContract = fmt.Errorf("error loading gno token contract")
var ErrorPackingArguments = fmt.Errorf("error packing arguments")

// ExecuteSystemWithdrawal gets the byte code to execute a system withdrawal.
func ExecuteSystemWithdrawal(maxNumberOfFailedWithdrawalsToProcess uint64, amount []uint64, addresses []common.Address) ([]byte, error) {
	if len(amount) != len(addresses) {
		return []byte{}, ErrorAmountAndAddressDifferentLength
	}
	withdrawalABI, err := abi.JSON(strings.NewReader(GNOWithdrawalContractABI))
	if err != nil {
		return []byte{}, ErrorLoadingWithdrawalContract
	}
	dataBytes, err := withdrawalABI.Pack("executeSystemWithdrawals", big.NewInt(int64(maxNumberOfFailedWithdrawalsToProcess)), amount, addresses)
	if err != nil {
		return []byte{}, fmt.Errorf("%w: %w", ErrorPackingArguments, err)
	}
	// if at some point we want to convert it to hex, use something like this: hex.EncodeToString(dataBytes)
	return dataBytes, nil
}

// ExecuteWithdrawalsClaims gets the byte code to execute a withdrawals claims.
func ExecuteWithdrawalsClaims(addresses []common.Address) ([]byte, error) {
	withdrawalABI, err := abi.JSON(strings.NewReader(GNOWithdrawalContractABI))
	if err != nil {
		return []byte{}, ErrorLoadingWithdrawalContract
	}
	dataBytes, err := withdrawalABI.Pack("claimWithdrawals", addresses)
	if err != nil {
		return []byte{}, fmt.Errorf("%w: %w", ErrorPackingArguments, err)
	}
	// if at some point we want to convert it to hex, use something like this: hex.EncodeToString(dataBytes)
	return dataBytes, nil
}

// BalanceOfAddressData return contract method to get the balance of a GNO token.
func BalanceOfAddressData(account common.Address) ([]byte, error) {
	gnoTokenABI, err := abi.JSON(strings.NewReader(GNOTokenContractABI))
	if err != nil {
		return []byte{}, ErrorLoadingGNOTokenContract
	}
	dataBytes, err := gnoTokenABI.Pack("balanceOf", account)
	if err != nil {
		return []byte{}, fmt.Errorf("%w: %w", ErrorPackingArguments, err)
	}
	return dataBytes, nil
}

// GetGNOTokenABI return the GNO token ABI.
func GetGNOTokenABI() (*abi.ABI, error) {
	gnoTokenABI, err := abi.JSON(strings.NewReader(GNOTokenContractABI))
	if err != nil {
		return nil, ErrorLoadingGNOTokenContract
	}
	return &gnoTokenABI, nil
}

// GetWithdrawalsABI return the Withdrawals contract ABI.
func GetWithdrawalsABI() (*abi.ABI, error) {
	withdrawalsABI, err := abi.JSON(strings.NewReader(GNOWithdrawalContractABI))
	if err != nil {
		return nil, ErrorLoadingWithdrawalContract
	}
	return &withdrawalsABI, nil
}
