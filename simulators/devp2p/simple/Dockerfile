FROM ethereum/client-go:latest

ADD ./genesis.json /tmp/genesis.json

RUN geth --datadir data init /tmp/genesis.json

EXPOSE 8545 8546 8547 30303 30303/udp
ENTRYPOINT ["/usr/local/bin/geth", "--networkid=15"]
