package helper

import (
	"fmt"

	"github.com/ethereum/go-ethereum/core/types"
)

func GenerateInvalidPayloadBlock(baseBlock *types.Block, uncle *types.Block, payloadField InvalidPayloadBlockField) (*types.Block, error) {
	if payloadField == InvalidOmmers {
		if uncle == nil {
			return nil, fmt.Errorf("No ommer provided")
		}
		body := baseBlock.Body()
		body.Uncles = []*types.Header{uncle.Header()}
		header := baseBlock.Header()
		header.UncleHash = types.CalcUncleHash(body.Uncles)
		modifiedBlock := types.NewBlockWithHeader(header).WithBody(*body)
		fmt.Printf("DEBUG: hash=%s, ommerLen=%d, ommersHash=%v\n", modifiedBlock.Hash(), len(modifiedBlock.Uncles()), modifiedBlock.UncleHash())
		return modifiedBlock, nil
	}

	return baseBlock, nil
}
