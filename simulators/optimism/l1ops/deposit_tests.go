package main

import (
	"encoding/json"
	"fmt"
	"math/big"
	"math/rand"
	"time"

	"github.com/ethereum-optimism/optimism/op-bindings/bindings"
	"github.com/ethereum-optimism/optimism/op-bindings/predeploys"
	"github.com/ethereum-optimism/optimism/op-chain-ops/crossdomain"
	"github.com/ethereum-optimism/optimism/op-node/rollup/derive"
	"github.com/ethereum-optimism/optimism/op-node/testutils"
	"github.com/ethereum-optimism/optimism/op-node/withdrawals"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/hive/hivesim"
	"github.com/ethereum/hive/optimism"
	hivebindings "github.com/ethereum/hive/optimism/bindings"
	"github.com/stretchr/testify/require"
)

var sentMessageEvent = common.HexToHash("0xcb0f7ffd78f9aee47a248fae8db181db6eee833039123e026dcbff529522e52a")
var optimismMintableERC20CreatedEvent = common.HexToHash("0x52fe89dd5930f343d25650b62fd367bae47088bcddffd2a88350a6ecdd620cdb")

func simplePortalDepositTest(t *hivesim.T, env *optimism.TestEnv) {
	l1 := env.Devnet.L1Client(0)
	l2 := env.Devnet.L2Client(0)
	l1Vault := env.Devnet.L1Vault
	depositor := l1Vault.CreateAccount(env.TimeoutCtx(time.Minute), l1, big.NewInt(params.Ether))
	startBalance, err := l2.BalanceAt(env.Ctx(), depositor, nil)
	require.NoError(t, err)
	require.EqualValues(t, 0, startBalance.Int64())

	mintAmount := big.NewInt(0.5 * params.Ether)
	doDeposit(t, env, depositor, mintAmount, false, nil)

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
	l1Opts.Value = big.NewInt(0.5 * params.Ether)
	// Set a high gas limit to prevent reverts. The gas limit
	// can sometimes be off by a bit as a result of the resource
	// metering code.
	l1Opts.GasLimit = 3_000_000

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

func erc20RoundtripTest(t *hivesim.T, env *optimism.TestEnv) {
	// Initial setup
	l1 := env.Devnet.L1Client(0)
	l2 := env.Devnet.L2Client(0)
	l1Vault := env.Devnet.L1Vault
	l2Vault := env.Devnet.L2Vault
	l1XDM := env.Devnet.Bindings.BindingsL1.L1CrossDomainMessenger
	l2XDM := env.Devnet.Bindings.BindingsL2.L2CrossDomainMessenger
	l1SB := env.Devnet.Bindings.BindingsL1.L1StandardBridge
	l2SB := env.Devnet.Bindings.BindingsL2.L2StandardBridge
	depositor := l1Vault.CreateAccount(env.TimeoutCtx(time.Minute), l1, big.NewInt(params.Ether))
	l2Vault.InsertKey(l1Vault.FindKey(depositor))

	// Deploy the ERC20 on L1
	l1Opts := l1Vault.KeyedTransactor(depositor)
	l1ERC20Addr, tx, l1ERC20, err := hivebindings.DeploySimpleERC20(l1Opts, l1, big.NewInt(1_000_000), "Test L1", 18, "L1")
	_, err = optimism.WaitReceipt(env.TimeoutCtx(30*time.Second), l1, tx.Hash())
	require.NoError(t, err)

	// Deposit some ETH onto L2
	doDeposit(t, env, depositor, big.NewInt(0.5*params.Ether), false, nil)

	// Deploy the bridged ERC20
	l2Opts := l2Vault.KeyedTransactor(depositor)
	factory := env.Devnet.Bindings.BindingsL2.OptimismMintableERC20Factory
	tx, err = factory.CreateOptimismMintableERC20(l2Opts, l1ERC20Addr, "Test L1", "L2")
	require.NoError(t, err)
	receipt, err := optimism.WaitReceipt(env.TimeoutCtx(30*time.Second), l2, tx.Hash())
	var creationEvent *bindings.OptimismMintableERC20FactoryOptimismMintableERC20Created
	for _, log := range receipt.Logs {
		if log.Topics[0] != optimismMintableERC20CreatedEvent {
			continue
		}
		creationEvent, err = factory.ParseOptimismMintableERC20Created(*log)
		require.NoError(t, err)
	}
	if creationEvent == nil {
		t.Fatalf("creation event not found")
	}

	var zeroAddr common.Address
	if creationEvent.LocalToken == zeroAddr {
		t.Fatalf("did not find creation event")
	}

	l2ERC20Addr := creationEvent.LocalToken
	l2ERC20, err := bindings.NewOptimismMintableERC20(creationEvent.LocalToken, l2)
	require.NoError(t, err)

	// approve
	tx, err = l1ERC20.Approve(l1Opts, predeploys.DevL1StandardBridgeAddr, abi.MaxUint256)
	require.NoError(t, err)
	_, err = optimism.WaitReceipt(env.TimeoutCtx(30*time.Second), l1, tx.Hash())
	require.NoError(t, err)

	// Remember starting L2 block to find the relay
	startBlock, err := l2.BlockNumber(env.Ctx())
	require.NoError(t, err)

	// Do the deposit
	tx, err = l1SB.DepositERC20(l1Opts, l1ERC20Addr, l2ERC20Addr, big.NewInt(1000), 200_000, nil)
	require.NoError(t, err)
	receipt, err = optimism.WaitReceipt(env.TimeoutCtx(30*time.Second), l1, tx.Hash())
	require.NoError(t, err)
	var l1SentMessage *bindings.L1CrossDomainMessengerSentMessage
	for _, log := range receipt.Logs {
		if log.Topics[0] != sentMessageEvent {
			continue
		}
		l1SentMessage, err = l1XDM.ParseSentMessage(*log)
		require.NoError(t, err)
	}
	if l1SentMessage == nil {
		t.Fatalf("could not find l1 SentMessage event")
	}

	// Derive the cross-domain message hash
	l1Hash, err := crossdomain.HashCrossDomainMessageV1(
		l1SentMessage.MessageNonce,
		&l1SentMessage.Sender,
		&l1SentMessage.Target,
		big.NewInt(0),
		l1SentMessage.GasLimit,
		l1SentMessage.Message,
	)
	require.NoError(t, err)

	// Poll for tx relay
	filterOpts := &bind.FilterOpts{
		Start:   startBlock,
		Context: env.TimeoutCtx(30 * time.Second),
	}
	pollTicker := time.NewTicker(time.Second)
	var l2RelayedEvent *bindings.L2CrossDomainMessengerRelayedMessage
	for {
		iter, err := l2XDM.FilterRelayedMessage(filterOpts, [][32]byte{l1Hash})
		require.NoError(t, err)
		for iter.Next() {
			l2RelayedEvent = iter.Event
			iter.Close()
			break
		}

		if l2RelayedEvent == nil {
			select {
			case <-filterOpts.Context.Done():
				t.Fatalf("timed out waiting for relayed message")
			case <-pollTicker.C:
				continue
			}
		}

		break
	}
	pollTicker.Stop()

	// check the balance
	balL1, err := l1ERC20.BalanceOf(&bind.CallOpts{Context: env.Ctx()}, depositor)
	require.NoError(t, err)
	require.EqualValues(t, big.NewInt(999_000), balL1)
	balL2, err := l2ERC20.BalanceOf(&bind.CallOpts{Context: env.Ctx()}, depositor)
	require.NoError(t, err)
	require.EqualValues(t, big.NewInt(1000), balL2)

	// Remember starting L1 block to find the relay
	startBlock, err = l1.BlockNumber(env.Ctx())
	require.NoError(t, err)

	// Perform the withdrawal
	tx, err = l2SB.Withdraw(l2Opts, l2ERC20Addr, big.NewInt(500), 0, nil)
	require.NoError(t, err)
	receipt, err = optimism.WaitReceipt(env.TimeoutCtx(30*time.Second), l2, tx.Hash())
	require.NoError(t, err)

	// Await finalization period
	finBlockNum, err := withdrawals.WaitForFinalizationPeriod(
		env.TimeoutCtx(5*time.Minute),
		l1,
		env.Devnet.Deployments.OptimismPortalProxy,
		receipt.BlockNumber,
	)

	// Get the last block number
	require.NoError(t, err)
	finHeader, err := l2.HeaderByNumber(env.Ctx(), big.NewInt(int64(finBlockNum)))
	require.NoError(t, err)

	j, _ := json.MarshalIndent(receipt, "", "  ")
	fmt.Println(string(j))

	// Get withdrawal parameters
	l2Client := withdrawals.NewClient(env.Devnet.GetOpL2Engine(0).RPC())
	wParams, err := withdrawals.FinalizeWithdrawalParameters(env.Ctx(), l2Client, tx.Hash(), finHeader)
	require.NoError(t, err)

	// Finalize the withdrawal
	portal := env.Devnet.Bindings.BindingsL1.OptimismPortal
	finTx, err := portal.FinalizeWithdrawalTransaction(
		l1Opts,
		bindings.TypesWithdrawalTransaction{
			Nonce:    wParams.Nonce,
			Sender:   wParams.Sender,
			Target:   wParams.Target,
			Value:    wParams.Value,
			GasLimit: wParams.GasLimit,
			Data:     wParams.Data,
		},
		wParams.BlockNumber,
		wParams.OutputRootProof,
		wParams.WithdrawalProof,
	)
	require.NoError(t, err)
	_, err = optimism.WaitReceipt(env.TimeoutCtx(time.Minute), l1, finTx.Hash())
	require.NoError(t, err)

	// Verify L1/L2 balances
	balL1, err = l1ERC20.BalanceOf(&bind.CallOpts{Context: env.Ctx()}, depositor)
	require.NoError(t, err)
	require.EqualValues(t, big.NewInt(999_500), balL1)

	balL2, err = l2ERC20.BalanceOf(&bind.CallOpts{Context: env.Ctx()}, depositor)
	require.NoError(t, err)
	require.EqualValues(t, big.NewInt(500), balL2)
}

func failingDepositWithMintTest(t *hivesim.T, env *optimism.TestEnv) {
	// Initial setup
	l1 := env.Devnet.L1Client(0)
	l2 := env.Devnet.L2Client(0)
	l1Vault := env.Devnet.L1Vault
	l2Vault := env.Devnet.L2Vault
	depositContract := env.Devnet.Bindings.BindingsL1.OptimismPortal
	depositor := l1Vault.CreateAccount(env.TimeoutCtx(time.Minute), l1, big.NewInt(3*params.Ether))
	l2Vault.InsertKey(l1Vault.FindKey(depositor))
	l2Opts := l2Vault.KeyedTransactor(depositor)

	// Fund account on L2
	doDeposit(t, env, depositor, big.NewInt(0.5*params.Ether), false, nil)

	// Deploy the failure contract on L2

	_, deployTx, failureContract, err := hivebindings.DeployFailure(l2Opts, l2)
	require.NoError(t, err)
	_, err = optimism.WaitReceipt(env.TimeoutCtx(30*time.Second), l2, deployTx.Hash())
	require.NoError(t, err)

	// Create the revert() call
	l2Opts.NoSend = true
	l2Opts.GasLimit = 1_000_000
	revertTx, err := failureContract.Fail(l2Opts)
	require.NoError(t, err)

	// Create garbage data
	randData := make([]byte, 32)
	_, err = rand.Read(randData)
	require.NoError(t, err)

	testData := [][]byte{
		randData,
		revertTx.Data(),
	}
	mintAmount := big.NewInt(0.5 * params.Ether)
	opts := l1Vault.KeyedTransactor(depositor)
	opts.Value = mintAmount
	opts.GasLimit = 3_000_000
	for _, data := range testData {
		startBal, err := l2.BalanceAt(env.Ctx(), depositor, nil)
		require.NoError(t, err)
		tx, err := depositContract.DepositTransaction(
			opts,
			depositor,
			mintAmount,
			1_000_000,
			false,
			data,
		)
		require.NoError(t, err)
		receipt, err := optimism.WaitReceipt(env.TimeoutCtx(time.Minute), l1, tx.Hash())
		require.NoError(t, err)

		reconstructedDep, err := derive.UnmarshalDepositLogEvent(receipt.Logs[0])
		require.NoError(t, err, "could not reconstruct L2 deposit")
		tx = types.NewTx(reconstructedDep)
		_, err = optimism.WaitReceipt(env.TimeoutCtx(45*time.Second), l2, tx.Hash())
		require.NoError(t, err)

		endBal, err := l2.BalanceAt(env.Ctx(), depositor, nil)
		require.Nil(t, err)
		require.True(t, testutils.BigEqual(mintAmount, new(big.Int).Sub(endBal, startBal)))
	}
}

func doDeposit(t *hivesim.T, env *optimism.TestEnv, depositor common.Address, mintAmount *big.Int, isCreation bool, data []byte) {
	depositContract := env.Devnet.Bindings.BindingsL1.OptimismPortal
	l1 := env.Devnet.L1Client(0)
	l2 := env.Devnet.L2Client(0)
	l1Vault := env.Devnet.L1Vault
	opts := l1Vault.KeyedTransactor(depositor)
	opts.Value = mintAmount

	// Set a high gas limit to prevent reverts. The gas limit
	// can sometimes be off by a bit as a result of the resource
	// metering code.
	opts.GasLimit = 3_000_000
	tx, err := depositContract.DepositTransaction(opts, depositor, common.Big0, 1_000_000, isCreation, data)
	require.NoError(t, err)
	receipt, err := optimism.WaitReceipt(env.TimeoutCtx(time.Minute), l1, tx.Hash())
	require.NoError(t, err)

	reconstructedDep, err := derive.UnmarshalDepositLogEvent(receipt.Logs[0])
	require.NoError(t, err, "could not reconstruct L2 deposit")
	tx = types.NewTx(reconstructedDep)
	_, err = optimism.WaitReceipt(env.TimeoutCtx(45*time.Second), l2, tx.Hash())
	require.NoError(t, err)
}

func awaitDeposit(t *hivesim.T, env *optimism.TestEnv, tx *types.Transaction, l1, l2 *ethclient.Client) *types.Receipt {
	receipt, err := optimism.WaitReceipt(env.TimeoutCtx(time.Minute), l1, tx.Hash())
	require.NoError(t, err)
	reconstructedDep, err := derive.UnmarshalDepositLogEvent(receipt.Logs[0])
	require.NoError(t, err, "could not reconstruct L2 deposit")
	tx = types.NewTx(reconstructedDep)
	receipt, err = optimism.WaitReceipt(env.TimeoutCtx(45*time.Second), l2, tx.Hash())
	require.NoError(t, err)
	return receipt
}
