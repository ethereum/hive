### Build Erigon Locally:
### Requires a copy of erigon/ -> hive/clients/erigon/

## Builder stage: Compiles erigon from a local directory
FROM golang:1-alpine as builder

# Default local client path: clients/erigon/<erigon>
ARG local_path=erigon
COPY $local_path erigon

RUN apk add bash build-base ca-certificates git \
    && cd erigon && make erigon \
    && cp build/bin/erigon /usr/local/bin/erigon

## Final stage: Sets up the environment for running erigon
FROM alpine:latest

# Copy compiled binary from builder
COPY --from=builder /usr/local/bin/erigon /usr/local/bin/

RUN apk add --no-cache bash curl jq libstdc++ libgcc

# Create version.txt
RUN erigon --version | sed -e 's/erigon version \(.*\)/\1/' > /version.txt

# Add genesis mapper script, startup script, and enode URL retriever script
COPY genesis.json /genesis.json
COPY mapper.jq /mapper.jq
COPY erigon.sh /erigon.sh
COPY enode.sh /hive-bin/enode.sh

# Set execute permissions for scripts
RUN chmod +x /erigon.sh /hive-bin/enode.sh

# Expose networking ports
EXPOSE 8545 8546 8551 30303 30303/udp

ENTRYPOINT ["/erigon.sh"]
