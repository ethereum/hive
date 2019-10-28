# Docker container spec for building the master branch of parity.

FROM ubuntu:16.04
ARG branch=master
# Build parity on the fly and delete all build tools afterwards
RUN \
  # Install parity from sources
  apt-get -y update                                        && \
  apt-get install -y bash jq bc curl git make cmake g++ gcc file    \
  binutils pkg-config openssl libssl-dev libudev-dev       && \
  curl https://sh.rustup.rs -sSf | sh -s --  -y            && \
  export PATH="$HOME/.cargo/bin:$PATH"                     && \
  git clone --depth 1 --branch $branch https://github.com/paritytech/parity && \
	cd parity && cargo build --release                    && \
  echo "{}"                                                      \
  | jq ".+ {\"repo\":\"$(git config --get remote.origin.url)\"}" \
  | jq ".+ {\"branch\":\"$(git rev-parse --abbrev-ref HEAD)\"}"  \
  | jq ".+ {\"commit\":\"$(git rev-parse HEAD)\"}"               \
  > /version.json                                       && \
	strip /parity/target/release/parity                   && \
	\
	cd / && cp /parity/target/release/parity /parity.bin && \
	rm -rf parity && mv /parity.bin /parity              && \
  apt-get remove -y  git make cmake g++ gcc file binutils pkg-config openssl libssl-dev && \
  rm -rf /root/.cargo

# Inject ethminer to support mining
#RUN \
#  apt-get install -y software-properties-common && \
#  add-apt-repository ppa:ethereum/ethereum      && \
#  apt-get -y update && apt-get install -y ethminer

# Inject the startup script and associated resources
ADD parity.sh /parity.sh
RUN chmod +x /parity.sh
ADD chain.json /chain.json

# Inject the enode id retriever script
ADD enode.sh /enode.sh
RUN chmod +x /enode.sh 

# Export the usual networking ports to allow outside access to the node
EXPOSE 8545 8547 30303 30303/udp

ADD genesis.json /genesis.json


ENTRYPOINT ["/parity.sh"]
