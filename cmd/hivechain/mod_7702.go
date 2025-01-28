package main

import (
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/holiman/uint256"
)

// mod7702 creates an EIP-7702 interaction in the chain.
//
// It's a multi-step process:
//
//  - (1) a contract that will later serve as the account code is deployed.
//  - (2) Then a SetCode transaction is added which creates a delegation from the
//        EOA to the deployed code.
//  - (3) Finally, a contract call to the EOA is published, which triggers the
//        delegated code.
//
// The account contract (contracts/7702account.eas) writes to storage when invoked with
// input. For no input, it returns the current address and the storage value.

func init() {
	register("tx-eip7702", func() blockModifier {
		return &mod7702{}
	})
}

type mod7702TxInfo struct {
	Account     common.Address `json:"account"`
	ProxyAddr   common.Address `json:"proxyAddr"`
	AuthorizeTx common.Hash    `json:"authorizeTx"`
}

type mod7702 struct {
	stage       int
	proxyAddr   common.Address
	authorizeTx common.Hash
}

const (
	mod7702stageDeploy = iota
	mod7702stageAuthorize
	mod7702stageInvoke
	mod7702stageDone
)

func (m *mod7702) apply(ctx *genBlockContext) bool {
	if !ctx.ChainConfig().IsPrague(ctx.Number(), ctx.Timestamp()) {
		return false
	}

	prevStage := m.stage
	for ; m.stage < mod7702stageDone; m.stage++ {
		switch m.stage {
		case mod7702stageDeploy:
			if err := m.deployAccountCode(ctx); err != nil {
				return false
			}
		case mod7702stageAuthorize:
			if err := m.authorizeCode(ctx); err != nil {
				return false
			}
		case mod7702stageInvoke:
			if err := m.invokeAccount(ctx); err != nil {
				return false
			}
		}
	}
	return m.stage > prevStage
}

func (m *mod7702) deployAccountCode(ctx *genBlockContext) error {
	code, gas := codeToDeploy(ctx, mod7702AccountCode)
	if !ctx.HasGas(gas) {
		return fmt.Errorf("not enough gas to deploy")
	}

	sender := ctx.TxSenderAccount()
	nonce := ctx.AccountNonce(sender.addr)
	ctx.AddNewTx(sender, &types.LegacyTx{
		Nonce:    nonce,
		Gas:      gas,
		GasPrice: ctx.TxGasFeeCap(),
		Data:     code,
	})
	m.proxyAddr = crypto.CreateAddress(sender.addr, nonce)
	return nil
}

func (m *mod7702) authorizeCode(ctx *genBlockContext) error {
	auth, err := types.SignSetCode(mod7702Account.key, types.SetCodeAuthorization{
		ChainID: ctx.ChainConfig().ChainID.Uint64(),
		Address: m.proxyAddr,
		Nonce:   ctx.AccountNonce(mod7702Account.addr),
	})
	if err != nil {
		panic(err)
	}

	sender := ctx.TxSenderAccount()
	txdata := &types.SetCodeTx{
		ChainID:   ctx.ChainConfig().ChainID.Uint64(),
		Nonce:     ctx.AccountNonce(sender.addr),
		GasTipCap: uint256.MustFromDecimal("1"),
		GasFeeCap: uint256.MustFromBig(ctx.TxGasFeeCap()),
		To:        common.Address{},
		AuthList:  []types.SetCodeAuthorization{auth},
	}
	gas, err := core.IntrinsicGas(txdata.Data, txdata.AccessList, txdata.AuthList, false, true, true, true)
	if err != nil {
		panic(err)
	}
	txdata.Gas = gas
	if !ctx.HasGas(gas) {
		return fmt.Errorf("not enough gas to authorize")
	}
	tx := ctx.AddNewTx(sender, txdata)
	m.authorizeTx = tx.Hash()

	return nil
}

func (m *mod7702) invokeAccount(ctx *genBlockContext) error {
	const gas = 70000
	if !ctx.HasGas(gas) {
		return fmt.Errorf("not enough gas to invoke")
	}

	sender := ctx.TxSenderAccount()
	ctx.AddNewTx(sender, &types.LegacyTx{
		Nonce:    ctx.AccountNonce(sender.addr),
		GasPrice: ctx.TxGasFeeCap(),
		Gas:      70000,
		To:       &mod7702Account.addr,
		Data:     []byte("invoked"),
	})
	return nil
}

func (m *mod7702) txInfo() any {
	if m.stage < mod7702stageDone {
		return nil
	}
	return &mod7702TxInfo{
		Account:     mod7702Account.addr,
		ProxyAddr:   m.proxyAddr,
		AuthorizeTx: m.authorizeTx,
	}
}

var mod7702Account = genAccount{
	key:  mustParseKey("14cdde09d1640eb8c3cda063891b0453073f57719583381ff78811efa6d4199f"),
	addr: common.HexToAddress("0xedA8645bA6948855E3B3cD596bbB07596d59c603"),
}
