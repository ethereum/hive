package bellatrix

import (
	"fmt"
	"math/big"

	el_common "github.com/ethereum/go-ethereum/common"
	api "github.com/ethereum/go-ethereum/beacon/engine"
	"github.com/ethereum/hive/simulators/eth2/common/builder/types/common"
	blsu "github.com/protolambda/bls12-381-util"
	"github.com/protolambda/zrnt/eth2/beacon/bellatrix"
	beacon "github.com/protolambda/zrnt/eth2/beacon/common"
	"github.com/protolambda/ztyp/tree"
	"github.com/protolambda/ztyp/view"
)

type SignedBeaconBlock bellatrix.SignedBeaconBlock

func (s *SignedBeaconBlock) ExecutionPayloadHash() el_common.Hash {
	var hash el_common.Hash
	copy(hash[:], s.Message.Body.ExecutionPayload.BlockHash[:])
	return hash
}

func (s *SignedBeaconBlock) Root(spec *beacon.Spec) tree.Root {
	return s.Message.HashTreeRoot(spec, tree.GetHashFn())
}

func (s *SignedBeaconBlock) StateRoot() tree.Root {
	return s.Message.StateRoot
}

func (s *SignedBeaconBlock) Slot() beacon.Slot {
	return s.Message.Slot
}

func (s *SignedBeaconBlock) SetExecutionPayload(
	ep common.ExecutionPayload,
) error {
	s.Message.Body.ExecutionPayload.ParentHash = ep.GetParentHash()
	s.Message.Body.ExecutionPayload.FeeRecipient = ep.GetFeeRecipient()
	s.Message.Body.ExecutionPayload.StateRoot = ep.GetStateRoot()
	s.Message.Body.ExecutionPayload.ReceiptsRoot = ep.GetReceiptsRoot()
	s.Message.Body.ExecutionPayload.LogsBloom = ep.GetLogsBloom()
	s.Message.Body.ExecutionPayload.PrevRandao = ep.GetPrevRandao()
	s.Message.Body.ExecutionPayload.BlockNumber = ep.GetBlockNumber()
	s.Message.Body.ExecutionPayload.GasLimit = ep.GetGasLimit()
	s.Message.Body.ExecutionPayload.GasUsed = ep.GetGasUsed()
	s.Message.Body.ExecutionPayload.Timestamp = ep.GetTimestamp()
	s.Message.Body.ExecutionPayload.ExtraData = ep.GetExtraData()
	s.Message.Body.ExecutionPayload.BaseFeePerGas = ep.GetBaseFeePerGas()
	s.Message.Body.ExecutionPayload.BlockHash = ep.GetBlockHash()
	s.Message.Body.ExecutionPayload.Transactions = ep.GetTransactions()
	return nil
}

type BuilderBid struct {
	Header bellatrix.ExecutionPayloadHeader `json:"header" yaml:"header"`
	Value  view.Uint256View                 `json:"value"  yaml:"value"`
	PubKey beacon.BLSPubkey                 `json:"pubkey" yaml:"pubkey"`
}

func (b *BuilderBid) HashTreeRoot(hFn tree.HashFn) tree.Root {
	return hFn.HashTreeRoot(
		&b.Header,
		&b.Value,
		&b.PubKey,
	)
}

func (b *BuilderBid) FromExecutableData(
	spec *beacon.Spec,
	ed *api.ExecutableData,
) error {
	if ed == nil {
		return fmt.Errorf("nil execution payload")
	}
	if ed.Withdrawals != nil {
		return fmt.Errorf("execution data contains withdrawals")
	}
	copy(b.Header.ParentHash[:], ed.ParentHash[:])
	copy(b.Header.FeeRecipient[:], ed.FeeRecipient[:])
	copy(b.Header.StateRoot[:], ed.StateRoot[:])
	copy(b.Header.ReceiptsRoot[:], ed.ReceiptsRoot[:])
	copy(b.Header.LogsBloom[:], ed.LogsBloom[:])
	copy(b.Header.PrevRandao[:], ed.Random[:])

	b.Header.BlockNumber = view.Uint64View(ed.Number)
	b.Header.GasLimit = view.Uint64View(ed.GasLimit)
	b.Header.GasUsed = view.Uint64View(ed.GasUsed)
	b.Header.Timestamp = beacon.Timestamp(ed.Timestamp)

	b.Header.ExtraData = make(beacon.ExtraData, len(ed.ExtraData))
	copy(b.Header.ExtraData[:], ed.ExtraData[:])
	b.Header.BaseFeePerGas.SetFromBig(ed.BaseFeePerGas)
	copy(b.Header.BlockHash[:], ed.BlockHash[:])

	txs := make(beacon.PayloadTransactions, len(ed.Transactions))
	for i, tx := range ed.Transactions {
		txs[i] = make(beacon.Transaction, len(tx))
		copy(txs[i][:], tx[:])
	}
	txRoot := txs.HashTreeRoot(spec, tree.GetHashFn())
	copy(b.Header.TransactionsRoot[:], txRoot[:])

	return nil
}

func (b *BuilderBid) SetValue(value *big.Int) {
	b.Value.SetFromBig(value)
}

func (b *BuilderBid) SetPubKey(pk beacon.BLSPubkey) {
	b.PubKey = pk
}

func (b *BuilderBid) Sign(
	domain beacon.BLSDomain,
	sk *blsu.SecretKey,
	pk *blsu.Pubkey,
) (*common.SignedBuilderBid, error) {
	pkBytes := pk.Serialize()
	copy(b.PubKey[:], pkBytes[:])
	sigRoot := beacon.ComputeSigningRoot(
		b.HashTreeRoot(tree.GetHashFn()),
		domain,
	)
	return &common.SignedBuilderBid{
		Message:   b,
		Signature: beacon.BLSSignature(blsu.Sign(sk, sigRoot[:]).Serialize()),
	}, nil
}

type ExecutionPayload bellatrix.ExecutionPayload

func (p *ExecutionPayload) FromExecutableData(ed *api.ExecutableData) error {
	if ed == nil {
		return fmt.Errorf("nil execution payload")
	}
	if ed.Withdrawals != nil {
		return fmt.Errorf("execution data contains withdrawals")
	}
	copy(p.ParentHash[:], ed.ParentHash[:])
	copy(p.FeeRecipient[:], ed.FeeRecipient[:])
	copy(p.StateRoot[:], ed.StateRoot[:])
	copy(p.ReceiptsRoot[:], ed.ReceiptsRoot[:])
	copy(p.LogsBloom[:], ed.LogsBloom[:])
	copy(p.PrevRandao[:], ed.Random[:])

	p.BlockNumber = view.Uint64View(ed.Number)
	p.GasLimit = view.Uint64View(ed.GasLimit)
	p.GasUsed = view.Uint64View(ed.GasUsed)
	p.Timestamp = beacon.Timestamp(ed.Timestamp)

	p.ExtraData = make(beacon.ExtraData, len(ed.ExtraData))
	copy(p.ExtraData[:], ed.ExtraData[:])
	p.BaseFeePerGas.SetFromBig(ed.BaseFeePerGas)
	copy(p.BlockHash[:], ed.BlockHash[:])
	p.Transactions = make(beacon.PayloadTransactions, len(ed.Transactions))
	for i, tx := range ed.Transactions {
		p.Transactions[i] = make(beacon.Transaction, len(tx))
		copy(p.Transactions[i][:], tx[:])
	}
	return nil
}

func (p *ExecutionPayload) GetParentHash() beacon.Hash32 {
	return p.ParentHash
}

func (p *ExecutionPayload) GetFeeRecipient() beacon.Eth1Address {
	return p.FeeRecipient
}

func (p *ExecutionPayload) GetStateRoot() beacon.Bytes32 {
	return p.StateRoot
}

func (p *ExecutionPayload) GetReceiptsRoot() beacon.Bytes32 {
	return p.ReceiptsRoot
}

func (p *ExecutionPayload) GetLogsBloom() beacon.LogsBloom {
	return p.LogsBloom
}

func (p *ExecutionPayload) GetPrevRandao() beacon.Bytes32 {
	return p.PrevRandao
}

func (p *ExecutionPayload) GetBlockNumber() view.Uint64View {
	return p.BlockNumber
}

func (p *ExecutionPayload) GetGasLimit() view.Uint64View {
	return p.GasLimit
}

func (p *ExecutionPayload) GetGasUsed() view.Uint64View {
	return p.GasUsed
}

func (p *ExecutionPayload) GetTimestamp() beacon.Timestamp {
	return p.Timestamp
}

func (p *ExecutionPayload) GetExtraData() beacon.ExtraData {
	return p.ExtraData
}

func (p *ExecutionPayload) GetBaseFeePerGas() view.Uint256View {
	return p.BaseFeePerGas
}

func (p *ExecutionPayload) GetBlockHash() beacon.Hash32 {
	return p.BlockHash
}

func (p *ExecutionPayload) GetTransactions() beacon.PayloadTransactions {
	return p.Transactions
}
