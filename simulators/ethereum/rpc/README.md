# RPC ETH test suite

The RPC test suite runs a set of RPC related tests against a running node. It tests
several real-world scenarios such as sending value transactions, deploying a contract or
interacting with one.

    hive --client=go-ethereum --sim=ethereum/rpc

The perform their RPC calls to both the HTTP and WebSocket endpoint. Most tests can be run
over both interfaces but there are exceptions such as tests that require subscription
support.

## Architecture

The test suite consists of 2 groups:

- ethclient, contains tests using the `ethclient` package
- abi, contains tests using the `abi` package

The genesis block also contains 2 contracts:

- vault contract, used to fund new accounts that are created for the tests
- events contract, contract that raises events and is used for various tests

Ethclient runs various tests that use the `ethclient.Client` API. Such as sending
transactions, retrieving logs and balances.

ABI, interacts with the pre-deployed events contract. It send transactions, executes calls
and examines generated logs.

Each test is designed to run in parallel with other tests. In most cases the first step a
test performs is to create a new account and fund it from the vault contract. After the
account is funded the actual test logic runs.
