# Docker container spec for building the master branch of parity.
FROM ethcore/parity

#centos:latest
#
## Build parity on the fly and delete all build tools afterwards
#RUN \
#  yum -y update                                                 && \
#  yum install -y git make gcc-c++ gcc file binutils             && \
#  curl -sSf https://static.rust-lang.org/rustup.sh -o rustup.sh && \
#  sh rustup.sh -s -- --disable-sudo && \
#	\
#  git clone --depth 1 https://github.com/ethcore/parity && \
#	cd parity && cargo build --release                    && \
#	strip /parity/target/release/parity                   && \
#	\
#	cd / && cp /parity/target/release/parity /parity.bin && \
#	rm -rf parity && mv /parity.bin /parity              && \
#  \
#  yum remove git make gcc-c++ gcc file binutils && \
#  rustup.sh --uninstall

# Make sure bash and jq is available for easier wrapper implementation
#RUN yum install -y bash jq
RUN cp /build/parity/target/release/parity /parity
RUN apt-get install jq bc

# Inject the startup script and associated resources
ADD parity.sh /parity.sh
RUN chmod +x /parity.sh
ADD chain.json /chain.json

# Export the usual networking ports to allow outside access to the node
EXPOSE 8545 30303

ENTRYPOINT ["/parity.sh"]
