## Hive Simulator Programming

This guide explains how to write hive simulators.

A simulator is a program written against the HTTP-based simulation API provided by hive.
Simulators can be written in any programming language as long as they are packaged using
docker.

Simulators live in the `simulators/` directory of the hive repository. There is a
dedicated sub-directory for every simulator. When hive runs a simulation, it first builds
an image using `docker build` in the simulator directory, using the Dockerfile. The image
must contain all resources needed for testing.

When hive starts the simulator container entry point, the `HIVE_SIMULATOR` environment
variable is set to the URL of the API server.

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

This returns a JSON array of client names available to the simulation run.

Response

    200 OK
    content-type: application/json

    ["go-ethereum","besu"]

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

[client interface documentation]: ./client.md
