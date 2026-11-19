ARG baseimage=ghcr.io/paradigmxyz/reth
ARG tag=latest

FROM $baseimage:$tag as builder

ARG state_root_fallback=""
ENV HIVE_STATE_ROOT_FALLBACK=${state_root_fallback}
ARG bal_parallel_disabled=""
ENV HIVE_BAL_PARALLEL_DISABLED=${bal_parallel_disabled}
ARG share_sparse_trie_with_builder=""
ENV HIVE_SHARE_SPARSE_TRIE_WITH_BUILDER=${share_sparse_trie_with_builder}

# Install script tools.
RUN apt-get update -y
RUN apt-get install -y bash curl jq

# Add genesis mapper script.
ADD genesis.json /genesis.json
ADD mapper.jq /mapper.jq

# Add the startup script.
ADD reth.sh /reth.sh
RUN chmod +x /reth.sh

# Add the enode URL retriever script.
ADD enode.sh /hive-bin/enode.sh
RUN chmod +x /hive-bin/enode.sh

# Create version.txt
RUN /usr/local/bin/reth --version | head -2 | awk 'NR==1 {gsub("reth Version: ", ""); version=$0} NR==2 {gsub("Commit SHA: ", ""); sha=substr($0, 1, 8)} END {print version"+"sha}' > /version.txt

# Export the usual networking ports to allow outside access to the node.
EXPOSE 8545 8546 30303 30303/udp

ENTRYPOINT ["/reth.sh"]
