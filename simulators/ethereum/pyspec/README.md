# Hive Pyspec
## The Python Execution Spec Tests Simulator

This is a `simulator` for running the python [execution-spec-tests](https://github.com/ethereum/execution-spec-tests) within [`hive`](https://github.com/ethereum/hive), based on the `consensus` simulator. It differs mostly by using the `Engine API` to feed blocks into clients. 

It can be run from the `hive` root directory simply, e.g for `geth`:
```sh
./hive --client go-ethereum --sim ethereum/pyspec
```

The `pyspec simulator` uses the latest test fixtures from the
most recent execution-spec-tests [release](https://github.com/ethereum/execution-spec-tests/releases).

# Pyspec Source Files

## Dockerfile

The Dockerfile is responsible for building the image that runs the Ethereum python execution-spec-tests.

```docker
# 1) Create pyspec builder container
FROM golang:1-alpine as builder
RUN apk add --update git ca-certificates gcc musl-dev linux-headers

ADD . /pyspec
WORKDIR /pyspec
RUN go build -v .

# 2) Create the simulator run container.
FROM alpine:latest as simulator
ADD . /pyspec
WORKDIR /pyspec
COPY --from=builder /pyspec/pyspec .

RUN apk add --update wget curl
RUN curl -s https://api.github.com/repos/ethereum/execution-spec-tests/releases/latest \
    | grep "browser_download_url" \
    | cut -d '"' -f 4 \
    | wget -qi -

RUN tar -xzvf fixtures.tar.gz fixtures/
RUN mv fixtures /fixtures
RUN rm fixtures.tar.gz

ENV TESTPATH /fixtures
ENTRYPOINT ["./pyspec"]
```

The `pyspec` simulator is built using two separate containers:

1. The first container, `builder`, is built using the Golang Alpine image. It adds several required dependencies: `git`, `ca-certificates`, `gcc`, `musl-dev` &  `linux-headers`.
    - The Dockerfile then adds the `pyspec` source code to the container and the go build command is executed to build the `pyspec` executable.
2. The second container, `simulator`, is built using the latest Alpine image. It has `wget` & `curl` as dependencies for downloading the latest test fixtures.
    - It then adds the `pyspec` source code, similar to the first container. It also copies the previously built `pyspec` executable from first container.
    - Next, `fixtures.tar.gz` is downloaded from the most recent Ethereum `execution-spec-tests` release. The contents are extracted, removing the `tar` file afterwards. `fixtures/` is moved to the root of the `simulator` container: `~/fixtures`.
    - The next step is to set the `TESTPATH` environment variable to `/fixtures`. Afterwards the Dockerfile finally sets the `ENTRYPOINT` to `./pyspec` (its actual path: `~/pyspec/pyspec`), which means that when the container is run, the built `pyspec` simulator will be executed. Note that the `TESTPATH` environment variable is used within the executable source code.

## main.go

When the `pyspec` simulator is ran using `hive`, a`./pyspec` executable is built and ran within two seperate docker containers as previously mentioned. The `./pyspec` executable that is ran, is built using sevaral `Go` source files from the `pyspec` project root: `hive/simulators/ethereum/pyspec`. 

Lets discuss the `main.go` file which acts as the entry point to the overall `pyspec` simulator <- built executable <- program. Starting at the top of the program tree is the `main()` function:

```go
func main() {
	suite := hivesim.Suite{
		Name: "pyspec",
		Description: "insert witty description here",
    }
	suite.Add(hivesim.TestSpec{
		Name: "pyspec_fixture_runner",
		Description: "insert witty description here",
		Run:       fixtureRunner,
		AlwaysRun: true,
	})
	hivesim.MustRunSuite(hivesim.New(), suite)
}
```
At a high-level the `main()` function first sets up the test suite using the `hivesim.Suite` struct and adds a single test to the suite, named `pyspec_fixture_runner`. This is a `meta-test` which simply launches the actual client testing process within `hive`. Once the `hive` suite is setup, it is ran by calling the `MustRunSuite()` function, using the configured suite as input, from a package named `hivesim`. 

Wheh suite is ran via executable in the docker container (or otherwise) the `pyspec_fixture_runner` `hivesim.TestSpec` simply calls the function `fixtureRunner()`, starting the test suite.