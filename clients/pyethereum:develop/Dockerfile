# Docker container spec for building the develop branch of pythereum.
FROM alpine:latest

RUN \
  apk add --update bash jq git curl python-dev musl-dev gcc make openssl-dev   \
         bsd-compat-headers g++ autoconf automake pkgconfig libtool libffi-dev gmp-dev && \
  curl -sSf https://bootstrap.pypa.io/get-pip.py -o get-pip.py              && \
  python get-pip.py                                                         && \
  # XXX: Workaround for https://github.com/ethereum/pyethereum/issues/720
  pip install http://github.com/ethereum/serpent/tarball/develop#egg=serpent && \
  git clone --depth 1 https://github.com/ethereum/pyethapp                  && \
  cd pyethapp                                                               && \
  USE_PYETHEREUM_DEVELOP=1 python setup.py install                          && \
  echo "{}"                                                                    \
      | jq ".+ {\"repo\":\"$(git config --get remote.origin.url)\"}"           \
      | jq ".+ {\"branch\":\"$(git rev-parse --abbrev-ref HEAD)\"}"            \
      | jq ".+ {\"commit\":\"$(git rev-parse HEAD)\"}"                         \
      > /version.json                                                       && \
  apk del git make gcc g++ musl-dev curl pkgconfig libtool automake autoconf

# We can't delete /pyethapp--it's necessary for pyethapp to function.

# Inject the startup script
ADD pyethapp.sh /pyethapp.sh
RUN chmod +x /pyethapp.sh

# Add the config file

ADD config.yaml /root/.config/pyethapp/config.yaml

# Add the block import script

ADD importblock.py /importblock.py
RUN chmod +x /importblock.py

# Export the usual networking ports to allow outside access to the node
EXPOSE 8545 8546 30303

CMD ["/pyethapp.sh"]
