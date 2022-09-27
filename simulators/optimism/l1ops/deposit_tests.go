package main

import (
	"math/big"
	"time"

	"github.com/ethereum-optimism/optimism/op-bindings/bindings"
	"github.com/ethereum-optimism/optimism/op-node/rollup/derive"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/hive/hivesim"
	"github.com/ethereum/hive/optimism"
	"github.com/stretchr/testify/require"
)

func simplePortalDepositTest(t *hivesim.T, env *optimism.TestEnv) {
	l1 := env.Devnet.L1Client(0)
	l2 := env.Devnet.L2Client(0)
	l1Vault := env.Devnet.L1Vault
	depositor := l1Vault.CreateAccount(env.TimeoutCtx(time.Minute), l1, big.NewInt(params.Ether))
	startBalance, err := l2.BalanceAt(env.Ctx(), depositor, nil)
	require.NoError(t, err)
	require.EqualValues(t, 0, startBalance.Int64())

	mintAmount := big.NewInt(0.5 * params.Ether)
	doDeposit(t, env, depositor, mintAmount)

	endBalance, err := l2.BalanceAt(env.Ctx(), depositor, nil)
	require.Nil(t, err)

	diff := new(big.Int)
	diff = diff.Sub(endBalance, startBalance)
	require.Equal(t, mintAmount, diff, "did not get expected balance change after mint")
}

func contractPortalDepositTest(t *hivesim.T, env *optimism.TestEnv) {
	l1 := env.Devnet.L1Client(0)
	l2 := env.Devnet.L2Client(0)
	l1Vault := env.Devnet.L1Vault
	l2Vault := env.Devnet.L2Vault
	depositor := l1Vault.CreateAccount(env.TimeoutCtx(time.Minute), l1, big.NewInt(params.Ether))
	l2Vault.InsertKey(l1Vault.FindKey(depositor))

	l2Opts := l2Vault.KeyedTransactor(depositor)
	l2Opts.Value = big.NewInt(0)
	l2Opts.NoSend = true

	_, deployTx, _, err := bindings.DeployERC20(l2Opts, l2, "TEST", "OP")
	require.NoError(t, err)

	l1Opts := l1Vault.KeyedTransactor(depositor)
	l1Opts.Value = big.NewInt(0.1 * params.Ether)

	portal := env.Devnet.Bindings.BindingsL1.OptimismPortal
	tx, err := portal.DepositTransaction(l1Opts, common.Address{}, common.Big0, 1_000_000, true, deployTx.Data())
	require.NoError(t, err)
	receipt := awaitDeposit(t, env, tx, l1, l2)
	require.NotEqual(t, common.Address{}, receipt.ContractAddress)

	opts := &bind.CallOpts{}
	l2Contract, err := bindings.NewERC20(receipt.ContractAddress, l2)
	require.NoError(t, err)
	name, err := l2Contract.Name(opts)
	require.NoError(t, err)
	require.Equal(t, "TEST", name)
	sym, err := l2Contract.Symbol(opts)
	require.NoError(t, err)
	require.Equal(t, "OP", sym)
}

func doDeposit(t *hivesim.T, env *optimism.TestEnv, depositor common.Address, mintAmount *big.Int) {
	depositContract, err := bindings.NewOptimismPortal(
		env.Devnet.Deployments.DeploymentsL1.OptimismPortalProxy,
		env.Devnet.L1Client(0),
	)
	require.NoError(t, err)

	l1 := env.Devnet.L1Client(0)
	l2 := env.Devnet.L2Client(0)
	l1Vault := env.Devnet.L1Vault
	opts := l1Vault.KeyedTransactor(depositor)
	opts.Value = mintAmount
	tx, err := depositContract.DepositTransaction(opts, depositor, common.Big0, 1_000_000, false, nil)
	require.NoError(t, err)
	receipt, err := optimism.WaitReceipt(env.TimeoutCtx(time.Minute), l1, tx.Hash())
	require.NoError(t, err)

	reconstructedDep, err := derive.UnmarshalDepositLogEvent(receipt.Logs[0])
	require.NoError(t, err, "could not reconstruct L2 deposit")
	tx = types.NewTx(reconstructedDep)
	receipt, err = optimism.WaitReceipt(env.TimeoutCtx(45*time.Second), l2, tx.Hash())
	require.NoError(t, err)
	require.Equal(t, receipt.Status, types.ReceiptStatusSuccessful)
}

func awaitDeposit(t *hivesim.T, env *optimism.TestEnv, tx *types.Transaction, l1, l2 *ethclient.Client) *types.Receipt {
	receipt, err := optimism.WaitReceipt(env.TimeoutCtx(time.Minute), l1, tx.Hash())
	require.NoError(t, err)
	reconstructedDep, err := derive.UnmarshalDepositLogEvent(receipt.Logs[0])
	require.NoError(t, err, "could not reconstruct L2 deposit")
	tx = types.NewTx(reconstructedDep)
	receipt, err = optimism.WaitReceipt(env.TimeoutCtx(45*time.Second), l2, tx.Hash())
	require.NoError(t, err)
	require.Equal(t, receipt.Status, types.ReceiptStatusSuccessful)
	return receipt
}
