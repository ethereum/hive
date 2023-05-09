package libgno

import (
	_ "embed"
	"fmt"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"math/big"
	"strings"
)

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
