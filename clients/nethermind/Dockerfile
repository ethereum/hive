## Build Nethermind Via Pre-Built Image:
ARG baseimage=nethermindeth/nethermind
ARG tag=master
FROM $baseimage:$tag

RUN apt-get update && apt-get install -y jq && \
    apt-get clean && rm -rf /var/lib/apt/lists/*

# Create version.txt
RUN /nethermind/nethermind --version > /raw_version.txt && tail -n 1 /raw_version.txt > /version.txt

ADD miner-0x5cd99ac2f0f8c25a1e670f6bab19d52aad69d875.json /networkdata/miner_keystores/miner-0x5cd99ac2f0f8c25a1e670f6bab19d52aad69d875.json
ADD keystore-password /networkdata/keystore_password_filename
ADD enode.sh /enode.sh
ADD test.cfg /configs/test.cfg
ADD nethermind.sh /nethermind.sh

RUN chmod +x /nethermind.sh
RUN mkdir -p /nethermind/Data

# Add the enode script.
RUN mkdir -p /hive-bin
ADD enode.sh /hive-bin/enode.sh
RUN chmod +x /hive-bin/enode.sh

# Expose networking ports
EXPOSE 8545 30303 8551 30303/udp

ENV NETHERMIND_HIVE_ENABLED true
ENTRYPOINT ["/nethermind.sh"]
