# Docker container spec defining and running a simple smoke test consisting of
# starting a single client node and retrieving some data from it.
FROM jfloff/alpine-python:2.7-onbuild

# Install curl to allow controlling the simulator as well as to interact with the
# client via HTTP-RPC.
RUN apk add --update curl git && \
  git clone --depth 1 https://github.com/ethereum/tests.git && echo "cacheoff6"


# Add the simulation controller
ADD simulator.sh /simulator.sh
RUN chmod +x /simulator.sh
COPY *.py /

ENTRYPOINT ["python","/simulator.py"]
