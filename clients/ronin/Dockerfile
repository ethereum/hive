ARG baseimage=kiendn60302/ronin
ARG tag=v2.6.2

FROM $baseimage:$tag as builder

FROM alpine:latest
RUN apk add --update bash curl jq
COPY --from=builder /usr/local/bin/ronin /usr/local/bin/ronin

# Generate the version.txt file.
RUN /usr/local/bin/ronin console --exec 'console.log(admin.nodeInfo.name)' --maxpeers=0 --nodiscover --dev 2>/dev/null | head -1 > /version.txt

# Inject the startup script.
ADD geth.sh /geth.sh
ADD mapper.jq /mapper.jq
RUN chmod +x /geth.sh

# Inject the enode id retriever script.
RUN mkdir /hive-bin
ADD enode.sh /hive-bin/enode.sh
RUN chmod +x /hive-bin/enode.sh

# Add a default genesis file.
ADD genesis.json /genesis.json

# Export the usual networking ports to allow outside access to the node
EXPOSE 8545 8546 8547 8551 30303 30303/udp

ENTRYPOINT ["/geth.sh"]
