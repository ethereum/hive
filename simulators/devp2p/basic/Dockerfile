# This validation runs various devp2p tests against the client

FROM golang:1-alpine



# Build a geth bootnode, regular node from the develop branch and unit test binary
RUN apk add --update git make gcc musl-dev curl jq linux-headers

# Get master branch of geth
RUN (git clone -b master --single-branch https://github.com/ethereum/go-ethereum /go/src/github.com/ethereum/go-ethereum)
RUN (cd /go/src/github.com/ethereum/go-ethereum )

# Build geth
RUN (cd /go/src/github.com/ethereum/go-ethereum && GOPATH=/go make all)
RUN (cd /go/src/github.com/ethereum/go-ethereum && GOPATH=/go go get)

# Add the docker client
RUN (git clone -b master https://github.com/fsouza/go-dockerclient /go/src/github.com/fsouza/go-dockerclient)

RUN (cd /go/src/github.com/fsouza/go-dockerclient && GOPATH=/go go get)

RUN go get -u -v github.com/derekparker/delve/cmd/dlv

#RUN (cd / && GOPATH=/go go get github.com/fsouza/go-dockerclient)

# Add the local test stuff
ADD devp2p_test.go /tests/simulators/devp2p/basic/devp2p_test.go
ADD node.go /tests/simulators/devp2p/basic/node.go
ADD udp.go /tests/simulators/devp2p/basic/udp.go


#go test -c -o means compile the tests (-c) and rename (-o) to 
#RUN (cd /go/src/github.com/frankszendzielarz/hive/simulators/devp2p/basic/ && GOPATH=/go go test -gcflags='all=-N -l' -c -o /go/src/github.com/frankszendzielarz/hive/simulators/devp2p/basic/devp2p.test)


# Add shell
RUN apk add --update bash curl jq
# Add the entry script
ADD tests.sh /tests.sh
#mandatory init file
ADD genesis.json /genesis.json

EXPOSE 2345 8545 8546 8080 30303 30303/udp 

ENTRYPOINT ["/tests.sh"]

