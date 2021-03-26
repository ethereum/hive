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
standard library "testing" package. Be sure to check the [Go API reference of package
hivesim][hivesimref] for more information about writing simulators in Go.

Simulators are contained in the hive repository as independent Go modules. To create one,
first create a new subdirectory in `./simulators` and initialize a Go module there:

    mkdir ./simulators/ethereum/my-simulation
    cd ./simulators/ethereum/my-simulation
    go mod init github.com/ethereum/hive/simulators/ethereum/my-simulation
    go get github.com/ethereum/hive/hivesim@latest

Now create the simulator program file `my-simulation.go` and place the folowing content into:

```
package main

import "github.com/ethereum/hive/hivesim"

func main() {
	// initialize test
    suite := hivesim.Suite{
        Name:        "my-suite",
        Description: "This test suite performs some tests.",
    }
	// add a plain test (does not run a client) (comment to remove)
	suite.Add(hivesim.TestSpec{
		Name:        "the-test",
		Description: "This is the example test case.",
		Run: runMyTest,
	})
	// add a client test (starts the client)
    suite.Add(hivesim.ClientTestSpec{
        Name:        "the-test",
        Description: "This is the example test case.",
        Files: map[string]string{"/genesis.json": "genesis.json"},
        Run: runMyClientTest,
    })
	// runs the tests (exist on failures) (RunSuit does not exit) 
    hivesim.MustRunSuite(hivesim.New(), suite)
}


// template for the example test
func runMyTest(t *hivesim.T) {
	// write your test code here
}

// template for the example client test
func runMyClientTest(t *hivesim.T, c *hivesim.Client) {
	// write your test code here
}
```

The templates of the funcions are to be filled with the actuall
code. To do that you need to program the test routines using hivesim
package. The [documentation of hivesim][hivesimref] provides a lot of
useful information about the functions you can use.

### Setup a simulation container

The simulation runs in a docker container. For that we need to setup a
correct environment. As can be seen in the client test `Files:` part,
it requires a `genesis.json` file that specifies the genesis state of
the client. On example of `genesis.json` can be found in
`simulators/devp2p/eth/init/genesis.json`. Copy it to your current
`my-simulation` directory (as `cp
../../../simulators/devp2p/eth/init/genesis.json .`.

Your simulator directory must also contain a `Dockerfile` so that hive
can build the container:

```
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
```

### Running the simulation

Finaly, go back to the root of the repository (`cd ../../..`) and [launch the simulation]:

	./hive --sim my-simulation --client go-ethereum,openethereum

[launch the simulation]: [./overview.md#running-hive]

### Checking the results

The test results are going to be written to the standard output and
you can also review the results [on your server in case you run the
`hiveview`][hiveview]. If you set everythin as described, your test
should pass and you have your testing environment set up correctly.

[hiveview]: ./commandline.md#viewing-simulation-results-hiveview

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
    content-disposition: form-data; name=HIVE_CHAIN_ID

    8
    --boundary--
    content-disposition: form-data; name=file; filename="/genesis.json"

    {
      "difficulty": "0x20000",
      "gasLimit": "0xFFFFFFFF",
      ...
    }
    --boundary----

This request starts a client container. The request body must be encoded as multipart form data.

The `CLIENT` form field is required and specifies the client type that should be started.
It must match one of the client names returned by the `/clients` endpoint.

Other form fields, specifically those with a prefix of `HIVE_`, are passed to the client
entry point as environment variables. Please see the [client interface documentation] for
environment variables supported by Ethereum clients.

Form fields with a filename are copied into the client container as files.

Response:

    200 OK
    content-type: text/plain

    <container ID>@<IP address>@<MAC address>

#### Geting the enode URL of a running client

    GET /testsuite/{suite}/test/{test}/node/{container}

This request returns the enode URL of a running client.

Response:

    200 OK
    content-type: text/plain

    enode://1ba850b467b3b96eacdcb6c133d2c7907878794dbdfc114269c7f240d278594439f79975f87e43c45152072c9bd68f9311eb15fd37f1fd438812240e82de9ef9@172.17.0.3:30303

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
the network name is assigned the API client.

Response:

    200 OK

#### Removing a network

    DELETE /testsuite/{suite}/network/{network}

This request removes a network. Note: the request will fail if containers are still
connected to the network.

Response:

    200 OK

#### Connecting containers to a network

    POST /testsuite/{suite}/network/{network}/{container}

This request connects a client container to a network. You can use any client container ID
as the `container`. You can also use `"simulation"` as the container ID, in which case the
simulator container will be connected.

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

This returns the IP of a client container on the given network.

Response:

    200 OK
    content-type: text/plain

    172.22.0.2

[client interface documentation]: ./clients.md
[hivesimref]: https://pkg.go.dev/github.com/ethereum/hive/hivesim
[Overview]: ./overview.md
[Hive Commands]: ./commandline.md
[Simulators]: ./simulators.md
[Clients]: ./clients.md
