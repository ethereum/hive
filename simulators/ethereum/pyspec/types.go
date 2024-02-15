package main

import (
	"context"
	"errors"
	"fmt"
	"math/big"

	api "github.com/ethereum/go-ethereum/beacon/engine"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/common/math"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/ethereum/go-ethereum/tests"

	"github.com/ethereum/hive/simulators/ethereum/engine/client"
	typ "github.com/ethereum/hive/simulators/ethereum/engine/types"
)

type Fail interface {
	Fatalf(format string, args ...interface{})
}

type TestCase struct {
	// test meta data
	Name       string
	FilePath   string
	ClientType string
	FailedErr  error
	// test fixture data
	*Fixture
	FailCallback Fail
}

func (tc *TestCase) Fatalf(format string, args ...interface{}) {
	tc.FailedErr = fmt.Errorf(format, args...)
	tc.FailCallback.Fatalf(format, args...)
}

type Fixture struct {
	Fork              string              `json:"network"`
	GenesisBlock      genesisBlock        `json:"genesisBlockHeader"`
	EngineNewPayloads []*EngineNewPayload `json:"engineNewPayloads"`
	EngineFcuVersion  int                 `json:"engineFcuVersion,string"`
	Pre               core.GenesisAlloc   `json:"pre"`
	PostAlloc         core.GenesisAlloc   `json:"postState"`
	SyncPayload       *EngineNewPayload   `json:"syncPayload"`
}

func (f *Fixture) Genesis() *core.Genesis {
	return &core.Genesis{
		Config:        tests.Forks[f.Fork],
		Coinbase:      f.GenesisBlock.Coinbase,
		Difficulty:    f.GenesisBlock.Difficulty,
		GasLimit:      f.GenesisBlock.GasLimit,
		Timestamp:     f.GenesisBlock.Timestamp.Uint64(),
		ExtraData:     f.GenesisBlock.ExtraData,
		Mixhash:       f.GenesisBlock.MixHash,
		Nonce:         f.GenesisBlock.Nonce.Uint64(),
		BaseFee:       f.GenesisBlock.BaseFee,
		BlobGasUsed:   f.GenesisBlock.BlobGasUsed,
		ExcessBlobGas: f.GenesisBlock.ExcessBlobGas,
		Alloc:         f.Pre,
	}
}

func (f *Fixture) ValidatePost(ctx context.Context, engineClient client.EngineClient) error {
	// check nonce, balance & storage of accounts in final block against fixture values
	for address, account := range f.PostAlloc {
		// get nonce & balance from last block (end of test execution)
		gotNonce, errN := engineClient.NonceAt(ctx, address, nil)
		gotBalance, errB := engineClient.BalanceAt(ctx, address, nil)
		if errN != nil {
			return fmt.Errorf("unable to call nonce from account: %v: %v", address, errN)
		} else if errB != nil {
			return fmt.Errorf("unable to call balance from account: %v: %v", address, errB)
		}
		// check final nonce & balance matches expected in fixture
		if account.Nonce != gotNonce {
			return fmt.Errorf(`nonce received from account %v doesn't match expected from fixture:
			received from block: %v
			expected in fixture: %v`, address, gotNonce, account.Nonce)
		}
		if account.Balance.Cmp(gotBalance) != 0 {
			return fmt.Errorf(`balance received from account %v doesn't match expected from fixture:
			received from block: %v
			expected in fixture: %v`, address, gotBalance, account.Balance)
		}
		// check final storage
		if len(account.Storage) > 0 {
			// extract fixture storage keys
			keys := make([]common.Hash, 0, len(account.Storage))
			for key := range account.Storage {
				keys = append(keys, key)
			}
			// get storage values for account with keys: keys
			gotStorage, errS := engineClient.StorageAtKeys(ctx, address, keys, nil)
			if errS != nil {
				return fmt.Errorf("unable to get storage values from account: %v: %v", address, errS)
			}
			// check values in storage match with fixture
			for _, key := range keys {
				if account.Storage[key] != *gotStorage[key] {
					return fmt.Errorf(`storage received from account %v doesn't match expected from fixture:
					received from block:  %v
					expected in fixture:  %v`, address, gotStorage[key], account.Storage[key])
				}
			}
		}
	}
	return nil
}

//go:generate go run github.com/fjl/gencodec -type genesisBlock -field-override genesisBlockUnmarshaling -out gen_gb.go
type genesisBlock struct {
	Coinbase      common.Address   `json:"coinbase"`
	Difficulty    *big.Int         `json:"difficulty"`
	GasLimit      uint64           `json:"gasLimit"`
	Timestamp     *big.Int         `json:"timestamp"`
	ExtraData     []byte           `json:"extraData"`
	MixHash       common.Hash      `json:"mixHash"`
	Nonce         types.BlockNonce `json:"nonce"`
	BaseFee       *big.Int         `json:"baseFeePerGas"`
	BlobGasUsed   *uint64          `json:"blobGasUsed"`
	ExcessBlobGas *uint64          `json:"excessBlobGas"`

	Hash common.Hash `json:"hash"`
}

type genesisBlockUnmarshaling struct {
	Difficulty    *math.HexOrDecimal256 `json:"difficulty"`
	GasLimit      math.HexOrDecimal64   `json:"gasLimit"`
	Timestamp     *math.HexOrDecimal256 `json:"timestamp"`
	ExtraData     hexutil.Bytes         `json:"extraData"`
	BaseFee       *math.HexOrDecimal256 `json:"baseFeePerGas"`
	BlobGasUsed   *math.HexOrDecimal64  `json:"dataGasUsed"`
	ExcessBlobGas *math.HexOrDecimal64  `json:"excessDataGas"`
}

type EngineNewPayload struct {
	ExecutionPayload      *api.ExecutableData `json:"executionPayload"`
	BlobVersionedHashes   []common.Hash       `json:"expectedBlobVersionedHashes"`
	ParentBeaconBlockRoot *common.Hash        `json:"parentBeaconBlockRoot"`
	Version               math.HexOrDecimal64 `json:"version"`
	ValidationError       *string             `json:"validationError"`
	ErrorCode             int                 `json:"errorCode,string"`
}

func (p *EngineNewPayload) ExecutableData() (*typ.ExecutableData, error) {
	executableData, err := typ.FromBeaconExecutableData(p.ExecutionPayload)
	if err != nil {
		return nil, errors.New("executionPayload param within engineNewPayload is invalid")
	}
	executableData.VersionedHashes = &p.BlobVersionedHashes
	executableData.ParentBeaconBlockRoot = p.ParentBeaconBlockRoot
	return &executableData, nil
}

func (p *EngineNewPayload) Valid() bool {
	return p.ErrorCode == 0 && p.ValidationError == nil
}

func (p *EngineNewPayload) ExpectedStatus() string {
	if p.ValidationError != nil {
		return "INVALID"
	}
	return "VALID"
}

func (p *EngineNewPayload) Execute(ctx context.Context, engineClient client.EngineClient) (api.PayloadStatusV1, rpc.Error) {
	executableData, err := p.ExecutableData()
	if err != nil {
		panic(err)
	}
	status, err := engineClient.NewPayload(
		ctx,
		int(p.Version),
		executableData,
	)
	return status, parseError(err)
}

func (p *EngineNewPayload) ExecuteValidate(ctx context.Context, engineClient client.EngineClient) (bool, error) {
	plStatus, plErr := p.Execute(ctx, engineClient)
	if err := p.ValidateRPCError(plErr); err != nil {
		return false, err
	} else if plErr != nil {
		// Got an expected error and is already validated in ValidateRPCError
		return false, nil
	}
	if plStatus.Status == "SYNCING" {
		return true, nil
	}
	// Check payload status matches expected
	if plStatus.Status != p.ExpectedStatus() {
		return false, fmt.Errorf("payload status mismatch: got %s, want %s", plStatus.Status, p.ExpectedStatus())
	}
	return false, nil
}

func (p *EngineNewPayload) ForkchoiceValidate(ctx context.Context, engineClient client.EngineClient, fcuVersion int) (bool, error) {
	response, err := engineClient.ForkchoiceUpdated(ctx, fcuVersion, &api.ForkchoiceStateV1{HeadBlockHash: p.ExecutionPayload.BlockHash}, nil)
	if err != nil {
		return false, err
	}
	if response.PayloadStatus.Status == "SYNCING" {
		return true, nil
	}
	if response.PayloadStatus.Status != p.ExpectedStatus() {
		return false, fmt.Errorf("forkchoice update status mismatch: got %s, want %s", response.PayloadStatus.Status, p.ExpectedStatus())
	}
	return false, nil
}

type HTTPErrorWithCode struct {
	rpc.HTTPError
}

func (e HTTPErrorWithCode) ErrorCode() int {
	return e.StatusCode
}

func parseError(plErr interface{}) rpc.Error {
	if plErr == nil {
		return nil
	}
	rpcErr, isRpcErr := plErr.(rpc.Error)
	if isRpcErr {
		return rpcErr
	}
	httpErr, isHttpErr := plErr.(rpc.HTTPError)
	if isHttpErr {
		return HTTPErrorWithCode{httpErr}
	}
	panic("unable to parse")
}

// checks for RPC errors and compares error codes if expected.
func (p *EngineNewPayload) ValidateRPCError(rpcErr rpc.Error) error {
	if rpcErr == nil && p.ErrorCode == 0 {
		return nil
	}
	if rpcErr == nil && p.ErrorCode != 0 {
		return fmt.Errorf("expected error code %d but received no error", p.ErrorCode)
	}
	if rpcErr != nil && p.ErrorCode == 0 {
		return fmt.Errorf("expected no error code but received %d", rpcErr.ErrorCode())
	}
	if rpcErr != nil && p.ErrorCode != 0 {
		plErrCode := rpcErr.ErrorCode()
		if plErrCode != p.ErrorCode {
			return fmt.Errorf("error code mismatch: got: %d, want: %d", plErrCode, p.ErrorCode)
		}
	}
	return nil
}
