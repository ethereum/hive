package main

import (
	"crypto/sha256"
	"encoding/binary"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/params"
)

type blockModifier interface {
	apply(*genBlockContext) bool
	txInfo() any
}

var modRegistry = make(map[string]func() blockModifier)

// register adds a block modifier.
func register(name string, new func() blockModifier) {
	modRegistry[name] = new
}

type genBlockContext struct {
	index   int
	block   *core.BlockGen
	gen     *generator
	txcount int
}

// Number returns the block number.
func (ctx *genBlockContext) Number() *big.Int {
	return ctx.block.Number()
}

// NumberU64 returns the block number.
func (ctx *genBlockContext) NumberU64() uint64 {
	return ctx.block.Number().Uint64()
}

// Timestamp returns the block timestamp.
func (ctx *genBlockContext) Timestamp() uint64 {
	return ctx.block.Timestamp()
}

// HasGas reports whether the block still has more than the given amount of gas left.
func (ctx *genBlockContext) HasGas(gas uint64) bool {
	return ctx.block.Gas() > gas
}

// AddNewTx adds a transaction into the block.
func (ctx *genBlockContext) AddNewTx(sender *genAccount, data types.TxData) *types.Transaction {
	tx, err := types.SignNewTx(sender.key, ctx.Signer(), data)
	if err != nil {
		panic(err)
	}
	ctx.block.AddTx(tx.WithoutBlobTxSidecar())
	ctx.txcount++
	return tx
}

// TxSenderAccount chooses an account to send transactions from.
func (ctx *genBlockContext) TxSenderAccount() *genAccount {
	a := ctx.gen.accounts[0]
	return &a
}

// TxCreateIntrinsicGas gives the 'intrinsic gas' of a contract creation transaction.
func (ctx *genBlockContext) TxCreateIntrinsicGas(data []byte) uint64 {
	genesis := ctx.gen.genesis
	isHomestead := genesis.Config.IsHomestead(ctx.block.Number())
	isEIP2028 := genesis.Config.IsIstanbul(ctx.block.Number())
	isEIP3860 := genesis.Config.IsShanghai(ctx.block.Number(), ctx.block.Timestamp())
	igas, err := core.IntrinsicGas(data, nil, nil, true, isHomestead, isEIP2028, isEIP3860)
	if err != nil {
		panic(err)
	}
	return igas
}

// TxGasFeeCap returns the minimum gasprice that should be used for transactions.
func (ctx *genBlockContext) TxGasFeeCap() *big.Int {
	fee := big.NewInt(1)
	if !ctx.ChainConfig().IsLondon(ctx.block.Number()) {
		return fee
	}
	return fee.Add(fee, ctx.block.BaseFee())
}

// AccountNonce returns the current nonce of an address.
func (ctx *genBlockContext) AccountNonce(addr common.Address) uint64 {
	return ctx.block.TxNonce(addr)
}

// Signer returns a signer for the current block.
func (ctx *genBlockContext) Signer() types.Signer {
	return ctx.block.Signer()
}

// TxCount returns the number of transactions added so far.
func (ctx *genBlockContext) TxCount() int {
	return ctx.txcount
}

// ChainConfig returns the chain config.
func (ctx *genBlockContext) ChainConfig() *params.ChainConfig {
	return ctx.gen.genesis.Config
}

// ParentBlock returns the parent of the current block.
func (ctx *genBlockContext) ParentBlock() *types.Block {
	return ctx.block.PrevBlock(ctx.index - 1)
}

// TxRandomValue returns a random value that depends on the block number and current transaction index.
func (ctx *genBlockContext) TxRandomValue() uint64 {
	var txindex [8]byte
	binary.BigEndian.PutUint64(txindex[:], uint64(ctx.TxCount()))
	h := sha256.New()
	h.Write(ctx.Number().Bytes())
	h.Write(txindex[:])
	return binary.BigEndian.Uint64(h.Sum(nil))
}
