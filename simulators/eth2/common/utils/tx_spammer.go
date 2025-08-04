package utils

import (
	"context"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/hive/hivesim"
	"github.com/ethereum/hive/simulators/ethereum/engine/globals"
	"github.com/ethereum/hive/simulators/ethereum/engine/helper"
	exec_client "github.com/marioevz/eth-clients/clients/execution"
)

type TransactionSpammer struct {
	*hivesim.T
	Name                     string
	ExecutionClients         []*exec_client.ExecutionClient
	Accounts                 []*globals.TestAccount
	Recipient                *common.Address
	TransactionType          helper.TestTransactionType
	TransactionsPerIteration int
	SecondsBetweenIterations int
}

func (t *TransactionSpammer) Run(ctx context.Context) error {
	// Send some transactions constantly in the bg
	nonceMap := make(map[common.Address]uint64)
	secondsBetweenIterations := time.Duration(t.SecondsBetweenIterations)
	txCreator := helper.BaseTransactionCreator{
		Recipient: t.Recipient,
		GasLimit:  500000,
		Amount:    common.Big1,
		TxType:    t.TransactionType,
	}
	txsSent := 0
	iteration := 0
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Second * secondsBetweenIterations):
			currentClient := t.ExecutionClients[iteration%len(t.ExecutionClients)]
			h, err := currentClient.HeaderByNumber(ctx, nil)
			if err == nil {
				for i := 0; i < t.TransactionsPerIteration; i++ {
					sender := t.Accounts[txsSent%len(t.Accounts)]
					nonce := nonceMap[sender.GetAddress()]
					tx, err := txCreator.MakeTransaction(sender, nonce, h.Time)
					if err != nil {
						panic(err)
					}
					if err := currentClient.SendTransaction(
						ctx,
						tx,
					); err != nil {
						t.Logf("INFO: Error sending tx (spammer %s): %v, sender: %s (%d), nonce=%d", t.Name, err, sender.GetAddress().String(), sender.GetIndex(), nonce)
					}
					nonceMap[sender.GetAddress()] = nonce + 1
					txsSent += 1
				}
				iteration += 1
			} else {
				t.Logf("INFO: Error fetching header: %v", err)
			}
		}
	}
}
