FROM golang:1-alpine

# Build a geth bootnode, regular node from the develop branch and unit test binary
RUN apk add --update git make gcc musl-dev curl linux-headers

# ADD contractABI.json /contractABI.json
# RUN (cd / && /abigen -abi /contractABI.json -pkg main -type TestContract -out /ABI_generated.go)
# RUN (cd / && GOPATH=/go go test -c -o /rpc.test)

# Cleanup dependencies
# RUN apk del git make gcc musl-dev linux-headers
# RUN (rm -rf /var/cache/apk/* && rm -rf /go)

# Each node has its own key with 123.45 preallocated ether in the genesis block
RUN mkdir /keystores1 /keystores2 /keystores3 /keystores4 /keystores5
ADD /ethereum/rpc/eth/keys/cf49fda3be353c69b41ed96333cd24302da4556f /keystores1/
ADD /ethereum/rpc/eth/keys/b97de4b8c857e4f6bc354f226dc3249aaee49209 /keystores2/
ADD /ethereum/rpc/eth/keys/87da6a8c6e9eff15d703fc2773e32f6af8dbe301 /keystores3/
ADD /ethereum/rpc/eth/keys/0161e041aad467a890839d5b08b138c1e6373072 /keystores4/
ADD /ethereum/rpc/eth/keys/c5065c9eeebe6df2c2284d046bfc906501846c51 /keystores5/

# Build the simulator.
ADD /ethereum/rpc/eth /source
WORKDIR /source
RUN go build -v

ENTRYPOINT ["./eth"]
