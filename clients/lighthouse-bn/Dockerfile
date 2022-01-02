FROM golang:1.16 AS builder
ARG repo=https://github.com/protolambda/eth2-testnet-genesis
ARG branch=master

RUN \
  git clone --depth 1 --branch $branch $repo && \
  cd eth2-testnet-genesis && go build .

FROM sigp/lighthouse:latest-unstable

COPY --from=builder /go/eth2-testnet-genesis/eth2-testnet-genesis /usr/local/bin

ADD make_config.sh /make_config.sh
RUN chmod +x /make_config.sh

ADD lighthouse_bn.sh /lighthouse_bn.sh
RUN chmod +x /lighthouse_bn.sh

# TODO: output accurate client version
RUN echo "latest" > /version.txt

ENTRYPOINT ["/lighthouse_bn.sh"]

