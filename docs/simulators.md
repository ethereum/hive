[Overview] | [Hive Commands] | [Simulators] | [Clients]

## Hive Simulator Programming

This guide explains how to write hive simulators.

A simulator is a program written against the HTTP-based simulation API provided by hive.
Simulators can be written in any programming language as long as they are packaged using
docker.

Simulators live in the `simulators/` directory of the hive repository. There is a
dedicated sub-directory for every simulator. When hive runs a simulation, it first builds
an image using `docker build` in the simulator directory, using the Dockerfile. The image
must contain all resources needed for testing.

When the simulator container entry point runs, the `HIVE_SIMULATOR` environment variable
is set to the URL of the API server.

The simulation API assumes a certain data model, and this model dictates how the API can
be used. In order to do anything with the API, the simulator must first request the start
of a *test suite* and remembers its ID. Test suites have a name and description assigned
by the simulator. All other resources provided by the API are scoped to the test suite and
are kept until the simulator ends the suite.

Next, the simulator can start *test cases* on the suite. Test cases are named and also
have an ID assigned by the API server. Multiple test cases may be running at any time
within a single suite. Note that test suites do not have an overall pass / fail status,
the only way to signal failure is with a test. At least one test case must be started for
a suite, otherwise no results can be reported.

Within the context of a test case, client containers may be started. Clients are
associated with the test and are shut down automatically when the test that started them
ends. If many tests are to be executed against a single client, it is good practice to
create a dedicated 'client launch' test just for starting the client, and then signal the
results of the other tests as individual test cases.

The simulator must report the results of all running test cases before ending the test
suite.

## Writing Simulators in Go

While simulators may be written in any language (they're just docker containers after
all), hive provides a Go library that wraps the simulation API in a way that resembles the
standard library "testing" package. Be sure to check the Go API reference of [package
hivesim] for more information about writing simulators in Go.

Simulators are contained in the hive repository as independent Go modules. To create one,
first create a new subdirectory in `./simulators` and initialize a Go module there:

    mkdir ./simulators/ethereum/my-simulation
    cd ./simulators/ethereum/my-simulation
    go mod init github.com/ethereum/hive/simulators/ethereum/my-simulation
    go get github.com/ethereum/hive/hivesim@latest

Now create the simulator program file `my-simulation.go`.

    package main

    import "github.com/ethereum/hive/hivesim"

    func main() {
        suite := hivesim.Suite{
            Name:        "my-suite",
            Description: "This test suite performs some tests.",
        }
        // add a plain test (does not run a client)
        suite.Add(hivesim.TestSpec{
            Name:        "the-test",
            Description: "This is an example test case.",
            Run: runMyTest,
        })
        // add a client test (starts the client)
        suite.Add(hivesim.ClientTestSpec{
            Name:        "the-test-2",
            Description: "This is an example test case.",
            Files: map[string]string{"/genesis.json": "genesis.json"},
            Run: runMyClientTest,
        })

        // Run the tests. This waits until all tests of the suite
        // have executed.
        hivesim.MustRunSuite(hivesim.New(), suite)
    }


    func runMyTest(t *hivesim.T) {
        // write your test code here
    }

    func runMyClientTest(t *hivesim.T, c *hivesim.Client) {
        // write your test code here
    }

### Creating the Dockerfile

The simulator needs to have a Dockerfile in order to run.

As can be seen in the client test `Files:` part, the simulation requires a `genesis.json`
file that specifies the genesis state of the client. An example of `genesis.json` can be
found in the `simulators/devp2p/eth/init/` directory. You can copy an existing genesis
block or create your own. Make sure to add all support files to container in the
Dockerfile. The Dockerfile might look like this:

    FROM golang:1-alpine AS builder
    RUN apk --no-cache add gcc musl-dev linux-headers
    ADD . /source
    WORKDIR /source
    RUN go build -o ./sim .

    # Build the runner container.
    FROM alpine:latest
    ADD . /
    COPY --from=builder /source/sim /
    ENTRYPOINT ["./sim"]

You can test this build by running `docker build .` in the simulator directory.

### Running the simulation

Finally, go back to the root of the repository (`cd ../../..`) and run the simulation.

    ./hive --sim my-simulation --client go-ethereum,openethereum

You can check the results using [hiveview].

## Simulation API Reference

This section lists all HTTP endpoints provided by the simulation API.

### Suite and Test Case Endpoints

#### Creating a test suite

    POST /testsuite
    content-type: application/x-www-form-urlencoded

    name=test-suite-name&description=this%20suite%20does%20...

This request signals the start of a test suite. The API responds with a test suite ID.

    200 OK
    content-type: text/plain

    1

#### Ending a test suite

    DELETE /testsuite/{suite}

This request ends a test suite. The simulator must end all running test cases before
ending the test suite.

Response:

    200 OK

#### Creating a test case

    POST /testsuite/{suite}/test
    content-type: application/x-www-form-urlencoded

The API responds with a test case ID.

    200 OK
    content-type: text/plain

    2

#### Ending a test case

    POST /testsuite/{suite}/test/{test}
    content-type: application/x-www-form-urlencoded

    summaryresult=%7B%22pass%22%3Atrue%2C%22details%22%3A%22this%20is%20the%20test%20output%22%7D

This request reports the result of a test case. The request body is a form submission
containing a single field `summaryresult`. The test result is a JSON object of the form:

    {"pass": true/false, "details": "text..."}

Response:

    200 OK

### Working with clients

#### Getting available client types

    GET /clients

This returns a JSON array of client definitions available to the simulation run.
Clients have a `name`, `version`, and `meta` for metadata as defined
in the [client interface documentation].

Response

    200 OK
    content-type: application/json

    [
      {
        "name": "go-ethereum",
        "version": "Geth/v1.10.0-unstable-8e547eec-20210224/linux-amd64/go1.16",
        "meta": {
          "roles": [
            "eth1"
          ]
        }
      },
      {
        "name": "besu",
        "version": "besu/v21.1.1-dev-f1c74ed2/linux-x86_64/oracle_openjdk-java-11",
        "meta": {
          "roles": [
            "eth1"
          ]
        }
      }
    ]

#### Starting a client container

    POST /testsuite/{suite}/test/{test}/node
    content-type: multipart/form-data; boundary=--boundary--

    --boundary--
    content-disposition: form-data; name=CLIENT

    go-ethereum
    --boundary--
    content-disposition: form-data; name=NETWORKS

    network1,network2
    --boundary--
    content-disposition: form-data; name=HIVE_CHAIN_ID

    8
    --boundary--
    content-disposition: form-data; name=/genesis.json; filename="genesis.json"

    {
      "difficulty": "0x20000",
      "gasLimit": "0xFFFFFFFF",
      ...
    }
    --boundary----

This request starts a client container. The request body must be encoded as multipart form data.

The `CLIENT` form parameter is required and specifies the client type that should be
started. It must match one of the client names returned by the `/clients` endpoint.

The `NETWORKS` form parameter is optional and configures networks to which the client will
be connected before it starts to run. Network names are supplied as a comma-separated
list. The client container will not be created if any of the given networks doesn't exist.

Other form parameters, specifically those with a prefix of `HIVE_`, are passed to the
client entry point as environment variables. Please see the [client interface
documentation] for environment variables supported by Ethereum clients.

Parameters with a filename are copied into the client container as files. Note: the
**parameter name** is used as the destination file name. The 'filename' submitted in the
form is ignored. This is because multipart/form-data does not support specifying directory
components in 'filename'.

Response:

    200 OK
    content-type: text/plain

    <container ID>@<IP address>@<MAC address>

#### Geting client information

    GET /testsuite/{suite}/test/{test}/node/{container}

This request returns basic information about a running client.

Response:

    200 OK
    content-type: application/json

    {"id":"abcdef1234","name":"go-ethereum_latest"}

#### Running client scripts

    POST /testsuite/{suite}/test/{test}/node/{container}/exec
    content-type: application/json

    {
      "command": ["my-script", "arg1"]
    }

This request invokes a script in the client container. The script must be present in the
client container's filesystem in the `/hive-bin` directory.

Response:

    200 OK
    content-type: application/json

    {
      "exitCode": 0,
      "stdout": "output",
      "stderr": "error output"
    }

#### Stopping a client

    DELETE /testsuite/{suite}/test/{test}/node/{container}

This terminates the given client container immediately. Using this endpoint is usually not
required because all clients associated with a test will be shut down when the test ends.

Response:

    200 OK

### Networks

#### Creating a network

    POST /testsuite/{suite}/network/{network}

This request creates a network. Unlike with other APIs, networks do not have IDs. Instead,
the network name is assigned by the simulator.

Response:

    200 OK

#### Removing a network

    DELETE /testsuite/{suite}/network/{network}

This request removes a network. Note: the request will fail if any containers are still
connected to the network.

Response:

    200 OK

#### Connecting containers to a network

    POST /testsuite/{suite}/network/{network}/{container}

This request connects a client container to a network. You can use any client container ID
as the `container`. You can also use `"simulation"` as the container ID, in which case the
container running the simulator will be connected.

Response:

    200 OK

#### Disconnecting a container from a network

    DELETE /testsuite/{suite}/network/{network}/{container}

This request disconnects a container from a network. As with the connect request, use any
client container ID or `"simulation"` as the `container` value.

Response:

    200 OK

#### Getting the client IP

    GET /testsuite/{suite}/network/{network}/{container}

This returns the IP of a container on the given network.

Response:

    200 OK
    content-type: text/plain

    172.22.0.2

[client interface documentation]: ./clients.md
[package hivesim]: https://pkg.go.dev/github.com/ethereum/hive/hivesim
[launch the simulation]: ./overview.md#running-hive
[hiveview]: ./commandline.md#viewing-simulation-results-hiveview
[Overview]: ./overview.md
[Hive Commands]: ./commandline.md
[Simulators]: ./simulators.md
[Clients]: ./clients.md
