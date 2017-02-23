# RPC ETH test suite
The RPC Eth test suite is designed to run a set of RPC related tests against a running node. It tests several real-world scenarios such as sending value transactions, deploying a contract or interacting with one. It can be run against a local node or within a hive multi-node simulation.

Tests are designed to run in parallel and can be run over http and/or websockets. Most tests can be run over both interfaces but there are exceptions such as tests that require subscription support.

## Standalone mode
This mode is useful when the user wants to safe some time by not running a full simulation. It allows the user to specify a subset of tests against a local or remote node.

### Setup and start local node
Get source.

```
> go get -u github.com/karalabe/hive
```

Create datadir and copy keys.

```
> geth --datadir $MY_DATADIR init $GOPATH/src/github.com/karalabe/hive/simulators/ethereum/rpc/eth/genesis.json
> mkdir $MY_DATADIR/keystore
> cp $GOPATH/src/github.com/karalabe/hive/simulators/ethereum/rpc/eth/keys/* $MY_DATADIR/keystore
```

Start node with HTTP and websocket interface and necessary API modules enabled.

```
> geth --datadir $MY_DATADIR --mine --minerthreads 1 --rpc --rpccorsdomain "*" --ws --wsapi "admin,eth,net,personal,web3" --wsorigins "*" --rpcapi "admin,eth,net,personal,web3"
```

### Run tests against local node
Generate ABI bindings (optional, repo contains already pre-generated bindings):

```
> cd $GOPATH/src/github.com/karalabe/hive/simulators/ethereum/rpc/eth
> abigen -abi contractABI.json -pkg main -type TestContract -out ABI_generated.go
```

Run ethclient test suite over websockets:

```
> go test -run "Eth/ethclient/websocket" -args 127.0.0.1
```

Or compile the tests and safe some time when running the tests multiple times:

```
> go test -c -o eth.test
> eth.test -test.run "Eth/ethclient/websocket" 127.0.0.1
```

## Run tests
The test suite consists of 2 groups:
- ethclient, contains tests using the `ethclient` package
- abi, contains tests using the `abi` package

Each of these test groups can be run over http and/or websockets.

### Specify specific tests
Run all ethclients tests over http and websockets against a local node.

```
> go test -run "Eth/ethclient" -args localhost
```

Run all ethclients tests over websockets against a local node.
```
> go test -run "Eth/ethclient/websocket" -args localhost
```

Run the `CallContract` test from the ABI suite over http.
```
> go test -run "Eth/abi/http/CallContract" -args localhost
```

Run the `CallContract` test from the ABI suite over http and websockets.
```
> go test -run "Eth/abi/[http|websocket]/CallContract" -args localhost
```


Run all tests against 1 local and 1 remote node.
```
> go test -run "Eth" -args localhost 192.168.1.1
```


See `main#TestEth` in `eth_test.go` for an overview of available tests.

## Hive simulation
The test suite can be run in a multi-node simulation. This simulation mimics a real world situation te best. The simulation constists of a bootnode and 5 nodes from which 2 nodes are mining. All tests are run by default.

### Run simulation against geth master branch

```
> go get -u github.com/karalabe/hive
> cd $GOPATH/src/github.com/karalabe/hive
> hive --client=go-ethereum:master --test=NONE --sim=ethereum/rpc/eth --docker-noshell --loglevel 6
```

Hive uses docker. On some platforms `--docker-noshell` argument needs to be given. More logging can be enabled by adding the `--loglevel 6` argument.

## Architecture

When run as a hive simulation the startup script will boot 5 nodes. 2 of them will be mining while the others are just spectator nodes. All nodes are connected with each other through a bootnode. 
The simulation will fail to start when a node isn't connected to the network within a reasonable time.

Each node will start with 1 account that has some pre-allocated ether assigned to it in the genesis block.
The genesis block also contains 2 contracts:
- vault contract, used to fund new accounts that are created for the tests
- events contract, contract that raises events and is used for various tests

### Tests

The test suite consists of 2 groups of tests:
- ethclient, raw RPC calls (ethclient.go)
- abi, generated contract bindings (abi.go)

Ethclient runs various tests that use the `ethclient.Client` API. Such as sending transactions, retrieving logs and balances.

ABI, interacts with the pre-deployed events contract. It send transactions, executs calls and examines generated logs.

All groups are further divided in interface sub-groups. Currently only http and websockets are supported.

#### Execution

When the test suite is started the first method that is called is the `TestMain` method. This method initialises the environment and calls the run method that executes the tests. Currently there is 1 top level test defined, `TestEth`. This method runs the ethclient and abi test groups. Both are `eth` related. In the future its possible that another top level test suite is added, e.g. swarm of whisper. By default all tests are executed but the user can filter and specify exact which tests needs to be run.

Each test is designed to run in parallel with other tests. In most cases the fist step a test performs is to create a new account and fund it from the vault contract. After the account is funded the actual test logic is run.

### Add test

New tests must added to the `ethclient.go` file or `abi.go` file depending in which group they belongs. Each test starts with a lower letter to ensure its not run by the go test framework on default. Instead its added to one or more of the appropriated groups in `eth_test.go`. 
