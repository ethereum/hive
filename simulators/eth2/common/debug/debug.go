package debug

import (
	"context"
	"fmt"
	"sort"

	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/hive/simulators/eth2/common/clients"
	"github.com/ethereum/hive/simulators/eth2/common/utils"
	"github.com/protolambda/eth2api"
	"github.com/protolambda/zrnt/eth2/beacon/common"
	"github.com/protolambda/ztyp/tree"
)

// Helper debugging functions
func PrintAllBeaconBlocks(
	parentCtx context.Context,
	l utils.Logging,
	b *clients.BeaconClient,
) error {
	headInfo, err := b.BlockHeader(parentCtx, eth2api.BlockHead)

	if err != nil {
		return fmt.Errorf("PrintAllBeaconBlocks: failed to poll head: %v", err)
	}
	l.Logf(
		"PrintAllBeaconBlocks: Printing beacon chain from %s\n",
		b.ClientType(),
	)
	l.Logf(
		"PrintAllBeaconBlocks: Head, slot %d, root %v\n",
		headInfo.Header.Message.Slot,
		headInfo.Root,
	)
	for i := 1; i <= int(headInfo.Header.Message.Slot); i++ {
		bHeader, err := b.BlockHeader(parentCtx, eth2api.BlockIdSlot(i))
		if err != nil {
			l.Logf("PrintAllBeaconBlocks: Slot %d, not found\n", i)
			continue
		}
		var (
			root      = bHeader.Root
			execution = "0x0000..0000"
		)

		if versionedBlock, err := b.BlockV2(parentCtx, eth2api.BlockIdRoot(root)); err == nil {
			if executionPayload, err := versionedBlock.ExecutionPayload(); err == nil {
				execution = utils.Shorten(executionPayload.BlockHash.Hex())
			}
		}

		l.Logf(
			"PrintAllBeaconBlocks: Slot=%d, root=%v, exec=%s\n",
			i,
			root,
			execution,
		)
	}
	return nil
}

type BeaconBlockInfo struct {
	Root      tree.Root
	Parent    tree.Root
	Execution ethcommon.Hash
	Nodes     []int
}

type BeaconBlockList []*BeaconBlockInfo

func (bl BeaconBlockList) Add(
	root tree.Root,
	parent tree.Root,
	execution ethcommon.Hash,
	nodeId int,
) (BeaconBlockList, error) {
	for _, b := range bl {
		if root == b.Root {
			if parent != b.Parent {
				return bl, fmt.Errorf(
					"roots equal (%s), parent root mismatch: %s != %s",
					root.String(),
					parent.String(),
					b.Parent.String(),
				)
			}
			if execution != b.Execution {
				return bl, fmt.Errorf(
					"roots equal (%s), exec hash mismatch: %s != %s",
					root.String(),
					execution.String(),
					b.Execution.String(),
				)
			}
			for _, n := range b.Nodes {
				if nodeId == n {
					return bl, nil
				}
			}
			b.Nodes = append(b.Nodes, nodeId)
			return bl, nil
		}
	}
	return append(bl, &BeaconBlockInfo{
		Root:      root,
		Parent:    parent,
		Execution: execution,
		Nodes:     []int{nodeId},
	}), nil
}

type BeaconBlockMap map[common.Slot]BeaconBlockList

func (bm BeaconBlockMap) Add(
	slot common.Slot,
	root tree.Root,
	parent tree.Root,
	execution ethcommon.Hash,
	nodeId int,
) error {
	if _, found := bm[slot]; !found {
		bm[slot] = make(BeaconBlockList, 0)
	}
	var err error
	bm[slot], err = bm[slot].Add(root, parent, execution, nodeId)
	if err != nil {
		return fmt.Errorf("conflicting block on slot %d: %v", slot, err)
	}
	return nil
}
func (bm BeaconBlockMap) Print(l utils.Logging) error {
	slots := make([]int, 0, len(bm))
	for s := range bm {
		slots = append(slots, int(s))
	}
	sort.Ints(slots)
	for _, s := range slots {
		l.Logf("- Slot=%d\n", s)
		for i, b := range bm[common.Slot(s)] {
			l.Logf(
				"    Fork %d: root=%v, parent=%v, exec=%s, nodes=%v \n",
				i,
				utils.Shorten(b.Root.String()),
				utils.Shorten(b.Parent.String()),
				utils.Shorten(b.Execution.String()),
				b.Nodes,
			)
		}
	}
	return nil
}

func PrintAllTestnetBeaconBlocks(
	parentCtx context.Context,
	l utils.Logging,
	runningBeacons clients.BeaconClients,
) error {
	beaconTree := make(BeaconBlockMap)

	for nodeId, beaconNode := range runningBeacons {

		var (
			nextBlock eth2api.BlockId
		)

		nextBlock = eth2api.BlockHead

		for {
			if nextBlock.BlockId() == eth2api.BlockIdRoot(tree.Root{}).
				BlockId() {
				break
			}
			// Get block header
			bHeader, err := beaconNode.BlockHeader(parentCtx, nextBlock)
			if err != nil {
				l.Logf(
					"Error fetching block (%s) from beacon node %d: %v",
					nextBlock.BlockId(),
					nodeId,
					err,
				)
				break
			}

			var (
				root      = bHeader.Root
				parent    = bHeader.Header.Message.ParentRoot
				execution = ethcommon.Hash{}
			)

			if versionedBlock, err := beaconNode.BlockV2(parentCtx, eth2api.BlockIdRoot(root)); err == nil {
				if executionPayload, err := versionedBlock.ExecutionPayload(); err == nil {
					execution = executionPayload.BlockHash
					l.Logf(
						"Node %d: Execution payload: hash=%s",
						nodeId,
						utils.Shorten(execution.String()),
					)
				}
			} else if err != nil {
				l.Logf("Error getting versioned block=%s node=%d: %v", nextBlock.BlockId(), nodeId, err)
				break
			}
			if err := beaconTree.Add(bHeader.Header.Message.Slot, root, parent, execution, nodeId); err != nil {
				return err
			}
			nextBlock = eth2api.BlockIdRoot(parent)
		}

	}
	beaconTree.Print(l)
	return nil
}
