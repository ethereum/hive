# Docker container spec for building the develop branch of cpp-ethereum.
#
# The build process it potentially longer running but every effort was made to
# produce a very minimalistic container that can be reused many times without
# needing to constantly rebuild.
FROM alpine:latest

# Since we need to build some libs, set the flags globally
ENV CFLAGS   '-fPIC'
ENV CPPFLAGS '-fPIC'
ENV LDFLAGS  '-fPIC'

# Build cpp-ethereum on the fly and delete all build tools afterwards
RUN \
  # Install any dependencies of cpp-ethereum
  apk add --update sed git make cmake gcc g++ musl-dev jsoncpp-dev curl-dev boost-dev libmicrohttpd-dev                                      && \
  apk add --update crypto++-dev leveldb-dev --update-cache --repository http://dl-cdn.alpinelinux.org/alpine/edge/testing/ --allow-untrusted && \
  \
  # Install libraries missing from Alpine
  wget http://prdownloads.sourceforge.net/argtable/argtable2-13.tar.gz && tar -zxvf argtable2-13.tar.gz                   && \
  cd argtable2-13 && mkdir build && cd build && cmake .. && make install && cp /argtable2-13/src/argtable2.h /usr/include && \
  cd / && rm -rf argtable2-13                                                                                             && \
  \
  git clone --depth 1 git://github.com/cinemast/libjson-rpc-cpp.git                                                 && \
  sed -i -e s/\<string\>/\<cstring\>/g /libjson-rpc-cpp/src/jsonrpccpp/server/connectors/unixdomainsocketserver.cpp && \
  cd libjson-rpc-cpp && mkdir build && cd build && cmake .. && make -j install                                      && \
  cd / && rm -rf libjson-rpc-cpp                                                                                    && \
  \
  git clone --depth 1 git://github.com/miniupnp/miniupnp.git && \
  cd miniupnp/miniupnpc && make -j && make install           && \
  cd / && rm -rf miniupnp                                    && \
  \
  # Install cpp-ethereum itself, with a bit of a hack to fix some includes
  git clone --depth 1 --branch develop https://github.com/ethereum/cpp-ethereum                                     && \
  sed -i -e s/\<sys\\/poll.h\>/\<poll.h\>/g /usr/include/boost/asio/detail/socket_types.hpp                         && \
  cd cpp-ethereum && mkdir build && cd build && cmake -DETHASHCL=NO -DTESTS=NO -DSOLIDITY=NO .. && make -j4 install && \
  rm -rf cpp-ethereum                                                                                               && \
  \
  # Clean up all the dependency crap
  apk add --update jsoncpp libcurl boost-random boost-regex boost-filesystem boost-thread libmicrohttpd crypto++ leveldb && \
  apk del sed git make cmake gcc g++ musl-dev jsoncpp-dev curl-dev boost-dev libmicrohttpd-dev crypto++-dev leveldb-dev  && \
  rm -rf /var/cache/apk/*

# Make sure bash and jq is available for easier wrapper implementation
RUN apk add --update bash jq

# Inject the startup script
ADD eth.sh /eth.sh
RUN chmod +x /eth.sh

# Export the usual networking ports to allow outside access to the node
EXPOSE 8545 30303

ENTRYPOINT ["eth"]
