# Docker container spec for building the develop branch of pythereum.
FROM ubuntu:16.04

RUN \
  apt-get -y update                                                         && \
  apt-get -y install git curl python-dev libssl-dev build-essential            \
                     automake pkg-config libtool libffi-dev libgmp-dev      && \
  curl -sSf https://bootstrap.pypa.io/get-pip.py -o get-pip.py              && \
  python get-pip.py &&                                                         \
  git clone --depth 1 --branch hive https://github.com/cdetrio/pyethapp  && \
  cd pyethapp && git checkout 9c54661c681e90c91e3d72d54c8a513e2d0f09a7      && \
  python setup.py develop                                                   && \
  apt-get -y remove git curl automake build-essential libtool pkg-config

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
