FROM ethereum/client-go:latest

RUN apk add --update bash curl jq

RUN mkdir /hive-bin
COPY ./enode.sh /hive-bin/enode.sh
RUN chmod +x /hive-bin/enode.sh

COPY ./l1_genesis.json /host/l1_genesis.json
COPY ./start.sh /start.sh

RUN /usr/local/bin/geth console --exec 'console.log(admin.nodeInfo.name)' --maxpeers=0 --nodiscover --dev 2>/dev/null | head -1 >/version.txt

#EXPOSE 8545 8546 8547 8551 30303 30303/udp

ENTRYPOINT ["/start.sh"]