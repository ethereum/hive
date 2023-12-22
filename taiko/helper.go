package taiko

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/params"
	"github.com/taikoxyz/taiko-client/bindings"
	"github.com/taikoxyz/taiko-client/pkg/rpc"
)

func WaitHeight(ctx context.Context, n *ELNode, f func(uint64) bool) error {
	client, err := n.EthClient()
	if err != nil {
		return err
	}
	for {
		height, err := client.BlockNumber(ctx)
		if err != nil {
			return err
		}
		if f(height) {
			break
		}
		time.Sleep(100 * time.Millisecond)
		continue
	}
	return nil
}

func GetBlockHashByNumber(ctx context.Context, n *ELNode, num *big.Int, needWait bool) (common.Hash, error) {
	if needWait {
		if err := WaitHeight(ctx, n, GreaterEqual(num.Uint64())); err != nil {
			return common.Hash{}, err
		}
	}
	cli, err := n.EthClient()
	if err != nil {
		return common.Hash{}, err
	}
	block, err := cli.BlockByNumber(ctx, num)
	if err != nil {
		return common.Hash{}, err
	}
	return block.Hash(), nil
}

func WaitReceiptOK(ctx context.Context, cli *ethclient.Client, hash common.Hash) (*types.Receipt, error) {
	return WaitReceipt(ctx, cli, hash, types.ReceiptStatusSuccessful)
}

func WaitReceiptFailed(ctx context.Context, cli *ethclient.Client, hash common.Hash) (*types.Receipt, error) {
	return WaitReceipt(ctx, cli, hash, types.ReceiptStatusFailed)
}

func WaitReceipt(ctx context.Context, client *ethclient.Client, hash common.Hash, status uint64) (*types.Receipt, error) {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		receipt, err := client.TransactionReceipt(ctx, hash)
		if errors.Is(err, ethereum.NotFound) {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-ticker.C:
				continue
			}
		}
		if err != nil {
			return nil, err
		}
		if receipt.Status != status {
			return receipt, fmt.Errorf("expected status %d, but got %d", status, receipt.Status)
		}
		return receipt, nil
	}
}

func SubscribeHeight(ctx context.Context, n *ELNode, f func(*big.Int) bool) error {
	ch := make(chan *types.Header)
	cli, err := n.EthClient()
	if err != nil {
		return err
	}
	sub, err := cli.SubscribeNewHead(ctx, ch)
	if err != nil {
		return err
	}
	defer sub.Unsubscribe()

	for {
		select {
		case h := <-ch:
			if f(h.Number) {
				return nil
			}
		case err := <-sub.Err():
			return err
		case <-ctx.Done():
			return fmt.Errorf("program close before test finish")
		}
	}
}

func WaitProveEvent(ctx context.Context, n *ELNode, hash common.Hash) error {
	start := uint64(0)
	opt := &bind.WatchOpts{Start: &start, Context: ctx}
	eventCh := make(chan *bindings.TaikoL1ClientBlockProven)
	taikoL1, err := n.TaikoL1Client()
	if err != nil {
		return err
	}
	sub, err := taikoL1.WatchBlockProven(opt, eventCh, nil)
	defer sub.Unsubscribe()
	if err != nil {
		return err
	}
	for {
		select {
		case err := <-sub.Err():
			return err
		case e := <-eventCh:
			if e.BlockHash == hash {
				return nil
			}
		case <-ctx.Done():
			return fmt.Errorf("test is finished before watch proved event")
		}
	}
}

func WaitStateChange(n *ELNode, f func(*bindings.LibUtilsStateVariables) bool) error {
	taikoL1, err := n.TaikoL1Client()
	if err != nil {
		return err
	}
	for i := 0; i < 60; i++ {
		s, err := rpc.GetProtocolStateVariables(taikoL1, nil)
		if err != nil {
			return err
		}
		if f(s) {
			return nil
		}
		time.Sleep(500 * time.Millisecond)
		continue
	}
	return nil
}

func GenSomeBlocks(ctx context.Context, n *ELNode, v *Vault, cnt uint64) error {
	cli, err := n.EthClient()
	if err != nil {
		return err
	}
	curr, err := cli.BlockNumber(ctx)
	if err != nil {
		return err
	}
	end := curr + cnt
	for curr < end {
		v.CreateAccount(ctx, cli, big.NewInt(params.GWei))
		curr, err = cli.BlockNumber(ctx)
		if err != nil {
			return err
		}
		time.Sleep(10 * time.Millisecond)
	}
	return nil
}

func GreaterEqual(want uint64) func(uint64) bool {
	return func(get uint64) bool {
		if get >= want {
			return true
		}
		return false
	}
}
