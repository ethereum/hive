FROM golang:1-alpine
# For debugging etc
RUN apk add --update git make gcc musl-dev curl jq linux-headers bash  
ENV GO111MODULE=off
RUN go get -u -v github.com/derekparker/delve/cmd/dlv
ENV GO111MODULE=on

# Add the local test stuff
ADD /ethereum/sync/sync_test.go /go/src/github.com/ethereum/hive/simulators/ethereum/sync/sync_test.go
ADD common /go/src/github.com/ethereum/hive/simulators/common
# Add the chains to test with
ADD /ethereum/sync/simplechain /simplechain

# Use modules (temp fix until sims are sub modules)
RUN (cd /go/src/github.com/ethereum/hive/simulators/ && go mod init)


# Add the entry script
ADD /ethereum/sync/tests.sh /tests.sh
RUN chmod +x /tests.sh

EXPOSE 2345 8545 8546 8080 30303 30303/udp 

ENTRYPOINT ["/tests.sh"]

