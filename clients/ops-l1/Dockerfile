FROM ethereum/client-go:v1.10.16 AS base

RUN apk add --no-cache jq yarn git linux-headers gcompat
RUN /usr/local/bin/geth console --exec 'console.log(admin.nodeInfo.name)' --maxpeers=0 --nodiscover --dev 2>/dev/null | head -1 > /version.txt
ENV GLIBC_KEY=https://alpine-pkgs.sgerrand.com/sgerrand.rsa.pub
ENV GLIBC_KEY_FILE=/etc/apk/keys/sgerrand.rsa.pub
ENV GLIBC_RELEASE=https://github.com/sgerrand/alpine-pkg-glibc/releases/download/2.35-r0/glibc-2.35-r0.apk
RUN wget -q -O ${GLIBC_KEY_FILE} ${GLIBC_KEY} \
    && wget -O glibc.apk ${GLIBC_RELEASE} \
    && apk add glibc.apk --force

# copy forge
FROM ghcr.io/foundry-rs/foundry:latest as foundry
FROM base
COPY --from=foundry /usr/local/bin/forge /usr/local/bin/forge

# Regenerate the L1 genesis file
RUN mkdir /hive
ADD genesis.json /genesis.json

# Inject the startup script
ADD entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

# Deploy contracts using Hardhat
RUN git clone https://github.com/ethereum-optimism/optimistic-specs /opt
RUN cd /opt && git submodule init && git submodule update
RUN cp -r /opt/packages/contracts /hive/contracts
# Defer until L1 rpc server is up; called from devnet.go
ADD deploy.sh /hive-bin/deploy.sh
RUN chmod +x /hive-bin/deploy.sh

# Helper scripts; called from devnet.go
ADD cat.sh /hive-bin/cat.sh
RUN chmod +x /hive-bin/cat.sh

VOLUME ["/db"]

EXPOSE 8545/tcp
EXPOSE 8546/tcp

ENTRYPOINT ["/bin/sh", "/entrypoint.sh"]
