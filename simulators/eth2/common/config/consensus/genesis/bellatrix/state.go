package bellatrix

import (
	"encoding/json"
	"fmt"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/hive/simulators/eth2/common/config/consensus/genesis/interfaces"
	"github.com/holiman/uint256"
	"github.com/protolambda/zrnt/eth2/beacon/bellatrix"
	"github.com/protolambda/zrnt/eth2/beacon/common"
	"github.com/protolambda/zrnt/eth2/configs"
	"github.com/protolambda/ztyp/tree"
	"github.com/protolambda/ztyp/view"
)

type GenesisStateView struct {
	*bellatrix.BeaconStateView
	Spec *common.Spec
}

func NewBeaconStateView(spec *common.Spec) interfaces.StateViewGenesis {
	return &GenesisStateView{
		BeaconStateView: bellatrix.NewBeaconStateView(spec),
		Spec:            spec,
	}
}

func (g *GenesisStateView) ToJson() ([]byte, error) {
	raw, err := g.BeaconStateView.Raw(g.Spec)
	if err != nil {
		return nil, err
	}
	return json.MarshalIndent(raw, "", "  ")
}

func (g *GenesisStateView) ForkVersion() common.Version {
	return g.Spec.BELLATRIX_FORK_VERSION
}

func (g *GenesisStateView) PreviousForkVersion() common.Version {
	return g.Spec.ALTAIR_FORK_VERSION
}

func (g *GenesisStateView) EmptyBodyRoot() common.Root {
	return bellatrix.BeaconBlockBodyType(configs.Mainnet).
		New().
		HashTreeRoot(tree.GetHashFn())
}

func (g *GenesisStateView) SetGenesisExecutionHeader(
	executionGenesis *types.Block,
) error {
	var execPayloadHeader *bellatrix.ExecutionPayloadHeader

	ttd := uint256.Int(g.Spec.TERMINAL_TOTAL_DIFFICULTY)

	if executionGenesis.Difficulty().Cmp(ttd.ToBig()) >= 0 {
		extra := executionGenesis.Extra()
		if len(extra) > common.MAX_EXTRA_DATA_BYTES {
			return fmt.Errorf(
				"extra data is %d bytes, max is %d",
				len(extra),
				common.MAX_EXTRA_DATA_BYTES,
			)
		}
		if len(executionGenesis.Transactions()) != 0 {
			return fmt.Errorf(
				"expected no transactions in genesis execution payload",
			)
		}

		baseFee, overflow := uint256.FromBig(executionGenesis.BaseFee())
		if overflow {
			return fmt.Errorf("basefee larger than 2^256-1")
		}

		execPayloadHeader = &bellatrix.ExecutionPayloadHeader{
			ParentHash:    common.Root(executionGenesis.ParentHash()),
			FeeRecipient:  common.Eth1Address(executionGenesis.Coinbase()),
			StateRoot:     common.Bytes32(executionGenesis.Root()),
			ReceiptsRoot:  common.Bytes32(executionGenesis.ReceiptHash()),
			LogsBloom:     common.LogsBloom(executionGenesis.Bloom()),
			PrevRandao:    common.Bytes32{},
			BlockNumber:   view.Uint64View(executionGenesis.NumberU64()),
			GasLimit:      view.Uint64View(executionGenesis.GasLimit()),
			GasUsed:       view.Uint64View(executionGenesis.GasUsed()),
			Timestamp:     common.Timestamp(executionGenesis.Time()),
			ExtraData:     extra,
			BaseFeePerGas: view.Uint256View(*baseFee),
			BlockHash:     common.Root(executionGenesis.Hash()),
			// empty transactions root
			TransactionsRoot: common.PayloadTransactionsType(g.Spec).
				DefaultNode().
				MerkleRoot(tree.GetHashFn()),
		}
	} else {
		execPayloadHeader = new(bellatrix.ExecutionPayloadHeader)
	}
	return g.SetLatestExecutionPayloadHeader(execPayloadHeader)
}
