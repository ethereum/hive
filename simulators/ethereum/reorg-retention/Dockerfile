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
ADD . /source
WORKDIR /source/reorg-retention
COPY --from=builder /source/reorg-retention/reorg-retention .
ENTRYPOINT ["./reorg-retention"]
