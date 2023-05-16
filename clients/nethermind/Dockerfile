ARG user=nethermindeth
ARG repo=hive
ARG branch=latest
FROM $user/$repo:$branch

RUN apt-get update && apt-get install -y wget
RUN wget https://github.com/stedolan/jq/releases/download/jq-1.6/jq-linux64 -O /usr/local/bin/jq && \
    echo 'af986793a515d500ab2d35f8d2aecd656e764504b789b66d7e1a0b727a124c44  /usr/local/bin/jq' | sha256sum -c && \
    chmod +x /usr/local/bin/jq

ADD genesis.json /genesis.json
ADD mapper.jq /mapper.jq
ADD mkconfig.jq /mkconfig.jq
ADD enode.sh /enode.sh
ADD nethermind.sh /nethermind.sh

RUN chmod +x /nethermind.sh

# Add the enode script.
RUN mkdir /hive-bin
ADD enode.sh /hive-bin/enode.sh
RUN chmod +x /hive-bin/enode.sh

# Write the version file.
RUN dotnet /nethermind/Nethermind.Runner.dll --version > /raw_version.txt && tail -n 1 /raw_version.txt > /version.txt

ENV NETHERMIND_HIVE_ENABLED true

EXPOSE 8545 30303 30303/udp
ENTRYPOINT ["/nethermind.sh"]
