package main

import (
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/params"
	"github.com/protolambda/ztyp/view"
)

// blobTransactionTest creates a blob transaction. Then asserts that it is
// included in a block
func blobTransactionTest(t *TestEnv) {
	var (
		sourceAddr  = t.Vault.createAccount(t, big.NewInt(params.Ether))
		sourceNonce = uint64(0)
		targetAddr  = t.Vault.createAccount(t, nil)
	)

	// Get current balance
	sourceAddressBalanceBefore, err := t.Eth.BalanceAt(t.Ctx(), sourceAddr, nil)
	if err != nil {
		t.Fatalf("Unable to retrieve balance: %v", err)
	}

	expected := big.NewInt(params.Ether)
	if sourceAddressBalanceBefore.Cmp(expected) != 0 {
		t.Errorf("Expected balance %d, got %d", expected, sourceAddressBalanceBefore)
	}

	nonceBefore, err := t.Eth.NonceAt(t.Ctx(), sourceAddr, nil)
	if err != nil {
		t.Fatalf("Unable to determine nonce: %v", err)
	}
	if nonceBefore != sourceNonce {
		t.Fatalf("Invalid nonce, want %d, got %d", sourceNonce, nonceBefore)
	}

	var (
		amount   = big.NewInt(1234)
		gasLimit = uint64(210000)
	)

	var blobs types.Blobs
	blobs = append(blobs, types.Blob{})
	kzgCommitments, versionedHashes, aggregatedProof, err := blobs.ComputeCommitmentsAndAggregatedProof()
	if err != nil {
		t.Fatalf("unable to compute kzg commitments: %v", err)
	}
	txData := types.SignedBlobTx{
		Message: types.BlobTxMessage{
			ChainID:             view.MustUint256(chainID.String()),
			Nonce:               view.Uint64View(sourceNonce),
			Gas:                 view.Uint64View(gasLimit),
			GasFeeCap:           view.MustUint256(gasPrice.String()),
			GasTipCap:           view.MustUint256(gasPrice.String()),
			MaxFeePerDataGas:    view.MustUint256("3000000000"), // needs to be at least the min fee
			Value:               view.MustUint256(amount.String()),
			To:                  types.AddressOptionalSSZ{Address: (*types.AddressSSZ)(&targetAddr)},
			BlobVersionedHashes: versionedHashes,
		},
	}
	wrapData := types.BlobTxWrapData{
		BlobKzgs:           kzgCommitments,
		Blobs:              blobs,
		KzgAggregatedProof: aggregatedProof,
	}
	rawTx := types.NewTx(&txData, types.WithTxWrapData(&wrapData))
	valueTx, err := t.Vault.signTransaction(sourceAddr, rawTx)
	if err != nil {
		t.Fatalf("Unable to sign value tx: %v", err)
	}
	sourceNonce++

	t.Logf("blobTransactionTest: BalanceAt: send %d wei from 0x%x to 0x%x in 0x%x", valueTx.Value(), sourceAddr, targetAddr, valueTx.Hash())
	if err := t.Eth.SendTransaction(t.Ctx(), valueTx); err != nil {
		t.Fatalf("Unable to send transaction: %v", err)
	}
	t.Logf("blobTransactionTest: Sent Transaction for %v", valueTx.Hash())

	var receipt *types.Receipt
	for {
		receipt, err = t.Eth.TransactionReceipt(t.Ctx(), valueTx.Hash())
		if receipt != nil {
			break
		}
		if err != ethereum.NotFound {
			t.Fatalf("Could not fetch receipt for 0x%x: %v", valueTx.Hash(), err)
		}
		time.Sleep(time.Second)
	}
	t.Logf("blobTransactionTest: receipt for %v", valueTx.Hash())

	// ensure balances have been updated
	accountBalanceAfter, err := t.Eth.BalanceAt(t.Ctx(), sourceAddr, nil)
	if err != nil {
		t.Fatalf("Unable to retrieve balance: %v", err)
	}
	balanceTargetAccountAfter, err := t.Eth.BalanceAt(t.Ctx(), targetAddr, nil)
	if err != nil {
		t.Fatalf("Unable to retrieve balance: %v", err)
	}

	// expected balance is previous balance - tx amount - tx fee (gasUsed * gasPrice)
	exp := new(big.Int).Set(sourceAddressBalanceBefore)
	exp.Sub(exp, amount)
	exp.Sub(exp, new(big.Int).Mul(big.NewInt(int64(receipt.GasUsed)), valueTx.GasPrice()))
	exp.Sub(exp, big.NewInt(int64(len(versionedHashes)*params.DataGasPerBlob)))

	if exp.Cmp(accountBalanceAfter) != 0 {
		t.Errorf("Expected sender account to have a balance of %d, got %d", exp, accountBalanceAfter)
	}
	if balanceTargetAccountAfter.Cmp(amount) != 0 {
		t.Errorf("Expected new account to have a balance of %d, got %d", valueTx.Value(), balanceTargetAccountAfter)
	}

	// ensure nonce is incremented by 1
	nonceAfter, err := t.Eth.NonceAt(t.Ctx(), sourceAddr, nil)
	if err != nil {
		t.Fatalf("Unable to determine nonce: %v", err)
	}
	expectedNonce := nonceBefore + 1
	if expectedNonce != nonceAfter {
		t.Fatalf("Invalid nonce, want %d, got %d", expectedNonce, nonceAfter)
	}
}
