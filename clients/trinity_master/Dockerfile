FROM python:3.6
# Set up code directory
RUN mkdir -p /usr/src/app
WORKDIR /usr/src/app

RUN \
  apt-get -y update                                        && \
  apt-get install -y bash  bc curl git make cmake netcat file      

RUN git clone --recursive https://github.com/ethereum/py-evm.git .
RUN pip install -e .[dev]  --no-cache-dir
RUN pip install -U trinity --no-cache-dir


RUN wget https://github.com/stedolan/jq/releases/download/jq-1.6/jq-linux64 -O /usr/local/bin/jq && \
    echo 'af986793a515d500ab2d35f8d2aecd656e764504b789b66d7e1a0b727a124c44  /usr/local/bin/jq' | sha256sum -c && \
    chmod +x /usr/local/bin/jq



ADD config.json /config.json
ADD mapper.jq /mapper.jq
ADD trinity.sh /trinity.sh
RUN chmod +x /trinity.sh
ADD enode.sh /enode.sh
RUN chmod +x /enode.sh
#debug genesis, overriden at runtime:
ADD genesis.json /genesis.json


EXPOSE  8545 8546 30303 30303/udp

RUN \
  echo "{}"                                                      \
  | jq ".+ {\"repo\":\"$(git config --get remote.origin.url)\"}" \
  | jq ".+ {\"branch\":\"$(git rev-parse --abbrev-ref HEAD)\"}"  \
  | jq ".+ {\"commit\":\"$(git rev-parse HEAD)\"}"               \
  > /version.json              


CMD ["/trinity.sh"]
