package libgno

import (
	_ "embed"
	"fmt"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
)

const (
	// MAX_FAILED_WITHDRAWALS_TO_PROCESS represents the maximum number of failed withdrawals to process.
	MAX_FAILED_WITHDRAWALS_TO_PROCESS = 4
	GAS_LIMIT                         = 1000000
)

var (
	// SYSTEM_SENDER represents the address of the system sender.
	// var SYSTEM_SENDER = common.HexToAddress("0xfffffffffffffffffffffffffffffffffffffffe")
	GNOTokenAddress            = common.HexToAddress("0xbabe2bed00000000000000000000000000000002")
	WithdrawalsContractAddress = common.HexToAddress("0xbabe2bed00000000000000000000000000000003")

	// GNOWithdrawalContractABI represents the path to the GNO withdrawal contract ABI.
	//
	//go:embed withdrawals.json
	GNOWithdrawalContractABI string

	// GNOTokenContractABI represents the path to the GNO token contract ABI.
	//
	//go:embed sbctoken.json
	GNOTokenContractABI string

	ErrorAmountAndAddressDifferentLength = fmt.Errorf("amount and addresses must be the same length")
	ErrorLoadingWithdrawalContract       = fmt.Errorf("error loading withdrawal contract")
	ErrorLoadingGNOTokenContract         = fmt.Errorf("error loading gno token contract")
	ErrorPackingArguments                = fmt.Errorf("error packing arguments")
)

// ExecuteSystemWithdrawal gets the byte code to execute a system withdrawal.
func ExecuteSystemWithdrawal(maxNumberOfFailedWithdrawalsToProcess uint64, amount []uint64, addresses []common.Address) ([]byte, error) {
	if len(amount) != len(addresses) {
		return []byte{}, ErrorAmountAndAddressDifferentLength
	}
	withdrawalABI, err := GetWithdrawalsABI()
	if err != nil {
		return []byte{}, err
	}
	dataBytes, err := withdrawalABI.Pack("executeSystemWithdrawals", big.NewInt(int64(maxNumberOfFailedWithdrawalsToProcess)), amount, addresses)
	if err != nil {
		return []byte{}, fmt.Errorf("%w: %w", ErrorPackingArguments, err)
	}
	// if at some point we want to convert it to hex, use something like this: hex.EncodeToString(dataBytes)
	return dataBytes, nil
}

// ClaimWithdrawalsData return the Tx calldata for "claimWithdrawals" call.
func ClaimWithdrawalsData(addresses []common.Address) ([]byte, error) {
	withdrawalABI, err := GetWithdrawalsABI()
	if err != nil {
		return []byte{}, err
	}
	dataBytes, err := withdrawalABI.Pack("claimWithdrawals", addresses)
	if err != nil {
		return []byte{}, fmt.Errorf("%w: %w", ErrorPackingArguments, err)
	}
	// if at some point we want to convert it to hex, use something like this: hex.EncodeToString(dataBytes)
	return dataBytes, nil
}

// BalanceOfAddressData return the Tx calldata for "balanceOf" call.
func BalanceOfAddressData(account common.Address) ([]byte, error) {
	gnoTokenABI, err := GetGNOTokenABI()
	if err != nil {
		return []byte{}, err
	}
	dataBytes, err := gnoTokenABI.Pack("balanceOf", account)
	if err != nil {
		return []byte{}, fmt.Errorf("%w: %w", ErrorPackingArguments, err)
	}
	return dataBytes, nil
}

// TransferData return the Tx calldata for "Transfer" call.
func TransferData(recipient common.Address, amount *big.Int) ([]byte, error) {
	gnoTokenABI, err := GetGNOTokenABI()
	if err != nil {
		return []byte{}, err
	}
	dataBytes, err := gnoTokenABI.Pack("transfer", recipient, amount)
	if err != nil {
		return []byte{}, fmt.Errorf("%w: %w", ErrorPackingArguments, err)
	}
	return dataBytes, nil
}

// GetWithdrawalsTransferEvents get all transfer events where:
//
// `From` = deposit contract address
// `To` = all addresses from addresses argument
//
// for specific block range and returns map { withdrawalAddress: transferAmount }
func GetWithdrawalsTransferEvents(client *ethclient.Client, addresses []common.Address, fromBlock, toBlock uint64) (map[string]*big.Int, error) {
	transfersList := make(map[string]*big.Int, 0)
	token, err := NewGnoToken(GNOTokenAddress, client)
	if err != nil {
		return nil, fmt.Errorf("can't bind token contract: %w", err)
	}
	eventsIterator, err := token.FilterTransfer(
		&bind.FilterOpts{
			Start: fromBlock,
			End:   &toBlock,
		},
		[]common.Address{WithdrawalsContractAddress},
		addresses,
	)
	if err != nil {
		return nil, fmt.Errorf("can't filter transfers: %w", err)
	}

	for eventsIterator.Next() {
		addr := eventsIterator.Event.To.Hex()
		amount := eventsIterator.Event.Value

		transfersList[addr] = amount
	}
	return transfersList, nil
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
		return nil, fmt.Errorf("%w: %w", ErrorLoadingWithdrawalContract, err)
	}
	return &withdrawalsABI, nil
}
