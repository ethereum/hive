# This validation runs various devp2p tests against the client

FROM golang:1-alpine



# Build a geth bootnode, regular node from the develop branch and unit test binary
RUN apk add --update git make gcc musl-dev curl jq linux-headers bash curl jq

# Get master branch of geth
ENV GO111MODULE=on

#RUN (git clone -b master --single-branch https://github.com/ethereum/go-ethereum /go/src/github.com/ethereum/go-ethereum)

#RUN (cd /go/src/github.com/ethereum/go-ethereum && GOPATH=/go go mod download )
#RUN (cd /go/src/github.com/ethereum/go-ethereum && GOPATH=/go go get -v -u all ./...)




# Build geth
#RUN (cd /go/src/github.com/ethereum/go-ethereum && GOPATH=/go make all)
#RUN (cd /go/src/github.com/ethereum/go-ethereum && GOPATH=/go go get)

# Add the docker client
# RUN (git clone -b master https://github.com/fsouza/go-dockerclient /go/src/github.com/fsouza/go-dockerclient)

# RUN (cd /go/src/github.com/fsouza/go-dockerclient && GOPATH=/go go get)


# RUN go get -u -v github.com/google/uuid


#  RUN go get -u -v github.com/VictoriaMetrics/fastcache
#  RUN go get -u -v github.com/aristanetworks/goarista/monotime
#  RUN go get -u -v github.com/davecgh/go-spew/spew
#  RUN go get -u -v github.com/deckarep/golang-set
#  RUN go get -u -v github.com/edsrzf/mmap-go
#RUN (cd / && GOPATH=/go go get github.com/fsouza/go-dockerclient)


# Extra packages
ENV GO111MODULE=off
RUN apk add --update libpcap-dev
RUN go get -u -v github.com/google/gopacket/pcap
RUN go get -u -v github.com/derekparker/delve/cmd/dlv
ENV GO111MODULE=on

# Add the local test stuff
ADD /devp2p/basic/devp2p_test.go /go/src/github.com/ethereum/hive/simulators/devp2p/basic/devp2p_test.go
ADD /devp2p/node.go  /go/src/github.com/ethereum/hive/simulators/devp2p/node.go
ADD /devp2p/udp.go  /go/src/github.com/ethereum/hive/simulators/devp2p/udp.go
ADD /devp2p/udpspoof.go  /go/src/github.com/ethereum/hive/simulators/devp2p/udpspoof.go
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

