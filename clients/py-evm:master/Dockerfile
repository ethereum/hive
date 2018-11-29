# Docker container spec for building the develop branch of pythereum.
FROM alpine:latest

RUN \
    apk add --update bash jq git curl python3 python3-dev musl-dev gcc make openssl-dev \
        bsd-compat-headers g++ autoconf automake pkgconfig libtool libffi-dev gmp-dev \
        linux-headers libxml2-dev libxslt-dev                                   && \
    apk --repository http://dl-3.alpinelinux.org/alpine/edge/testing/ --update add leveldb leveldb-dev && \
    python3 -m ensurepip                                                        && \
    pip3 install --upgrade pip                                                  && \
    pip3 install virtualenv setuptools                                          && \
    git clone --recursive https://github.com/ethereum/py-evm.git                && \
    virtualenv -p python3 venv                                                  && \
    . venv/bin/activate                                                         && \
    cd py-evm                                                                   && \
    pip3 install -e .[dev]                                                      && \
    echo "{}"                                                                      \
        | jq ".+ {\"repo\":\"$(git config --get remote.origin.url)\"}"             \
        | jq ".+ {\"branch\":\"$(git rev-parse --abbrev-ref HEAD)\"}"              \
        | jq ".+ {\"commit\":\"$(git rev-parse HEAD)\"}"                           \
        > /version.json                                                         && \
    cp ../venv/bin/trinity /trinity                                             && \
    apk del git make gcc g++ musl-dev curl pkgconfig libtool automake autoconf  && \
    rm -rf /var/cache/apk/*

# Inject the startup script
ADD trinity.sh /trinity.sh
RUN chmod +x /trinity.sh

# Export the usual networking ports to allow outside access to the node
EXPOSE 8545 8546 30303

CMD ["/trinity.sh"]
