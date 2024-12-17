package main

import (
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/holiman/uint256"
)

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

// mod7702 first deploys a 7702 proxy.
// It then publishes an authorization for the proxy.
// In a third transaction, the proxy is invoked.
type mod7702 struct {
	stage       int
	proxyAddr   common.Address
	authorizeTx common.Hash
	ran         bool
}

func (m *mod7702) apply(ctx *genBlockContext) bool {
	if m.ran || !ctx.ChainConfig().IsPrague(ctx.Number(), ctx.Timestamp()) {
		return false
	}

	if err := m.deployProxy(ctx); err != nil {
		return false
	}
	if err := m.authorizeProxy(ctx); err != nil {
		return false
	}
	if err := m.invokeProxy(ctx); err != nil {
		return false
	}
	m.ran = true
	return true
}

func (m *mod7702) deployProxy(ctx *genBlockContext) error {
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

func (m *mod7702) authorizeProxy(ctx *genBlockContext) error {
	auth := types.Authorization{
		ChainID: ctx.ChainConfig().ChainID.Uint64(),
		Address: m.proxyAddr,
		Nonce:   ctx.AccountNonce(mod7702Account.addr),
	}
	signedAuth, err := types.SignAuth(auth, mod7702Account.key)
	if err != nil {
		return err
	}

	sender := ctx.TxSenderAccount()
	txdata := &types.SetCodeTx{
		ChainID:   ctx.ChainConfig().ChainID.Uint64(),
		Nonce:     ctx.AccountNonce(sender.addr),
		GasTipCap: uint256.MustFromDecimal("1"),
		GasFeeCap: uint256.MustFromBig(ctx.TxGasFeeCap()),
		To:        common.Address{},
		AuthList:  []types.Authorization{signedAuth},
	}
	gas, err := core.IntrinsicGas(txdata.Data, txdata.AccessList, txdata.AuthList, false, true, true, true)
	if err != nil {
		panic(err)
	}
	txdata.Gas = gas
	ctx.AddNewTx(sender, txdata)

	return nil
}

func (m *mod7702) invokeProxy(ctx *genBlockContext) error {
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
	if m.proxyAddr == (common.Address{}) || m.authorizeTx == (common.Hash{}) {
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
