# Docker container spec defining and running a simulation to start a single
# client node and run RPC tests against it.
FROM python:2.7

RUN \
  apt-get update                                                 && \
  apt-get install -y git ca-certificates --no-install-recommends

COPY requirements.txt /tmp/
RUN pip install --requirement /tmp/requirements.txt
COPY . /tmp/

# Download and install the test repository itself
RUN \
  git clone https://github.com/cdetrio/interfaces             && \
  cd interfaces                                               && \
  cp -a rpc-specs-tests/. /                                   && \
  mv /configs/bcRPC_API_Test.json /bcRPC_API_Test.json

ADD genesis.json /genesis.json
ADD keys.tar.gz /keys
# Add the simulation controller
COPY *.py /

ENTRYPOINT ["python","/simulator.py"]
