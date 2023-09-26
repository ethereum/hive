FROM alpine

USER root

RUN apk add --no-cache socat
RUN apk add --no-cache jq 

COPY genesis.json /genesis.json
#TODO: check env var for mapper to use 
COPY mapper.jq /mapper.jq


CMD ["sh", "-c", "\
     socat -V > /version.txt & \
     jq -f /mapper.jq /genesis.json > /besugenesis.json; \
     cat /besugenesis.json; \
     socat -v TCP-LISTEN:8545,fork TCP:host.docker.internal:8545 & \
     socat -v TCP-LISTEN:8551,fork TCP:host.docker.internal:8551 & \
     socat TCP-LISTEN:30303,fork TCP:host.docker.internal:30303 & \
     socat UDP-LISTEN:30303,fork UDP:host.docker.internal:30303 & \
     wait"]

