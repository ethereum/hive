# This simulation runs the EIP-8252 reorg state-retention test.
#
# Note: the build context of this image is the PARENT directory
# (simulators/ethereum), set via hive_context.txt, so the engine simulator's
# packages reused through the `replace => ../engine` directive are present in the
# build.
FROM golang:1.25-alpine as builder
ARG GOPROXY
ENV GOPROXY=${GOPROXY}

RUN apk add --update gcc musl-dev linux-headers

# Build the simulator executable.
ADD . /source
WORKDIR /source/reorg-retention
RUN go build -v .

# Build the simulator run container.
FROM alpine:latest
# Hive only forwards a fixed set of env vars to the simulator container, not
# HIVE_REORG_RETENTION_LONG, so the long/gated clients (reth, erigon) can be
# enabled only by baking it in at build time: `--sim.buildarg reorg_retention_long=1`.
ARG reorg_retention_long=""
ENV HIVE_REORG_RETENTION_LONG=${reorg_retention_long}
ADD . /source
WORKDIR /source/reorg-retention
COPY --from=builder /source/reorg-retention/reorg-retention .
ENTRYPOINT ["./reorg-retention"]
