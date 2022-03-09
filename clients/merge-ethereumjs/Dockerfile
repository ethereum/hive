FROM node:16-alpine as build
RUN apk update && apk add --no-cache bash git jq && rm -rf /var/cache/apk/*
RUN git clone --depth 1 https://github.com/ethereumjs/ethereumjs-monorepo.git && \
    (cd ethereumjs-monorepo && npm i)

ADD ethereumjs.sh /ethereumjs.sh
ADD genesis.json /genesis.json
ADD mapper.jq /mapper.jq
RUN chmod +x /ethereumjs.sh

RUN mkdir /ethereumjs-monorepo/packages/client/kiln/datadir

EXPOSE 8545

# Sanity check
RUN node /ethereumjs-monorepo/packages/client/dist/bin/cli.js --help

# NodeJS applications have a default memory limit of 2.5GB.
# This limit is bit tight, it is recommended to raise the limit
# since memory may spike during certain network conditions.
ENV NODE_OPTIONS=--max_old_space_size=6144

ENTRYPOINT ["/ethereumjs.sh"]