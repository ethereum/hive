# ---------------------------------------------------------------#
# This simulation runs the Ethereum python execution-spec-tests. #
# ---------------------------------------------------------------#

# 1) Create pyspec builder container.
FROM golang:1-alpine as builder
RUN apk add --update git ca-certificates gcc musl-dev linux-headers

# Build the pyspec simulator executable.
ADD . /pyspec
WORKDIR /pyspec
RUN go build -v .

# 2) Create the simulator container.
FROM alpine:latest as simulator
RUN apk add --update wget curl
ADD . /pyspec
WORKDIR /pyspec
COPY --from=builder /pyspec/pyspec .

# Download the latest fixture release.
RUN curl -s https://api.github.com/repos/ethereum/execution-spec-tests/releases/latest \
    | grep "browser_download_url" \
    | cut -d '"' -f 4 \
    | wget -qi -

# Extract the latest fixtures.
RUN tar -xzvf fixtures.tar.gz
RUN mv fixtures /fixtures

# Point to executable and test fixtures.
ENV TESTPATH /fixtures
ENTRYPOINT ["./pyspec"]