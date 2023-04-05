FROM node:16-alpine as build
RUN apk update && apk add --no-cache bash git jq curl python3 gcc make g++ && rm -rf /var/cache/apk/*
RUN git clone --depth 1 -b develop-v7 https://github.com/ethereumjs/ethereumjs-monorepo.git && \
    (cd ethereumjs-monorepo && npm i)

RUN cd ethereumjs-monorepo/packages/client && npm ethereumjs --version > /version.txt

ADD ethereumjs.sh /ethereumjs.sh
ADD mapper.jq /mapper.jq
ADD jwtsecret /jwtsecret
RUN chmod +x /ethereumjs.sh

# Inject the enode id retriever script
RUN mkdir /hive-bin
ADD enode.sh /hive-bin/enode.sh
RUN chmod +x /hive-bin/enode.sh

ADD genesis.json /genesis.json

# Export the usual networking ports to allow outside access to the node
EXPOSE 8545 8546 8551 8547 30303 30303/udp

# NodeJS applications have a default memory limit of 2.5GB.
# This limit is bit tight, it is recommended to raise the limit
# since memory may spike during certain network conditions.
ENV NODE_OPTIONS=--max_old_space_size=6144

ENTRYPOINT ["/ethereumjs.sh"]
