# Builds and runs the EELS (execution-specs) Engine API server as a client.
#
# EELS runs the execution layer specification directly: payloads received
# over `engine_newPayloadV5` are executed by the spec's `state_transition`.
# Only the Amsterdam fork is supported.
FROM ghcr.io/astral-sh/uv:python3.12-bookworm-slim

RUN apt-get update && apt-get install -y bash curl git jq

## Clone and install EELS
# TODO: default to github=ethereum/execution-specs with an empty branch
# once ethereum/execution-specs#3363 is merged.
ARG github=spencer-tb/execution-specs
ARG branch=amsterdam-execution-engine

RUN git clone --depth 1 "https://github.com/$github.git" /execution-specs && \
    cd /execution-specs && \
    if [ -n "$branch" ]; then \
        git fetch --depth 1 origin "$branch" && \
        git checkout FETCH_HEAD; \
    fi

WORKDIR /execution-specs
RUN uv sync --no-dev

# Generate the version.txt file.
RUN echo "eels/$(git -C /execution-specs rev-parse --short HEAD)" > /version.txt

# Inject the startup script.
ADD eels.sh /eels.sh
RUN chmod +x /eels.sh

# Inject the enode id retriever script.
RUN mkdir /hive-bin
ADD enode.sh /hive-bin/enode.sh
RUN chmod +x /hive-bin/enode.sh

# Export the usual networking ports to allow outside access to the node.
EXPOSE 8545 8551

ENTRYPOINT ["/eels.sh"]
