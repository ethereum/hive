FROM ethereumoptimism/ci-builder:latest

RUN npm install http-server -g

ADD install_foundry.sh /install_foundry.sh
RUN chmod +x /install_foundry.sh && /bin/bash /install_foundry.sh

ARG branch=develop
# Git clone Optimism monorepo
RUN git clone -b $branch https://github.com/ethereum-optimism/optimism /hive/optimism

RUN cd /hive/optimism && git rev-parse HEAD > /version.txt

# download submodules, so container does not have to at startup
RUN cd /hive/optimism && git submodule init && git submodule update

# build the monorepo typescript part
RUN cd /hive/optimism && make build-ts

ADD entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

# Helper scripts; called from devnet.go
COPY scripts /hive-bin
RUN chmod +x /hive-bin/*

EXPOSE 8545/tcp

WORKDIR /hive/optimism

ENTRYPOINT ["/bin/bash", "/entrypoint.sh"]
