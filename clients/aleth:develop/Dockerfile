# Multistage Dockerfile for the Aleth Ethereum node.
# Build stage

# This can be run separately, as a standalone image, 
# to obtain a cache and save rebuilding all the time (during development)
#
# docker build --target builder -t aleth_base
#

FROM alpine:latest as builder
RUN apk add --no-cache \
		bash jq git \
        cmake \
        g++ \
        make \
        linux-headers \
        leveldb-dev --repository http://dl-cdn.alpinelinux.org/alpine/edge/testing/
RUN git clone --recursive https://github.com/ethereum/cpp-ethereum --branch develop --single-branch --depth 1        
RUN cd cpp-ethereum && echo "{}"                                               \
            | jq ".+ {\"repo\":\"$(git config --get remote.origin.url)\"}" \
            | jq ".+ {\"branch\":\"$(git rev-parse --abbrev-ref HEAD)\"}"  \
            | jq ".+ {\"commit\":\"$(git rev-parse HEAD)\"}"               \
            > /version.json 
WORKDIR /build
RUN cmake /cpp-ethereum -DCMAKE_INSTALL_PREFIX=/usr -DCMAKE_BUILD_TYPE=Release -DTOOLS=OFF -DTESTS=OFF -DHUNTER_JOBS_NUMBER=$(nproc)
RUN make -j $(nproc) && make install

# If the base is built separately, enable this and put 
# the section below in a separate dockerfile
#
#FROM aleth_base as builder
#

# Install stage
FROM alpine:latest
RUN apk add --no-cache \
        python3 \
        libstdc++ \
        leveldb --repository http://dl-cdn.alpinelinux.org/alpine/edge/testing/ \
        bash jq bc
COPY --from=builder /usr/bin/aleth /usr/bin/jsonrpcproxy.py /usr/bin/

# Our modified aleth.py, needed because Hive waits for a socket to 
# open before it considers the node to be 'up'. 
# However, the aleth.py script starts the http socket immediately, 
# but the backend ipc socket is not available yet. Hence, 
# the modified scrip waits until the ipc socket exists before starting the http service

ADD aleth.py /usr/bin
COPY --from=builder /version.json /

# Genesis template
ADD config.json /config.json

# Inject the startup script
ADD eth.sh /eth.sh
RUN chmod +x /eth.sh

# Export the usual networking ports to allow outside access to the node
EXPOSE 8545 30303

ENTRYPOINT ["/eth.sh"]
