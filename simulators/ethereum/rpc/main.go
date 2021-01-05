package main

import (
	"math/big"

	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/hive/hivesim"
)

var (
	chainID   = big.NewInt(7) // used for signing transactions
	networkID = big.NewInt(8)
	gasPrice  = big.NewInt(50 * params.GWei)
)

var clientEnv = hivesim.Params{
	"HIVE_NETWORK_ID":     networkID.String(),
	"HIVE_CHAIN_ID":       chainID.String(),
	"HIVE_FORK_HOMESTEAD": "0",
	"HIVE_FORK_TANGERINE": "0",
	"HIVE_FORK_SPURIOUS":  "0",
	// All tests use clique PoA to mine new blocks.
	"HIVE_CLIQUE_PERIOD":     "1",
	"HIVE_CLIQUE_PRIVATEKEY": "9c647b8b7c4e7c3490668fb6c11473619db80c93704c70893d3813af4090c39c",
	"HIVE_MINER":             "658bdf435d810c91414ec09147daa6db62406379",
}

var files = map[string]string{
	"/genesis.json": "./init/genesis.json",
}

var tests = []hivesim.ClientTestSpec{
	// HTTP RPC tests.
	{Name: "http/BalanceAndNonceAt", Run: runHTTP(balanceAndNonceAtTest)},
	{Name: "http/CanonicalChain", Run: runHTTP(canonicalChainTest)},
	{Name: "http/CodeAt", Run: runHTTP(CodeAtTest)},
	{Name: "http/ContractDeployment", Run: runHTTP(deployContractTest)},
	{Name: "http/ContractDeploymentOutOfGas", Run: runHTTP(deployContractOutOfGasTest)},
	{Name: "http/EstimateGas", Run: runHTTP(estimateGasTest)},
	{Name: "http/GenesisBlockByHash", Run: runHTTP(genesisBlockByHashTest)},
	{Name: "http/GenesisBlockByNumber", Run: runHTTP(genesisBlockByNumberTest)},
	{Name: "http/GenesisHeaderByHash", Run: runHTTP(genesisHeaderByHashTest)},
	{Name: "http/GenesisHeaderByNumber", Run: runHTTP(genesisHeaderByNumberTest)},
	{Name: "http/Receipt", Run: runHTTP(receiptTest)},
	{Name: "http/SyncProgress", Run: runHTTP(syncProgressTest)},
	{Name: "http/TransactionCount", Run: runHTTP(transactionCountTest)},
	{Name: "http/TransactionInBlock", Run: runHTTP(transactionInBlockTest)},
	{Name: "http/TransactionReceipt", Run: runHTTP(TransactionReceiptTest)},

	// HTTP ABI tests.
	{Name: "http/ABICall", Run: runHTTP(callContractTest)},
	{Name: "http/ABITransact", Run: runHTTP(transactContractTest)},

	// WebSocket RPC tests.
	{Name: "ws/BalanceAndNonceAt", Run: runWS(balanceAndNonceAtTest)},
	{Name: "ws/CanonicalChain", Run: runWS(canonicalChainTest)},
	{Name: "ws/CodeAt", Run: runWS(CodeAtTest)},
	{Name: "ws/ContractDeployment", Run: runWS(deployContractTest)},
	{Name: "ws/ContractDeploymentOutOfGas", Run: runWS(deployContractOutOfGasTest)},
	{Name: "ws/EstimateGas", Run: runWS(estimateGasTest)},
	{Name: "ws/GenesisBlockByHash", Run: runWS(genesisBlockByHashTest)},
	{Name: "ws/GenesisBlockByNumber", Run: runWS(genesisBlockByNumberTest)},
	{Name: "ws/GenesisHeaderByHash", Run: runWS(genesisHeaderByHashTest)},
	{Name: "ws/GenesisHeaderByNumber", Run: runWS(genesisHeaderByNumberTest)},
	{Name: "ws/Receipt", Run: runWS(receiptTest)},
	{Name: "ws/SyncProgress", Run: runWS(syncProgressTest)},
	{Name: "ws/TransactionCount", Run: runWS(transactionCountTest)},
	{Name: "ws/TransactionInBlock", Run: runWS(transactionInBlockTest)},
	{Name: "ws/TransactionReceipt", Run: runWS(TransactionReceiptTest)},

	// WebSocket subscription tests.
	{Name: "ws/NewHeadSubscription", Run: runWS(newHeadSubscriptionTest)},
	{Name: "ws/LogSubscription", Run: runWS(logSubscriptionTest)},
	{Name: "ws/TransactionInBlockSubscription", Run: runWS(transactionInBlockSubscriptionTest)},

	// WebSocket ABI tests.
	{Name: "ws/ABICall", Run: runWS(callContractTest)},
	{Name: "ws/ABITransact", Run: runWS(transactContractTest)},
}

func main() {
	suite := hivesim.Suite{
		Name: "rpc",
		Description: `
The RPC test suite runs a set of RPC related tests against a running node. It tests
several real-world scenarios such as sending value transactions, deploying a contract or
interacting with one.`[1:],
	}
	for _, test := range tests {
		test.Parameters = clientEnv
		test.Files = files
		suite.Add(test)
	}
	hivesim.MustRunSuite(hivesim.New(), suite)
}
