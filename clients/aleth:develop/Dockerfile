# Multistage Dockerfile for the Aleth Ethereum node.

# Build stage
FROM alpine:latest as builder
RUN apk add --no-cache \
        cmake \
        g++ \
        make \
        linux-headers \
        leveldb-dev --repository http://dl-cdn.alpinelinux.org/alpine/edge/testing/ \
        bash jq bc git
RUN git clone --recursive https://github.com/ethereum/cpp-ethereum --branch develop --single-branch --depth 1        
RUN cd cpp-ethereum && echo "{}"                                               \
            | jq ".+ {\"repo\":\"$(git config --get remote.origin.url)\"}" \
            | jq ".+ {\"branch\":\"$(git rev-parse --abbrev-ref HEAD)\"}"  \
            | jq ".+ {\"commit\":\"$(git rev-parse HEAD)\"}"               \
            > /version.json 
WORKDIR /build
RUN cmake /cpp-ethereum -DCMAKE_INSTALL_PREFIX=/usr -DCMAKE_BUILD_TYPE=Release -DTOOLS=OFF -DTESTS=OFF -DHUNTER_JOBS_NUMBER=$(nproc)
RUN make -j $(nproc) && make install

# Install stage
FROM alpine:latest
RUN apk add --no-cache \
        python3 \
        libstdc++ \
        leveldb --repository http://dl-cdn.alpinelinux.org/alpine/edge/testing/
COPY --from=builder /usr/bin/aleth /source/scripts/aleth.py /source/scripts/jsonrpcproxy.py /usr/bin/
COPY --from=builder /version.json /

# Genesis template
ADD config.json /config.json
# Inject the startup script
ADD eth.sh /eth.sh
RUN chmod +x /eth.sh
# Export the usual networking ports to allow outside access to the node
EXPOSE 8545 30303

ENTRYPOINT ["/eth.sh"]
