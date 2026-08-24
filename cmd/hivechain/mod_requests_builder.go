package main

import (
	"bytes"
	"encoding/binary"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/params"
)

var builderRequestPubkey = bytes.Repeat([]byte{0x42}, 48)

func init() {
	register("tx-request-eip8282-deposit", func() blockModifier {
		return &modBuilderRequest{deposit: true}
	})
	register("tx-request-eip8282-exit", func() blockModifier {
		return &modBuilderRequest{}
	})
}

type modBuilderRequest struct {
	deposit bool
	info    *modBuilderRequestInfo
}

type modBuilderRequestInfo struct {
	TxHash common.Hash    `json:"txhash"`
	Block  hexutil.Uint64 `json:"block"`
}

func (m *modBuilderRequest) apply(ctx *genBlockContext) bool {
	if m.info != nil || !ctx.ChainConfig().IsAmsterdam(ctx.Number(), ctx.Timestamp()) {
		return false
	}

	const gas = 1_000_000
	if !ctx.HasGas(gas) {
		return false
	}

	var (
		to    common.Address
		input []byte
		value *big.Int
	)
	if m.deposit {
		// EIP-8282 deposit input is pubkey || withdrawal_credentials ||
		// amount_gwei || signature. The execution layer only queues the
		// signature; consensus-layer tests are responsible for validating it.
		input = make([]byte, 184)
		copy(input[:48], builderRequestPubkey)
		copy(input[60:80], ctx.TxSenderAccount().addr.Bytes())
		binary.BigEndian.PutUint64(input[80:88], 1_000_000_000)
		copy(input[88:], bytes.Repeat([]byte{0x24}, 96))
		to = params.BuilderDepositAddress
		value = new(big.Int).Add(big.NewInt(1_000_000_000_000_000_000), big.NewInt(1))
	} else {
		input = bytes.Clone(builderRequestPubkey)
		to = params.BuilderExitAddress
		value = big.NewInt(1)
	}

	sender := ctx.TxSenderAccount()
	tx := ctx.AddNewTx(sender, &types.DynamicFeeTx{
		ChainID:   ctx.ChainConfig().ChainID,
		Nonce:     ctx.AccountNonce(sender.addr),
		Value:     value,
		To:        &to,
		Data:      input,
		GasFeeCap: ctx.TxGasFeeCap(),
		GasTipCap: big.NewInt(2),
		Gas:       gas,
	})
	m.info = &modBuilderRequestInfo{
		TxHash: tx.Hash(),
		Block:  hexutil.Uint64(ctx.NumberU64()),
	}
	return true
}

func (m *modBuilderRequest) txInfo() any {
	return m.info
}
