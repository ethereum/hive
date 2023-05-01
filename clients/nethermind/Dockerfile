ARG branch=latest
FROM nethermindeth/hive:$branch

RUN apt-get update && apt-get install -y wget
RUN wget https://github.com/stedolan/jq/releases/download/jq-1.6/jq-linux64 -O /usr/local/bin/jq && \
    echo 'af986793a515d500ab2d35f8d2aecd656e764504b789b66d7e1a0b727a124c44  /usr/local/bin/jq' | sha256sum -c && \
    chmod +x /usr/local/bin/jq
ADD genesis.json /chainspec.json
ADD miner-0x5cd99ac2f0f8c25a1e670f6bab19d52aad69d875.json /networkdata/miner_keystores/miner-0x5cd99ac2f0f8c25a1e670f6bab19d52aad69d875.json
ADD keystore-password /networkdata/keystore_password_filename
ADD enode.sh /enode.sh
ADD test.cfg /configs/test.cfg
ADD nethermind.sh /nethermind.sh

RUN chmod +x /nethermind.sh

# Add the enode script.
RUN mkdir -p /hive-bin
ADD enode.sh /hive-bin/enode.sh
RUN chmod +x /hive-bin/enode.sh

# Write the version file.
RUN dotnet /nethermind/Nethermind.Runner.dll --version > /raw_version.txt && tail -n 1 /raw_version.txt > /version.txt

ENV NETHERMIND_HIVE_ENABLED true

EXPOSE 8545 30303 8551 30303/udp
ENTRYPOINT ["/nethermind.sh"]
