FROM ethereum/client-go:release-1.10 AS base

RUN apk add --no-cache jq yarn git linux-headers gcompat
RUN /usr/local/bin/geth console --exec 'console.log(admin.nodeInfo.name)' --maxpeers=0 --nodiscover --dev 2>/dev/null | head -1 > /version.txt


ENV GLIBC_KEY=https://alpine-pkgs.sgerrand.com/sgerrand.rsa.pub
ENV GLIBC_KEY_FILE=/etc/apk/keys/sgerrand.rsa.pub
ENV GLIBC_RELEASE=https://github.com/sgerrand/alpine-pkg-glibc/releases/download/2.35-r0/glibc-2.35-r0.apk
RUN wget -q -O ${GLIBC_KEY_FILE} ${GLIBC_KEY} \
    && wget -O glibc.apk ${GLIBC_RELEASE} \
    && apk add glibc.apk --force

# copy forge
FROM ghcr.io/foundry-rs/foundry:nightly-64fe4acc97e6d76551cea7598c201f05ecd65639 as foundry
FROM base
COPY --from=foundry /usr/local/bin/forge /usr/local/bin/forge

ARG branch=develop
# Git clone Optimism monorepo
RUN git clone -b $branch https://github.com/ethereum-optimism/optimism /hive/optimism

# download submodules, so container does not have to at startup
RUN cd /hive/optimism && git submodule init && git submodule update


# Defer until L1 rpc server is up; called from devnet.go
ADD deploy.sh /hive-bin/deploy.sh
RUN chmod +x /hive-bin/deploy.sh

# Helper scripts; called from devnet.go
COPY scripts /hive-bin
RUN chmod +x /hive-bin/cat.sh

VOLUME ["/db"]

EXPOSE 8545/tcp
EXPOSE 8546/tcp

ENTRYPOINT ["/bin/sh", "/entrypoint.sh"]
