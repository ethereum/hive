FROM golang:1-alpine

# Add the local test stuff
ADD /devp2p/simple/devp2p_test.go /go/src/github.com/ethereum/hive/simulators/devp2p/simple/devp2p_test.go
ADD common /go/src/github.com/ethereum/hive/simulators/common
RUN (cd /go/src/github.com/ethereum/hive/simulators/ && go mod init)



# Add the entry script
ADD /devp2p/basic/tests.sh /tests.sh
RUN chmod +x /tests.sh

# For debugging simulators
ADD /devp2p/basic/genesis.json /genesis.json

EXPOSE 2345 8545 8546 8080 30303 30303/udp 
ENV GOPATH=/go

ENTRYPOINT ["/tests.sh"]

