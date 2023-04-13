package common

import (
	"math/big"

	api "github.com/ethereum/go-ethereum/beacon/engine"
	el_common "github.com/ethereum/go-ethereum/common"
	blsu "github.com/protolambda/bls12-381-util"
	"github.com/protolambda/zrnt/eth2/beacon/common"
	beacon "github.com/protolambda/zrnt/eth2/beacon/common"
	"github.com/protolambda/ztyp/tree"
	"github.com/protolambda/ztyp/view"
)

type ValidatorRegistrationV1 struct {
	FeeRecipient common.Eth1Address `json:"fee_recipient" yaml:"fee_recipient"`
	GasLimit     view.Uint64View    `json:"gas_limit"     yaml:"gas_limit"`
	Timestamp    view.Uint64View    `json:"timestamp"     yaml:"timestamp"`
	PubKey       common.BLSPubkey   `json:"pubkey"        yaml:"pubkey"`
}

func (vr *ValidatorRegistrationV1) HashTreeRoot(hFn tree.HashFn) tree.Root {
	return hFn.HashTreeRoot(
		&vr.FeeRecipient,
		&vr.GasLimit,
		&vr.Timestamp,
		&vr.PubKey,
	)
}

type SignedValidatorRegistrationV1 struct {
	Message   ValidatorRegistrationV1 `json:"message"   yaml:"message"`
	Signature common.BLSSignature     `json:"signature" yaml:"signature"`
}

type BuilderBid interface {
	FromExecutableData(*beacon.Spec, *api.ExecutableData) error
	SetValue(*big.Int)
	SetPubKey(beacon.BLSPubkey)
	Sign(domain beacon.BLSDomain,
		sk *blsu.SecretKey,
		pk *blsu.Pubkey) (*SignedBuilderBid, error)
}

type SignedBuilderBid struct {
	Message   BuilderBid          `json:"message"   yaml:"message"`
	Signature common.BLSSignature `json:"signature" yaml:"signature"`
}

func (s *SignedBuilderBid) Versioned(
	version string,
) *VersionedSignedBuilderBid {
	return &VersionedSignedBuilderBid{
		Version: version,
		Data:    s,
	}
}

type VersionedSignedBuilderBid struct {
	Version string            `json:"version" yaml:"version"`
	Data    *SignedBuilderBid `json:"data"    yaml:"data"`
}

type SignedBeaconBlock interface {
	ExecutionPayloadHash() el_common.Hash
	Root(*beacon.Spec) tree.Root
	StateRoot() tree.Root
	SetExecutionPayload(ExecutionPayload) error
	Slot() beacon.Slot
	ProposerIndex() beacon.ValidatorIndex
	BlockSignature() *common.BLSSignature
}

type ExecutionPayload interface {
	FromExecutableData(*api.ExecutableData) error
	GetParentHash() beacon.Hash32
	GetFeeRecipient() beacon.Eth1Address
	GetStateRoot() beacon.Bytes32
	GetReceiptsRoot() beacon.Bytes32
	GetLogsBloom() beacon.LogsBloom
	GetPrevRandao() beacon.Bytes32
	GetBlockNumber() view.Uint64View
	GetGasLimit() view.Uint64View
	GetGasUsed() view.Uint64View
	GetTimestamp() beacon.Timestamp
	GetExtraData() beacon.ExtraData
	GetBaseFeePerGas() view.Uint256View
	GetBlockHash() beacon.Hash32
	GetTransactions() beacon.PayloadTransactions
}

type ExecutionPayloadWithdrawals interface {
	ExecutionPayload
	GetWithdrawals() beacon.Withdrawals
}

type ExecutionPayloadResponse struct {
	Version string           `json:"version" yaml:"version"`
	Data    ExecutionPayload `json:"data"    yaml:"data"`
}
