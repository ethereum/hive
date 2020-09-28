# hive - Ethereum end-to-end test harness

Ethereum grew large to the point where testing implementations is a huge burden. Unit tests are fine
for checking various implementation quirks, but validating that a client conforms to some baseline
quality or validating that clients can play nicely together in a multi client environment is all but
simple.

This project is meant to serve as an easily expandable test harness where **anyone** can add tests in
**any** programming language they feel comfortable with, and it should simultaneously be able to run
those tests against all potential clients. The harness is meant to do black box testing where no
client specific internal details/state is tested and/or inspected. Instead, the emphasis is put on
adherence to official specs or behaviours under different circumstances.

Most importantly, it is essential to be able to run this test suite as part of the CI workflow!
Therefore, the entire suite is based on docker containers.

# Public test results

An Ethereum Foundation server often runs Hive to check for consensus, p2p and blockchain compatibility.
Test results are made public [here](https://hivetests.ethdevops.io/)

# Installing the hive validator

The `hive` project is based on Go. Although there are plans to make running `hive` possible using
only docker, for now you'll need a valid Go (1.6 and upwards) installation available.

You can install `hive` via:

```
$ go get -v github.com/ethereum/hive
```
or if you don't have GO111MODULE set: 
```
$ GO111MODULE=on go get -v github.com/ethereum/hive
```
After installing add hive binary (default path `~go/bin`) to PATH and clone the `Hive` repository
to your local folder.

 *Note: For now `hive` requires running from the repository root as it needs access to quite a number
of resource files to build the corrent docker images and containers. This requirement will be removed
in the future.*

# Quickstart command lines

This section is a quickstart of command line options covering typical Hive usage scenarios.

For additional Windows-specific command line options please check the section following.

## Consensus test
To execute the consensus tests on parity run the latest simulator:

```text
   --docker-noshell
   --client parity_latest
   --sim ethereum/consensus2
   --sim.rootcontext
   --results-root /mytests/test
```

## Devp2p tests

These run devp2p tests, mainly Discovery tests.

```text
    --sim devp2p
    --sim.rootcontext
    --test none
    --loglevel 6
    --docker-noshell
    --results-root /mytests/test
    --client go-ethereum_latest
    --sim-parallelism 1
```

## Sync simulation

These run a test verifying that a blockchain can be synced between differing implementations.

      --sim ethereum\\\\sync
      --sim.rootcontext
      --test none
      --loglevel 6
      --docker-noshell
      --results-root /mytests/test
      --client go-ethereum_latest,parity_latest
      --sim-parallelism 1

## Generate a blockchain

This uses Hive to generate a blockchain based on a genesis file for subsequent testing in sync simulations.

      --loglevel 6
      --chainGenerate
      --chainLength 10
      --chainOutputPath C:\\Ethereum\\OutputPath
      --chainGenesis C:\\Ethereum\\Input\\Genesis.json
      --chainBlockTime 30

## Iterating on bugfixes locally

Often, after reproducing a failure with a hive run on your machine, you'll want to build a docker container with code changes and see if it fixes the issue. Enabling this sort of workflow will require a couple of steps:

* Build your client's docker image locally with a different tag to the one in your client's dockerfile.
* Edit the FROM field in the dockerfile for your client, found in `clients/[your_client_name_here]/dockerfile`, to point to your newly created image
* Run hive again with the flag `--docker-nocache <regexp>`, where `<regexp>` is a regular expression matching your client's name, such as `besu`


# Running on Windows

The following information assumes Docker for Windows (CE) is installed on Windows 10 Pro. 

## Docker daemon
`hive` uses the Docker API to connect to the Docker Daemon to dynamically create and run containers. At the time of writing, the daemon is disabled by default. This must be enabled.

To enable the daemon, right click on the Docker icon in the system tray and press Settings. Under 'General' select "Expose daemon on tcp.... without TLS".

To run `hive`, use the following command line option 

```text
  --docker-endpoint tcp://localhost:2375 
```

Alternatively, if using VSCode simply run using the supplied launch.json (see below)

## Shell container
Currently, the Windows version must be run from the Host. To achieve this run with the --docker-noshell command line option. 

## Debugging or executing from Visual Studio Code
As described above, golang must be installed on the machine. The golang extension for VSCode is then required, along with Delve and the standard tools recommended by the Golang extension.

When VS Code is configured for general go development, `hive` may be run simply by launching with F5 with the following `launch.json`. This `launch.json` includes example parameters that limit the client to `geth` as the full client suite may take significant time to build initial docker images.
```json
{
   
    "version": "0.2.0",
    "configurations": [
        {
            "name": "Launch",
            "type": "go",
            "request": "launch",
            "mode": "debug",
            "remotePath": "",
            "port": 2345,
            "host": "127.0.0.1",
            "program": "${workspaceFolder}",
            "env": {},
            "args": [
                
                "--docker-endpoint","tcp://localhost:2375",
                "--docker-noshell",
                "--client","go-ethereum_latest" , 
                "--loglevel","6",
                "--smoke"
                
               
            ],
            "showLog": true
        }
    ]
}
```

## Access to the local drive
Docker will need access to the `workspace` folder. This will either be requested automatically in an Windows notification, or permission can be set in the docker settings in advance.

To set the permissions to access your drive, right click on the Docker Whale in the system tray and press Settings. Under 'Shared Drives' select the drive where the `workspace` folder is for sharing.

## Host access to the docker network

`hive` requires network access to the docker containers it creates. While this is automatically available on Linux, at the time of writing because of virtualisation there needs to be some further network configuration so that the `hive` host can connect. The following is dependent on your docker configuration, and there may be other ways to achieve the same result, but a typical setting may be:

'route /P add 172.17.0.0 MASK 255.255.0.0 10.0.75.2'

An administrator level command prompt must be opened and the target IPs of the containers routed to HyperV's IP address for the docker containers.


# Validating clients

UPDATE: Unless we hear a desire to keep them, Validators will be deprecated. Please see `Simulators` for updates.

You can run the full suite of `hive` validation tests against all the known implementations tagged
`latest` by simply running `hive` from the repository root. It will build a docker image for every
known client as well as one for every known validation test.

The end result should be a JSON report, detailing for each client the list of validations failed and
those passed. If you wish to explore the reasons of failure, full logs from all clients and testers
are pushed into the `workspace/logs` folder.

```
$ hive --client=go-ethereum_latest --test=.
...
Validation results:
{
  "go-ethereum:latest": {
    "fail": [
      "ethereum/rpc-tests"
    ],
    "pass": [
      "smoke/genesis-chain-blocks",
      "smoke/genesis-only",
      "smoke/genesis-chain"
    ]
  }
}
```

You may request a different set of clients to be validated via the `--client` flag. 

You may request only a subset of validation tests to be run via the `--test` regexp flag (e.g. running only the
smoke validation tests would be `--test=smoke`).

# Simulating clients


----
`hive` supports a more advanced form of client testing called *simulations*, where entire networks
of clients are run concurrently under various circumstances and their behavior monitored and checked.

Running network simulations is completely analogous to validations from the user's perspective: you
can specify which clients to simulate with the `--client` flag, and you can specify which
simulations to run via the `--sim` regexp flag. By default simulations aren't being run as they can
be quite lengthy.


`--client` is now an explicit list of permitted clients, where the desired client is of the format
clientname_branch. For example `go-ethereum_latest` and `parity_beta`. Previously this was a regex. 
Other parameters such as the sim or validator still use regexp. This change was introduced to allow
for branches to be specified as parameters without the need for creating a dedicated client subfolder 
for that branch. `go-ethereum` and `parity` in this example need to be subfolders of the Hive `clients`
folder with their own dockerfile. The branch is parsed out and passed as a docker build arg.


Simulators now offer a golang client framework, that allows them to call into the Hive Simulator 
API and create different types of client. The simulator can run tests or other experiments written in 
Golang against one or more instances of clients. To achieve this, a number of new options are added:

`--sim.rootcontext` a boolean, which when set tells the compiler to build the docker image with 'simulators'
as the root of the context, allowing the simulators\common and simulators\devp2p common code to be included
in the simulator. 

Sim.rootcontext needs to be set differently depending on the type of simulation being run. For the consensus tests
the base simulator image relies on files to be added from a folder local to the image. For developing new simulations,
or extending the existing ones, it is recommended to use sim-rootcontext as true. 


`--debug` allows a flag to be set that is passed into the simulator as an environment variable, allowing the 
simulator to be run as a delve 'headless server. The go simulator can then be remote debugged by attaching to 
the delve headless server.

`--sim-parallelism` a flag to indicate how many tests or containers should be run concurrently. This can be
implementation specific. In this version it is used to drive the -test.parallel flag in the devp2p simulation.



Similarly to validations, end result of simulations should be a JSON report, detailing for each
client the list of simulations failed and those passed. Likewise, if you wish to explore the reasons
of failure, full logs from all clients and testers are pushed into the `log.txt` log file.

```
$ hive --client=go-ethereum_latest --test=NONE --sim=.
...
Simulation results:
{
  "go-ethereum:latest": {
    "pass": [
      "dao-hard-fork",
      "smoke/single-node"
    ]
  }
}
```



# Adding new clients

The `hive` test harness can validate arbitrary implementations of the [Ethereum yellow paper](http://gavwood.com/paper.pdf).

Since `hive` is based on docker containers, it is pretty liberal on most aspects of a client
implementation:

 * `hive` doesn't care what dependencies a client has: language, libraries or otherwise.
 * `hive` doesn't care how the client is built: environment, tooling or otherwise.
 * `hive` doesn't care what garbage a client generates during execution.

As long as a client can run on Linux, and you can package it up into a Docker image, `hive` can test it!

## Creating a client image

Adding a new client implementation to `hive` entails creating a Dockerfile (and related resources),
based on which `hive` will assemble the docker image to use as the blueprint for testing.

The client definition(s) should reside in the `clients` folder, each named `<project>_<tag>` where
`<project>` is the official name of the client (lowercase, no fancy characters), and `<tag>` is an
arbitrary id up to the client maintainers to make the best use of. `hive` will automatically pick
up all clients from this folder.

There aren't many contraints on the image itself, though a few required caveats exist:

 * It should be as tiny as possible (play nice with others). Preferably use `alpine` Linux.
 * It should expose the following ports: 8545 (HTTP RPC), 8546 (WS RPC), 30303 (devp2p).
 * It should have a single entrypoint (script?) defined, which can initialize and run the client.

For devp2p tests or other simulations that require to know the specific enode of the client instance, 
the client must provide an `enode.sh` that echoes the enode of the running instance. This is executed
by the Hive host remotely to get the id. 

The client has the responsibility of mapping the Hive genesis.json and Hive environment variables
to its own local genesis format and command line flags. To assist in this, Hive illustrates a technique
in the `clients/trinity` folder using `mapper.jq`, which is invoked in `trinity.sh` This 
technique can be replicated for other clients.

For guidance, check out the reference [`go-ethereum:latest`](https://github.com/ethereum/hive/blob/master/clients/go-ethereum/Dockerfile) client.

### Initializing the client

Since `hive` does not want to enforce any CLI parameterization scheme on client implementations, it
injects all the required configurations into the Linux containers prior to launching the client's
`entrypoint` script. It is then left to this script to interpret all the environmental configs and
initialize the client appropriately.

The chain configurations files:

 * `/genesis.json` contains the JSON specification of the Ethereum genesis states
 * `/chain.rlp` contains a batch of RLP encoded blocks to import before startup
 * `/blocks/` folder with numbered singleton blocks to import before startup
 * `/keys/` contains account keys that should be imported before startup

Client startup scripts need to ensure that they load the genesis state first, then import a possibly
longer blockchain and then import possibly numerous individual blocks. The reason for requiring two
different block sources is that specifying a single chain is more optimal, but tests requiring forking
chains cannot create a single chain.

Besides the standardized chain configurations, clients can in general be modified behavior-wise in
quite a few ways that are mostly supported by all clients, yet are implemented differently in each.
As such, each possible behavioral change required by some validator or simulator is characterized by
an environment variable, which clients should interpret as best as they can.

The behavioral configuration variables:

  * `HIVE_BOOTNODE` enode URL of the discovery-only node to bootstrap the client
  * `HIVE_TESTNET` whether clients should run with modified starting nonces (`2^20`)
  * `HIVE_NODETYPE` specifying the sync and pruning algos that should be used
    * If unset, then uninteresting and run in the node's default mode
    * If `archive`, assumes that all historical state is retained after sync
    * If `full`, assumes fast sync and consecutive pruning of historical state
    * If `light`, assumes header only sync and no state maintenance at all
  * `HIVE_FORK_HOMESTEAD` the block number of the Ethereum Homestead transition
  * `HIVE_FORK_DAO_BLOCK` the block number of the DAO hard-fork transition
  * `HIVE_FORK_DAO_VOTE` whether the node supports or opposes the DAO hard-fork
  * `HIVE_FORK_TANGERINE` the block number of the Ethereum TangerineWhistle transition
    * The HF for repricing certain opcodes, EIP 150
  * `HIVE_FORK_SPURIOUS` the block number of the Ethereum Homestead transition
    * The HF for replay protection, state cleaning etc. EIPs 155,160,161. 
  * `HIVE_FORK_METROPOLIS` the block number of the Metropolis hardfork
  * `HIVE_MINER` address to credit with mining rewards (if set, start mining)
  * `HIVE_MINER_EXTRA` extra-data field to set for newly minted blocks


### Starting the client

After initializing the client blockchain (genesis, chain, blocks), the last task of the entry script
is to start up the client itself. The following defaults are required by `hive` to enable automatic
network assembly and firewall enforcement:

 * Clients should open their HTTP-RPC endpoint on `0.0.0.0:8545` (mandatory)
 * Clients should open their WS-RPC endpoint on `0.0.0.0:8546` (optional)
 * Clients should open their IPC-RPC endpoints at `/rpc.ipc` (optional)

There is no need to handle graceful client termination. Clients will be forcefully aborted upon test
suite completion and all related data purged. A new instance will be started for every test.

### Smoke testing new clients

To quickly check if a client adheres to the requirements of `hive`, there is a suite of smoke test
validations and simulations that just initialize clients with some pre-configured states and queries
it from the various RPC endpoints.

```
$ hive --client=go-ethereum_latest --smoke
...
Validation results:
{
  "go-ethereum:latest": {
    "pass": [
      "smoke/genesis-chain",
      "smoke/genesis-chain-blocks",
      "smoke/genesis-only"
    ]
  }
}
...
Simulation results:
{
  "go-ethereum:latest": {
    "smoke/lifecycle": {
      "start": "2017-01-31T09:20:16.975219924Z",
      "end": "2017-01-31T09:20:18.705302536Z",
      "success": true
    }
  }
}
```

*Note: All smoke tests must pass for a client to be included into `hive`.*

# Adding new validators

Validators are `hive` testers whose sole purpose is to check that a client implementation conforms to
some standardized specifications (e.g. RPC API conformance, Mist compatibility, consensus tests, etc).
They are not meant to check a client's behavior in complex network environment, rather that given a
single client instance, it seems to function correctly from the **users** perspective.

Similar to client implementations inside `hive`, validators themselves are also based on docker images
and containers:

 * `hive` doesn't care what dependencies a validator has: language, libraries or otherwise.
 * `hive` doesn't care how the validator is built: environment, tooling or otherwise.
 * `hive` doesn't care what garbage a validator generates during execution.

As long as a validator can run on Linux, and you can package it up into a Docker image, `hive` can
use it to test every client implementation with it!

## Creating a validator image

Adding a new client validator to `hive` entails creating a Dockerfile (and related resources), based
on which `hive` will assemble the docker image to use as the blueprint for validation.

The validator definition(s) should reside in the `validators` folder, each nested as `<group>/<test>`,
where `<group>` is a higher level collection of similar tests (e.g. `mist`, `consensus`), and `<test>`
is an individual validator. Contributors are free to define new groups as long as it makes sense from
a cross-client perspective. A few existing ones are:

 * `ethereum` contains ports of old test frameworks from the Ethereum repositories
 * `issues` contains interesting corner cases from clients that may affect others too
 * `mist` contains API conformance tests to validate if a client can be a [Mist](https://github.com/ethereum/mist) backend
 * `smoke` contains general smoke tests to insta-check if a client image is correct

There are little contraints on the image itself, though a few required caveats are:

 * It should be as tiny as possible (play nice with others). Preferably use `alpine` Linux.
 * It should have a single entrypoint (script?) defined, which can initialize and run the test.

For guidance, check out the [genesis-only](https://github.com/karalabe/hive/blob/master/validators/smoke/genesis-only/Dockerfile) smoke test.

### Defining the validator

> Since `hive` does not want to enforce any CLI parameterization scheme on client implementations, it
injects all the required configurations into the Linux containers prior to launching the client's
`entrypoint` script. It is then left to this script to interpret all the environmental configs and
initialize the client appropriately.

What this means from a validator's perspective is, that validators themselves must define these
initial chain configurations files and client behavioral modifier environment variables that will
be loaded into the client images.

To prevent duplicating the list of config files and env vars that need to be implemented to cross
over from validators to clients, we'll refer the reader to the readme's [*"Initializing the client"*](#initializing-the-client)
section which lists all of them, also detailing what each means.

In short, a validator must create all of the above linked chain configuration files and define some
subset of behavioral environmental variables **in the validator's docker image**. This is important
as the validator is always started **after** the client, so all information needs to be already ready
for client initialization prior to running the validator entrypoint.

*Trick: If you don't want to initialize a tested client with a starting chain, only the genesis file,
you can specify `RUN touch chain.rlp && mkdir /blocks` in the validator Dockerfile, which will create
an empy initial chain and empty set of blocks.*

### Executing the validation

After `hive` creates and initializes a client implementation based on the docker **image** of the
chosen validator, it will extract the IP address of the running client and boot up the validator
with the client's IP address injected as the `HIVE_CLIENT_IP` environmental variable.

From this point onward the validator may execute arbitrary code against the running clients:

 * HTTP RPC endpoint exposed at `HIVE_CLIENT_IP:8545`
 * WebSocket RPC (if supported) at `HIVE_CLIENT_IP:8546`
 * devp2p TCP and UDP endpoints at `HIVE_CLIENT_IP:30303`

The validation will be considered successful if and only if the exit code of the entrypoint script
is zero! Any output that the validator generates will be saved to an appropriate log file in the `hive`
workspace folder and also echoed out to the console on `--loglevel=6`.

*Note: There is no constraint on how much a validation may run, but please be considerate.*

# Adding new simulators

Simulators are `hive` testers whose purpose is to check that client implementations conform to some
desired behavior when running in both multi-node network environments. The goal of a simulator is to try
and test a node in an almost-live way: don't take testing shortcuts like fake PoW or less
secure key encryption schemas.

Similar to all other entities inside `hive`, simulators too are based on docker images and containers:

 * `hive` doesn't care what dependencies a simulator has: language, libraries or otherwise.
 * `hive` doesn't care how the simulator is built: environment, tooling or otherwise.
 * `hive` doesn't care what garbage a simulator generates during execution.

As long as a simulator can run on Linux, and you can package it up into a Docker image, `hive` can
use it to test every client implementation with it!

## Creating a simulator image

Adding a new client simulator to `hive` entails creating a Dockerfile (and related resources), based
on which `hive` will assemble the docker image to use as the blueprint for simulation.

The simulator definition(s) should reside in the `simulators` folder, each nested as `<group>/<sim>`,
where `<group>` is a higher level collection of similar tests (e.g. `dao-hard-fork`), and `<sim>` is
an individual simulator. Contributors are free to define new groups as long as it makes sense from a
cross-client perspective. A few existing ones are:

 * `dao-hard-fork` contains network simulations/tests with regard to the DAO hard-fork
 * `smoke` contains general smoke tests to insta-check if a client image is correct

There are little contraints on the image itself, though a few required caveats are:

 * It should be as tiny as possible (play nice with others). Preferably use `alpine` Linux.
 * It should have a single entrypoint (script?) defined, which can initialize and run the test.

For guidance, check out the [lifecycle](https://github.com/karalabe/hive/blob/master/simulators/smoke/lifecycle/Dockerfile) smoke test.

### Defining the simulator

Defining a simulator is **exactly** the same as defining the docker image of a validator with regard
to every aspect of `hive` (chain configs, behavioral envvars), As such, we refer the user to the readme's
[*"Defining the validator"*](#defining-the-validator) section. Apart from what the `entrypoint` script
is allowed to do, validator and simulator images are equivalent.

### Executing the simulation

As detailed in the readme's [*"Executing the validation"*](#executing-the-validation) section, during
validation `hive` starts a client node first, and then the validator itself. This is **not** true in
the case of simulations however. Since it is impossible to define arbitrary networking scenarios with
simple configuration files, `hive` will instead boot up only the simulator instance, and will provide
it with the necessary mechanisms to create any scenario it wants.

To this effect, `hive` exposes a RESTful HTTP API that all simulators can use to direct how `hive`
should create and organize the simulated network. This API is exposed at the HTTP endpoint set in the
`HIVE_SIMULATOR` environmental variable. The currently available topology endpoints are:

 * `/nodes` with method `POST` boots up a new client instance, returning its unique ID
   * Simulators may override any [chain init files](#initializing-the-client) via `URL` and `form` parameters (see below)
   * Simulators may override any [behavioral envvars](#initializing-the-client) directly via `URL` and `form` parameters
 * `/nodes/$ID` with method `GET` retrieves the IP address of an existing client instance
   * The client's exposed services can be reached via ports `8545`, `8546` and `30303`
 * `/nodes/$ID` with method `DELETE` instantly terminates an existing client instance
 * `/log` with method `POST` sends a logging message from the simulator to the main process.
 

Overriding environmental variables that change client behaviors via HTTP parameters is easy to do in
any HTTP client utility and/or library, but uploading files needed for chain initializations is much
more complex, especially if multiple files are needed. As long as all clients run with the same set
of init files, this is not an issue (they can be placed in the default locations). However if instances
need to run with different initial chain setups, a simulator needs to be able to specify these per
client. To avoid file uploads, `hive` solves this by defining a set of API variables that allow a
simulator to specify the source paths to use for specific init files which will be extracted from the
live container:

 * `HIVE_INIT_GENESIS` path to the genesis file to seed the client with (default = "/genesis.json")
 * `HIVE_INIT_CHAIN` path to an initial blockchain to seed the client with (default = "/chain.rlp")
 * `HIVE_INIT_BLOCKS` path to a folder of blocks to import after seeding (default = "/blocks/")
 * `HIVE_INIT_KEYS` path to a folder of account keys to import after init (default = "/keys/")

*Note: It is up to simulators to wire the clients together. The simplest way to do this is to start
a bootnode inside the simulator and specify it for new clients via the documented `HIVE_BOOTNODE`
environment variable. This is required to make simulators fully self contained, also enabling much
more complex networking scenarios not doable with forced fixed topologies.*

The simulation will be considered successful if and only if the exit code of the entrypoint script
is zero! Any output that the simulator generates will be saved to an appropriate log file in the `hive`
workspace folder and also echoed out to the console on `--loglevel=6`.

#### Reporting sub-results

It may happen that the setup/teardown cost of a simulation be large enough to warrant validating not
just one, but perhaps multiple invariants in the same test. Although the results for these subtests
could be reported in the log file, retrieving them would become unwieldy. As such, `hive` exposes a
special HTTP endpoint on its RESTful API that can add sub-results to a single simulation. The endpoint
resides at `/subresults` and has the following parameters:

 * `name`: textual name to report for the subtest
 * `success`: boolean flag (`true` or `false`) representing whether the subtest failed
 * `error`: textual details for the reason behind the subtest failing
 * `details`: structured JSON object containing arbitrary extra infos to surface

For example, doing a call to this endpoint with curl:

```
$ curl -sf -v -X POST -F 'details={"Preconditions failed": ["nonce 1 != 2", "balance 0"]}' \
  "$HIVE_SIMULATOR/subresults?name=Demo%20error&success=false&error=Something%20went%20wrong..."
```

will result a `subresults` field

```json
"subresults": [
  {
    "name": "Demo error",
    "success": false,
    "error": "Something went wrong...",
    "details": {
      "Preconditions failed": [
        "nonce 1 != 2",
        "balance 0"
      ]
    }
  }
]
```
### Closing notes

 * There is no constraint on how much time a simulation may run, but please be considerate.
 * The simulator doesn't have to terminate nodes itself, upon exit all resources are reclaimed.

# Generating test blockchains

`Hive` allows you to create rlp encoded blockchains for inclusion into sync tests 

## To generate a blockchain offline

`Hive` can be run with the `chainGenerate` setting to generate a blockchain according to specification
and then exit. The current version only generates blocks with empty transactions, but this will be
improved in the future to offer generation of chains that exhibit different characteristics for testing.

* `chainGenerate`   Bool    (false)  Tell Hive to generate a blockchain on the basis of a supplied genesis and terminate
* `chainLength`     Uint    (2)     The length of the chain to generate
* `chainConfig`     String              Reserved for future usage. Will allow Hive to generate test chains of different types
*	`chainOutputPath` String  (.)   Chain destination folder
*	`chainGenesis`    String  ("")  The path and filename to the source genesis.json
*	`chainBlockTime`  Uint    (30)    The desired block time in seconds. OBS: It's recommended to set this value to somwhere above 20s to keep the mining difficulty low.

For example   `hive --loglevel 6 --chainGenerate --chainLength 2 --chainOutputPath "C:\Ethereum\Path" --chainGenesis "C:\Ethereum\Path\Genesis.json" --chainBlockTime 30`

# Continuous integration

As mentioned in the beginning of this document, one of the major goals of `hive` is to support running
the validators, simulators and benchmarks as part of an Ethereum client's CI workflow. This is needed
to ensure a baseline quality for implementations, particularly because developers notoriously hate the
idea of having to do a manual testing step, heaven forbid having to install dependencies of "that"
language.

Providing detailed configuration description for multiple CI services is out of scope of this project,
but we did put together a document about configuring it for [`circleci`](https://circleci.com/), leaving
it up to the user to port it to other platforms. The single most important feature a CI service must
support for `hive` is docker containers, specifically allowing multiple ones concurrently.

## Integration via [circleci](https://circleci.com/)

Since `hive` is quite an unorthodox test harness, `circleci` has no chance of automatically inferring
any details on how it should be used. As such all configurations need to be manually specified via a
`circle.yml` YAML file placed into the repository root. You'll need to add three important sections
to this.

The first part is easy, we request a machine that has a docker service running:

```yaml
machine:
  services:
    - docker
```

Now comes the juicy part. Although we could just simply install `hive` and run it as the main test
step, the `circleci` service offers an interesting concept called *dependencies*. Users can define
and install various libraries, tools, etc. that will be persisted between test runs. This is a very
powerful feature as it allows caching all those non-changing dependencies between CI runs and only
download and rebuild those that have changed.

Since `hive` is both based on docker containers that may take a non-insignificant time to build, we'd
like to cache as much as possible and only rebuild what changed.

To this effect we will define a special folder we'd like to include in the `circleci` caches to collect 
all the docker images built by `hive`:

```yaml
dependencies:
  cache_directories:
    - "~/.docker" # Cache all docker images manually to avoid lengthy rebuilds
```

Of course we also need to tell `circleci` how to populate these caches and how to reload them into
the live test environment. To do that we'll add an additional sub-section to the top `dependencies`
section called `override`, which will do a couple things:

 * Import all the docker images from the cache into the locally running service
 * Install `hive`
 * Dry run `hive`, to rebuild all docker images, but not run any tests
 * Export all new docker images into the cache

```yaml
  override:
    # Restore all previously cached docker images
    - mkdir -p ~/.docker
    - for img in `ls ~/.docker`; do docker load -i ~/.docker/$img; done

    # Pull in and hive, and do a dry run
    - go get -u github.com/karalabe/hive
    - (cd ~/.go_workspace/src/github.com/karalabe/hive && hive --docker-noshell --client=NONE --test=. --sim=. --loglevel=6)

    # Cache all the docker images
    - for img in `docker images | grep -v "^<none>" | tail -n +2 | awk '{print $1}'`; do docker save $img > ~/.docker/`echo $img | tr '/' ':'`.tar; done
  ```

Although it may look a bit overwhelming, all the above code does is it loads the cache contents,
updates it with a freshly installed version of `hive`, and then pushes everything new back into
the cache for next time.

With `hive` installed and all optimisations and caches out of the way, the remaining step is to run
the actual continuous integration: build your project and invoke hive to test it. The first part is
implementation dependent, but for example `go-ethereum` has a simple `make geth` command for building
the client.

With the client built, we'll need to start `hive` from its repository root, specify the client image
we want to use (e.g. `--client=go-ethereum:local`), override any files in the image (i.e. inject our
freshly built binary `--override=$HOME/geth`) and run the whole suite (`--test=. --sim=.`).

*Note, as `circleci` seems unable to handle multiple docker containers embedded in one another, we'll
need to specify the `--docker-noshell` flag to omit `hive`'s outer shell container. This is fine as
we don't care about any junk generated at this point, `circleci` will just discard it after the test.*

```yaml
test:
  override:
    # Build Geth and move into a known folder
    - make geth
    - cp ./build/bin/geth $HOME/geth

    # Run hive and move all generated logs into the public artifacts folder
    - (cd ~/.go_workspace/src/github.com/karalabe/hive && hive --docker-noshell --client=go-ethereum:local --override=$HOME/geth --test=. --sim=.)
    - cp -r ~/.go_workspace/src/github.com/karalabe/hive/workspace/logs/* $CIRCLE_ARTIFACTS
```

If all went well, you should see `hive` assembled in the `circleci` dashboard (first run can take a
bit of time to cache all the tester images) and `hive` should be happily crunching
through its defined tests with your fresh client binary.

If you get stuck, you can always take a look at the [current live `circle.yml`](https://github.com/ethereum/go-ethereum/blob/develop/circle.yml)
file being used by the `go-ethereum` client.

# Trophies

If you find a bug in your client implementation due to this project, please be so
kind as to add it here to the trophy list. It could help prove that `hive` is indeed
a useful tool for validating Ethereum client implementations.

 * go-ethereum
   * Genesis chain config couldn't handle present but empty settings: [#2790](https://github.com/ethereum/go-ethereum/pull/2790)
   * Data race between remote block import and local block mining: [#2793](https://github.com/ethereum/go-ethereum/pull/2793)
   * Downloader didn't penalize incompatible forks hashly enough: [#2801](https://github.com/ethereum/go-ethereum/pull/2801)
 * Nethermind
   * Bug in p2p whith bonding nodes algorithm found by Hive: [#1894](https://github.com/NethermindEth/nethermind/pull/1894)

# Contributions

This project takes a different approach to code contributions than your usual FOSS project with well
ingrained maintainers and relatively few external contributors. It is an experiment. Whether it will
work out or not is for the future to decide.

We follow the [Collective Code Construction Contract (C4)](http://rfc.zeromq.org/spec:22/C4/), code
contribution model, as expanded and explained in [The ZeroMQ Process](https://hintjens.gitbooks.io/social-architecture/content/chapter4.html).
The core idea being that any patch that successfully solves an issue (bug/feature) and doesn't break
any existing code/contracts **must** be optimistically merged by maintainers. Followup patches may
be used to for additional polishes – and patches may even be outright reverted if they turn out to
have a negative impact – but no change must be rejected based on personal values.

Please consult the two C4 documents for details:

 * [Collective Code Construction Contract (C4)](http://rfc.zeromq.org/spec:22/C4/)
 * [The ZeroMQ Process](https://hintjens.gitbooks.io/social-architecture/content/chapter4.html)

# License

The `hive` project is licensed under the [GNU General Public License v3.0](http://www.gnu.org/licenses/gpl-3.0.en.html),
also included in our repository in the COPYING file.
