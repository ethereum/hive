# This simulation runs Engine API tests.
FROM golang:1.25-alpine as builder
ARG GOPROXY
ENV GOPROXY=${GOPROXY}

RUN apk add --update gcc musl-dev linux-headers

# Build the simulator executable.
ADD . /source
WORKDIR /source
RUN go build -v .

# Build the simulator run container.
FROM alpine:latest
ADD . /source
WORKDIR /source
COPY --from=builder /source/engine .
# COPY --from=geth    /ethash /ethash
ENTRYPOINT ["./engine"]
