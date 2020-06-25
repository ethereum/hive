FROM ethereum/client-go:latest

ADD ./genesis.json /tmp/genesis.json

RUN geth --datadir data init /tmp/genesis.json
