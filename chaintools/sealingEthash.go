package chaintools

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus"
	"github.com/ethereum/go-ethereum/consensus/ethash"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rpc"
)

// SealingEthash is intended for use with chain_makers GenerateChain
// to produce blocks with a valid proof of work
type SealingEthash struct {
	EthashEngine *ethash.Ethash
}

// NewSealingEthash : ctor
func NewSealingEthash(config ethash.Config, notify []string, noverify bool) *SealingEthash {
	sealingEthash := &SealingEthash{
		EthashEngine: ethash.New(config, notify, noverify),
	}
	return sealingEthash
}

// Finalize wraps ethhash engine
func (e *SealingEthash) Finalize(chain consensus.ChainReader, header *types.Header, state *state.StateDB, txs []*types.Transaction, uncles []*types.Header) {
	e.EthashEngine.Finalize(chain, header, state, txs, uncles)
}

// FinalizeAndAssemble implements consensus.Engine, accumulating the block and uncle rewards,
// setting the final state and assembling the block.
func (e *SealingEthash) FinalizeAndAssemble(chain consensus.ChainReader, header *types.Header, state *state.StateDB, txs []*types.Transaction, uncles []*types.Header, receipts []*types.Receipt) (*types.Block, error) {

	finalizedBlock, err := e.EthashEngine.FinalizeAndAssemble(chain, header, state, txs, uncles, receipts)
	if err != nil {
		return nil, err
	}

	results := make(chan *types.Block)
	e.EthashEngine.Seal(nil, finalizedBlock, results, nil)

	// Header seems complete, assemble into a block and return
	return <-results, nil
}

// Author retrieves the Ethereum address of the account that minted the given
// block, which may be different from the header's coinbase if a consensus
// engine is based on signatures.
func (e *SealingEthash) Author(header *types.Header) (common.Address, error) {
	return e.EthashEngine.Author(header)
}

// VerifyHeader checks whether a header conforms to the consensus rules of a
// given engine. Verifying the seal may be done optionally here, or explicitly
// via the VerifySeal method.
func (e *SealingEthash) VerifyHeader(chain consensus.ChainReader, header *types.Header, seal bool) error {
	return e.EthashEngine.VerifyHeader(chain, header, seal)
}

// VerifyHeaders is similar to VerifyHeader, but verifies a batch of headers
// concurrently. The method returns a quit channel to abort the operations and
// a results channel to retrieve the async verifications (the order is that of
// the input slice).
func (e *SealingEthash) VerifyHeaders(chain consensus.ChainReader, headers []*types.Header, seals []bool) (chan<- struct{}, <-chan error) {
	return e.EthashEngine.VerifyHeaders(chain, headers, seals)
}

// VerifyUncles verifies that the given block's uncles conform to the consensus
// rules of a given engine.
func (e *SealingEthash) VerifyUncles(chain consensus.ChainReader, block *types.Block) error {
	return e.EthashEngine.VerifyUncles(chain, block)
}

// VerifySeal checks whether the crypto seal on a header is valid according to
// the consensus rules of the given engine.
func (e *SealingEthash) VerifySeal(chain consensus.ChainReader, header *types.Header) error {
	return e.EthashEngine.VerifySeal(chain, header)
}

// Prepare initializes the consensus fields of a block header according to the
// rules of a particular engine. The changes are executed inline.
func (e *SealingEthash) Prepare(chain consensus.ChainReader, header *types.Header) error {
	return e.EthashEngine.Prepare(chain, header)
}

// Seal generates a new sealing request for the given input block and pushes
// the result into the given channel.
//
// Note, the method returns immediately and will send the result async. More
// than one result may also be returned depending on the consensus algorithm.
func (e *SealingEthash) Seal(chain consensus.ChainReader, block *types.Block, results chan<- *types.Block, stop <-chan struct{}) error {
	panic("SealingEthash returns a sealed block from Finalize. Seal is not supported.")
}

// SealHash returns the hash of a block prior to it being sealed.
func (e *SealingEthash) SealHash(header *types.Header) common.Hash {
	return e.EthashEngine.SealHash(header)
}

// CalcDifficulty is the difficulty adjustment algorithm. It returns the difficulty
// that a new block should have.
func (e *SealingEthash) CalcDifficulty(chain consensus.ChainReader, time uint64, parent *types.Header) *big.Int {
	return e.EthashEngine.CalcDifficulty(chain, time, parent)
}

// APIs returns the RPC APIs this consensus engine provides.
func (e *SealingEthash) APIs(chain consensus.ChainReader) []rpc.API {
	return e.EthashEngine.APIs(chain)
}

// Close terminates any background threads maintained by the consensus engine.
func (e *SealingEthash) Close() error {
	return e.EthashEngine.Close()
}
