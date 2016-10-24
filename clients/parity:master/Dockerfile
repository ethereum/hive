# Docker container spec for building the master branch of parity.
FROM ubuntu:16.04

# Build parity on the fly and delete all build tools afterwards
RUN \
  # Install parity from sources
  apt-get -y update                                             && \
  apt-get install -y curl git make g++ gcc file binutils        && \
  curl -sSf https://static.rust-lang.org/rustup.sh -o rustup.sh && \
  sh rustup.sh --disable-sudo                             && \
	\
  git clone --depth 1 https://github.com/ethcore/parity && \
	cd parity && cargo build --release                    && \
	strip /parity/target/release/parity                   && \
	\
	cd / && cp /parity/target/release/parity /parity.bin && \
	rm -rf parity && mv /parity.bin /parity              && \
  \
  sh rustup.sh --uninstall --disable-sudo               && \
  apt-get remove -y curl git make g++ gcc file binutils && \
  rm -rf /root/.cargo

# Inject ethminer to support mining
RUN \
  apt-get install -y software-properties-common && \
  add-apt-repository ppa:ethereum/ethereum      && \
  apt-get -y update && apt-get install -y ethminer

# Make sure bash and jq is available for easier wrapper implementation
RUN apt-get install -y bash jq bc

# Inject the startup script and associated resources
ADD parity.sh /parity.sh
RUN chmod +x /parity.sh
ADD chain.json /chain.json

# Export the usual networking ports to allow outside access to the node
EXPOSE 8545 30303

ENTRYPOINT ["/parity.sh"]
