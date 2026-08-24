package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/types/bal"
)

// balOutputDir is the subdirectory of the output directory which receives the
// per-block EIP-7928 block access list dumps.
const balOutputDir = "bal"

// balDump is the JSON structure written for each Amsterdam block.
type balDump struct {
	Number    uint64      `json:"number"`
	Hash      common.Hash `json:"hash"`
	Timestamp uint64      `json:"timestamp"`
	// HeaderHash is the blockAccessListHash of the header.
	HeaderHash *common.Hash `json:"blockAccessListHash"`
	// ListHash is the hash of the access list in this file. It differs from
	// HeaderHash if the block was assembled with a different list.
	ListHash     *common.Hash         `json:"blockAccessListRootOfList"`
	Transactions []common.Hash        `json:"transactions"`
	AccessList   *bal.BlockAccessList `json:"blockAccessList"`
}

// writeBlockAccessLists writes the block access list of every Amsterdam block in the
// chain into the 'bal' subdirectory of the output directory, one file per block.
func (g *generator) writeBlockAccessLists() error {
	last := g.blockchain.CurrentBlock().Number.Uint64()
	blocks := make([]*types.Block, 0, last+1)
	for num := uint64(0); num <= last; num++ {
		blocks = append(blocks, g.blockchain.GetBlockByNumber(num))
	}
	return g.dumpBlockAccessLists(blocks)
}

// dumpBlockAccessLists writes one file per Amsterdam block of the given list. Unlike
// writeBlockAccessLists, it does not need an imported chain, which is what makes it
// usable for debugging access list mismatches reported by chain import.
func (g *generator) dumpBlockAccessLists(blocks []*types.Block) error {
	dir := filepath.Join(g.cfg.outputDir, balOutputDir)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	count := 0
	for _, b := range blocks {
		if b == nil || !g.genesis.Config.IsAmsterdam(b.Number(), b.Time()) {
			continue
		}
		if err := g.writeBlockAccessList(b); err != nil {
			return err
		}
		count++
	}
	fmt.Printf("wrote %d block access lists to %s\n", count, dir)
	return nil
}

// writeBlockAccessList writes the access list of a single block as JSON, plus a
// human-readable rendering of the same list next to it.
func (g *generator) writeBlockAccessList(b *types.Block) error {
	num := b.NumberU64()
	al := b.AccessList()
	dump := &balDump{
		Number:       num,
		Hash:         b.Hash(),
		Timestamp:    b.Time(),
		HeaderHash:   b.BlockAccessListHash(),
		Transactions: make([]common.Hash, 0, len(b.Transactions())),
		AccessList:   al,
	}
	for _, tx := range b.Transactions() {
		dump.Transactions = append(dump.Transactions, tx.Hash())
	}
	if al != nil {
		h := al.Hash()
		dump.ListHash = &h
	}

	name := fmt.Sprintf("block-%05d", num)
	if err := g.writeJSON(filepath.Join(balOutputDir, name+".json"), dump); err != nil {
		return err
	}
	out, err := g.openOutputFile(filepath.Join(balOutputDir, name+".txt"))
	if err != nil {
		return err
	}
	defer out.Close()
	fmt.Fprintf(out, "block %d (%s), timestamp %d\n", num, b.Hash(), b.Time())
	fmt.Fprintf(out, "header blockAccessListHash: %v\n", dump.HeaderHash)
	fmt.Fprintf(out, "hash of list below:         %v\n\n", dump.ListHash)
	if al == nil {
		fmt.Fprintln(out, "(block has no access list)")
		return nil
	}
	_, err = fmt.Fprint(out, al.PrettyPrint())
	return err
}
