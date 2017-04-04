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

# For local testing
#RUN \
#  cd interfaces                                               && \
#  cp -a rpc-specs-tests/. /                                   && \
#  mv /configs/bcRPC_API_Test.json /bcRPC_API_Test.json

#COPY interfaces/rpc-specs-tests/* /
COPY interfaces/rpc-specs-tests/*.py /
COPY interfaces/rpc-specs-tests/tests /tests
COPY interfaces/rpc-specs-tests/schemas /schemas
COPY interfaces/rpc-specs-tests/configs/bcRPC_API_Test.json /bcRPC_API_Test.json
COPY interfaces/rpc-specs-tests/configs /config

#COPY configs/bcRPC_API_Test.json /bcRPC_API_Test.json
#RUN mv /configs/bcRPC_API_Test.json /bcRPC_API_Test.json

#ADD parity-config.toml /config.toml

# Add the simulation controller
ADD genesis.json /genesis.json
# put keys in folder for parity
ADD keys.tar.gz /keys/Hive
# put keys in folder for geth
ADD keys.tar.gz /keys
COPY *.py /
COPY parity-config.toml /keys/config.toml
COPY password.file /keys/password.file

ENTRYPOINT ["python","/simulator.py"]
